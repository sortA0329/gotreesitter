package cgoharness

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	gotreesitter "github.com/odvcencio/gotreesitter"
)

func makeGoBenchmarkSource(funcCount int) []byte {
	var sb strings.Builder
	sb.Grow(funcCount * 48)
	sb.WriteString("package main\n\n")
	for i := 0; i < funcCount; i++ {
		fmt.Fprintf(&sb, "func f%d() int { v := %d; return v }\n", i, i)
	}
	return []byte(sb.String())
}

func pointAtOffset(src []byte, offset int) gotreesitter.Point {
	var row uint32
	var col uint32
	for i := 0; i < offset && i < len(src); {
		r, size := utf8.DecodeRune(src[i:])
		if r == '\n' {
			row++
			col = 0
		} else {
			col++
		}
		i += size
	}
	return gotreesitter.Point{Row: row, Column: col}
}

func benchmarkFuncCount(b *testing.B) int {
	if raw := strings.TrimSpace(os.Getenv("GOT_BENCH_FUNC_COUNT")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err == nil && n > 0 {
			return n
		}
		b.Fatalf("invalid GOT_BENCH_FUNC_COUNT=%q", raw)
	}
	if testing.Short() {
		return 100
	}
	return 500
}

type benchmarkEditSite struct {
	offset int
	start  gotreesitter.Point
	end    gotreesitter.Point
}

func makeBenchmarkEditSites(src []byte, marker string) []benchmarkEditSite {
	needle := []byte(marker)
	sites := make([]benchmarkEditSite, 0, 64)
	from := 0
	for from < len(src) {
		idx := bytes.Index(src[from:], needle)
		if idx < 0 {
			break
		}
		offset := from + idx + len(marker)
		if offset >= len(src) {
			break
		}
		sites = append(sites, benchmarkEditSite{
			offset: offset,
			start:  pointAtOffset(src, offset),
			end:    pointAtOffset(src, offset+1),
		})
		from = offset + 1
	}
	return sites
}

func makeGoBenchmarkEditSites(src []byte) []benchmarkEditSite {
	return makeBenchmarkEditSites(src, "v := ")
}

func makeTypeScriptBenchmarkSource(funcCount int) []byte {
	var sb strings.Builder
	sb.Grow(funcCount * len("export function f0(): number { const v = 0; return v }\n"))
	for i := 0; i < funcCount; i++ {
		fmt.Fprintf(&sb, "export function f%d(): number { const v = %d; return v }\n", i, i)
	}
	return []byte(sb.String())
}

func makePythonBenchmarkSource(funcCount int) []byte {
	var sb strings.Builder
	sb.Grow(funcCount * len("def f0():\n    v = 0\n    return v\n\n"))
	for i := 0; i < funcCount; i++ {
		fmt.Fprintf(&sb, "def f%d():\n    v = %d\n    return v\n\n", i, i)
	}
	return []byte(sb.String())
}

func toggleDigitAt(src []byte, offset int) {
	if offset < 0 || offset >= len(src) {
		return
	}
	if src[offset] == '0' {
		src[offset] = '1'
		return
	}
	src[offset] = '0'
}

func prepareEditedBenchmarkSource(cur, scratch []byte, offset int) []byte {
	if len(scratch) != len(cur) {
		scratch = make([]byte, len(cur))
	}
	copy(scratch, cur)
	toggleDigitAt(scratch, offset)
	return scratch
}
