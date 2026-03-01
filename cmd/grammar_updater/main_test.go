package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseEntryLine(t *testing.T) {
	entry, err := parseEntryLine("go https://github.com/tree-sitter/tree-sitter-go 2346a3ab1bb3857b48b29d779a1ef9799a248cd7 src .go")
	if err != nil {
		t.Fatalf("parse entry: %v", err)
	}
	if entry.Name != "go" {
		t.Fatalf("unexpected name: %q", entry.Name)
	}
	if entry.RepoURL != "https://github.com/tree-sitter/tree-sitter-go" {
		t.Fatalf("unexpected repo: %q", entry.RepoURL)
	}
	if entry.Commit != "2346a3ab1bb3857b48b29d779a1ef9799a248cd7" {
		t.Fatalf("unexpected commit: %q", entry.Commit)
	}
	if entry.Subdir != "src" {
		t.Fatalf("unexpected subdir: %q", entry.Subdir)
	}
	if len(entry.Extensions) != 1 || entry.Extensions[0] != ".go" {
		t.Fatalf("unexpected extensions: %#v", entry.Extensions)
	}
}

func TestParseEntryLineManifestStyle(t *testing.T) {
	entry, err := parseEntryLine("xml https://github.com/tree-sitter-grammars/tree-sitter-xml xml/src .xml")
	if err != nil {
		t.Fatalf("parse entry: %v", err)
	}
	if entry.Commit != "" {
		t.Fatalf("expected empty commit, got %q", entry.Commit)
	}
	if entry.Subdir != "xml/src" {
		t.Fatalf("unexpected subdir: %q", entry.Subdir)
	}
	if got, want := strings.Join(entry.Extensions, ","), ".xml"; got != want {
		t.Fatalf("extensions mismatch: got %q want %q", got, want)
	}
}

func TestSyncMissingEntriesFromManifest(t *testing.T) {
	lock := &lockFile{
		lines: []lockLine{
			{raw: "# header"},
			{isEntry: true, entry: lockEntry{Name: "go", RepoURL: "https://example.com/go", Commit: "aaaaaaaa", Subdir: "src", Extensions: []string{".go"}}},
		},
	}

	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "languages.manifest")
	manifest := strings.Join([]string{
		"# name repo [subdir] [exts]",
		"go https://example.com/go src .go",
		"rust https://example.com/rust src .rs",
	}, "\n")
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	added, err := syncMissingEntriesFromManifest(lock, manifestPath)
	if err != nil {
		t.Fatalf("sync manifest: %v", err)
	}
	if added != 1 {
		t.Fatalf("added mismatch: got %d want 1", added)
	}

	entries := lock.entryPointers()
	if len(entries) != 2 {
		t.Fatalf("entry count mismatch: got %d want 2", len(entries))
	}
	if entries[1].Name != "rust" {
		t.Fatalf("expected rust entry, got %q", entries[1].Name)
	}
}

func TestWriteLockFilePreservesComments(t *testing.T) {
	lock := &lockFile{
		lines: []lockLine{
			{raw: "# first"},
			{isEntry: true, entry: lockEntry{Name: "go", RepoURL: "https://example.com/go", Commit: "aaaaaaaa", Subdir: "src", Extensions: []string{".go"}}},
			{raw: ""},
			{raw: "# second"},
			{isEntry: true, entry: lockEntry{Name: "rust", RepoURL: "https://example.com/rust", Commit: "bbbbbbbb", Subdir: "src", Extensions: []string{".rs"}}},
		},
	}
	outPath := filepath.Join(t.TempDir(), "languages.lock")
	if err := writeLockFile(outPath, lock); err != nil {
		t.Fatalf("write lock: %v", err)
	}
	raw, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read lock: %v", err)
	}
	got := string(raw)
	if !strings.Contains(got, "# first\n") || !strings.Contains(got, "\n# second\n") {
		t.Fatalf("comments not preserved:\n%s", got)
	}
	if !strings.Contains(got, "go https://example.com/go aaaaaaaa src .go") {
		t.Fatalf("missing go entry: %s", got)
	}
}
