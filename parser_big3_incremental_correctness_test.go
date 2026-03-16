package gotreesitter_test

import (
	"testing"

	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

func TestBig3SyntheticIncrementalParsesStayComplete(t *testing.T) {
	cases := []struct {
		name   string
		lang   func() *gotreesitter.Language
		source func(int) []byte
		marker string
	}{
		{
			name:   "typescript",
			lang:   grammars.TypescriptLanguage,
			source: makeTypeScriptBenchmarkSource,
			marker: "const v = ",
		},
		{
			name:   "python",
			lang:   grammars.PythonLanguage,
			source: makePythonBenchmarkSource,
			marker: "v = ",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lang := tc.lang()
			parser := gotreesitter.NewParser(lang)
			src := tc.source(128)
			sites := makeBenchmarkEditSites(src, tc.marker)
			if len(sites) == 0 {
				t.Fatalf("missing edit sites for marker %q", tc.marker)
			}
			site := sites[0]

			oldTree, err := parser.Parse(src)
			if err != nil {
				t.Fatalf("initial parse failed: %v", err)
			}
			requireCompleteParse(t, oldTree, src, lang, "initial")
			if oldTree.RootNode().HasError() {
				t.Fatal("initial parse produced error root")
			}

			next := append([]byte(nil), src...)
			toggleDigitAt(next, site.offset)
			oldTree.Edit(gotreesitter.InputEdit{
				StartByte:   uint32(site.offset),
				OldEndByte:  uint32(site.offset + 1),
				NewEndByte:  uint32(site.offset + 1),
				StartPoint:  site.start,
				OldEndPoint: site.end,
				NewEndPoint: site.end,
			})

			newTree, err := parser.ParseIncremental(next, oldTree)
			if err != nil {
				t.Fatalf("incremental parse failed: %v", err)
			}
			requireCompleteParse(t, newTree, next, lang, "incremental")
			if newTree.RootNode().HasError() {
				t.Fatal("incremental parse produced error root")
			}
		})
	}
}
