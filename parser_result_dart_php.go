package gotreesitter

func normalizePHPSingletonTypeWrappers(root *Node, lang *Language) {
	if root == nil || lang == nil || lang.Name != "php" {
		return
	}
	var walk func(*Node)
	walk = func(n *Node) {
		if n == nil {
			return
		}
		for i, child := range n.children {
			if child == nil {
				continue
			}
			switch child.Type(lang) {
			case "intersection_type", "union_type":
				if len(child.children) == 1 && child.children[0] != nil && child.children[0].IsNamed() {
					n.children[i] = child.children[0]
					child = n.children[i]
				}
			}
			walk(child)
		}
	}
	walk(root)
}

func normalizeDartSingleTypeArgumentFreeCalls(root *Node, lang *Language) {
	if root == nil || lang == nil || lang.Name != "dart" {
		return
	}
	relExprSym, ok := lang.SymbolByName("relational_expression")
	if !ok {
		return
	}
	relOpSym, ok := lang.SymbolByName("relational_operator")
	if !ok {
		return
	}
	parenSym, ok := lang.SymbolByName("parenthesized_expression")
	if !ok {
		return
	}
	relExprNamed := false
	if idx := int(relExprSym); idx < len(lang.SymbolMetadata) {
		relExprNamed = lang.SymbolMetadata[relExprSym].Named
	}
	relOpNamed := false
	if idx := int(relOpSym); idx < len(lang.SymbolMetadata) {
		relOpNamed = lang.SymbolMetadata[relOpSym].Named
	}
	parenNamed := false
	if idx := int(parenSym); idx < len(lang.SymbolMetadata) {
		parenNamed = lang.SymbolMetadata[parenSym].Named
	}

	var walk func(*Node)
	walk = func(n *Node) {
		if n == nil {
			return
		}
		for i := 0; i+1 < len(n.children); i++ {
			if rewriteDartSingleTypeArgumentFreeCall(n, i, lang, relExprSym, relExprNamed, relOpSym, relOpNamed, parenSym, parenNamed) {
				break
			}
		}
		for _, child := range n.children {
			walk(child)
		}
	}
	walk(root)
}

func normalizeDartConstructorSignatureKinds(root *Node, source []byte, lang *Language) {
	if root == nil || lang == nil || lang.Name != "dart" {
		return
	}
	constructorSym, ok := lang.SymbolByName("constructor_signature")
	if !ok {
		return
	}
	parametersID, _ := lang.FieldByName("parameters")
	constructorNamed := false
	if idx := int(constructorSym); idx < len(lang.SymbolMetadata) {
		constructorNamed = lang.SymbolMetadata[constructorSym].Named
	}
	var walk func(*Node)
	walk = func(n *Node) {
		if n == nil {
			return
		}
		if n.Type(lang) == "class_definition" {
			className := n.ChildByFieldName("name", lang)
			body := n.ChildByFieldName("body", lang)
			if className != nil && body != nil {
				classText := className.Text(source)
				for _, member := range body.children {
					if member == nil || member.Type(lang) != "method_signature" || len(member.children) != 1 {
						continue
					}
					sig := member.children[0]
					if sig == nil || sig.Type(lang) != "function_signature" || len(sig.children) != 2 {
						continue
					}
					name := sig.children[0]
					params := sig.children[1]
					if name == nil || params == nil || name.Type(lang) != "identifier" || params.Type(lang) != "formal_parameter_list" {
						continue
					}
					if name.Text(source) != classText {
						continue
					}
					sig.symbol = constructorSym
					sig.isNamed = constructorNamed
					if len(sig.fieldIDs) != len(sig.children) {
						ensureNodeFieldStorage(sig, len(sig.children))
					}
					if parametersID != 0 && len(sig.fieldIDs) > 1 {
						sig.fieldIDs[1] = parametersID
						if len(sig.fieldSources) == len(sig.children) {
							sig.fieldSources[1] = fieldSourceDirect
						}
					}
				}
			}
		}
		for _, child := range n.children {
			walk(child)
		}
	}
	walk(root)
}

func normalizeDartSwitchExpressionBodyFields(root *Node, lang *Language) {
	if root == nil || lang == nil || lang.Name != "dart" {
		return
	}
	bodyID, ok := lang.FieldByName("body")
	if !ok {
		return
	}
	var walk func(*Node)
	walk = func(n *Node) {
		if n == nil {
			return
		}
		if n.Type(lang) == "switch_expression" && len(n.children) > 0 {
			ensureNodeFieldStorage(n, len(n.children))
			start := -1
			for i := 0; i < len(n.children); i++ {
				if n.fieldIDs[i] == bodyID {
					start = i
					break
				}
			}
			if start >= 0 {
				for i := start; i < len(n.children); i++ {
					if n.children[i] == nil {
						continue
					}
					n.fieldIDs[i] = bodyID
					if len(n.fieldSources) == len(n.children) {
						n.fieldSources[i] = fieldSourceDirect
					}
				}
			}
		}
		for _, child := range n.children {
			walk(child)
		}
	}
	walk(root)
}

