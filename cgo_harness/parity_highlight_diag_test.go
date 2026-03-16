//go:build cgo && treesitter_c_parity

package cgoharness

import (
	"fmt"
	"strings"
	"testing"

	sitter "github.com/tree-sitter/go-tree-sitter"

	gotreesitter "github.com/odvcencio/gotreesitter"
)

// TestHighlightGapDiagnosis dumps detailed information about every C-only
// highlight capture for languages in knownHighlightDivergence. This helps
// identify the exact query patterns and node structures causing gaps.
func TestHighlightGapDiagnosis(t *testing.T) {
	for langName, threshold := range knownHighlightDivergence {
		if threshold == 0 {
			continue
		}
		langName, threshold := langName, threshold
		t.Run(langName, func(t *testing.T) {
			if parityLanguageExcluded(langName) {
				t.Skip("excluded by GTS_PARITY_SKIP_LANGS")
			}
			// Find test case
			var tc parityCase
			found := false
			for _, c := range parityCases {
				if c.name == langName {
					tc = c
					found = true
					break
				}
			}
			if !found {
				t.Skipf("no parity case for %q", langName)
			}

			entry, ok := parityEntriesByName[tc.name]
			if !ok {
				t.Skipf("no registry entry for %q", tc.name)
			}
			queryStr := entry.HighlightQuery
			if queryStr == "" {
				t.Skipf("no highlight query for %q", tc.name)
			}

			src := normalizedSource(tc.name, tc.source)

			// --- Go side ---
			goTree, goLang, err := parseWithGo(tc, src, nil)
			if err != nil {
				t.Fatalf("Go parse error: %v", err)
			}
			goCaps := collectGoHighlightCaptures(t, goLang, goTree, queryStr, src)

			// --- C side ---
			cLang, err := ParityCLanguage(tc.name)
			if err != nil {
				t.Skipf("skip C reference: %v", err)
			}
			cParser := sitter.NewParser()
			defer cParser.Close()
			if err := cParser.SetLanguage(cLang); err != nil {
				t.Skipf("skip C SetLanguage: %v", err)
			}
			cTree := cParser.Parse(src, nil)
			if cTree == nil {
				t.Fatal("C parser returned nil tree")
			}
			defer cTree.Close()

			cCaps := collectCHighlightCaptures(t, cLang, cTree, queryStr, src)

			// --- Diff ---
			_, onlyC := diffCaptures(goCaps, cCaps)
			t.Logf("Go captures: %d, C captures: %d, C-only: %d (threshold: %d)",
				len(goCaps), len(cCaps), len(onlyC), threshold)

			if len(onlyC) == 0 {
				t.Logf("No C-only captures — gap may already be fixed!")
				return
			}

			goRoot := goTree.RootNode()

			for i, cap := range onlyC {
				text := textSlice(src, cap)
				t.Logf("\n=== C-only capture #%d ===", i+1)
				t.Logf("  Capture: @%s [%d-%d] %q", cap.Name, cap.StartByte, cap.EndByte, text)

				// Find Go node at this position
				goNode := goRoot.DescendantForByteRange(cap.StartByte, cap.EndByte)
				if goNode != nil {
					nodeType := goNode.Type(goLang)
					t.Logf("  Go node at range: type=%q named=%v symbol=%d [%d-%d]",
						nodeType, goNode.IsNamed(), goNode.Symbol(),
						goNode.StartByte(), goNode.EndByte())

					// Walk up to show parent chain
					parent := goNode.Parent()
					depth := 0
					for parent != nil && depth < 5 {
						pType := parent.Type(goLang)
						fieldName := ""
						// Find what field this child has
						for ci := 0; ci < parent.ChildCount(); ci++ {
							child := parent.Child(ci)
							if child == goNode {
								fieldName = parent.FieldNameForChild(ci, goLang)
								break
							}
						}
						fieldStr := ""
						if fieldName != "" {
							fieldStr = fmt.Sprintf(" field=%q", fieldName)
						}

						// Count how many children have the same field
						if fieldName != "" {
							fieldCount := 0
							for ci := 0; ci < parent.ChildCount(); ci++ {
								if parent.FieldNameForChild(ci, goLang) == fieldName {
									fieldCount++
								}
							}
							if fieldCount > 1 {
								fieldStr += fmt.Sprintf(" (field has %d children)", fieldCount)
							}
						}

						t.Logf("  Parent[%d]: type=%q%s named=%v children=%d",
							depth, pType, fieldStr, parent.IsNamed(), parent.ChildCount())
						goNode = parent
						parent = parent.Parent()
						depth++
					}
				} else {
					t.Logf("  Go node at range: NOT FOUND")
				}

				// Find potentially matching query pattern
				patternSnippet := findQueryPatternForCapture(queryStr, cap.Name)
				if patternSnippet != "" {
					t.Logf("  Query pattern snippet: %s", patternSnippet)
				}
			}
		})
	}
}

