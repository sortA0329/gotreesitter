package grammars

import (
	"fmt"
	"testing"

	"github.com/odvcencio/gotreesitter"
)

// TestJavaGLRDiag traces the GLR behavior for Java's obj.method() ambiguity.
func TestJavaGLRDiag(t *testing.T) {
	entry := DetectLanguage("Test.java")
	lang := entry.Language()
	parser := gotreesitter.NewParser(lang)

	// Find symbol IDs
	syms := map[string]gotreesitter.Symbol{}
	for i, name := range lang.SymbolNames {
		syms[name] = gotreesitter.Symbol(i)
	}

	dotSym := syms["."]
	identSym := syms["identifier"]
	lparenSym := syms["("]

	t.Logf("identifier=%d  dot=%d  lparen=%d", identSym, dotSym, lparenSym)
	t.Logf("StateCount=%d  TokenCount=%d", lang.StateCount, lang.TokenCount)

	// Find states with GLR conflicts for identifier
	identConflicts := 0
	for state := uint32(0); state < lang.StateCount; state++ {
		actions := lookupActionsGLR(lang, gotreesitter.StateID(state), identSym)
		if len(actions) > 1 {
			identConflicts++
			if identConflicts <= 20 {
				t.Logf("GLR conflict for 'identifier' at state %d: %d actions", state, len(actions))
				for _, a := range actions {
					t.Logf("  %s", descActionGLR(lang, a))
				}
			}
		}
	}
	t.Logf("Total states with GLR conflicts for 'identifier': %d", identConflicts)

	// Find states with GLR conflicts for '.'
	dotConflicts := 0
	for state := uint32(0); state < lang.StateCount; state++ {
		actions := lookupActionsGLR(lang, gotreesitter.StateID(state), dotSym)
		if len(actions) > 1 {
			dotConflicts++
			t.Logf("GLR conflict for '.' at state %d: %d actions", state, len(actions))
			for _, a := range actions {
				t.Logf("  %s", descActionGLR(lang, a))
			}
		}
	}
	t.Logf("Total states with GLR conflicts for '.': %d", dotConflicts)

	// Trace: after shifting identifier, which target states allow '.' and '('?
	t.Log("\n=== States reachable by shift(identifier) that allow '.' or '(' ===")
	for state := uint32(0); state < lang.StateCount; state++ {
		actions := lookupActionsGLR(lang, gotreesitter.StateID(state), identSym)
		for _, a := range actions {
			if a.Type == gotreesitter.ParseActionShift && !a.Extra {
				dotActions := lookupActionsGLR(lang, a.State, dotSym)
				lparenActions := lookupActionsGLR(lang, a.State, lparenSym)
				if len(dotActions) > 0 || len(lparenActions) > 0 {
					t.Logf("State %d --shift(ident)--> State %d:", state, a.State)
					for _, da := range dotActions {
						t.Logf("  '.' -> %s", descActionGLR(lang, da))
					}
					for _, la := range lparenActions {
						t.Logf("  '(' -> %s", descActionGLR(lang, la))
					}
				}
			}
		}
	}

	// Parse working vs broken
	t.Log("\n=== Parse results ===")
	cases := []struct {
		label string
		src   string
	}{
		{"simple call", `public class Foo { void f() { println("hello"); } }`},
		{"method on obj", `public class Foo { void f() { obj.method(); } }`},
		{"field access", `class A { int x = obj.field; }`},
		{"dotted type", `class A { java.lang.String x; }`},
		{"assign + call", `class A { void f() { int x = obj.method(); } }`},
		{"local var + call", `class A { void f() { Object x; x.method(); } }`},
	}
	for _, tc := range cases {
		tree, _ := parser.Parse([]byte(tc.src))
		root := tree.RootNode()
		t.Logf("%-25s hasError=%-5v  %s", tc.label, root.HasError(), root.SExpr(lang))
	}
	_ = parser
}

func lookupActionsGLR(lang *gotreesitter.Language, state gotreesitter.StateID, sym gotreesitter.Symbol) []gotreesitter.ParseAction {
	// Dense table lookup
	if int(state) < len(lang.ParseTable) {
		row := lang.ParseTable[state]
		if int(sym) < len(row) {
			idx := row[sym]
			if idx != 0 && int(idx) < len(lang.ParseActions) {
				return lang.ParseActions[idx].Actions
			}
		}
		return nil
	}
	// Small (sparse) table lookup
	smallIdx := int(state) - len(lang.ParseTable)
	if smallIdx < 0 || smallIdx >= len(lang.SmallParseTableMap) {
		return nil
	}
	offset := lang.SmallParseTableMap[smallIdx]
	tbl := lang.SmallParseTable
	if int(offset) >= len(tbl) {
		return nil
	}
	count := tbl[offset]
	pos := int(offset) + 1
	for g := uint16(0); g < count; g++ {
		if pos >= len(tbl) {
			break
		}
		nSyms := tbl[pos]
		pos++
		actionIdx := tbl[pos]
		pos++
		found := false
		for s := uint16(0); s < nSyms; s++ {
			if pos >= len(tbl) {
				break
			}
			if gotreesitter.Symbol(tbl[pos]) == sym {
				found = true
			}
			pos++
		}
		if found && actionIdx != 0 && int(actionIdx) < len(lang.ParseActions) {
			return lang.ParseActions[actionIdx].Actions
		}
	}
	return nil
}

func descActionGLR(lang *gotreesitter.Language, a gotreesitter.ParseAction) string {
	switch a.Type {
	case gotreesitter.ParseActionShift:
		extra := ""
		if a.Extra {
			extra = "(extra)"
		}
		return fmt.Sprintf("SHIFT→%d%s", a.State, extra)
	case gotreesitter.ParseActionReduce:
		symName := "?"
		if int(a.Symbol) < len(lang.SymbolNames) {
			symName = lang.SymbolNames[a.Symbol]
		}
		return fmt.Sprintf("REDUCE(%s/%d,cnt=%d,prec=%d,prod=%d)", symName, a.Symbol, a.ChildCount, a.DynamicPrecedence, a.ProductionID)
	case gotreesitter.ParseActionAccept:
		return "ACCEPT"
	default:
		return fmt.Sprintf("?type=%d", a.Type)
	}
}