func normalizeMakeConditionalConsequenceFields(root *Node, lang *Language) {
	if root == nil || lang == nil || lang.Name != "make" {
		return
	}
	consequenceID, ok := lang.FieldByName("consequence")
	if !ok {
		return
	}
	var walk func(*Node)
	walk = func(n *Node) {
		if n == nil {
			return
		}
		switch n.Type(lang) {
		case "conditional", "elsif_directive", "else_directive":
			ensureNodeFieldStorage(n, len(n.children))
			start, end := -1, -1
			for i := 0; i < len(n.children); i++ {
				if n.fieldIDs[i] != consequenceID {
					continue
				}
				if start < 0 {
					start = i
				}
				end = i
			}
			if start >= 0 && end >= start {
				for start > 0 {
					prev := n.children[start-1]
					if prev == nil || prev.isNamed || prev.isExtra || prev.Type(lang) != "\t" {
						break
					}
					start--
				}
				for i := start; i <= end; i++ {
					if n.children[i] == nil {
						continue
					}
					n.fieldIDs[i] = consequenceID
					if len(n.fieldSources) == len(n.children) {
						n.fieldSources[i] = fieldSourceDirect
					}
				}
			}
		}
		for _, child := range n.children {
			walk(child)
		}
	}
	walk(root)
}

func normalizeIniSectionStarts(root *Node, lang *Language) {
	if root == nil || lang == nil || lang.Name != "ini" {
		return
	}
	var walk func(*Node)
	walk = func(n *Node) {
		if n == nil {
			return
		}
		if n.Type(lang) == "section" {
			for _, child := range n.children {
				if child == nil {
					continue
				}
				if n.startByte < child.startByte {
					n.startByte = child.startByte
					n.startPoint = child.startPoint
				}
				break
			}
		}
		for _, child := range n.children {
			walk(child)
		}
	}
	walk(root)
}

func normalizeZigEmptyInitListFields(root *Node, lang *Language) {
	if root == nil || lang == nil || lang.Name != "zig" {
		return
	}
	fieldConstantID, ok := lang.FieldByName("field_constant")
	if !ok {
		return
	}
	var walk func(*Node)
	walk = func(n *Node) {
		if n == nil {
			return
		}
		if len(n.fieldIDs) == len(n.children) {
			for i, child := range n.children {
				if child == nil || n.fieldIDs[i] != fieldConstantID || child.Type(lang) != "InitList" {
					continue
				}
				if n.Type(lang) != "SuffixExpr" || len(n.children) != 2 || i != 1 || n.children[0] == nil || n.children[0].Type(lang) != "." {
					continue
				}
				n.fieldIDs[i] = 0
				if len(n.fieldSources) == len(n.children) {
					n.fieldSources[i] = fieldSourceNone
				}
			}
		}
		for _, child := range n.children {
			walk(child)
		}
	}
	walk(root)
}

func ensureNodeFieldStorage(n *Node, childCount int) {
	if n == nil || childCount <= 0 {
		return
	}
	if len(n.fieldIDs) != childCount {
		fieldIDs := make([]FieldID, childCount)
		copy(fieldIDs, n.fieldIDs)
		if n.ownerArena != nil {
			buf := n.ownerArena.allocFieldIDSlice(childCount)
			copy(buf, fieldIDs)
			fieldIDs = buf
		}
		n.fieldIDs = fieldIDs
	}
	if len(n.fieldSources) != childCount {
		fieldSources := make([]uint8, childCount)
		copy(fieldSources, n.fieldSources)
		n.fieldSources = fieldSources
	}
}

func rewriteDartSingleTypeArgumentFreeCall(parent *Node, idx int, lang *Language, relExprSym Symbol, relExprNamed bool, relOpSym Symbol, relOpNamed bool, parenSym Symbol, parenNamed bool) bool {
	if parent == nil || idx < 0 || idx+1 >= len(parent.children) || lang == nil {
		return false
	}
	callee := parent.children[idx]
	selector := parent.children[idx+1]
	if callee == nil || selector == nil || callee.Type(lang) != "identifier" || selector.Type(lang) != "selector" || len(selector.children) != 1 {
		return false
	}
	argPart := selector.children[0]
	if argPart == nil || argPart.Type(lang) != "argument_part" || len(argPart.children) != 2 {
		return false
	}
	typeArgs := argPart.children[0]
	args := argPart.children[1]
	if typeArgs == nil || args == nil || typeArgs.Type(lang) != "type_arguments" || args.Type(lang) != "arguments" {
		return false
	}
	typeIdent, lt, gt, ok := dartSimpleTypeArgumentParts(typeArgs, lang)
	if !ok {
		return false
	}
	if len(args.children) < 2 {
		return false
	}

	arena := parent.ownerArena
	if typeIdent.Type(lang) == "type_identifier" {
		identSym, ok := lang.SymbolByName("identifier")
		if !ok {
			return false
		}
		identNamed := false
		if idx := int(identSym); idx < len(lang.SymbolMetadata) {
			identNamed = lang.SymbolMetadata[identSym].Named
		}
		typeIdent = newLeafNodeInArena(arena, identSym, identNamed, typeIdent.startByte, typeIdent.endByte, typeIdent.startPoint, typeIdent.endPoint)
	}
	lessOp := newParentNodeInArena(arena, relOpSym, relOpNamed, []*Node{lt}, nil, 0)
	left := newParentNodeInArena(arena, relExprSym, relExprNamed, []*Node{callee, lessOp, typeIdent}, nil, 0)
	greaterOp := newParentNodeInArena(arena, relOpSym, relOpNamed, []*Node{gt}, nil, 0)
	parenChildren := dartParenthesizedExpressionChildren(args, lang)
	paren := newParentNodeInArena(arena, parenSym, parenNamed, parenChildren, nil, args.productionID)
	outer := newParentNodeInArena(arena, relExprSym, relExprNamed, []*Node{left, greaterOp, paren}, nil, 0)
	replaceChildRangeWithSingleNode(parent, idx, idx+2, outer)
	return true
}

