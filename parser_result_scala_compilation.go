package gotreesitter

import "bytes"

func normalizeScalaCompilationUnitRoot(root *Node, source []byte, lang *Language) {
	if root == nil || lang == nil || lang.Name != "scala" || root.Type(lang) != "ERROR" {
		return
	}
	sym, ok := symbolByName(lang, "compilation_unit")
	if !ok {
		return
	}
	if children, ok := scalaRebuildCompilationUnitChildren(source, lang, root.ownerArena); ok {
		root.children = children
		root.fieldIDs = nil
		root.fieldSources = nil
		root.symbol = sym
		root.isNamed = int(sym) < len(lang.SymbolMetadata) && lang.SymbolMetadata[sym].Named
		populateParentNode(root, root.children)
		root.hasError = false
		for _, child := range root.children {
			if child != nil && (child.IsError() || child.HasError()) {
				root.hasError = true
				break
			}
		}
		if !root.hasError {
			return
		}
	}
	if !rootLooksLikeScalaCompilationUnit(root, lang) {
		return
	}
	root.symbol = sym
	root.isNamed = int(sym) < len(lang.SymbolMetadata) && lang.SymbolMetadata[sym].Named
	root.hasError = false
	for _, child := range root.children {
		if child != nil && (child.IsError() || child.HasError()) {
			root.hasError = true
			break
		}
	}
}

func scalaRebuildCompilationUnitChildren(source []byte, lang *Language, arena *nodeArena) ([]*Node, bool) {
	if lang == nil || len(source) == 0 {
		return nil, false
	}
	spans := scalaCompilationUnitSpans(source)
	if len(spans) == 0 {
		return nil, false
	}
	sawPackageOrImport := false
	sawDefinition := false
	for _, span := range spans {
		switch span.kind {
		case scalaTemplateMemberPackage, scalaTemplateMemberImport:
			sawPackageOrImport = true
		case scalaTemplateMemberClass, scalaTemplateMemberObject, scalaTemplateMemberTrait, scalaTemplateMemberEnum:
			sawDefinition = true
		}
	}
	if !sawPackageOrImport || !sawDefinition {
		return nil, false
	}
	children := make([]*Node, 0, len(spans))
	for _, span := range spans {
		node, ok := scalaRecoverCompilationUnitMemberNode(source, span, lang, arena)
		if !ok || node == nil {
			switch span.kind {
			case scalaTemplateMemberComment, scalaTemplateMemberBlockComment:
				continue
			default:
				return nil, false
			}
		}
		children = append(children, node)
	}
	if len(children) == 0 {
		return nil, false
	}
	if arena != nil {
		buf := arena.allocNodeSlice(len(children))
		copy(buf, children)
		children = buf
	}
	return children, true
}

func scalaCompilationUnitSpans(source []byte) []scalaTemplateMemberSpan {
	var spans []scalaTemplateMemberSpan
	pos := 0
	limit := len(source)
	for pos < limit {
		start, kind, ok := scalaFindNextCompilationUnitMemberStart(source, pos, limit)
		if !ok {
			break
		}
		end := scalaFindCompilationUnitMemberEnd(source, start, limit, kind)
		if end <= start {
			pos = start + 1
			continue
		}
		spans = append(spans, scalaTemplateMemberSpan{
			start: uint32(start),
			end:   uint32(end),
			kind:  kind,
		})
		pos = end
	}
	return spans
}

func scalaFindNextCompilationUnitMemberStart(source []byte, pos, limit int) (int, scalaTemplateMemberKind, bool) {
	braceDepth := 0
	parenDepth := 0
	bracketDepth := 0
	inLineComment := false
	inBlockComment := false
	var stringQuote byte
	tripleQuote := false
	lineStart := true
	for i := pos; i < limit; i++ {
		ch := source[i]
		next := byte(0)
		if i+1 < limit {
			next = source[i+1]
		}
		if inLineComment {
			if ch == '\n' {
				inLineComment = false
				lineStart = true
			}
			continue
		}
		if inBlockComment {
			if ch == '*' && next == '/' {
				inBlockComment = false
				i++
				continue
			}
			if ch == '\n' {
				lineStart = true
			}
			continue
		}
		if stringQuote != 0 {
			if tripleQuote {
				if i+2 < limit && source[i] == stringQuote && source[i+1] == stringQuote && source[i+2] == stringQuote {
					stringQuote = 0
					tripleQuote = false
					i += 2
				}
				continue
			}
			if ch == '\\' {
				i++
				continue
			}
			if ch == stringQuote {
				stringQuote = 0
			}
			continue
		}
		if lineStart {
			j := skipHorizontalTrivia(source, i, limit)
			if braceDepth == 0 && parenDepth == 0 && bracketDepth == 0 {
				switch {
				case j+1 < limit && source[j] == '/' && source[j+1] == '/':
					return j, scalaTemplateMemberComment, true
				case j+1 < limit && source[j] == '/' && source[j+1] == '*':
					return j, scalaTemplateMemberBlockComment, true
				default:
					if kind, ok := scalaCompilationUnitKindAt(source, j, limit); ok {
						return j, kind, true
					}
				}
			}
			lineStart = false
		}
		switch {
		case ch == '/' && next == '/':
			inLineComment = true
			i++
			continue
		case ch == '/' && next == '*':
			inBlockComment = true
			i++
			continue
		case ch == '"' || ch == '\'':
			if i+2 < limit && source[i+1] == ch && source[i+2] == ch {
				stringQuote = ch
				tripleQuote = true
				i += 2
				continue
			}
			stringQuote = ch
			tripleQuote = false
			continue
		case ch == '{':
			braceDepth++
		case ch == '}':
			if braceDepth > 0 {
				braceDepth--
			}
		case ch == '(':
			parenDepth++
		case ch == ')':
			if parenDepth > 0 {
				parenDepth--
			}
		case ch == '[':
			bracketDepth++
		case ch == ']':
			if bracketDepth > 0 {
				bracketDepth--
			}
		}
		if ch == '\n' {
			lineStart = true
		}
	}
	return 0, scalaTemplateMemberUnknown, false
}

