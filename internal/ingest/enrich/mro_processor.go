package enrich

import (
	"fmt"

	graph "github.com/mengshi02/codetrip/internal/model"
)

// MRO (Method Resolution Order) Processor.
//
// Walks the inheritance DAG (EXTENDS/IMPLEMENTS edges), collects methods from
// each ancestor via HAS_METHOD edges, detects method-name collisions across
// parents, and applies language-specific resolution rules to emit OVERRIDES edges.
//
// Language-specific rules:
//   - C++:       leftmost base class in declaration order wins
//   - C#/Java:   class method wins over interface default; multiple interface
//                methods with same name are ambiguous (null resolution)
//   - Python:    C3 linearization determines MRO; first in linearized order wins
//   - Rust:      no auto-resolution — requires qualified syntax, resolvedTo = null
//   - Default:   single inheritance — first definition wins
//
// OVERRIDES edge direction: Class → Method (not Method → Method).
// The source is the child class that inherits conflicting methods,
// the target is the winning ancestor method node.

// ─────────────────────────────────────────────────────────────────────────────
// Public types
// ─────────────────────────────────────────────────────────────────────────────

// MethodDef represents a method definition in an ancestor class.
type MethodDef struct {
	ClassID   string
	ClassName string
	MethodID  string
}

// MethodAmbiguity represents a method name collision across ancestors.
type MethodAmbiguity struct {
	MethodName string
	DefinedIn  []MethodDef
	ResolvedTo string // winning methodId, or "" if truly ambiguous
	Reason     string
}

// MROEntry holds the MRO result for a single class.
type MROEntry struct {
	ClassID     string
	ClassName   string
	Language    string
	MRO         []string // linearized parent names
	Ambiguities []MethodAmbiguity
}

// MROResult holds the complete MRO computation result.
type MROResult struct {
	Entries        []MROEntry
	OverrideEdges  int
	AmbiguityCount int
}

// ─────────────────────────────────────────────────────────────────────────────
// buildAdjacency — collect EXTENDS, IMPLEMENTS, and HAS_METHOD adjacency.
// ─────────────────────────────────────────────────────────────────────────────

type adjacency struct {
	// parentMap: childId → parentIds (in insertion / declaration order)
	parentMap map[string][]string
	// methodMap: classId → methodIds
	methodMap map[string][]string
	// parentEdgeType: childId → (parentId → "EXTENDS" or "IMPLEMENTS")
	parentEdgeType map[string]map[string]string
}

func buildAdjacency(g *graph.KnowledgeGraph) adjacency {
	a := adjacency{
		parentMap:      make(map[string][]string),
		methodMap:      make(map[string][]string),
		parentEdgeType: make(map[string]map[string]string),
	}

	g.ForEachRelationship(func(rel *graph.GraphRelationship) {
		if rel.Type == graph.RelEXTENDS || rel.Type == graph.RelIMPLEMENTS {
			a.parentMap[rel.SourceID] = append(a.parentMap[rel.SourceID], rel.TargetID)
			if a.parentEdgeType[rel.SourceID] == nil {
				a.parentEdgeType[rel.SourceID] = make(map[string]string)
			}
			a.parentEdgeType[rel.SourceID][rel.TargetID] = string(rel.Type)
		}
		if rel.Type == graph.RelHAS_METHOD {
			a.methodMap[rel.SourceID] = append(a.methodMap[rel.SourceID], rel.TargetID)
		}
	})

	return a
}

// ─────────────────────────────────────────────────────────────────────────────
// gatherAncestors — BFS topological order of all ancestor IDs.
// ─────────────────────────────────────────────────────────────────────────────

func gatherAncestors(classID string, parentMap map[string][]string) []string {
	visited := make(map[string]bool)
	var order []string
	queue := append([]string{}, parentMap[classID]...)

	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		if visited[id] {
			continue
		}
		visited[id] = true
		order = append(order, id)
		for _, gp := range parentMap[id] {
			if !visited[gp] {
				queue = append(queue, gp)
			}
		}
	}

	return order
}

