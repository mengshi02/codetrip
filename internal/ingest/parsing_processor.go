package ingest

// Parsing processor — core definition extraction engine.
//
// Go implementation uses the sequential path only (no Worker Pool).
// The ExtractedData fast path is supported via ProcessImportsFromExtracted,
// ProcessCallsFromExtracted, ProcessHeritageFromExtracted in their respective processors.

import (
	"fmt"
	"log"
	"regexp"
	"strings"

	graph "github.com/mengshi02/codetrip/internal/model"
	sitter "github.com/tree-sitter/go-tree-sitter"
)

// TreeSitterMaxBuffer is the maximum file size (32 MB) that tree-sitter can parse.
const TreeSitterMaxBuffer = 32 * 1024 * 1024

// FileProgressCallback is called to report parsing progress.
type FileProgressCallback func(current int, total int, filePath string)

// ProcessParsing processes all files sequentially, extracting definitions,
// imports, calls, and heritage relationships into the graph and symbol table.
// Returns ExtractedData for fast-path processing by import/call/heritage processors,
// or nil if the sequential path was used (data is already in the graph).
func ProcessParsing(
	g *graph.KnowledgeGraph,
	files []FileInput,
	symbolTable *SymbolTable,
	registry *LanguageRegistry,
	onProgress FileProgressCallback,
) *ExtractedData {
	return processParsingSequential(g, files, symbolTable, registry, onProgress)
}

// FileInput represents a source file with its path and content.
type FileInput struct {
	Path    string
	Content string
}

// processParsingSequential implements the sequential parsing path.
func processParsingSequential(
	g *graph.KnowledgeGraph,
	files []FileInput,
	symbolTable *SymbolTable,
	registry *LanguageRegistry,
	onProgress FileProgressCallback,
) *ExtractedData {
	total := len(files)
	extracted := &ExtractedData{}

	for i, file := range files {
		if onProgress != nil {
			onProgress(i+1, total, file.Path)
		}

		language := GetLanguageFromFilename(file.Path)
		if language == "" {
			continue
		}

		// Skip files larger than the max tree-sitter buffer (32 MB)
		if len(file.Content) > TreeSitterMaxBuffer {
			continue
		}

		// Load language
		lang, err := registry.GetLanguage(language)
		if err != nil {
			continue // parser unavailable
		}

		// Parse
		parser := sitter.NewParser()
		parser.SetLanguage(lang)

		source := []byte(file.Content)
		parseSource := source
		if language == "cpp" {
			parseSource = maskCppTypeDecorationMacros(source)
		}
		tree := parser.Parse(parseSource, nil)
		if tree == nil {
			log.Printf("Skipping unparseable file: %s", file.Path)
			continue
		}

		rootNode := tree.RootNode()

		// Get query string
		queryString := LanguageQueries(language)
		if queryString == "" {
			tree.Close()
			continue
		}

		// Execute query
		query, queryErr := sitter.NewQuery(lang, queryString)
		if query == nil || (queryErr != nil && queryErr.Message != "") {
			log.Printf("Query error for %s: %v", file.Path, queryErr)
			tree.Close()
			continue
		}

		cursor := sitter.NewQueryCursor()
		matches := cursor.Matches(query, rootNode, source)

		// Build capture name index for resolving capture names from IDs
		captureNames := query.CaptureNames()

		// Build TypeEnv for this file (used for receiver type resolution on calls)
		typeEnv := BuildTypeEnv(rootNode, language, source)

		// Process each match
		for {
			match := matches.Next()
			if match == nil {
				break
			}

			processMatch(match, captureNames, file.Path, language, source, g, symbolTable, extracted, typeEnv)
		}

		cursor.Close()
		query.Close()
		tree.Close()
	}

	return extracted
}

var cppTypeDecorationPattern = regexp.MustCompile(`\b(?:class|struct)\s+([A-Z][A-Z0-9_]*(?:EXPORT|API)[A-Z0-9_]*)\s+[A-Za-z_]`)