func scalaCompilationUnitKindAt(source []byte, pos, limit int) (scalaTemplateMemberKind, bool) {
	if pos >= limit {
		return scalaTemplateMemberUnknown, false
	}
	switch {
	case bytes.HasPrefix(source[pos:limit], []byte("package ")):
		return scalaTemplateMemberPackage, true
	case bytes.HasPrefix(source[pos:limit], []byte("import ")):
		return scalaTemplateMemberImport, true
	case bytes.HasPrefix(source[pos:limit], []byte("final class ")):
		return scalaTemplateMemberClass, true
	case bytes.HasPrefix(source[pos:limit], []byte("implicit class ")):
		return scalaTemplateMemberClass, true
	case bytes.HasPrefix(source[pos:limit], []byte("class ")):
		return scalaTemplateMemberClass, true
	case bytes.HasPrefix(source[pos:limit], []byte("object ")):
		return scalaTemplateMemberObject, true
	case bytes.HasPrefix(source[pos:limit], []byte("trait ")):
		return scalaTemplateMemberTrait, true
	case bytes.HasPrefix(source[pos:limit], []byte("enum ")):
		return scalaTemplateMemberEnum, true
	default:
		return scalaTemplateMemberUnknown, false
	}
}

func scalaFindCompilationUnitMemberEnd(source []byte, start, limit int, kind scalaTemplateMemberKind) int {
	switch kind {
	case scalaTemplateMemberComment:
		end := start
		for end < limit && source[end] != '\n' && source[end] != '\r' {
			end++
		}
		return trimTrailingHorizontalAndVerticalTrivia(source, start, end)
	case scalaTemplateMemberBlockComment:
		end := start + 2
		for end+1 < limit {
			if source[end] == '*' && source[end+1] == '/' {
				end += 2
				return trimTrailingHorizontalAndVerticalTrivia(source, start, end)
			}
			end++
		}
		return trimTrailingHorizontalAndVerticalTrivia(source, start, limit)
	case scalaTemplateMemberPackage, scalaTemplateMemberImport:
		end := start
		for end < limit && source[end] != '\n' && source[end] != '\r' {
			end++
		}
		return trimTrailingHorizontalAndVerticalTrivia(source, start, end)
	case scalaTemplateMemberObject, scalaTemplateMemberClass, scalaTemplateMemberTrait, scalaTemplateMemberEnum:
		openRel := bytes.IndexByte(source[start:limit], '{')
		if openRel < 0 {
			end := start
			for end < limit && source[end] != '\n' && source[end] != '\r' {
				end++
			}
			return trimTrailingHorizontalAndVerticalTrivia(source, start, end)
		}
		openPos := start + openRel
		if closePos := scalaFindMatchingBraceByteWithTrivia(source, openPos, uint32(limit)); closePos >= 0 {
			return closePos + 1
		}
		return trimTrailingHorizontalAndVerticalTrivia(source, start, limit)
	default:
		return 0
	}
}

func scalaRecoverCompilationUnitMemberNode(source []byte, span scalaTemplateMemberSpan, lang *Language, arena *nodeArena) (*Node, bool) {
	switch span.kind {
	case scalaTemplateMemberPackage:
		return scalaRecoverTopLevelNamedNodeFromRange(source, span.start, span.end, lang, arena, "package_clause")
	case scalaTemplateMemberImport:
		return scalaRecoverTopLevelNamedNodeFromRange(source, span.start, span.end, lang, arena, "import_declaration")
	case scalaTemplateMemberObject:
		return scalaRecoverTopLevelObjectNodeFromRange(source, span.start, span.end, lang, arena)
	case scalaTemplateMemberClass:
		return scalaRecoverTopLevelClassNodeFromRange(source, span.start, span.end, lang, arena)
	case scalaTemplateMemberTrait:
		return scalaRecoverTopLevelNamedNodeFromRange(source, span.start, span.end, lang, arena, "trait_definition")
	case scalaTemplateMemberEnum:
		return scalaRecoverTopLevelNamedNodeFromRange(source, span.start, span.end, lang, arena, "enum_definition")
	case scalaTemplateMemberComment:
		return scalaRecoverTopLevelNamedNodeFromRange(source, span.start, span.end, lang, arena, "comment")
	case scalaTemplateMemberBlockComment:
		return scalaRecoverTopLevelNamedNodeFromRange(source, span.start, span.end, lang, arena, "block_comment")
	default:
		return nil, false
	}
}

