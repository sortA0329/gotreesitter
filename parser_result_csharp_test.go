package gotreesitter

import "testing"

func TestCSharpFindQueryAssignmentSpecs(t *testing.T) {
	src := []byte("var x = from a in source\n  where a.B == \"A\"\n  select new { Name = a.B };\n")

	specs, ok := csharpFindQueryAssignmentSpecs(src)
	if !ok {
		t.Fatal("expected query assignment spec")
	}
	if got := len(specs); got != 1 {
		t.Fatalf("spec count = %d, want 1", got)
	}
	if got := len(specs[0].clauses); got != 3 {
		t.Fatalf("clause count = %d, want 3", got)
	}
	if got := specs[0].clauses[0].kind; got != csharpQueryFromClause {
		t.Fatalf("first clause kind = %v, want from", got)
	}
	if got := specs[0].clauses[1].kind; got != csharpQueryWhereClause {
		t.Fatalf("second clause kind = %v, want where", got)
	}
	if got := specs[0].clauses[2].kind; got != csharpQuerySelectClause {
		t.Fatalf("third clause kind = %v, want select", got)
	}
}
