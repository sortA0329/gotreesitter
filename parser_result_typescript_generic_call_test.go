package gotreesitter_test

import (
	"testing"

	gotreesitter "github.com/odvcencio/gotreesitter"
)

func TestTSXPredefinedGenericCallParsesAsCallExpression(t *testing.T) {
	const src = "const [inputValue, setInputValue] = useState<string>(values[0].toString())\n"
	tree, lang := parseByLanguageName(t, "tsx", src)
	root := tree.RootNode()
	if root.HasError() {
		t.Fatalf("unexpected tsx parse error: %s", root.SExpr(lang))
	}

	call := firstNode(root, func(n *gotreesitter.Node) bool {
		return n.Type(lang) == "call_expression" && n.Text([]byte(src)) == "useState<string>(values[0].toString())"
	})
	if call == nil {
		t.Fatalf("tsx generic call missing call_expression: %s", root.SExpr(lang))
	}

	typeArgs := firstNode(call, func(n *gotreesitter.Node) bool {
		return n.Type(lang) == "type_arguments"
	})
	if typeArgs == nil {
		t.Fatalf("tsx generic call missing type_arguments: %s", call.SExpr(lang))
	}
	predefined := firstNode(typeArgs, func(n *gotreesitter.Node) bool {
		return n.Type(lang) == "predefined_type"
	})
	if predefined == nil || predefined.Text([]byte(src)) != "string" {
		t.Fatalf("tsx generic call missing predefined string type: %s", call.SExpr(lang))
	}
	if bad := firstNode(root, func(n *gotreesitter.Node) bool {
		return n.Type(lang) == "binary_expression" && n.Text([]byte(src)) == "useState<string>(values[0].toString())"
	}); bad != nil {
		t.Fatalf("tsx generic call still parsed as binary_expression: %s", bad.SExpr(lang))
	}
}

func TestTSXCustomGenericCallStillParsesAsCallExpression(t *testing.T) {
	const src = "const x = foo<Bar>(baz)\n"
	tree, lang := parseByLanguageName(t, "tsx", src)
	root := tree.RootNode()
	if root.HasError() {
		t.Fatalf("unexpected tsx parse error: %s", root.SExpr(lang))
	}
	call := firstNode(root, func(n *gotreesitter.Node) bool {
		return n.Type(lang) == "call_expression" && n.Text([]byte(src)) == "foo<Bar>(baz)"
	})
	if call == nil {
		t.Fatalf("tsx custom generic call missing call_expression: %s", root.SExpr(lang))
	}
	typeArgs := firstNode(call, func(n *gotreesitter.Node) bool {
		return n.Type(lang) == "type_arguments"
	})
	if typeArgs == nil {
		t.Fatalf("tsx custom generic call missing type_arguments: %s", call.SExpr(lang))
	}
	typeIdent := firstNode(typeArgs, func(n *gotreesitter.Node) bool {
		return n.Type(lang) == "type_identifier"
	})
	if typeIdent == nil || typeIdent.Text([]byte(src)) != "Bar" {
		t.Fatalf("tsx custom generic call lost type_identifier: %s", call.SExpr(lang))
	}
}

func TestTypeScriptCustomGenericCallStillParsesAsCallExpression(t *testing.T) {
	const src = "const x = createMissingNode<Identifier>(token(), reportAtCurrentPosition, diagnosticMessage)\n"
	tree, lang := parseByLanguageName(t, "typescript", src)
	root := tree.RootNode()
	if root.HasError() {
		t.Fatalf("unexpected typescript parse error: %s", root.SExpr(lang))
	}

	call := firstNode(root, func(n *gotreesitter.Node) bool {
		return n.Type(lang) == "call_expression" && n.Text([]byte(src)) == "createMissingNode<Identifier>(token(), reportAtCurrentPosition, diagnosticMessage)"
	})
	if call == nil {
		t.Fatalf("typescript custom generic call missing call_expression: %s", root.SExpr(lang))
	}
	typeArgs := firstNode(call, func(n *gotreesitter.Node) bool {
		return n.Type(lang) == "type_arguments"
	})
	if typeArgs == nil {
		t.Fatalf("typescript custom generic call missing type_arguments: %s", call.SExpr(lang))
	}
	typeIdent := firstNode(typeArgs, func(n *gotreesitter.Node) bool {
		return n.Type(lang) == "type_identifier"
	})
	if typeIdent == nil || typeIdent.Text([]byte(src)) != "Identifier" {
		t.Fatalf("typescript custom generic call lost type_identifier: %s", call.SExpr(lang))
	}
	args := firstNode(call, func(n *gotreesitter.Node) bool {
		return n.Type(lang) == "arguments"
	})
	if args == nil || args.ChildCount() != 7 {
		t.Fatalf("typescript custom generic call arguments child count = %d, want 7: %s", args.ChildCount(), call.SExpr(lang))
	}
}

