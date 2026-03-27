package gotreesitter

import "strings"

func normalizeCobolLeadingAreaStart(root *Node, source []byte, lang *Language) {
	if root == nil || lang == nil || (lang.Name != "cobol" && lang.Name != "COBOL") || len(source) == 0 {
		return
	}
	start := firstNonWhitespaceByte(source)
	if start == 0 {
		// COBOL fixed format: columns 1-6 are sequence numbers (non-whitespace).
		// Detect this pattern and use column 7 (byte 6) as the adjusted start.
		if len(source) >= 7 && (source[6] == ' ' || source[6] == '*' || source[6] == '-' || source[6] == '/') {
			start = 6
		} else {
			return
		}
	}
	startPoint := advancePointByBytes(Point{}, source[:start])
	setNodeStartTo := func(n *Node) {
		if n == nil || n.startByte == start {
			return
		}
		n.startByte = start
		n.startPoint = startPoint
	}
	setNodeStartTo(root)
	if len(root.children) == 0 {
		return
	}
	def := (*Node)(nil)
	for _, child := range root.children {
		if child != nil && !child.IsExtra() && child.Type(lang) == "program_definition" {
			def = child
			break
		}
	}
	if def == nil {
		return
	}
	setNodeStartTo(def)
	if len(def.children) == 0 {
		return
	}
	for _, child := range def.children {
		if child != nil && !child.IsExtra() && child.Type(lang) == "identification_division" {
			setNodeStartTo(child)
			break
		}
	}
}

func normalizeCobolTopLevelDefinitionEnd(root *Node, source []byte, lang *Language) {
	if root == nil || lang == nil || (lang.Name != "cobol" && lang.Name != "COBOL") || root.Type(lang) != "start" || len(root.children) == 0 {
		return
	}
	def := (*Node)(nil)
	for _, child := range root.children {
		if child != nil && !child.IsExtra() && child.Type(lang) == "program_definition" {
			def = child
			break
		}
	}
	if def == nil {
		return
	}
	end := lastNonTriviaByteEnd(source)
	if end == 0 || end >= def.endByte {
		return
	}
	def.endByte = end
	def.endPoint = advancePointByBytes(Point{}, source[:end])
}

func normalizeCobolDivisionSiblingEnds(root *Node, source []byte, lang *Language) {
	if root == nil || lang == nil || (lang.Name != "cobol" && lang.Name != "COBOL") || root.Type(lang) != "start" || len(root.children) == 0 {
		return
	}
	def := (*Node)(nil)
	for _, child := range root.children {
		if child != nil && !child.IsExtra() && child.Type(lang) == "program_definition" {
			def = child
			break
		}
	}
	if def == nil {
		return
	}
	for i := 0; i+1 < len(def.children); i++ {
		cur := def.children[i]
		next := def.children[i+1]
		if cur == nil || next == nil || cur.IsExtra() || next.IsExtra() {
			continue
		}
		if !strings.HasSuffix(cur.Type(lang), "_division") {
			continue
		}
		end := lastNonTriviaByteEnd(source[:next.startByte])
		if end == 0 || end <= cur.startByte || end >= cur.endByte {
			continue
		}
		cur.endByte = end
		cur.endPoint = advancePointByBytes(Point{}, source[:end])
	}
}

func normalizeCobolPeriodChildren(root *Node, source []byte, lang *Language) {
	if root == nil || lang == nil || (lang.Name != "cobol" && lang.Name != "COBOL") {
		return
	}
	normalizeCollapsedNamedLeafChildren(root, lang, "period", ".")
}

