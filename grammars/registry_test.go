package grammars

import (
	"testing"

	"github.com/odvcencio/gotreesitter"
)

func TestDetectLanguageGo(t *testing.T) {
	entry := DetectLanguage("main.go")
	if entry == nil {
		t.Fatal("expected to detect Go language for main.go, got nil")
	}
	if entry.Name != "go" {
		t.Fatalf("expected language name %q, got %q", "go", entry.Name)
	}
	if entry.TokenSourceFactory == nil {
		t.Fatal("expected Go language to register a TokenSourceFactory")
	}
}

func TestDetectLanguageUnknown(t *testing.T) {
	entry := DetectLanguage("readme.xyz")
	if entry != nil {
		t.Fatalf("expected nil for unknown extension, got %q", entry.Name)
	}
}

func TestAllLanguages(t *testing.T) {
	langs := AllLanguages()
	if len(langs) == 0 {
		t.Fatal("expected at least one registered language, got 0")
	}

	found := false
	for _, l := range langs {
		if l.Name == "go" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected Go language to be registered")
	}
}

func TestDetectLanguageByShebang(t *testing.T) {
	// No languages have shebangs registered, so this should return nil.
	entry := DetectLanguageByShebang("#!/usr/bin/env python3")
	if entry != nil {
		t.Fatalf("expected nil for unregistered shebang, got %q", entry.Name)
	}
}

func TestAuditParseSupportIncludesGoCustomTokenSource(t *testing.T) {
	reports := AuditParseSupport()
	if len(reports) == 0 {
		t.Fatal("expected parse support reports")
	}

	var goReport *ParseSupport
	for i := range reports {
		if reports[i].Name == "go" {
			goReport = &reports[i]
			break
		}
	}
	if goReport == nil {
		t.Fatal("expected go parse support report")
	}
	if goReport.Backend != ParseBackendTokenSource {
		t.Fatalf("expected go backend %q, got %q", ParseBackendTokenSource, goReport.Backend)
	}
}

func TestAuditParseSupportIncludesCCustomTokenSource(t *testing.T) {
	reports := AuditParseSupport()
	if len(reports) == 0 {
		t.Fatal("expected parse support reports")
	}

	var cReport *ParseSupport
	for i := range reports {
		if reports[i].Name == "c" {
			cReport = &reports[i]
			break
		}
	}
	if cReport == nil {
		t.Fatal("expected c parse support report")
	}
	if cReport.Backend != ParseBackendTokenSource {
		t.Fatalf("expected c backend %q, got %q", ParseBackendTokenSource, cReport.Backend)
	}
}

func TestAuditParseSupportIncludesCppDFA(t *testing.T) {
	reports := AuditParseSupport()
	if len(reports) == 0 {
		t.Fatal("expected parse support reports")
	}

	var cppReport *ParseSupport
	for i := range reports {
		if reports[i].Name == "cpp" {
			cppReport = &reports[i]
			break
		}
	}
	if cppReport == nil {
		t.Fatal("expected cpp parse support report")
	}
	if cppReport.Backend != ParseBackendDFA {
		t.Fatalf("expected cpp backend %q, got %q", ParseBackendDFA, cppReport.Backend)
	}
}

func TestAuditParseSupportIncludesJSONCustomTokenSource(t *testing.T) {
	reports := AuditParseSupport()
	if len(reports) == 0 {
		t.Fatal("expected parse support reports")
	}

	var jsonReport *ParseSupport
	for i := range reports {
		if reports[i].Name == "json" {
			jsonReport = &reports[i]
			break
		}
	}
	if jsonReport == nil {
		t.Fatal("expected json parse support report")
	}
	if jsonReport.Backend != ParseBackendTokenSource {
		t.Fatalf("expected json backend %q, got %q", ParseBackendTokenSource, jsonReport.Backend)
	}
}

func TestAuditParseSupportIncludesJavaCustomTokenSource(t *testing.T) {
	reports := AuditParseSupport()
	if len(reports) == 0 {
		t.Fatal("expected parse support reports")
	}

	var javaReport *ParseSupport
	for i := range reports {
		if reports[i].Name == "java" {
			javaReport = &reports[i]
			break
		}
	}
	if javaReport == nil {
		t.Fatal("expected java parse support report")
	}
	if javaReport.Backend != ParseBackendTokenSource {
		t.Fatalf("expected java backend %q, got %q", ParseBackendTokenSource, javaReport.Backend)
	}
}

