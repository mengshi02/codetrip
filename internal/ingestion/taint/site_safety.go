package taint

// Site-safety validation for the taint pass (#2083 M3 U1, plan KTD2).
//
// Mirrors hasEmitSafeFacts (cfg/emit.ts): an untrusted cfgSideChannel
// element — possibly from a corrupted durable parsedfile store — must never
// crash the taint pass or fabricate matches from out-of-range indices. The
// degradation contract is per-FUNCTION and one-directional: a CFG whose sites
// fail this check is SKIPPED FOR TAINT ONLY — the BasicBlock/CFG layer and
// the REACHING_DEF projection (guarded by their own checks) are unaffected.
//
// Checked: exactly the indices the taint matcher dereferences — binding
// indices (receiver/object/resultDefs/arg occurrences) against the
// function's binding table, and intra-statement site references (parent
// site / via-tags) against the OWNING statement's sites array. Site
// references are statement-local by construction (each statement's
// FactAccumulator starts at index 0); a cross-statement reference is
// corruption, not a feature.
//
// Lives in taint/ (not cfg/emit.ts): U4's taint emit path is the only
// consumer, and the guard must evolve with the matcher that dereferences
// these fields.

// SiteArgOccurrence represents one occurrence of a binding inside a call/new
// site's argument position. A bare int is a DIRECT occurrence (binding index);
// a [bindingIdx, viaSiteIdx] pair marks an occurrence that reaches this
// argument THROUGH the nested site at viaSiteIdx (index into the same
// statement's sites array).
type SiteArgOccurrence struct {
	Direct     int  // binding index when IsVia is false
	ViaBinding int  // binding index when IsVia is true
	ViaSite    int  // site index when IsVia is true
	IsVia      bool // true = [bindingIdx, viaSiteIdx] tuple
}

// TaintSiteRecord is the M3 call/construct/member-read site record used by
// the taint pass. Spec-AGNOSTIC — records structure only, never
// source/sink/sanitizer-ness (matching is a main-thread concern).
//
// Integer indices: binding fields (receiver/object/resultDefs/arg occurrences)
// index the function's bindings; site references (parent, via-tags) index the
// OWNING statement's sites array.
type TaintSiteRecord struct {
	Kind       string `json:"kind"`                 // "call" | "new" | "member-read"
	Callee     string `json:"callee,omitempty"`      // dotted callee path
	Receiver   *int   `json:"receiver,omitempty"`    // binding index of callee root
	RequireArg string `json:"requireArg,omitempty"`  // CommonJS require string literal
	Template   *bool  `json:"template,omitempty"`    // tagged-template call
	Spread     *int   `json:"spread,omitempty"`      // first spread argument index
	Parent     [2]int `json:"parent,omitempty"`      // [siteIdx, argIdx] of enclosing site
	HasParent  bool   `json:"-"`                     // whether Parent is set
	ResultDefs []int  `json:"resultDefs,omitempty"`  // binding indices defined by this call
	Args       [][]SiteArgOccurrence `json:"args,omitempty"` // per-argument-position occurrences
	Object     *int   `json:"object,omitempty"`       // member-read: binding index of object root
	Property   string `json:"property,omitempty"`     // member-read: property name
}

// TaintStatementFacts carries per-statement def/use facts plus M3 site records.
type TaintStatementFacts struct {
	Line   int               `json:"line"`
	Defs   []int             `json:"defs"`
	Uses   []int             `json:"uses"`
	MayDefs []int            `json:"mayDefs,omitempty"`
	Sites  []TaintSiteRecord `json:"sites,omitempty"`
}

// TaintBasicBlockData is a basic block with optional M3 statement facts.
type TaintBasicBlockData struct {
	Index      int                   `json:"index"`
	StartLine  int                   `json:"startLine"`
	EndLine    int                   `json:"endLine"`
	Text       string                `json:"text"`
	Kind       string                `json:"kind"` // "entry" | "exit" | "normal"
	Statements []TaintStatementFacts `json:"statements,omitempty"`
}

