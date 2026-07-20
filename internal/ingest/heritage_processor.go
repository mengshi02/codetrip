package ingest

import (
	"log"
	"regexp"

	graph "github.com/mengshi02/codetrip/internal/model"
	sitter "github.com/tree-sitter/go-tree-sitter"
)

// Heritage Processor — extracts class inheritance relationships.
//
// Relationship types:
//   - EXTENDS: Class extends another Class (TS, JS, Python, C#, C++)
//   - IMPLEMENTS: Class implements an Interface (TS, C#, Java, Kotlin, PHP)
//
// For languages where tree-sitter can't distinguish classes from interfaces
// in the base_list (C#, Java), we resolve the correct edge type via the
// symbol table. If the parent is registered as Interface -> IMPLEMENTS,
// otherwise EXTENDS. For unresolved external symbols, language-gated heuristic:
//   - C#/Java: I[A-Z] naming convention (IDisposable -> IMPLEMENTS)
//   - Swift: default IMPLEMENTS (protocol conformance is more common)
//   - All others: default EXTENDS

// interfaceNameRE matches C#/Java interface naming convention: I followed by uppercase.
var interfaceNameRE = regexp.MustCompile(`^I[A-Z]`)

func isHeritageTypeDefinition(definition *SymbolDefinition) bool {
	if definition == nil {
		return false
	}
	switch definition.Type {
	case "Class", "Struct", "Record", "Interface", "Trait", "Impl":
		return true
	default:
		return false
	}
}