func TestTSXNestedGenericCallStillParsesAsCallExpression(t *testing.T) {
	const src = "const [inputRef, setInputRef] = useState<React.RefObject<HTMLInputElement>>()\n"
	tree, lang := parseByLanguageName(t, "tsx", src)
	root := tree.RootNode()
	if root.HasError() {
		t.Fatalf("unexpected tsx parse error: %s", root.SExpr(lang))
	}

	call := firstNode(root, func(n *gotreesitter.Node) bool {
		return n.Type(lang) == "call_expression" && n.Text([]byte(src)) == "useState<React.RefObject<HTMLInputElement>>()"
	})
	if call == nil {
		t.Fatalf("tsx nested generic call missing call_expression: %s", root.SExpr(lang))
	}
	typeArgs := firstNode(call, func(n *gotreesitter.Node) bool {
		return n.Type(lang) == "type_arguments"
	})
	if typeArgs == nil {
		t.Fatalf("tsx nested generic call missing type_arguments: %s", call.SExpr(lang))
	}
	if got := countNodes(call, func(n *gotreesitter.Node) bool {
		return n.Type(lang) == "type_arguments"
	}); got < 2 {
		t.Fatalf("tsx nested generic call type_arguments count = %d, want at least 2: %s", got, call.SExpr(lang))
	}
}

func TestTSXShiftExpressionStillUsesRightShiftToken(t *testing.T) {
	const src = "const x = foo >> bar\n"
	tree, lang := parseByLanguageName(t, "tsx", src)
	root := tree.RootNode()
	if root.HasError() {
		t.Fatalf("unexpected tsx parse error: %s", root.SExpr(lang))
	}
	shift := firstNode(root, func(n *gotreesitter.Node) bool {
		return n.Type(lang) == "binary_expression" && n.Text([]byte(src)) == "foo >> bar"
	})
	if shift == nil {
		t.Fatalf("tsx shift expression missing binary_expression: %s", root.SExpr(lang))
	}
}

func TestTSXEnumAssignmentsDoNotInheritNameFieldFromEnumBody(t *testing.T) {
	const src = "export enum MaterialsInputType {\n  ELEMENTS = 'elements',\n}\n"
	tree, lang := parseByLanguageName(t, "tsx", src)
	root := tree.RootNode()
	if root.HasError() {
		t.Fatalf("unexpected tsx parse error: %s", root.SExpr(lang))
	}
	enumBody := firstNode(root, func(n *gotreesitter.Node) bool {
		return n.Type(lang) == "enum_body"
	})
	if enumBody == nil {
		t.Fatalf("missing enum_body: %s", root.SExpr(lang))
	}
	assignment := firstNode(enumBody, func(n *gotreesitter.Node) bool {
		return n.Type(lang) == "enum_assignment"
	})
	if assignment == nil {
		t.Fatalf("missing enum_assignment: %s", enumBody.SExpr(lang))
	}
	foundIdx := -1
	for i := 0; i < enumBody.ChildCount(); i++ {
		if enumBody.Child(i) == assignment {
			foundIdx = i
			break
		}
	}
	if foundIdx < 0 {
		t.Fatalf("enum_assignment not found as direct enum_body child: %s", enumBody.SExpr(lang))
	}
	if got := enumBody.FieldNameForChild(foundIdx, lang); got != "" {
		t.Fatalf("enum_body.FieldNameForChild(%d) = %q, want empty", foundIdx, got)
	}
}

func firstNode(root *gotreesitter.Node, pred func(*gotreesitter.Node) bool) *gotreesitter.Node {
	if root == nil {
		return nil
	}
	if pred(root) {
		return root
	}
	for i := 0; i < root.ChildCount(); i++ {
		child := root.Child(i)
		if found := firstNode(child, pred); found != nil {
			return found
		}
	}
	return nil
}

func countNodes(root *gotreesitter.Node, pred func(*gotreesitter.Node) bool) int {
	if root == nil {
		return 0
	}
	total := 0
	if pred(root) {
		total++
	}
	for i := 0; i < root.ChildCount(); i++ {
		total += countNodes(root.Child(i), pred)
	}
	return total
}
