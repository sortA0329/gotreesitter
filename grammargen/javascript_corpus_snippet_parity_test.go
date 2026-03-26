package grammargen

import "testing"

func TestJavaScriptCorpusSnippetParity(t *testing.T) {
	if raceEnabled {
		t.Skip("skip heavyweight JavaScript parity generation under -race; non-race coverage keeps the generated-vs-reference check")
	}

	genLang, refLang := loadImportedParityLanguages(t, "javascript")
	tests := []struct {
		name string
		src  string
	}{
		{
			name: "import_attributes_with_clause",
			src:  "import pkg from \"./package.json\" with { type: \"json\" };\n",
		},
		{
			name: "jsx_in_javascript_corpus",
			src:  "var a = <Foo></Foo>\nb = <Foo.Bar></Foo.Bar>\n",
		},
		{
			name: "template_strings_from_corpus",
			src:  "`one line`;\n`multi\\n  line`;\n`$${'$'}$$${'$'}$$$$`;\n",
		},
		{
			name: "template_strings_corpus_block_exact",
			src: "`one line`;\n" +
				"`multi\n" +
				"  line`;\n\n" +
				"`multi\n" +
				"  ${2 + 2}\n" +
				"  hello\n" +
				"  ${1 + 1, 2 + 2}\n" +
				"  line`;\n\n" +
				"`$$$$`;\n" +
				"`$$$$${ 1 }`;\n\n" +
				"`(a|b)$`;\n\n" +
				"`$`;\n\n" +
				"`$${'$'}$$${'$'}$$$$`;\n\n" +
				"`\\ `;\n\n" +
				"`The command \\`git ${args.join(' ')}\\` exited with an unexpected code: ${exitCode}. The caller should either handle this error, or expect that exit code.`;\n\n" +
				"`\\\\`;\n\n" +
				"`//`;\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertGeneratedAndReferenceParity(t, genLang, refLang, tt.src)
		})
	}
}