func normalizeScalaImportPathFields(root *Node, lang *Language) {
	if root == nil || lang == nil || lang.Name != "scala" {
		return
	}
	pathID, ok := lang.FieldByName("path")
	if !ok || pathID == 0 {
		return
	}
	var walk func(*Node)
	walk = func(n *Node) {
		if n == nil {
			return
		}
		if n.Type(lang) == "import_declaration" && len(n.children) > 0 {
			for i, child := range n.children {
				if child == nil || child.Type(lang) != "." {
					continue
				}
				prevHasPath := i > 0 && i-1 < len(n.fieldIDs) && n.fieldIDs[i-1] == pathID
				nextHasPath := i+1 < len(n.children) && i+1 < len(n.fieldIDs) && n.fieldIDs[i+1] == pathID
				if !prevHasPath || !nextHasPath {
					continue
				}
				ensureNodeFieldStorage(n, len(n.children))
				n.fieldIDs[i] = pathID
				n.fieldSources[i] = fieldSourceDirect
			}
		}
		for _, child := range n.children {
			walk(child)
		}
	}
	walk(root)
}

func normalizeScalaDefinitionFields(root *Node, source []byte, lang *Language) {
	if root == nil || lang == nil || lang.Name != "scala" {
		return
	}
	nameID, _ := lang.FieldByName("name")
	classParamsID, _ := lang.FieldByName("class_parameters")
	extendID, _ := lang.FieldByName("extend")
	parametersID, _ := lang.FieldByName("parameters")
	patternID, _ := lang.FieldByName("pattern")
	valueID, _ := lang.FieldByName("value")
	typeID, _ := lang.FieldByName("type")
	returnTypeID, _ := lang.FieldByName("return_type")
	bodyID, ok := lang.FieldByName("body")
	if !ok {
		return
	}
	conditionID, _ := lang.FieldByName("condition")
	consequenceID, _ := lang.FieldByName("consequence")
	alternativeID, _ := lang.FieldByName("alternative")
	var walk func(*Node)
	walk = func(n *Node) {
		if n == nil {
			return
		}
		switch n.Type(lang) {
		case "object_definition", "class_definition", "trait_definition", "enum_definition":
			for i, child := range n.children {
				if child == nil {
					continue
				}
				var want FieldID
				switch n.Type(lang) {
				case "object_definition", "trait_definition":
					switch child.Type(lang) {
					case "identifier":
						want = nameID
					case "extends_clause":
						want = extendID
					case "template_body":
						want = bodyID
					}
				case "class_definition":
					switch child.Type(lang) {
					case "identifier":
						want = nameID
					case "class_parameters":
						want = classParamsID
					case "extends_clause":
						want = extendID
					case "template_body":
						want = bodyID
					}
				case "enum_definition":
					switch child.Type(lang) {
					case "identifier":
						want = nameID
					case "enum_body":
						want = bodyID
					}
				}
				if want == 0 {
					continue
				}
				ensureNodeFieldStorage(n, len(n.children))
				if n.fieldIDs[i] == 0 {
					n.fieldIDs[i] = want
					n.fieldSources[i] = fieldSourceDirect
				}
			}
		case "function_definition":
			for i, child := range n.children {
				if child == nil {
					continue
				}
				var want FieldID
				switch {
				case child.Type(lang) == "identifier":
					want = nameID
				case child.Type(lang) == "parameters":
					want = parametersID
				case i > 0 && n.children[i-1] != nil && n.children[i-1].Type(lang) == ":" && child.isNamed:
					want = returnTypeID
				case i > 0 && n.children[i-1] != nil && (n.children[i-1].Type(lang) == "=" || n.children[i-1].Type(lang) == "=>") && child.isNamed:
					want = bodyID
				}
				if want == 0 {
					continue
				}
				ensureNodeFieldStorage(n, len(n.children))
				if n.fieldIDs[i] == 0 {
					n.fieldIDs[i] = want
					n.fieldSources[i] = fieldSourceDirect
				}
			}
		case "val_definition", "var_definition":
			patternAssigned := false
			typePending := false
			valuePending := false
			for i, child := range n.children {
				if child == nil {
					continue
				}
				switch child.Type(lang) {
				case ":":
					typePending = true
					continue
				case "=":
					valuePending = true
					typePending = false
					continue
				case "modifiers":
					continue
				}
				if !child.isNamed {
					continue
				}
				var want FieldID
				switch {
				case valuePending:
					want = valueID
					valuePending = false
				case typePending:
					want = typeID
					typePending = false
				case !patternAssigned:
					want = patternID
					patternAssigned = true
				}
				if want == 0 {
					continue
				}
				ensureNodeFieldStorage(n, len(n.children))
				if n.fieldIDs[i] == 0 {
					n.fieldIDs[i] = want
					n.fieldSources[i] = fieldSourceDirect
				}
			}
		case "if_expression":
			conditionAssigned := false
			consequenceAssigned := false
			afterElse := false
			for i, child := range n.children {
				if child == nil {
					continue
				}
				if child.Type(lang) == "else" {
					afterElse = true
					continue
				}
				if !child.isNamed {
					continue
				}
				var want FieldID
				switch {
				case !conditionAssigned:
					want = conditionID
					conditionAssigned = true
				case !afterElse && !consequenceAssigned:
					want = consequenceID
					consequenceAssigned = true
				case afterElse:
					want = alternativeID
				}
				if want == 0 {
					continue
				}
				ensureNodeFieldStorage(n, len(n.children))
				if n.fieldIDs[i] == 0 {
					n.fieldIDs[i] = want
					n.fieldSources[i] = fieldSourceDirect
				}
			}
		case "case_block":
			for i := 0; i+1 < len(n.children); i++ {
				curr := n.children[i]
				if curr == nil || curr.Type(lang) != "case_clause" {
					continue
				}
				next := scalaNextCaseClauseBoundaryNode(n.children, i, lang)
				if next == nil {
					continue
				}
				if curr.endByte >= next.startByte {
					continue
				}
				gap := source[curr.endByte:next.startByte]
				if !bytesAreTrivia(gap) || !bytesContainLineBreak(gap) {
					continue
				}
				extendNodeEndTo(curr, next.startByte, source)
			}
		}
		for _, child := range n.children {
			walk(child)
		}
	}
	walk(root)
}