// TestHighlightGapMiniQuery compiles a minimal query for just the missing
// capture type and runs it, to isolate whether the issue is in full query
// compilation, the pattern index, or something else.
func TestHighlightGapMiniQuery(t *testing.T) {
	type miniTest struct {
		lang      string
		query     string
		expectMin int // minimum expected captures
	}
	tests := []miniTest{
		{"html", "(tag_name) @tag", 4},    // all 4 tag_name nodes
		{"julia", "(identifier) @var", 2}, // at least M and x
		{"elixir", "(call target: (identifier) @fn)", 2},
		{"yaml", "(block_mapping_pair key: (_) @k)", 1},
		{"kotlin", "(call_expression (_) @fn)", 1},
	}
	for _, tt := range tests {
		t.Run(tt.lang+"_mini", func(t *testing.T) {
			if parityLanguageExcluded(tt.lang) {
				t.Skip("excluded by GTS_PARITY_SKIP_LANGS")
			}
			var tc parityCase
			found := false
			for _, c := range parityCases {
				if c.name == tt.lang {
					tc = c
					found = true
					break
				}
			}
			if !found {
				t.Skipf("no parity case for %q", tt.lang)
			}

			src := normalizedSource(tc.name, tc.source)
			goTree, goLang, err := parseWithGo(tc, src, nil)
			if err != nil {
				t.Fatalf("Go parse error: %v", err)
			}

			// Compile mini query
			q, qErr := gotreesitter.NewQuery(tt.query, goLang)
			if qErr != nil {
				t.Fatalf("mini query compile error: %v", qErr)
			}

			matches := q.Execute(goTree)
			total := 0
			for _, m := range matches {
				for _, c := range m.Captures {
					total++
					t.Logf("  capture: @%s [%d-%d] %q type=%q sym=%d",
						c.Name, c.Node.StartByte(), c.Node.EndByte(),
						c.Node.Text(src), c.Node.Type(goLang), c.Node.Symbol())
				}
			}
			t.Logf("total mini captures: %d (expect >= %d)", total, tt.expectMin)

			// Also dump all nodes of the expected type via DFS
			t.Logf("--- DFS walk of all nodes ---")
			var walk func(n *gotreesitter.Node, depth int)
			walk = func(n *gotreesitter.Node, depth int) {
				if n == nil {
					return
				}
				indent := strings.Repeat("  ", depth)
				t.Logf("%s%s (sym=%d named=%v) [%d-%d] %q",
					indent, n.Type(goLang), n.Symbol(), n.IsNamed(),
					n.StartByte(), n.EndByte(), n.Text(src))
				for i := 0; i < n.ChildCount(); i++ {
					walk(n.Child(i), depth+1)
				}
			}
			walk(goTree.RootNode(), 0)
		})
	}
}

