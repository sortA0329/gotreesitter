package grammars

import (
	"testing"

	ts "github.com/odvcencio/gotreesitter"
)

func TestDartLibraryDirectiveRecoversMissingName(t *testing.T) {
	src := []byte("library;\n")
	parser := ts.NewParser(DartLanguage())
	tree, err := parser.Parse(src)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	root := tree.RootNode()
	if root == nil {
		t.Fatal("missing root node")
	}
	if tree.ParseStopReason() != ts.ParseStopAccepted {
		t.Fatalf("stop=%s runtime=%s", tree.ParseStopReason(), tree.ParseRuntime().Summary())
	}
	if !root.HasError() {
		t.Fatalf("expected missing-name recovery to retain error flag, got %s", root.SExpr(DartLanguage()))
	}
	if got := root.NamedChildCount(); got != 1 {
		t.Fatalf("named child count = %d, want 1; tree=%s", got, root.SExpr(DartLanguage()))
	}
	libraryName := root.NamedChild(0)
	if libraryName == nil || libraryName.Type(DartLanguage()) != "library_name" {
		t.Fatalf("first named child = %v, want library_name; tree=%s", libraryName, root.SExpr(DartLanguage()))
	}
	if got := libraryName.NamedChildCount(); got != 1 {
		t.Fatalf("library_name named child count = %d, want 1", got)
	}
	dotted := libraryName.NamedChild(0)
	if dotted == nil || dotted.Type(DartLanguage()) != "dotted_identifier_list" {
		t.Fatalf("library_name named child = %v, want dotted_identifier_list", dotted)
	}
	if got := dotted.NamedChildCount(); got != 1 {
		t.Fatalf("dotted_identifier_list named child count = %d, want 1", got)
	}
	ident := dotted.NamedChild(0)
	if ident == nil || ident.Type(DartLanguage()) != "identifier" {
		t.Fatalf("dotted_identifier_list named child = %v, want identifier", ident)
	}
	if !ident.IsMissing() {
		t.Fatalf("identifier should be missing; tree=%s", root.SExpr(DartLanguage()))
	}
}