// maskCppTypeDecorationMacros keeps source offsets stable while allowing the
// C++ grammar to recognize declarations such as `class LEVELDB_EXPORT DB`.
// These all-uppercase tokens are build/export annotations, not type names.
func maskCppTypeDecorationMacros(source []byte) []byte {
	masked := append([]byte(nil), source...)
	for _, match := range cppTypeDecorationPattern.FindAllSubmatchIndex(source, -1) {
		if len(match) < 4 {
			continue
		}
		for index := match[2]; index < match[3]; index++ {
			masked[index] = ' '
		}
	}
	return masked
}

// processMatch processes a single query match, extracting definitions, imports, calls, and heritage.
func processMatch(
	match *sitter.QueryMatch,
	captureNames []string,
	filePath string,
	language string,
	source []byte,
	g *graph.KnowledgeGraph,
	symbolTable *SymbolTable,
	extracted *ExtractedData,
	typeEnv TypeEnv,
) {
	// Build capture map from match — QueryCapture.Node is value type Node, not *Node
	captureMap := make(map[string]*sitter.Node)
	for _, c := range match.Captures {
		idx := int(c.Index)
		name := ""
		if idx < len(captureNames) {
			name = captureNames[idx]
		}
		// Store pointer to Node — we need to make a copy to take address
		node := c.Node // copy the value
		captureMap[name] = &node
	}

	// Skip import captures — handled by import_processor
	if _, ok := captureMap["import"]; ok {
		importNode := captureMap["import"]
		importPath := ""
		// Prefer captureMap["import.source"] / captureMap["import.path"]
		if srcNode, ok2 := captureMap["import.source"]; ok2 && srcNode != nil {
			importPath = stripQuotes(srcNode.Utf8Text(source))
		} else if pathNode, ok2 := captureMap["import.path"]; ok2 && pathNode != nil {
			importPath = stripQuotes(pathNode.Utf8Text(source))
		} else {
			importPath = extractImportPath(importNode, language, source)
		}
		if language == "kotlin" {
			importPath = appendKotlinWildcard(importPath, importNode)
		}
		if importPath != "" {
			extracted.Imports = append(extracted.Imports, ExtractedImport{
				FilePath:   filePath,
				ImportPath: importPath,
				Language:   language,
			})
		}
		return
	}

	// Skip call captures — handled by call_processor
	if _, ok := captureMap["call"]; ok {
		callNode := captureMap["call"]
		callNameNode := captureMap["call.name"]
		callName := ""
		if callNameNode != nil {
			callName = callNameNode.Utf8Text(source)
		}
		if language == "cpp" && isCppStandardLibraryCall(callNameNode, callNode, source) {
			return
		}
		callForm := InferCallForm(callNode, callNameNode, source)
		receiverName := ExtractReceiverName(callNameNode, source)
		var receiverChain []string
		var receiverChainArgCounts []int
		if language == "cpp" && callForm == CallFormMember {
			if base, chain, argCounts := ExtractCppReceiverChain(callNameNode, source); base != "" {
				receiverName, receiverChain, receiverChainArgCounts = base, chain, argCounts
			}
		}

		// Resolve receiver type name for member calls
		receiverTypeName := ""
		if callForm == CallFormMember && receiverName != "" {
			receiverTypeName = LookupTypeEnv(typeEnv, receiverName, callNode, source)
		}
		// Kotlin permits values with operator fun invoke to use function-call
		// syntax. A typed local/parameter named queryBuilder in queryBuilder { }
		// calls QueryBuilder.invoke; it is not a free function named queryBuilder.
		if language == "kotlin" && callForm == CallFormFree && callName != "" {
			if valueType := LookupTypeEnv(typeEnv, callName, callNode, source); valueType != "" {
				receiverName = callName
				receiverTypeName = valueType
				callName = "invoke"
				callForm = CallFormMember
			}
		}

		extracted.Calls = append(extracted.Calls, ExtractedCall{
			FilePath: filePath,
			Language: language,
			// Source ownership is resolved from the byte range in the call
			// declaration-level source inference over-attributes C++ header calls.
			SourceID:               "",
			CallName:               callName,
			ReceiverName:           receiverName,
			ReceiverTypeName:       receiverTypeName,
			ReceiverChain:          receiverChain,
			ReceiverChainArgCounts: receiverChainArgCounts,
			CallForm:               callForm,
			ArgCount:               CountCallArguments(callNode),
			StartByte:              callNode.StartByte(),
		})
		return
	}

	// Heritage captures — extract for heritage_processor fast path
	// Check for heritage.class (some queries use @heritage as parent, others just @heritage.class/@heritage.extends)
	if classNode, ok := captureMap["heritage.class"]; ok {
		extendsNode := captureMap["heritage.extends"]
		implementsNode := captureMap["heritage.implements"]
		traitNode := captureMap["heritage.trait"]

		if classNode != nil {
			className := classNode.Utf8Text(source)
			if extendsNode != nil {
				parentName := extendsNode.Utf8Text(source)
				extracted.Heritage = append(extracted.Heritage, ExtractedHeritage{
					FilePath:     filePath,
					ChildID:      className,
					ParentName:   parentName,
					HeritageType: "extends",
					StartByte:    extendsNode.StartByte(),
				})
			}
			if implementsNode != nil {
				interfaceName := implementsNode.Utf8Text(source)
				extracted.Heritage = append(extracted.Heritage, ExtractedHeritage{
					FilePath:     filePath,
					ChildID:      className,
					ParentName:   interfaceName,
					HeritageType: "implements",
					StartByte:    implementsNode.StartByte(),
				})
			}
			if traitNode != nil {
				traitName := traitNode.Utf8Text(source)
				extracted.Heritage = append(extracted.Heritage, ExtractedHeritage{
					FilePath:     filePath,
					ChildID:      className,
					ParentName:   traitName,
					HeritageType: "trait-impl",
					StartByte:    traitNode.StartByte(),
				})
			}
		}
		return
	}

	// ── Definition node processing ──────────────────────────────────────────
	nameNode := captureMap["name"]
	if nameNode == nil {
		if _, isConstructor := captureMap["definition.constructor"]; !isConstructor {
			return
		}
	}

	nodeName := ""
	if nameNode != nil {
		nodeName = nameNode.Utf8Text(source)
	} else {
		nodeName = "init"
	}

	// Determine label from captures
	nodeLabel := getLabelFromCaptures(captureMap)

	// Get definition node for range
	definitionNode := GetDefinitionNodeFromCaptures(captureMap)
	if language == "cpp" && definitionNode != nil && definitionNode.Kind() == "field_declaration_list" && nameNode != nil {
		for current := nameNode.Parent(); current != nil && current != definitionNode; current = current.Parent() {
			if current.Kind() == "function_definition" {
				definitionNode = current
				break
			}
		}
	}
	cppTestSuite := ""
	if language == "cpp" && definitionNode != nil {
		if testName := extractCppTestMacroName(definitionNode, nodeName, source); testName != "" {
			nodeName = testName
			if idx := strings.Index(testName, "."); idx > 0 {
				cppTestSuite = testName[:idx]
			}
		}
	}

	startLine := 0
	var defStartByte, defEndByte uint
	if definitionNode != nil {
		startLine = int(definitionNode.StartPosition().Row)
		// Use byte offsets from tree-sitter node
		defStartByte = uint(definitionNode.StartByte())
		defEndByte = uint(definitionNode.EndByte())
	} else if nameNode != nil {
		startLine = int(nameNode.StartPosition().Row)
		defStartByte = uint(nameNode.StartByte())
		defEndByte = uint(nameNode.EndByte())
	}

	endLine := startLine
	if definitionNode != nil {
		endLine = int(definitionNode.EndPosition().Row)
	}

	// Check if exported
	isExported := false
	if definitionNode != nil {
		isExported = IsNodeExported(definitionNode, nodeName, language, source)
	} else if nameNode != nil {
		isExported = IsNodeExported(nameNode, nodeName, language, source)
	}

	// Extract method signature for Function/Method/Constructor
	var methodSig *MethodSignature
	if nodeLabel == graph.LabelFunction || nodeLabel == graph.LabelMethod || nodeLabel == graph.LabelConstructor {
		if definitionNode != nil {
			sig := ExtractMethodSignature(definitionNode, source)
			methodSig = &sig
		}
	}

	// Compute enclosing class for Method/Constructor/Property/Function
	needsOwner := nodeLabel == graph.LabelMethod || nodeLabel == graph.LabelConstructor ||
		nodeLabel == graph.LabelProperty || nodeLabel == graph.LabelFunction
	var enclosingClassId string
	if needsOwner {
		rangeNode := nameNode
		if rangeNode == nil {
			rangeNode = definitionNode
		}
		if rangeNode != nil {
			// Use the definition boundary to distinguish direct members from
			// locals nested in a member function or lambda. Starting at the name
			// alone would walk through the declaration's own function node.
			if definitionNode != nil {
				enclosingClassId = FindDirectEnclosingClassId(definitionNode, filePath, source)
			} else {
				enclosingClassId = FindEnclosingClassId(rangeNode, filePath, source)
			}
		}
	}
	if cppTestSuite != "" {
		for _, candidate := range symbolTable.LookupFuzzy(cppTestSuite) {
			if candidate.FilePath == filePath && (candidate.Type == "Class" || candidate.Type == "Struct") {
				enclosingClassId = candidate.NodeID
				break
			}
		}
	}
	if language == "cpp" && enclosingClassId != "" &&
		(nodeLabel == graph.LabelMethod || nodeLabel == graph.LabelFunction) {
		ownerName := enclosingClassId
		if idx := strings.LastIndex(ownerName, ":"); idx >= 0 {
			ownerName = ownerName[idx+1:]
		}
		if nodeName == ownerName {
			nodeLabel = graph.LabelConstructor
		}
	}

	// Direct-owner identity prevents same-name methods in one file from
	// collapsing into a single graph node.
	identityName := nodeName
	if enclosingClassId != "" && cppTestSuite == "" {
		ownerName := enclosingClassId
		if idx := strings.LastIndex(ownerName, ":"); idx >= 0 {
			ownerName = ownerName[idx+1:]
		}
		identityName = ownerName + "." + nodeName
	}
	nodeId := graph.GenerateID(string(nodeLabel), fmt.Sprintf("%s:%s", filePath, identityName))

	// Build graph node after semantic identity and owner are known.
	isExpBool := isExported
	node := &graph.GraphNode{
		ID:    nodeId,
		Label: nodeLabel,
		Properties: graph.NodeProperties{
			Name:       nodeName,
			FilePath:   filePath,
			StartLine:  intPtr(startLine),
			EndLine:    intPtr(endLine),
			Language:   language,
			IsExported: &isExpBool,
		},
	}
	if methodSig != nil {
		node.Properties.ParameterCount = methodSig.ParameterCount
		node.Properties.ReturnType = methodSig.ReturnType
	}
	if language == "cpp" && nodeLabel == graph.LabelConstructor {
		g.AddNodeMergingRange(node)
	} else {
		g.AddNode(node)
	}

	// Add to symbol table
	symType := string(nodeLabel)
	var paramCount *int
	if methodSig != nil {
		paramCount = methodSig.ParameterCount
	}
	symbolTable.Add(filePath, nodeName, nodeId, symType, paramCount, enclosingClassId, defStartByte, defEndByte)
	if methodSig != nil {
		symbolTable.SetReturnType(nodeId, defStartByte, methodSig.ReturnType)
	}
	if language == "cpp" && nodeLabel == graph.LabelTypeAlias && definitionNode != nil {
		if target := definitionNode.ChildByFieldName("type"); target != nil {
			symbolTable.AddTypeAlias(filePath, nodeName, extractSimpleTypeName(target, source))
		}
	}

	// DEFINES relationship
	fileId := graph.GenerateID("File", filePath)
	relId := graph.GenerateID("DEFINES", fmt.Sprintf("%s->%s", fileId, nodeId))
	g.AddRelationship(&graph.GraphRelationship{
		ID:         relId,
		SourceID:   fileId,
		TargetID:   nodeId,
		Type:       graph.RelDEFINES,
		Confidence: 1.0,
		Reason:     "",
	})

	// HAS_METHOD: link method/constructor/property to enclosing class
	if enclosingClassId != "" {
		g.AddRelationship(&graph.GraphRelationship{
			ID:         graph.GenerateID("HAS_METHOD", fmt.Sprintf("%s->%s", enclosingClassId, nodeId)),
			SourceID:   enclosingClassId,
			TargetID:   nodeId,
			Type:       graph.RelHAS_METHOD,
			Confidence: 1.0,
			Reason:     "",
		})
	}
}

