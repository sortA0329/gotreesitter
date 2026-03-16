package gotreesitter_test

import (
	"testing"

	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

func TestExternalScannerIncrementalReusePolicy(t *testing.T) {
	cases := []struct {
		name           string
		lang           func() *gotreesitter.Language
		source         func(int) []byte
		marker         string
		wantReuse      bool
		wantReason     string
		wantSubtreeMin uint64
	}{
		{
			name:           "typescript",
			lang:           grammars.TypescriptLanguage,
			source:         makeTypeScriptBenchmarkSource,
			marker:         "const v = ",
			wantReuse:      true,
			wantSubtreeMin: 1,
		},
		{
			name:           "python",
			lang:           grammars.PythonLanguage,
			source:         makePythonBenchmarkSource,
			marker:         "v = ",
			wantReuse:      true,
			wantSubtreeMin: 1,
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

			newTree, prof, err := parser.ParseIncrementalProfiled(next, oldTree)
			if err != nil {
				t.Fatalf("incremental parse failed: %v", err)
			}
			requireCompleteParse(t, newTree, next, lang, "incremental")
			if newTree.RootNode().HasError() {
				t.Fatal("incremental parse produced error root")
			}
			if tc.wantReuse {
				if prof.ReuseUnsupported {
					t.Fatalf("ReuseUnsupported = true, want false (reason=%q)", prof.ReuseUnsupportedReason)
				}
				if prof.ReusedSubtrees < tc.wantSubtreeMin {
					t.Fatalf("ReusedSubtrees = %d, want >= %d", prof.ReusedSubtrees, tc.wantSubtreeMin)
				}
				return
			}
			if !prof.ReuseUnsupported {
				t.Fatal("ReuseUnsupported = false, want true")
			}
			if prof.ReuseUnsupportedReason != tc.wantReason {
				t.Fatalf("ReuseUnsupportedReason = %q, want %q", prof.ReuseUnsupportedReason, tc.wantReason)
			}
		})
	}
}
