package gotreesitter

// glrStack is one version of the parse stack in a GLR parser.
// When the parse table has multiple actions for a (state, symbol) pair,
// the parser forks: one glrStack per alternative. Stacks that hit errors
// are dropped; surviving stacks are merged when their top states converge.
type glrStack struct {
	entries []stackEntry
	// byteOffset tracks the end byte of the latest non-nil node on stack.
	// It avoids rescanning entries in merge/retention hot paths.
	byteOffset uint32
	// score tracks dynamic precedence accumulated through reduce actions.
	// It is used for tie-breaking when choosing a final parse.
	score int
	// dead marks a stack version that encountered an error and should be
	// removed at the next merge point.
	dead bool
	// accepted is set when the stack reaches a ParseActionAccept.
	accepted bool
	// shifted is set when this stack consumed the current token via a SHIFT
	// action in a GLR fork that also produced REDUCE actions. When the
	// reducing stacks cause the same token to be re-processed, shifted
	// stacks must be skipped since they already consumed it.
	shifted bool
}

const (
	defaultStackEntrySlabCap = 4 * 1024
	// Retain enough entry-scratch capacity to avoid re-allocating large
	// GLR stacks on every parse pass.
	maxRetainedStackEntryCap = 64 * 1024 * 1024
	// Tree-sitter's C runtime caps links per stack node at 8.
	// We cap distinct alternatives per merge key similarly to avoid
	// unbounded stack growth while preserving multiple paths.
	maxStacksPerMergeKey = 8
)

type glrMergeScratch struct {
	result []glrStack
	keys   []glrMergeKey
	alive  []glrStack
}

type glrMergeKey struct {
	state      StateID
	byteOffset uint32
}

type glrEntryScratch struct {
	slabs      []stackEntrySlab
	slabCursor int
}

type stackEntrySlab struct {
	data []stackEntry
	used int
}

func newGLRStack(initial StateID) glrStack {
	return glrStack{
		entries: []stackEntry{{state: initial, node: nil}},
	}
}

func newGLRStackWithScratch(initial StateID, scratch *glrEntryScratch) glrStack {
	if scratch == nil {
		return newGLRStack(initial)
	}
	entries := scratch.allocWithCap(1, 8)
	entries[0] = stackEntry{state: initial}
	return glrStack{entries: entries}
}

func (s *glrStack) top() stackEntry {
	return s.entries[len(s.entries)-1]
}

func (s *glrStack) clone() glrStack {
	entries := make([]stackEntry, len(s.entries))
	copy(entries, s.entries)
	return glrStack{entries: entries, byteOffset: s.byteOffset, score: s.score}
}

func (s *glrStack) cloneWithScratch(scratch *glrEntryScratch) glrStack {
	if scratch == nil {
		return s.clone()
	}
	entries := scratch.clone(s.entries)
	return glrStack{entries: entries, byteOffset: s.byteOffset, score: s.score}
}

func (s *glrStack) push(state StateID, node *Node, scratch *glrEntryScratch) {
	if scratch == nil {
		s.entries = append(s.entries, stackEntry{state: state, node: node})
		if node != nil {
			s.byteOffset = node.endByte
		}
		return
	}
	if len(s.entries) == cap(s.entries) {
		s.entries = scratch.grow(s.entries, len(s.entries)+1)
	}
	idx := len(s.entries)
	s.entries = s.entries[:idx+1]
	s.entries[idx] = stackEntry{state: state, node: node}
	if node != nil {
		s.byteOffset = node.endByte
	}
}

func (s *glrStack) recomputeByteOffset() {
	s.byteOffset = stackByteOffset(s.entries)
}

// mergeStacks removes dead stacks and collapses only truly duplicate
// active stacks. Two stacks are considered merge-compatible only when
// they share the same top parser state and byte position (matching the
// C runtime's stack merge preconditions), and their stack entries are
// identical. Distinct parse paths are preserved.
func mergeStacks(stacks []glrStack) []glrStack {
	var scratch glrMergeScratch
	return mergeStacksWithScratch(stacks, &scratch)
}