func isCppStandardLibraryCall(nameNode, callNode *sitter.Node, source []byte) bool {
	if nameNode == nil || callNode == nil {
		return false
	}
	for current := nameNode.Parent(); current != nil; current = current.Parent() {
		text := strings.TrimSpace(current.Utf8Text(source))
		if strings.HasPrefix(text, "std::") {
			return true
		}
		if current.Kind() == callNode.Kind() && current.StartByte() == callNode.StartByte() {
			break
		}
	}
	return false
}

var cppTestDefinitionMacros = map[string]bool{
	"TEST": true, "TEST_F": true, "TEST_P": true,
	"TYPED_TEST": true, "TYPED_TEST_P": true,
}

// extractCppTestMacroName gives macro-generated GoogleTest bodies a stable,
// semantic owner. Without preprocessing, tree-sitter parses TEST_F(Suite, Name)
// as a function named TEST_F, collapsing unrelated tests into one symbol.
func extractCppTestMacroName(definitionNode *sitter.Node, capturedName string, source []byte) string {
	if definitionNode.Kind() != "function_definition" || !cppTestDefinitionMacros[capturedName] {
		return ""
	}
	declarator := findChildByKind(definitionNode, "function_declarator")
	if declarator == nil {
		return ""
	}
	parameters := declarator.ChildByFieldName("parameters")
	if parameters == nil {
		parameters = findChildByKind(declarator, "parameter_list")
	}
	if parameters == nil || parameters.NamedChildCount() < 2 {
		return ""
	}
	suite := strings.TrimSpace(parameters.NamedChild(0).Utf8Text(source))
	name := strings.TrimSpace(parameters.NamedChild(1).Utf8Text(source))
	if suite == "" || name == "" {
		return ""
	}
	return suite + "." + name
}

