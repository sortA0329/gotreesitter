package main

import "testing"

func TestBuildBoardAggregatesL3AndL4(t *testing.T) {
	manifest := &corpusManifest{
		Languages: []string{"go", "rust", "yaml"},
		Entries: []corpusManifestEntry{
			{Language: "go", Bucket: "medium", SHA256: "m1", OutputPath: "/tmp/go/medium__a.go"},
			{Language: "go", Bucket: "large", SHA256: "l1", OutputPath: "/tmp/go/large__a.go"},
			{Language: "rust", Bucket: "medium", SHA256: "m2", OutputPath: "/tmp/rust/medium__a.rs"},
			{Language: "yaml", Bucket: "large", SHA256: "l2", OutputPath: "/tmp/yaml/large__a.yaml"},
		},
	}
	results := []parityResult{
		{Language: "go", FileID: "medium__a.go", SourceSHA256: "m1", Pass: true},
		{Language: "go", FileID: "large__a.go", SourceSHA256: "l1", Pass: false, Category: "error"},
		{Language: "rust", FileID: "medium__a.rs", SourceSHA256: "m2", Pass: true},
		{Language: "yaml", FileID: "large__a.yaml", SourceSHA256: "l2", Pass: true},
	}

	board := buildBoard("manifest.json", "results.jsonl", manifest, results)

	if got, want := board.L3.ApplicableLangs, 2; got != want {
		t.Fatalf("L3.ApplicableLangs = %d, want %d", got, want)
	}
	if got, want := board.L3.GreenLangs, 2; got != want {
		t.Fatalf("L3.GreenLangs = %d, want %d", got, want)
	}
	if got, want := board.L4.ApplicableLangs, 2; got != want {
		t.Fatalf("L4.ApplicableLangs = %d, want %d", got, want)
	}
	if got, want := board.L4.GreenLangs, 1; got != want {
		t.Fatalf("L4.GreenLangs = %d, want %d", got, want)
	}

	for _, lang := range board.Languages {
		switch lang.Language {
		case "go":
			if lang.L3.Status != "green" {
				t.Fatalf("go L3 status = %q, want green", lang.L3.Status)
			}
			if lang.L4.Status != "red" {
				t.Fatalf("go L4 status = %q, want red", lang.L4.Status)
			}
		case "rust":
			if lang.L3.Status != "green" {
				t.Fatalf("rust L3 status = %q, want green", lang.L3.Status)
			}
			if lang.L4.Status != "na" {
				t.Fatalf("rust L4 status = %q, want na", lang.L4.Status)
			}
		case "yaml":
			if lang.L3.Status != "na" {
				t.Fatalf("yaml L3 status = %q, want na", lang.L3.Status)
			}
			if lang.L4.Status != "green" {
				t.Fatalf("yaml L4 status = %q, want green", lang.L4.Status)
			}
		}
	}
}

func TestSummarizeLevelFlagsMissingFiles(t *testing.T) {
	entries := []corpusManifestEntry{
		{Language: "go", Bucket: "medium", SHA256: "a", OutputPath: "/tmp/go/medium__a.go"},
	}

	summary := summarizeLevel(entries, "go", map[string]parityResult{})
	if summary.Status != "red" {
		t.Fatalf("status = %q, want red", summary.Status)
	}
	if len(summary.MissingFiles) != 1 || summary.MissingFiles[0] != "medium__a.go" {
		t.Fatalf("missing files = %#v, want medium__a.go", summary.MissingFiles)
	}
}
