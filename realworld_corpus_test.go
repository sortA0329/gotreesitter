package gotreesitter_test

import (
	"os"
	"testing"
)

func readRealworldCorpusOrSkip(t *testing.T, path string) []byte {
	t.Helper()

	src, err := os.ReadFile(path)
	if err == nil {
		return src
	}
	if os.IsNotExist(err) {
		t.Skipf("real-world corpus fixture unavailable: %s", path)
	}
	t.Fatalf("read %s: %v", path, err)
	return nil
}