// ─────────────────────────────────────────────────────────────────────────────
// C3 linearization (Python MRO)
// ─────────────────────────────────────────────────────────────────────────────

// c3Linearize computes C3 linearization for a class.
// Returns the linearized list of ancestor IDs (excluding the class itself),
// or nil if linearization fails (inconsistent or cyclic hierarchy).
func c3Linearize(
	classID string,
	parentMap map[string][]string,
	cache map[string][]string,
	inProgress map[string]bool,
) []string {
	if result, ok := cache[classID]; ok {
		return result
	}

	// Cycle detection
	if inProgress == nil {
		inProgress = make(map[string]bool)
	}
	if inProgress[classID] {
		cache[classID] = nil
		return nil
	}
	inProgress[classID] = true

	directParents, ok := parentMap[classID]
	if !ok || len(directParents) == 0 {
		inProgress[classID] = false
		cache[classID] = []string{}
		return []string{}
	}

	// Compute linearization for each parent first
	var parentLinearizations [][]string
	for _, pid := range directParents {
		pLin := c3Linearize(pid, parentMap, cache, inProgress)
		if pLin == nil {
			inProgress[classID] = false
			cache[classID] = nil
			return nil
		}
		merged := append([]string{pid}, pLin...)
		parentLinearizations = append(parentLinearizations, merged)
	}

	// Add the direct parents list as the final sequence
	sequences := make([][]string, 0, len(parentLinearizations)+1)
	sequences = append(sequences, parentLinearizations...)
	directCopy := make([]string, len(directParents))
	copy(directCopy, directParents)
	sequences = append(sequences, directCopy)

	var result []string

	for {
		// Check if any sequence has remaining elements
		hasElements := false
		for _, seq := range sequences {
			if len(seq) > 0 {
				hasElements = true
				break
			}
		}
		if !hasElements {
			break
		}

		// Find a good head: one that doesn't appear in the tail of any other sequence
		var head string
		foundHead := false
		for _, seq := range sequences {
			if len(seq) == 0 {
				continue
			}
			candidate := seq[0]
			inTail := false
			for _, other := range sequences {
				for i := 1; i < len(other); i++ {
					if other[i] == candidate {
						inTail = true
						break
					}
				}
				if inTail {
					break
				}
			}
			if !inTail {
				head = candidate
				foundHead = true
				break
			}
		}

		if !foundHead {
			// Inconsistent hierarchy
			inProgress[classID] = false
			cache[classID] = nil
			return nil
		}

		result = append(result, head)

		// Remove the chosen head from all sequences
		for i, seq := range sequences {
			if len(seq) > 0 && seq[0] == head {
				sequences[i] = seq[1:]
			}
		}
	}

	inProgress[classID] = false
	cache[classID] = result
	return result
}

// ─────────────────────────────────────────────────────────────────────────────
// Language-specific resolution
// ─────────────────────────────────────────────────────────────────────────────

// resolveByMROOrder resolves by MRO order — first ancestor in linearized order wins.
func resolveByMROOrder(
	methodName string,
	defs []MethodDef,
	mroOrder []string,
	reasonPrefix string,
) (resolvedTo string, reason string) {
	for _, ancestorID := range mroOrder {
		for _, d := range defs {
			if d.ClassID == ancestorID {
				return d.MethodID, fmt.Sprintf("%s: %s::%s", reasonPrefix, d.ClassName, methodName)
			}
		}
	}
	return defs[0].MethodID, reasonPrefix + " fallback: first definition"
}

