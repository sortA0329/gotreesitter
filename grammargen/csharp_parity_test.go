package grammargen

import (
	"os"
	"testing"
	"time"

	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

func TestCSharpInterfaceDefaultMethodInvocationParity(t *testing.T) {
	genLang := loadGeneratedCSharpLanguageForParity(t)
	refLang := grammars.CSharpLanguage()
	adaptExternalScanner(refLang, genLang)

	sample := "interface MyDefault {\n" +
		"  void Log(string message) {\n" +
		"    Console.WriteLine(message);\n" +
		"  }\n" +
		"}\n"

	assertGeneratedAndReferenceDeepParity(t, genLang, refLang, sample)
}

func loadGeneratedCSharpLanguageForParity(t *testing.T) *gotreesitter.Language {
	t.Helper()

	candidates := []string{
		"/tmp/grammar_parity/c_sharp/src/grammar.json",
		".claude/worktrees/grammargen-pr9-resume/harness_out/grammar_seeds/c_sharp/src/grammar.json",
		"../.claude/worktrees/grammargen-pr9-resume/harness_out/grammar_seeds/c_sharp/src/grammar.json",
	}

	var grammarPath string
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			grammarPath = path
			break
		}
	}
	if grammarPath == "" {
		t.Skip("C# grammar.json not available")
	}

	source, err := os.ReadFile(grammarPath)
	if err != nil {
		t.Fatalf("read C# grammar.json: %v", err)
	}
	gram, err := ImportGrammarJSON(source)
	if err != nil {
		t.Fatalf("import C# grammar.json: %v", err)
	}
	genLang, err := generateWithTimeout(gram, 90*time.Second)
	if err != nil {
		t.Fatalf("generate C# language: %v", err)
	}
	return genLang
}