// TestHighlightGapFieldCheck verifies that field IDs are correctly assigned
// on nodes that should have them.
func TestHighlightGapFieldCheck(t *testing.T) {
	type fieldTest struct {
		lang      string
		nodeType  string // parent node type to find
		fieldName string // expected field on a child
	}
	tests := []fieldTest{
		{"elixir", "call", "target"},
		{"javascript", "variable_declarator", "name"},
		{"yaml", "block_mapping_pair", "key"},
		{"scala", "function_definition", "name"},
		{"julia", "module_definition", "name"},
	}
	for _, tt := range tests {
		t.Run(tt.lang+"_"+tt.nodeType+"_"+tt.fieldName, func(t *testing.T) {
			if parityLanguageExcluded(tt.lang) {
				t.Skip("excluded by GTS_PARITY_SKIP_LANGS")
			}
			var tc parityCase
			found := false
			for _, c := range parityCases {
				if c.name == tt.lang {
					tc = c
					found = true
					break
				}
			}
			if !found {
				t.Skipf("no parity case for %q", tt.lang)
			}

			src := normalizedSource(tc.name, tc.source)
			goTree, goLang, err := parseWithGo(tc, src, nil)
			if err != nil {
				t.Fatalf("Go parse error: %v", err)
			}

			// Find first node of the target type
			var findNode func(n *gotreesitter.Node) *gotreesitter.Node
			findNode = func(n *gotreesitter.Node) *gotreesitter.Node {
				if n.Type(goLang) == tt.nodeType {
					return n
				}
				for i := 0; i < n.ChildCount(); i++ {
					if found := findNode(n.Child(i)); found != nil {
						return found
					}
				}
				return nil
			}
			node := findNode(goTree.RootNode())
			if node == nil {
				t.Fatalf("no %q node found", tt.nodeType)
			}

			t.Logf("Found %q at [%d-%d], children=%d", tt.nodeType,
				node.StartByte(), node.EndByte(), node.ChildCount())

			// Check field assignments
			fieldChild := node.ChildByFieldName(tt.fieldName, goLang)
			if fieldChild != nil {
				t.Logf("  ChildByFieldName(%q) = %q [%d-%d] sym=%d",
					tt.fieldName, fieldChild.Type(goLang),
					fieldChild.StartByte(), fieldChild.EndByte(), fieldChild.Symbol())
			} else {
				t.Logf("  ChildByFieldName(%q) = nil (MISSING!)", tt.fieldName)
			}

			// Print all children with field names
			for i := 0; i < node.ChildCount(); i++ {
				child := node.Child(i)
				fname := node.FieldNameForChild(i, goLang)
				t.Logf("  child[%d]: %q (sym=%d) field=%q [%d-%d] %q",
					i, child.Type(goLang), child.Symbol(), fname,
					child.StartByte(), child.EndByte(), child.Text(src))
			}

			// Also check C tree fields via parity
			cLang, err := ParityCLanguage(tc.name)
			if err != nil {
				t.Skipf("skip C reference: %v", err)
			}
			cParser := sitter.NewParser()
			defer cParser.Close()
			if err := cParser.SetLanguage(cLang); err != nil {
				t.Skipf("skip C SetLanguage: %v", err)
			}
			cTree := cParser.Parse(src, nil)
			if cTree == nil {
				t.Skip("C tree is nil")
			}
			defer cTree.Close()

			// Find same node in C tree by byte range
			cNode := cTree.RootNode().DescendantForByteRange(uint(node.StartByte()), uint(node.EndByte()))
			if cNode == nil || cNode.Kind() != tt.nodeType {
				t.Logf("  C tree: could not find matching %q node", tt.nodeType)
				return
			}

			t.Logf("  C tree %q at [%d-%d] children=%d", cNode.Kind(),
				cNode.StartByte(), cNode.EndByte(), cNode.ChildCount())
			for i := uint(0); i < cNode.ChildCount(); i++ {
				cChild := cNode.Child(i)
				cField := cNode.FieldNameForChild(uint32(i))
				t.Logf("  C child[%d]: %q field=%q [%d-%d]",
					i, cChild.Kind(), cField,
					cChild.StartByte(), cChild.EndByte())
			}
		})
	}
}