// normalizeCollapsedNamedLeafChildren restores collapsed single-anonymous-child
// nodes. When a named node (parentName) wraps a single anonymous token
// (childName) and the collapse logic strips the child, this function
// reconstructs the child so the tree matches C tree-sitter output.
func normalizeCollapsedNamedLeafChildren(root *Node, lang *Language, parentName, childName string) {
	if root == nil || lang == nil {
		return
	}
	parentSym, ok := symbolByName(lang, parentName)
	if !ok {
		return
	}
	childSym, childOk := symbolByName(lang, childName)
	if !childOk {
		return
	}
	childNamed := false
	if int(childSym) < len(lang.SymbolMetadata) {
		childNamed = lang.SymbolMetadata[childSym].Named
	}
	var walk func(*Node)
	walk = func(n *Node) {
		if n == nil {
			return
		}
		if n.symbol == parentSym && len(n.children) == 0 {
			child := newLeafNodeInArena(n.ownerArena, childSym, childNamed, n.startByte, n.endByte, n.startPoint, n.endPoint)
			n.children = cloneNodeSliceInArena(n.ownerArena, []*Node{child})
		}
		for _, child := range n.children {
			walk(child)
		}
	}
	walk(root)
}

func normalizeNimTopLevelCallEnd(root *Node, source []byte, lang *Language) {
	if root == nil || lang == nil || lang.Name != "nim" || root.Type(lang) != "source_file" || len(root.children) != 1 {
		return
	}
	call := root.children[0]
	if call == nil || call.IsExtra() || call.Type(lang) != "call" {
		return
	}
	end := lastNonTriviaByteEnd(source)
	if end == 0 || end >= call.endByte {
		return
	}
	call.endByte = end
	call.endPoint = advancePointByBytes(Point{}, source[:end])
}

func normalizePascalTopLevelProgramEnd(root *Node, source []byte, lang *Language) {
	if root == nil || lang == nil || lang.Name != "pascal" || root.Type(lang) != "root" || len(root.children) == 0 {
		return
	}
	program := root.children[0]
	if program == nil || program.IsExtra() || program.Type(lang) != "program" {
		return
	}
	end := lastNonTriviaByteEnd(source)
	if end == 0 || end >= program.endByte {
		return
	}
	program.endByte = end
	program.endPoint = advancePointByBytes(Point{}, source[:end])
}

func normalizeCommentTrailingExtraTrivia(root *Node, source []byte, lang *Language) {
	if root == nil || lang == nil || lang.Name != "comment" || root.Type(lang) != "source" {
		return
	}
	trimTrailingExtraTriviaRoot(root, source)
}

func normalizePascalTrailingExtraTrivia(root *Node, source []byte, lang *Language) {
	if root == nil || lang == nil || lang.Name != "pascal" || root.Type(lang) != "root" {
		return
	}
	trimTrailingExtraTriviaRoot(root, source)
}

func normalizeRSTTopLevelSectionEnd(root *Node, source []byte, lang *Language) {
	if root == nil || lang == nil || lang.Name != "rst" || root.Type(lang) != "document" || len(root.children) == 0 {
		return
	}
	trimTrailingExtraTriviaRoot(root, source)
	section := root.children[0]
	if section == nil || section.IsExtra() || section.Type(lang) != "section" {
		return
	}
	end := lastNonTriviaByteEnd(source)
	if end == 0 || end >= section.endByte {
		return
	}
	section.endByte = end
	section.endPoint = advancePointByBytes(Point{}, source[:end])
}

func bytesAreCooklangStepTail(b []byte) bool {
	sawPunctuation := false
	for _, c := range b {
		switch c {
		case '.', '!', '?':
			if sawPunctuation {
				return false
			}
			sawPunctuation = true
		case ' ', '\t', '\n', '\r':
		default:
			return false
		}
	}
	return sawPunctuation
}

func normalizeDModuleDefinitionBounds(root *Node, lang *Language) {
	if root == nil || lang == nil || lang.Name != "d" {
		return
	}
	var walk func(*Node)
	walk = func(n *Node) {
		if n == nil {
			return
		}
		if n.Type(lang) == "module_def" {
			if first := pythonBlockStartAnchor(n.children, lang); first != nil && n.startByte < first.startByte {
				n.startByte = first.startByte
				n.startPoint = first.startPoint
			}
			if last := pythonBlockEndAnchor(n.children); last != nil && n.endByte > last.endByte {
				n.endByte = last.endByte
				n.endPoint = last.endPoint
			}
		}
		for _, child := range n.children {
			walk(child)
		}
	}
	walk(root)
}

