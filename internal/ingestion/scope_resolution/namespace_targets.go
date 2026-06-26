package scope_resolution

import (
	"github.com/mengshi02/codetrip/internal/ingestion/model"
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// NamespaceTargets builds the namespace-qualified name index for the
// workspace. For languages with explicit namespaces (C++, C#, Java packages),
// this maps short names to their fully-qualified counterparts so that
// cross-file references can be resolved correctly.
//
// Mirrors TS scope-resolution/scope/namespace-targets.ts collectNamespaceTargets.
//
// For namespace imports (`import X`, `import X as Y`), builds a per-file
// localName → targetFilePath[] map over the file's module-scope namespace-kind
// import edges. This is needed because for namespace imports where the target
// module has no self-named def, finalize-algorithm skips binding creation
// entirely, so scope.bindings.get('X') returns undefined. We iterate
// indexes.imports to recover those targets.
func BuildNamespaceTargets(
	parsed *shared.ParsedFile,
	indexes *model.ScopeResolutionIndexes,
) map[string][]string {
	out := make(map[string][]string)

	moduleEdges, ok := indexes.Imports()[parsed.ModuleScope.ID]
	if !ok {
		return out
	}

	// Collect local names from namespace-kind imports
	namespaceLocals := make(map[string]bool)
	for _, imp := range parsed.ParsedImports {
		if imp.Kind == shared.ParsedImportNamespace {
			namespaceLocals[imp.LocalName] = true
		}
	}

	// Walk module-level import edges, matching namespace locals
	for _, edge := range moduleEdges {
		if edge.TargetFile == nil {
			continue
		}
		if !namespaceLocals[edge.LocalName] {
			continue
		}
		targetFile := *edge.TargetFile
		targets := out[edge.LocalName]
		// Dedup
		alreadySeen := false
		for _, t := range targets {
			if t == targetFile {
				alreadySeen = true
				break
			}
		}
		if !alreadySeen {
			out[edge.LocalName] = append(targets, targetFile)
		}
	}

	return out
}

// ResolveNamespaceTarget resolves a short name to a fully-qualified definition
// within the context of a given file's namespace.
//
// Returns nil if no unambiguous resolution can be found.
// If a single candidate matches the short name, return it.
// If multiple candidates, prefer the one in the same namespace as fileNamespace.
// If still ambiguous, return nil.
func ResolveNamespaceTarget(
	shortName string,
	fileNamespace string,
	targets map[string][]shared.SymbolDefinition,
) *shared.SymbolDefinition {
	candidates, ok := targets[shortName]
	if !ok || len(candidates) == 0 {
		return nil
	}

	if len(candidates) == 1 {
		return &candidates[0]
	}

	// Try to find a same-namespace match
	var sameNamespaceMatches []shared.SymbolDefinition
	for _, def := range candidates {
		if def.NamespacePrefix != nil && *def.NamespacePrefix == fileNamespace {
			sameNamespaceMatches = append(sameNamespaceMatches, def)
		}
	}

	if len(sameNamespaceMatches) == 1 {
		return &sameNamespaceMatches[0]
	}

	// Still ambiguous
	return nil
}

// BuildAllNamespaceTargets builds namespace targets for all parsed files.
// Returns a map of filePath → localName → targetFilePath[].
func BuildAllNamespaceTargets(
	parsedFiles []*shared.ParsedFile,
	indexes *model.ScopeResolutionIndexes,
) map[string]map[string][]string {
	result := make(map[string]map[string][]string)
	for _, parsed := range parsedFiles {
		result[parsed.FilePath] = BuildNamespaceTargets(parsed, indexes)
	}
	return result
}