package grammars

import (
	"strings"
	"testing"

	"github.com/odvcencio/gotreesitter"
)

func findFirstNamedDescendant(node *gotreesitter.Node, lang *gotreesitter.Language, typ string) *gotreesitter.Node {
	if node == nil {
		return nil
	}
	if node.IsNamed() && node.Type(lang) == typ {
		return node
	}
	for i := 0; i < node.NamedChildCount(); i++ {
		if found := findFirstNamedDescendant(node.NamedChild(i), lang, typ); found != nil {
			return found
		}
	}
	return nil
}

func assertMainStringArrayShape(t *testing.T, tree *gotreesitter.Tree, lang *gotreesitter.Language, src []byte) {
	t.Helper()

	if tree == nil || tree.RootNode() == nil {
		t.Fatal("parse returned nil root")
	}

	root := tree.RootNode()
	if root.HasError() {
		t.Fatalf("expected parse without syntax errors, got sexpr: %s", root.SExpr(lang))
	}
	if root.NamedChildCount() != 2 {
		t.Fatalf("expected root to have 2 named children, got %d: %s", root.NamedChildCount(), root.SExpr(lang))
	}
	if got := root.NamedChild(0).Type(lang); got != "package_declaration" {
		t.Fatalf("root child[0] = %q, want package_declaration", got)
	}
	if got := root.NamedChild(1).Type(lang); got != "class_declaration" {
		t.Fatalf("root child[1] = %q, want class_declaration", got)
	}

	methodDecl := findFirstNamedDescendant(root, lang, "method_declaration")
	if methodDecl == nil {
		t.Fatalf("no method_declaration in parse tree: %s", root.SExpr(lang))
	}
	nameNode := methodDecl.ChildByFieldName("name", lang)
	if nameNode == nil || nameNode.Text(src) != "main" {
		got := "<nil>"
		if nameNode != nil {
			got = nameNode.Text(src)
		}
		t.Fatalf("method name = %q, want %q", got, "main")
	}

	params := findFirstNamedDescendant(methodDecl, lang, "formal_parameters")
	if params == nil {
		t.Fatalf("method_declaration missing formal_parameters: %s", methodDecl.SExpr(lang))
	}
	paramText := strings.Join(strings.Fields(params.Text(src)), "")
	if !strings.Contains(paramText, "String[]args") {
		t.Fatalf("formal_parameters = %q, want to contain String[]args", params.Text(src))
	}

	invocation := findFirstNamedDescendant(methodDecl, lang, "method_invocation")
	if invocation == nil {
		t.Fatalf("method_declaration missing method_invocation: %s", methodDecl.SExpr(lang))
	}
	if !strings.Contains(invocation.Text(src), "System.out.println") {
		t.Fatalf("method_invocation text = %q, want to contain System.out.println", invocation.Text(src))
	}
}

func TestJavaParseMainStringArrayRegression(t *testing.T) {
	lang := JavaLanguage()
	parser := gotreesitter.NewParser(lang)

	src := []byte(`package com.example;

public class App {
    public static void main(String[] args) {
        System.out.println("hello");
    }
}
`)

	tree, err := parser.Parse(src)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	assertMainStringArrayShape(t, tree, lang, src)
}

func TestJavaParseWithTokenSourceMainStringArrayRegression(t *testing.T) {
	lang := JavaLanguage()
	parser := gotreesitter.NewParser(lang)

	src := []byte(`package com.example;

public class App {
    public static void main(String[] args) {
        System.out.println("hello");
    }
}
`)

	ts, err := NewJavaTokenSource(src, lang)
	if err != nil {
		t.Fatalf("NewJavaTokenSource failed: %v", err)
	}
	tree, err := parser.ParseWithTokenSource(src, ts)
	if err != nil {
		t.Fatalf("parse with token source failed: %v", err)
	}
	assertMainStringArrayShape(t, tree, lang, src)
}
