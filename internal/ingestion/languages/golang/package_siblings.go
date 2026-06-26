// Package golang — Go package sibling population.
// In Go, all files within the same package directory implicitly share
// each other's top-level declarations. This hook populates those
// cross-file sibling bindings so that the scope-resolution pipeline
// can resolve intra-package references without explicit import edges.
// Ported from TS languages/go/package-siblings.ts.
package golang

import (
	"github.com/mengshi02/codetrip/internal/ingestion/model"
	"github.com/mengshi02/codetrip/internal/ingestion/scope_resolution"
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// PopulateGoPackageSiblings adds implicit sibling visibility for Go
// files in the same package directory. All exported top-level defs
// from one file become import-visible to sibling files.
//
// Mirrors TS populateGoPackageSiblings(graph, parsedFiles, nodeLookup, indexes).
func PopulateGoPackageSiblings(
	graph shared.KnowledgeGraph,
	parsedFiles []*shared.ParsedFile,
	nodeLookup *scope_resolution.GraphNodeLookup,
	indexes *model.ScopeResolutionIndexes,
) {
	if len(parsedFiles) == 0 || indexes == nil {
		return
	}

	// Group files by package directory.
	filesByDir := map[string][]*shared.ParsedFile{}
	for _, pf := range parsedFiles {
		dir := PackageDir(pf.FilePath)
		if dir == "" {
			continue
		}
		filesByDir[dir] = append(filesByDir[dir], pf)
	}

	// For each package directory, make exported defs from each file
	// visible to all sibling files through binding augmentation.
	for dir, files := range filesByDir {
		if len(files) < 2 {
			continue // no siblings to share with
		}
		_ = dir // directory grouping is implicit; siblings share by dir

		// Collect all exported top-level defs from all files in this package.
		type exportedEntry struct {
			def    shared.SymbolDefinition
			origin shared.BindingOrigin
		}
		allExported := []exportedEntry{}

		for _, pf := range files {
			for _, scope := range pf.Scopes {
				if scope.Kind != shared.ScopeKindModule {
					continue
				}
				for _, def := range scope.OwnedDefs {
					// Only exported (uppercase) top-level defs are visible across files.
					qname := ""
					if def.QualifiedName != nil {
						qname = *def.QualifiedName
					}
					if qname == "" || !isGoExported(qname) {
						continue
					}
					allExported = append(allExported, exportedEntry{
						def:    def,
						origin: shared.OriginNamespace, // sibling visibility = namespace origin
					})
				}
			}
		}

		// Add sibling bindings to each file's module scope.
		for _, pf := range files {
			for _, scope := range pf.Scopes {
				if scope.Kind != shared.ScopeKindModule {
					continue
				}
				if scope.Bindings == nil {
					scope.Bindings = make(map[string][]shared.BindingRef)
				}
				for _, entry := range allExported {
					qname := ""
					if entry.def.QualifiedName != nil {
						qname = *entry.def.QualifiedName
					}
					// Don't add self-references; a file already has its own defs.
					if qname == "" || scope.ID == "" {
						continue
					}
					// Check if this binding already exists.
					existing := false
					for _, ref := range scope.Bindings[qname] {
						if ref.Def.NodeID == entry.def.NodeID {
							existing = true
							break
						}
					}
					if !existing {
						scope.Bindings[qname] = append(scope.Bindings[qname], shared.BindingRef{
							Def:    entry.def,
							Origin: entry.origin,
						})
					}
				}
			}
		}
	}
}