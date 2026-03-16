package gotreesitter

import "testing"

func TestExternalLexerDefaultEndWithoutMarkEnd(t *testing.T) {
	l := newExternalLexer([]byte("abc"), 0, 0, 0)
	l.Advance(false) // consume 'a'
	l.SetResultSymbol(1)

	tok, ok := l.token()
	if !ok {
		t.Fatal("token() returned !ok")
	}
	if got, want := tok.StartByte, uint32(0); got != want {
		t.Fatalf("StartByte=%d want=%d", got, want)
	}
	if got, want := tok.EndByte, uint32(1); got != want {
		t.Fatalf("EndByte=%d want=%d", got, want)
	}
}

func TestExternalLexerMarkEndFreezesSpan(t *testing.T) {
	l := newExternalLexer([]byte("abc"), 0, 0, 0)
	l.Advance(false) // consume 'a'
	l.MarkEnd()      // end at 1
	l.Advance(false) // look ahead through 'b'
	l.SetResultSymbol(1)

	tok, ok := l.token()
	if !ok {
		t.Fatal("token() returned !ok")
	}
	if got, want := tok.StartByte, uint32(0); got != want {
		t.Fatalf("StartByte=%d want=%d", got, want)
	}
	if got, want := tok.EndByte, uint32(1); got != want {
		t.Fatalf("EndByte=%d want=%d", got, want)
	}
}

func TestExternalLexerMarkBeforeSkipZeroWidth(t *testing.T) {
	l := newExternalLexer([]byte(" abc"), 0, 0, 0)
	l.MarkEnd()     // mark at 0
	l.Advance(true) // skip leading space
	l.SetResultSymbol(1)

	tok, ok := l.token()
	if !ok {
		t.Fatal("token() returned !ok")
	}
	if got, want := tok.StartByte, uint32(0); got != want {
		t.Fatalf("StartByte=%d want=%d", got, want)
	}
	if got, want := tok.EndByte, uint32(0); got != want {
		t.Fatalf("EndByte=%d want=%d", got, want)
	}
}

func TestExternalLexerSkipOnlyWithoutMarkEndUsesCurrentCursor(t *testing.T) {
	l := newExternalLexer([]byte("\n    }"), 0, 0, 0)
	l.Advance(true)
	l.Advance(true)
	l.Advance(true)
	l.Advance(true)
	l.Advance(true)
	l.SetResultSymbol(1)

	tok, ok := l.token()
	if !ok {
		t.Fatal("token() returned !ok")
	}
	if got, want := tok.StartByte, uint32(5); got != want {
		t.Fatalf("StartByte=%d want=%d", got, want)
	}
	if got, want := tok.EndByte, uint32(5); got != want {
		t.Fatalf("EndByte=%d want=%d", got, want)
	}
	if got, want := tok.StartPoint, (Point{Row: 1, Column: 4}); got != want {
		t.Fatalf("StartPoint=%+v want=%+v", got, want)
	}
	if got, want := tok.EndPoint, (Point{Row: 1, Column: 4}); got != want {
		t.Fatalf("EndPoint=%+v want=%+v", got, want)
	}
}

func TestExternalLexerUsesByteColumnsForUTF8(t *testing.T) {
	l := newExternalLexer([]byte("x✗z"), 0, 0, 0)

	l.Advance(false) // x
	if got, want := l.Column(), uint32(1); got != want {
		t.Fatalf("column after x = %d want %d", got, want)
	}

	l.Advance(false) // ✗
	if got, want := l.Column(), uint32(4); got != want {
		t.Fatalf("column after utf8 rune = %d want %d", got, want)
	}

	l.MarkEnd()
	l.SetResultSymbol(1)
	tok, ok := l.token()
	if !ok {
		t.Fatal("token() returned !ok")
	}
	if got, want := tok.EndByte, uint32(4); got != want {
		t.Fatalf("EndByte=%d want=%d", got, want)
	}
	if got, want := tok.EndPoint.Column, uint32(4); got != want {
		t.Fatalf("EndPoint.Column=%d want=%d", got, want)
	}
}

func TestExternalLexerResetClearsScannerState(t *testing.T) {
	l := &ExternalLexer{}
	l.reset([]byte("abc"), 0, 0, 0)
	l.Advance(false)
	l.MarkEnd()
	l.SetResultSymbol(7)

	l.reset([]byte("z"), 0, 0, 0)
	if l.hasResult {
		t.Fatal("expected reset lexer to clear prior result")
	}
	if l.endMarked {
		t.Fatal("expected reset lexer to clear end mark")
	}
	tok, ok := l.token()
	if ok {
		t.Fatalf("token() on reset lexer unexpectedly succeeded: %+v", tok)
	}
	l.Advance(false)
	l.SetResultSymbol(1)
	tok, ok = l.token()
	if !ok {
		t.Fatal("token() on reset lexer returned !ok")
	}
	if got, want := tok.EndByte, uint32(1); got != want {
		t.Fatalf("EndByte=%d want=%d", got, want)
	}
}
