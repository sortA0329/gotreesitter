package gotreesitter

func normalizeCSharpConditionalIsPatternExpressions(root *Node, lang *Language) {
	if root == nil || lang == nil || lang.Name != "c_sharp" {
		return
	}
	isPatternSym, ok := symbolByName(lang, "is_pattern_expression")
	if !ok {
		return
	}
	constantPatternSym, ok := symbolByName(lang, "constant_pattern")
	if !ok {
		return
	}
	isPatternNamed := int(isPatternSym) < len(lang.SymbolMetadata) && lang.SymbolMetadata[isPatternSym].Named
	constantPatternNamed := int(constantPatternSym) < len(lang.SymbolMetadata) && lang.SymbolMetadata[constantPatternSym].Named
	expressionFieldID, _ := lang.FieldByName("expression")
	patternFieldID, _ := lang.FieldByName("pattern")

	var walk func(*Node)
	walk = func(n *Node) {
		if n == nil {
			return
		}
		if n.Type(lang) == "conditional_expression" {
			for i, child := range n.children {
				if child == nil || n.FieldNameForChild(i, lang) != "condition" || child.Type(lang) != "is_expression" {
					continue
				}
				csharpRewriteConditionalIsPatternExpression(child, lang, isPatternSym, isPatternNamed, constantPatternSym, constantPatternNamed, expressionFieldID, patternFieldID)
			}
		}
		for _, child := range n.children {
			walk(child)
		}
	}
	walk(root)
}

func csharpRewriteConditionalIsPatternExpression(n *Node, lang *Language, isPatternSym Symbol, isPatternNamed bool, constantPatternSym Symbol, constantPatternNamed bool, expressionFieldID, patternFieldID FieldID) bool {
	if n == nil || lang == nil || n.Type(lang) != "is_expression" || len(n.children) < 3 {
		return false
	}
	exprIdx := -1
	patternIdx := -1
	for i, child := range n.children {
		if child == nil || !child.IsNamed() {
			continue
		}
		if exprIdx == -1 {
			exprIdx = i
			continue
		}
		patternIdx = i
		break
	}
	if exprIdx < 0 || patternIdx < 0 {
		return false
	}
	patternValue := n.children[patternIdx]
	if patternValue == nil || patternValue.Type(lang) != "identifier" {
		return false
	}
	patternChildren := []*Node{patternValue}
	if n.ownerArena != nil {
		buf := n.ownerArena.allocNodeSlice(len(patternChildren))
		copy(buf, patternChildren)
		patternChildren = buf
	}
	constantPattern := newParentNodeInArena(n.ownerArena, constantPatternSym, constantPatternNamed, patternChildren, nil, 0)
	constantPattern.hasError = false

	children := append([]*Node(nil), n.children...)
	children[patternIdx] = constantPattern
	if n.ownerArena != nil {
		buf := n.ownerArena.allocNodeSlice(len(children))
		copy(buf, children)
		children = buf
	}
	fieldIDs := make([]FieldID, len(children))
	fieldIDs[exprIdx] = expressionFieldID
	fieldIDs[patternIdx] = patternFieldID
	if n.ownerArena != nil {
		buf := n.ownerArena.allocFieldIDSlice(len(fieldIDs))
		copy(buf, fieldIDs)
		fieldIDs = buf
	}

	n.symbol = isPatternSym
	n.isNamed = isPatternNamed
	n.children = children
	n.fieldIDs = fieldIDs
	n.fieldSources = defaultFieldSourcesInArena(n.ownerArena, fieldIDs)
	n.productionID = 0
	n.hasError = false
	populateParentNode(n, n.children)
	return true
}

