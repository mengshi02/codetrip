package scope_resolution

// ResolutionOutcome records a single resolver diagnostic event.
// Mirrors TS scope-resolution/resolution-outcome.ts.
//
// These are additive diagnostics that do not affect graph edges —
// they exist for observability and debugging.
type ResolutionOutcome struct {
	// Language is the language that produced this outcome.
	Language string
	// Phase is the sub-phase that produced this outcome.
	Phase ScopeResolutionSubPhase
	// FilePath is the file being processed (may be empty for workspace-level outcomes).
	FilePath string
	// Kind categorizes the outcome (e.g. "resolved", "unresolved", "ambiguous", "error").
	Kind string
	// Detail is a human-readable description.
	Detail string
	// Count is the number of items this outcome represents.
	Count int
}