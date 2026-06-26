// Phase registry — dependency-ordered phase list assembly.
//
// A small, behaviour-preserving abstraction over phase-list assembly.
// Each phase may optionally declare an EnabledWhen predicate; the registry
// filters out disabled phases at build time based on the provided options.
//
// Ported from gitnexus pipeline-phases/registry.ts (73 lines).
package pipeline

// PhaseEnabledWhen is a predicate that decides whether a phase is included
// for a given options object. Return true to include the phase.
type PhaseEnabledWhen func(options *PipelineOptions) bool

// phaseRegistration holds a phase and its optional enable-when predicate.
type phaseRegistration struct {
	phase       PipelinePhase
	enabledWhen PhaseEnabledWhen
}

// PhaseRegistry is an ordered registry of pipeline phases.
// Not a global singleton — callers construct a fresh registry
// (so registration order is deterministic and there is no import-order
// or test-isolation hazard) and Build it per run.
type PhaseRegistry struct {
	registrations []phaseRegistration
}

// NewPhaseRegistry creates a new empty PhaseRegistry.
func NewPhaseRegistry() *PhaseRegistry {
	return &PhaseRegistry{
		registrations: make([]phaseRegistration, 0),
	}
}

// Register adds a phase to the registry. Returns the registry for fluent chaining.
// Registration order is preserved by Build.
func (r *PhaseRegistry) Register(phase PipelinePhase, enabledWhen ...PhaseEnabledWhen) *PhaseRegistry {
	var predicate PhaseEnabledWhen
	if len(enabledWhen) > 0 {
		predicate = enabledWhen[0]
	}
	r.registrations = append(r.registrations, phaseRegistration{
		phase:       phase,
		enabledWhen: predicate,
	})
	return r
}

// Build constructs the ordered phase list for the given options.
// A phase is included iff it has no EnabledWhen predicate or its predicate
// returns true. Order matches registration order.
// If options is nil, all phases without predicates are included (those with
// predicates are evaluated with nil — callers should normalize beforehand).
func (r *PhaseRegistry) Build(options *PipelineOptions) []PipelinePhase {
	result := make([]PipelinePhase, 0, len(r.registrations))
	for _, reg := range r.registrations {
		if reg.enabledWhen == nil || reg.enabledWhen(options) {
			result = append(result, reg.phase)
		}
	}
	return result
}