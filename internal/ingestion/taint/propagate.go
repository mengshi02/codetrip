package taint

import (
	"sort"
	"strconv"
)

// Pure intra-procedural taint propagation engine (#2083 M3 U3).
//
// Forward taint reachability over one function's reaching-definition facts
// (M2 computeReachingDefs) and matched taint sites (U2 matchFunctionSites)
// — sources in, findings + sanitizer kills + coverage status out. PURE AND
// DETERMINISTIC: no graph, no I/O, no logger; insertion-ordered worklist;
// explicitly sorted outputs; snapshot tests and content-derived edge ids
// (U4) rely on it.
//
// PRECONDITIONS: the caller gates the CFG through HasTaintSafeSites
// (taint/site_safety.go) and the emit-safety checks before calling — this
// module dereferences binding/site/statement indices without re-validating.
//
// ## The two-rule model (plan HTD)
//
// - Rule (b), statement-local: a matched SOURCE occurrence (member read)
//   whose intra-statement occurrence path — the member-read's parent chain —
//   reaches a matched SINK argument position produces an immediate single-hop
//   finding. The same statement SEEDS taint: every binding the statement
//   defines becomes tainted.
// - Rule (a), worklist: for each tainted (binding, defPoint), every def→use
//   fact delivers the taint to a use statement, where occurrences of the
//   binding in matched sink argument positions produce findings and the
//   statement's own defs are tainted onward.
//
// ## Sanitizer semantics — the KIND-SET exclusion model (KTD4, sharpened)
//
// A taint carries a set of *excluded* (neutralized) SinkKinds accumulated
// through sanitizer hops, and a sink fires unless its kind is in the taint's
// exclusion set. Intersection over paths: a def fed by several occurrence
// paths excludes a kind only when EVERY path neutralizes it.

// DefaultPDGMaxTaintFindingsPerFunction is the default per-function findings
// cap. 200 is generous — a real function with more deduped source→sink
// findings is a fixture or a disaster.
const DefaultPDGMaxTaintFindingsPerFunction = 200

// DefaultPDGMaxTaintHops is the default per-finding hop cap. 32
// intra-procedural def→use hops is far beyond any legible path.
const DefaultPDGMaxTaintHops = 32

// TaintLimits controls propagation budget.
type TaintLimits struct {
	MaxFindingsPerFunction int // 0 = unlimited
	MaxHops                int // 0 = unlimited
}

// ProgramPoint identifies a statement within a function.
type ProgramPoint struct {
	BlockIndex int
	StmtIndex  int
	Line       int
}

// TaintHop is one hop of a finding's path.
type TaintHop struct {
	BindingIdx int
	Name       string
	Point      ProgramPoint
	ViaCall    bool
}

// TaintSourceOccurrence is the rule-(b) source identity material.
type TaintSourceOccurrence struct {
	Point            ProgramPoint
	SiteIndex        int
	ObjectBindingIdx int
	Property         string
	Kind             SourceKind
}

// TaintSinkOccurrence is the sink side of a finding's identity.
type TaintSinkOccurrence struct {
	Point      ProgramPoint
	SiteIndex  int
	ArgIndex   int
	BindingIdx int
	EntryName  string
}

// TaintFinding is a deduped taint flow finding.
type TaintFinding struct {
	SinkKind      SinkKind
	Source        TaintSourceOccurrence
	Sink          TaintSinkOccurrence
	Hops          []TaintHop
	HopsTruncated bool
}

// SanitizerKill records a sanitizer that neutralized kinds on a flowing taint.
type SanitizerKill struct {
	Sanitizer   ProgramPoint
	KilledDef   ProgramPoint
	BindingIdx  int
	Neutralized []SinkKind
}

// FunctionTaintResult is the output of taint propagation for one function.
type FunctionTaintResult struct {
	Status          string // "computed" | "coverage-gap"
	GapReason       string // "truncated" | "overflow" | "no-facts"
	Findings        []TaintFinding
	Kills           []SanitizerKill
	DroppedFindings int
}

