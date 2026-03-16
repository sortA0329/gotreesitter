package gotreesitter

import "testing"

type retryingExternalScanner struct{}

func (retryingExternalScanner) Create() any { return nil }
func (retryingExternalScanner) Destroy(any) {}
func (retryingExternalScanner) Serialize(any, []byte) int { return 0 }
func (retryingExternalScanner) Deserialize(any, []byte)   {}

func (retryingExternalScanner) Scan(payload any, lexer *ExternalLexer, validSymbols []bool) bool {
	if len(validSymbols) > 0 && validSymbols[0] {
		lexer.SetResultSymbol(Symbol(2))
		return false
	}
	if len(validSymbols) > 1 && validSymbols[1] {
		lexer.SetResultSymbol(Symbol(3))
		lexer.Advance(false)
		lexer.MarkEnd()
		return true
	}
	return false
}

func TestNextExternalTokenRetriesScannerAfterFailedPreferredCandidate(t *testing.T) {
	lang := &Language{
		Name:               "test",
		SymbolNames:        []string{"end", "x", "first_ext", "second_ext"},
		SymbolMetadata:     []SymbolMetadata{{Name: "end"}, {Name: "x", Visible: true, Named: true}, {Name: "first_ext"}, {Name: "second_ext"}},
		SymbolCount:        4,
		TokenCount:         4,
		ExternalTokenCount: 2,
		StateCount:         2,
		LargeStateCount:    2,
		LexStates:          []LexState{{Default: -1, EOF: -1}},
		LexModes:           []LexMode{{LexState: 0}, {LexState: 0, ExternalLexState: 1}},
		ExternalSymbols:    []Symbol{2, 3},
		ExternalLexStates: [][]bool{
			{false, false},
			{true, true},
		},
		ExternalScanner: retryingExternalScanner{},
	}
	d := &dfaTokenSource{
		lexer:             NewLexer(lang.LexStates, []byte("x")),
		language:          lang,
		state:             1,
		lookupActionIndex: func(StateID, Symbol) uint16 { return 1 },
	}

	tok, ok := d.nextExternalToken()
	if !ok {
		t.Fatal("nextExternalToken returned ok=false, want true")
	}
	if got, want := tok.Symbol, Symbol(3); got != want {
		t.Fatalf("token symbol = %d, want %d", got, want)
	}
	if got, want := tok.StartByte, uint32(0); got != want {
		t.Fatalf("StartByte = %d, want %d", got, want)
	}
	if got, want := tok.EndByte, uint32(1); got != want {
		t.Fatalf("EndByte = %d, want %d", got, want)
	}
}
