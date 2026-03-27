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

func TestCSharpQueryJoinClauseParity(t *testing.T) {
	genLang := loadGeneratedCSharpLanguageForParity(t)
	refLang := grammars.CSharpLanguage()
	adaptExternalScanner(refLang, genLang)

	sample := "class C\n" +
		"{\n" +
		"    void M()\n" +
		"    {\n" +
		"        var x = from a in sourceA\n" +
		"                join b in sourceB on a.FK equals b.PK\n" +
		"                select a;\n" +
		"    }\n" +
		"}\n"

	assertGeneratedAndReferenceDeepParity(t, genLang, refLang, sample)
}

func TestCSharpQuerySyntaxClauseParity(t *testing.T) {
	genLang := loadGeneratedCSharpLanguageForParity(t)
	refLang := grammars.CSharpLanguage()
	adaptExternalScanner(refLang, genLang)

	cases := []struct {
		name string
		src  string
	}{
		{
			name: "select_conditional",
			src:  "var x = from a in source select a.B() ? c : c * 2;\n",
		},
		{
			name: "select_assignment",
			src:  "var x = from a in source select somevar = a;\n",
		},
		{
			name: "select_anonymous_object",
			src:  "var x = from a in source select new { Name = a.B };\n",
		},
		{
			name: "where_clause",
			src: "var x = from a in source\n" +
				"  where a.B == \"A\"\n" +
				"  select new { Name = a.B };\n",
		},
		{
			name: "order_by_clause",
			src: "var x = from a in source\n" +
				"  orderby a.A descending\n" +
				"  orderby a.C ascending\n" +
				"  orderby 1\n" +
				"  select a;\n",
		},
		{
			name: "let_clause",
			src: "var x = from a in source\n" +
				"  let z = new { a.A, a.B }\n" +
				"  select z;\n",
		},
		{
			name: "nested_from_clause",
			src: "var x = from a in sourceA\n" +
				"  from b in sourceB\n" +
				"  where a.FK == b.FK\n" +
				"  select new { A.A, B.B };\n",
		},
		{
			name: "group_into_clause",
			src: "var x = from a in sourceA\n" +
				"  group a by a.Country into g\n" +
				"  select new { Country = g.Key, Population = g.Sum(p => p.Population) };\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertGeneratedAndReferenceDeepParity(t, genLang, refLang, tc.src)
		})
	}
}

func TestCSharpConstrainedTypeDeclarationParity(t *testing.T) {
	genLang := loadGeneratedCSharpLanguageForParity(t)
	refLang := grammars.CSharpLanguage()
	adaptExternalScanner(refLang, genLang)

	cases := []struct {
		name string
		src  string
	}{
		{
			name: "class_constraint",
			src:  "public class F<T> where T:struct {}\n",
		},
		{
			name: "struct_constraint",
			src:  "public struct F<T> where T:struct {}\n",
		},
		{
			name: "record_constraints",
			src:  "private record F<T1, T2> where T1 : I1, I2, new() where T2 : I2 { }\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertGeneratedAndReferenceDeepParity(t, genLang, refLang, tc.src)
		})
	}
}

func TestCSharpSourceFileStructureParity(t *testing.T) {
	genLang := loadGeneratedCSharpLanguageForParity(t)
	refLang := grammars.CSharpLanguage()
	adaptExternalScanner(refLang, genLang)

	cases := []struct {
		name string
		src  string
	}{
		{
			name: "namespace_with_using",
			src: "namespace Foo {\n" +
				"  using A;\n" +
				"}\n",
		},
		{
			name: "extern_alias_then_namespace",
			src: "extern alias A;\n" +
				"namespace Foo {\n" +
				"  using A;\n" +
				"}\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertGeneratedAndReferenceDeepParity(t, genLang, refLang, tc.src)
		})
	}
}

func TestCSharpUnicodeIdentifierParity(t *testing.T) {
	genLang := loadGeneratedCSharpLanguageForParity(t)
	refLang := grammars.CSharpLanguage()
	adaptExternalScanner(refLang, genLang)

	sample := "int ග්‍රහලෝකය = 0;\n"
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
