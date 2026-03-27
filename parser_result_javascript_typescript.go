package gotreesitter

import "bytes"

func normalizeJavaScriptTopLevelObjectLiterals(root *Node, lang *Language) {
	if root == nil || lang == nil || lang.Name != "javascript" || root.Type(lang) != "program" {
		return
	}
	exprSym, exprNamed, ok := javaScriptSymbolMeta(lang, "expression_statement")
	if !ok {
		return
	}
	objectSym, objectNamed, ok := javaScriptSymbolMeta(lang, "object")
	if !ok {
		return
	}
	pairSym, pairNamed, ok := javaScriptSymbolMeta(lang, "pair")
	if !ok {
		return
	}
	propSym, _, ok := javaScriptSymbolMeta(lang, "property_identifier")
	if !ok {
		return
	}
	for i, child := range root.children {
		repl, ok := rewriteJavaScriptTopLevelObjectLiteral(child, lang, root.ownerArena, exprSym, exprNamed, objectSym, objectNamed, pairSym, pairNamed, propSym)
		if ok {
			root.children[i] = repl
		}
	}
}

func normalizeJavaScriptProgramStart(root *Node, lang *Language) {
	if root == nil || lang == nil || lang.Name != "javascript" || root.Type(lang) != "program" {
		return
	}
	first, _ := firstAndLastNonNilChild(root.children)
	if first == nil {
		return
	}
	root.startByte = first.startByte
	root.startPoint = first.startPoint
}

func normalizeRubyTopLevelModuleBounds(root *Node, source []byte, lang *Language) {
	if root == nil || lang == nil || lang.Name != "ruby" || root.Type(lang) != "program" || len(source) == 0 {
		return
	}
	end := lastNonTriviaByteEnd(source)
	for _, child := range root.children {
		if child == nil || child.IsExtra() || child.Type(lang) != "module" {
			continue
		}
		if len(child.children) > 0 && child.children[0] != nil && child.startByte < child.children[0].startByte {
			child.startByte = child.children[0].startByte
			child.startPoint = child.children[0].startPoint
		}
		if child.endByte == root.endByte && end > child.startByte && end < child.endByte {
			child.endByte = end
			child.endPoint = advancePointByBytes(Point{}, source[:end])
		}
	}
}

func normalizeRubyThenStarts(root *Node, lang *Language) {
	if root == nil || lang == nil || lang.Name != "ruby" {
		return
	}
	var walk func(*Node)
	walk = func(n *Node) {
		if n == nil {
			return
		}
		switch n.Type(lang) {
		case "elsif", "if", "unless", "when":
			normalizeRubyThenChildStarts(n, lang)
		}
		for _, child := range n.children {
			walk(child)
		}
	}
	walk(root)
}

func normalizeRubyThenChildStarts(parent *Node, lang *Language) {
	if parent == nil || lang == nil || len(parent.children) < 2 {
		return
	}
	for i, child := range parent.children {
		if child == nil || child.Type(lang) != "then" || i == 0 {
			continue
		}
		prev := (*Node)(nil)
		for j := i - 1; j >= 0; j-- {
			if parent.children[j] != nil {
				prev = parent.children[j]
				break
			}
		}
		if prev == nil || prev.endByte >= child.startByte {
			continue
		}
		child.startByte = prev.endByte
		child.startPoint = prev.endPoint
	}
}

func normalizeJavaScriptTopLevelExpressionStatementBounds(root *Node, lang *Language) {
	if root == nil || lang == nil || root.Type(lang) != "program" {
		return
	}
	switch lang.Name {
	case "javascript", "typescript", "tsx":
	default:
		return
	}
	for _, child := range root.children {
		if child == nil || child.Type(lang) != "expression_statement" || len(child.children) == 0 {
			continue
		}
		first, last := firstAndLastNonNilChild(child.children)
		if first == nil || last == nil {
			continue
		}
		child.startByte = first.startByte
		child.startPoint = first.startPoint
		child.endByte = last.endByte
		child.endPoint = last.endPoint
	}
}

func normalizeJavaScriptTrailingContinueComments(root *Node, source []byte, lang *Language) {
	if root == nil || lang == nil || lang.Name != "javascript" || len(source) == 0 {
		return
	}
	var walk func(*Node)
	walk = func(n *Node) {
		if n == nil {
			return
		}
		normalizeJavaScriptTrailingContinueCommentSiblings(n, source, lang)
		for _, child := range n.children {
			walk(child)
		}
	}
	walk(root)
}

func normalizeJavaScriptTrailingContinueCommentSiblings(parent *Node, source []byte, lang *Language) {
	if parent == nil || len(parent.children) < 3 || parent.Type(lang) != "statement_block" {
		return
	}
	for i := 1; i+1 < len(parent.children); i++ {
		if comment, ok := extractJavaScriptTrailingContinueComment(parent.children[i], source, lang); ok {
			insertJavaScriptStatementBlockComment(parent, i, comment)
			i++
			continue
		}
		stmt := parent.children[i]
		if stmt == nil || stmt.Type(lang) != "if_statement" || len(stmt.children) < 3 {
			continue
		}
		branch := stmt.children[len(stmt.children)-1]
		comment, ok := extractJavaScriptTrailingContinueComment(branch, source, lang)
		if !ok {
			continue
		}
		stmt.endByte = branch.endByte
		stmt.endPoint = branch.endPoint
		insertJavaScriptStatementBlockComment(parent, i, comment)
		i++
	}
}

func extractJavaScriptTrailingContinueComment(node *Node, source []byte, lang *Language) (*Node, bool) {
	if node == nil || lang == nil || node.Type(lang) != "continue_statement" || len(node.children) < 3 {
		return nil, false
	}
	comment := node.children[len(node.children)-1]
	if comment == nil || comment.Type(lang) != "comment" || comment.startByte >= comment.endByte {
		return nil, false
	}
	if int(comment.endByte) > len(source) || !bytes.HasPrefix(source[comment.startByte:comment.endByte], []byte("//")) {
		return nil, false
	}
	prev := node.children[len(node.children)-2]
	if prev == nil || prev.endByte > comment.startByte || bytesContainLineBreak(source[prev.endByte:comment.startByte]) {
		return nil, false
	}
	node.children = node.children[:len(node.children)-1]
	if len(node.fieldIDs) > len(node.children) {
		node.fieldIDs = node.fieldIDs[:len(node.children)]
		if len(node.fieldSources) > len(node.children) {
			node.fieldSources = node.fieldSources[:len(node.children)]
		}
	}
	node.endByte = prev.endByte
	node.endPoint = prev.endPoint
	return comment, true
}