func normalizeScalaTemplateBodyFunctionAnnotations(root *Node, source []byte, lang *Language) {
	if root == nil || lang == nil || lang.Name != "scala" || len(source) == 0 {
		return
	}
	var walk func(*Node)
	walk = func(n *Node) {
		if n == nil {
			return
		}
		if n.Type(lang) == "template_body" {
			for i, child := range n.children {
				if child == nil || child.Type(lang) != "function_definition" || len(child.children) == 0 {
					continue
				}
				if child.children[0] != nil && child.children[0].Type(lang) == "annotation" {
					continue
				}
				gapStart := n.startByte
				if i > 0 && n.children[i-1] != nil {
					gapStart = n.children[i-1].endByte
				}
				annotations := scalaRecoverLeadingAnnotations(source, gapStart, child.startByte, child.endByte, lang, child.ownerArena)
				if len(annotations) == 0 {
					continue
				}
				newChildren := make([]*Node, 0, len(annotations)+len(child.children))
				newChildren = append(newChildren, annotations...)
				newChildren = append(newChildren, child.children...)
				if child.ownerArena != nil {
					buf := child.ownerArena.allocNodeSlice(len(newChildren))
					copy(buf, newChildren)
					newChildren = buf
				}
				child.children = newChildren
				if len(child.fieldIDs) > 0 {
					fieldIDs := make([]FieldID, 0, len(child.children))
					for range annotations {
						fieldIDs = append(fieldIDs, 0)
					}
					fieldIDs = append(fieldIDs, child.fieldIDs...)
					child.fieldIDs = fieldIDs
				}
				if len(child.fieldSources) > 0 {
					fieldSources := make([]uint8, 0, len(child.children))
					for range annotations {
						fieldSources = append(fieldSources, fieldSourceNone)
					}
					fieldSources = append(fieldSources, child.fieldSources...)
					child.fieldSources = fieldSources
				}
				populateParentNode(child, child.children)
			}
		}
		for _, child := range n.children {
			walk(child)
		}
	}
	walk(root)
}

func normalizeScalaCaseClauseEnds(root *Node, source []byte, lang *Language) {
	if root == nil || lang == nil || lang.Name != "scala" || len(source) == 0 {
		return
	}
	var walk func(*Node)
	walk = func(n *Node) {
		if n == nil {
			return
		}
		if n.Type(lang) == "case_block" {
			for i := 0; i+1 < len(n.children); i++ {
				curr := n.children[i]
				if curr == nil || curr.Type(lang) != "case_clause" {
					continue
				}
				next := scalaNextCaseClauseBoundaryNode(n.children, i, lang)
				if next == nil {
					continue
				}
				if curr.endByte >= next.startByte || int(next.startByte) > len(source) {
					continue
				}
				gap := source[curr.endByte:next.startByte]
				if !bytesAreTrivia(gap) || !bytesContainLineBreak(gap) {
					continue
				}
				extendNodeEndTo(curr, next.startByte, source)
			}
		}
		for _, child := range n.children {
			walk(child)
		}
	}
	walk(root)
}

func normalizeScalaTemplateBodyFunctionEnds(root *Node, source []byte, lang *Language) {
	if root == nil || lang == nil || lang.Name != "scala" || len(source) == 0 {
		return
	}
	var walk func(*Node)
	walk = func(n *Node) {
		if n == nil {
			return
		}
		if n.Type(lang) == "template_body" {
			for i := 0; i+1 < len(n.children); i++ {
				curr := n.children[i]
				next := n.children[i+1]
				if curr == nil || next == nil || curr.Type(lang) != "function_definition" || next.IsExtra() {
					continue
				}
				if len(curr.children) == 0 {
					continue
				}
				last := curr.children[len(curr.children)-1]
				if last == nil || last.Type(lang) != "indented_block" {
					continue
				}
				if curr.endByte >= next.startByte || int(next.startByte) > len(source) {
					continue
				}
				gap := source[curr.endByte:next.startByte]
				if !bytesAreTrivia(gap) || !bytesContainLineBreak(gap) {
					continue
				}
				extendNodeEndTo(last, next.startByte, source)
				extendNodeEndTo(curr, next.startByte, source)
			}
		}
		for _, child := range n.children {
			walk(child)
		}
	}
	walk(root)
}