func normalizeDSourceFileLeadingTrivia(root *Node, source []byte, lang *Language) {
	if root == nil || lang == nil || lang.Name != "d" || root.Type(lang) != "source_file" || len(root.children) == 0 {
		return
	}
	first := root.children[0]
	if first == nil || root.startByte >= first.startByte || int(first.startByte) > len(source) {
		return
	}
	if !bytesAreTrivia(source[root.startByte:first.startByte]) {
		return
	}
	root.startByte = first.startByte
	root.startPoint = first.startPoint
}

func normalizeDVariableStorageClassWrappers(root *Node, lang *Language) {
	if root == nil || lang == nil || lang.Name != "d" {
		return
	}
	storageClassSym, ok := lang.SymbolByName("storage_class")
	if !ok {
		return
	}
	storageClassNamed := false
	if idx := int(storageClassSym); idx < len(lang.SymbolMetadata) {
		storageClassNamed = lang.SymbolMetadata[storageClassSym].Named
	}
	var walk func(*Node)
	walk = func(n *Node) {
		if n == nil {
			return
		}
		if n.Type(lang) == "variable_declaration" {
			for i, child := range n.children {
				if child == nil || child.Type(lang) != "static" {
					continue
				}
				wrapper := newParentNodeInArena(n.ownerArena, storageClassSym, storageClassNamed, []*Node{child}, nil, 0)
				n.children[i] = wrapper
			}
		}
		for _, child := range n.children {
			walk(child)
		}
	}
	walk(root)
}

func normalizeDCallExpressionTemplateTypes(root *Node, lang *Language) {
	if root == nil || lang == nil || lang.Name != "d" {
		return
	}
	typeSym, ok := lang.SymbolByName("type")
	if !ok {
		return
	}
	typeNamed := false
	if idx := int(typeSym); idx < len(lang.SymbolMetadata) {
		typeNamed = lang.SymbolMetadata[typeSym].Named
	}
	var walk func(*Node)
	walk = func(n *Node) {
		if n == nil {
			return
		}
		if n.Type(lang) == "call_expression" && len(n.children) > 0 {
			child := n.children[0]
			if child != nil && child.Type(lang) == "template_instance" {
				n.children[0] = newParentNodeInArena(n.ownerArena, typeSym, typeNamed, []*Node{child}, nil, 0)
			}
		}
		for _, child := range n.children {
			walk(child)
		}
	}
	walk(root)
}

func normalizeDVariableTypeQualifiers(root *Node, lang *Language) {
	if root == nil || lang == nil || lang.Name != "d" {
		return
	}
	var walk func(*Node)
	walk = func(n *Node) {
		if n == nil {
			return
		}
		if n.Type(lang) == "variable_declaration" && len(n.children) >= 3 {
			for i := 0; i+1 < len(n.children); i++ {
				left := n.children[i]
				right := n.children[i+1]
				if left == nil || right == nil || left.Type(lang) != "storage_class" || right.Type(lang) != "type" {
					continue
				}
				if len(left.children) != 1 || left.children[0] == nil || left.children[0].Type(lang) != "type_ctor" {
					continue
				}
				mergedType := cloneNodeInArena(n.ownerArena, right)
				mergedChildren := make([]*Node, 0, 1+len(right.children))
				mergedChildren = append(mergedChildren, left.children[0])
				mergedChildren = append(mergedChildren, right.children...)
				if n.ownerArena != nil {
					buf := n.ownerArena.allocNodeSlice(len(mergedChildren))
					copy(buf, mergedChildren)
					mergedChildren = buf
				}
				mergedType.children = mergedChildren
				mergedType.startByte = mergedChildren[0].startByte
				mergedType.startPoint = mergedChildren[0].startPoint
				n.children[i+1] = mergedType
				n.children = append(n.children[:i], n.children[i+1:]...)
				break
			}
		}
		for _, child := range n.children {
			walk(child)
		}
	}
	walk(root)
}

