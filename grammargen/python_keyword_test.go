package grammargen

import (
	"os"
	"testing"
)

func TestPythonKeywordIdentificationIncludesSoftKeywords(t *testing.T) {
	jsonPath := "/tmp/python-locked-26855ea/src/grammar.json"
	if _, err := os.Stat(jsonPath); err != nil {
		jsonPath = "/tmp/grammar_parity/python/src/grammar.json"
	}
	source, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Skipf("Python grammar.json not available: %v", err)
	}

	gram, err := ImportGrammarJSON(source)
	if err != nil {
		t.Fatalf("import Python grammar.json: %v", err)
	}

	ng, err := Normalize(gram)
	if err != nil {
		t.Fatalf("normalize Python grammar: %v", err)
	}

	want := map[string]bool{
		"match":  false,
		"case":   false,
		"except": false,
	}
	for _, symID := range ng.KeywordSymbols {
		if symID >= 0 && symID < len(ng.Symbols) {
			if _, ok := want[ng.Symbols[symID].Name]; ok {
				want[ng.Symbols[symID].Name] = true
			}
		}
	}

	for name, found := range want {
		if !found {
			t.Fatalf("keyword %q missing from normalized keyword set", name)
		}
	}
}