func scalaNextCaseClauseBoundaryNode(children []*Node, start int, lang *Language) *Node {
	for i := start + 1; i < len(children); i++ {
		child := children[i]
		if child == nil {
			continue
		}
		switch child.Type(lang) {
		case "_automatic_semicolon":
			continue
		}
		return child
	}
	return nil
}

func rootLooksLikeScalaCompilationUnit(root *Node, lang *Language) bool {
	if root == nil || lang == nil || len(root.children) == 0 {
		return false
	}
	sawTopLevel := false
	for _, child := range root.children {
		if child == nil {
			continue
		}
		switch child.Type(lang) {
		case "comment",
			"block_comment",
			"package_clause",
			"import_declaration",
			"object_definition",
			"class_definition",
			"trait_definition",
			"enum_definition",
			"function_definition",
			"type_definition",
			"val_definition",
			"var_definition",
			"given_definition":
			sawTopLevel = true
		default:
			return false
		}
	}
	return sawTopLevel
}

func normalizeScalaTrailingCommentOwnership(root *Node, source []byte, lang *Language) {
	if root == nil || len(source) == 0 || lang == nil || lang.Name != "scala" {
		return
	}
	var walk func(*Node, int)
	walk = func(n *Node, depth int) {
		if n == nil || depth > maxTreeWalkDepth {
			return
		}
		normalizeScalaTrailingCommentSiblings(n, source, lang)
		for _, child := range n.children {
			walk(child, depth+1)
		}
	}
	walk(root, 0)
}

func normalizeScalaTrailingCommentSiblings(parent *Node, source []byte, lang *Language) {
	if parent == nil || len(parent.children) < 3 {
		return
	}
	for i := 1; i+1 < len(parent.children); {
		firstComment := parent.children[i]
		if !isScalaCommentNode(firstComment, lang) {
			i++
			continue
		}
		prev := parent.children[i-1]
		body := scalaTrailingCommentTarget(prev, lang)
		if body == nil || body.endByte != firstComment.startByte {
			i++
			continue
		}
		j := i
		for j < len(parent.children) && isScalaCommentNode(parent.children[j], lang) {
			j++
		}
		if j >= len(parent.children) {
			i++
			continue
		}
		next := parent.children[j]
		if next == nil || next.isExtra {
			i++
			continue
		}
		lastComment := parent.children[j-1]

		targetEndByte := lastComment.endByte
		targetEndPoint := lastComment.endPoint
		if lastComment.endByte <= uint32(len(source)) && next.startByte >= lastComment.endByte && next.startByte <= uint32(len(source)) {
			gap := source[lastComment.endByte:next.startByte]
			if bytesAreTrivia(gap) {
				targetEndByte = next.startByte
				targetEndPoint = advancePointByBytes(lastComment.endPoint, gap)
			}
		}

		added := parent.children[i:j]
		rebuiltChildren := make([]*Node, 0, len(body.children)+len(added))
		rebuiltChildren = append(rebuiltChildren, body.children...)
		rebuiltChildren = append(rebuiltChildren, added...)
		body.children = rebuiltChildren

		if len(body.fieldIDs) > 0 {
			rebuiltFieldIDs := make([]FieldID, 0, len(body.fieldIDs)+len(added))
			rebuiltFieldIDs = append(rebuiltFieldIDs, body.fieldIDs...)
			for range added {
				rebuiltFieldIDs = append(rebuiltFieldIDs, 0)
			}
			body.fieldIDs = rebuiltFieldIDs
		}
		if len(body.fieldSources) > 0 {
			rebuiltFieldSources := make([]uint8, 0, len(body.fieldSources)+len(added))
			rebuiltFieldSources = append(rebuiltFieldSources, body.fieldSources...)
			for range added {
				rebuiltFieldSources = append(rebuiltFieldSources, 0)
			}
			body.fieldSources = rebuiltFieldSources
		}
		if targetEndByte > body.endByte {
			body.endByte = targetEndByte
			body.endPoint = targetEndPoint
		}
		if targetEndByte > prev.endByte {
			prev.endByte = targetEndByte
			prev.endPoint = targetEndPoint
		}

		parent.children = append(parent.children[:i], parent.children[j:]...)
		if len(parent.fieldIDs) > 0 {
			parent.fieldIDs = append(parent.fieldIDs[:i], parent.fieldIDs[j:]...)
			if len(parent.fieldSources) > 0 {
				parent.fieldSources = append(parent.fieldSources[:i], parent.fieldSources[j:]...)
			}
		}
	}
}

func isScalaCommentNode(n *Node, lang *Language) bool {
	if n == nil {
		return false
	}
	switch n.Type(lang) {
	case "comment", "block_comment":
		return true
	default:
		return false
	}
}

