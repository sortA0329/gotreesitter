package grammars

import (
	"fmt"
	"testing"

	"github.com/odvcencio/gotreesitter"
)

func findSymJ(lang *gotreesitter.Language, name string) gotreesitter.Symbol {
	for i, n := range lang.SymbolNames {
		if n == name {
			return gotreesitter.Symbol(i)
		}
	}
	return 0
}

func descActionsJ(lang *gotreesitter.Language, actions []gotreesitter.ParseAction) string {
	if len(actions) == 0 {
		return "(none)"
	}
	result := ""
	for i, a := range actions {
		if i > 0 {
			result += " | "
		}
		switch a.Type {
		case gotreesitter.ParseActionShift:
			extra := ""
			if a.Extra {
				extra = "(extra)"
			}
			result += fmt.Sprintf("SHIFT→%d%s", a.State, extra)
		case gotreesitter.ParseActionReduce:
			symName := "?"
			if int(a.Symbol) < len(lang.SymbolNames) {
				symName = lang.SymbolNames[a.Symbol]
			}
			result += fmt.Sprintf("REDUCE(%s/%d,cnt=%d,prec=%d)", symName, a.Symbol, a.ChildCount, a.DynamicPrecedence)
		case gotreesitter.ParseActionAccept:
			result += "ACCEPT"
		case gotreesitter.ParseActionRecover:
			result += fmt.Sprintf("RECOVER→%d", a.State)
		default:
			result += fmt.Sprintf("?type=%d", a.Type)
		}
	}
	return result
}

