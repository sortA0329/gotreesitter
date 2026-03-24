package grammargen

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/odvcencio/gotreesitter"
)

func TestTypeScriptCorpusSnippetParity(t *testing.T) {
	if raceEnabled {
		t.Skip("skip heavyweight TypeScript parity generation under -race; non-race coverage keeps the generated-vs-reference check")
	}

	genLang, refLang := loadImportedParityLanguages(t, "typescript")
	tests := []struct {
		name string
		src  string
	}{
		{
			name: "generic_call",
			src:  "f<T>(x)\n",
		},
		{
			name: "optional_chained_generic_call",
			src:  "A?.<B>();\n",
		},
		{
			name: "member_generic_call_and_nested_type_args",
			src:  "a.b<[C]>();\na<C.D[]>();\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertGeneratedAndReferenceParity(t, genLang, refLang, tt.src)
		})
	}
}

func TestTSXCorpusSnippetParity(t *testing.T) {
	if raceEnabled {
		t.Skip("skip heavyweight TypeScript parity generation under -race; non-race coverage keeps the generated-vs-reference check")
	}

	genLang, refLang := loadImportedParityLanguages(t, "tsx")
	tests := []struct {
		name string
		src  string
	}{
		{
			name: "generic_call",
			src:  "f<T>(x)\n",
		},
		{
			name: "optional_chained_generic_call",
			src:  "A?.<B>();\n",
		},
		{
			name: "member_generic_call_and_nested_type_args",
			src:  "a.b<[C]>();\na<C.D[]>();\n",
		},
		{
			name: "jsx_generic_ambiguity_from_functions_corpus",
			src:  "<A>(amount, interestRate, duration): number => 2\n\nfunction* foo<A>(amount, interestRate, duration): number {\n\tyield amount * interestRate * duration / 12\n}\n\n(module: any): number => 2\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertGeneratedAndReferenceParity(t, genLang, refLang, tt.src)
		})
	}
}

func loadImportedParityLanguages(t *testing.T, grammarName string) (*gotreesitter.Language, *gotreesitter.Language) {
	t.Helper()

	var grammarSpec importParityGrammar
	for _, g := range importParityGrammars {
		if g.name == grammarName {
			grammarSpec = g
			break
		}
	}
	if grammarSpec.name == "" {
		t.Fatalf("%s import parity grammar not found", grammarName)
	}
	if grammarSpec.jsonPath != "" {
		if _, err := os.Stat(grammarSpec.jsonPath); err != nil && strings.HasPrefix(grammarSpec.jsonPath, "/tmp/grammar_parity/") {
			relSeedPath := filepath.Join(".parity_seed", strings.TrimPrefix(grammarSpec.jsonPath, "/tmp/grammar_parity/"))
			switch {
			case fileExists(relSeedPath):
				grammarSpec.jsonPath = relSeedPath
			case fileExists(filepath.Join("..", relSeedPath)):
				grammarSpec.jsonPath = filepath.Join("..", relSeedPath)
			}
		}
	}

	gram, err := importParityGrammarSource(grammarSpec)
	if err != nil {
		t.Fatalf("import %s grammar: %v", grammarName, err)
	}

	timeout := grammarSpec.genTimeout
	if timeout == 0 {
		timeout = 180 * time.Second
	}
	genLang, err := generateWithTimeout(gram, timeout)
	if err != nil {
		t.Fatalf("generate %s language: %v", grammarName, err)
	}
	refLang := grammarSpec.blobFunc()
	adaptExternalScanner(refLang, genLang)
	return genLang, refLang
}

func assertGeneratedAndReferenceParity(t *testing.T, genLang, refLang *gotreesitter.Language, src string) {
	t.Helper()

	data := []byte(src)
	genTree, err := gotreesitter.NewParser(genLang).Parse(data)
	if err != nil {
		t.Fatalf("generated parse: %v", err)
	}
	refTree, err := gotreesitter.NewParser(refLang).Parse(data)
	if err != nil {
		t.Fatalf("reference parse: %v", err)
	}

	genRoot := genTree.RootNode()
	refRoot := refTree.RootNode()
	genSExpr := safeSExpr(genRoot, genLang, 256)
	refSExpr := safeSExpr(refRoot, refLang, 256)

	if genRoot.HasError() != refRoot.HasError() {
		if os.Getenv("DIAG_TS_CORPUS_SNIPPET") == "1" {
			logCorpusSnippetDiag(t, "gen", genLang, data)
			logCorpusSnippetDiag(t, "ref", refLang, data)
		}
		t.Fatalf("error mismatch: gen=%v ref=%v\nGEN: %s\nREF: %s", genRoot.HasError(), refRoot.HasError(), genSExpr, refSExpr)
	}
	if genSExpr != refSExpr {
		divs := compareTreesDeep(genRoot, genLang, refRoot, refLang, "root", 10)
		if os.Getenv("DIAG_TS_CORPUS_SNIPPET") == "1" {
			logCorpusSnippetDiag(t, "gen", genLang, data)
			logCorpusSnippetDiag(t, "ref", refLang, data)
		}
		if len(divs) == 0 {
			return
		}
		t.Fatalf("sexpr mismatch\nGEN: %s\nREF: %s\nDIVS: %v", genSExpr, refSExpr, divs)
	}
}

func logCorpusSnippetDiag(t *testing.T, label string, lang *gotreesitter.Language, src []byte) {
	t.Helper()

	parser := gotreesitter.NewParser(lang)
	parser.SetGLRTrace(true)
	parser.SetLogger(func(kind gotreesitter.ParserLogType, msg string) {
		switch kind {
		case gotreesitter.ParserLogLex:
			var sym, start, end int
			if _, err := fmt.Sscanf(msg, "token sym=%d start=%d end=%d", &sym, &start, &end); err == nil &&
				sym >= 0 && sym < len(lang.SymbolNames) && start >= 0 && end >= start && end <= len(src) {
				t.Logf("[%s][lex] sym=%d raw=%q text=%q start=%d end=%d", label, sym, lang.SymbolNames[sym], string(src[start:end]), start, end)
				return
			}
			t.Logf("[%s][lex] %s", label, msg)
		case gotreesitter.ParserLogParse:
			t.Logf("[%s][parse] %s", label, msg)
		}
	})

	tree, err := parser.Parse(src)
	if err != nil {
		t.Logf("[%s] parse error: %v", label, err)
		return
	}
	t.Logf("[%s] hasError=%v sexpr=%s", label, tree.RootNode().HasError(), safeSExpr(tree.RootNode(), lang, 256))
}