// resolveCsharpJava resolves for C#/Java/Kotlin: class method wins over interface default.
func resolveCsharpJava(
	methodName string,
	defs []MethodDef,
	parentEdgeTypes map[string]string,
) (resolvedTo string, reason string) {
	var classDefs, interfaceDefs []MethodDef

	for _, def := range defs {
		edgeType, ok := parentEdgeTypes[def.ClassID]
		if ok && edgeType == "IMPLEMENTS" {
			interfaceDefs = append(interfaceDefs, def)
		} else {
			classDefs = append(classDefs, def)
		}
	}

	if len(classDefs) > 0 {
		return classDefs[0].MethodID, fmt.Sprintf("class method wins: %s::%s", classDefs[0].ClassName, methodName)
	}

	if len(interfaceDefs) > 1 {
		names := make([]string, len(interfaceDefs))
		for i, d := range interfaceDefs {
			names[i] = d.ClassName
		}
		return "", fmt.Sprintf("ambiguous: %s defined in multiple interfaces: %v", methodName, names)
	}

	if len(interfaceDefs) == 1 {
		return interfaceDefs[0].MethodID, fmt.Sprintf("single interface default: %s::%s", interfaceDefs[0].ClassName, methodName)
	}

	return "", "no resolution found"
}

// ─────────────────────────────────────────────────────────────────────────────
// buildTransitiveEdgeTypes — BFS propagation of edge types.
// ─────────────────────────────────────────────────────────────────────────────

func buildTransitiveEdgeTypes(
	classID string,
	parentMap map[string][]string,
	parentEdgeType map[string]map[string]string,
) map[string]string {
	result := make(map[string]string)
	directEdges, ok := parentEdgeType[classID]
	if !ok {
		return result
	}

	type bfsEntry struct {
		ID       string
		EdgeType string
	}

	var queue []bfsEntry
	directParents := parentMap[classID]

	for _, pid := range directParents {
		et := "EXTENDS"
		if t, ok := directEdges[pid]; ok {
			et = t
		}
		if _, exists := result[pid]; !exists {
			result[pid] = et
			queue = append(queue, bfsEntry{ID: pid, EdgeType: et})
		}
	}

	for len(queue) > 0 {
		entry := queue[0]
		queue = queue[1:]
		for _, gp := range parentMap[entry.ID] {
			if _, exists := result[gp]; !exists {
				result[gp] = entry.EdgeType
				queue = append(queue, bfsEntry{ID: gp, EdgeType: entry.EdgeType})
			}
		}
	}

	return result
}

// ─────────────────────────────────────────────────────────────────────────────
// ComputeMRO — main entry point.
// ─────────────────────────────────────────────────────────────────────────────

