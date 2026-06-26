package classextractors

import (
	"github.com/odvcencio/gotreesitter"

	"github.com/mengshi02/codetrip/internal/ingestion/core"
	"github.com/mengshi02/codetrip/internal/ingestion/utils"
)

// Default scope name node types that carry naming information.
var defaultScopeNameNodeTypes = map[string]bool{
	"nested_namespace_specifier": true,
	"scoped_identifier":         true,
	"scoped_type_identifier":    true,
	"qualified_name":            true,
	"namespace_name":            true,
	"namespace_identifier":      true,
	"type_identifier":           true,
	"identifier":                true,
	"name":                      true,
	"constant":                  true,
}

// Default type name node types used for extracting a name from a declaration node.
var defaultTypeNameNodeTypes = map[string]bool{
	"type_identifier":      true,
	"identifier":           true,
	"simple_identifier":    true,
	"namespace_identifier": true,
	"constant":             true,
	"name":                 true,
}

// Default node type → class label mapping.
var defaultLabelByNodeType = map[string]core.ClassLikeNodeLabel{
	"class_declaration":         core.NodeLabelClass,
	"abstract_class_declaration": core.NodeLabelClass,
	"interface_declaration":     core.NodeLabelInterface,
	"struct_declaration":        core.NodeLabelStruct,
	"record_declaration":        core.NodeLabelRecord,
	"enum_declaration":          core.NodeLabelEnum,
	"class_definition":          core.NodeLabelClass,
	"struct_specifier":          core.NodeLabelStruct,
	"class_specifier":           core.NodeLabelClass,
	"enum_specifier":            core.NodeLabelEnum,
	"struct_item":               core.NodeLabelStruct,
	"enum_item":                 core.NodeLabelEnum,
	"class":                     core.NodeLabelClass,
	"object_declaration":        core.NodeLabelClass,
	"companion_object":          core.NodeLabelClass,
	"protocol_declaration":      core.NodeLabelInterface,
	"extension_declaration":     core.NodeLabelClass,
}

// classLikeLabels is the set of valid ClassLikeNodeLabel values.
var classLikeLabels = map[core.ClassLikeNodeLabel]bool{
	core.NodeLabelClass:     true,
	core.NodeLabelStruct:    true,
	core.NodeLabelInterface: true,
	core.NodeLabelEnum:      true,
	core.NodeLabelRecord:    true,
}

// CreateClassExtractor creates a ClassExtractor from a declarative config.
func CreateClassExtractor(config core.ClassExtractionConfig) core.ClassExtractor {
	typeDeclSet := makeStringSet(config.TypeDeclarationNodes)
	fileScopeSet := makeStringSet(config.FileScopeNodeTypes)
	ancestorScopeSet := makeStringSet(config.AncestorScopeNodeTypes)

	// Merge default + config scope name node types
	scopeNameNodeTypes := make(map[string]bool)
	for k := range defaultScopeNameNodeTypes {
		scopeNameNodeTypes[k] = true
	}
	for _, t := range config.ScopeNameNodeTypes {
		scopeNameNodeTypes[t] = true
	}

	return &classExtractorImpl{
		language:           config.Language,
		qualifiedNodeID:    config.QualifiedNodeId,
		config:             config,
		typeDeclSet:        typeDeclSet,
		fileScopeSet:       fileScopeSet,
		ancestorScopeSet:   ancestorScopeSet,
		scopeNameNodeTypes: scopeNameNodeTypes,
	}
}

type classExtractorImpl struct {
	language           core.SupportedLanguage
	qualifiedNodeID    bool
	config             core.ClassExtractionConfig
	typeDeclSet        map[string]bool
	fileScopeSet       map[string]bool
	ancestorScopeSet   map[string]bool
	scopeNameNodeTypes map[string]bool
}

func (e *classExtractorImpl) Language() core.SupportedLanguage { return e.language }
func (e *classExtractorImpl) QualifiedNodeId() bool            { return e.qualifiedNodeID }

func (e *classExtractorImpl) IsTypeDeclaration(node *gotreesitter.Node, lang *gotreesitter.Language) bool {
	return e.typeDeclSet[node.Type(lang)]
}

func (e *classExtractorImpl) Extract(node *gotreesitter.Node, fallback *core.ClassExtractFallback, source []byte, lang *gotreesitter.Language) *core.ExtractedClassSymbol {
	if !e.typeDeclSet[node.Type(lang)] {
		return nil
	}

	var name *string
	if e.config.ExtractName != nil {
		name = e.config.ExtractName(node, source, lang)
	}
	if name == nil {
		name = extractTypeNameFromNode(node, source, lang)
	}
	if name == nil && fallback != nil {
		name = fallback.Name
	}

	var typ *core.ClassLikeNodeLabel
	if e.config.ExtractType != nil {
		typ = e.config.ExtractType(node, lang)
	}
	if typ == nil {
		if lbl, ok := defaultLabelByNodeType[node.Type(lang)]; ok {
			typ = &lbl
		}
	}
	if typ == nil && fallback != nil && fallback.Type != nil && isClassLikeLabel(fallback.Type) {
		cl := *fallback.Type
		typ = &cl
	}

	if name == nil || typ == nil {
		return nil
	}

	result := &core.ExtractedClassSymbol{
		Name:          *name,
		Type:          *typ,
		QualifiedName: e.buildQualifiedName(node, *name, source, lang),
	}
	if result.QualifiedName == "" {
		result.QualifiedName = *name
	}

	if e.config.ExtractTemplateArguments != nil {
		if args := e.config.ExtractTemplateArguments(node, source, lang); args != nil {
			result.TemplateArguments = args
		}
	}

	return result
}