func dartSimpleTypeArgumentParts(typeArgs *Node, lang *Language) (*Node, *Node, *Node, bool) {
	if typeArgs == nil || lang == nil || typeArgs.Type(lang) != "type_arguments" || len(typeArgs.children) < 3 {
		return nil, nil, nil, false
	}
	lt := typeArgs.children[0]
	gt := typeArgs.children[len(typeArgs.children)-1]
	if lt == nil || gt == nil || lt.Type(lang) != "<" || gt.Type(lang) != ">" {
		return nil, nil, nil, false
	}
	if got := typeArgs.NamedChildCount(); got != 1 {
		return nil, nil, nil, false
	}
	typeIdent := typeArgs.NamedChild(0)
	if typeIdent == nil || typeIdent.Type(lang) != "type_identifier" || nodeContainsNamedType(typeIdent, lang, "type_arguments") {
		return nil, nil, nil, false
	}
	return typeIdent, lt, gt, true
}

func nodeContainsNamedType(root *Node, lang *Language, want string) bool {
	if root == nil || lang == nil {
		return false
	}
	for _, child := range root.children {
		if child == nil {
			continue
		}
		if child.Type(lang) == want {
			return true
		}
		if nodeContainsNamedType(child, lang, want) {
			return true
		}
	}
	return false
}

func replaceChildRangeWithSingleNode(parent *Node, start, end int, replacement *Node) {
	if parent == nil || replacement == nil || start < 0 || start >= end || end > len(parent.children) {
		return
	}
	oldLen := len(parent.children)
	newChildren := make([]*Node, 0, oldLen-(end-start)+1)
	newChildren = append(newChildren, parent.children[:start]...)
	newChildren = append(newChildren, replacement)
	newChildren = append(newChildren, parent.children[end:]...)
	parent.children = newChildren

	if len(parent.fieldIDs) == oldLen {
		newFieldIDs := make([]FieldID, 0, len(newChildren))
		newFieldIDs = append(newFieldIDs, parent.fieldIDs[:start]...)
		mergedField := FieldID(0)
		for i := start; i < end; i++ {
			if parent.fieldIDs[i] != 0 {
				mergedField = parent.fieldIDs[i]
				break
			}
		}
		newFieldIDs = append(newFieldIDs, mergedField)
		newFieldIDs = append(newFieldIDs, parent.fieldIDs[end:]...)
		parent.fieldIDs = newFieldIDs
	}
	if len(parent.fieldSources) == oldLen {
		newFieldSources := make([]uint8, 0, len(newChildren))
		newFieldSources = append(newFieldSources, parent.fieldSources[:start]...)
		mergedSource := uint8(fieldSourceNone)
		for i := start; i < end; i++ {
			if parent.fieldSources[i] != fieldSourceNone {
				mergedSource = parent.fieldSources[i]
				break
			}
		}
		newFieldSources = append(newFieldSources, mergedSource)
		newFieldSources = append(newFieldSources, parent.fieldSources[end:]...)
		parent.fieldSources = newFieldSources
	}
	for i, child := range parent.children {
		if child == nil {
			continue
		}
		child.parent = parent
		child.childIndex = i
	}
}

func dartParenthesizedExpressionChildren(args *Node, lang *Language) []*Node {
	if args == nil || lang == nil {
		return nil
	}
	if len(args.children) != 3 {
		return append([]*Node(nil), args.children...)
	}
	open := args.children[0]
	mid := args.children[1]
	close := args.children[2]
	if open == nil || mid == nil || close == nil {
		return append([]*Node(nil), args.children...)
	}
	if mid.Type(lang) != "argument" || len(mid.children) != 1 || mid.children[0] == nil {
		return append([]*Node(nil), args.children...)
	}
	return []*Node{open, mid.children[0], close}
}