func insertJavaScriptStatementBlockComment(parent *Node, childIdx int, comment *Node) {
	if parent == nil || comment == nil || childIdx < 0 || childIdx >= len(parent.children) {
		return
	}
	parent.children = append(parent.children[:childIdx+1], append([]*Node{comment}, parent.children[childIdx+1:]...)...)
	if len(parent.fieldIDs) > 0 {
		fieldIDs := append([]FieldID(nil), parent.fieldIDs[:childIdx+1]...)
		fieldIDs = append(fieldIDs, 0)
		fieldIDs = append(fieldIDs, parent.fieldIDs[childIdx+1:]...)
		parent.fieldIDs = fieldIDs
		if len(parent.fieldSources) > 0 {
			fieldSources := append([]uint8(nil), parent.fieldSources[:childIdx+1]...)
			fieldSources = append(fieldSources, fieldSourceNone)
			fieldSources = append(fieldSources, parent.fieldSources[childIdx+1:]...)
			parent.fieldSources = fieldSources
		}
	}
	populateParentNode(parent, parent.children)
}

func normalizeJavaScriptTypeScriptOptionalChainLeaves(root *Node, lang *Language) {
	if root == nil || lang == nil {
		return
	}
	switch lang.Name {
	case "javascript", "typescript", "tsx":
	default:
		return
	}

	var walk func(*Node)
	walk = func(n *Node) {
		if n == nil {
			return
		}
		if n.Type(lang) == "optional_chain" && len(n.children) == 1 {
			child := n.children[0]
			if child != nil && !child.IsNamed() && !child.IsExtra() &&
				child.startByte == n.startByte && child.endByte == n.endByte &&
				child.startPoint == n.startPoint && child.endPoint == n.endPoint {
				n.children = nil
				n.fieldIDs = nil
				n.fieldSources = nil
			}
		}
		for _, child := range n.children {
			walk(child)
		}
	}
	walk(root)
}

func normalizeJavaScriptTypeScriptCallPrecedence(root *Node, lang *Language) {
	if root == nil || lang == nil {
		return
	}
	switch lang.Name {
	case "javascript", "typescript", "tsx":
	default:
		return
	}

	var walk func(*Node)
	walk = func(n *Node) {
		if n == nil {
			return
		}
		for i, child := range n.children {
			if rewritten := rewriteJavaScriptTypeScriptCallPrecedence(child, lang); rewritten != nil {
				n.children[i] = rewritten
				rewritten.parent = n
				rewritten.childIndex = i
				child = rewritten
			}
			walk(child)
		}
	}
	walk(root)
}

func rewriteJavaScriptTypeScriptCallPrecedence(node *Node, lang *Language) *Node {
	if node == nil || lang == nil || node.Type(lang) != "call_expression" || len(node.children) != 2 {
		return nil
	}
	function := node.children[0]
	arguments := node.children[1]
	if function == nil || arguments == nil {
		return nil
	}
	return rewriteJavaScriptTypeScriptCallTarget(function, arguments, node, lang)
}

func rewriteJavaScriptTypeScriptCallTarget(target, arguments, callNode *Node, lang *Language) *Node {
	if target == nil || arguments == nil || callNode == nil || lang == nil {
		return nil
	}
	if isJavaScriptTypeScriptCallableShape(target, lang) {
		rewrittenCall := cloneNodeInArena(callNode.ownerArena, callNode)
		rewrittenCall.children = cloneNodeSliceInArena(callNode.ownerArena, []*Node{target, arguments})
		populateParentNode(rewrittenCall, rewrittenCall.children)
		return rewrittenCall
	}

	switch target.Type(lang) {
	case "unary_expression":
		if len(target.children) < 2 {
			return nil
		}
		operandIdx := len(target.children) - 1
		rewrittenOperand := rewriteJavaScriptTypeScriptCallTarget(target.children[operandIdx], arguments, callNode, lang)
		if rewrittenOperand == nil {
			return nil
		}
		rewrittenUnary := cloneNodeInArena(callNode.ownerArena, target)
		unaryChildren := cloneNodeSliceInArena(callNode.ownerArena, target.children)
		unaryChildren[operandIdx] = rewrittenOperand
		rewrittenUnary.children = unaryChildren
		populateParentNode(rewrittenUnary, rewrittenUnary.children)
		return rewrittenUnary
	case "binary_expression":
		operator, rightIdx, ok := javaScriptTypeScriptBinaryOperatorAndRight(target, lang)
		if !ok || rightIdx < 0 || rightIdx >= len(target.children) {
			return nil
		}
		if operator == nil {
			return nil
		}
		if _, ok := javaScriptTypeScriptBinaryOperatorPrecedence(operator.Type(lang)); !ok {
			return nil
		}
		rewrittenRight := rewriteJavaScriptTypeScriptCallTarget(target.children[rightIdx], arguments, callNode, lang)
		if rewrittenRight == nil {
			return nil
		}
		rewrittenBinary := cloneNodeInArena(callNode.ownerArena, target)
		binaryChildren := cloneNodeSliceInArena(callNode.ownerArena, target.children)
		binaryChildren[rightIdx] = rewrittenRight
		rewrittenBinary.children = binaryChildren
		populateParentNode(rewrittenBinary, rewrittenBinary.children)
		return rewrittenBinary
	default:
		return nil
	}
}

func javaScriptTypeScriptBinaryOperatorAndRight(node *Node, lang *Language) (*Node, int, bool) {
	if node == nil || lang == nil || node.Type(lang) != "binary_expression" || len(node.children) < 3 {
		return nil, -1, false
	}
	operatorIdx := -1
	rightIdx := -1
	for i := 0; i < len(node.children); i++ {
		switch node.FieldNameForChild(i, lang) {
		case "operator":
			operatorIdx = i
		case "right":
			rightIdx = i
		}
	}
	if operatorIdx < 0 && len(node.children) >= 2 {
		operatorIdx = 1
	}
	if rightIdx < 0 {
		for i := len(node.children) - 1; i >= 0; i-- {
			child := node.children[i]
			if child == nil || child.isExtra {
				continue
			}
			if i != operatorIdx {
				rightIdx = i
				break
			}
		}
	}
	if operatorIdx < 0 || rightIdx < 0 || operatorIdx >= len(node.children) {
		return nil, -1, false
	}
	return node.children[operatorIdx], rightIdx, true
}

func isJavaScriptTypeScriptCallableShape(node *Node, lang *Language) bool {
	if node == nil || lang == nil {
		return false
	}
	switch node.Type(lang) {
	case "identifier", "member_expression", "subscript_expression", "call_expression", "parenthesized_expression":
		return true
	default:
		return false
	}
}

func cloneNodeSliceInArena(arena *nodeArena, nodes []*Node) []*Node {
	if len(nodes) == 0 {
		return nil
	}
	if arena != nil {
		buf := arena.allocNodeSlice(len(nodes))
		copy(buf, nodes)
		return buf
	}
	buf := make([]*Node, len(nodes))
	copy(buf, nodes)
	return buf
}