// TestJavaGLRGotoTrace traces the GOTO targets after reduces at the GLR
// conflict point (state 423 + '.') to understand which path survives.
func TestJavaGLRGotoTrace(t *testing.T) {
	lang := DetectLanguage("Test.java").Language()
	parser := gotreesitter.NewParser(lang)

	dotSym := findSymJ(lang, ".")
	identSym := findSymJ(lang, "identifier")

	// Find all parent states that SHIFT identifier → 423
	t.Log("=== Parent states that shift identifier → 423 ===")
	parentStates := []gotreesitter.StateID{}
	for state := uint32(0); state < lang.StateCount; state++ {
		actions := lookupActionsAll(lang, gotreesitter.StateID(state), identSym)
		for _, a := range actions {
			if a.Type == gotreesitter.ParseActionShift && !a.Extra && a.State == 423 {
				parentStates = append(parentStates, gotreesitter.StateID(state))
				break
			}
		}
	}
	t.Logf("Found %d parent states", len(parentStates))

	// For each parent state, trace what happens when we:
	// 1. REDUCE(primary_expression, cnt=1) → GOTO(parent, primary_expression)
	// 2. REDUCE(_unannotated_type, cnt=1) → GOTO(parent, _unannotated_type)
	// Then check if the goto state has an action for '.'
	primaryExprSym := findSymJ(lang, "primary_expression")
	unannotatedTypeSym := findSymJ(lang, "_unannotated_type")
	t.Logf("primary_expression=%d  _unannotated_type=%d", primaryExprSym, unannotatedTypeSym)

	for _, parentState := range parentStates {
		t.Logf("\nParent state %d:", parentState)

		// Path 1: REDUCE(primary_expression) → GOTO
		peGoto := lookupGotoTest(parser, lang, parentState, primaryExprSym)
		if peGoto != 0 {
			peActions := lookupActionsAll(lang, peGoto, dotSym)
			t.Logf("  primary_expression GOTO → state %d", peGoto)
			if len(peActions) > 0 {
				for _, a := range peActions {
					t.Logf("    '.' → %s", descActionJ(lang, a))
				}
			} else {
				t.Logf("    '.' → NO ACTION (expression path dead here!)")
			}
		} else {
			t.Logf("  primary_expression GOTO → 0 (no goto!)")
		}

		// Path 2: REDUCE(_unannotated_type) → GOTO
		utGoto := lookupGotoTest(parser, lang, parentState, unannotatedTypeSym)
		if utGoto != 0 {
			utActions := lookupActionsAll(lang, utGoto, dotSym)
			t.Logf("  _unannotated_type GOTO → state %d", utGoto)
			if len(utActions) > 0 {
				for _, a := range utActions {
					t.Logf("    '.' → %s", descActionJ(lang, a))
				}
			} else {
				t.Logf("    '.' → NO ACTION")
			}
		} else {
			t.Logf("  _unannotated_type GOTO → 0 (no goto!)")
		}
	}

	// === Trace all 3 GLR paths through '.' and 'identifier("method")' and '(' ===
	t.Log("\n=== Tracing 3 GLR paths through identifier.identifier( ===")

	lparenSym := findSymJ(lang, "(")
	semiSym := findSymJ(lang, ";")

	type pathTrace struct {
		name  string
		state gotreesitter.StateID
	}

	// Pick one representative parent state (any state that shifts ident→423)
	// For inside a block, the parent is likely one of the common ones
	parent := parentStates[0] // state 1

	paths := []pathTrace{
		{"expr(primary_expression)", lookupGotoTest(parser, lang, parent, primaryExprSym)},
		{"type(_unannotated_type)", lookupGotoTest(parser, lang, parent, unannotatedTypeSym)},
		{"direct_shift", 953},
	}

	// Each path starts at the state after reducing/shifting identifier("obj")
	// Now they need to process '.' then 'identifier("method")' then '('
	for _, p := range paths {
		t.Logf("\n--- Path: %s, starting state: %d ---", p.name, p.state)

		// Step 1: process '.' in current state
		dotActions := lookupActionsAll(lang, p.state, dotSym)
		t.Logf("State %d + '.': %d actions", p.state, len(dotActions))
		for _, a := range dotActions {
			t.Logf("  %s", descActionJ(lang, a))
		}

		if len(dotActions) == 0 {
			t.Logf("  PATH DIES at '.'")
			continue
		}

		// For each '.' action, trace further
		for ai, dotAct := range dotActions {
			var afterDotState gotreesitter.StateID
			if dotAct.Type == gotreesitter.ParseActionShift && !dotAct.Extra {
				afterDotState = dotAct.State
			} else if dotAct.Type == gotreesitter.ParseActionReduce {
				// After reduce, GOTO from parent with the reduced symbol
				afterDotState = lookupGotoTest(parser, lang, p.state, dotAct.Symbol)
				t.Logf("  (reduce GOTO → %d, still needs to process '.')", afterDotState)
				// Then re-check '.' in the new state
				dotActions2 := lookupActionsAll(lang, afterDotState, dotSym)
				if len(dotActions2) > 0 && dotActions2[0].Type == gotreesitter.ParseActionShift {
					afterDotState = dotActions2[0].State
				} else {
					t.Logf("  PATH DIES after reduce+goto for '.'")
					continue
				}
			} else {
				continue
			}

			t.Logf("  action[%d]: after '.', state=%d", ai, afterDotState)

			// Step 2: process 'identifier("method")' in afterDotState
			identActions := lookupActionsAll(lang, afterDotState, identSym)
			t.Logf("  State %d + 'identifier': %d actions", afterDotState, len(identActions))
			for _, a := range identActions {
				t.Logf("    %s", descActionJ(lang, a))
			}

			if len(identActions) == 0 {
				t.Logf("    PATH DIES at 'identifier'")
				continue
			}

			for iai, identAct := range identActions {
				var afterIdentState gotreesitter.StateID
				if identAct.Type == gotreesitter.ParseActionShift && !identAct.Extra {
					afterIdentState = identAct.State
				} else if identAct.Type == gotreesitter.ParseActionReduce {
					afterIdentState = lookupGotoTest(parser, lang, afterDotState, identAct.Symbol)
					t.Logf("    (reduce GOTO → %d)", afterIdentState)
				} else {
					continue
				}

				t.Logf("    action[%d]: after 'identifier', state=%d", iai, afterIdentState)

				// Step 3: process '(' and ';' in afterIdentState
				lparenActions := lookupActionsAll(lang, afterIdentState, lparenSym)
				semiActions := lookupActionsAll(lang, afterIdentState, semiSym)
				t.Logf("    State %d + '(': %d actions", afterIdentState, len(lparenActions))
				for _, a := range lparenActions {
					t.Logf("      %s", descActionJ(lang, a))
				}
				t.Logf("    State %d + ';': %d actions", afterIdentState, len(semiActions))
				for _, a := range semiActions {
					t.Logf("      %s", descActionJ(lang, a))
				}
			}
		}
	}

	// Parse for reference
	tree, _ := parser.Parse([]byte(`public class Foo { void f() { obj.method(); } }`))
	t.Logf("\nParse result: hasError=%v  %s", tree.RootNode().HasError(), tree.RootNode().SExpr(lang))
}

