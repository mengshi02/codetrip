package taint

// Import-aware taint-site matcher (#2083 M3 U2, plan KTD7).
//
// Classifies a function's harvested TaintSiteRecords against a registered
// SourceSinkSanitizerSpec: which member reads are SOURCES, which call/new
// sites are SINKS (and at which argument positions), and which are
// SANITIZERS. Pure main-thread data work — sites + bindings come from the
// U1 worker harvest, imports from ParsedFile.parsedImports; no AST, no I/O.
//
// PRECONDITION: the caller must gate the CFG through HasTaintSafeSites
// (taint/site_safety.go) first — this module dereferences binding/site
// indices without re-validating them.
//
// ## Callee resolution precedence (bare and member-rooted calls)
//
// 1. ESM import join — the callee root's local name is resolved through the
//    TaintImportIndex built from parsedImports (named/alias members,
//    namespace/default-import module handles).
// 2. require-literal join — a binding whose in-function defining site carries
//    requireArg resolves like a namespace handle. A BARE call of a
//    require-joined binding is matched under BOTH interpretations:
//    <module>.default and <module>.<localName>.
// 3. Bare-name fallback — TRUE GLOBALS only (global: true entries: eval,
//    new Function, encodeURIComponent), and only when the name is neither
//    import-bound nor shadowed.
//
// ## Shadowing rule (exact)
//
// A name is treated as function-local — blocking import/global resolution —
// iff the function's binding table contains a NON-synthetic entry with that
// name. Synthetic bindings (kind "module", synthetic: true) are imports,
// true globals, or enclosing-scope captures and do not shadow.