func normalizePHPStaticFunctionFragments(root *Node, source []byte, lang *Language) {
	if root == nil || lang == nil || lang.Name != "php" || len(root.children) == 0 {
		return
	}
	rootType := root.Type(lang)
	if rootType != "program" && rootType != "ERROR" {
		return
	}
	children := root.children
	changed := false
	if children[0] != nil && ((rootType == "program" && children[0].Type(lang) == rootType) || (rootType == "ERROR" && children[0].Type(lang) == "program")) {
		flat := make([]*Node, 0, len(children[0].children)+len(children)-1)
		flat = append(flat, children[0].children...)
		flat = append(flat, children[1:]...)
		children = flat
		changed = true
	}
	arena := root.ownerArena
	out := make([]*Node, 0, len(children))
	seenNonExtra := false
	for i := 0; i < len(children); {
		if repl, consumed, ok := rewritePHPStaticAnonymousHeaderWithTrailingArrowFragments(children[i:], source, lang, arena); ok {
			out = append(out, repl...)
			i += consumed
			changed = true
			for _, n := range repl {
				if phpCountsAsPriorTopLevelNode(n, lang) {
					seenNonExtra = true
				}
			}
			continue
		}
		if repl, consumed, ok := rewritePHPStaticNamedFunctionFragmentsWithTrailingMalformedSibling(children[i:], source, lang, arena, seenNonExtra); ok {
			out = append(out, repl...)
			i += consumed
			changed = true
			for _, n := range repl {
				if phpCountsAsPriorTopLevelNode(n, lang) {
					seenNonExtra = true
				}
			}
			continue
		}
		if repl, consumed, ok := rewritePHPStaticNamedFunctionFragments(children[i:], source, lang, arena, seenNonExtra); ok {
			out = append(out, repl...)
			i += consumed
			changed = true
			for _, n := range repl {
				if phpCountsAsPriorTopLevelNode(n, lang) {
					seenNonExtra = true
				}
			}
			continue
		}
		if repl, consumed, ok := rewritePHPStaticAnonymousFunctionFragments(children[i:], source, lang, arena); ok {
			out = append(out, repl...)
			i += consumed
			changed = true
			for _, n := range repl {
				if phpCountsAsPriorTopLevelNode(n, lang) {
					seenNonExtra = true
				}
			}
			continue
		}
		out = append(out, children[i])
		if phpCountsAsPriorTopLevelNode(children[i], lang) {
			seenNonExtra = true
		}
		i++
	}
	if !changed {
		return
	}
	if arena != nil {
		buf := arena.allocNodeSlice(len(out))
		copy(buf, out)
		out = buf
	}
	root.children = out
	root.fieldIDs = nil
	root.fieldSources = nil
	assignPHPTopLevelFragmentFields(root, lang, arena)
	populateParentNode(root, out)
	extendNodeToTrailingWhitespace(root, source)
}

func rewritePHPStaticAnonymousHeaderWithTrailingArrowFragments(nodes []*Node, source []byte, lang *Language, arena *nodeArena) ([]*Node, int, bool) {
	if len(nodes) < 4 {
		return nil, 0, false
	}
	headerErr := nodes[0]
	openBrace := nodes[1]
	body := nodes[2]
	arrowStmt := nodes[3]
	if headerErr == nil || openBrace == nil || body == nil || arrowStmt == nil {
		return nil, 0, false
	}
	if headerErr.Type(lang) != "ERROR" || len(headerErr.children) != 1 || headerErr.children[0] == nil || headerErr.children[0].Type(lang) != "_anonymous_function_header" {
		return nil, 0, false
	}
	header := headerErr.children[0]
	if len(header.children) != 3 || header.children[0] == nil || header.children[1] == nil || header.children[2] == nil {
		return nil, 0, false
	}
	if header.children[0].Type(lang) != "static_modifier" || header.children[1].Type(lang) != "function" || header.children[2].Type(lang) != "formal_parameters" {
		return nil, 0, false
	}
	if openBrace.Type(lang) != "{" || body.Type(lang) != "compound_statement" || len(body.children) < 2 {
		return nil, 0, false
	}
	closeBrace := body.children[0]
	if closeBrace == nil || closeBrace.Type(lang) != "}" {
		return nil, 0, false
	}
	var trailingComment *Node
	var suffixStart uint32
	switch {
	case len(body.children) >= 3 && body.children[1] != nil && body.children[1].Type(lang) == "comment" && body.children[2] != nil:
		trailingComment = body.children[1]
		suffixStart = body.children[2].startByte
	case len(body.children) >= 2 && body.children[1] != nil:
		suffixStart = body.children[1].startByte
	default:
		return nil, 0, false
	}
	if arrowStmt.Type(lang) != "statement" || suffixStart == 0 || int(suffixStart) >= len(source) {
		return nil, 0, false
	}

	closeErrChildren := phpAllocChildren(arena, 1)
	closeErrChildren[0] = closeBrace
	closeErr := newParentNodeInArena(arena, errorSymbol, true, closeErrChildren, nil, 0)
	closeErr.hasError = true
	closeErr.isExtra = true

	prefixLen := 5
	if trailingComment != nil {
		prefixLen++
	}
	prefix := phpAllocChildren(arena, prefixLen)
	prefix[0] = header.children[0]
	prefix[1] = header.children[1]
	prefix[2] = header.children[2]
	prefix[3] = openBrace
	prefix[4] = closeErr
	if trailingComment != nil {
		prefix[5] = trailingComment
	}

	suffix, ok := phpReparsedTopLevelSuffix(source, suffixStart, lang, arena)
	if !ok {
		return nil, 0, false
	}
	combined := phpAllocChildren(arena, len(prefix)+len(suffix))
	copy(combined, prefix)
	copy(combined[len(prefix):], suffix)
	return combined, len(nodes), true
}

