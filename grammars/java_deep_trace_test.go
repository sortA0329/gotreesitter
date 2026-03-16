package grammars

import (
	"fmt"
	"testing"

	"github.com/odvcencio/gotreesitter"
)

// TestJavaDeepTrace checks the exact parser state when inside a block
// to understand the GLR fork behavior.
func TestJavaDeepTrace(t *testing.T) {
	lang := DetectLanguage("Test.java").Language()
	parser := gotreesitter.NewParser(lang)

	identSym := findSymJ(lang, "identifier")
	dotSym := findSymJ(lang, ".")
	lparenSym := findSymJ(lang, "(")
	rparenSym := findSymJ(lang, ")")
	semiSym := findSymJ(lang, ";")

	// Check state 1356 + identifier (type path after shifting '.')
	t.Log("=== State 1356 + identifier ===")
	actions1356 := lookupActionsAll(lang, 1356, identSym)
	t.Logf("Actions: %d", len(actions1356))
	for _, a := range actions1356 {
		t.Logf("  %s", descActionJ(lang, a))
	}

	// Check state 953 + identifier (direct shift path)
	t.Log("\n=== State 953 + identifier ===")
	actions953 := lookupActionsAll(lang, 953, identSym)
	t.Logf("Actions: %d", len(actions953))
	for _, a := range actions953 {
		t.Logf("  %s", descActionJ(lang, a))
	}

	// Check state 704 (after 953+identifier shift)
	t.Log("\n=== State 704 ===")
	for _, sym := range []struct {
		name string
		sym  gotreesitter.Symbol
	}{
		{"identifier", identSym},
		{".", dotSym},
		{"(", lparenSym},
		{")", rparenSym},
		{";", semiSym},
	} {
		actions := lookupActionsAll(lang, 704, sym.sym)
		if len(actions) > 0 {
			t.Logf("State 704 + '%s': %d actions", sym.name, len(actions))
			for _, a := range actions {
				t.Logf("  %s", descActionJ(lang, a))
			}
		}
	}

	// Now the critical question: what state is the parser ACTUALLY in
	// when it hits 'obj'? Let me trace by parsing the prefix tokens.
	// The prefix is: public class Foo { void f() {
	t.Log("\n=== Simulating token-by-token through prefix ===")

	// Parse a working version to see what state we'd be in
	// We can check: which parent state is active by looking at
	// the token before 'obj'
	//
	// Actually, let me check ALL states that have shift for '{'
	// and then chain to states that have shift for identifier→423
	t.Log("\n=== Looking for block-entry states that shift '{' ===")
	lbraceSym := findSymJ(lang, "{")

	// States that shift '{' and then have identifier shift→423
	for state := uint32(0); state < lang.StateCount; state++ {
		braceActions := lookupActionsAll(lang, gotreesitter.StateID(state), lbraceSym)
		for _, ba := range braceActions {
			if ba.Type == gotreesitter.ParseActionShift && !ba.Extra {
				// In the post-brace state, check for identifier→423
				identActs := lookupActionsAll(lang, ba.State, identSym)
				for _, ia := range identActs {
					if ia.Type == gotreesitter.ParseActionShift && !ia.Extra && ia.State == 423 {
						// This state leads to '{' which leads to identifier→423
						// Check GOTO for primary_expression from the brace state
						primaryExprSym := findSymJ(lang, "primary_expression")
						unannotatedTypeSym := findSymJ(lang, "_unannotated_type")

						peGoto := lookupGotoTest(parser, lang, ba.State, primaryExprSym)
						utGoto := lookupGotoTest(parser, lang, ba.State, unannotatedTypeSym)

						if peGoto != 0 || utGoto != 0 {
							t.Logf("State %d --shift({)--> State %d --shift(ident)--> 423", state, ba.State)
							t.Logf("  GOTO(primary_expression) → %d", peGoto)
							t.Logf("  GOTO(_unannotated_type) → %d", utGoto)

							// But wait: the identifier shift to 423 means
							// the PARENT state for the reduce is ba.State, not state
							// Let me check what reduce does from ba.State

							if peGoto != 0 {
								dotActs := lookupActionsAll(lang, peGoto, dotSym)
								t.Logf("  primary_expression(%d)+'.': %d actions", peGoto, len(dotActs))
								for _, da := range dotActs {
									t.Logf("    %s", descActionJ(lang, da))
								}
							}
						}
					}
				}
			}
		}
	}

	// Let me also check: what's the actual state of the parser
	// right before 'obj'? This would be the state inside the block.
	// After '{' is shifted, we're in the "block" state, which typically
	// allows statements.

	// Actually, the simplest diagnostic: parse with DFA token source
	// (the default) and see what token sequence it produces with
	// state tracking.
	t.Log("\n=== Parse: checking with DFA vs custom token source ===")

	// Parse with standard DFA (default for unknown languages)
	// vs custom Java token source
	src := []byte(`public class Foo { void f() { obj.method(); } }`)

	// Try parsing WITHOUT the custom token source
	tree1, _ := parser.Parse(src)
	t.Logf("Standard parse: hasError=%v %s", tree1.RootNode().HasError(), tree1.RootNode().SExpr(lang))

	// Try parsing with the custom Java token source
	ts, err := NewJavaTokenSource(src, lang)
	if err != nil {
		t.Fatalf("NewJavaTokenSource: %v", err)
	}
	tree2, err := parser.ParseWithTokenSource(src, ts)
	if err != nil {
		t.Fatalf("ParseWithTokenSource: %v", err)
	}
	t.Logf("Java TS parse: hasError=%v %s", tree2.RootNode().HasError(), tree2.RootNode().SExpr(lang))

	_ = fmt.Sprintf("done")
}