func TestAuditParseSupportIncludesLuaCustomTokenSource(t *testing.T) {
	reports := AuditParseSupport()
	if len(reports) == 0 {
		t.Fatal("expected parse support reports")
	}

	var luaReport *ParseSupport
	for i := range reports {
		if reports[i].Name == "lua" {
			luaReport = &reports[i]
			break
		}
	}
	if luaReport == nil {
		t.Fatal("expected lua parse support report")
	}
	if luaReport.Backend != ParseBackendTokenSource {
		t.Fatalf("expected lua backend %q, got %q", ParseBackendTokenSource, luaReport.Backend)
	}
}

func TestAuditParseSupportIncludesJavaScriptDFA(t *testing.T) {
	reports := AuditParseSupport()
	if len(reports) == 0 {
		t.Fatal("expected parse support reports")
	}

	var jsReport *ParseSupport
	for i := range reports {
		if reports[i].Name == "javascript" {
			jsReport = &reports[i]
			break
		}
	}
	if jsReport == nil {
		t.Fatal("expected javascript parse support report")
	}
	if jsReport.Backend != ParseBackendDFA {
		t.Fatalf("expected javascript backend %q, got %q", ParseBackendDFA, jsReport.Backend)
	}
}

func TestAuditParseSupportIncludesTypeScriptDFA(t *testing.T) {
	reports := AuditParseSupport()
	if len(reports) == 0 {
		t.Fatal("expected parse support reports")
	}

	var tsReport *ParseSupport
	for i := range reports {
		if reports[i].Name == "typescript" {
			tsReport = &reports[i]
			break
		}
	}
	if tsReport == nil {
		t.Fatal("expected typescript parse support report")
	}
	if tsReport.Backend != ParseBackendDFA {
		t.Fatalf("expected typescript backend %q, got %q", ParseBackendDFA, tsReport.Backend)
	}
}

func TestAuditParseSupportIncludesRustDFA(t *testing.T) {
	reports := AuditParseSupport()
	if len(reports) == 0 {
		t.Fatal("expected parse support reports")
	}

	var rustReport *ParseSupport
	for i := range reports {
		if reports[i].Name == "rust" {
			rustReport = &reports[i]
			break
		}
	}
	if rustReport == nil {
		t.Fatal("expected rust parse support report")
	}
	if rustReport.Backend != ParseBackendDFA {
		t.Fatalf("expected rust backend %q, got %q", ParseBackendDFA, rustReport.Backend)
	}
}

func TestCoreLanguagesHaveCompilableTagsQuery(t *testing.T) {
	core := []string{
		"go",
		"python",
		"javascript",
		"typescript",
		"tsx",
		"rust",
		"java",
		"c",
		"cpp",
	}

	entries := AllLanguages()
	entryByName := make(map[string]LangEntry, len(entries))
	for _, entry := range entries {
		entryByName[entry.Name] = entry
	}

	for _, name := range core {
		name := name
		t.Run(name, func(t *testing.T) {
			entry, ok := entryByName[name]
			if !ok {
				t.Fatalf("expected %q language to be registered", name)
			}
			if entry.Language == nil {
				t.Fatalf("expected %q language loader", name)
			}
			if entry.TagsQuery == "" {
				t.Fatalf("expected non-empty TagsQuery for %q", name)
			}
			if _, err := gotreesitter.NewTagger(entry.Language(), entry.TagsQuery); err != nil {
				t.Fatalf("compile tags query for %q: %v", name, err)
			}
		})
	}
}

func TestInferredTagsQueryCoverage(t *testing.T) {
	entries := AllLanguages()
	if len(entries) == 0 {
		t.Fatal("expected registered languages")
	}

	withTags := 0
	for _, entry := range entries {
		if entry.TagsQuery != "" {
			withTags++
		}
	}

	// Core set (9) is explicit. Inference should expand this materially.
	if withTags < 30 {
		t.Fatalf("expected inferred tags query coverage to be >=30 languages, got %d", withTags)
	}
}
