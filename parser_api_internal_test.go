package gotreesitter

import "testing"

func TestShouldRetryNodeLimitParse(t *testing.T) {
	tree := &Tree{
		parseRuntime: ParseRuntime{
			StopReason:     ParseStopNodeLimit,
			NodeLimit:      300_000,
			NodesAllocated: 300_001,
		},
	}

	if !shouldRetryNodeLimitParse(tree, 4096) {
		t.Fatal("shouldRetryNodeLimitParse = false, want true")
	}
}

func TestShouldNotRetryNodeLimitParseForLargeSource(t *testing.T) {
	tree := &Tree{
		parseRuntime: ParseRuntime{
			StopReason:     ParseStopNodeLimit,
			NodeLimit:      300_000,
			NodesAllocated: 300_001,
		},
	}

	if shouldRetryNodeLimitParse(tree, fullParseRetryMaxSourceBytes+1) {
		t.Fatal("shouldRetryNodeLimitParse = true, want false")
	}
}

func TestFullParseRetryNodeLimitOverride(t *testing.T) {
	tree := &Tree{
		parseRuntime: ParseRuntime{
			StopReason:     ParseStopNodeLimit,
			NodeLimit:      300_000,
			NodesAllocated: 300_001,
		},
	}

	got := fullParseRetryNodeLimitOverride(tree, 4096)
	want := 600_000
	if got != want {
		t.Fatalf("fullParseRetryNodeLimitOverride = %d, want %d", got, want)
	}
}

func TestFullParseRetrySecondaryNodeLimitOverride(t *testing.T) {
	tree := &Tree{
		parseRuntime: ParseRuntime{
			StopReason:     ParseStopNodeLimit,
			NodeLimit:      600_000,
			NodesAllocated: 600_001,
		},
	}

	got := fullParseRetrySecondaryNodeLimitOverride(tree, 4096)
	want := 1_800_000
	if got != want {
		t.Fatalf("fullParseRetrySecondaryNodeLimitOverride = %d, want %d", got, want)
	}
}

func TestPreferRetryTreePrefersFurtherAcceptedProgress(t *testing.T) {
	incumbent := &Tree{
		root: &Node{
			endByte:  100,
			hasError: true,
			children: []*Node{{}, {}, {}},
		},
		parseRuntime: ParseRuntime{
			StopReason:      ParseStopNoStacksAlive,
			ExpectedEOFByte: 200,
			Truncated:       true,
		},
	}
	candidate := &Tree{
		root: &Node{
			endByte:  200,
			hasError: true,
			children: []*Node{{}, {}},
		},
		parseRuntime: ParseRuntime{
			StopReason:      ParseStopAccepted,
			ExpectedEOFByte: 200,
		},
	}

	if !preferRetryTree(candidate, incumbent) {
		t.Fatal("preferRetryTree = false, want true for accepted full-length retry")
	}
}

func TestPreferRetryTreePrefersFewerChildrenOnEqualErrorTrees(t *testing.T) {
	incumbent := &Tree{
		root: &Node{
			endByte:  200,
			hasError: true,
			children: make([]*Node, 12),
		},
		parseRuntime: ParseRuntime{
			StopReason:      ParseStopAccepted,
			ExpectedEOFByte: 200,
			NodesAllocated:  1200,
		},
	}
	candidate := &Tree{
		root: &Node{
			endByte:  200,
			hasError: true,
			children: make([]*Node, 4),
		},
		parseRuntime: ParseRuntime{
			StopReason:      ParseStopAccepted,
			ExpectedEOFByte: 200,
			NodesAllocated:  800,
		},
	}

	if !preferRetryTree(candidate, incumbent) {
		t.Fatal("preferRetryTree = false, want true for smaller equal-span error tree")
	}
}

func TestGLRStackCullTrigger(t *testing.T) {
	if got := glrStackCullTrigger(8, arenaClassFull, "go"); got != 12 {
		t.Fatalf("glrStackCullTrigger(full, go) = %d, want 12", got)
	}
	if got := glrStackCullTrigger(8, arenaClassFull, "c_sharp"); got != 8 {
		t.Fatalf("glrStackCullTrigger(full, c_sharp) = %d, want 8", got)
	}
	if got := glrStackCullTrigger(8, arenaClassIncremental, "go"); got != 8 {
		t.Fatalf("glrStackCullTrigger(incremental, go) = %d, want 8", got)
	}
	maxInt := int(^uint(0) >> 1)
	if got := glrStackCullTrigger(maxInt, arenaClassFull, "go"); got != maxInt {
		t.Fatalf("glrStackCullTrigger(maxInt) = %d, want %d", got, maxInt)
	}
}
