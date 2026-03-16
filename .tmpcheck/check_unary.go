package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammargen"
	"github.com/odvcencio/gotreesitter/grammars"
)

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

func dumpNode(n *gotreesitter.Node, lang *gotreesitter.Language, src []byte, depth int) {
	if n == nil {
		return
	}
	t := n.Type(lang)
	text := strings.TrimSpace(string(src[n.StartByte():n.EndByte()]))
	fmt.Printf("%s%q [%d:%d] text=%q children=%d\n", strings.Repeat("  ", depth), t, n.StartByte(), n.EndByte(), text, n.ChildCount())
	for i := 0; i < n.ChildCount(); i++ {
		dumpNode(n.Child(i), lang, src, depth+1)
	}
}

func inspect(label string, lang *gotreesitter.Language, root *gotreesitter.Node, src []byte) {
	fmt.Println("===", label)
	for i := 0; i < root.ChildCount(); i++ {
		dumpNode(root.Child(i), lang, src, 0)
	}
}

func main() {
	src := []byte("package p\n\nfunc f(r rune) { _ = !IsLetter(r) }\n")
	p := gotreesitter.NewParser(grammars.GoLanguage())
	goRoot := must(p.Parse(src)).RootNode()
	inspect("builtin", grammars.GoLanguage(), goRoot, src)

	path := "/tmp/grammar_parity/go/src/grammar.json"
	if _, err := os.Stat(path); err != nil {
		fmt.Println("grammar json missing")
		return
	}
	g := must(grammargen.ImportGrammarJSON(must(os.ReadFile(path))))
	gen := must(grammargen.GenerateLanguage(g))
	groot := must(gotreesitter.NewParser(gen).Parse(src)).RootNode()
	inspect("generated", gen, groot, src)
}
