package scope

import (
	"fmt"
	"strings"

	"github.com/mengshi02/codetrip/internal/graph"
)

// overloadGroup groups method nodes that share the same base name but differ
// by argument-count suffix (#N) or type-hash suffix (~type1,type2).
type overloadGroup struct {
	baseName string
	variants map[string][]*graph.Node // key: "#N" or "~t1,t2" → nodes
}

// OverloadNarrowing resolves overloaded method calls by matching call-site
// argument counts and type signatures against method variants.
//
// Steps:
//  1. Collects all method nodes and groups same-name methods by arg-count
//     suffix (#N) and type-hash suffix (~type1,type2)
//  2. For each unhandled call site, matches the most specific variant
//     based on argument count
//  3. Creates precise CALLS edges with confidence=0.9
//  4. Adds matched sites to handledSites
func OverloadNarrowing(ctx *ScopeContext) int {
	edgesAdded := 0

	// Step 1: Build overload groups from all files' class infos
	groups := buildOverloadGroups(ctx)
	if len(groups) == 0 {
		return 0
	}

	// Step 2: Resolve call sites
	for _, f := range ctx.Files {
		for _, cs := range f.CallSites {
			callerID := cs.CallerID
			if callerID == "" {
				callerID = findCallerID(ctx, cs)
			}
			if callerID == "" {
				continue
			}

			// Skip already-handled sites
			key := siteKey(callerID, cs.Name)
			if ctx.HandledSites[key] {
				continue
			}

			// Look up overload group by base name
			group, ok := groups[cs.Name]
			if !ok {
				continue
			}

			// Match by argument count
			argKey := fmt.Sprintf("#%d", cs.Args)
			candidates, ok := group.variants[argKey]
			if !ok {
				// Fallback: try the generic (no-suffix) variant
				candidates, ok = group.variants[""]
				if !ok {
					continue
				}
			}

			for _, targetNode := range candidates {
				e := graph.NewEdge(graph.RelCalls, callerID, targetNode.ID).
					WithProp("confidence", 0.9).
					WithProp("line", cs.Line).
					WithProp("file", cs.FilePath).
					WithProp("overloadResolution", argKey)
				if err := ctx.Graph.BufferEdge(e); err == nil {
					edgesAdded++
				}
			}

			ctx.HandledSites[key] = true
		}
	}

	return edgesAdded
}

// buildOverloadGroups scans the graph for method nodes and groups them
// by base name. Variant keys are derived from the node name suffix:
//
//	"foo#2"       → base="foo", key="#2"
//	"bar~int,str" → base="bar", key="~int,str"
//	"baz"         → base="baz", key=""
func buildOverloadGroups(ctx *ScopeContext) map[string]*overloadGroup {
	groups := make(map[string]*overloadGroup)

	// Collect methods from class infos in parsed files
	for _, f := range ctx.Files {
		for _, ci := range f.ClassInfos {
			for _, methodName := range ci.Methods {
				base, variant := splitOverloadSuffix(methodName)
				if _, ok := groups[base]; !ok {
					groups[base] = &overloadGroup{
						baseName: base,
						variants: make(map[string][]*graph.Node),
					}
				}
				// Find corresponding method node(s) in the graph
				nodes, err := ctx.Graph.GetNodesByName(ctx.Repo, methodName)
				if err != nil || len(nodes) == 0 {
					// Also try looking up by base name
					nodes, err = ctx.Graph.GetNodesByName(ctx.Repo, base)
					if err != nil {
						continue
					}
				}
				for _, n := range nodes {
					if n.Label != graph.LabelMethod && n.Label != graph.LabelFunction {
						continue
					}
					groups[base].variants[variant] = append(
						groups[base].variants[variant], n)
				}
			}
		}
	}

	// Also scan graph for method nodes with overload suffixes
	iter := ctx.Graph.IterNodes(ctx.Repo)
	defer iter.Close()
	for iter.Next() {
		n := iter.Node()
		if n.Label != graph.LabelMethod && n.Label != graph.LabelFunction {
			continue
		}
		// Only process names with overload suffixes
		if !strings.Contains(n.Name, "#") && !strings.Contains(n.Name, "~") {
			continue
		}
		base, variant := splitOverloadSuffix(n.Name)
		if base == "" {
			continue
		}
		if _, ok := groups[base]; !ok {
			groups[base] = &overloadGroup{
				baseName: base,
				variants: make(map[string][]*graph.Node),
			}
		}
		groups[base].variants[variant] = append(
			groups[base].variants[variant], n)
	}

	return groups
}

// splitOverloadSuffix splits a name with an overload suffix into base + variant.
//   - "foo#2"       → ("foo", "#2")
//   - "bar~int,str" → ("bar", "~int,str")
//   - "baz"         → ("baz", "")
func splitOverloadSuffix(name string) (base, variant string) {
	if idx := strings.Index(name, "#"); idx >= 0 {
		return name[:idx], name[idx:]
	}
	if idx := strings.Index(name, "~"); idx >= 0 {
		return name[:idx], name[idx:]
	}
	return name, ""
}