func normalizeDCallExpressionPropertyTypes(root *Node, lang *Language) {
	if root == nil || lang == nil || lang.Name != "d" {
		return
	}
	typeSym, ok := lang.SymbolByName("type")
	if !ok {
		return
	}
	typeNamed := false
	if idx := int(typeSym); idx < len(lang.SymbolMetadata) {
		typeNamed = lang.SymbolMetadata[typeSym].Named
	}
	var walk func(*Node)
	walk = func(n *Node) {
		if n == nil {
			return
		}
		if n.Type(lang) == "call_expression" && len(n.children) > 0 {
			child := n.children[0]
			if parts, ok := flattenDPropertyTypeChain(child, lang); ok {
				if n.ownerArena != nil {
					buf := n.ownerArena.allocNodeSlice(len(parts))
					copy(buf, parts)
					parts = buf
				}
				n.children[0] = newParentNodeInArena(n.ownerArena, typeSym, typeNamed, parts, nil, 0)
			}
		}
		for _, child := range n.children {
			walk(child)
		}
	}
	walk(root)
}

func normalizeDCallExpressionSimpleTypeCallees(root *Node, lang *Language) {
	if root == nil || lang == nil || lang.Name != "d" {
		return
	}
	var walk func(*Node)
	walk = func(n *Node) {
		if n == nil {
			return
		}
		if n.Type(lang) == "call_expression" && len(n.children) > 0 {
			child := n.children[0]
			if child != nil && child.Type(lang) == "type" && len(child.children) == 1 && child.children[0] != nil && child.children[0].Type(lang) == "identifier" {
				n.children[0] = child.children[0]
			}
		}
		for _, child := range n.children {
			walk(child)
		}
	}
	walk(root)
}

func normalizeCooklangTrailingStepTail(root *Node, source []byte, lang *Language) {
	if root == nil || lang == nil || lang.Name != "cooklang" || len(source) == 0 {
		return
	}
	if root.Type(lang) != "recipe" || len(root.children) != 1 {
		return
	}
	step := root.children[0]
	if step == nil || step.Type(lang) != "step" || step.endByte >= uint32(len(source)) {
		return
	}
	tail := source[step.endByte:]
	if !bytesAreCooklangStepTail(tail) {
		return
	}
	stepEnd := step.endByte
	for i := int(step.endByte); i < len(source); i++ {
		switch source[i] {
		case '.', '!', '?':
			stepEnd = uint32(i + 1)
		}
	}
	if stepEnd > step.endByte {
		extendNodeEndTo(step, stepEnd, source)
	}
	if root.endByte < uint32(len(source)) {
		extendNodeEndTo(root, uint32(len(source)), source)
	}
}

func lineBreakEndAt(source []byte, start, limit uint32) uint32 {
	if start >= limit || start >= uint32(len(source)) {
		return 0
	}
	switch source[start] {
	case '\n':
		return start + 1
	case '\r':
		if start+1 < limit && start+1 < uint32(len(source)) && source[start+1] == '\n' {
			return start + 2
		}
		return start + 1
	default:
		return 0
	}
}

func normalizeFortranStatementLineBreaks(root *Node, source []byte, lang *Language) {
	if root == nil || lang == nil || lang.Name != "fortran" || len(source) == 0 {
		return
	}
	var walk func(*Node, int)
	walk = func(n *Node, depth int) {
		if n == nil || depth > maxTreeWalkDepth {
			return
		}
		if n.Type(lang) == "program" {
			for i := 0; i+1 < len(n.children); i++ {
				cur := n.children[i]
				next := n.children[i+1]
				if cur == nil || next == nil || cur.endByte >= next.startByte {
					continue
				}
				if cur.Type(lang) != "program_statement" {
					continue
				}
				if end := lineBreakEndAt(source, cur.endByte, next.startByte); end > cur.endByte {
					extendNodeEndTo(cur, end, source)
				}
			}
		}
		for _, child := range n.children {
			walk(child, depth+1)
		}
	}
	walk(root, 0)
}

