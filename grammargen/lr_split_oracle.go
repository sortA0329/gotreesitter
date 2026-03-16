package grammargen

// splitCandidate describes a state that may benefit from LR(1) state splitting.
type splitCandidate struct {
	stateIdx     int
	reason       string
	mergeCount   int
	conflictKind ConflictKind
	lookaheadSym int
	resolution   string
}

// splitOracle analyzes conflict diagnostics and merge provenance to identify
// states where unmerging the LALR state back to canonical LR(1) states would
// resolve or reduce conflicts.
type splitOracle struct {
	conflicts []ConflictDiag
	prov      *mergeProvenance
}

func newSplitOracle(conflicts []ConflictDiag, prov *mergeProvenance) *splitOracle {
	return &splitOracle{
		conflicts: conflicts,
		prov:      prov,
	}
}

// candidates returns states that are split candidates.
// A state is a candidate if:
//  1. It has an unresolved conflict (GLR entry with multiple actions), AND
//  2. It was produced by LALR merging (has merge origins)
func (o *splitOracle) candidates() []splitCandidate {
	var result []splitCandidate
	seen := make(map[int]bool)

	for _, c := range o.conflicts {
		if len(c.Actions) <= 1 {
			continue
		}
		reason := ""
		switch c.Resolution {
		case "GLR (multiple actions kept)":
			reason = "unresolved GLR conflict in merged LALR state"
		case "shift wins (default yacc behavior)":
			reason = "default shift resolution in merged LALR state"
		default:
			continue
		}

		// Check merge status via provenance directly (more reliable
		// than ConflictDiag fields which may not be populated yet).
		isMerged := false
		mc := 0
		if o.prov != nil {
			isMerged = o.prov.isMerged(c.State)
			mc = len(o.prov.origins(c.State))
		}

		if !isMerged {
			continue
		}

		if seen[c.State] {
			continue
		}
		seen[c.State] = true

		result = append(result, splitCandidate{
			stateIdx:     c.State,
			reason:       reason,
			mergeCount:   mc,
			conflictKind: c.Kind,
			lookaheadSym: c.LookaheadSym,
			resolution:   c.Resolution,
		})
	}

	return result
}
