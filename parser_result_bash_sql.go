package gotreesitter

func normalizeBashProgramVariableAssignments(root *Node, lang *Language) {
	if root == nil || lang == nil || lang.Name != "bash" || root.Type(lang) != "program" || len(root.children) == 0 {
		return
	}
	normalizeBashVariableAssignmentsInNode(root, lang)
}

func normalizeBashVariableAssignmentsInNode(node *Node, lang *Language) {
	if node == nil || lang == nil || len(node.children) == 0 {
		return
	}
	for _, child := range node.children {
		if child != nil {
			normalizeBashVariableAssignmentsInNode(child, lang)
		}
	}
	out := make([]*Node, 0, len(node.children))
	changed := false
	for _, child := range node.children {
		if child == nil {
			continue
		}
		if child.Type(lang) == "variable_assignments" && bashAllVariableAssignments(child, lang) && bashShouldSplitVariableAssignments(node.Type(lang)) {
			out = append(out, child.children...)
			changed = true
			continue
		}
		out = append(out, child)
	}
	if !changed {
		assignBashIfConditionField(node, lang)
		return
	}
	if node.ownerArena != nil {
		buf := node.ownerArena.allocNodeSlice(len(out))
		copy(buf, out)
		out = buf
	}
	node.children = out
	node.fieldIDs = nil
	node.fieldSources = nil
	assignBashIfConditionField(node, lang)
}

func bashAllVariableAssignments(node *Node, lang *Language) bool {
	if node == nil || lang == nil || len(node.children) < 2 {
		return false
	}
	for _, child := range node.children {
		if child == nil || child.Type(lang) != "variable_assignment" {
			return false
		}
	}
	return true
}

func bashShouldSplitVariableAssignments(parentType string) bool {
	switch parentType {
	case "command", "redirected_statement", "declaration_command", "unset_command":
		return false
	default:
		return true
	}
}

func assignBashIfConditionField(node *Node, lang *Language) {
	if node == nil || lang == nil || node.Type(lang) != "if_statement" || len(node.children) <= 1 {
		return
	}
	fid, ok := lang.FieldByName("condition")
	if !ok {
		return
	}
	ensureNodeFieldStorage(node, len(node.children))
	thenIndex := -1
	for i, child := range node.children {
		if child != nil && child.Type(lang) == "then" {
			thenIndex = i
			break
		}
	}
	if thenIndex < 0 {
		thenIndex = len(node.children)
	}
	for i := 1; i < thenIndex; i++ {
		if node.children[i] == nil {
			continue
		}
		node.fieldIDs[i] = fid
		node.fieldSources[i] = fieldSourceDirect
	}
}

func normalizeSQLRecoveredSelectRoot(root *Node, lang *Language) {
	if root == nil || lang == nil || lang.Name != "sql" || root.Type(lang) != "source_file" || len(root.children) < 3 {
		return
	}
	if !sqlLooksLikeFlatRecoveredSelect(root, lang) {
		return
	}
	selectStmtSym, ok := symbolByName(lang, "select_statement")
	if !ok {
		return
	}
	selectClauseSym, ok := symbolByName(lang, "select_clause")
	if !ok {
		return
	}
	selectClauseBodySym, ok := symbolByName(lang, "select_clause_body")
	if !ok {
		return
	}
	nullParentSym, ok := findVisibleSymbolByName(lang, "NULL", true)
	if !ok {
		return
	}
	nullLeafSym, ok := findVisibleSymbolByName(lang, "NULL", false)
	if !ok {
		return
	}
	bodyChildren := sqlFlattenRecoveredSelectBody(root.children[1:], nil, lang)
	if !sqlNeedsRecoveredMissingNull(bodyChildren, lang) {
		return
	}
	bodyChildren = append(bodyChildren, sqlRecoveredNullNode(root.ownerArena, bodyChildren[len(bodyChildren)-1], nullParentSym, nullLeafSym))
	if root.ownerArena != nil {
		buf := root.ownerArena.allocNodeSlice(len(bodyChildren))
		copy(buf, bodyChildren)
		bodyChildren = buf
	}
	selectClauseBody := newParentNodeInArena(root.ownerArena, selectClauseBodySym, lang.SymbolMetadata[selectClauseBodySym].Named, bodyChildren, nil, 0)
	selectClause := newParentNodeInArena(root.ownerArena, selectClauseSym, lang.SymbolMetadata[selectClauseSym].Named, []*Node{root.children[0], selectClauseBody}, nil, 0)
	selectStatement := newParentNodeInArena(root.ownerArena, selectStmtSym, lang.SymbolMetadata[selectStmtSym].Named, []*Node{selectClause}, nil, 0)
	children := []*Node{selectStatement}
	if root.ownerArena != nil {
		buf := root.ownerArena.allocNodeSlice(1)
		buf[0] = selectStatement
		children = buf
	}
	root.children = children
	root.fieldIDs = nil
	root.fieldSources = nil
	root.hasError = selectStatement.HasError()
}

func sqlLooksLikeFlatRecoveredSelect(root *Node, lang *Language) bool {
	if len(root.children) < 3 || root.children[0] == nil || root.children[0].Type(lang) != "SELECT" {
		return false
	}
	sawRepeat := false
	for _, child := range root.children[1:] {
		if child == nil {
			continue
		}
		switch child.Type(lang) {
		case "_aliasable_expression", "_expression", ",", "comment":
			continue
		case "select_clause_body_repeat1":
			sawRepeat = true
			continue
		default:
			return false
		}
	}
	return sawRepeat
}

func sqlFlattenRecoveredSelectBody(nodes []*Node, out []*Node, lang *Language) []*Node {
	for _, node := range nodes {
		if node == nil {
			continue
		}
		switch node.Type(lang) {
		case "_aliasable_expression", "_expression", "select_clause_body_repeat1":
			if len(node.children) > 0 {
				out = sqlFlattenRecoveredSelectBody(node.children, out, lang)
				continue
			}
		}
		out = append(out, node)
	}
	return out
}

func sqlNeedsRecoveredMissingNull(children []*Node, lang *Language) bool {
	last, prev := sqlLastAndPrevNonNilChild(children)
	if last == nil {
		return false
	}
	if last.Type(lang) == "NULL" {
		return false
	}
	if last.Type(lang) == "comment" && prev != nil && prev.Type(lang) == "," {
		return true
	}
	return last.Type(lang) == ","
}

func sqlLastAndPrevNonNilChild(children []*Node) (last *Node, prev *Node) {
	for i := len(children) - 1; i >= 0; i-- {
		if children[i] == nil {
			continue
		}
		last = children[i]
		for j := i - 1; j >= 0; j-- {
			if children[j] != nil {
				prev = children[j]
				break
			}
		}
		return last, prev
	}
	return nil, nil
}

func sqlRecoveredNullNode(arena *nodeArena, anchor *Node, nullParentSym, nullLeafSym Symbol) *Node {
	if anchor == nil {
		return nil
	}
	leaf := newLeafNodeInArena(arena, nullLeafSym, false, anchor.endByte, anchor.endByte, anchor.endPoint, anchor.endPoint)
	leaf.isMissing = true
	leaf.hasError = true
	node := newParentNodeInArena(arena, nullParentSym, true, []*Node{leaf}, nil, 0)
	node.hasError = true
	return node
}