func normalizeNginxAttributeLineBreaks(root *Node, source []byte, lang *Language) {
	if root == nil || lang == nil || lang.Name != "nginx" || len(source) == 0 {
		return
	}
	var walk func(*Node, int)
	walk = func(n *Node, depth int) {
		if n == nil || depth > maxTreeWalkDepth {
			return
		}
		if n.Type(lang) == "attribute" {
			if end := lineBreakEndAt(source, n.endByte, uint32(len(source))); end > n.endByte {
				extendNodeEndTo(n, end, source)
			}
		}
		for _, child := range n.children {
			walk(child, depth+1)
		}
	}
	walk(root, 0)
}

func normalizeTopLevelTrailingLineBreakSpan(root *Node, source []byte, lang *Language) {
	if root == nil || lang == nil || len(source) == 0 {
		return
	}
	switch lang.Name {
	case "caddy", "fortran", "pug":
	default:
		return
	}
	if len(root.children) != 1 {
		return
	}
	child := root.children[0]
	if child == nil || child.endByte >= root.endByte || root.endByte > uint32(len(source)) {
		return
	}
	gap := source[child.endByte:root.endByte]
	if !bytesAreTrivia(gap) || !bytesContainLineBreak(gap) {
		return
	}
	extendNodeEndTo(child, root.endByte, source)
}

func normalizeRootEOFNewlineSpan(root *Node, source []byte, lang *Language) {
	if root == nil || lang == nil || len(source) == 0 || root.endByte >= uint32(len(source)) {
		return
	}
	switch {
	case lang.Name == "go" && root.Type(lang) == "source_file":
	case lang.Name == "scala" && root.Type(lang) == "compilation_unit":
	default:
		return
	}
	gap := source[root.endByte:]
	if !bytesAreTrivia(gap) || !bytesContainLineBreak(gap) {
		return
	}
	extendNodeEndTo(root, uint32(len(source)), source)
}

func normalizeHaskellImportsSpan(root *Node, source []byte, lang *Language) {
	if root == nil || len(root.children) < 2 || len(source) == 0 || lang == nil || lang.Name != "haskell" {
		return
	}
	for i := 0; i+1 < len(root.children); i++ {
		left := root.children[i]
		right := root.children[i+1]
		if left == nil || right == nil {
			continue
		}
		if left.Type(lang) != "imports" {
			continue
		}
		if left.endByte >= right.startByte {
			continue
		}
		if left.endByte > uint32(len(source)) || right.startByte > uint32(len(source)) {
			continue
		}
		gap := source[left.endByte:right.startByte]
		if !bytesAreTrivia(gap) {
			continue
		}
		left.endByte = right.startByte
		left.endPoint = advancePointByBytes(left.endPoint, gap)
	}
}

func normalizeHaskellZeroWidthTokens(root *Node, lang *Language) {
	if root == nil || lang == nil || lang.Name != "haskell" || len(root.children) == 0 {
		return
	}
	filtered := root.children[:0]
	for _, child := range root.children {
		if child == nil {
			continue
		}
		if child.Type(lang) == "_token1" && child.startByte == child.endByte {
			continue
		}
		filtered = append(filtered, child)
	}
	root.children = filtered
}