// appendKotlinWildcard preserves the wildcard marker, which is represented by
// a sibling AST node rather than being part of the captured identifier text.
func appendKotlinWildcard(importPath string, importNode *sitter.Node) string {
	if importNode == nil || strings.HasSuffix(importPath, ".*") {
		return importPath
	}
	for i := uint(0); i < importNode.ChildCount(); i++ {
		child := importNode.Child(i)
		if child != nil && child.Kind() == "wildcard_import" {
			return importPath + ".*"
		}
	}
	return importPath
}

// ─────────────────────────────────────────────────────────────────────────────
// getLabelFromCaptures — determines node label from query capture keys.
// ─────────────────────────────────────────────────────────────────────────────

func getLabelFromCaptures(captureMap map[string]*sitter.Node) graph.NodeLabel {
	labelMap := []struct {
		key   string
		label graph.NodeLabel
	}{
		{"definition.function", graph.LabelFunction},
		{"definition.class", graph.LabelClass},
		{"definition.interface", graph.LabelInterface},
		{"definition.method", graph.LabelMethod},
		{"definition.struct", graph.LabelStruct},
		{"definition.enum", graph.LabelEnum},
		{"definition.namespace", graph.LabelNamespace},
		{"definition.module", graph.LabelModule},
		{"definition.trait", graph.LabelTrait},
		{"definition.impl", graph.LabelImpl},
		{"definition.type", graph.LabelTypeAlias},
		{"definition.const", graph.LabelConst},
		{"definition.static", graph.LabelStatic},
		{"definition.typedef", graph.LabelTypedef},
		{"definition.macro", graph.LabelMacro},
		{"definition.union", graph.LabelUnion},
		{"definition.property", graph.LabelProperty},
		{"definition.record", graph.LabelRecord},
		{"definition.delegate", graph.LabelDelegate},
		{"definition.annotation", graph.LabelAnnotation},
		{"definition.constructor", graph.LabelConstructor},
		{"definition.template", graph.LabelTemplate},
	}

	for _, entry := range labelMap {
		if _, ok := captureMap[entry.key]; ok {
			return entry.label
		}
	}

	return graph.LabelCodeElement
}

