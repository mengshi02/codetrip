package taint

// Source/sink/sanitizer config model — mirrors TS taint/source-sink-config.ts.
//
// The per-language taint configuration shape. Entries describe WHAT a callable
// is (category + how its name resolves), never HOW matching works — matching
// semantics (import joins, shadow checks, spread/template position rules) live
// in the matcher so the spec stays declarative data that can hash into
// taintModelVersion.

// SourceKind is the category of a taint source. Currently only "remote-input".
type SourceKind string

const (
	SourceKindRemoteInput SourceKind = "remote-input"
)

// SinkKind is the vulnerability category for sinks. Sanitizers reference the
// same taxonomy via TaintSanitizerEntry.Neutralizes: a sanitizer kill applies
// only when it neutralizes the matched sink's kind.
type SinkKind string

const (
	SinkKindCommandInjection SinkKind = "command-injection"
	SinkKindCodeInjection    SinkKind = "code-injection"
	SinkKindPathTraversal    SinkKind = "path-traversal"
	SinkKindSQLInjection     SinkKind = "sql-injection"
	SinkKindXSS              SinkKind = "xss"
)

// AllSinkKinds returns the complete set of valid SinkKind values.
func AllSinkKinds() []SinkKind {
	return []SinkKind{
		SinkKindCommandInjection,
		SinkKindCodeInjection,
		SinkKindPathTraversal,
		SinkKindSQLInjection,
		SinkKindXSS,
	}
}

// TaintCallableMatcher identifies a callable that participates in taint flow.
// Name is the callable's own (unqualified) name — qualification comes from
// the resolution-mechanism fields on the extending entry types. Args
// optionally narrows to specific 0-based argument positions that carry taint
// into a sink (or are cleared by a sanitizer); nil means "all positions".
type TaintCallableMatcher struct {
	Name string   `json:"name"`
	Args []int    `json:"args,omitempty"`
	Kind string   `json:"kind"`
}

// TaintSinkEntry is a sink callable. Exactly one resolution mechanism should
// be set per entry:
//   - Module: import-aware resolution against parsed imports
//   - Global: bare-name match when not shadowed by local decl or import
//   - AnyReceiver: method matched on any receiver by final segment
//   - Receivers: method matched only on the listed receiver names
type TaintSinkEntry struct {
	TaintCallableMatcher
	Module     string   `json:"module,omitempty"`
	Global     bool     `json:"global,omitempty"`
	NewOnly    bool     `json:"newOnly,omitempty"`
	AnyReceiver bool    `json:"anyReceiver,omitempty"`
	Receivers  []string `json:"receivers,omitempty"`
}

// TaintSanitizerEntry is a sanitizer callable. Carries the sink kinds it
// Neutralizes instead of a kind of its own. STRICTER resolution than sinks:
// only Module (import-aware) and Global mechanisms exist — never bare-name
// convention — because a sanitizer mis-match is a false KILL.
type TaintSanitizerEntry struct {
	Name        string    `json:"name"`
	Args        []int     `json:"args,omitempty"`
	Neutralizes []SinkKind `json:"neutralizes"`
	Module      string    `json:"module,omitempty"`
	Global      bool      `json:"global,omitempty"`
}

// TaintMemberSourceEntry is a member-read taint source: reading
// <object>.<property> where the object is one of Objects and the property is
// one of Properties. One entry fans out over the Objects × Properties product.
type TaintMemberSourceEntry struct {
	Kind       SourceKind `json:"kind"`
	Objects    []string   `json:"objects"`
	Properties []string   `json:"properties"`
}

// SourceSinkSanitizerSpec is the taint configuration for a single language:
// which member reads introduce taint (sources), which callables are dangerous
// to reach with tainted input (sinks), and which callables clear it
// (sanitizers).
type SourceSinkSanitizerSpec struct {
	Sources    []TaintMemberSourceEntry `json:"sources"`
	Sinks      []TaintSinkEntry         `json:"sinks"`
	Sanitizers []TaintSanitizerEntry    `json:"sanitizers"`
}

// MatchesArg returns whether the matcher applies to the given 0-based
// argument position. Nil Args means all positions match.
func (m *TaintCallableMatcher) MatchesArg(argPos int) bool {
	if len(m.Args) == 0 {
		return true
	}
	for _, a := range m.Args {
		if a == argPos {
			return true
		}
	}
	return false
}

// NeutralizesKind returns whether the sanitizer neutralizes the given sink kind.
func (s *TaintSanitizerEntry) NeutralizesKind(kind SinkKind) bool {
	for _, k := range s.Neutralizes {
		if k == kind {
			return true
		}
	}
	return false
}