func rewritePHPStaticNamedFunctionFragments(nodes []*Node, source []byte, lang *Language, arena *nodeArena, hasPriorNonExtra bool) ([]*Node, int, bool) {
	if len(nodes) < 3 {
		return nil, 0, false
	}
	staticErr := nodes[0]
	header := nodes[1]
	bodyErr := nodes[2]
	if staticErr == nil || header == nil || bodyErr == nil {
		return nil, 0, false
	}
	if staticErr.Type(lang) != "ERROR" || len(staticErr.children) != 1 || staticErr.children[0] == nil || staticErr.children[0].Type(lang) != "static_modifier" {
		return nil, 0, false
	}
	if header.Type(lang) != "_anonymous_function_header" || len(header.children) != 3 {
		return nil, 0, false
	}
	if header.children[0] == nil || header.children[0].Type(lang) != "function" {
		return nil, 0, false
	}
	if header.children[1] == nil || header.children[1].Type(lang) != "ERROR" {
		return nil, 0, false
	}
	if header.children[2] == nil || header.children[2].Type(lang) != "formal_parameters" {
		return nil, 0, false
	}
	body, ok := phpSyntheticCompoundStatementFromError(bodyErr, source, lang, arena)
	if !ok {
		return nil, 0, false
	}
	nameNode, ok := phpSyntheticNamedFunctionName(header.children[1], lang, arena)
	if !ok {
		return nil, 0, false
	}
	args, ok := phpSyntheticArgumentsFromFormals(header.children[2], lang, arena)
	if !ok {
		return nil, 0, false
	}
	callSym, callNamed, ok := phpSymbolMeta(lang, "function_call_expression")
	if !ok {
		return nil, 0, false
	}
	callChildren := phpAllocChildren(arena, 2)
	callChildren[0] = nameNode
	callChildren[1] = args
	call := newParentNodeInArena(arena, callSym, callNamed, callChildren, phpSyntheticFieldIDs(arena, 2, lang, map[int]string{
		0: "function",
		1: "arguments",
	}), 0)

	errChildren := phpAllocChildren(arena, 3)
	errChildren[0] = staticErr.children[0]
	errChildren[1] = header.children[0]
	errChildren[2] = call
	if hasPriorNonExtra {
		errChildren = errChildren[:2]
		errNode := newParentNodeInArena(arena, errorSymbol, true, errChildren, nil, 0)
		errNode.hasError = true
		errNode.isExtra = true

		semiSym, ok := lang.SymbolByName(";")
		if !ok {
			return nil, 0, false
		}
		semi := newLeafNodeInArena(arena, semiSym, false, call.endByte, call.endByte, call.endPoint, call.endPoint)
		semi.hasError = true

		exprSym, exprNamed, ok := phpSymbolMeta(lang, "expression_statement")
		if !ok {
			return nil, 0, false
		}
		exprChildren := phpAllocChildren(arena, 2)
		exprChildren[0] = call
		exprChildren[1] = semi
		expr := newParentNodeInArena(arena, exprSym, exprNamed, exprChildren, nil, 0)

		repl := phpAllocChildren(arena, 3)
		repl[0] = errNode
		repl[1] = expr
		repl[2] = body
		if suffix, ok := phpReparsedTopLevelSuffix(source, body.endByte, lang, arena); ok {
			combined := phpAllocChildren(arena, len(repl)+len(suffix))
			copy(combined, repl)
			copy(combined[len(repl):], suffix)
			return combined, len(nodes), true
		}
		return repl, 3, true
	}

	errNode := newParentNodeInArena(arena, errorSymbol, true, errChildren, nil, 0)
	errNode.hasError = true
	errNode.isExtra = true

	repl := phpAllocChildren(arena, 2)
	repl[0] = errNode
	repl[1] = body
	if suffix, ok := phpReparsedTopLevelSuffix(source, body.endByte, lang, arena); ok {
		combined := phpAllocChildren(arena, len(repl)+len(suffix))
		copy(combined, repl)
		copy(combined[len(repl):], suffix)
		return combined, len(nodes), true
	}
	return repl, 3, true
}

