package cgoharness

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

type realCorpusManifest struct {
	Languages      []string `json:"languages"`
	MinMediumBytes int      `json:"min_medium_bytes"`
	Entries        []struct {
		Language string `json:"language"`
		Bytes    int64  `json:"bytes"`
	} `json:"entries"`
}

// TestRealCorpusManifestQuality validates generated real-corpus coverage when
// a manifest path is provided via GTS_REAL_CORPUS_MANIFEST.
func TestRealCorpusManifestQuality(t *testing.T) {
	path := strings.TrimSpace(os.Getenv("GTS_REAL_CORPUS_MANIFEST"))
	if path == "" {
		t.Skip("set GTS_REAL_CORPUS_MANIFEST to validate real-corpus quality")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var m realCorpusManifest
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	if len(m.Languages) == 0 {
		t.Fatal("manifest has no languages")
	}
	if m.MinMediumBytes <= 0 {
		t.Fatalf("manifest min_medium_bytes invalid: %d", m.MinMediumBytes)
	}

	type stat struct {
		count      int
		totalBytes int64
	}
	byLang := make(map[string]stat, len(m.Languages))
	for _, entry := range m.Entries {
		s := byLang[entry.Language]
		s.count++
		s.totalBytes += entry.Bytes
		byLang[entry.Language] = s
	}
	for _, lang := range m.Languages {
		s := byLang[lang]
		if s.count < 2 {
			t.Fatalf("language %q has weak corpus coverage: files=%d (<2)", lang, s.count)
		}
		if s.totalBytes < int64(m.MinMediumBytes) {
			t.Fatalf("language %q corpus too small: total_bytes=%d (<%d)", lang, s.totalBytes, m.MinMediumBytes)
		}
	}
}