func (e *classExtractorImpl) ExtractQualifiedName(node *gotreesitter.Node, simpleName string, source []byte, lang *gotreesitter.Language) *string {
	result := e.Extract(node, &core.ClassExtractFallback{Name: &simpleName}, source, lang)
	if result == nil {
		return nil
	}
	return &result.QualifiedName
}

func (e *classExtractorImpl) QualifyScopeName(node *gotreesitter.Node, simpleName string, source []byte, lang *gotreesitter.Language) string {
	return e.buildQualifiedName(node, simpleName, source, lang)
}

func (e *classExtractorImpl) ShouldSkipClassCapture(ctx *core.ClassCaptureContext, nodeLabel core.ClassLikeNodeLabel, lang *gotreesitter.Language) bool {
	if e.config.ShouldSkipClassCapture != nil {
		return e.config.ShouldSkipClassCapture(ctx, nodeLabel, lang)
	}
	return false
}

func (e *classExtractorImpl) ExtractTemplateArgumentsFromCapture(ctx *core.ClassCaptureContext, source []byte, lang *gotreesitter.Language) []string {
	if e.config.ExtractTemplateArgumentsFromCapture != nil {
		return e.config.ExtractTemplateArgumentsFromCapture(ctx, source, lang)
	}
	return nil
}

// buildQualifiedName constructs the qualified name by collecting file-scope
// and ancestor-scope segments, then appending the simple name.
func (e *classExtractorImpl) buildQualifiedName(node *gotreesitter.Node, simpleName string, source []byte, lang *gotreesitter.Language) string {
	root := node
	for root.Parent() != nil {
		root = root.Parent()
	}

	readScopeSegments := func(scopeNode *gotreesitter.Node) []string {
		if e.config.ExtractScopeSegments != nil {
			if segs := e.config.ExtractScopeSegments(scopeNode, source, lang); segs != nil {
				return segs
			}
		}
		return extractScopeSegmentsFromNode(scopeNode, source, lang, e.scopeNameNodeTypes)
	}

	// File scope segments
	var fileScopeSegments []string
	for i := 0; i < root.NamedChildCount(); i++ {
		child := root.NamedChild(i)
		if child != nil && e.fileScopeSet[child.Type(lang)] {
			fileScopeSegments = append(fileScopeSegments, readScopeSegments(child)...)
		}
	}

	// Ancestor scope segments (collect inner→outer, then reverse)
	var ancestorScopes [][]string
	current := node.Parent()
	for current != nil {
		if e.ancestorScopeSet[current.Type(lang)] {
			segs := readScopeSegments(current)
			if len(segs) > 0 {
				ancestorScopes = append(ancestorScopes, segs)
			}
		}
		current = current.Parent()
	}
	// Reverse ancestor scopes
	for i, j := 0, len(ancestorScopes)-1; i < j; i, j = i+1, j-1 {
		ancestorScopes[i], ancestorScopes[j] = ancestorScopes[j], ancestorScopes[i]
	}

	// Assemble
	var parts []string
	parts = append(parts, fileScopeSegments...)
	for _, segs := range ancestorScopes {
		parts = append(parts, segs...)
	}
	parts = append(parts, utils.SplitQualifiedName(simpleName)...)

	// Filter empty segments and join with dots
	var nonEmpty []string
	for _, p := range parts {
		if p != "" {
			nonEmpty = append(nonEmpty, p)
		}
	}
	if len(nonEmpty) == 0 {
		return ""
	}
	result := nonEmpty[0]
	for _, p := range nonEmpty[1:] {
		result += "." + p
	}
	return result
}

// extractScopeSegmentsFromNode extracts scope segments from a scope node.
func extractScopeSegmentsFromNode(scopeNode *gotreesitter.Node, source []byte, lang *gotreesitter.Language, scopeNameNodeTypes map[string]bool) []string {
	var nameNode *gotreesitter.Node
	if nf := scopeNode.ChildByFieldName("name", lang); nf != nil {
		nameNode = nf
	}
	if nameNode == nil {
		for i := 0; i < scopeNode.NamedChildCount(); i++ {
			child := scopeNode.NamedChild(i)
			if child != nil && scopeNameNodeTypes[child.Type(lang)] {
				nameNode = child
				break
			}
		}
	}
	if nameNode == nil {
		return nil
	}
	return utils.SplitQualifiedName(nameNode.Text(source))
}

// extractTypeNameFromNode extracts a type name from a declaration node.
func extractTypeNameFromNode(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
	if nameField := node.ChildByFieldName("name", lang); nameField != nil {
		t := nameField.Text(source)
		return &t
	}
	for i := 0; i < node.NamedChildCount(); i++ {
		child := node.NamedChild(i)
		if child != nil && defaultTypeNameNodeTypes[child.Type(lang)] {
			t := child.Text(source)
			return &t
		}
	}
	return nil
}

// isClassLikeLabel checks whether a label is a valid ClassLikeNodeLabel.
func isClassLikeLabel(label *core.ClassLikeNodeLabel) bool {
	if label == nil {
		return false
	}
	return classLikeLabels[*label]
}

// makeStringSet creates a map[string]bool from a string slice.
func makeStringSet(items []string) map[string]bool {
	m := make(map[string]bool, len(items))
	for _, s := range items {
		m[s] = true
	}
	return m
}