func ComputeMRO(g *graph.KnowledgeGraph) MROResult {
	a := buildAdjacency(g)
	c3Cache := make(map[string][]string)

	var entries []MROEntry
	overrideEdges := 0
	ambiguityCount := 0

	// Process every class that has at least one parent
	for classID, directParents := range a.parentMap {
		if len(directParents) == 0 {
			continue
		}

		classNode, ok := g.GetNode(classID)
		if !ok {
			continue
		}

		language := classNode.Properties.Language
		if language == "" {
			continue
		}
		className := classNode.Properties.Name

		// Compute linearized MRO depending on language
		var mroOrder []string
		if language == "python" {
			c3Result := c3Linearize(classID, a.parentMap, c3Cache, nil)
			if c3Result != nil {
				mroOrder = c3Result
			} else {
				mroOrder = gatherAncestors(classID, a.parentMap)
			}
		} else {
			mroOrder = gatherAncestors(classID, a.parentMap)
		}

		// Get the parent names for the MRO entry
		var mroNames []string
		for _, id := range mroOrder {
			n, ok := g.GetNode(id)
			if ok && n.Properties.Name != "" {
				mroNames = append(mroNames, n.Properties.Name)
			}
		}

		// Collect methods from all ancestors, grouped by method name
		methodsByName := make(map[string][]MethodDef)
		for _, ancestorID := range mroOrder {
			ancestorNode, ok := g.GetNode(ancestorID)
			if !ok {
				continue
			}
			methods := a.methodMap[ancestorID]
			for _, methodID := range methods {
				methodNode, ok := g.GetNode(methodID)
				if !ok {
					continue
				}
				// Properties don't participate in method resolution order
				if methodNode.Label == graph.LabelProperty {
					continue
				}
				methodName := methodNode.Properties.Name
				defs := methodsByName[methodName]
				// Avoid duplicates (same method seen via multiple paths)
				dup := false
				for _, d := range defs {
					if d.MethodID == methodID {
						dup = true
						break
					}
				}
				if !dup {
					methodsByName[methodName] = append(defs, MethodDef{
						ClassID:   ancestorID,
						ClassName: ancestorNode.Properties.Name,
						MethodID:  methodID,
					})
				}
			}
		}

		// Detect collisions: methods defined in 2+ different ancestors
		var ambiguities []MethodAmbiguity

		// A concrete method overriding an inherited declaration is a direct
		// method-to-method semantic fact, independent of ambiguity handling.
		for _, ownMethodID := range a.methodMap[classID] {
			ownNode, ok := g.GetNode(ownMethodID)
			if !ok || ownNode.Label == graph.LabelProperty {
				continue
			}
			for _, ancestorID := range mroOrder {
				matched := false
				for _, inheritedID := range a.methodMap[ancestorID] {
					inheritedNode, ok := g.GetNode(inheritedID)
					if ok && inheritedNode.Properties.Name == ownNode.Properties.Name {
						relID := graph.GenerateID("OVERRIDES", ownMethodID+"->"+inheritedID)
						g.AddRelationship(&graph.GraphRelationship{
							ID: relID, SourceID: ownMethodID, TargetID: inheritedID,
							Type: graph.RelOVERRIDES, Confidence: 1.0,
							Reason: "direct semantic override",
						})
						overrideEdges++
						matched = true
						break
					}
				}
				if matched {
					break
				}
			}
		}

		// Compute transitive edge types once per class (only needed for C#/Java/Kotlin)
		needsEdgeTypes := language == "csharp" || language == "java" || language == "kotlin"
		var classEdgeTypes map[string]string
		if needsEdgeTypes {
			classEdgeTypes = buildTransitiveEdgeTypes(classID, a.parentMap, a.parentEdgeType)
		}

		for methodName, defs := range methodsByName {
			if len(defs) < 2 {
				continue
			}

			// Own method shadows inherited — no ambiguity
			ownMethods := a.methodMap[classID]
			ownDefinesIt := false
			for _, mid := range ownMethods {
				mn, ok := g.GetNode(mid)
				if ok && mn.Properties.Name == methodName {
					ownDefinesIt = true
					break
				}
			}
			if ownDefinesIt {
				continue
			}

			var resolvedTo, reason string

			switch language {
			case "cpp":
				resolvedTo, reason = resolveByMROOrder(methodName, defs, mroOrder, "C++ leftmost base")
			case "csharp", "java", "kotlin":
				resolvedTo, reason = resolveCsharpJava(methodName, defs, classEdgeTypes)
			case "python":
				resolvedTo, reason = resolveByMROOrder(methodName, defs, mroOrder, "Python C3 MRO")
			case "rust":
				resolvedTo = ""
				reason = fmt.Sprintf("Rust requires qualified syntax: <Type as Trait>::%s()", methodName)
			default:
				resolvedTo, reason = resolveByMROOrder(methodName, defs, mroOrder, "first definition")
			}

			ambiguities = append(ambiguities, MethodAmbiguity{
				MethodName: methodName,
				DefinedIn:  defs,
				ResolvedTo: resolvedTo,
				Reason:     reason,
			})

			if resolvedTo == "" {
				ambiguityCount++
			}

		}

		entries = append(entries, MROEntry{
			ClassID:     classID,
			ClassName:   className,
			Language:    language,
			MRO:         mroNames,
			Ambiguities: ambiguities,
		})
	}

	return MROResult{
		Entries:        entries,
		OverrideEdges:  overrideEdges,
		AmbiguityCount: ambiguityCount,
	}
}