// stripQuotes removes surrounding quotes from a string (single, double, or backtick).
func stripQuotes(s string) string {
	if len(s) >= 2 && (s[0] == '\'' || s[0] == '"' || s[0] == '`') {
		return s[1 : len(s)-1]
	}
	return s
}

// ─────────────────────────────────────────────────────────────────────────────

func extractImportPath(importNode *sitter.Node, language string, source []byte) string {
	switch language {
	case "go":
		for i := uint(0); i < importNode.NamedChildCount(); i++ {
			child := importNode.NamedChild(i)
			if child == nil {
				continue
			}
			if child.Kind() == "import_spec" {
				pathIdent := findChildByKindNamed(child, "path_identifier")
				if pathIdent != nil {
					return pathIdent.Utf8Text(source)
				}
				strLit := findChildByKindNamed(child, "string_literal")
				if strLit != nil {
					text := strLit.Utf8Text(source)
					if len(text) >= 2 {
						return text[1 : len(text)-1]
					}
					return text
				}
			}
			if child.Kind() == "import_spec_list" {
				for j := uint(0); j < child.NamedChildCount(); j++ {
					spec := child.NamedChild(j)
					if spec != nil && spec.Kind() == "import_spec" {
						pathIdent := findChildByKindNamed(spec, "path_identifier")
						if pathIdent != nil {
							return pathIdent.Utf8Text(source)
						}
						strLit := findChildByKindNamed(spec, "string_literal")
						if strLit != nil {
							text := strLit.Utf8Text(source)
							if len(text) >= 2 {
								return text[1 : len(text)-1]
							}
							return text
						}
					}
				}
			}
		}
	case "typescript", "tsx", "javascript":
		src := importNode.ChildByFieldName("source")
		if src != nil {
			text := src.Utf8Text(source)
			if len(text) >= 2 && (text[0] == '\'' || text[0] == '"') {
				return text[1 : len(text)-1]
			}
			return text
		}
		for i := uint(0); i < importNode.NamedChildCount(); i++ {
			child := importNode.NamedChild(i)
			if child != nil && child.Kind() == "string" {
				text := child.Utf8Text(source)
				if len(text) >= 2 {
					return text[1 : len(text)-1]
				}
				return text
			}
		}
	case "python":
		modName := importNode.ChildByFieldName("module_name")
		if modName != nil {
			return modName.Utf8Text(source)
		}
		for i := uint(0); i < importNode.NamedChildCount(); i++ {
			child := importNode.NamedChild(i)
			if child != nil && child.Kind() == "dotted_name" {
				return child.Utf8Text(source)
			}
		}
	case "java":
		scopedId := findChildByKindNamed(importNode, "scoped_identifier")
		if scopedId != nil {
			return scopedId.Utf8Text(source)
		}
	case "rust":
		scopedId := findChildByKindNamed(importNode, "scoped_identifier")
		if scopedId != nil {
			return scopedId.Utf8Text(source)
		}
		arg := findChildByKindNamed(importNode, "argument")
		if arg != nil {
			return arg.Utf8Text(source)
		}
	case "csharp":
		qn := findChildByKindNamed(importNode, "qualified_name")
		if qn != nil {
			return qn.Utf8Text(source)
		}
		nameIdent := findChildByKindNamed(importNode, "name")
		if nameIdent != nil {
			return nameIdent.Utf8Text(source)
		}
	case "php":
		// namespace_use_declaration → namespace_use_clause → qualified_name
		qn := findChildByKindNamed(importNode, "qualified_name")
		if qn != nil {
			return qn.Utf8Text(source)
		}
		// qualified_name may be nested inside namespace_use_clause
		clause := findChildByKindNamed(importNode, "namespace_use_clause")
		if clause != nil {
			qn = findChildByKindNamed(clause, "qualified_name")
			if qn != nil {
				return qn.Utf8Text(source)
			}
		}
		// Also try namespace_use_group_clause for grouped use statements
		groupClause := findChildByKindNamed(importNode, "namespace_use_group")
		if groupClause != nil {
			for i := uint(0); i < groupClause.NamedChildCount(); i++ {
				child := groupClause.NamedChild(i)
				if child != nil {
					qn = findChildByKindNamed(child, "qualified_name")
					if qn != nil {
						return qn.Utf8Text(source)
					}
				}
			}
		}
	case "kotlin":
		ident := findChildByKindNamed(importNode, "identifier")
		if ident != nil {
			return ident.Utf8Text(source)
		}
	}

	return ""
}

// intPtr returns a pointer to the given int value.
func intPtr(v int) *int {
	return &v
}