func scalaTrailingCommentTarget(prev *Node, lang *Language) *Node {
	if prev == nil || lang == nil || len(prev.children) == 0 {
		return nil
	}
	last := prev.children[len(prev.children)-1]
	if last == nil {
		return nil
	}
	switch prev.Type(lang) {
	case "function_definition":
		if last.Type(lang) == "indented_block" {
			return last
		}
	case "trait_definition", "object_definition", "class_definition":
		if last.Type(lang) == "template_body" {
			return last
		}
	case "enum_definition":
		if last.Type(lang) == "enum_body" {
			return last
		}
	}
	return nil
}

func normalizeScalaFunctionModifierFields(root *Node, lang *Language) {
	if root == nil || lang == nil || lang.Name != "scala" {
		return
	}
	returnTypeID, ok := lang.FieldByName("return_type")
	if !ok {
		return
	}
	var walk func(*Node, int)
	walk = func(n *Node, depth int) {
		if n == nil || depth > maxTreeWalkDepth {
			return
		}
		if n.Type(lang) == "function_definition" {
			for i, child := range n.children {
				if child == nil || child.Type(lang) != "modifiers" {
					continue
				}
				if i < len(n.fieldIDs) && n.fieldIDs[i] == returnTypeID {
					n.fieldIDs[i] = 0
					if i < len(n.fieldSources) {
						n.fieldSources[i] = fieldSourceNone
					}
				}
			}
		}
		for _, child := range n.children {
			walk(child, depth+1)
		}
	}
	walk(root, 0)
}

func normalizeScalaInterpolatedStringTail(root *Node, source []byte, lang *Language) {
	if root == nil || len(source) == 0 || lang == nil || lang.Name != "scala" {
		return
	}
	var walk func(*Node, int)
	walk = func(n *Node, depth int) {
		if n == nil || depth > maxTreeWalkDepth {
			return
		}
		if n.Type(lang) == "interpolated_string_expression" && len(n.children) >= 2 {
			inner := n.children[1]
			if inner != nil && inner.Type(lang) == "interpolated_string" {
				normalizeScalaSingleLineInterpolatedStringTail(n, inner, source)
			}
		}
		if n.Type(lang) == "field_expression" && len(n.children) >= 2 {
			left := n.children[0]
			right := n.children[1]
			if left != nil && right != nil &&
				left.Type(lang) == "interpolated_string_expression" &&
				right.Type(lang) == "." &&
				left.endByte < right.startByte &&
				right.startByte <= uint32(len(source)) &&
				scalaInterpolatedStringTail(source[left.endByte:right.startByte]) {
				extendNodeEndTo(left, right.startByte, source)
				if len(left.children) >= 2 {
					inner := left.children[1]
					if inner != nil && inner.Type(lang) == "interpolated_string" {
						extendNodeEndTo(inner, right.startByte, source)
					}
				}
			}
		}
		for _, child := range n.children {
			walk(child, depth+1)
		}
		if n.Type(lang) == "infix_expression" && len(n.children) > 0 {
			last := n.children[len(n.children)-1]
			if last != nil && last.Type(lang) == "interpolated_string_expression" && n.endByte < last.endByte {
				extendNodeEndTo(n, last.endByte, source)
			}
		}
	}
	walk(root, 0)
}

func normalizeScalaSingleLineInterpolatedStringTail(expr *Node, inner *Node, source []byte) {
	if expr == nil || inner == nil || inner.startByte >= uint32(len(source)) {
		return
	}
	if source[inner.startByte] != '"' {
		return
	}
	if inner.startByte+2 < uint32(len(source)) &&
		source[inner.startByte+1] == '"' &&
		source[inner.startByte+2] == '"' {
		return
	}
	end, ok := scanScalaSingleLineStringTail(source, inner.endByte)
	if !ok || end <= inner.endByte {
		return
	}
	extendNodeEndTo(inner, end, source)
	extendNodeEndTo(expr, end, source)
}

func scalaInterpolatedStringTail(gap []byte) bool {
	if len(gap) == 0 {
		return false
	}
	hasQuote := false
	for _, c := range gap {
		switch c {
		case ' ', '\t', '\n', '\r', '\f', '|', '}', '"':
			if c == '"' {
				hasQuote = true
			}
		default:
			return false
		}
	}
	return hasQuote
}

func scanScalaSingleLineStringTail(source []byte, start uint32) (uint32, bool) {
	if start >= uint32(len(source)) {
		return 0, false
	}
	for i := start; i < uint32(len(source)); i++ {
		switch source[i] {
		case '\n', '\r':
			return 0, false
		case '"':
			if !isEscapedQuote(source, i) {
				return i + 1, true
			}
		}
	}
	return 0, false
}

func isEscapedQuote(source []byte, idx uint32) bool {
	if idx == 0 || idx > uint32(len(source)) {
		return false
	}
	backslashes := 0
	for i := int(idx) - 1; i >= 0 && source[i] == '\\'; i-- {
		backslashes++
	}
	return backslashes%2 == 1
}

func extendNodeEndTo(n *Node, end uint32, source []byte) {
	if n == nil || end <= n.endByte || end > uint32(len(source)) {
		return
	}
	gap := source[n.endByte:end]
	n.endByte = end
	n.endPoint = advancePointByBytes(n.endPoint, gap)
}

