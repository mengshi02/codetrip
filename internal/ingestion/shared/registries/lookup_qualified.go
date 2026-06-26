// lookupQualified: fast-path resolution via global qualified name.
// Ported from gitnexus scope-resolution/registries/lookup-qualified.ts (72 lines).
package registries

import (
	"sort"

	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// LookupQualifiedParams holds the parameters for qualified-name lookup.
type LookupQualifiedParams struct {
	// AcceptedKinds filters candidates by NodeLabel.
	AcceptedKinds []shared.NodeLabel
}

// LookupQualified resolves a name by directly looking up its global
// qualified name in the QualifiedNameIndex, bypassing scope-chain walk.
// Returns results sorted by confidence + tie-break cascade.
func LookupQualified(name string, params LookupQualifiedParams, ctx *RegistryContext) []shared.Resolution {
	acceptedKindsSet := make(map[shared.NodeLabel]bool, len(params.AcceptedKinds))
	for _, k := range params.AcceptedKinds {
		acceptedKindsSet[k] = true
	}

	if ctx.QualifiedNames == nil || ctx.Defs == nil {
		return nil
	}

	defIDs := ctx.QualifiedNames.Get(name)
	if len(defIDs) == 0 {
		return nil
	}

	tieKeys := make(map[string]TieBreakKey)
	resolutions := make([]shared.Resolution, 0, len(defIDs))

	for _, defID := range defIDs {
		def := ctx.Defs.Get(defID)
		if def == nil {
			continue
		}

		// Kind filter
		if !acceptedKindsSet[def.Type] {
			continue
		}

		signals := shared.RawSignals{
			Origin:          shared.GlobalQualifiedOrigin,
			KindMatch:       true,
			GlobalQualified: true,
		}

		evidence := shared.ComposeEvidence(signals)
		confidence := shared.ConfidenceFromEvidence(evidence)

		resolutions = append(resolutions, shared.Resolution{
			Def:        *def,
			Confidence: confidence,
			Evidence:   evidence,
		})

		tieKeys[defID] = TieBreakKey{
			ScopeDepth:     0,
			MroDepth:       0,
			OriginPriority: shared.OriginPriorityGlobalQN,
			DefID:          defID,
		}
	}

	// Sort by confidence + tie-break cascade (descending)
	sort.SliceStable(resolutions, func(i, j int) bool {
		cmp := CompareByConfidenceWithTiebreaks(resolutions[i], resolutions[j], tieKeys)
		return cmp > 0
	})

	return resolutions
}