func normalizeCSharpDereferenceLogicalAndCasts(root *Node, source []byte, lang *Language) {
	if root == nil || lang == nil || lang.Name != "c_sharp" || len(source) == 0 {
		return
	}
	castSym, ok := symbolByName(lang, "cast_expression")
	if !ok {
		return
	}
	prefixUnarySym, ok := symbolByName(lang, "prefix_unary_expression")
	if !ok {
		return
	}
	castNamed := int(castSym) < len(lang.SymbolMetadata) && lang.SymbolMetadata[castSym].Named
	prefixUnaryNamed := int(prefixUnarySym) < len(lang.SymbolMetadata) && lang.SymbolMetadata[prefixUnarySym].Named
	typeFieldID, _ := lang.FieldByName("type")
	valueFieldID, _ := lang.FieldByName("value")

	var walk func(*Node)
	walk = func(n *Node) {
		if n == nil {
			return
		}
		if n.Type(lang) == "binary_expression" {
			csharpRewriteLogicalAndCastExpression(n, source, lang, castSym, castNamed, prefixUnarySym, prefixUnaryNamed, typeFieldID, valueFieldID)
		}
		for _, child := range n.children {
			walk(child)
		}
	}
	walk(root)
}

func csharpRewriteLogicalAndCastExpression(n *Node, source []byte, lang *Language, castSym Symbol, castNamed bool, prefixUnarySym Symbol, prefixUnaryNamed bool, typeFieldID, valueFieldID FieldID) bool {
	if n == nil || lang == nil || n.Type(lang) != "binary_expression" || len(n.children) != 3 {
		return false
	}
	left := n.children[0]
	op := n.children[1]
	right := n.children[2]
	if left == nil || op == nil || right == nil || left.Type(lang) != "parenthesized_expression" || len(left.children) != 3 {
		return false
	}
	typeNode := left.children[1]
	if typeNode == nil || typeNode.Type(lang) != "identifier" {
		return false
	}
	if string(source[op.startByte:op.endByte]) != "&&" || op.endByte-op.startByte != 2 {
		return false
	}
	openTok := left.children[0]
	closeTok := left.children[2]
	if openTok == nil || closeTok == nil || openTok.Type(lang) != "(" || closeTok.Type(lang) != ")" {
		return false
	}
	amp0, ok := csharpBuildLeafNodeByName(n.ownerArena, source, lang, "&", op.startByte, op.startByte+1)
	if !ok {
		return false
	}
	amp1, ok := csharpBuildLeafNodeByName(n.ownerArena, source, lang, "&", op.startByte+1, op.endByte)
	if !ok {
		return false
	}
	innerChildren := []*Node{amp1, right}
	if n.ownerArena != nil {
		buf := n.ownerArena.allocNodeSlice(len(innerChildren))
		copy(buf, innerChildren)
		innerChildren = buf
	}
	inner := newParentNodeInArena(n.ownerArena, prefixUnarySym, prefixUnaryNamed, innerChildren, nil, 0)
	inner.hasError = false
	outerChildren := []*Node{amp0, inner}
	if n.ownerArena != nil {
		buf := n.ownerArena.allocNodeSlice(len(outerChildren))
		copy(buf, outerChildren)
		outerChildren = buf
	}
	outer := newParentNodeInArena(n.ownerArena, prefixUnarySym, prefixUnaryNamed, outerChildren, nil, 0)
	outer.hasError = false

	children := []*Node{openTok, typeNode, closeTok, outer}
	if n.ownerArena != nil {
		buf := n.ownerArena.allocNodeSlice(len(children))
		copy(buf, children)
		children = buf
	}
	fieldIDs := []FieldID{0, typeFieldID, 0, valueFieldID}
	if n.ownerArena != nil {
		buf := n.ownerArena.allocFieldIDSlice(len(fieldIDs))
		copy(buf, fieldIDs)
		fieldIDs = buf
	}

	n.symbol = castSym
	n.isNamed = castNamed
	n.children = children
	n.fieldIDs = fieldIDs
	n.fieldSources = defaultFieldSourcesInArena(n.ownerArena, fieldIDs)
	n.productionID = 0
	n.hasError = false
	populateParentNode(n, n.children)
	return true
}