func setNodeEndTo(n *Node, end uint32, source []byte) {
	if n == nil || end > uint32(len(source)) || end < n.startByte || end == n.endByte {
		return
	}
	if end > n.endByte {
		extendNodeEndTo(n, end, source)
		return
	}
	n.endByte = end
	n.endPoint = advancePointByBytes(Point{}, source[:end])
}

func advancePointByBytes(start Point, b []byte) Point {
	p := start
	for _, c := range b {
		if c == '\n' {
			p.Row++
			p.Column = 0
			continue
		}
		p.Column++
	}
	return p
}

func collapsePythonRootFragments(nodes []*Node, arena *nodeArena, lang *Language) []*Node {
	if len(nodes) == 0 || lang == nil || lang.Name != "python" {
		return nodes
	}
	nodes = dropZeroWidthUnnamedTail(nodes, lang)
	for {
		next, changed := collapsePythonClassFragments(nodes, arena, lang)
		if changed {
			nodes = next
			nodes = dropZeroWidthUnnamedTail(nodes, lang)
			continue
		}
		next, changed = collapsePythonFunctionFragments(nodes, arena, lang)
		if changed {
			nodes = next
			nodes = dropZeroWidthUnnamedTail(nodes, lang)
			continue
		}
		next, changed = collapsePythonTerminalIfSuffix(nodes, arena, lang)
		if changed {
			nodes = next
			nodes = dropZeroWidthUnnamedTail(nodes, lang)
			continue
		}
		return normalizePythonModuleChildren(nodes, arena, lang)
	}
}

func collapsePythonClassFragments(nodes []*Node, arena *nodeArena, lang *Language) ([]*Node, bool) {
	if len(nodes) < 5 {
		return nodes, false
	}
	classDefSym, ok := symbolByName(lang, "class_definition")
	if !ok {
		return nodes, false
	}
	blockSym, ok := symbolByName(lang, "block")
	if !ok {
		return nodes, false
	}
	for i := 0; i < len(nodes)-4; i++ {
		j := i
		classNode := nodes[j]
		nameNode := nodes[j+1]
		if classNode == nil || nameNode == nil {
			continue
		}
		if classNode.Type(lang) != "class" || nameNode.Type(lang) != "identifier" {
			continue
		}
		var argNode *Node
		if j+5 < len(nodes) && nodes[j+2] != nil && nodes[j+2].Type(lang) == "argument_list" {
			argNode = nodes[j+2]
			j++
		}
		colonNode := nodes[j+2]
		indentNode := nodes[j+3]
		bodyNode := nodes[j+4]
		if colonNode == nil || indentNode == nil || bodyNode == nil {
			continue
		}
		if colonNode.Type(lang) != ":" || indentNode.Type(lang) != "_indent" {
			continue
		}

		bodyStart := j + 4
		bodyEnd := bodyStart + 1
		var bodyChildren []*Node
		if bodyNode.Type(lang) == "module_repeat1" {
			bodyChildren = flattenPythonModuleRepeat(bodyNode, nil, lang)
		} else {
			var ok bool
			bodyChildren, bodyEnd, ok = pythonCollectIndentedSuite(nodes, bodyStart, classNode.startPoint.Column)
			if !ok {
				continue
			}
			bodyChildren = collapsePythonRootFragments(bodyChildren, arena, lang)
		}
		if len(bodyChildren) == 0 {
			continue
		}
		if arena != nil {
			buf := arena.allocNodeSlice(len(bodyChildren))
			copy(buf, bodyChildren)
			bodyChildren = buf
		}
		blockNode := newParentNodeInArena(arena, blockSym, true, bodyChildren, nil, 0)
		if repairedBlock, changed := repairPythonBlock(blockNode, arena, lang, true); changed {
			blockNode = repairedBlock
		} else {
			blockNode.hasError = false
		}

		classChildren := make([]*Node, 0, 5)
		classChildren = append(classChildren, classNode, nameNode)
		if argNode != nil {
			classChildren = append(classChildren, argNode)
		}
		classChildren = append(classChildren, colonNode, blockNode)
		if arena != nil {
			buf := arena.allocNodeSlice(len(classChildren))
			copy(buf, classChildren)
			classChildren = buf
		}
		classFieldIDs := pythonSyntheticClassFieldIDs(arena, len(classChildren), argNode != nil, lang)
		classDef := newParentNodeInArena(arena, classDefSym, true, classChildren, classFieldIDs, 0)
		classDef.hasError = false

		out := make([]*Node, 0, len(nodes)-(bodyEnd-i)+1)
		out = append(out, nodes[:i]...)
		out = append(out, classDef)
		out = append(out, nodes[bodyEnd:]...)
		if arena != nil {
			buf := arena.allocNodeSlice(len(out))
			copy(buf, out)
			out = buf
		}
		return out, true
	}
	return nodes, false
}

