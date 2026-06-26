// Package shared — Evidence weights and composition logic.
// Ported from gitnexus-shared scope-resolution/evidence-weights.ts (91 lines)
// and registries/evidence.ts (196 lines).
package shared

import "math"

// EvidenceWeights — signal weights for each evidence kind.
// These are the canonical constants from the RFC; the sum is capped at 1.0.
var EvidenceWeights = map[ResolutionEvidenceKind]float64{
	EvidenceLocal:                1.0,  // local binding — winner
	EvidenceScopeChain:           0.5,  // lexical chain walk
	EvidenceImport:               0.4,  // import binding
	EvidenceTypeBinding:          0.3,  // type-binding MRO walk
	EvidenceOwnerMatch:           0.25, // owner-scoped contributor
	EvidenceKindMatch:            0.15, // acceptedKinds filter pass
	EvidenceArityMatch:           0.1,  // arity compatibility
	EvidenceGlobalName:           0.2,  // global-name fallback
	EvidenceGlobalQualified:      0.35, // qualified-name fast path
	EvidenceDynamicImportUnresolved: 0.0, // no resolution possible
}

// TypeBindingWeightAtDepth returns the type-binding evidence weight attenuated
// by MRO depth. The deeper in the MRO chain the binding was found,
// the weaker the signal.
//
// weight = base * attenuation^depth
func TypeBindingWeightAtDepth(depth int) float64 {
	base := EvidenceWeights[EvidenceTypeBinding]
	attenuation := 0.8 // each MRO step reduces weight by 20%
	return base * math.Pow(attenuation, float64(depth))
}

// RawSignals represents the composite signals collected during a single lookup.
// Multiple signals can be active simultaneously; they compose additively.
type RawSignals struct {
	Origin        BindingOrigin // where the binding came from
	KindMatch     bool          // def.Type is in acceptedKinds
	ArityMatch    ArityVerdict  // arity compatibility result
	ScopeDepth    int           // depth of the scope where binding was found (0 = current)
	MroDepth      int           // depth in the MRO chain (0 = self)
	OwnerMatch    bool          // owner-scoped contributor matched
	TypeBinding   bool          // type-binding walk produced this binding
	GlobalName    bool          // global-name fallback matched
	GlobalQualified bool        // qualified-name fast path matched
	ScopeChain    bool          // lexical chain walk produced this binding
	Local         bool          // binding is local (same scope)
	Import        bool          // binding came from import
	DynamicImportUnresolved bool // dynamic import could not be resolved
}

// ArityVerdict represents the result of an arity compatibility check.
type ArityVerdict string

const (
	ArityExact   ArityVerdict = "exact"   // parameter count matches exactly
	ArityInRange ArityVerdict = "in-range" // within optional parameter range
	ArityMismatch ArityVerdict = "mismatch" // clearly incompatible
	ArityUnknown ArityVerdict = "unknown"  // arity info not available
)

// ComposeEvidence composes a ResolutionEvidence slice from the raw signals.
// Each active signal produces one evidence entry with its corresponding weight.
// Returns the composed evidence slice.
func ComposeEvidence(signals RawSignals) []ResolutionEvidence {
	var evidence []ResolutionEvidence

	add := func(kind ResolutionEvidenceKind, weight float64) {
		evidence = append(evidence, ResolutionEvidence{Kind: kind, Weight: weight})
	}

	if signals.Local {
		add(EvidenceLocal, EvidenceWeights[EvidenceLocal])
	}
	if signals.ScopeChain {
		add(EvidenceScopeChain, EvidenceWeights[EvidenceScopeChain])
	}
	if signals.Import {
		add(EvidenceImport, EvidenceWeights[EvidenceImport])
	}
	if signals.TypeBinding {
		add(EvidenceTypeBinding, TypeBindingWeightAtDepth(signals.MroDepth))
	}
	if signals.OwnerMatch {
		add(EvidenceOwnerMatch, EvidenceWeights[EvidenceOwnerMatch])
	}
	if signals.KindMatch {
		add(EvidenceKindMatch, EvidenceWeights[EvidenceKindMatch])
	}
	switch signals.ArityMatch {
	case ArityExact:
		add(EvidenceArityMatch, EvidenceWeights[EvidenceArityMatch])
	case ArityInRange:
		add(EvidenceArityMatch, EvidenceWeights[EvidenceArityMatch]*0.5) // partial credit
		// ArityMismatch and ArityUnknown produce no evidence
	}
	if signals.GlobalName {
		add(EvidenceGlobalName, EvidenceWeights[EvidenceGlobalName])
	}
	if signals.GlobalQualified {
		add(EvidenceGlobalQualified, EvidenceWeights[EvidenceGlobalQualified])
	}
	if signals.DynamicImportUnresolved {
		add(EvidenceDynamicImportUnresolved, EvidenceWeights[EvidenceDynamicImportUnresolved])
	}

	return evidence
}

// ConfidenceFromEvidence computes the confidence score from a slice of evidence.
// Σ of evidence weights, capped at [0, 1.0].
func ConfidenceFromEvidence(evidence []ResolutionEvidence) float64 {
	sum := 0.0
	for _, e := range evidence {
		sum += e.Weight
	}
	return math.Min(sum, 1.0)
}