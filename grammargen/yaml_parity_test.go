package grammargen

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

func TestYAMLSimpleMappingParity(t *testing.T) {
	genLang, refLang := loadGeneratedYAMLLanguagesForParity(t)
	assertGeneratedAndReferenceDeepParity(t, genLang, refLang, "key: value\n")
}

func TestYAMLTagDirectiveSequenceParity(t *testing.T) {
	genLang, refLang := loadGeneratedYAMLLanguagesForParity(t)
	src := "%TAG ! tag:clarkevans.com,2002:\n" +
		"--- !shape\n" +
		"- !circle\n"
	assertGeneratedAndReferenceDeepParity(t, genLang, refLang, src)
}

func TestYAMLFoldedBlockScalarParity(t *testing.T) {
	genLang, refLang := loadGeneratedYAMLLanguagesForParity(t)
	src := ">\n" +
		" Sammy Sosa completed another\n" +
		" fine season with great stats.\n" +
		"\n" +
		"   63 Home Runs\n" +
		"   0.288 Batting Average\n" +
		"\n" +
		" What a year!\n"
	assertGeneratedAndReferenceDeepParity(t, genLang, refLang, src)
}

func loadGeneratedYAMLLanguagesForParity(t *testing.T) (*gotreesitter.Language, *gotreesitter.Language) {
	t.Helper()

	source, err := os.ReadFile(yamlGrammarJSONPathForTest(t))
	if err != nil {
		t.Fatalf("read yaml grammar.json: %v", err)
	}
	gram, err := ImportGrammarJSON(source)
	if err != nil {
		t.Fatalf("import yaml grammar: %v", err)
	}
	genLang, err := generateWithTimeout(gram, 90*time.Second)
	if err != nil {
		t.Fatalf("generate yaml language: %v", err)
	}
	refLang := grammars.YamlLanguage()
	adaptExternalScanner(refLang, genLang)
	return genLang, refLang
}

func yamlGrammarJSONPathForTest(t *testing.T) string {
	t.Helper()

	candidates := []string{
		"/tmp/grammar_parity/yaml/src/grammar.json",
		"/tmp/tree-sitter-yaml/src/grammar.json",
		".parity_seed/yaml/src/grammar.json",
		"../.parity_seed/yaml/src/grammar.json",
	}
	globs := []string{
		"/tmp/gotreesitter-parity-*/repos/yaml/src/grammar.json",
	}
	for _, pattern := range globs {
		matches, err := filepath.Glob(pattern)
		if err == nil && len(matches) > 0 {
			candidates = append(candidates, matches...)
		}
	}
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	t.Skip("YAML grammar.json not available")
	return ""
}