func lookupActionsAll(lang *gotreesitter.Language, state gotreesitter.StateID, sym gotreesitter.Symbol) []gotreesitter.ParseAction {
	// Dense table
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
	// Small (sparse) table
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
		if pos+1 >= len(tbl) {
			break
		}
		sectionValue := tbl[pos]
		symbolCount := tbl[pos+1]
		pos += 2
		for s := uint16(0); s < symbolCount; s++ {
			if pos >= len(tbl) {
				break
			}
			if gotreesitter.Symbol(tbl[pos]) == sym {
				if int(sectionValue) < len(lang.ParseActions) {
					return lang.ParseActions[sectionValue].Actions
				}
				return nil
			}
			pos++
		}
	}
	return nil
}

func lookupGotoTest(parser *gotreesitter.Parser, lang *gotreesitter.Language, state gotreesitter.StateID, sym gotreesitter.Symbol) gotreesitter.StateID {
	// For ts2go grammars, non-terminal GOTO is stored directly as state ID
	idx := lookupRawGoto(lang, state, sym)
	if idx == 0 {
		return 0
	}
	// ts2go grammars encode goto values directly as state IDs
	if lang.TokenCount > 0 && uint32(sym) >= lang.TokenCount && lang.StateCount > 0 && lang.InitialState > 0 {
		return gotreesitter.StateID(idx)
	}
	// Fallback: look up in parse actions
	if int(idx) < len(lang.ParseActions) {
		entry := &lang.ParseActions[idx]
		if len(entry.Actions) > 0 && entry.Actions[0].Type == gotreesitter.ParseActionShift {
			return entry.Actions[0].State
		}
	}
	return 0
}

func lookupRawGoto(lang *gotreesitter.Language, state gotreesitter.StateID, sym gotreesitter.Symbol) uint16 {
	// Dense table
	if int(state) < len(lang.ParseTable) {
		row := lang.ParseTable[state]
		if int(sym) < len(row) {
			return row[sym]
		}
		return 0
	}
	// Small table
	smallIdx := int(state) - len(lang.ParseTable)
	if smallIdx < 0 || smallIdx >= len(lang.SmallParseTableMap) {
		return 0
	}
	offset := lang.SmallParseTableMap[smallIdx]
	tbl := lang.SmallParseTable
	if int(offset) >= len(tbl) {
		return 0
	}
	count := tbl[offset]
	pos := int(offset) + 1
	for g := uint16(0); g < count; g++ {
		if pos+1 >= len(tbl) {
			break
		}
		sectionValue := tbl[pos]
		symbolCount := tbl[pos+1]
		pos += 2
		for s := uint16(0); s < symbolCount; s++ {
			if pos >= len(tbl) {
				break
			}
			if gotreesitter.Symbol(tbl[pos]) == sym {
				return sectionValue
			}
			pos++
		}
	}
	return 0
}

func descActionJ(lang *gotreesitter.Language, a gotreesitter.ParseAction) string {
	switch a.Type {
	case gotreesitter.ParseActionShift:
		if a.Extra {
			return fmt.Sprintf("SHIFT→%d(extra)", a.State)
		}
		return fmt.Sprintf("SHIFT→%d", a.State)
	case gotreesitter.ParseActionReduce:
		symName := "?"
		if int(a.Symbol) < len(lang.SymbolNames) {
			symName = lang.SymbolNames[a.Symbol]
		}
		return fmt.Sprintf("REDUCE(%s/%d,cnt=%d,prec=%d)", symName, a.Symbol, a.ChildCount, a.DynamicPrecedence)
	case gotreesitter.ParseActionAccept:
		return "ACCEPT"
	case gotreesitter.ParseActionRecover:
		return fmt.Sprintf("RECOVER→%d", a.State)
	default:
		return fmt.Sprintf("?type=%d", a.Type)
	}
}

