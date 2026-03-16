package grammars

import (
	"testing"

	ts "github.com/odvcencio/gotreesitter"
)

func TestPHPMixedGroupedUseRetainsNamespaceUseDeclaration(t *testing.T) {
	src := []byte("<?php\nuse Foo\\Baz\\{\n  Bar as Barr,\n  function foo as fooo,\n  const FOO as FOOO,\n};\n")
	parser := ts.NewParser(PhpLanguage())
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
	if got := root.EndByte(); got != uint32(len(src)) {
		t.Fatalf("root end = %d, want %d; tree=%s", got, len(src), root.SExpr(PhpLanguage()))
	}
	if got := root.ChildCount(); got != 2 {
		t.Fatalf("root child count = %d, want 2; tree=%s", got, root.SExpr(PhpLanguage()))
	}
	if decl := root.Child(1); decl == nil || decl.Type(PhpLanguage()) != "namespace_use_declaration" {
		t.Fatalf("second child = %v, want namespace_use_declaration; tree=%s", decl, root.SExpr(PhpLanguage()))
	} else if !decl.HasError() {
		t.Fatalf("grouped use should retain error flag for trailing comma recovery; tree=%s", root.SExpr(PhpLanguage()))
	}
}

func TestPHPGroupedUseRecoveryPreservesFollowingFunction(t *testing.T) {
	src := []byte("<?php\nnamespace A;\n\nuse Foo\\Baz as Baaz;\n\nuse Foo\\Baz\\{\n  const FOO,\n};\n\nfunction a() {}\n")
	parser := ts.NewParser(PhpLanguage())
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
	if got := root.EndByte(); got != uint32(len(src)) {
		t.Fatalf("root end = %d, want %d; tree=%s", got, len(src), root.SExpr(PhpLanguage()))
	}
	if got := root.ChildCount(); got != 5 {
		t.Fatalf("root child count = %d, want 5; tree=%s", got, root.SExpr(PhpLanguage()))
	}
	if fn := root.Child(4); fn == nil || fn.Type(PhpLanguage()) != "function_definition" {
		t.Fatalf("last child = %v, want function_definition; tree=%s", fn, root.SExpr(PhpLanguage()))
	}
}

func TestPHPTopLevelStaticAnonymousFunctionRecovery(t *testing.T) {
	src := []byte("<?php\nstatic function () {}\n")
	parser := ts.NewParser(PhpLanguage())
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
	if got := root.EndByte(); got != uint32(len(src)) {
		t.Fatalf("root end = %d, want %d; tree=%s", got, len(src), root.SExpr(PhpLanguage()))
	}
	if got := root.ChildCount(); got != 2 {
		t.Fatalf("root child count = %d, want 2; tree=%s", got, root.SExpr(PhpLanguage()))
	}
	stmt := root.Child(1)
	if stmt == nil || stmt.Type(PhpLanguage()) != "expression_statement" {
		t.Fatalf("second child = %v, want expression_statement; tree=%s", stmt, root.SExpr(PhpLanguage()))
	}
	if got := stmt.ChildCount(); got != 2 {
		t.Fatalf("expression_statement child count = %d, want 2; tree=%s", got, root.SExpr(PhpLanguage()))
	}
	if fn := stmt.Child(0); fn == nil || fn.Type(PhpLanguage()) != "anonymous_function" {
		t.Fatalf("first expression child = %v, want anonymous_function; tree=%s", fn, root.SExpr(PhpLanguage()))
	}
	if semi := stmt.Child(1); semi == nil || semi.Type(PhpLanguage()) != ";" || !semi.HasError() {
		t.Fatalf("second expression child = %v, want missing semicolon; tree=%s", semi, root.SExpr(PhpLanguage()))
	}
}