import (
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// TaintImportBinding records what a local name imported into the file denotes.
type TaintImportBinding struct {
	// Normalized module specifier (node: scheme stripped).
	Module string
	// Exported member bound by a named/aliased import; empty when the local
	// name is a MODULE HANDLE (namespace import, or a default import).
	Member string
}

// TaintImportIndex maps local name → import provenance for one file.
// Build once per file (U4).
type TaintImportIndex map[string]TaintImportBinding

// MatchedSourceRead is a member-read site matched as a taint source.
type MatchedSourceRead struct {
	SiteIndex int
	Entry     TaintMemberSourceEntry
}

// MatchedSinkCall is a call/new site matched as a sink.
type MatchedSinkCall struct {
	SiteIndex    int
	Entry        TaintSinkEntry
	ArgPositions []int
}

// MatchedSanitizerCall is a call site matched as a sanitizer.
type MatchedSanitizerCall struct {
	SiteIndex  int
	Entry      TaintSanitizerEntry
	ResultDefs []int
}

// StatementMatches holds all matches within one statement.
type StatementMatches struct {
	BlockIndex     int
	StatementIndex int
	Line           int
	Sources        []MatchedSourceRead
	Sinks          []MatchedSinkCall
	Sanitizers     []MatchedSanitizerCall
}

// FunctionSiteMatches holds classified sites for one function.
type FunctionSiteMatches struct {
	Statements []StatementMatches
	HasSource  bool
	HasSink    bool
}

// resolvedCallee is the internal resolution of a callee — canonical dotted
// names + syntactic path.
type resolvedCallee struct {
	Path      []string // syntactic dotted path segments
	Canonical []string // module-resolved canonical names
	GlobalRoot bool    // bare root may denote an ECMAScript global
}

// stripNodeScheme strips the "node:" prefix from a module specifier.
func stripNodeScheme(specifier string) string {
	if len(specifier) > 5 && specifier[:5] == "node:" {
		return specifier[5:]
	}
	return specifier
}

// BuildTaintImportIndex builds the local-name → module/member index from a
// file's parsedImports. Only named/alias/namespace kinds bind
// matcher-visible local names; importedName "default" collapses to a module
// handle.
func BuildTaintImportIndex(imports []shared.ParsedImport) TaintImportIndex {
	index := make(TaintImportIndex, len(imports))
	for _, imp := range imports {
		switch imp.Kind {
		case shared.ParsedImportNamed, shared.ParsedImportAlias:
			if imp.TargetRaw == nil {
				continue
			}
			mod := stripNodeScheme(*imp.TargetRaw)
			if imp.ImportedName == "default" {
				index[imp.LocalName] = TaintImportBinding{Module: mod}
			} else {
				index[imp.LocalName] = TaintImportBinding{Module: mod, Member: imp.ImportedName}
			}
		case shared.ParsedImportNamespace:
			if imp.TargetRaw == nil {
				continue
			}
			index[imp.LocalName] = TaintImportBinding{Module: stripNodeScheme(*imp.TargetRaw)}
		}
	}
	return index
}

// MatchFunctionSites classifies a function's harvested sites against a
// language spec. See module doc for resolution precedence, the shadowing
// rule, and gaps.
func MatchFunctionSites(
	cfg *TaintFunctionCfg,
	spec *SourceSinkSanitizerSpec,
	imports TaintImportIndex,
) FunctionSiteMatches {
	bindings := cfg.Bindings

	// Non-synthetic (in-function-declared) binding indices by name.
	nonSyntheticByName := make(map[string][]int)
	for i, b := range bindings {
		if b.Synthetic {
			continue
		}
		nonSyntheticByName[b.Name] = append(nonSyntheticByName[b.Name], i)
	}

	// require-literal join: binding index → module specifier.
	requireByBinding := make(map[int]string)
	conflicted := make(map[int]bool)
	for bi := range cfg.Blocks {
		block := &cfg.Blocks[bi]
		for si := range block.Statements {
			stmt := &block.Statements[si]
			for ssi := range stmt.Sites {
				site := &stmt.Sites[ssi]
				if site.RequireArg == "" || len(site.ResultDefs) == 0 {
					continue
				}
				mod := stripNodeScheme(site.RequireArg)
				for _, def := range site.ResultDefs {
					if conflicted[def] {
						continue
					}
					if prior, ok := requireByBinding[def]; ok {
						if prior != mod {
							delete(requireByBinding, def)
							conflicted[def] = true
						}
					} else {
						requireByBinding[def] = mod
					}
				}
			}
		}
	}

	// resolveCallee resolves a call/new site's callee chain.
	resolveCallee := func(site *TaintSiteRecord) *resolvedCallee {
		if site.Callee == "" {
			return nil
		}
		// Split dotted path
		path := splitDottedPath(site.Callee)
		root := path[0]
		rest := path[1:]
		var canonical []string
		globalRoot := false

		if site.Receiver != nil {
			// Member chain with an identifier root.
			rb := bindings[*site.Receiver]
			if rb.Synthetic {
				imp, ok := imports[rb.Name]
				if ok {
					var base []string
					if imp.Member == "" {
						base = []string{imp.Module}
					} else {
						base = []string{imp.Module, imp.Member}
					}
					full := make([]string, 0, len(base)+len(rest))
					full = append(full, base...)
					full = append(full, rest...)
					canonical = append(canonical, joinDots(full))
				}
			} else {
				if mod, ok := requireByBinding[*site.Receiver]; ok {
					full := make([]string, 0, 1+len(rest))
					full = append(full, mod)
					full = append(full, rest...)
					canonical = append(canonical, joinDots(full))
				}
			}
		} else if len(path) == 1 {
			// Bare call.
			locals, hasLocal := nonSyntheticByName[root]
			if hasLocal {
				for _, idx := range locals {
					if mod, ok := requireByBinding[idx]; ok {
						canonical = append(canonical, mod+".default", mod+"."+root)
					}
				}
			} else {
				imp, ok := imports[root]
				if ok {
					if imp.Member == "" {
						canonical = append(canonical, imp.Module+".default")
					} else {
						canonical = append(canonical, imp.Module+"."+imp.Member)
					}
				} else {
					globalRoot = true
				}
			}
		}

		return &resolvedCallee{Path: path, Canonical: canonical, GlobalRoot: globalRoot}
	}

	var statements []StatementMatches
	hasSource := false
	hasSink := false

	for blockIndex := range cfg.Blocks {
		block := &cfg.Blocks[blockIndex]
		for statementIndex := range block.Statements {
			stmt := &block.Statements[statementIndex]
			if len(stmt.Sites) == 0 {
				continue
			}
			var sources []MatchedSourceRead
			var sinks []MatchedSinkCall
			var sanitizers []MatchedSanitizerCall

			for siteIndex := range stmt.Sites {
				site := &stmt.Sites[siteIndex]

				if site.Kind == "member-read" {
					if site.Object == nil {
						continue
					}
					objIdx := *site.Object
					if objIdx < 0 || objIdx >= len(bindings) {
						continue
					}
					objectName := bindings[objIdx].Name
					property := site.Property
					for ei := range spec.Sources {
						entry := &spec.Sources[ei]
						if containsString(entry.Objects, objectName) && containsString(entry.Properties, property) {
							sources = append(sources, MatchedSourceRead{
								SiteIndex: siteIndex,
								Entry:     *entry,
							})
						}
					}
					continue
				}

				// call / new
				resolved := resolveCallee(site)
				if resolved == nil {
					continue
				}

				// Check sinks
				for ei := range spec.Sinks {
					entry := &spec.Sinks[ei]
					if !sinkMechanismHit(entry, site, resolved) {
						continue
					}
					var argPositions []int
					for p, position := range site.Args {
						if len(position) > 0 && positionMatches(entry, site, p) {
							argPositions = append(argPositions, p)
						}
					}
					if len(argPositions) > 0 {
						sinks = append(sinks, MatchedSinkCall{
							SiteIndex:    siteIndex,
							Entry:        *entry,
							ArgPositions: argPositions,
						})
					}
				}

				// Check sanitizers (module + global mechanisms ONLY)
				for ei := range spec.Sanitizers {
					entry := &spec.Sanitizers[ei]
					if !sanitizerMechanismHit(entry, resolved) {
						continue
					}
					sanitizers = append(sanitizers, MatchedSanitizerCall{
						SiteIndex:  siteIndex,
						Entry:      *entry,
						ResultDefs: site.ResultDefs,
					})
				}
			}

			if len(sources) == 0 && len(sinks) == 0 && len(sanitizers) == 0 {
				continue
			}
			if len(sources) > 0 {
				hasSource = true
			}
			if len(sinks) > 0 {
				hasSink = true
			}
			statements = append(statements, StatementMatches{
				BlockIndex:     blockIndex,
				StatementIndex: statementIndex,
				Line:           stmt.Line,
				Sources:        sources,
				Sinks:          sinks,
				Sanitizers:     sanitizers,
			})
		}
	}

	return FunctionSiteMatches{
		Statements: statements,
		HasSource:  hasSource,
		HasSink:    hasSink,
	}
}

// sinkMechanismHit checks whether a sink entry matches a resolved callee.
func sinkMechanismHit(entry *TaintSinkEntry, site *TaintSiteRecord, r *resolvedCallee) bool {
	if entry.Module != "" {
		target := entry.Module + "." + entry.Name
		for _, c := range r.Canonical {
			if c == target {
				return true
			}
		}
		return false
	}
	if entry.Global {
		return r.GlobalRoot && len(r.Path) == 1 && r.Path[0] == entry.Name &&
			!entry.NewOnly || (entry.NewOnly && site.Kind == "new")
	}
	if entry.AnyReceiver {
		return len(r.Path) >= 2 && r.Path[len(r.Path)-1] == entry.Name
	}
	if len(entry.Receivers) > 0 {
		return len(r.Path) == 2 && containsString(entry.Receivers, r.Path[0]) && r.Path[1] == entry.Name
	}
	return false
}

// sanitizerMechanismHit checks whether a sanitizer entry matches a resolved
// callee. Module + global mechanisms ONLY — never receiver-conventional,
// never bare-name for non-globals.
func sanitizerMechanismHit(entry *TaintSanitizerEntry, r *resolvedCallee) bool {
	if entry.Module != "" {
		target := entry.Module + "." + entry.Name
		for _, c := range r.Canonical {
			if c == target {
				return true
			}
		}
		return false
	}
	if entry.Global {
		return r.GlobalRoot && len(r.Path) == 1 && r.Path[0] == entry.Name
	}
	return false
}

// positionMatches applies the spread/template/registered-position rule.
func positionMatches(entry *TaintSinkEntry, site *TaintSiteRecord, p int) bool {
	if site.Template != nil && *site.Template {
		return true
	}
	if len(entry.Args) == 0 {
		return true
	}
	if site.Spread != nil && p >= *site.Spread {
		spread := *site.Spread
		for _, q := range entry.Args {
			if q >= spread {
				return true
			}
		}
		return false
	}
	return containsInt(entry.Args, p)
}

// splitDottedPath splits a dotted callee path into segments.
func splitDottedPath(s string) []string {
	if s == "" {
		return nil
	}
	count := 1
	for i := 0; i < len(s); i++ {
		if s[i] == '.' {
			count++
		}
	}
	parts := make([]string, 0, count)
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '.' {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	parts = append(parts, s[start:])
	return parts
}

// joinDots joins path segments with dots.
func joinDots(parts []string) string {
	result := parts[0]
	for _, p := range parts[1:] {
		result += "." + p
	}
	return result
}

// containsString reports whether s is in list.
func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// containsInt reports whether n is in list.
func containsInt(list []int, n int) bool {
	for _, v := range list {
		if v == n {
			return true
		}
	}
	return false
}