// DefUseFact represents a reaching-definition fact: one (binding, def-point)
// that reaches a use-point.
type DefUseFact struct {
	BindingIdx int
	Def        ProgramPoint
	Use        ProgramPoint
}

// FunctionDefUse is the reaching-definitions result for one function.
type FunctionDefUse struct {
	Status   string // "computed" | "truncated" | "overflow" | "no-facts"
	Bindings []TaintBindingEntry
	Facts    []DefUseFact
}

// kindOrder is the canonical SinkKind order for deterministic output.
var kindOrder = []SinkKind{
	SinkKindCodeInjection,
	SinkKindCommandInjection,
	SinkKindPathTraversal,
	SinkKindSQLInjection,
	SinkKindXSS,
}

var kindRank map[SinkKind]int

func init() {
	kindRank = make(map[SinkKind]int, len(kindOrder))
	for i, k := range kindOrder {
		kindRank[k] = i
	}
}

// sortKinds deduplicates and sorts kinds in canonical order.
func sortKinds(kinds []SinkKind) []SinkKind {
	seen := make(map[SinkKind]bool, len(kinds))
	deduped := make([]SinkKind, 0, len(kinds))
	for _, k := range kinds {
		if !seen[k] {
			seen[k] = true
			deduped = append(deduped, k)
		}
	}
	sortSlice(deduped, func(a, b SinkKind) bool {
		ra, okA := kindRank[a]
		rb, okB := kindRank[b]
		if !okA {
			ra = 99
		}
		if !okB {
			rb = 99
		}
		return ra < rb
	})
	return deduped
}

// occPath is one intra-statement occurrence path (interposition evidence).
type occPath struct {
	Kinds      map[SinkKind]bool // kinds neutralized along the path
	ViaCall    bool              // path traverses an unmodeled call/new site
	Sanitizers []sanTraversal    // matched sanitizers traversed
}

type sanTraversal struct {
	SiteIndex int
	Kinds     []SinkKind
}

// stmtContext is per-statement match/site context.
type stmtContext struct {
	Point            ProgramPoint
	Facts            *TaintStatementFacts
	Sites            []TaintSiteRecord
	SinksBySite      map[int][]MatchedSinkCall
	SanitizersBySite map[int][]MatchedSanitizerCall
	ResultDefSites   map[int][]int // binding → site indices whose resultDefs contain it
}

// taintState is one tainted (binding, defPoint) with its exclusion set.
type taintState struct {
	BindingIdx    int
	Point         ProgramPoint
	Exclusions    map[SinkKind]bool // mutable: only ever shrinks
	ParentKey     string
	Source        TaintSourceOccurrence
	ViaCall       bool
	ProcessedSize int // exclusion-set size at last processing
}