func TestPHPTopLevelStaticNamedFunctionRecovery(t *testing.T) {
	src := []byte("<?php\nstatic function a() {}\n")
	parser := ts.NewParser(PhpLanguage())
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
	if got := root.EndByte(); got != uint32(len(src)) {
		t.Fatalf("root end = %d, want %d; tree=%s", got, len(src), root.SExpr(PhpLanguage()))
	}
	if got := root.ChildCount(); got != 3 {
		t.Fatalf("root child count = %d, want 3; tree=%s", got, root.SExpr(PhpLanguage()))
	}
	if errNode := root.Child(1); errNode == nil || errNode.Type(PhpLanguage()) != "ERROR" {
		t.Fatalf("second child = %v, want ERROR; tree=%s", errNode, root.SExpr(PhpLanguage()))
	}
	if body := root.Child(2); body == nil || body.Type(PhpLanguage()) != "compound_statement" {
		t.Fatalf("third child = %v, want compound_statement; tree=%s", body, root.SExpr(PhpLanguage()))
	}
}

func TestPHPContextStaticNamedFunctionRecovery(t *testing.T) {
	src := []byte("<?php\nfunction a() {}\n// <- @keyword\n\nstatic function a() {}\n")
	parser := ts.NewParser(PhpLanguage())
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
	if got := root.EndByte(); got != uint32(len(src)) {
		t.Fatalf("root end = %d, want %d; tree=%s", got, len(src), root.SExpr(PhpLanguage()))
	}
	if got := root.ChildCount(); got != 6 {
		t.Fatalf("root child count = %d, want 6; tree=%s", got, root.SExpr(PhpLanguage()))
	}
	if errNode := root.Child(3); errNode == nil || errNode.Type(PhpLanguage()) != "ERROR" {
		t.Fatalf("child[3] = %v, want ERROR; tree=%s", errNode, root.SExpr(PhpLanguage()))
	}
	if stmt := root.Child(4); stmt == nil || stmt.Type(PhpLanguage()) != "expression_statement" {
		t.Fatalf("child[4] = %v, want expression_statement; tree=%s", stmt, root.SExpr(PhpLanguage()))
	}
	if body := root.Child(5); body == nil || body.Type(PhpLanguage()) != "compound_statement" {
		t.Fatalf("child[5] = %v, want compound_statement; tree=%s", body, root.SExpr(PhpLanguage()))
	}
}

func TestPHPTopLevelStaticNamedFunctionFollowedByArrowAndClassRecovery(t *testing.T) {
	src := []byte("<?php\nstatic function a() {}\nstatic fn () => 1;\nabstract class A {}\n")
	parser := ts.NewParser(PhpLanguage())
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
	if got := root.EndByte(); got != uint32(len(src)) {
		t.Fatalf("root end = %d, want %d; tree=%s", got, len(src), root.SExpr(PhpLanguage()))
	}
	if got := root.ChildCount(); got != 5 {
		t.Fatalf("root child count = %d, want 5; tree=%s", got, root.SExpr(PhpLanguage()))
	}
	if errNode := root.Child(1); errNode == nil || errNode.Type(PhpLanguage()) != "ERROR" {
		t.Fatalf("child[1] = %v, want ERROR; tree=%s", errNode, root.SExpr(PhpLanguage()))
	}
	if body := root.Child(2); body == nil || body.Type(PhpLanguage()) != "compound_statement" {
		t.Fatalf("child[2] = %v, want compound_statement; tree=%s", body, root.SExpr(PhpLanguage()))
	}
	if stmt := root.Child(3); stmt == nil || stmt.Type(PhpLanguage()) != "expression_statement" {
		t.Fatalf("child[3] = %v, want expression_statement; tree=%s", stmt, root.SExpr(PhpLanguage()))
	}
	if decl := root.Child(4); decl == nil || decl.Type(PhpLanguage()) != "class_declaration" {
		t.Fatalf("child[4] = %v, want class_declaration; tree=%s", decl, root.SExpr(PhpLanguage()))
	}
}