// TestJavaStateDiag checks the small parse table behavior for state 423
// (the target after shifting 'identifier' in Java's grammar).
func TestJavaStateDiag(t *testing.T) {
	lang := DetectLanguage("Test.java").Language()

	t.Logf("Dense parse table rows: %d", len(lang.ParseTable))
	t.Logf("Small parse table map entries: %d", len(lang.SmallParseTableMap))
	t.Logf("StateCount: %d  TokenCount: %d", lang.StateCount, lang.TokenCount)

	dotSym := findSymJ(lang, ".")
	identSym := findSymJ(lang, "identifier")
	lparenSym := findSymJ(lang, "(")
	semiSym := findSymJ(lang, ";")

	t.Logf("identifier=%d  dot=%d  lparen=%d  semi=%d", identSym, dotSym, lparenSym, semiSym)

	// State 423: decode each group separately to see action indices
	state423 := 423
	smallIdx := state423 - len(lang.ParseTable)
	t.Logf("\nState 423 in small table: smallIdx=%d", smallIdx)

	if smallIdx < 0 || smallIdx >= len(lang.SmallParseTableMap) {
		t.Fatalf("smallIdx %d out of range", smallIdx)
	}

	offset := lang.SmallParseTableMap[smallIdx]
	tbl := lang.SmallParseTable
	t.Logf("Small table offset=%d", offset)

	if int(offset) >= len(tbl) {
		t.Fatalf("offset %d out of range (len=%d)", offset, len(tbl))
	}

	groupCount := tbl[offset]
	t.Logf("Group count: %d", groupCount)

	pos := int(offset) + 1
	for g := uint16(0); g < groupCount; g++ {
		if pos+1 >= len(tbl) {
			break
		}
		sectionValue := tbl[pos]
		symbolCount := tbl[pos+1]
		pos += 2

		// Describe the action at this section value
		actionDesc := "NO_ACTION"
		if int(sectionValue) < len(lang.ParseActions) {
			entry := lang.ParseActions[sectionValue]
			actionDesc = descActionsJ(lang, entry.Actions)
		}

		// Check if our key symbols are in this group
		hasDot := false
		hasIdent := false
		hasLparen := false
		hasSemi := false
		terminalCount := 0
		nonterminalCount := 0

		savedPos := pos
		for j := uint16(0); j < symbolCount; j++ {
			if pos >= len(tbl) {
				break
			}
			sym := tbl[pos]
			pos++
			if uint32(sym) < lang.TokenCount {
				terminalCount++
			} else {
				nonterminalCount++
			}
			if gotreesitter.Symbol(sym) == dotSym {
				hasDot = true
			}
			if gotreesitter.Symbol(sym) == identSym {
				hasIdent = true
			}
			if gotreesitter.Symbol(sym) == lparenSym {
				hasLparen = true
			}
			if gotreesitter.Symbol(sym) == semiSym {
				hasSemi = true
			}
		}
		_ = savedPos

		t.Logf("Group %d: sectionValue=%d symbolCount=%d terminals=%d nonterminals=%d action=%s",
			g, sectionValue, symbolCount, terminalCount, nonterminalCount, actionDesc)
		t.Logf("  has: dot=%v identifier=%v lparen=%v semi=%v",
			hasDot, hasIdent, hasLparen, hasSemi)
	}

	// Now let's also check: what does the C tree-sitter small table return
	// for state 423 + '.'? In the C runtime, the linear scan goes through
	// ALL groups and the LAST match wins (it overwrites result on each hit).
	// Our Go runtime returns the FIRST match. Let's check if this matters.
	t.Log("\n=== Checking C vs Go lookup semantics ===")
	checkSymbol := func(sym gotreesitter.Symbol, name string) {
		firstMatch := uint16(0)
		lastMatch := uint16(0)
		matchCount := 0

		pos := int(offset) + 1
		for g := uint16(0); g < groupCount; g++ {
			if pos+1 >= len(tbl) {
				break
			}
			sectionValue := tbl[pos]
			symbolCount := tbl[pos+1]
			pos += 2
			for j := uint16(0); j < symbolCount; j++ {
				if pos >= len(tbl) {
					break
				}
				if gotreesitter.Symbol(tbl[pos]) == sym {
					matchCount++
					if matchCount == 1 {
						firstMatch = sectionValue
					}
					lastMatch = sectionValue
				}
				pos++
			}
		}

		firstDesc := "(none)"
		lastDesc := "(none)"
		if matchCount > 0 {
			if int(firstMatch) < len(lang.ParseActions) {
				firstDesc = fmt.Sprintf("idx=%d %s", firstMatch, descActionsJ(lang, lang.ParseActions[firstMatch].Actions))
			}
			if int(lastMatch) < len(lang.ParseActions) {
				lastDesc = fmt.Sprintf("idx=%d %s", lastMatch, descActionsJ(lang, lang.ParseActions[lastMatch].Actions))
			}
		}
		t.Logf("%s (sym=%d): matches=%d first=%s last=%s same=%v",
			name, sym, matchCount, firstDesc, lastDesc, firstMatch == lastMatch)
	}

	checkSymbol(dotSym, ".")
	checkSymbol(identSym, "identifier")
	checkSymbol(lparenSym, "(")
	checkSymbol(semiSym, ";")
}
