//go:build cgo && treesitter_c_parity

package cgoharness

import "testing"

func TestCSharpInvocationStatementParity(t *testing.T) {
	src := []byte("class C { void F(){ newLines.Add(line); } }\n")
	tc := parityCase{name: "c_sharp", source: string(src)}
	runParityCase(t, tc, "invocation-statement-vs-local-declaration", src)
}

func TestCSharpReadToEndMemberAccessWithTopLevelVarParity(t *testing.T) {
	src := []byte(`using System.Diagnostics;

var filePath = "";

string GetOutput()
{
    var process = new Process
    {
        StartInfo = new ProcessStartInfo
        {
            Arguments = $"test --filter skip-all-corpus-tests",

        }
    };
    var output = process.StandardOutput.ReadToEnd();
    process.WaitForExit();
    return output;
}
`)
	tc := parityCase{name: "c_sharp", source: string(src)}
	runParityCase(t, tc, "read-to-end-member-access", src)
}