func normalizeHaskellRootImportField(root *Node, lang *Language) {
	if root == nil || lang == nil || lang.Name != "haskell" || len(root.children) == 0 {
		return
	}
	if len(lang.FieldNames) == 0 {
		return
	}
	for i, child := range root.children {
		if child == nil {
			continue
		}
		fid := FieldID(0)
		for j, name := range lang.FieldNames {
			if name == child.Type(lang) {
				fid = FieldID(j)
				break
			}
		}
		if fid == 0 {
			continue
		}
		if len(root.fieldIDs) < len(root.children) {
			fieldIDs := make([]FieldID, len(root.children))
			copy(fieldIDs, root.fieldIDs)
			root.fieldIDs = fieldIDs
		}
		if len(root.fieldSources) < len(root.children) {
			fieldSources := make([]uint8, len(root.children))
			copy(fieldSources, root.fieldSources)
			root.fieldSources = fieldSources
		}
		root.fieldIDs[i] = fid
		root.fieldSources[i] = fieldSourceInherited
	}
}

func normalizeHaskellDeclarationsSpan(root *Node, source []byte, lang *Language) {
	if root == nil || lang == nil || lang.Name != "haskell" || len(source) == 0 {
		return
	}
	for _, child := range root.children {
		if child == nil || child.Type(lang) != "declarations" {
			continue
		}
		if child.endByte >= root.endByte || root.endByte > uint32(len(source)) {
			continue
		}
		gap := source[child.endByte:root.endByte]
		if !bytesAreTrivia(gap) {
			continue
		}
		extendNodeEndTo(child, root.endByte, source)
	}
}

func normalizeHaskellLocalBindsStarts(root *Node, source []byte, lang *Language) {
	if root == nil || lang == nil || lang.Name != "haskell" || len(source) == 0 {
		return
	}
	var walk func(*Node)
	walk = func(n *Node) {
		if n == nil {
			return
		}
		if n.Type(lang) == "let_in" && len(n.children) >= 2 {
			letNode := n.children[0]
			localBinds := n.children[1]
			if letNode != nil && localBinds != nil && letNode.Type(lang) == "let" && localBinds.Type(lang) == "local_binds" && letNode.endByte < localBinds.startByte && localBinds.startByte <= uint32(len(source)) {
				gap := source[letNode.endByte:localBinds.startByte]
				if len(gap) > 0 && bytesAreTrivia(gap) && !bytesContainLineBreak(gap) {
					localBinds.startByte = letNode.endByte
					localBinds.startPoint = letNode.endPoint
				}
			}
		}
		for _, child := range n.children {
			walk(child)
		}
	}
	walk(root)
}

func normalizeHaskellQuasiquoteStarts(root *Node, source []byte, lang *Language) {
	if root == nil || lang == nil || lang.Name != "haskell" || len(source) == 0 {
		return
	}
	var walk func(*Node)
	walk = func(n *Node) {
		if n == nil {
			return
		}
		if n.Type(lang) == "quasiquote" && n.startByte > 0 {
			start := int(n.startByte)
			if source[start-1] == ' ' && start < len(source) && source[start] == '[' {
				n.startByte--
				if n.startPoint.Column > 0 {
					n.startPoint.Column--
				} else if n.startPoint.Row > 0 {
					n.startPoint = advancePointByBytes(Point{}, source[:n.startByte])
				}
			}
		}
		for _, child := range n.children {
			walk(child)
		}
	}
	walk(root)
}

func flattenDPropertyTypeChain(n *Node, lang *Language) ([]*Node, bool) {
	if n == nil || lang == nil {
		return nil, false
	}
	switch n.Type(lang) {
	case "identifier":
		return []*Node{n}, true
	case "property_expression":
		if len(n.children) != 3 || n.children[1] == nil || n.children[2] == nil {
			return nil, false
		}
		if n.children[1].Type(lang) != "." || n.children[2].Type(lang) != "identifier" {
			return nil, false
		}
		left, ok := flattenDPropertyTypeChain(n.children[0], lang)
		if !ok {
			return nil, false
		}
		out := make([]*Node, 0, len(left)+2)
		out = append(out, left...)
		out = append(out, n.children[1], n.children[2])
		return out, true
	default:
		return nil, false
	}
}