// ComputeTaintFlows computes taint flows for one function.
func ComputeTaintFlows(
	cfg *TaintFunctionCfg,
	defUse *FunctionDefUse,
	matches *FunctionSiteMatches,
	limits *TaintLimits,
) FunctionTaintResult {
	if defUse.Status != "computed" {
		return FunctionTaintResult{
			Status:    "coverage-gap",
			GapReason: defUse.Status,
		}
	}

	bindings := defUse.Bindings
	emptyKinds := map[SinkKind]bool{}

	// ── per-statement context (built lazily) ──────────────────────────────
	matchByPoint := make(map[string]*StatementMatches)
	for i := range matches.Statements {
		sm := &matches.Statements[i]
		matchByPoint[pointKeyStr(sm.BlockIndex, sm.StatementIndex)] = sm
	}
	ctxCache := make(map[string]*stmtContext)

	contextAt := func(blockIndex, stmtIndex int) *stmtContext {
		key := pointKeyStr(blockIndex, stmtIndex)
		if ctx, ok := ctxCache[key]; ok {
			return ctx
		}
		if blockIndex < 0 || blockIndex >= len(cfg.Blocks) {
			ctxCache[key] = nil
			return nil
		}
		block := &cfg.Blocks[blockIndex]
		if stmtIndex < 0 || stmtIndex >= len(block.Statements) {
			ctxCache[key] = nil
			return nil
		}
		facts := &block.Statements[stmtIndex]

		sm := matchByPoint[key]
		sinksBySite := make(map[int][]MatchedSinkCall)
		sanitizersBySite := make(map[int][]MatchedSanitizerCall)
		if sm != nil {
			for _, s := range sm.Sinks {
				sinksBySite[s.SiteIndex] = append(sinksBySite[s.SiteIndex], s)
			}
			for _, s := range sm.Sanitizers {
				sanitizersBySite[s.SiteIndex] = append(sanitizersBySite[s.SiteIndex], s)
			}
		}

		resultDefSites := make(map[int][]int)
		for si := range facts.Sites {
			for _, d := range facts.Sites[si].ResultDefs {
				resultDefSites[d] = append(resultDefSites[d], si)
			}
		}

		sites := facts.Sites
		if sites == nil {
			sites = []TaintSiteRecord{}
		}

		ctx := &stmtContext{
			Point:            ProgramPoint{BlockIndex: blockIndex, StmtIndex: stmtIndex, Line: facts.Line},
			Facts:            facts,
			Sites:            sites,
			SinksBySite:      sinksBySite,
			SanitizersBySite: sanitizersBySite,
			ResultDefSites:   resultDefSites,
		}
		ctxCache[key] = ctx
		return ctx
	}

	// neutralizedAt returns kinds neutralized by sanitizers at siteIndex for argPos.
	neutralizedAt := func(ctx *stmtContext, siteIndex, argPos int) []SinkKind {
		sans := ctx.SanitizersBySite[siteIndex]
		if len(sans) == 0 {
			return nil
		}
		site := &ctx.Sites[siteIndex]
		if site.Template != nil && *site.Template {
			return nil
		}
		if site.Spread != nil && argPos >= *site.Spread {
			return nil
		}
		var kinds []SinkKind
		for _, san := range sans {
			if len(san.Entry.Args) == 0 || containsInt(san.Entry.Args, argPos) {
				kinds = append(kinds, san.Entry.Neutralizes...)
			}
		}
		return sortKinds(kinds)
	}

	isUnmodeledCall := func(ctx *stmtContext, siteIndex int) bool {
		site := &ctx.Sites[siteIndex]
		return site.Kind != "member-read" && len(ctx.SanitizersBySite[siteIndex]) == 0
	}

	emerge := func(ctx *stmtContext, siteIndex, argPos int, inner occPath) occPath {
		added := neutralizedAt(ctx, siteIndex, argPos)
		newKinds := make(map[SinkKind]bool, len(inner.Kinds)+len(added))
		for k := range inner.Kinds {
			newKinds[k] = true
		}
		for _, k := range added {
			newKinds[k] = true
		}
		newSans := inner.Sanitizers
		if len(added) > 0 {
			newSans = append(append([]sanTraversal(nil), inner.Sanitizers...), sanTraversal{SiteIndex: siteIndex, Kinds: added})
		}
		return occPath{
			Kinds:      newKinds,
			ViaCall:    inner.ViaCall || isUnmodeledCall(ctx, siteIndex),
			Sanitizers: newSans,
		}
	}

	directPath := occPath{Kinds: emptyKinds}

	// flowsOutOf returns strict occurrence paths of binding b flowing OUT of
	// site siteIndex's result.
	var flowsOutOf func(ctx *stmtContext, b, siteIndex int, guard map[int]bool) []occPath
	flowsOutOf = func(ctx *stmtContext, b, siteIndex int, guard map[int]bool) []occPath {
		if guard[siteIndex] {
			return nil
		}
		guard[siteIndex] = true
		site := &ctx.Sites[siteIndex]
		var out []occPath
		for argPos, entries := range site.Args {
			for _, e := range entries {
				if !e.IsVia {
					if e.Direct == b {
						out = append(out, emerge(ctx, siteIndex, argPos, directPath))
					}
				} else if e.ViaBinding == b {
					inner := flowsOutOf(ctx, b, e.ViaSite, guard)
					if len(inner) > 0 {
						for _, p := range inner {
							out = append(out, emerge(ctx, siteIndex, argPos, p))
						}
					} else {
						fallback := directPath
						fallback.ViaCall = isUnmodeledCall(ctx, e.ViaSite)
						out = append(out, emerge(ctx, siteIndex, argPos, fallback))
					}
				}
			}
		}
		if site.Receiver != nil && *site.Receiver == b {
			p := directPath
			p.ViaCall = isUnmodeledCall(ctx, siteIndex)
			out = append(out, p)
		}
		delete(guard, siteIndex)
		return out
	}

	// pathsIntoPosition returns occurrence paths of b INTO sink position.
	pathsIntoPosition := func(ctx *stmtContext, b, siteIndex, argPos int) []occPath {
		site := &ctx.Sites[siteIndex]
		if argPos >= len(site.Args) {
			return nil
		}
		entries := site.Args[argPos]
		var out []occPath
		for _, e := range entries {
			if !e.IsVia {
				if e.Direct == b {
					out = append(out, directPath)
				}
			} else if e.ViaBinding == b {
				inner := flowsOutOf(ctx, b, e.ViaSite, make(map[int]bool))
				if len(inner) > 0 {
					out = append(out, inner...)
				} else {
					fallback := directPath
					fallback.ViaCall = isUnmodeledCall(ctx, e.ViaSite)
					out = append(out, fallback)
				}
			}
		}
		return out
	}

	// climbSourceChain walks a SOURCE member-read's parent chain.
	climbSourceChain := func(ctx *stmtContext, srcSiteIndex int, onPosition func(siteIndex, argPos int, sofar occPath) bool) {
		visited := make(map[int]bool)
		visited[srcSiteIndex] = true
		cur := &ctx.Sites[srcSiteIndex]
		sofar := directPath
		for cur.HasParent {
			si := cur.Parent[0]
			ap := cur.Parent[1]
			if visited[si] {
				return
			}
			visited[si] = true
			if onPosition(si, ap, sofar) {
				return
			}
			sofar = emerge(ctx, si, ap, sofar)
			if si >= len(ctx.Sites) {
				return
			}
			cur = &ctx.Sites[si]
		}
	}

	// sourceFlowsOutOf returns the source path INTO target (with emergence).
	sourceFlowsOutOf := func(ctx *stmtContext, srcSiteIndex, target int) *occPath {
		var found *occPath
		climbSourceChain(ctx, srcSiteIndex, func(siteIndex, argPos int, sofar occPath) bool {
			if siteIndex == target {
				p := emerge(ctx, target, argPos, sofar)
				found = &p
				return true
			}
			return false
		})
		return found
	}

	// ── accumulators ──────────────────────────────────────────────────────
	findingsByIdentity := make(map[string]TaintFinding)
	type killAccum struct {
		Kill  SanitizerKill
		Kinds map[SinkKind]bool
	}
	killsByIdentity := make(map[string]*killAccum)

	recordKill := func(sanitizer, killedDef ProgramPoint, bindingIdx int, kinds []SinkKind) {
		if len(kinds) == 0 {
			return
		}
		k := programPointKey(sanitizer) + "|" + programPointKey(killedDef) + "|" + itoa(bindingIdx)
		if existing, ok := killsByIdentity[k]; ok {
			for _, kind := range kinds {
				existing.Kinds[kind] = true
			}
		} else {
			km := make(map[SinkKind]bool, len(kinds))
			for _, kind := range kinds {
				km[kind] = true
			}
			killsByIdentity[k] = &killAccum{
				Kill:  SanitizerKill{Sanitizer: sanitizer, KilledDef: killedDef, BindingIdx: bindingIdx},
				Kinds: km,
			}
		}
	}

	findingKey := func(sinkKind SinkKind, source TaintSourceOccurrence, sink TaintSinkOccurrence) string {
		return string(sinkKind) + "|" +
			programPointKey(source.Point) + "|" + itoa(source.SiteIndex) + "|" + itoa(source.ObjectBindingIdx) + "|" + source.Property + "|" +
			programPointKey(sink.Point) + "|" + itoa(sink.SiteIndex) + "|" + itoa(sink.ArgIndex) + "|" + itoa(sink.BindingIdx)
	}

	maxHops := 0
	if limits != nil && limits.MaxHops > 0 {
		maxHops = limits.MaxHops
	}

	recordFinding := func(sinkKind SinkKind, source TaintSourceOccurrence, sink TaintSinkOccurrence, hops []TaintHop, hopsTruncated bool) {
		key := findingKey(sinkKind, source, sink)
		if _, ok := findingsByIdentity[key]; ok {
			return
		}
		truncated := hopsTruncated
		kept := hops
		if maxHops > 0 && len(hops) > maxHops {
			kept = make([]TaintHop, maxHops)
			copy(kept, hops[:maxHops])
			truncated = true
		}
		findingsByIdentity[key] = TaintFinding{
			SinkKind:      sinkKind,
			Source:        source,
			Sink:          sink,
			Hops:          kept,
			HopsTruncated: truncated,
		}
	}

	// ── taint state ───────────────────────────────────────────────────────
	taints := make(map[string]*taintState)
	var queue []string

	stateKey := func(bindingIdx int, point ProgramPoint, source TaintSourceOccurrence) string {
		return itoa(bindingIdx) + ":" + programPointKey(point) + "#" + programPointKey(source.Point) + ":" + itoa(source.SiteIndex)
	}

	deriveTaint := func(bindingIdx int, point ProgramPoint, exclusions map[SinkKind]bool, parentKey string, source TaintSourceOccurrence, viaCall bool) {
		key := stateKey(bindingIdx, point, source)
		if existing, ok := taints[key]; !ok {
			taints[key] = &taintState{
				BindingIdx:    bindingIdx,
				Point:         point,
				Exclusions:    exclusions,
				ParentKey:     parentKey,
				Source:        source,
				ViaCall:       viaCall,
				ProcessedSize: -1,
			}
			queue = append(queue, key)
		} else {
			// Monotone shrink: keep intersection; re-process only when smaller.
			inter := make(map[SinkKind]bool, len(existing.Exclusions))
			for k := range existing.Exclusions {
				if exclusions[k] {
					inter[k] = true
				}
			}
			if len(inter) < len(existing.Exclusions) {
				existing.Exclusions = inter
				existing.ParentKey = parentKey
				existing.Source = source
				existing.ViaCall = viaCall
				queue = append(queue, key)
			}
		}
	}

	// chainHops reconstructs the taint chain from seed to key.
	chainHops := func(key string) (hops []TaintHop, truncated bool) {
		var reversed []TaintHop
		seen := make(map[string]bool)
		cur := key
		for cur != "" {
			if seen[cur] {
				truncated = true
				break
			}
			seen[cur] = true
			t, ok := taints[cur]
			if !ok {
				break
			}
			name := "#" + itoa(t.BindingIdx)
			if t.BindingIdx >= 0 && t.BindingIdx < len(bindings) {
				name = bindings[t.BindingIdx].Name
			}
			hop := TaintHop{BindingIdx: t.BindingIdx, Name: name, Point: t.Point, ViaCall: t.ViaCall}
			reversed = append(reversed, hop)
			cur = t.ParentKey
		}
		// Reverse
		for i, j := 0, len(reversed)-1; i < j; i, j = i+1, j-1 {
			reversed[i], reversed[j] = reversed[j], reversed[i]
		}
		return reversed, truncated
	}

	// summarizePaths returns intersection of path kind-sets; viaCall = any.
	summarizePaths := func(paths []occPath) (kinds map[SinkKind]bool, viaCall bool) {
		var result map[SinkKind]bool
		for _, p := range paths {
			if p.ViaCall {
				viaCall = true
			}
			if result == nil {
				result = make(map[SinkKind]bool, len(p.Kinds))
				for k := range p.Kinds {
					result[k] = true
				}
			} else {
				inter := make(map[SinkKind]bool, len(result))
				for k := range result {
					if p.Kinds[k] {
						inter[k] = true
					}
				}
				result = inter
			}
		}
		if result == nil {
			result = emptyKinds
		}
		return result, viaCall
	}

	// feedDefs taints every def of the statement from one input.
	feedDefs := func(ctx *stmtContext, inputExclusions map[SinkKind]bool, parentKey string, source TaintSourceOccurrence, pathsInto func(siteIndex int) []occPath) {
		defs := ctx.Facts.Defs
		defs = append(defs, ctx.Facts.MayDefs...)
		if len(defs) == 0 {
			return
		}
		seen := make(map[int]bool, len(defs))
		for _, d := range defs {
			if seen[d] {
				continue
			}
			seen[d] = true
			rdSites := ctx.ResultDefSites[d]
			addKinds := emptyKinds
			viaCall := false
			if len(rdSites) > 0 {
				var paths []occPath
				for _, c := range rdSites {
					paths = append(paths, pathsInto(c)...)
				}
				if len(paths) > 0 {
					summary, vc := summarizePaths(paths)
					addKinds = summary
					viaCall = vc
					for _, p := range paths {
						for _, san := range p.Sanitizers {
							recordKill(ctx.Point, ctx.Point, d, san.Kinds)
						}
					}
				}
			}
			exclusions := inputExclusions
			if len(addKinds) > 0 {
				exclusions = make(map[SinkKind]bool, len(inputExclusions)+len(addKinds))
				for k := range inputExclusions {
					exclusions[k] = true
				}
				for k := range addKinds {
					exclusions[k] = true
				}
			}
			deriveTaint(d, ctx.Point, exclusions, parentKey, source, viaCall)
		}
	}

	// ── rule (b) + seeding: statements with matched sources ──────────────
	for _, sm := range matches.Statements {
		if len(sm.Sources) == 0 {
			continue
		}
		ctx := contextAt(sm.BlockIndex, sm.StatementIndex)
		if ctx == nil {
			continue
		}
		for _, src := range sm.Sources {
			if src.SiteIndex >= len(ctx.Sites) {
				continue
			}
			srcSite := &ctx.Sites[src.SiteIndex]
			if srcSite.Object == nil || srcSite.Property == "" {
				continue
			}
			sourceOcc := TaintSourceOccurrence{
				Point:            ctx.Point,
				SiteIndex:        src.SiteIndex,
				ObjectBindingIdx: *srcSite.Object,
				Property:         srcSite.Property,
				Kind:             src.Entry.Kind,
			}

			// Statement-local sink checks along the member-read's parent chain.
			climbSourceChain(ctx, src.SiteIndex, func(siteIndex, argPos int, sofar occPath) bool {
				for _, sink := range ctx.SinksBySite[siteIndex] {
					if !containsInt(sink.ArgPositions, argPos) {
						continue
					}
					kind := SinkKind(sink.Entry.Kind)
					if !sofar.Kinds[kind] {
						bindingName := "#" + itoa(*srcSite.Object)
						if *srcSite.Object >= 0 && *srcSite.Object < len(bindings) {
							bindingName = bindings[*srcSite.Object].Name
						}
						recordFinding(
							kind,
							sourceOcc,
							TaintSinkOccurrence{
								Point:      ctx.Point,
								SiteIndex:  siteIndex,
								ArgIndex:   argPos,
								BindingIdx: *srcSite.Object,
								EntryName:  sink.Entry.Name,
							},
							[]TaintHop{{
								BindingIdx: *srcSite.Object,
								Name:       bindingName,
								Point:      ctx.Point,
								ViaCall:    sofar.ViaCall,
							}},
							false,
						)
					} else {
						for _, san := range sofar.Sanitizers {
							if containsSinkKind(san.Kinds, kind) {
								recordKill(ctx.Point, ctx.Point, *srcSite.Object, san.Kinds)
							}
						}
					}
				}
				return false
			})

			// Seed every def of the statement.
			feedDefs(ctx, emptyKinds, "", sourceOcc, func(c int) []occPath {
				p := sourceFlowsOutOf(ctx, src.SiteIndex, c)
				if p != nil {
					return []occPath{*p}
				}
				return nil
			})
		}
	}

	// ── rule (a): worklist over def→use facts ─────────────────────────────
	factsByDef := make(map[string][]DefUseFact)
	for _, f := range defUse.Facts {
		dk := itoa(f.BindingIdx) + ":" + programPointKey(f.Def)
		factsByDef[dk] = append(factsByDef[dk], f)
	}

	head := 0
	for head < len(queue) {
		key := queue[head]
		head++
		// Reclaim consumed prefix periodically.
		if head > 1024 && head*2 > len(queue) {
			queue = queue[head:]
			head = 0
		}
		t, ok := taints[key]
		if !ok {
			continue
		}
		if t.ProcessedSize == len(t.Exclusions) {
			continue
		}
		t.ProcessedSize = len(t.Exclusions)
		b := t.BindingIdx
		E := t.Exclusions

		dk := itoa(b) + ":" + programPointKey(t.Point)
		for _, fact := range factsByDef[dk] {
			ctx := contextAt(fact.Use.BlockIndex, fact.Use.StmtIndex)
			if ctx == nil {
				continue
			}

			// Sink check: occurrences of b at matched sink argument positions.
			for siteIndex, sinks := range ctx.SinksBySite {
				for _, sink := range sinks {
					kind := SinkKind(sink.Entry.Kind)
					for _, argPos := range sink.ArgPositions {
						paths := pathsIntoPosition(ctx, b, siteIndex, argPos)
						if len(paths) == 0 {
							continue
						}
						if E[kind] {
							continue
						}
						// Find a justifying path (not neutralized).
						var justify *occPath
						for i := range paths {
							if !paths[i].Kinds[kind] {
								justify = &paths[i]
								break
							}
						}
						if justify != nil {
							sinkOcc := TaintSinkOccurrence{
								Point:      ctx.Point,
								SiteIndex:  siteIndex,
								ArgIndex:   argPos,
								BindingIdx: b,
								EntryName:  sink.Entry.Name,
							}
							fk := findingKey(kind, t.Source, sinkOcc)
							if _, exists := findingsByIdentity[fk]; exists {
								continue
							}
							chain, trunc := chainHops(key)
							bindingName := "#" + itoa(b)
							if b >= 0 && b < len(bindings) {
								bindingName = bindings[b].Name
							}
							chain = append(chain, TaintHop{
								BindingIdx: b,
								Name:       bindingName,
								Point:      ctx.Point,
								ViaCall:    justify.ViaCall,
							})
							recordFinding(kind, t.Source, sinkOcc, chain, trunc)
						} else {
							// EVERY path interposed — value-position kill(s).
							for _, p := range paths {
								for _, san := range p.Sanitizers {
									if containsSinkKind(san.Kinds, kind) {
										recordKill(ctx.Point, ctx.Point, b, san.Kinds)
									}
								}
							}
						}
					}
				}
			}

			// Def-feed: the use statement's own defs become tainted.
			feedDefs(ctx, E, key, t.Source, func(c int) []occPath {
				return flowsOutOf(ctx, b, c, make(map[int]bool))
			})
		}
	}

	// ── deterministic assembly ────────────────────────────────────────────
	findings := make([]TaintFinding, 0, len(findingsByIdentity))
	for _, f := range findingsByIdentity {
		findings = append(findings, f)
	}
	sortSlice(findings, func(a, b TaintFinding) bool {
		if c := comparePoints(a.Source.Point, b.Source.Point); c != 0 {
			return c < 0
		}
		if a.Source.SiteIndex != b.Source.SiteIndex {
			return a.Source.SiteIndex < b.Source.SiteIndex
		}
		if c := comparePoints(a.Sink.Point, b.Sink.Point); c != 0 {
			return c < 0
		}
		if a.Sink.SiteIndex != b.Sink.SiteIndex {
			return a.Sink.SiteIndex < b.Sink.SiteIndex
		}
		if a.Sink.ArgIndex != b.Sink.ArgIndex {
			return a.Sink.ArgIndex < b.Sink.ArgIndex
		}
		if a.Sink.BindingIdx != b.Sink.BindingIdx {
			return a.Sink.BindingIdx < b.Sink.BindingIdx
		}
		ra, _ := kindRank[a.SinkKind]
		rb, _ := kindRank[b.SinkKind]
		return ra < rb
	})

	maxFindings := 0
	if limits != nil && limits.MaxFindingsPerFunction > 0 {
		maxFindings = limits.MaxFindingsPerFunction
	}
	kept := findings
	dropped := 0
	if maxFindings > 0 && len(findings) > maxFindings {
		kept = findings[:maxFindings]
		dropped = len(findings) - maxFindings
	}

	kills := make([]SanitizerKill, 0, len(killsByIdentity))
	for _, acc := range killsByIdentity {
		k := acc.Kill
		k.Neutralized = sortKinds(mapKeys(acc.Kinds))
		kills = append(kills, k)
	}
	sortSlice(kills, func(a, b SanitizerKill) bool {
		if c := comparePoints(a.Sanitizer, b.Sanitizer); c != 0 {
			return c < 0
		}
		if c := comparePoints(a.KilledDef, b.KilledDef); c != 0 {
			return c < 0
		}
		return a.BindingIdx < b.BindingIdx
	})

	return FunctionTaintResult{
		Status:          "computed",
		Findings:        kept,
		Kills:           kills,
		DroppedFindings: dropped,
	}
}

// ── helpers ────────────────────────────────────────────────────────────────

func pointKeyStr(blockIndex, stmtIndex int) string {
	return itoa(blockIndex) + ":" + itoa(stmtIndex)
}

func programPointKey(p ProgramPoint) string {
	return itoa(p.BlockIndex) + ":" + itoa(p.StmtIndex) + ":" + itoa(p.Line)
}

func itoa(n int) string {
	return strconv.Itoa(n)
}

func comparePoints(a, b ProgramPoint) int {
	if a.BlockIndex != b.BlockIndex {
		return a.BlockIndex - b.BlockIndex
	}
	return a.StmtIndex - b.StmtIndex
}

func mapKeys(m map[SinkKind]bool) []SinkKind {
	result := make([]SinkKind, 0, len(m))
	for k := range m {
		result = append(result, k)
	}
	return result
}

func containsSinkKind(list []SinkKind, kind SinkKind) bool {
	for _, k := range list {
		if k == kind {
			return true
		}
	}
	return false
}

// sortSlice sorts a slice using the provided less function.
func sortSlice[T any](s []T, less func(a, b T) bool) {
	sort.Slice(s, func(i, j int) bool { return less(s[i], s[j]) })
}