func normalizeJavaScriptTypeScriptUnaryPrecedence(root *Node, lang *Language) {
	if root == nil || lang == nil {
		return
	}
	switch lang.Name {
	case "javascript", "typescript", "tsx":
	default:
		return
	}

	var walk func(*Node)
	walk = func(n *Node) {
		if n == nil {
			return
		}
		for i, child := range n.children {
			walk(child)
			for {
				rewritten := rewriteJavaScriptTypeScriptUnaryPrecedence(child, lang)
				if rewritten == nil {
					break
				}
				n.children[i] = rewritten
				rewritten.parent = n
				rewritten.childIndex = i
				child = rewritten
			}
		}
	}
	walk(root)
}

func rewriteJavaScriptTypeScriptUnaryPrecedence(node *Node, lang *Language) *Node {
	if node == nil || lang == nil || node.Type(lang) != "unary_expression" || len(node.children) < 2 {
		return nil
	}
	operandIdx := len(node.children) - 1
	operand := node.children[operandIdx]
	if operand == nil || operand.Type(lang) != "binary_expression" || len(operand.children) != 3 {
		return nil
	}
	if _, ok := javaScriptTypeScriptBinaryOperatorPrecedence(operand.children[1].Type(lang)); !ok {
		return nil
	}

	rewrittenUnary := cloneNodeInArena(node.ownerArena, node)
	unaryChildren := cloneNodeSliceInArena(node.ownerArena, node.children)
	unaryChildren[operandIdx] = operand.children[0]
	rewrittenUnary.children = unaryChildren
	populateParentNode(rewrittenUnary, rewrittenUnary.children)

	rewrittenBinary := cloneNodeInArena(node.ownerArena, operand)
	binaryChildren := cloneNodeSliceInArena(node.ownerArena, operand.children)
	binaryChildren[0] = rewrittenUnary
	rewrittenBinary.children = binaryChildren
	populateParentNode(rewrittenBinary, rewrittenBinary.children)
	return rewrittenBinary
}

func normalizeJavaScriptTypeScriptBinaryPrecedence(root *Node, lang *Language) {
	if root == nil || lang == nil {
		return
	}
	switch lang.Name {
	case "javascript", "typescript", "tsx":
	default:
		return
	}

	var walk func(*Node)
	walk = func(n *Node) {
		if n == nil {
			return
		}
		for i, child := range n.children {
			walk(child)
			for {
				rewritten := rewriteJavaScriptTypeScriptBinaryPrecedence(child, lang)
				if rewritten == nil {
					break
				}
				n.children[i] = rewritten
				rewritten.parent = n
				rewritten.childIndex = i
				child = rewritten
			}
		}
	}
	walk(root)
}

func rewriteJavaScriptTypeScriptBinaryPrecedence(node *Node, lang *Language) *Node {
	if node == nil || lang == nil || node.Type(lang) != "binary_expression" || len(node.children) != 3 {
		return nil
	}
	left := node.children[0]
	op := node.children[1]
	right := node.children[2]
	if left == nil || op == nil || right == nil || left.Type(lang) != "binary_expression" || len(left.children) != 3 {
		return nil
	}
	leftOp := left.children[1]
	if leftOp == nil {
		return nil
	}

	parentPrec, ok := javaScriptTypeScriptBinaryOperatorPrecedence(op.Type(lang))
	if !ok {
		return nil
	}
	leftPrec, ok := javaScriptTypeScriptBinaryOperatorPrecedence(leftOp.Type(lang))
	if !ok || parentPrec <= leftPrec {
		return nil
	}

	rotatedInner := cloneNodeInArena(node.ownerArena, node)
	rotatedInner.children = cloneNodeSliceInArena(node.ownerArena, []*Node{left.children[2], op, right})
	populateParentNode(rotatedInner, rotatedInner.children)

	rotatedOuter := cloneNodeInArena(node.ownerArena, left)
	rotatedOuter.children = cloneNodeSliceInArena(node.ownerArena, []*Node{left.children[0], leftOp, rotatedInner})
	populateParentNode(rotatedOuter, rotatedOuter.children)
	return rotatedOuter
}

func javaScriptTypeScriptBinaryOperatorPrecedence(op string) (int, bool) {
	switch op {
	case "??":
		return 1, true
	case "||":
		return 2, true
	case "&&":
		return 3, true
	case "|":
		return 4, true
	case "^":
		return 5, true
	case "&":
		return 6, true
	case "==", "!=", "===", "!==":
		return 7, true
	case "<", "<=", ">", ">=", "instanceof", "in":
		return 8, true
	case "<<", ">>", ">>>":
		return 9, true
	case "+", "-":
		return 10, true
	case "*", "/", "%":
		return 11, true
	case "**":
		return 12, true
	default:
		return 0, false
	}
}

type typeScriptNormalizationContext struct {
	source []byte
	lang   *Language

	canRewriteGenericCalls      bool
	canRewriteInstantiatedCalls bool
	canRewriteAsExpressions     bool
	canClearEnumBodyFields      bool

	callSym                Symbol
	callNamed              bool
	instantiationExprSym   Symbol
	instantiationExprNamed bool
	typeArgsSym            Symbol
	typeArgsNamed          bool
	argsSym                Symbol
	argsNamed              bool
	predefinedTypeSym      Symbol
	predefinedTypeNamed    bool
	asExpressionSym        Symbol
	asExpressionNamed      bool
	functionFieldID        FieldID
	typeArgsFieldID        FieldID
	argumentsFieldID       FieldID
	binaryExpressionSym    Symbol
	assignmentExprSym      Symbol
	assignmentExprNamed    bool
	ternaryExprSym         Symbol
	ternaryExprNamed       bool
	unionTypeSym           Symbol
	unionTypeNamed         bool
	intersectionTypeSym    Symbol
	intersectionTypeNamed  bool
	objectTypeSym          Symbol
	objectTypeNamed        bool
	propertySignatureSym   Symbol
	propertySignatureNamed bool
	typeAnnotationSym      Symbol
	typeAnnotationNamed    bool
	objectSym              Symbol
	pairSym                Symbol
	propertyIdentifierSym  Symbol
	colonSym               Symbol
	greaterThanSym         Symbol
	parenthesizedExprSym   Symbol
	lessThanSym            Symbol
	identifierSym          Symbol
	memberExpressionSym    Symbol
	sequenceExpressionSym  Symbol
	typeIdentifierSym      Symbol
	typeIdentifierNamed    bool
	hasTypeIdentifierSym   bool
	enumBodySym            Symbol
	enumAssignmentSym      Symbol
}