func rewritePHPStaticNamedFunctionFragmentsWithTrailingMalformedSibling(nodes []*Node, source []byte, lang *Language, arena *nodeArena, hasPriorNonExtra bool) ([]*Node, int, bool) {
	if len(nodes) < 3 {
		return nil, 0, false
	}
	staticErr := nodes[0]
	header := nodes[1]
	bodyCarrier := nodes[2]
	if staticErr == nil || header == nil || bodyCarrier == nil {
		return nil, 0, false
	}
	if staticErr.Type(lang) != "ERROR" || len(staticErr.children) != 1 || staticErr.children[0] == nil || staticErr.children[0].Type(lang) != "static_modifier" {
		return nil, 0, false
	}
	if header.Type(lang) != "_anonymous_function_header" || len(header.children) != 3 {
		return nil, 0, false
	}
	if header.children[0] == nil || header.children[0].Type(lang) != "function" {
		return nil, 0, false
	}
	if header.children[1] == nil || header.children[1].Type(lang) != "ERROR" {
		return nil, 0, false
	}
	if header.children[2] == nil || header.children[2].Type(lang) != "formal_parameters" {
		return nil, 0, false
	}
	if bodyCarrier.Type(lang) != "_anonymous_function_header" && bodyCarrier.Type(lang) != "_arrow_function_header" {
		return nil, 0, false
	}
	if len(bodyCarrier.children) == 0 || bodyCarrier.children[0] == nil || bodyCarrier.children[0].Type(lang) != "ERROR" {
		return nil, 0, false
	}
	body, ok := phpSyntheticCompoundStatementFromError(bodyCarrier.children[0], source, lang, arena)
	if !ok {
		return nil, 0, false
	}
	nameNode, ok := phpSyntheticNamedFunctionName(header.children[1], lang, arena)
	if !ok {
		return nil, 0, false
	}
	args, ok := phpSyntheticArgumentsFromFormals(header.children[2], lang, arena)
	if !ok {
		return nil, 0, false
	}
	callSym, callNamed, ok := phpSymbolMeta(lang, "function_call_expression")
	if !ok {
		return nil, 0, false
	}
	callChildren := phpAllocChildren(arena, 2)
	callChildren[0] = nameNode
	callChildren[1] = args
	call := newParentNodeInArena(arena, callSym, callNamed, callChildren, phpSyntheticFieldIDs(arena, 2, lang, map[int]string{
		0: "function",
		1: "arguments",
	}), 0)

	errChildren := phpAllocChildren(arena, 3)
	errChildren[0] = staticErr.children[0]
	errChildren[1] = header.children[0]
	errChildren[2] = call
	var repl []*Node
	if hasPriorNonExtra {
		errChildren = errChildren[:2]
		errNode := newParentNodeInArena(arena, errorSymbol, true, errChildren, nil, 0)
		errNode.hasError = true
		errNode.isExtra = true

		semiSym, ok := lang.SymbolByName(";")
		if !ok {
			return nil, 0, false
		}
		semi := newLeafNodeInArena(arena, semiSym, false, call.endByte, call.endByte, call.endPoint, call.endPoint)
		semi.hasError = true

		exprSym, exprNamed, ok := phpSymbolMeta(lang, "expression_statement")
		if !ok {
			return nil, 0, false
		}
		exprChildren := phpAllocChildren(arena, 2)
		exprChildren[0] = call
		exprChildren[1] = semi
		expr := newParentNodeInArena(arena, exprSym, exprNamed, exprChildren, nil, 0)

		repl = phpAllocChildren(arena, 3)
		repl[0] = errNode
		repl[1] = expr
		repl[2] = body
	} else {
		errNode := newParentNodeInArena(arena, errorSymbol, true, errChildren, nil, 0)
		errNode.hasError = true
		errNode.isExtra = true
		repl = phpAllocChildren(arena, 2)
		repl[0] = errNode
		repl[1] = body
	}
	suffix, ok := phpReparsedTopLevelSuffix(source, body.endByte, lang, arena)
	if !ok {
		return nil, 0, false
	}
	combined := phpAllocChildren(arena, len(repl)+len(suffix))
	copy(combined, repl)
	copy(combined[len(repl):], suffix)
	return combined, len(nodes), true
}

func rewritePHPStaticAnonymousFunctionFragments(nodes []*Node, source []byte, lang *Language, arena *nodeArena) ([]*Node, int, bool) {
	if len(nodes) < 3 {
		return nil, 0, false
	}
	errNode := nodes[0]
	openBrace := nodes[1]
	closeBrace := nodes[2]
	if errNode == nil || openBrace == nil || closeBrace == nil {
		return nil, 0, false
	}
	if errNode.Type(lang) != "ERROR" || len(errNode.children) != 1 || errNode.children[0] == nil || errNode.children[0].Type(lang) != "_anonymous_function_header" {
		return nil, 0, false
	}
	header := errNode.children[0]
	if len(header.children) != 3 || header.children[0] == nil || header.children[1] == nil || header.children[2] == nil {
		return nil, 0, false
	}
	if header.children[0].Type(lang) != "static_modifier" || header.children[1].Type(lang) != "function" || header.children[2].Type(lang) != "formal_parameters" {
		return nil, 0, false
	}
	if openBrace.Type(lang) != "{" || closeBrace.Type(lang) != "}" {
		return nil, 0, false
	}
	compoundSym, compoundNamed, ok := phpSymbolMeta(lang, "compound_statement")
	if !ok {
		return nil, 0, false
	}
	bodyChildren := phpAllocChildren(arena, 2)
	bodyChildren[0] = openBrace
	bodyChildren[1] = closeBrace
	body := newParentNodeInArena(arena, compoundSym, compoundNamed, bodyChildren, nil, 0)

	anonSym, anonNamed, ok := phpSymbolMeta(lang, "anonymous_function")
	if !ok {
		return nil, 0, false
	}
	anonChildren := phpAllocChildren(arena, 4)
	anonChildren[0] = header.children[0]
	anonChildren[1] = header.children[1]
	anonChildren[2] = header.children[2]
	anonChildren[3] = body
	anon := newParentNodeInArena(arena, anonSym, anonNamed, anonChildren, phpSyntheticFieldIDs(arena, 4, lang, map[int]string{
		0: "static_modifier",
		2: "parameters",
		3: "body",
	}), 0)

	extraCount := 0
	for 3+extraCount < len(nodes) {
		next := nodes[3+extraCount]
		if next == nil || !next.isExtra {
			break
		}
		extraCount++
	}

	semiSym, ok := lang.SymbolByName(";")
	if !ok {
		return nil, 0, false
	}
	semiStartByte := closeBrace.endByte
	semiStartPoint := closeBrace.endPoint
	if extraCount > 0 {
		lastExtra := nodes[3+extraCount-1]
		semiStartByte = lastExtra.endByte
		semiStartPoint = lastExtra.endPoint
	}
	semi := newLeafNodeInArena(arena, semiSym, false, semiStartByte, semiStartByte, semiStartPoint, semiStartPoint)
	semi.hasError = true

	exprSym, exprNamed, ok := phpSymbolMeta(lang, "expression_statement")
	if !ok {
		return nil, 0, false
	}
	exprChildren := phpAllocChildren(arena, 2+extraCount)
	exprChildren[0] = anon
	for i := 0; i < extraCount; i++ {
		exprChildren[1+i] = nodes[3+i]
	}
	exprChildren[len(exprChildren)-1] = semi
	expr := newParentNodeInArena(arena, exprSym, exprNamed, exprChildren, nil, 0)

	repl := phpAllocChildren(arena, 1)
	repl[0] = expr
	return repl, 3 + extraCount, true
}

