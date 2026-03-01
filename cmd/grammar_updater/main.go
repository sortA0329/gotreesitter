// Command grammar_updater refreshes pinned grammar commits in
// grammars/languages.lock and emits a machine-readable update report.
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"
)

type lockEntry struct {
	Name       string
	RepoURL    string
	Commit     string
	Subdir     string
	Extensions []string
}

type lockLine struct {
	raw     string
	isEntry bool
	entry   lockEntry
}

type lockFile struct {
	lines []lockLine
}

type updateStatus string

const (
	updateStatusUnchanged updateStatus = "unchanged"
	updateStatusApplied   updateStatus = "applied"
	updateStatusAvailable updateStatus = "available"
	updateStatusError     updateStatus = "error"
	updateStatusSkipped   updateStatus = "skipped"
)

type updateResult struct {
	Name       string       `json:"name"`
	RepoURL    string       `json:"repo_url"`
	OldRef     string       `json:"old_ref,omitempty"`
	NewRef     string       `json:"new_ref,omitempty"`
	Subdir     string       `json:"subdir,omitempty"`
	Extensions []string     `json:"extensions,omitempty"`
	Status     updateStatus `json:"status"`
	Applied    bool         `json:"applied"`
	Error      string       `json:"error,omitempty"`
}

type updateReport struct {
	GeneratedAt       string         `json:"generated_at"`
	LockPath          string         `json:"lock_path"`
	ManifestPath      string         `json:"manifest_path,omitempty"`
	WriteApplied      bool           `json:"write_applied"`
	SyncManifest      bool           `json:"sync_manifest"`
	FilterAllowList   string         `json:"filter_allow_list,omitempty"`
	MaxUpdates        int            `json:"max_updates"`
	TotalEntries      int            `json:"total_entries"`
	CheckedEntries    int            `json:"checked_entries"`
	AppliedCount      int            `json:"applied_count"`
	AvailableCount    int            `json:"available_count"`
	UnchangedCount    int            `json:"unchanged_count"`
	SkippedCount      int            `json:"skipped_count"`
	ErrorCount        int            `json:"error_count"`
	AddedFromManifest int            `json:"added_from_manifest"`
	Results           []updateResult `json:"results"`
}