func normalizeTypeScriptCompatibility(root *Node, source []byte, lang *Language) {
	ctx, ok := newTypeScriptNormalizationContext(source, lang)
	if !ok || root == nil {
		return
	}

	var walk func(*Node)
	walk = func(n *Node) {
		if n == nil {
			return
		}
		normalizeTypeScriptIdentifierKeywordAliases(n, &ctx)
		normalizeTypeScriptImportKeywordNamedness(n, &ctx)
		if ctx.canClearEnumBodyFields && n.symbol == ctx.enumBodySym && len(n.fieldIDs) > 0 {
			limit := len(n.children)
			if len(n.fieldIDs) < limit {
				limit = len(n.fieldIDs)
			}
			for i := 0; i < limit; i++ {
				child := n.children[i]
				if child == nil || child.symbol != ctx.enumAssignmentSym {
					continue
				}
				n.fieldIDs[i] = 0
				if len(n.fieldSources) > i {
					n.fieldSources[i] = fieldSourceNone
				}
			}
		}
		for i, child := range n.children {
			for {
				var rewritten *Node
				switch {
				case ctx.canRewriteGenericCalls:
					rewritten = rewriteTypeScriptPredefinedGenericCall(child, &ctx)
				}
				if rewritten == nil && ctx.canRewriteInstantiatedCalls {
					rewritten = rewriteTypeScriptInstantiatedCall(child, &ctx)
				}
				if rewritten == nil && ctx.canRewriteAsExpressions {
					rewritten = rewriteTypeScriptAsExpressionCompatibility(child, &ctx)
				}
				if rewritten == nil {
					break
				}
				n.children[i] = rewritten
				rewritten.parent = n
				rewritten.childIndex = i
				child = rewritten
			}
			walk(child)
		}
	}
	walk(root)
}

func normalizeTypeScriptIdentifierKeywordAliases(node *Node, ctx *typeScriptNormalizationContext) {
	if node == nil || ctx == nil || ctx.lang == nil || node.symbol != ctx.identifierSym || len(node.children) != 1 {
		return
	}
	child := node.children[0]
	if child == nil || child.IsNamed() || child.IsExtra() {
		return
	}
	if child.startByte != node.startByte || child.endByte != node.endByte || child.startPoint != node.startPoint || child.endPoint != node.endPoint {
		return
	}
	node.children = nil
	node.fieldIDs = nil
	node.fieldSources = nil
}

func normalizeTypeScriptImportKeywordNamedness(node *Node, ctx *typeScriptNormalizationContext) {
	if node == nil || ctx == nil || ctx.lang == nil || node.Type(ctx.lang) != "import" {
		return
	}
	node.isNamed = false
}

func normalizeTypeScriptRecoveredNamespaceRoot(root *Node, source []byte, lang *Language) {
	if root == nil || lang == nil || len(root.children) < 4 {
		return
	}
	if lang.Name != "tsx" && lang.Name != "typescript" {
		return
	}
	rootType := root.Type(lang)
	if rootType != "ERROR" && rootType != "program" {
		return
	}
	stmtBlockSym, ok := lang.SymbolByName("statement_block")
	if !ok {
		return
	}
	internalModuleSym, ok := lang.SymbolByName("internal_module")
	if !ok {
		return
	}
	exprStmtSym, hasExprStmtSym := lang.SymbolByName("expression_statement")
	programSym, hasProgramSym := lang.SymbolByName("program")

	namespaceIdx := -1
	for i, child := range root.children {
		if child == nil || child.isExtra {
			continue
		}
		if child.Type(lang) != "namespace" {
			if child.Type(lang) != "comment" {
				return
			}
			continue
		}
		namespaceIdx = i
		break
	}
	if namespaceIdx < 0 || namespaceIdx+2 >= len(root.children) {
		return
	}
	nameNode := root.children[namespaceIdx+1]
	openBrace := root.children[namespaceIdx+2]
	if nameNode == nil || openBrace == nil || nameNode.Type(lang) != "identifier" || openBrace.Type(lang) != "{" {
		return
	}

	bodyChildren := make([]*Node, 0, len(root.children)-(namespaceIdx+3))
	for i := namespaceIdx + 3; i < len(root.children); i++ {
		child := root.children[i]
		if child == nil {
			continue
		}
		if typeScriptWhitespaceOnlyRecoverySubtree(child, source) {
			continue
		}
		bodyChildren = append(bodyChildren, child)
	}
	if len(bodyChildren) == 0 {
		return
	}
	if root.ownerArena != nil {
		buf := root.ownerArena.allocNodeSlice(len(bodyChildren))
		copy(buf, bodyChildren)
		bodyChildren = buf
	}

	stmtBlockNamed := int(stmtBlockSym) < len(lang.SymbolMetadata) && lang.SymbolMetadata[stmtBlockSym].Named
	internalModuleNamed := int(internalModuleSym) < len(lang.SymbolMetadata) && lang.SymbolMetadata[internalModuleSym].Named
	block := newParentNodeInArena(root.ownerArena, stmtBlockSym, stmtBlockNamed, bodyChildren, nil, 0)
	block.startByte = openBrace.startByte
	block.startPoint = openBrace.startPoint
	if len(bodyChildren) > 0 {
		last := bodyChildren[len(bodyChildren)-1]
		block.endByte = last.endByte
		block.endPoint = last.endPoint
	}

	moduleChildren := []*Node{nameNode, block}
	if root.ownerArena != nil {
		buf := root.ownerArena.allocNodeSlice(len(moduleChildren))
		copy(buf, moduleChildren)
		moduleChildren = buf
	}
	internalModule := newParentNodeInArena(root.ownerArena, internalModuleSym, internalModuleNamed, moduleChildren, nil, 0)
	internalModule.startByte = root.children[namespaceIdx].startByte
	internalModule.startPoint = root.children[namespaceIdx].startPoint
	internalModule.endByte = block.endByte
	internalModule.endPoint = block.endPoint

	wrapped := internalModule
	if hasExprStmtSym {
		exprStmtNamed := int(exprStmtSym) < len(lang.SymbolMetadata) && lang.SymbolMetadata[exprStmtSym].Named
		exprChildren := []*Node{internalModule}
		if root.ownerArena != nil {
			buf := root.ownerArena.allocNodeSlice(1)
			buf[0] = internalModule
			exprChildren = buf
		}
		exprStmt := newParentNodeInArena(root.ownerArena, exprStmtSym, exprStmtNamed, exprChildren, nil, 0)
		exprStmt.startByte = internalModule.startByte
		exprStmt.startPoint = internalModule.startPoint
		exprStmt.endByte = internalModule.endByte
		exprStmt.endPoint = internalModule.endPoint
		wrapped = exprStmt
	}

	newChildren := make([]*Node, 0, namespaceIdx+1)
	for i := 0; i < namespaceIdx; i++ {
		if root.children[i] != nil {
			newChildren = append(newChildren, root.children[i])
		}
	}
	newChildren = append(newChildren, wrapped)
	if root.ownerArena != nil {
		buf := root.ownerArena.allocNodeSlice(len(newChildren))
		copy(buf, newChildren)
		newChildren = buf
	}
	root.children = newChildren
	root.fieldIDs = nil
	root.fieldSources = nil
	if hasProgramSym {
		root.symbol = programSym
		root.isNamed = int(programSym) < len(lang.SymbolMetadata) && lang.SymbolMetadata[programSym].Named
	}
	populateParentNode(root, root.children)
}

