// Package registries — tie-break key and comparison for resolution ranking.
// Ported from gitnexus-shared scope-resolution/registries/tie-breaks.ts (77 lines).
package registries

import (
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// CONFIDENCE_EPSILON is the minimum confidence difference to consider two
// candidates as having meaningfully different confidence scores.
// Candidates within epsilon of each other are "tied" and need further
// tie-breaking via scope depth, MRO depth, and origin priority.
const CONFIDENCE_EPSILON = 0.001

// TieBreakKey holds the sortable attributes used when two Resolutions
// have approximately equal confidence. The cascade order is:
//   1. Confidence DESC (higher is better)
//   2. ScopeDepth ASC (closer scope is better)
//   3. MroDepth ASC (shallower MRO is better)
//   4. OriginPriority ASC (lower number = higher priority)
//   5. DefID lexicographic (deterministic tiebreaker of last resort)
type TieBreakKey struct {
	ScopeDepth     int
	MroDepth       int
	OriginPriority shared.OriginForTieBreak
	DefID          shared.DefID
}

// CompareByConfidenceWithTiebreaks compares two Resolutions using the
// full tie-break cascade. Returns -1 if a < b, 0 if equal, 1 if a > b.
//
// The cascade is:
//   1. Confidence: higher wins (descending)
//   2. ScopeDepth: shallower wins (ascending)
//   3. MroDepth: shallower wins (ascending)
//   4. OriginPriority: lower number wins (ascending)
//   5. DefID: lexicographic (deterministic, never returns 0 for distinct defs)
func CompareByConfidenceWithTiebreaks(
	a, b shared.Resolution,
	keys map[string]TieBreakKey,
) int {
	// Step 1: Confidence descending
	diff := b.Confidence - a.Confidence
	if diff > CONFIDENCE_EPSILON {
		return -1 // b has higher confidence
	}
	if diff < -CONFIDENCE_EPSILON {
		return 1 // a has higher confidence
	}

	// Within epsilon: apply tie-break cascade
	keyA, okA := keys[a.Def.NodeID]
	keyB, okB := keys[b.Def.NodeID]

	if !okA && !okB {
		// Both missing keys: fall through to DefID comparison
	} else if okA && !okB {
		return 1 // a has tie-break info, b doesn't
	} else if !okA && okB {
		return -1
	} else {
		// Step 2: ScopeDepth ascending (shallower = better)
		if keyA.ScopeDepth != keyB.ScopeDepth {
			if keyA.ScopeDepth < keyB.ScopeDepth {
				return 1 // a is shallower, wins
			}
			return -1
		}

		// Step 3: MroDepth ascending (shallower = better)
		if keyA.MroDepth != keyB.MroDepth {
			if keyA.MroDepth < keyB.MroDepth {
				return 1
			}
			return -1
		}

		// Step 4: OriginPriority ascending (lower = better)
		if keyA.OriginPriority != keyB.OriginPriority {
			if keyA.OriginPriority < keyB.OriginPriority {
				return 1
			}
			return -1
		}
	}

	// Step 5: DefID lexicographic (deterministic)
	if a.Def.NodeID < b.Def.NodeID {
		return 1
	}
	if a.Def.NodeID > b.Def.NodeID {
		return -1
	}
	return 0
}