func phpSyntheticNamedFunctionName(errNode *Node, lang *Language, arena *nodeArena) (*Node, bool) {
	if errNode == nil || errNode.startByte >= errNode.endByte {
		return nil, false
	}
	nameSym, nameNamed, ok := phpSymbolMeta(lang, "name")
	if !ok {
		return nil, false
	}
	return newLeafNodeInArena(arena, nameSym, nameNamed, errNode.startByte, errNode.endByte, errNode.startPoint, errNode.endPoint), true
}

func phpSyntheticArgumentsFromFormals(formals *Node, lang *Language, arena *nodeArena) (*Node, bool) {
	if formals == nil || formals.Type(lang) != "formal_parameters" || len(formals.children) != 2 {
		return nil, false
	}
	argsSym, argsNamed, ok := phpSymbolMeta(lang, "arguments")
	if !ok {
		return nil, false
	}
	children := phpAllocChildren(arena, 2)
	children[0] = formals.children[0]
	children[1] = formals.children[1]
	return newParentNodeInArena(arena, argsSym, argsNamed, children, nil, 0), true
}

func phpSyntheticCompoundStatementFromError(errNode *Node, source []byte, lang *Language, arena *nodeArena) (*Node, bool) {
	if errNode == nil || errNode.startByte >= errNode.endByte || int(errNode.endByte) > len(source) {
		return nil, false
	}
	body := source[errNode.startByte:errNode.endByte]
	if len(body) < 2 || body[0] != '{' || body[len(body)-1] != '}' {
		return nil, false
	}
	compoundSym, compoundNamed, ok := phpSymbolMeta(lang, "compound_statement")
	if !ok {
		return nil, false
	}
	openSym, ok := lang.SymbolByName("{")
	if !ok {
		return nil, false
	}
	closeSym, ok := lang.SymbolByName("}")
	if !ok {
		return nil, false
	}
	openEndByte := errNode.startByte + 1
	openEndPoint := advancePointByBytes(errNode.startPoint, source[errNode.startByte:openEndByte])
	closeStartByte := errNode.endByte - 1
	closeStartPoint := advancePointByBytes(errNode.startPoint, source[errNode.startByte:closeStartByte])
	open := newLeafNodeInArena(arena, openSym, false, errNode.startByte, openEndByte, errNode.startPoint, openEndPoint)
	close := newLeafNodeInArena(arena, closeSym, false, closeStartByte, errNode.endByte, closeStartPoint, errNode.endPoint)
	children := phpAllocChildren(arena, 2)
	children[0] = open
	children[1] = close
	return newParentNodeInArena(arena, compoundSym, compoundNamed, children, nil, 0), true
}

func phpSyntheticFieldIDs(arena *nodeArena, childCount int, lang *Language, byIndex map[int]string) []FieldID {
	fieldIDs := make([]FieldID, childCount)
	if arena != nil {
		fieldIDs = arena.allocFieldIDSlice(childCount)
	}
	for idx, name := range byIndex {
		if idx < 0 || idx >= childCount {
			continue
		}
		if fid, ok := lang.FieldByName(name); ok {
			fieldIDs[idx] = fid
		}
	}
	return fieldIDs
}

func phpAllocChildren(arena *nodeArena, n int) []*Node {
	if arena != nil {
		return arena.allocNodeSlice(n)
	}
	return make([]*Node, n)
}

func phpSymbolMeta(lang *Language, name string) (Symbol, bool, bool) {
	if lang == nil {
		return 0, false, false
	}
	sym, ok := lang.SymbolByName(name)
	if !ok {
		return 0, false, false
	}
	named := false
	if idx := int(sym); idx < len(lang.SymbolMetadata) {
		named = lang.SymbolMetadata[sym].Named
	}
	return sym, named, true
}