func typeScriptWhitespaceOnlyRecoverySubtree(node *Node, source []byte) bool {
	if node == nil || (!node.HasError() && node.symbol != errorSymbol) {
		return false
	}
	if int(node.endByte) > len(source) || node.startByte > node.endByte {
		return false
	}
	return bytesAreTrivia(source[node.startByte:node.endByte])
}

func newTypeScriptNormalizationContext(source []byte, lang *Language) (typeScriptNormalizationContext, bool) {
	ctx := typeScriptNormalizationContext{
		source: source,
		lang:   lang,
	}
	if lang == nil {
		return ctx, false
	}
	switch lang.Name {
	case "tsx", "typescript":
	default:
		return ctx, false
	}

	if callSym, ok := lang.SymbolByName("call_expression"); ok {
		if instantiationExprSym, ok := lang.SymbolByName("instantiation_expression"); ok {
			ctx.instantiationExprSym = instantiationExprSym
			ctx.instantiationExprNamed = int(instantiationExprSym) < len(lang.SymbolMetadata) && lang.SymbolMetadata[instantiationExprSym].Named
		}
		if typeArgsSym, ok := lang.SymbolByName("type_arguments"); ok {
			if argsSym, ok := lang.SymbolByName("arguments"); ok {
				if predefinedTypeSym, ok := lang.SymbolByName("predefined_type"); ok {
					if binaryExpressionSym, ok := lang.SymbolByName("binary_expression"); ok {
						if greaterThanSym, ok := lang.SymbolByName(">"); ok {
							if parenthesizedExprSym, ok := lang.SymbolByName("parenthesized_expression"); ok {
								if lessThanSym, ok := lang.SymbolByName("<"); ok {
									if identifierSym, ok := lang.SymbolByName("identifier"); ok {
										if memberExpressionSym, ok := lang.SymbolByName("member_expression"); ok {
											if sequenceExpressionSym, ok := lang.SymbolByName("sequence_expression"); ok {
												ctx.canRewriteGenericCalls = true
												ctx.callSym = callSym
												ctx.callNamed = int(callSym) < len(lang.SymbolMetadata) && lang.SymbolMetadata[callSym].Named
												ctx.typeArgsSym = typeArgsSym
												ctx.typeArgsNamed = int(typeArgsSym) < len(lang.SymbolMetadata) && lang.SymbolMetadata[typeArgsSym].Named
												ctx.argsSym = argsSym
												ctx.argsNamed = int(argsSym) < len(lang.SymbolMetadata) && lang.SymbolMetadata[argsSym].Named
												ctx.predefinedTypeSym = predefinedTypeSym
												ctx.predefinedTypeNamed = int(predefinedTypeSym) < len(lang.SymbolMetadata) && lang.SymbolMetadata[predefinedTypeSym].Named
												ctx.binaryExpressionSym = binaryExpressionSym
												ctx.greaterThanSym = greaterThanSym
												ctx.parenthesizedExprSym = parenthesizedExprSym
												ctx.lessThanSym = lessThanSym
												ctx.identifierSym = identifierSym
												ctx.memberExpressionSym = memberExpressionSym
												ctx.sequenceExpressionSym = sequenceExpressionSym
												ctx.functionFieldID, _ = lang.FieldByName("function")
												ctx.typeArgsFieldID, _ = lang.FieldByName("type_arguments")
												ctx.argumentsFieldID, _ = lang.FieldByName("arguments")
												ctx.typeIdentifierSym, ctx.hasTypeIdentifierSym = lang.SymbolByName("type_identifier")
												if ctx.hasTypeIdentifierSym {
													ctx.typeIdentifierNamed = int(ctx.typeIdentifierSym) < len(lang.SymbolMetadata) && lang.SymbolMetadata[ctx.typeIdentifierSym].Named
												}
												ctx.canRewriteInstantiatedCalls = ctx.instantiationExprSym != 0 && ctx.functionFieldID != 0 && ctx.typeArgsFieldID != 0 && ctx.argumentsFieldID != 0
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}

	if asExpressionSym, ok := lang.SymbolByName("as_expression"); ok {
		if assignmentExprSym, ok := lang.SymbolByName("assignment_expression"); ok {
			if ternaryExprSym, ok := lang.SymbolByName("ternary_expression"); ok {
				if unionTypeSym, ok := lang.SymbolByName("union_type"); ok {
					if intersectionTypeSym, ok := lang.SymbolByName("intersection_type"); ok {
						ctx.canRewriteAsExpressions = true
						ctx.asExpressionSym = asExpressionSym
						ctx.asExpressionNamed = int(asExpressionSym) < len(lang.SymbolMetadata) && lang.SymbolMetadata[asExpressionSym].Named
						ctx.assignmentExprSym = assignmentExprSym
						ctx.assignmentExprNamed = int(assignmentExprSym) < len(lang.SymbolMetadata) && lang.SymbolMetadata[assignmentExprSym].Named
						ctx.ternaryExprSym = ternaryExprSym
						ctx.ternaryExprNamed = int(ternaryExprSym) < len(lang.SymbolMetadata) && lang.SymbolMetadata[ternaryExprSym].Named
						ctx.unionTypeSym = unionTypeSym
						ctx.unionTypeNamed = int(unionTypeSym) < len(lang.SymbolMetadata) && lang.SymbolMetadata[unionTypeSym].Named
						ctx.intersectionTypeSym = intersectionTypeSym
						ctx.intersectionTypeNamed = int(intersectionTypeSym) < len(lang.SymbolMetadata) && lang.SymbolMetadata[intersectionTypeSym].Named
						if objectTypeSym, ok := lang.SymbolByName("object_type"); ok {
							if propertySignatureSym, ok := lang.SymbolByName("property_signature"); ok {
								if typeAnnotationSym, ok := lang.SymbolByName("type_annotation"); ok {
									if objectSym, ok := lang.SymbolByName("object"); ok {
										if pairSym, ok := lang.SymbolByName("pair"); ok {
											if propertyIdentifierSym, ok := lang.SymbolByName("property_identifier"); ok {
												if colonSym, ok := lang.SymbolByName(":"); ok {
													ctx.objectTypeSym = objectTypeSym
													ctx.objectTypeNamed = int(objectTypeSym) < len(lang.SymbolMetadata) && lang.SymbolMetadata[objectTypeSym].Named
													ctx.propertySignatureSym = propertySignatureSym
													ctx.propertySignatureNamed = int(propertySignatureSym) < len(lang.SymbolMetadata) && lang.SymbolMetadata[propertySignatureSym].Named
													ctx.typeAnnotationSym = typeAnnotationSym
													ctx.typeAnnotationNamed = int(typeAnnotationSym) < len(lang.SymbolMetadata) && lang.SymbolMetadata[typeAnnotationSym].Named
													ctx.objectSym = objectSym
													ctx.pairSym = pairSym
													ctx.propertyIdentifierSym = propertyIdentifierSym
													ctx.colonSym = colonSym
												}
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}

	if enumBodySym, ok := lang.SymbolByName("enum_body"); ok {
		if enumAssignmentSym, ok := lang.SymbolByName("enum_assignment"); ok {
			ctx.canClearEnumBodyFields = true
			ctx.enumBodySym = enumBodySym
			ctx.enumAssignmentSym = enumAssignmentSym
		}
	}

	return ctx, ctx.canRewriteGenericCalls || ctx.canRewriteInstantiatedCalls || ctx.canRewriteAsExpressions || ctx.canClearEnumBodyFields
}

func rewriteTypeScriptPredefinedGenericCall(node *Node, ctx *typeScriptNormalizationContext) *Node {
	if node == nil || ctx == nil || ctx.lang == nil || node.symbol != ctx.binaryExpressionSym || len(node.children) != 3 {
		return nil
	}
	left := node.children[0]
	gt := node.children[1]
	paren := node.children[2]
	if left == nil || gt == nil || paren == nil || left.symbol != ctx.binaryExpressionSym || gt.symbol != ctx.greaterThanSym || paren.symbol != ctx.parenthesizedExprSym {
		return nil
	}
	if len(left.children) != 3 || len(paren.children) != 3 {
		return nil
	}
	callee := left.children[0]
	lt := left.children[1]
	typeArg := left.children[2]
	if callee == nil || lt == nil || typeArg == nil || lt.symbol != ctx.lessThanSym {
		return nil
	}
	switch callee.Type(ctx.lang) {
	case "identifier", "member_expression":
	default:
		return nil
	}
	typeArg = normalizeTypeScriptGenericCallTypeArgument(typeArg, ctx)
	if typeArg == nil {
		return nil
	}
	arena := node.ownerArena
	if typeArg.ownerArena != arena {
		typeArg = cloneNodeInArena(arena, typeArg)
	}
	typeArgs := newParentNodeInArena(arena, ctx.typeArgsSym, ctx.typeArgsNamed, []*Node{lt, typeArg, gt}, nil, 0)
	argsChildren := typeScriptGenericCallArgumentChildren(paren, ctx.sequenceExpressionSym)
	if arena != nil && len(argsChildren) > 0 {
		buf := arena.allocNodeSlice(len(argsChildren))
		copy(buf, argsChildren)
		argsChildren = buf
	}
	args := newParentNodeInArena(arena, ctx.argsSym, ctx.argsNamed, argsChildren, nil, paren.productionID)

	callChildren := phpAllocChildren(arena, 3)
	callChildren[0] = callee
	callChildren[1] = typeArgs
	callChildren[2] = args
	var fieldIDs []FieldID
	if ctx.functionFieldID != 0 || ctx.typeArgsFieldID != 0 || ctx.argumentsFieldID != 0 {
		if arena != nil {
			fieldIDs = arena.allocFieldIDSlice(3)
		} else {
			fieldIDs = make([]FieldID, 3)
		}
		fieldIDs[0] = ctx.functionFieldID
		fieldIDs[1] = ctx.typeArgsFieldID
		fieldIDs[2] = ctx.argumentsFieldID
	}
	call := newParentNodeInArena(arena, ctx.callSym, ctx.callNamed, callChildren, fieldIDs, node.productionID)
	call.fieldSources = defaultFieldSourcesInArena(arena, fieldIDs)
	return call
}

func rewriteTypeScriptInstantiatedCall(node *Node, ctx *typeScriptNormalizationContext) *Node {
	if node == nil || ctx == nil || ctx.lang == nil || node.symbol != ctx.callSym || len(node.children) != 2 {
		return nil
	}
	function := node.children[0]
	arguments := node.children[1]
	if function == nil || arguments == nil || function.symbol != ctx.instantiationExprSym || arguments.symbol != ctx.argsSym || len(function.children) != 2 {
		return nil
	}
	callee := function.children[0]
	typeArgs := function.children[1]
	if callee == nil || typeArgs == nil || typeArgs.symbol != ctx.typeArgsSym {
		return nil
	}
	children := phpAllocChildren(node.ownerArena, 3)
	children[0] = callee
	children[1] = typeArgs
	children[2] = arguments
	var fieldIDs []FieldID
	if ctx.functionFieldID != 0 || ctx.typeArgsFieldID != 0 || ctx.argumentsFieldID != 0 {
		if node.ownerArena != nil {
			fieldIDs = node.ownerArena.allocFieldIDSlice(3)
		} else {
			fieldIDs = make([]FieldID, 3)
		}
		fieldIDs[0] = ctx.functionFieldID
		fieldIDs[1] = ctx.typeArgsFieldID
		fieldIDs[2] = ctx.argumentsFieldID
	}
	call := newParentNodeInArena(node.ownerArena, ctx.callSym, ctx.callNamed, children, fieldIDs, node.productionID)
	call.fieldSources = defaultFieldSourcesInArena(node.ownerArena, fieldIDs)
	return call
}

func rewriteTypeScriptAsExpressionCompatibility(node *Node, ctx *typeScriptNormalizationContext) *Node {
	if node == nil || ctx == nil || ctx.lang == nil {
		return nil
	}
	if rewritten := rewriteTypeScriptAsAssignmentOrTernary(node, ctx); rewritten != nil {
		return rewritten
	}
	return rewriteTypeScriptAsTypeChain(node, ctx)
}

func rewriteTypeScriptAsAssignmentOrTernary(node *Node, ctx *typeScriptNormalizationContext) *Node {
	if node == nil || ctx == nil || ctx.lang == nil || node.symbol != ctx.asExpressionSym || len(node.children) < 2 {
		return nil
	}
	valueIdx, typeIdx := 0, len(node.children)-1
	value := node.children[valueIdx]
	if value == nil {
		return nil
	}

	switch value.symbol {
	case ctx.assignmentExprSym:
		if len(value.children) < 2 {
			return nil
		}
		rightIdx := len(value.children) - 1
		rewrittenAs := cloneNodeInArena(node.ownerArena, node)
		asChildren := cloneNodeSliceInArena(node.ownerArena, node.children)
		asChildren[valueIdx] = value.children[rightIdx]
		rewrittenAs.children = asChildren
		populateParentNode(rewrittenAs, rewrittenAs.children)

		rewrittenAssign := cloneNodeInArena(node.ownerArena, value)
		assignChildren := cloneNodeSliceInArena(node.ownerArena, value.children)
		assignChildren[rightIdx] = rewrittenAs
		rewrittenAssign.children = assignChildren
		populateParentNode(rewrittenAssign, rewrittenAssign.children)
		return rewrittenAssign
	case ctx.ternaryExprSym:
		if len(value.children) < 3 {
			return nil
		}
		falseIdx := len(value.children) - 1
		rewrittenAs := cloneNodeInArena(node.ownerArena, node)
		asChildren := cloneNodeSliceInArena(node.ownerArena, node.children)
		asChildren[valueIdx] = value.children[falseIdx]
		rewrittenAs.children = asChildren
		populateParentNode(rewrittenAs, rewrittenAs.children)

		rewrittenTernary := cloneNodeInArena(node.ownerArena, value)
		ternaryChildren := cloneNodeSliceInArena(node.ownerArena, value.children)
		ternaryChildren[falseIdx] = rewrittenAs
		rewrittenTernary.children = ternaryChildren
		populateParentNode(rewrittenTernary, rewrittenTernary.children)
		return rewrittenTernary
	default:
		_ = typeIdx
		return nil
	}
}

func rewriteTypeScriptAsTypeChain(node *Node, ctx *typeScriptNormalizationContext) *Node {
	if node == nil || ctx == nil || ctx.lang == nil || node.symbol != ctx.binaryExpressionSym || len(node.children) != 3 {
		return nil
	}
	baseAs, rewrittenType, ok := collapseTypeScriptAsTypeChain(node, ctx)
	if !ok || baseAs == nil || rewrittenType == nil || len(baseAs.children) < 2 {
		return nil
	}
	rewrittenAs := cloneNodeInArena(node.ownerArena, baseAs)
	asChildren := cloneNodeSliceInArena(node.ownerArena, baseAs.children)
	asChildren[len(asChildren)-1] = rewrittenType
	rewrittenAs.children = asChildren
	populateParentNode(rewrittenAs, rewrittenAs.children)
	return rewrittenAs
}

func collapseTypeScriptAsTypeChain(node *Node, ctx *typeScriptNormalizationContext) (*Node, *Node, bool) {
	if node == nil || ctx == nil || ctx.lang == nil || node.symbol != ctx.binaryExpressionSym || len(node.children) != 3 {
		return nil, nil, false
	}
	left := node.children[0]
	op := node.children[1]
	right := node.children[2]
	if left == nil || op == nil || right == nil {
		return nil, nil, false
	}
	var typeSym Symbol
	var typeNamed bool
	switch op.Type(ctx.lang) {
	case "|":
		typeSym = ctx.unionTypeSym
		typeNamed = ctx.unionTypeNamed
	case "&":
		typeSym = ctx.intersectionTypeSym
		typeNamed = ctx.intersectionTypeNamed
	default:
		return nil, nil, false
	}

	rightType := normalizeTypeScriptTypeExpression(right, ctx)
	if rightType == nil {
		return nil, nil, false
	}

	if left.symbol == ctx.asExpressionSym && len(left.children) >= 2 {
		leftType := normalizeTypeScriptTypeExpression(left.children[len(left.children)-1], ctx)
		if leftType == nil {
			return nil, nil, false
		}
		children := cloneNodeSliceInArena(node.ownerArena, []*Node{leftType, op, rightType})
		return left, newParentNodeInArena(node.ownerArena, typeSym, typeNamed, children, nil, node.productionID), true
	}

	leftAs, leftType, ok := collapseTypeScriptAsTypeChain(left, ctx)
	if !ok || leftAs == nil || leftType == nil {
		return nil, nil, false
	}
	children := cloneNodeSliceInArena(node.ownerArena, []*Node{leftType, op, rightType})
	return leftAs, newParentNodeInArena(node.ownerArena, typeSym, typeNamed, children, nil, node.productionID), true
}

func normalizeTypeScriptTypeExpression(node *Node, ctx *typeScriptNormalizationContext) *Node {
	if node == nil || ctx == nil || ctx.lang == nil {
		return nil
	}
	switch node.Type(ctx.lang) {
	case "type_identifier", "predefined_type", "union_type", "intersection_type", "object_type", "literal_type", "generic_type", "lookup_type", "template_literal_type", "conditional_type", "tuple_type", "array_type", "function_type", "constructor_type", "readonly_type", "type_query", "infer_type", "index_type_query", "nested_type_identifier":
		return node
	case "identifier":
		if ctx.hasTypeIdentifierSym {
			return newLeafNodeInArena(node.ownerArena, ctx.typeIdentifierSym, ctx.typeIdentifierNamed, node.startByte, node.endByte, node.startPoint, node.endPoint)
		}
		return node
	case "binary_expression":
		if len(node.children) != 3 || node.children[1] == nil {
			return nil
		}
		var typeSym Symbol
		var typeNamed bool
		switch node.children[1].Type(ctx.lang) {
		case "|":
			typeSym = ctx.unionTypeSym
			typeNamed = ctx.unionTypeNamed
		case "&":
			typeSym = ctx.intersectionTypeSym
			typeNamed = ctx.intersectionTypeNamed
		default:
			return nil
		}
		leftType := normalizeTypeScriptTypeExpression(node.children[0], ctx)
		rightType := normalizeTypeScriptTypeExpression(node.children[2], ctx)
		if leftType == nil || rightType == nil {
			return nil
		}
		children := cloneNodeSliceInArena(node.ownerArena, []*Node{leftType, node.children[1], rightType})
		return newParentNodeInArena(node.ownerArena, typeSym, typeNamed, children, nil, node.productionID)
	case "object":
		return rewriteTypeScriptObjectExpressionAsType(node, ctx)
	default:
		return nil
	}
}

func rewriteTypeScriptObjectExpressionAsType(node *Node, ctx *typeScriptNormalizationContext) *Node {
	if node == nil || ctx == nil || ctx.lang == nil || node.Type(ctx.lang) != "object" {
		return nil
	}
	children := cloneNodeSliceInArena(node.ownerArena, node.children)
	changed := false
	for i, child := range children {
		if child == nil || child.Type(ctx.lang) != "pair" {
			continue
		}
		propSig := rewriteTypeScriptObjectPairAsPropertySignature(child, ctx)
		if propSig == nil {
			return nil
		}
		children[i] = propSig
		changed = true
	}
	if !changed && len(children) != 2 {
		return nil
	}
	return newParentNodeInArena(node.ownerArena, ctx.objectTypeSym, ctx.objectTypeNamed, children, nil, node.productionID)
}

func rewriteTypeScriptObjectPairAsPropertySignature(node *Node, ctx *typeScriptNormalizationContext) *Node {
	if node == nil || ctx == nil || ctx.lang == nil || node.Type(ctx.lang) != "pair" || len(node.children) < 3 {
		return nil
	}
	key := node.children[0]
	colon := node.children[1]
	value := node.children[len(node.children)-1]
	if key == nil || colon == nil || value == nil || key.Type(ctx.lang) != "property_identifier" || colon.Type(ctx.lang) != ":" {
		return nil
	}
	valueType := normalizeTypeScriptTypeExpression(value, ctx)
	if valueType == nil {
		return nil
	}
	typeAnnChildren := cloneNodeSliceInArena(node.ownerArena, []*Node{colon, valueType})
	typeAnnotation := newParentNodeInArena(node.ownerArena, ctx.typeAnnotationSym, ctx.typeAnnotationNamed, typeAnnChildren, nil, 0)
	propChildren := cloneNodeSliceInArena(node.ownerArena, []*Node{key, typeAnnotation})
	return newParentNodeInArena(node.ownerArena, ctx.propertySignatureSym, ctx.propertySignatureNamed, propChildren, nil, node.productionID)
}

func typeScriptGenericCallArgumentChildren(paren *Node, sequenceExpressionSym Symbol) []*Node {
	if paren == nil {
		return nil
	}
	if len(paren.children) != 3 || paren.children[1] == nil || paren.children[1].symbol != sequenceExpressionSym {
		return append([]*Node(nil), paren.children...)
	}
	seq := paren.children[1]
	out := make([]*Node, 0, len(seq.children)+2)
	out = append(out, paren.children[0])
	out = append(out, seq.children...)
	out = append(out, paren.children[2])
	return out
}

func normalizeTypeScriptGenericCallTypeArgument(node *Node, ctx *typeScriptNormalizationContext) *Node {
	if node == nil || ctx == nil || ctx.lang == nil {
		return nil
	}
	switch node.Type(ctx.lang) {
	case "predefined_type":
		return node
	case "type_identifier":
		if ctx.hasTypeIdentifierSym {
			return node
		}
	case "identifier":
		if typeKeywordSym, ok := typeScriptPredefinedTypeSymbol(ctx.lang, node.Text(ctx.source)); ok {
			typeKeywordNamed := int(typeKeywordSym) < len(ctx.lang.SymbolMetadata) && ctx.lang.SymbolMetadata[typeKeywordSym].Named
			typeLeaf := newLeafNodeInArena(node.ownerArena, typeKeywordSym, typeKeywordNamed, node.startByte, node.endByte, node.startPoint, node.endPoint)
			return newParentNodeInArena(node.ownerArena, ctx.predefinedTypeSym, ctx.predefinedTypeNamed, []*Node{typeLeaf}, nil, 0)
		}
		if ctx.hasTypeIdentifierSym {
			typeIdentifierNamed := int(ctx.typeIdentifierSym) < len(ctx.lang.SymbolMetadata) && ctx.lang.SymbolMetadata[ctx.typeIdentifierSym].Named
			return newLeafNodeInArena(node.ownerArena, ctx.typeIdentifierSym, typeIdentifierNamed, node.startByte, node.endByte, node.startPoint, node.endPoint)
		}
	}
	return nil
}

func typeScriptPredefinedTypeSymbol(lang *Language, text string) (Symbol, bool) {
	if lang == nil {
		return 0, false
	}
	switch text {
	case "any", "bigint", "boolean", "never", "number", "object", "string", "symbol", "undefined", "unknown", "void":
		return lang.SymbolByName(text)
	default:
		return 0, false
	}
}

func rewriteJavaScriptTopLevelObjectLiteral(node *Node, lang *Language, arena *nodeArena, exprSym Symbol, exprNamed bool, objectSym Symbol, objectNamed bool, pairSym Symbol, pairNamed bool, propSym Symbol) (*Node, bool) {
	if node == nil || lang == nil || node.Type(lang) != "statement_block" || len(node.children) != 3 {
		return nil, false
	}
	if node.children[0] == nil || node.children[0].Type(lang) != "{" || node.children[2] == nil || node.children[2].Type(lang) != "}" {
		return nil, false
	}
	label := node.children[1]
	if label == nil || label.Type(lang) != "labeled_statement" || len(label.children) != 3 {
		return nil, false
	}
	key := label.children[0]
	colon := label.children[1]
	valueStmt := label.children[2]
	if key == nil || key.Type(lang) != "statement_identifier" || colon == nil || colon.Type(lang) != ":" || valueStmt == nil || valueStmt.Type(lang) != "expression_statement" || len(valueStmt.children) != 1 || valueStmt.children[0] == nil {
		return nil, false
	}
	pair := newParentNodeInArena(arena, pairSym, pairNamed, []*Node{
		aliasedNodeInArena(arena, lang, key, propSym),
		colon,
		valueStmt.children[0],
	}, nil, 0)
	for fieldIdx, fieldName := range lang.FieldNames {
		switch fieldName {
		case "key":
			ensureNodeFieldStorage(pair, len(pair.children))
			pair.fieldIDs[0] = FieldID(fieldIdx)
			pair.fieldSources[0] = fieldSourceDirect
		case "value":
			ensureNodeFieldStorage(pair, len(pair.children))
			pair.fieldIDs[2] = FieldID(fieldIdx)
			pair.fieldSources[2] = fieldSourceDirect
		}
	}
	object := newParentNodeInArena(arena, objectSym, objectNamed, []*Node{
		node.children[0],
		pair,
		node.children[2],
	}, nil, 0)
	return newParentNodeInArena(arena, exprSym, exprNamed, []*Node{object}, nil, 0), true
}

func javaScriptSymbolMeta(lang *Language, name string) (Symbol, bool, bool) {
	if lang == nil {
		return 0, false, false
	}
	sym, ok := symbolByName(lang, name)
	if !ok {
		return 0, false, false
	}
	named := false
	if int(sym) < len(lang.SymbolMetadata) {
		named = lang.SymbolMetadata[sym].Named
	}
	return sym, named, true
}

func symbolByName(lang *Language, name string) (Symbol, bool) {
	if lang == nil {
		return 0, false
	}
	for i, symName := range lang.SymbolNames {
		if symName == name {
			return Symbol(i), true
		}
	}
	return 0, false
}