// resolveHeritageDefinition excludes same-named constructors and methods from
// inheritance endpoints. Constructors commonly share the class name and may
// overwrite the compatibility file index, but can never be an EXTENDS node.
func resolveHeritageDefinition(name, filePath string, ctx *ResolveContext) *SymbolDefinition {
	for _, definition := range ctx.SymbolTable.LookupFuzzy(name) {
		if definition.FilePath == filePath && isHeritageTypeDefinition(definition) {
			return definition
		}
	}
	for _, definition := range visibleTypeDefinitions(name, filePath, ctx) {
		if isHeritageTypeDefinition(definition) {
			return definition
		}
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// resolveExtendsType — determine whether an extends capture is actually IMPLEMENTS.
// ─────────────────────────────────────────────────────────────────────────────

// ExtendsTypeResult holds the resolved relationship type and ID prefix.
type ExtendsTypeResult struct {
	RelType  graph.RelationshipType
	IDPrefix string
}

func resolveExtendsType(
	parentName string,
	currentFilePath string,
	symbolTable *SymbolTable,
	importMap ImportMap,
	language string,
	packageMap PackageMap,
) ExtendsTypeResult {
	ctx := &ResolveContext{
		SymbolTable:    symbolTable,
		NamedImportMap: make(NamedImportMap),
		ImportMap:      importMap,
		PackageMap:     packageMap,
	}
	resolved := resolveHeritageDefinition(parentName, currentFilePath, ctx)
	if resolved != nil {
		if resolved.Type == "Interface" {
			return ExtendsTypeResult{RelType: graph.RelIMPLEMENTS, IDPrefix: "Interface"}
		}
		return ExtendsTypeResult{RelType: graph.RelEXTENDS, IDPrefix: "Class"}
	}
	// Unresolved symbol — fall back to language-specific heuristic
	if language == "csharp" || language == "java" {
		if interfaceNameRE.MatchString(parentName) {
			return ExtendsTypeResult{RelType: graph.RelIMPLEMENTS, IDPrefix: "Interface"}
		}
	} else if language == "swift" {
		// Protocol conformance is far more common than class inheritance in Swift
		return ExtendsTypeResult{RelType: graph.RelIMPLEMENTS, IDPrefix: "Interface"}
	}
	return ExtendsTypeResult{RelType: graph.RelEXTENDS, IDPrefix: "Class"}
}

func makeFallbackID(label string, name string) string {
	return label + ":" + name
}

// makeHeritageRelID creates a relationship ID for heritage edges.
func makeHeritageRelID(relType graph.RelationshipType, src, tgt string) string {
	return string(relType) + ":" + src + "->" + tgt
}

// ─────────────────────────────────────────────────────────────────────────────
// ProcessHeritage — full heritage processing with AST parsing.
// ─────────────────────────────────────────────────────────────────────────────

func ProcessHeritage(
	g *graph.KnowledgeGraph,
	reg *LanguageRegistry,
	files []string,
	langMap map[string]string,
	astCache map[string]*sitter.Tree,
	srcCache map[string][]byte,
	parser *sitter.Parser,
	symbolTable *SymbolTable,
	im ImportMap,
	pm PackageMap,
	nim NamedImportMap,
	order ImportOrderMap,
) {
	for _, fp := range files {
		lang, ok := langMap[fp]
		if !ok {
			continue
		}
		tree, ok := astCache[fp]
		if !ok {
			continue
		}
		src, ok := srcCache[fp]
		if !ok {
			continue
		}
		l, err := reg.GetLanguage(lang)
		if err != nil {
			continue
		}
		qs := LanguageQueries(lang)
		if qs == "" {
			continue
		}
		q, queryErr := sitter.NewQuery(l, qs)
		if queryErr != nil {
			continue
		}
		captureNames := q.CaptureNames()
		qc := sitter.NewQueryCursor()
		matches := qc.Matches(q, tree.RootNode(), src)

		for {
			m := matches.Next()
			if m == nil {
				break
			}
			captureMap := buildCaptureMap(m, captureNames)

			// EXTENDS or IMPLEMENTS: resolve via symbol table for languages where
			// the tree-sitter query can't distinguish classes from interfaces
			classNode, hasClass := captureMap["heritage.class"]
			extendsNode, hasExtends := captureMap["heritage.extends"]

			if hasClass && classNode != nil && hasExtends && extendsNode != nil {
				// Go struct embedding: skip named fields (only anonymous fields are embedded)
				fieldDecl := extendsNode.Parent()
				if fieldDecl != nil && fieldDecl.Kind() == "field_declaration" {
					nameChild := fieldDecl.ChildByFieldName("name")
					if nameChild != nil {
						continue // Named field, not struct embedding
					}
				}

				className := classNode.Utf8Text(src)
				parentClassName := extendsNode.Utf8Text(src)

				ext := resolveExtendsType(parentClassName, fp, symbolTable, im, lang, pm)

				childID := symbolTable.LookupExact(fp, className)
				if childID == "" {
					ctx := &ResolveContext{
						SymbolTable:    symbolTable,
						NamedImportMap: nim,
						ImportMap:      im,
						PackageMap:     pm,
					}
					resolved := ResolveSymbol(className, fp, ctx)
					if resolved != nil && resolved.Definition != nil {
						childID = resolved.Definition.NodeID
					}
				}
				if childID == "" {
					childID = makeFallbackID("Class", fp+":"+className)
				}

				parentID := ""
				{
					ctx := &ResolveContext{
						SymbolTable:    symbolTable,
						NamedImportMap: nim,
						ImportMap:      im,
						PackageMap:     pm,
					}
					resolved := ResolveSymbol(parentClassName, fp, ctx)
					if resolved != nil && resolved.Definition != nil {
						parentID = resolved.Definition.NodeID
					}
				}
				if parentID == "" {
					parentID = makeFallbackID(ext.IDPrefix, parentClassName)
				}

				if childID != "" && parentID != "" && childID != parentID {
					g.AddRelationship(&graph.GraphRelationship{
						ID:         makeHeritageRelID(ext.RelType, childID, parentID),
						SourceID:   childID,
						TargetID:   parentID,
						Type:       ext.RelType,
						Confidence: 1.0,
						Reason:     "",
					})
				}
			}

			// IMPLEMENTS: Class implements Interface (TypeScript explicit)
			implementsNode, hasImplements := captureMap["heritage.implements"]
			if hasClass && classNode != nil && hasImplements && implementsNode != nil {
				className := classNode.Utf8Text(src)
				interfaceName := implementsNode.Utf8Text(src)

				classID := symbolTable.LookupExact(fp, className)
				if classID == "" {
					ctx := &ResolveContext{
						SymbolTable:    symbolTable,
						NamedImportMap: nim,
						ImportMap:      im,
						PackageMap:     pm,
					}
					resolved := ResolveSymbol(className, fp, ctx)
					if resolved != nil && resolved.Definition != nil {
						classID = resolved.Definition.NodeID
					}
				}
				if classID == "" {
					classID = makeFallbackID("Class", fp+":"+className)
				}

				interfaceID := ""
				{
					ctx := &ResolveContext{
						SymbolTable:    symbolTable,
						NamedImportMap: nim,
						ImportMap:      im,
						PackageMap:     pm,
					}
					resolved := ResolveSymbol(interfaceName, fp, ctx)
					if resolved != nil && resolved.Definition != nil {
						interfaceID = resolved.Definition.NodeID
					}
				}
				if interfaceID == "" {
					interfaceID = makeFallbackID("Interface", interfaceName)
				}

				if classID != "" && interfaceID != "" {
					g.AddRelationship(&graph.GraphRelationship{
						ID:         makeHeritageRelID(graph.RelIMPLEMENTS, classID, interfaceID),
						SourceID:   classID,
						TargetID:   interfaceID,
						Type:       graph.RelIMPLEMENTS,
						Confidence: 1.0,
						Reason:     "",
					})
				}
			}

			// IMPLEMENTS (Rust): impl Trait for Struct
			traitNode, hasTrait := captureMap["heritage.trait"]
			if hasClass && classNode != nil && hasTrait && traitNode != nil {
				structName := classNode.Utf8Text(src)
				traitName := traitNode.Utf8Text(src)

				structID := symbolTable.LookupExact(fp, structName)
				if structID == "" {
					ctx := &ResolveContext{
						SymbolTable:    symbolTable,
						NamedImportMap: nim,
						ImportMap:      im,
						PackageMap:     pm,
					}
					resolved := ResolveSymbol(structName, fp, ctx)
					if resolved != nil && resolved.Definition != nil {
						structID = resolved.Definition.NodeID
					}
				}
				if structID == "" {
					structID = makeFallbackID("Struct", fp+":"+structName)
				}

				traitID := ""
				{
					ctx := &ResolveContext{
						SymbolTable:    symbolTable,
						NamedImportMap: nim,
						ImportMap:      im,
						PackageMap:     pm,
					}
					resolved := ResolveSymbol(traitName, fp, ctx)
					if resolved != nil && resolved.Definition != nil {
						traitID = resolved.Definition.NodeID
					}
				}
				if traitID == "" {
					traitID = makeFallbackID("Trait", traitName)
				}

				if structID != "" && traitID != "" {
					g.AddRelationship(&graph.GraphRelationship{
						ID:         makeHeritageRelID(graph.RelIMPLEMENTS, structID, traitID),
						SourceID:   structID,
						TargetID:   traitID,
						Type:       graph.RelIMPLEMENTS,
						Confidence: 1.0,
						Reason:     "trait-impl",
					})
				}
			}
		}
		qc.Close()
		q.Close()
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// ProcessHeritageFromExtracted — fast path using pre-extracted heritage data.
// ─────────────────────────────────────────────────────────────────────────────

func ProcessHeritageFromExtracted(
	g *graph.KnowledgeGraph,
	extractedHeritage []ExtractedHeritage,
	symbolTable *SymbolTable,
	im ImportMap,
	pm PackageMap,
	nim NamedImportMap,
	order ImportOrderMap,
) {
	log.Printf("[heritage-processor] ProcessHeritageFromExtracted: %d heritage entries", len(extractedHeritage))
	for _, h := range extractedHeritage {
		if h.HeritageType == "extends" {
			lang := GetLanguageFromFilename(h.FilePath)
			if lang == "" {
				continue
			}
			ext := resolveExtendsType(h.ParentName, h.FilePath, symbolTable, im, lang, pm)

			ctx := &ResolveContext{SymbolTable: symbolTable, NamedImportMap: nim, ImportMap: im, PackageMap: pm, ImportOrderMap: order}
			childID := ""
			if resolved := resolveHeritageDefinition(h.ChildID, h.FilePath, ctx); resolved != nil {
				childID = resolved.NodeID
			}
			if childID == "" {
				childID = makeFallbackID("Class", h.FilePath+":"+h.ChildID)
			}

			parentID := ""
			if resolved := resolveHeritageDefinition(h.ParentName, h.FilePath, ctx); resolved != nil {
				parentID = resolved.NodeID
			}
			if parentID == "" {
				parentID = makeFallbackID(ext.IDPrefix, h.ParentName)
			}

			if childID != "" && parentID != "" && childID != parentID {
				g.AddRelationship(&graph.GraphRelationship{
					ID:         makeHeritageRelID(ext.RelType, childID, parentID),
					SourceID:   childID,
					TargetID:   parentID,
					Type:       ext.RelType,
					Confidence: 1.0,
					Reason:     "",
				})
			}
		} else if h.HeritageType == "implements" {
			ctx := &ResolveContext{SymbolTable: symbolTable, NamedImportMap: nim, ImportMap: im, PackageMap: pm}
			classID := ""
			if resolved := resolveHeritageDefinition(h.ChildID, h.FilePath, ctx); resolved != nil {
				classID = resolved.NodeID
			}
			if classID == "" {
				classID = makeFallbackID("Class", h.FilePath+":"+h.ChildID)
			}

			interfaceID := ""
			if resolved := resolveHeritageDefinition(h.ParentName, h.FilePath, ctx); resolved != nil {
				interfaceID = resolved.NodeID
			}
			if interfaceID == "" {
				interfaceID = makeFallbackID("Interface", h.ParentName)
			}

			if classID != "" && interfaceID != "" {
				g.AddRelationship(&graph.GraphRelationship{
					ID:         makeHeritageRelID(graph.RelIMPLEMENTS, classID, interfaceID),
					SourceID:   classID,
					TargetID:   interfaceID,
					Type:       graph.RelIMPLEMENTS,
					Confidence: 1.0,
					Reason:     "",
				})
			}
		} else if h.HeritageType == "trait-impl" {
			structID := symbolTable.LookupExact(h.FilePath, h.ChildID)
			if structID == "" {
				ctx := &ResolveContext{
					SymbolTable:    symbolTable,
					NamedImportMap: nim,
					ImportMap:      im,
					PackageMap:     pm,
				}
				resolved := ResolveSymbol(h.ChildID, h.FilePath, ctx)
				if resolved != nil && resolved.Definition != nil {
					structID = resolved.Definition.NodeID
				}
			}
			if structID == "" {
				structID = makeFallbackID("Struct", h.FilePath+":"+h.ChildID)
			}

			traitID := ""
			{
				ctx := &ResolveContext{
					SymbolTable:    symbolTable,
					NamedImportMap: nim,
					ImportMap:      im,
					PackageMap:     pm,
				}
				resolved := ResolveSymbol(h.ParentName, h.FilePath, ctx)
				if resolved != nil && resolved.Definition != nil {
					traitID = resolved.Definition.NodeID
				}
			}
			if traitID == "" {
				traitID = makeFallbackID("Trait", h.ParentName)
			}

			if structID != "" && traitID != "" {
				g.AddRelationship(&graph.GraphRelationship{
					ID:         makeHeritageRelID(graph.RelIMPLEMENTS, structID, traitID),
					SourceID:   structID,
					TargetID:   traitID,
					Type:       graph.RelIMPLEMENTS,
					Confidence: 1.0,
					Reason:     "trait-impl",
				})
			}
		}
	}
}
