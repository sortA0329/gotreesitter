// emit_grammargen_go_blob rebuilds grammars/grammar_blobs/go.bin using our
// own grammargen (pure-Go LALR(1) + LR(1) state splitting) rather than the
// ts2go pipeline. grammargen's table compilation produces a different
// state/symbol layout that happens to sidestep a dead-end state in
// tree-sitter-go's C tables where `}` has no action after certain nested
// switch/case/if patterns (triggers an ERROR root on some valid Go files,
// e.g. parser_reduce.go).
//
// Usage:
//
//	go run ./cmd/emit_grammargen_go_blob -o grammars/grammar_blobs/go.bin
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/odvcencio/gotreesitter/grammargen"
)

func main() {
	out := flag.String("o", "grammars/grammar_blobs/go.bin", "output blob path")
	flag.Parse()

	start := time.Now()
	g := grammargen.GoGrammar()
	g.EnableLRSplitting = true
	lang, blob, err := grammargen.GenerateLanguageAndBlob(g)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if werr := os.WriteFile(*out, blob, 0o644); werr != nil {
		fmt.Fprintf(os.Stderr, "write %s: %v\n", *out, werr)
		os.Exit(1)
	}
	fmt.Printf("wrote %s (%d bytes, %d states, %d symbols, elapsed %v)\n",
		*out, len(blob), lang.StateCount, len(lang.SymbolNames), time.Since(start))
}