func TestHighlightObjcMiniQueryDiagnosis(t *testing.T) {
	langName := "objc"
	var tc parityCase
	found := false
	for _, c := range parityCases {
		if c.name == langName {
			tc = c
			found = true
			break
		}
	}
	if !found {
		t.Skipf("no parity case for %q", langName)
	}

	src := normalizedSource(tc.name, tc.source)
	goTree, goLang, err := parseWithGo(tc, src, nil)
	if err != nil {
		t.Fatalf("Go parse error: %v", err)
	}
	defer releaseGoTree(goTree)

	cLang, err := ParityCLanguage(langName)
	if err != nil {
		t.Skipf("skip C reference: %v", err)
	}
	cParser := sitter.NewParser()
	defer cParser.Close()
	if err := cParser.SetLanguage(cLang); err != nil {
		t.Skipf("skip C SetLanguage: %v", err)
	}
	cTree := cParser.Parse(src, nil)
	if cTree == nil {
		t.Fatal("C parser returned nil tree")
	}
	defer cTree.Close()

	t.Logf("Go root: %s", goTree.RootNode().SExpr(goLang))
	if sym, ok := goLang.SymbolByName("struct_declaration"); ok {
		t.Logf("Go struct_declaration: sym=%d supertype=%v children=%v", sym, goLang.IsSupertype(sym), goLang.SupertypeChildren(sym))
	} else {
		t.Logf("Go struct_declaration: missing")
	}
	if sym, ok := goLang.SymbolByName("class_interface"); ok {
		t.Logf("Go class_interface: sym=%d public=%d", sym, goLang.PublicSymbol(sym))
	} else {
		t.Logf("Go class_interface: missing")
	}

	tests := []struct {
		name  string
		query string
	}{
		{
			name:  "class_interface_types",
			query: `(class_interface "@interface" . (identifier) @type superclass: _? @type)`,
		},
		{
			name:  "struct_ancestor_property",
			query: `((identifier) @property (#has-ancestor? @property struct_declaration))`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			goCaps := collectGoHighlightCaptures(t, goLang, goTree, tt.query, src)
			cCaps := collectCHighlightCaptures(t, cLang, cTree, tt.query, src)
			t.Logf("go=%v", goCaps)
			t.Logf("c=%v", cCaps)
			onlyGo, onlyC := diffCaptures(goCaps, cCaps)
			t.Logf("onlyGo=%v", onlyGo)
			t.Logf("onlyC=%v", onlyC)
		})
	}
}

// findQueryPatternForCapture searches the query string for patterns that
// reference the given capture name. Returns a truncated snippet of the
// first matching pattern.
func findQueryPatternForCapture(queryStr string, captureName string) string {
	target := "@" + captureName
	lines := strings.Split(queryStr, "\n")

	// Find lines containing this capture
	var matchingLines []int
	for i, line := range lines {
		if strings.Contains(line, target) {
			matchingLines = append(matchingLines, i)
		}
	}

	if len(matchingLines) == 0 {
		return ""
	}

	// For the first match, walk backwards to find the start of the pattern
	// (look for a line starting with '(' at depth 0)
	lineIdx := matchingLines[0]
	startLine := lineIdx
	depth := 0
	for i := lineIdx; i >= 0; i-- {
		trimmed := strings.TrimSpace(lines[i])
		depth += strings.Count(trimmed, ")") - strings.Count(trimmed, "(")
		if depth <= 0 && strings.HasPrefix(trimmed, "(") {
			startLine = i
			break
		}
	}

	// Collect up to the end of the pattern
	endLine := lineIdx
	depth = 0
	for i := startLine; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		depth += strings.Count(trimmed, "(") - strings.Count(trimmed, ")")
		if depth <= 0 && i >= lineIdx {
			endLine = i
			break
		}
	}

	var result []string
	for i := startLine; i <= endLine && i < len(lines); i++ {
		result = append(result, lines[i])
	}

	snippet := strings.Join(result, "\n")
	if len(snippet) > 500 {
		snippet = snippet[:500] + "..."
	}
	return snippet
}