func main() {
	var (
		lockPath      = flag.String("lock", "grammars/languages.lock", "path to lock file")
		reportPath    = flag.String("report", "", "optional output path for JSON report")
		writeChanges  = flag.Bool("write", false, "write updated lock file commits in place")
		maxUpdates    = flag.Int("max-updates", 0, "max number of updates to apply (0 = unlimited)")
		workers       = flag.Int("workers", 8, "number of concurrent remote HEAD lookups")
		failOnError   = flag.Bool("fail-on-error", true, "exit non-zero if any repo lookup fails")
		failOnChange  = flag.Bool("fail-on-change", false, "exit non-zero when updates are available")
		syncManifest  = flag.Bool("sync-manifest", false, "add manifest languages missing from lock")
		manifestPath  = flag.String("manifest", "grammars/languages.manifest", "path to manifest (used when -sync-manifest)")
		allowListPath = flag.String("allow-list", "", "optional newline-delimited language allow-list")
	)
	flag.Parse()

	lf, err := parseLockFile(*lockPath)
	if err != nil {
		exitf("parse lock file: %v", err)
	}

	report := updateReport{
		GeneratedAt:  time.Now().UTC().Format(time.RFC3339),
		LockPath:     *lockPath,
		ManifestPath: "",
		WriteApplied: *writeChanges,
		SyncManifest: *syncManifest,
		MaxUpdates:   *maxUpdates,
		Results:      make([]updateResult, 0),
	}

	allowSet := map[string]struct{}{}
	if strings.TrimSpace(*allowListPath) != "" {
		allowSet, err = parseAllowList(*allowListPath)
		if err != nil {
			exitf("parse allow-list: %v", err)
		}
		report.FilterAllowList = *allowListPath
	}

	if *syncManifest {
		report.ManifestPath = *manifestPath
		added, syncErr := syncMissingEntriesFromManifest(lf, *manifestPath)
		if syncErr != nil {
			exitf("sync manifest: %v", syncErr)
		}
		report.AddedFromManifest = added
	}

	entries := lf.entryPointers()
	report.TotalEntries = len(entries)

	filtered := entries
	if len(allowSet) > 0 {
		filtered = make([]*lockEntry, 0, len(entries))
		for _, entry := range entries {
			if _, ok := allowSet[entry.Name]; ok {
				filtered = append(filtered, entry)
			}
		}
	}
	report.CheckedEntries = len(filtered)

	heads, headErrs := resolveRepoHeads(filtered, *workers)
	appliedBudget := 0
	if *maxUpdates <= 0 {
		appliedBudget = int(^uint(0) >> 1)
	} else {
		appliedBudget = *maxUpdates
	}

	changedInMemory := false
	for _, entry := range entries {
		if len(allowSet) > 0 {
			if _, ok := allowSet[entry.Name]; !ok {
				report.Results = append(report.Results, updateResult{
					Name:       entry.Name,
					RepoURL:    entry.RepoURL,
					OldRef:     entry.Commit,
					NewRef:     "",
					Subdir:     entry.Subdir,
					Extensions: append([]string(nil), entry.Extensions...),
					Status:     updateStatusSkipped,
				})
				report.SkippedCount++
				continue
			}
		}

		headErr, hasErr := headErrs[entry.RepoURL]
		if hasErr {
			report.Results = append(report.Results, updateResult{
				Name:       entry.Name,
				RepoURL:    entry.RepoURL,
				OldRef:     entry.Commit,
				Subdir:     entry.Subdir,
				Extensions: append([]string(nil), entry.Extensions...),
				Status:     updateStatusError,
				Error:      headErr.Error(),
			})
			report.ErrorCount++
			continue
		}

		newRef := heads[entry.RepoURL]
		oldRef := entry.Commit
		result := updateResult{
			Name:       entry.Name,
			RepoURL:    entry.RepoURL,
			OldRef:     oldRef,
			NewRef:     newRef,
			Subdir:     entry.Subdir,
			Extensions: append([]string(nil), entry.Extensions...),
			Status:     updateStatusUnchanged,
		}

		if newRef == "" {
			result.Status = updateStatusError
			result.Error = "resolved empty remote ref"
			report.ErrorCount++
			report.Results = append(report.Results, result)
			continue
		}

		if oldRef == newRef {
			report.UnchangedCount++
			report.Results = append(report.Results, result)
			continue
		}

		if appliedBudget <= 0 || !*writeChanges {
			result.Status = updateStatusAvailable
			report.AvailableCount++
			report.Results = append(report.Results, result)
			continue
		}

		entry.Commit = newRef
		result.Status = updateStatusApplied
		result.Applied = true
		report.AppliedCount++
		appliedBudget--
		changedInMemory = true
		report.Results = append(report.Results, result)
	}

	if *writeChanges && changedInMemory {
		if err := writeLockFile(*lockPath, lf); err != nil {
			exitf("write lock file: %v", err)
		}
	}

	sort.Slice(report.Results, func(i, j int) bool {
		return report.Results[i].Name < report.Results[j].Name
	})

	fmt.Printf(
		"grammar_updater: checked=%d total=%d applied=%d available=%d unchanged=%d skipped=%d errors=%d added=%d\n",
		report.CheckedEntries,
		report.TotalEntries,
		report.AppliedCount,
		report.AvailableCount,
		report.UnchangedCount,
		report.SkippedCount,
		report.ErrorCount,
		report.AddedFromManifest,
	)

	if reportPath != nil && strings.TrimSpace(*reportPath) != "" {
		if err := writeJSONReport(*reportPath, report); err != nil {
			exitf("write report: %v", err)
		}
		fmt.Printf("grammar_updater: wrote report %s\n", *reportPath)
	}

	if *failOnError && report.ErrorCount > 0 {
		os.Exit(1)
	}
	if *failOnChange && (report.AvailableCount > 0 || report.AppliedCount > 0) {
		os.Exit(1)
	}
}