func phpCountsAsPriorTopLevelNode(n *Node, lang *Language) bool {
	return n != nil && !n.isExtra && (lang == nil || n.Type(lang) != "php_tag")
}

func assignPHPTopLevelFragmentFields(root *Node, lang *Language, arena *nodeArena) {
	if root == nil || lang == nil || lang.Name != "php" || len(root.children) == 0 {
		return
	}
	var fieldIDs []FieldID
	var fieldSources []uint8
	for i := 0; i+6 < len(root.children); i++ {
		if root.children[i] == nil || root.children[i+1] == nil || root.children[i+2] == nil || root.children[i+3] == nil || root.children[i+4] == nil || root.children[i+6] == nil {
			continue
		}
		if root.children[i].Type(lang) != "static_modifier" ||
			root.children[i+1].Type(lang) != "function" ||
			root.children[i+2].Type(lang) != "formal_parameters" ||
			root.children[i+3].Type(lang) != "{" ||
			root.children[i+4].Type(lang) != "ERROR" ||
			root.children[i+6].Type(lang) != "expression_statement" {
			continue
		}
		if fieldIDs == nil {
			if arena != nil {
				fieldIDs = arena.allocFieldIDSlice(len(root.children))
				fieldSources = make([]uint8, len(root.children))
			} else {
				fieldIDs = make([]FieldID, len(root.children))
				fieldSources = make([]uint8, len(root.children))
			}
		}
		if fid, ok := lang.FieldByName("static_modifier"); ok {
			fieldIDs[i] = fid
			fieldSources[i] = fieldSourceDirect
		}
		if fid, ok := lang.FieldByName("parameters"); ok {
			fieldIDs[i+2] = fid
			fieldSources[i+2] = fieldSourceDirect
		}
	}
	if fieldIDs != nil {
		root.fieldIDs = fieldIDs
		root.fieldSources = fieldSources
	}
}

func phpReparsedTopLevelSuffix(source []byte, start uint32, lang *Language, arena *nodeArena) ([]*Node, bool) {
	if lang == nil || lang.Name != "php" || int(start) >= len(source) {
		return nil, false
	}
	start = phpSkipLeadingLayout(source, start)
	if int(start) >= len(source) {
		return nil, false
	}
	const prefix = "<?php\n"
	wrapped := make([]byte, 0, len(prefix)+len(source)-int(start))
	wrapped = append(wrapped, prefix...)
	wrapped = append(wrapped, source[start:]...)
	tree, err := parseWithSnippetParser(lang, wrapped)
	if err != nil || tree == nil || tree.RootNode() == nil {
		return nil, false
	}
	defer tree.Release()
	startPoint := advancePointByBytes(Point{}, source[:start])
	prefixPoint := advancePointByBytes(Point{}, []byte(prefix))
	if start < uint32(len(prefix)) || startPoint.Row < prefixPoint.Row {
		return nil, false
	}
	offsetRoot := tree.RootNodeWithOffset(
		start-uint32(len(prefix)),
		Point{Row: startPoint.Row - prefixPoint.Row, Column: startPoint.Column},
	)
	if offsetRoot == nil || len(offsetRoot.children) == 0 {
		return nil, false
	}
	out := make([]*Node, 0, len(offsetRoot.children))
	for _, child := range offsetRoot.children {
		if child == nil || child.Type(lang) == "php_tag" {
			continue
		}
		out = append(out, cloneTreeNodesIntoArena(child, arena))
	}
	return out, len(out) > 0
}

func phpSkipLeadingLayout(source []byte, start uint32) uint32 {
	for int(start) < len(source) {
		switch source[start] {
		case ' ', '\t', '\n', '\r':
			start++
		default:
			return start
		}
	}
	return start
}

func bytesContainLineBreak(b []byte) bool {
	for _, c := range b {
		if c == '\n' || c == '\r' {
			return true
		}
	}
	return false
}

func firstNonWhitespaceByte(source []byte) uint32 {
	for i, c := range source {
		switch c {
		case ' ', '\t', '\n', '\r':
			continue
		default:
			return uint32(i)
		}
	}
	return 0
}

func dartProgramChildrenLookComplete(nodes []*Node, lang *Language) bool {
	if len(nodes) == 0 || lang == nil || lang.Name != "dart" {
		return false
	}
	seen := 0
	for _, n := range nodes {
		if n == nil || n.isExtra {
			continue
		}
		if n.IsNamed() {
			seen++
			continue
		}
		switch n.Type(lang) {
		case ";":
			seen++
		default:
			return false
		}
	}
	return seen > 0
}

func dropZeroWidthUnnamedTail(nodes []*Node, lang *Language) []*Node {
	for len(nodes) > 0 {
		last := nodes[len(nodes)-1]
		if last == nil {
			nodes = nodes[:len(nodes)-1]
			continue
		}
		if last.IsNamed() || last.startByte != last.endByte || len(last.children) > 0 {
			break
		}
		if lang != nil && last.Type(lang) != "" {
			break
		}
		nodes = nodes[:len(nodes)-1]
	}
	return nodes
}