// TaintFunctionCfg is the M3-extended function CFG used by the taint pass.
// It carries the full binding table and statement-local site records.
type TaintFunctionCfg struct {
	FilePath           string              `json:"filePath"`
	FunctionStartLine  int                 `json:"functionStartLine"`
	FunctionEndLine    int                 `json:"functionEndLine"`
	FunctionStartCol   int                 `json:"functionStartColumn"`
	EntryIndex         int                 `json:"entryIndex"`
	ExitIndex          int                 `json:"exitIndex"`
	Blocks             []TaintBasicBlockData `json:"blocks"`
	Bindings           []TaintBindingEntry `json:"bindings,omitempty"`
}

// TaintBindingEntry represents a variable binding in the M3 taint model.
type TaintBindingEntry struct {
	Name       string `json:"name"`
	DeclLine   int    `json:"declLine"`
	DeclColumn int    `json:"declColumn"`
	Kind       string `json:"kind"` // "var"|"let"|"const"|"param"|"catch"|"function"|"class"|"module"
	Synthetic  bool   `json:"synthetic,omitempty"`
}

// allowedSiteKinds are the site kinds the taint matcher dereferences.
var allowedSiteKinds = map[string]bool{
	"call":        true,
	"new":         true,
	"member-read": true,
}

// HasTaintSafeSites reports whether a structurally-valid CFG's M3 sites
// annotations are safe to feed to the taint matcher/propagator. Returns true
// when no statement carries sites (pre-M3 channel, or no calls) — absence is
// the well-formed empty case.
func HasTaintSafeSites(cfg *TaintFunctionCfg) bool {
	if cfg == nil {
		return true
	}
	bindingCount := -1
	if cfg.Bindings != nil {
		bindingCount = len(cfg.Bindings)
	}

	for i := range cfg.Blocks {
		block := &cfg.Blocks[i]
		if block.Statements == nil {
			continue
		}
		if bindingCount < 0 {
			// Sites carry binding indices — a channel with sites but no
			// binding table has nothing to range-check them against: reject.
			return false
		}
		for j := range block.Statements {
			stmt := &block.Statements[j]
			if stmt.Sites == nil {
				continue
			}
			if !isSafeSiteList(stmt.Sites, bindingCount) {
				return false
			}
		}
	}
	return true
}

// isSafeSiteList validates every site in the list against binding and
// intra-statement site index ranges.
func isSafeSiteList(sites []TaintSiteRecord, bindingCount int) bool {
	siteCount := len(sites)
	bindingInRange := func(i int) bool {
		return i >= 0 && i < bindingCount
	}
	siteInRange := func(i int) bool {
		return i >= 0 && i < siteCount
	}

	for si := range sites {
		site := &sites[si]

		// kind must be a recognized taint site kind
		if !allowedSiteKinds[site.Kind] {
			return false
		}
		// receiver: optional binding index
		if site.Receiver != nil && !bindingInRange(*site.Receiver) {
			return false
		}
		// resultDefs: all must be valid binding indices
		for _, idx := range site.ResultDefs {
			if !bindingInRange(idx) {
				return false
			}
		}
		// parent: [siteIdx, argIdx] — site index within same statement
		if site.HasParent {
			p := site.Parent
			if !siteInRange(p[0]) || p[1] < 0 {
				return false
			}
		}
		// spread: non-negative integer
		if site.Spread != nil && *site.Spread < 0 {
			return false
		}
		// args: per-position occurrence lists
		for _, position := range site.Args {
			for _, occ := range position {
				if occ.IsVia {
					if !bindingInRange(occ.ViaBinding) || !siteInRange(occ.ViaSite) {
						return false
					}
				} else {
					if !bindingInRange(occ.Direct) {
						return false
					}
				}
			}
		}
		// member-read: object and property are unconditionally dereferenced
		if site.Kind == "member-read" {
			if site.Object == nil || !bindingInRange(*site.Object) {
				return false
			}
			if site.Property == "" {
				return false
			}
		} else {
			// call/new: object and property are optional
			if site.Object != nil && !bindingInRange(*site.Object) {
				return false
			}
		}
	}
	return true
}