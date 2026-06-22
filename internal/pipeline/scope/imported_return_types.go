package scope

import (
	"strings"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/pipeline"
)

// EmitImportedReturnTypes implements imported return-type propagation.
//
// For each ImportInfo it:
//  1. Finds definition nodes for each imported symbol
//  2. Inspects the "returnType" property on definition nodes
//  3. If the return type refers to a type from the import path, creates a
//     USES edge binding the caller's scope to that type
//  4. Propagates type information into the caller's scope via node Props
func EmitImportedReturnTypes(ctx *ScopeContext) int {
	edgesAdded := 0

	for _, f := range ctx.Files {
		for _, imp := range f.Imports {
			// Determine the set of symbol names to inspect
			symbolNames := imp.Symbols
			if imp.IsWildcard {
				// For wildcard imports, discover exported symbols from
				// files that match the import path.
				symbolNames = collectWildcardSymbols(ctx, imp)
			}
			if len(symbolNames) == 0 {
				continue
			}

			for _, symName := range symbolNames {
				defNodes, err := ctx.Graph.GetNodesByName(ctx.Repo, symName)
				if err != nil || len(defNodes) == 0 {
					continue
				}

				for _, defNode := range defNodes {
					// Only consider function/method definitions that have a returnType
					if defNode.Label != graph.LabelFunction && defNode.Label != graph.LabelMethod {
						continue
					}
					returnType := defNode.GetPropString("returnType")
					if returnType == "" {
						continue
					}

					// Check whether the return type originates from the imported path
					typeNodes := resolveTypeFromImport(ctx, returnType, imp)
					if len(typeNodes) == 0 {
						continue
					}

					// Find caller-scope nodes in the importing file
					callerNodes, err := ctx.Graph.GetNodesByFile(ctx.Repo, f.Path)
					if err != nil || len(callerNodes) == 0 {
						continue
					}

					for _, callerNode := range callerNodes {
						if callerNode.Label != graph.LabelFunction && callerNode.Label != graph.LabelMethod {
							continue
						}
						for _, typeNode := range typeNodes {
							e := graph.NewEdge(graph.RelUses, callerNode.ID, typeNode.ID).
								WithProp("confidence", 0.8).
								WithProp("returnType", returnType).
								WithProp("importPath", imp.Path)
							if err := ctx.Graph.BufferEdge(e); err == nil {
								edgesAdded++
							}
						}

						// Propagate type info into caller scope via Props
						existing, _ := callerNode.Props.GetProp("importedReturnTypes")
						var existingSlice []string
						if s, ok := existing.([]string); ok {
							existingSlice = s
						}
						callerNode.Props.SetProp("importedReturnTypes", append(existingSlice, returnType))
					}
				}
			}
		}
	}

	return edgesAdded
}

// resolveTypeFromImport finds type nodes whose name matches returnType and
// whose file path aligns with the import path.
func resolveTypeFromImport(ctx *ScopeContext, returnType string, imp *pipeline.ImportInfo) []*graph.Node {
	candidates, err := ctx.Graph.GetNodesByName(ctx.Repo, returnType)
	if err != nil {
		return nil
	}

	// Filter: type must be a class-like or type-like label and its file path
	// must contain the import path (heuristic for same package).
	var result []*graph.Node
	for _, n := range candidates {
		if !isTypeLikeLabel(n.Label) {
			continue
		}
		if n.FilePath != "" && strings.Contains(n.FilePath, imp.Path) {
			result = append(result, n)
		}
	}
	return result
}

// isTypeLikeLabel returns true for labels that represent types.
func isTypeLikeLabel(label graph.Label) bool {
	switch label {
	case graph.LabelClass, graph.LabelStruct, graph.LabelInterface,
		graph.LabelTrait, graph.LabelType, graph.LabelTypeAlias,
		graph.LabelTypedef:
		return true
	}
	return false
}

// collectWildcardSymbols discovers symbol names from files matching the
// import path for wildcard (dot) imports.
func collectWildcardSymbols(ctx *ScopeContext, imp *pipeline.ImportInfo) []string {
	seen := make(map[string]struct{})
	var names []string

	for _, f := range ctx.Files {
		if !strings.Contains(f.Path, imp.Path) {
			continue
		}
		for _, sym := range f.Symbols {
			if sym.Name == "" {
				continue
			}
			// Only export capitalized names for Go wildcard imports
			if len(sym.Name) > 0 && sym.Name[0] >= 'A' && sym.Name[0] <= 'Z' {
				if _, ok := seen[sym.Name]; !ok {
					seen[sym.Name] = struct{}{}
					names = append(names, sym.Name)
				}
			}
		}
	}
	return names
}