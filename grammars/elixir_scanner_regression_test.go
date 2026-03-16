package grammars

import (
	"testing"

	gotreesitter "github.com/odvcencio/gotreesitter"
)

func TestElixirBitstringAfterBlankLineRegression(t *testing.T) {
	lang := ElixirLanguage()
	parser := gotreesitter.NewParser(lang)

	src := []byte("<<1, 2, 3>>\n\n<< header :: size(8), data :: binary >>\n")
	tree, err := parser.Parse(src)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	root := tree.RootNode()
	if root == nil || root.HasError() {
		t.Fatalf("unexpected error tree: %s", root.SExpr(lang))
	}
	if got, want := root.NamedChildCount(), 2; got != want {
		t.Fatalf("root named child count = %d, want %d: %s", got, want, root.SExpr(lang))
	}
	if got := root.NamedChild(0).Type(lang); got != "bitstring" {
		t.Fatalf("first named child type = %q, want bitstring: %s", got, root.SExpr(lang))
	}
	if got := root.NamedChild(1).Type(lang); got != "bitstring" {
		t.Fatalf("second named child type = %q, want bitstring: %s", got, root.SExpr(lang))
	}
}

func TestElixirMultipleModuledocBeforeHeredocRegression(t *testing.T) {
	lang := ElixirLanguage()
	parser := gotreesitter.NewParser(lang)

	src := []byte("defmodule M do\n  @moduledoc \"Simple doc\"\n\n  @moduledoc false\n\n  @moduledoc \"\"\"\n  Heredoc doc\n  \"\"\"\nend\n")
	tree, err := parser.Parse(src)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	root := tree.RootNode()
	if root == nil || root.HasError() {
		t.Fatalf("unexpected error tree: %s", root.SExpr(lang))
	}
	if got, want := root.NamedChildCount(), 1; got != want {
		t.Fatalf("root named child count = %d, want %d: %s", got, want, root.SExpr(lang))
	}
	if got := root.NamedChild(0).Type(lang); got != "call" {
		t.Fatalf("root child type = %q, want call: %s", got, root.SExpr(lang))
	}
}

func TestElixirNestedCallTargetFieldRegression(t *testing.T) {
	lang := ElixirLanguage()
	parser := gotreesitter.NewParser(lang)

	src := []byte("def unquote(f)(x), do: nil\n")
	tree, err := parser.Parse(src)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	root := tree.RootNode()
	if root == nil || root.HasError() {
		t.Fatalf("unexpected error tree: %s", root.SExpr(lang))
	}
	defCall := root.NamedChild(0)
	if defCall == nil || defCall.Type(lang) != "call" {
		t.Fatalf("root child type = %q, want call: %s", defCall.Type(lang), root.SExpr(lang))
	}
	args := defCall.Child(1)
	if args == nil || args.Type(lang) != "arguments" {
		t.Fatalf("def args type = %q, want arguments: %s", args.Type(lang), root.SExpr(lang))
	}
	nested := args.NamedChild(0)
	if nested == nil || nested.Type(lang) != "call" {
		t.Fatalf("nested child type = %q, want call: %s", nested.Type(lang), root.SExpr(lang))
	}
	if got := nested.FieldNameForChild(0, lang); got != "target" {
		t.Fatalf("nested call child field = %q, want target: %s", got, root.SExpr(lang))
	}
}
