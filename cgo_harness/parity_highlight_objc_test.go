//go:build cgo && treesitter_c_parity

package cgoharness

import (
	"testing"

	sitter "github.com/tree-sitter/go-tree-sitter"
)

func TestHighlightObjcInterfacePropertyParity(t *testing.T) {
	const langName = "objc"
	const query = `((identifier) @property
  (#has-ancestor? @property struct_declaration class_interface class_implementation))`

	var tc parityCase
	found := false
	for _, c := range parityCases {
		if c.name == langName {
			tc = c
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("missing parity case for %q", langName)
	}

	src := normalizedSource(tc.name, tc.source)
	goTree, goLang, err := parseWithGo(tc, src, nil)
	if err != nil {
		t.Fatalf("Go parse error: %v", err)
	}
	defer releaseGoTree(goTree)

	cLang, err := ParityCLanguage(langName)
	if err != nil {
		t.Fatalf("load C parser: %v", err)
	}
	cParser := sitter.NewParser()
	defer cParser.Close()
	if err := cParser.SetLanguage(cLang); err != nil {
		t.Fatalf("C SetLanguage: %v", err)
	}
	cTree := cParser.Parse(src, nil)
	if cTree == nil {
		t.Fatal("C parser returned nil tree")
	}
	defer cTree.Close()

	goCaps := collectGoHighlightCaptures(t, goLang, goTree, query, src)
	cCaps := collectCHighlightCaptures(t, cLang, cTree, query, src)
	onlyGo, onlyC := diffCaptures(goCaps, cCaps)
	if len(onlyGo) > 0 || len(onlyC) > 0 {
		t.Fatalf("objc interface property captures mismatch: onlyGo=%v onlyC=%v", onlyGo, onlyC)
	}
}
