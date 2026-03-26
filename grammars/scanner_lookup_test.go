package grammars

import "testing"

func TestLookupExternalScanner(t *testing.T) {
	// Registered scanners should be found.
	for _, name := range []string{"python", "css", "javascript", "html", "yaml"} {
		if s := LookupExternalScanner(name); s == nil {
			t.Errorf("LookupExternalScanner(%q) = nil, want non-nil", name)
		}
	}

	// Non-existent scanner should return nil.
	if s := LookupExternalScanner("nonexistent_language_xyz"); s != nil {
		t.Errorf("LookupExternalScanner(%q) = %v, want nil", "nonexistent_language_xyz", s)
	}
}

func TestLookupExternalLexStates(t *testing.T) {
	// scss and yaml have registered external lex states.
	for _, name := range []string{"scss", "yaml"} {
		if els := LookupExternalLexStates(name); els == nil {
			t.Errorf("LookupExternalLexStates(%q) = nil, want non-nil", name)
		}
	}

	if els := LookupExternalLexStates("nonexistent_language_xyz"); els != nil {
		t.Errorf("LookupExternalLexStates(%q) = %v, want nil", "nonexistent_language_xyz", els)
	}
}

func TestAdaptScannerForLanguageNilTarget(t *testing.T) {
	if AdaptScannerForLanguage("css", nil) {
		t.Fatal("expected false for nil target language")
	}
}

func TestAdaptScannerForLanguagePreservesExistingExternalLexStates(t *testing.T) {
	ref := YamlLanguage()
	target := *ref
	target.ExternalScanner = nil
	target.ExternalLexStates = [][]bool{
		{false, false},
		{true, false},
	}

	if !AdaptScannerForLanguage("yaml", &target) {
		t.Fatal("AdaptScannerForLanguage(yaml) returned false")
	}
	if target.ExternalScanner == nil {
		t.Fatal("expected external scanner to be attached")
	}
	if len(target.ExternalLexStates) != 2 {
		t.Fatalf("len(ExternalLexStates) = %d, want 2", len(target.ExternalLexStates))
	}
	if !target.ExternalLexStates[1][0] || target.ExternalLexStates[1][1] {
		t.Fatalf("ExternalLexStates was overwritten: %+v", target.ExternalLexStates)
	}
}