func parseAllowList(path string) (map[string]struct{}, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	out := make(map[string]struct{})
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out[line] = struct{}{}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func syncMissingEntriesFromManifest(lf *lockFile, manifestPath string) (int, error) {
	manifestEntries, err := parseManifestEntries(manifestPath)
	if err != nil {
		return 0, err
	}
	seen := make(map[string]struct{}, len(lf.entryPointers()))
	for _, entry := range lf.entryPointers() {
		seen[entry.Name] = struct{}{}
	}

	added := 0
	for _, me := range manifestEntries {
		if _, ok := seen[me.Name]; ok {
			continue
		}
		lf.lines = append(lf.lines, lockLine{
			isEntry: true,
			entry: lockEntry{
				Name:       me.Name,
				RepoURL:    me.RepoURL,
				Commit:     me.Commit,
				Subdir:     me.Subdir,
				Extensions: append([]string(nil), me.Extensions...),
			},
		})
		seen[me.Name] = struct{}{}
		added++
	}
	return added, nil
}

func parseManifestEntries(path string) ([]lockEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var out []lockEntry
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		entry, err := parseEntryLine(line)
		if err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, lineNum, err)
		}
		out = append(out, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func parseLockFile(path string) (*lockFile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	out := &lockFile{lines: make([]lockLine, 0, 128)}
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		raw := scanner.Text()
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			out.lines = append(out.lines, lockLine{raw: raw, isEntry: false})
			continue
		}
		entry, err := parseEntryLine(line)
		if err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, lineNum, err)
		}
		out.lines = append(out.lines, lockLine{isEntry: true, entry: entry})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func parseEntryLine(line string) (lockEntry, error) {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return lockEntry{}, fmt.Errorf("invalid entry line: %q", line)
	}
	entry := lockEntry{
		Name:    fields[0],
		RepoURL: fields[1],
		Subdir:  "src",
	}
	next := 2
	if len(fields) > next && looksLikeCommitHash(fields[next]) {
		entry.Commit = fields[next]
		next++
	}
	if len(fields) > next {
		entry.Subdir = fields[next]
		next++
	}
	if len(fields) > next {
		exts := strings.Split(fields[next], ",")
		entry.Extensions = make([]string, 0, len(exts))
		for _, ext := range exts {
			ext = strings.TrimSpace(ext)
			if ext != "" {
				entry.Extensions = append(entry.Extensions, ext)
			}
		}
	}
	return entry, nil
}

func looksLikeCommitHash(s string) bool {
	if len(s) < 7 || len(s) > 40 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') {
			continue
		}
		return false
	}
	return true
}

func (lf *lockFile) entryPointers() []*lockEntry {
	out := make([]*lockEntry, 0, len(lf.lines))
	for i := range lf.lines {
		if !lf.lines[i].isEntry {
			continue
		}
		out = append(out, &lf.lines[i].entry)
	}
	return out
}

func writeLockFile(path string, lf *lockFile) error {
	var b strings.Builder
	for i, line := range lf.lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		if !line.isEntry {
			b.WriteString(line.raw)
			continue
		}
		b.WriteString(renderEntry(line.entry))
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func renderEntry(entry lockEntry) string {
	parts := []string{entry.Name, entry.RepoURL}
	if entry.Commit != "" {
		parts = append(parts, entry.Commit)
	}
	if strings.TrimSpace(entry.Subdir) != "" {
		parts = append(parts, entry.Subdir)
	}
	if len(entry.Extensions) > 0 {
		parts = append(parts, strings.Join(entry.Extensions, ","))
	}
	return strings.Join(parts, " ")
}

func resolveRepoHeads(entries []*lockEntry, workers int) (map[string]string, map[string]error) {
	if workers <= 0 {
		workers = 1
	}
	repoSet := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		repoSet[entry.RepoURL] = struct{}{}
	}
	repos := make([]string, 0, len(repoSet))
	for repo := range repoSet {
		repos = append(repos, repo)
	}
	sort.Strings(repos)

	type result struct {
		repo string
		ref  string
		err  error
	}

	workCh := make(chan string)
	resCh := make(chan result, len(repos))
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for repo := range workCh {
				ref, err := resolveRemoteHead(repo)
				resCh <- result{repo: repo, ref: ref, err: err}
			}
		}()
	}

	go func() {
		for _, repo := range repos {
			workCh <- repo
		}
		close(workCh)
		wg.Wait()
		close(resCh)
	}()

	heads := make(map[string]string, len(repos))
	errs := make(map[string]error)
	for res := range resCh {
		if res.err != nil {
			errs[res.repo] = res.err
			continue
		}
		heads[res.repo] = res.ref
	}
	return heads, errs
}

func resolveRemoteHead(repoURL string) (string, error) {
	out, err := exec.Command("git", "ls-remote", "--quiet", repoURL, "HEAD").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git ls-remote %s HEAD: %w (%s)", repoURL, err, strings.TrimSpace(string(out)))
	}
	line := strings.TrimSpace(string(out))
	if line == "" {
		return "", errors.New("empty ls-remote output")
	}
	fields := strings.Fields(line)
	if len(fields) < 1 {
		return "", fmt.Errorf("unexpected ls-remote output: %q", line)
	}
	ref := strings.TrimSpace(fields[0])
	if !looksLikeCommitHash(ref) {
		return "", fmt.Errorf("invalid remote ref %q", ref)
	}
	return ref, nil
}

func writeJSONReport(path string, report updateReport) error {
	out, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	return os.WriteFile(path, out, 0o644)
}

func exitf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "grammar_updater: "+format+"\n", args...)
	os.Exit(1)
}
