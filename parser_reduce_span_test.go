package gotreesitter

import "testing"

func TestExtendParentSpanToWindowExtendsBothSides(t *testing.T) {
	parent := NewParentNode(3, true, nil, nil, 0)
	parent.startByte = 10
	parent.endByte = 20
	parent.startPoint = Point{Row: 1, Column: 10}
	parent.endPoint = Point{Row: 1, Column: 20}

	leadingExtra := NewLeafNode(1, false, 8, 9, Point{Row: 1, Column: 8}, Point{Row: 1, Column: 9})
	leadingExtra.isExtra = true
	core := NewLeafNode(2, true, 10, 20, Point{Row: 1, Column: 10}, Point{Row: 1, Column: 20})
	trailingExtra := NewLeafNode(1, false, 20, 24, Point{Row: 1, Column: 20}, Point{Row: 1, Column: 24})
	trailingExtra.isExtra = true

	entries := []stackEntry{
		{state: 0, node: leadingExtra},
		{state: 0, node: core},
		{state: 0, node: trailingExtra},
	}
	extendParentSpanToWindow(parent, entries, 0, len(entries), []*Node{trailingExtra}, false)

	if got, want := parent.startByte, uint32(8); got != want {
		t.Fatalf("parent.startByte = %d, want %d", got, want)
	}
	if got, want := parent.endByte, uint32(24); got != want {
		t.Fatalf("parent.endByte = %d, want %d", got, want)
	}
	if parent.startPoint != (Point{Row: 1, Column: 8}) {
		t.Fatalf("parent.startPoint = %+v, want {Row:1 Column:8}", parent.startPoint)
	}
	if parent.endPoint != (Point{Row: 1, Column: 24}) {
		t.Fatalf("parent.endPoint = %+v, want {Row:1 Column:24}", parent.endPoint)
	}
}

func TestExtendParentSpanToWindowNoTrailingExtras(t *testing.T) {
	parent := NewParentNode(3, true, nil, nil, 0)
	parent.startByte = 10
	parent.endByte = 20
	parent.startPoint = Point{Row: 2, Column: 10}
	parent.endPoint = Point{Row: 2, Column: 20}

	core := NewLeafNode(2, true, 10, 20, Point{Row: 2, Column: 10}, Point{Row: 2, Column: 20})
	entries := []stackEntry{{state: 0, node: core}}
	extendParentSpanToWindow(parent, entries, 0, len(entries), nil, false)

	if got, want := parent.startByte, uint32(10); got != want {
		t.Fatalf("parent.startByte = %d, want %d", got, want)
	}
	if got, want := parent.endByte, uint32(20); got != want {
		t.Fatalf("parent.endByte = %d, want %d", got, want)
	}
}

func TestExtendParentSpanToWindowIncludesInvisibleWindowWhenEnabled(t *testing.T) {
	parent := NewParentNode(3, true, nil, nil, 0)
	parent.startByte = 16
	parent.endByte = 18
	parent.startPoint = Point{Row: 1, Column: 16}
	parent.endPoint = Point{Row: 1, Column: 18}

	invisible := NewLeafNode(4, false, 13, 16, Point{Row: 1, Column: 13}, Point{Row: 1, Column: 16})
	core := NewLeafNode(5, true, 16, 18, Point{Row: 1, Column: 16}, Point{Row: 1, Column: 18})
	entries := []stackEntry{
		{state: 0, node: invisible},
		{state: 0, node: core},
	}

	extendParentSpanToWindow(parent, entries, 0, len(entries), nil, true)

	if got, want := parent.startByte, uint32(13); got != want {
		t.Fatalf("parent.startByte = %d, want %d", got, want)
	}
	if got, want := parent.endByte, uint32(18); got != want {
		t.Fatalf("parent.endByte = %d, want %d", got, want)
	}
}