func collapsePythonFunctionFragments(nodes []*Node, arena *nodeArena, lang *Language) ([]*Node, bool) {
	if len(nodes) < 6 || lang == nil || lang.Name != "python" {
		return nodes, false
	}
	functionDefSym, ok := symbolByName(lang, "function_definition")
	if !ok {
		return nodes, false
	}
	blockSym, ok := symbolByName(lang, "block")
	if !ok {
		return nodes, false
	}
	for i := 0; i < len(nodes)-5; i++ {
		defNode := nodes[i]
		nameNode := nodes[i+1]
		paramsNode := nodes[i+2]
		colonNode := nodes[i+3]
		indentNode := nodes[i+4]
		if defNode == nil || nameNode == nil || paramsNode == nil || colonNode == nil || indentNode == nil {
			continue
		}
		if defNode.Type(lang) != "def" || nameNode.Type(lang) != "identifier" || paramsNode.Type(lang) != "parameters" {
			continue
		}
		if colonNode.Type(lang) != ":" || indentNode.Type(lang) != "_indent" {
			continue
		}
		bodyChildren, bodyEnd, ok := pythonCollectIndentedSuite(nodes, i+5, defNode.startPoint.Column)
		if !ok {
			continue
		}
		bodyChildren = collapsePythonRootFragments(bodyChildren, arena, lang)
		if len(bodyChildren) == 0 {
			continue
		}
		if arena != nil {
			buf := arena.allocNodeSlice(len(bodyChildren))
			copy(buf, bodyChildren)
			bodyChildren = buf
		}
		blockNode := newParentNodeInArena(arena, blockSym, true, bodyChildren, nil, 0)
		if repairedBlock, changed := repairPythonBlock(blockNode, arena, lang, false); changed {
			blockNode = repairedBlock
		} else {
			blockNode.hasError = false
		}
		fnChildren := []*Node{defNode, nameNode, paramsNode, colonNode, blockNode}
		if arena != nil {
			buf := arena.allocNodeSlice(len(fnChildren))
			copy(buf, fnChildren)
			fnChildren = buf
		}
		fn := newParentNodeInArena(arena, functionDefSym, true, fnChildren, pythonSyntheticFunctionFieldIDs(arena, len(fnChildren), lang), 0)
		fn.hasError = false

		out := make([]*Node, 0, len(nodes)-(bodyEnd-i)+1)
		out = append(out, nodes[:i]...)
		out = append(out, fn)
		out = append(out, nodes[bodyEnd:]...)
		if arena != nil {
			buf := arena.allocNodeSlice(len(out))
			copy(buf, out)
			out = buf
		}
		return out, true
	}
	return nodes, false
}
func collapsePythonTerminalIfSuffix(nodes []*Node, arena *nodeArena, lang *Language) ([]*Node, bool) {
	if len(nodes) < 6 {
		return nodes, false
	}
	ifSym, ok := symbolByName(lang, "if_statement")
	if !ok {
		return nodes, false
	}
	blockSym, ok := symbolByName(lang, "block")
	if !ok {
		return nodes, false
	}
	n := len(nodes)
	ifNode := nodes[n-6]
	condNode := nodes[n-5]
	colonNode := nodes[n-4]
	indentNode := nodes[n-3]
	bodyNode := nodes[n-2]
	dedentNode := nodes[n-1]
	if ifNode == nil || condNode == nil || colonNode == nil || indentNode == nil || bodyNode == nil || dedentNode == nil {
		return nodes, false
	}
	if ifNode.Type(lang) != "if" || colonNode.Type(lang) != ":" || indentNode.Type(lang) != "_indent" || bodyNode.Type(lang) != "_simple_statements" || dedentNode.Type(lang) != "_dedent" {
		return nodes, false
	}
	if !condNode.IsNamed() {
		return nodes, false
	}

	blockChildren := []*Node{indentNode, bodyNode, dedentNode}
	if arena != nil {
		buf := arena.allocNodeSlice(len(blockChildren))
		copy(buf, blockChildren)
		blockChildren = buf
	}
	blockNode := newParentNodeInArena(arena, blockSym, true, blockChildren, nil, 0)
	blockNode.hasError = false

	ifChildren := []*Node{ifNode, condNode, colonNode, blockNode}
	if arena != nil {
		buf := arena.allocNodeSlice(len(ifChildren))
		copy(buf, ifChildren)
		ifChildren = buf
	}
	ifFieldIDs := pythonSyntheticIfFieldIDs(arena, len(ifChildren), lang)
	ifStmt := newParentNodeInArena(arena, ifSym, true, ifChildren, ifFieldIDs, 0)
	ifStmt.hasError = false

	out := make([]*Node, 0, n-5)
	out = append(out, nodes[:n-6]...)
	out = append(out, ifStmt)
	if arena != nil {
		buf := arena.allocNodeSlice(len(out))
		copy(buf, out)
		return buf, true
	}
	return out, true
}

func flattenPythonModuleRepeat(node *Node, out []*Node, lang *Language) []*Node {
	if node == nil {
		return out
	}
	if node.Type(lang) == "module_repeat1" {
		for _, child := range node.children {
			out = flattenPythonModuleRepeat(child, out, lang)
		}
		return out
	}
	if node.IsNamed() {
		out = append(out, node)
	}
	return out
}

func pythonCollectIndentedSuite(nodes []*Node, start int, baseColumn uint32) ([]*Node, int, bool) {
	if start >= len(nodes) {
		return nil, start, false
	}
	end := start
	for end < len(nodes) {
		cur := nodes[end]
		if cur == nil {
			end++
			continue
		}
		if cur.startPoint.Column <= baseColumn {
			break
		}
		end++
	}
	if end == start {
		return nil, start, false
	}
	return nodes[start:end], end, true
}
