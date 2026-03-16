package gotreesitter

import "testing"

func TestNormalizeHaskellLocalBindsStartsIncludesTriviaAfterLet(t *testing.T) {
	lang := &Language{
		Name:        "haskell",
		SymbolNames: []string{"EOF", "let_in", "let", "local_binds"},
		SymbolMetadata: []SymbolMetadata{
			{Name: "EOF", Visible: false, Named: false},
			{Name: "let_in", Visible: true, Named: true},
			{Name: "let", Visible: false, Named: false},
			{Name: "local_binds", Visible: true, Named: true},
		},
	}

	arena := newNodeArena(arenaClassFull)
	letNode := newLeafNodeInArena(arena, 2, false, 0, 3, Point{}, Point{Column: 3})
	localBinds := newLeafNodeInArena(arena, 3, true, 4, 18, Point{Column: 4}, Point{Column: 18})
	root := newParentNodeInArena(arena, 1, true, []*Node{letNode, localBinds}, nil, 0)
	source := []byte("let uniqueBackends")

	normalizeHaskellLocalBindsStarts(root, source, lang)

	if got, want := localBinds.startByte, uint32(3); got != want {
		t.Fatalf("localBinds.startByte = %d, want %d", got, want)
	}
	if got, want := localBinds.startPoint, letNode.endPoint; got != want {
		t.Fatalf("localBinds.startPoint = %#v, want %#v", got, want)
	}
}