func stackByteOffset(entries []stackEntry) uint32 {
	for i := len(entries) - 1; i >= 0; i-- {
		if entries[i].node != nil {
			return entries[i].node.endByte
		}
		if i == 0 {
			break
		}
	}
	return 0
}

func mergeKeyForStack(s glrStack) glrMergeKey {
	if len(s.entries) == 0 {
		return glrMergeKey{}
	}
	top := s.entries[len(s.entries)-1]
	return glrMergeKey{
		state:      top.state,
		byteOffset: s.byteOffset,
	}
}

func stackEntriesEqual(a, b []stackEntry) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].state != b[i].state || !stackEntryNodesEquivalent(a[i].node, b[i].node) {
			return false
		}
	}
	return true
}

func stackEntryNodesEquivalent(a, b *Node) bool {
	if a == b {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if a.symbol != b.symbol {
		return false
	}
	if a.startByte != b.startByte ||
		a.endByte != b.endByte ||
		a.isExtra != b.isExtra ||
		a.isNamed != b.isNamed ||
		a.hasError != b.hasError ||
		a.parseState != b.parseState ||
		a.productionID != b.productionID ||
		len(a.children) != len(b.children) {
		return false
	}
	if a.hasError && b.hasError {
		return true
	}
	for i := range a.children {
		ca := a.children[i]
		cb := b.children[i]
		if ca == cb {
			continue
		}
		if ca == nil || cb == nil {
			return false
		}
		if ca.symbol != cb.symbol ||
			ca.startByte != cb.startByte ||
			ca.endByte != cb.endByte ||
			ca.isExtra != cb.isExtra ||
			ca.isNamed != cb.isNamed ||
			ca.hasError != cb.hasError ||
			len(ca.children) != len(cb.children) {
			return false
		}
	}
	return true
}

func isBetterStack(candidate, current glrStack) bool {
	if candidate.accepted != current.accepted {
		return candidate.accepted
	}
	if candidate.score != current.score {
		return candidate.score > current.score
	}
	if len(candidate.entries) != len(current.entries) {
		return len(candidate.entries) > len(current.entries)
	}
	if candidate.shifted != current.shifted {
		return !candidate.shifted && current.shifted
	}
	return false
}

func mergeStacksWithScratch(stacks []glrStack, scratch *glrMergeScratch) []glrStack {
	if len(stacks) == 0 {
		return stacks
	}

	// Remove dead stacks first.
	alive := stacks[:0]
	for i := range stacks {
		if !stacks[i].dead {
			alive = append(alive, stacks[i])
		}
	}
	if len(alive) <= 1 {
		return alive
	}
	if scratch == nil {
		local := glrMergeScratch{}
		scratch = &local
	}
	aliveSnapshot := ensureMergeAliveCap(scratch, len(alive))
	copy(aliveSnapshot, alive)

	// Merge exact duplicates and keep a bounded number of distinct
	// alternatives per merge key. This approximates the C runtime's
	// graph-stack link fanout while keeping memory bounded.
	result := ensureMergeResultCap(scratch, len(aliveSnapshot))
	keys := ensureMergeKeyCap(scratch, len(aliveSnapshot))
	for i := range aliveSnapshot {
		stack := aliveSnapshot[i]
		key := mergeKeyForStack(stack)
		duplicateIndex := -1
		sameKeyCount := 0
		worstIndex := -1
		for j := range result {
			if keys[j] != key {
				continue
			}
			sameKeyCount++
			if !stackEntriesEqual(result[j].entries, stack.entries) {
				if worstIndex == -1 || isBetterStack(result[worstIndex], result[j]) {
					worstIndex = j
				}
				continue
			}
			duplicateIndex = j
			break
		}
		if duplicateIndex >= 0 {
			if isBetterStack(stack, result[duplicateIndex]) {
				result[duplicateIndex] = stack
			}
			continue
		}

		if sameKeyCount < maxStacksPerMergeKey {
			result = append(result, stack)
			keys = append(keys, key)
			continue
		}

		// Per-key alternative budget reached: replace the weakest
		// retained candidate only if this stack is better.
		if worstIndex >= 0 && isBetterStack(stack, result[worstIndex]) {
			result[worstIndex] = stack
		}
	}
	scratch.result = result
	scratch.keys = keys
	return result
}

func ensureMergeResultCap(scratch *glrMergeScratch, n int) []glrStack {
	if cap(scratch.result) < n {
		scratch.result = make([]glrStack, 0, n)
	}
	return scratch.result[:0]
}

func ensureMergeKeyCap(scratch *glrMergeScratch, n int) []glrMergeKey {
	if cap(scratch.keys) < n {
		scratch.keys = make([]glrMergeKey, 0, n)
	}
	return scratch.keys[:0]
}

func ensureMergeAliveCap(scratch *glrMergeScratch, n int) []glrStack {
	if cap(scratch.alive) < n {
		scratch.alive = make([]glrStack, n)
		return scratch.alive
	}
	return scratch.alive[:n]
}

func (s *glrEntryScratch) alloc(n int) []stackEntry {
	return s.allocWithCap(n, n)
}

func (s *glrEntryScratch) allocWithCap(length, capacity int) []stackEntry {
	if length <= 0 {
		return nil
	}
	if capacity < length {
		capacity = length
	}
	if capacity <= 0 {
		capacity = length
	}

	n := capacity
	if n <= 0 {
		return nil
	}
	if len(s.slabs) == 0 {
		capacity := defaultStackEntrySlabCap
		if n > capacity {
			capacity = n
		}
		s.slabs = append(s.slabs, stackEntrySlab{data: make([]stackEntry, capacity)})
		s.slabCursor = 0
	}
	if s.slabCursor < 0 || s.slabCursor >= len(s.slabs) {
		s.slabCursor = 0
	}
	for i := s.slabCursor; ; i++ {
		if i >= len(s.slabs) {
			lastCap := defaultStackEntrySlabCap
			if len(s.slabs) > 0 {
				lastCap = len(s.slabs[len(s.slabs)-1].data)
			}
			capacity := lastCap * 2
			if capacity < defaultStackEntrySlabCap {
				capacity = defaultStackEntrySlabCap
			}
			if n > capacity {
				capacity = n
			}
			s.slabs = append(s.slabs, stackEntrySlab{data: make([]stackEntry, capacity)})
		}
		slab := &s.slabs[i]
		if len(slab.data)-slab.used < n {
			continue
		}
		start := slab.used
		slab.used += n
		s.slabCursor = i
		end := start + length
		return slab.data[start : end : start+capacity]
	}
}

func (s *glrEntryScratch) clone(entries []stackEntry) []stackEntry {
	if len(entries) == 0 {
		return nil
	}
	// Keep clone capacity tight to minimize per-fork copied bytes.
	out := s.allocWithCap(len(entries), len(entries)+1)
	copy(out, entries)
	return out
}

func (s *glrEntryScratch) grow(entries []stackEntry, minCap int) []stackEntry {
	newCap := cap(entries) * 2
	if newCap < 1 {
		newCap = 1
	}
	if newCap < minCap {
		newCap = minCap
	}
	out := s.alloc(newCap)
	copy(out, entries)
	return out[:len(entries)]
}

func (s *glrEntryScratch) reset() {
	if len(s.slabs) == 0 {
		return
	}

	totalCap := 0
	for i := range s.slabs {
		totalCap += len(s.slabs[i].data)
	}

	if totalCap > maxRetainedStackEntryCap {
		// Keep the newest/largest slabs up to the retention budget.
		keepFrom := len(s.slabs) - 1
		retained := len(s.slabs[keepFrom].data)
		for keepFrom > 0 {
			next := retained + len(s.slabs[keepFrom-1].data)
			if next > maxRetainedStackEntryCap {
				break
			}
			keepFrom--
			retained = next
		}
		if keepFrom > 0 {
			copy(s.slabs, s.slabs[keepFrom:])
			s.slabs = s.slabs[:len(s.slabs)-keepFrom]
		}
		for i := range s.slabs {
			slab := &s.slabs[i]
			clear(slab.data[:slab.used])
			slab.used = 0
		}
		s.slabCursor = 0
		return
	}

	for i := range s.slabs {
		slab := &s.slabs[i]
		clear(slab.data[:slab.used])
		slab.used = 0
	}
	s.slabCursor = 0
}
