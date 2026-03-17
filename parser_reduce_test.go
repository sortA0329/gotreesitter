package gotreesitter

import (
	"testing"
)

// TestExtendParentSpanToWindow_ContiguityGap verifies that
// extendParentSpanToWindow handles visible entries in the reduce window
// without incorrect span extension. The actual span for visible entries
// is set by populateParentNode (from children), not by
// extendParentSpanToWindow (which only handles invisible entries).
func TestExtendParentSpanToWindow_ContiguityGap(t *testing.T) {
	meta := []SymbolMetadata{
		{Visible: false, Named: false}, // sym 0: end
		{Visible: true, Named: true},   // sym 1: expression (visible, named)
		{Visible: false, Named: true},  // sym 2: _expr_term (invisible, named)
		{Visible: true, Named: true},   // sym 3: variable_expr (visible, named)
		{Visible: true, Named: true},   // sym 4: get_attr (visible, named)
		{Visible: true, Named: false},  // sym 5: "." (visible, anonymous)
		{Visible: true, Named: true},   // sym 6: identifier (visible, named)
	}
	names := []string{"end", "expression", "_expr_term", "variable_expr", "get_attr", ".", "identifier"}

	// Simulated stack entries where all are visible — extendParentSpanToWindow
	// should not modify the parent span since it only handles invisible entries.
	entries := []stackEntry{
		{node: &Node{symbol: 3, startByte: 212, endByte: 230}},
		{node: &Node{symbol: 5, startByte: 230, endByte: 231}},
		{node: &Node{symbol: 4, startByte: 231, endByte: 250}},
		{node: &Node{symbol: 5, startByte: 250, endByte: 251}},
		{node: &Node{symbol: 4, startByte: 251, endByte: 253}},
	}

	parent := &Node{
		symbol:    1,
		startByte: 212,
		endByte:   230,
	}

	extendParentSpanToWindow(parent, entries, 0, len(entries), meta, names)

	// Visible entries are not processed by extendParentSpanToWindow.
	// The parent span is unchanged because all entries are visible.
	if parent.endByte != 230 {
		t.Errorf("expected parent endByte=230 (unchanged for visible entries), got %d", parent.endByte)
	}
}

// TestExtendParentSpan_InvisibleEntryWithVisibleChildren tests that an
// invisible entry node with a span wider than its inlined children correctly
// extends the parent's span.
func TestExtendParentSpan_InvisibleEntryWithVisibleChildren(t *testing.T) {
	meta := []SymbolMetadata{
		{Visible: false, Named: false}, // sym 0
		{Visible: true, Named: true},   // sym 1: parent
		{Visible: false, Named: true},  // sym 2: _expr_term (invisible)
	}
	names := []string{"end", "expression", "_expr_term"}

	invisNode := &Node{
		symbol:    2,
		startByte: 212,
		endByte:   253,
		children: []*Node{
			{symbol: 3, startByte: 212, endByte: 230},
			{symbol: 4, startByte: 231, endByte: 253},
		},
	}

	entries := []stackEntry{
		{node: invisNode},
	}

	parent := &Node{
		symbol:    1,
		startByte: 212,
		endByte:   230,
	}

	extendParentSpanToWindow(parent, entries, 0, len(entries), meta, names)

	if parent.endByte != 253 {
		t.Errorf("expected parent endByte=253 after extending from invisible entry, got %d", parent.endByte)
	}
}

// TestComputeReduceRawSpan_BasicSpan verifies that computeReduceRawSpan
// computes the correct span from stack entries, using the first and last
// non-extra entries.
func TestComputeReduceRawSpan_BasicSpan(t *testing.T) {
	entries := []stackEntry{
		{node: &Node{startByte: 10, endByte: 20}},
		{node: &Node{startByte: 20, endByte: 30}},
		{node: &Node{startByte: 30, endByte: 50}},
	}

	span := computeReduceRawSpan(entries, 0, len(entries))

	if span.startByte != 10 {
		t.Errorf("expected startByte=10, got %d", span.startByte)
	}
	if span.endByte != 50 {
		t.Errorf("expected endByte=50, got %d", span.endByte)
	}
}

// TestComputeReduceRawSpan_SkipsExtras verifies that computeReduceRawSpan
// skips extra nodes when computing the span.
func TestComputeReduceRawSpan_SkipsExtras(t *testing.T) {
	entries := []stackEntry{
		{node: &Node{startByte: 5, endByte: 10, isExtra: true}},
		{node: &Node{startByte: 10, endByte: 20}},
		{node: &Node{startByte: 20, endByte: 30}},
		{node: &Node{startByte: 30, endByte: 40, isExtra: true}},
	}

	span := computeReduceRawSpan(entries, 0, len(entries))

	if span.startByte != 10 {
		t.Errorf("expected startByte=10, got %d", span.startByte)
	}
	if span.endByte != 30 {
		t.Errorf("expected endByte=30, got %d", span.endByte)
	}
}
