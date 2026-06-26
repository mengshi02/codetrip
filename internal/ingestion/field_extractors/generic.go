package fieldextractors

import (
	"github.com/odvcencio/gotreesitter"

	"github.com/mengshi02/codetrip/internal/ingestion/core"
)

// CreateFieldExtractor creates a FieldExtractor from a declarative config.
func CreateFieldExtractor(config core.FieldExtractionConfig) core.FieldExtractor {
	typeDeclSet := makeStringSet(config.TypeDeclarationNodes)
	fieldNodeSet := makeStringSet(config.FieldNodeTypes)
	bodyNodeSet := makeStringSet(config.BodyNodeTypes)
	base := &core.BaseFieldExtractor{LanguageTag: config.Language}

	return &fieldExtractorImpl{
		language:      config.Language,
		config:        config,
		typeDeclSet:   typeDeclSet,
		fieldNodeSet:  fieldNodeSet,
		bodyNodeSet:   bodyNodeSet,
		base:          base,
	}
}

type fieldExtractorImpl struct {
	language     core.SupportedLanguage
	config       core.FieldExtractionConfig
	typeDeclSet  map[string]bool
	fieldNodeSet map[string]bool
	bodyNodeSet  map[string]bool
	base         *core.BaseFieldExtractor
}

func (e *fieldExtractorImpl) Language() core.SupportedLanguage { return e.language }

func (e *fieldExtractorImpl) IsTypeDeclaration(node *gotreesitter.Node, lang *gotreesitter.Language) bool {
	return e.typeDeclSet[node.Type(lang)]
}

func (e *fieldExtractorImpl) Extract(node *gotreesitter.Node, ctx *core.FieldExtractorContext, source []byte, lang *gotreesitter.Language) *core.ExtractedFields {
	if !e.typeDeclSet[node.Type(lang)] {
		return nil
	}

	var ownerFQN *string
	if e.config.ExtractOwnerName != nil {
		ownerFQN = e.config.ExtractOwnerName(node, source, lang)
	}
	if ownerFQN == nil {
		if nameField := node.ChildByFieldName("name", lang); nameField != nil {
			t := nameField.Text(source)
			ownerFQN = &t
		}
	}
	if ownerFQN == nil {
		return nil
	}

	var fields []core.FieldInfo

	// Find body containers
	bodies := e.findBodies(node, lang)
	for _, body := range bodies {
		e.extractFieldsFromBody(body, ctx, source, lang, &fields)
	}

	// Extract primary constructor parameters (e.g. C# record positional params)
	if e.config.ExtractPrimaryFields != nil {
		primaryFields := e.config.ExtractPrimaryFields(node, ctx, source, lang)
		fields = append(fields, primaryFields...)
	}

	return &core.ExtractedFields{
		OwnerFQN: *ownerFQN,
		Fields:   fields,
	}
}

// findBodies finds body container nodes for the type declaration.
func (e *fieldExtractorImpl) findBodies(node *gotreesitter.Node, lang *gotreesitter.Language) []*gotreesitter.Node {
	if e.config.FindBodyNodes != nil {
		return e.config.FindBodyNodes(node, lang)
	}

	var result []*gotreesitter.Node

	// Try named 'body' field first
	if bodyField := node.ChildByFieldName("body", lang); bodyField != nil && e.bodyNodeSet[bodyField.Type(lang)] {
		result = append(result, bodyField)
		return result
	}

	// Walk direct named children for matching body node types
	for i := 0; i < node.NamedChildCount(); i++ {
		child := node.NamedChild(i)
		if child != nil && e.bodyNodeSet[child.Type(lang)] {
			result = append(result, child)
		}
	}

	// Fallback: use 'body' field even if its type is not in bodyNodeSet
	if len(result) == 0 {
		if bodyField := node.ChildByFieldName("body", lang); bodyField != nil {
			result = append(result, bodyField)
		}
	}

	return result
}

// extractFieldsFromBody extracts fields from a body node.
func (e *fieldExtractorImpl) extractFieldsFromBody(
	body *gotreesitter.Node,
	ctx *core.FieldExtractorContext,
	source []byte,
	lang *gotreesitter.Language,
	out *[]core.FieldInfo,
) {
	for i := 0; i < body.NamedChildCount(); i++ {
		child := body.NamedChild(i)
		if child == nil {
			continue
		}

		if e.fieldNodeSet[child.Type(lang)] {
			if e.config.ExtractNames != nil {
				// Multi-name path: one node may declare multiple fields (e.g. Ruby attr_accessor)
				names := e.config.ExtractNames(child, source, lang)
				for _, name := range names {
					if field := e.buildField(child, name, ctx, source, lang); field != nil {
						*out = append(*out, *field)
					}
				}
			} else if e.config.ExtractName != nil {
				name := e.config.ExtractName(child, source, lang)
				if name != nil {
					if field := e.buildField(child, *name, ctx, source, lang); field != nil {
						*out = append(*out, *field)
					}
				}
			}
		}
	}
}

// buildField constructs a single FieldInfo.
func (e *fieldExtractorImpl) buildField(
	node *gotreesitter.Node,
	name string,
	ctx *core.FieldExtractorContext,
	source []byte,
	lang *gotreesitter.Language,
) *core.FieldInfo {
	if name == "" {
		return nil
	}

	var typ *string
	if e.config.ExtractType != nil {
		if t := e.config.ExtractType(node, source, lang); t != nil {
			normalized := e.base.NormalizeType(*t)
			resolved := e.base.ResolveType(normalized, ctx)
			typ = &resolved
		}
	}

	visibility := e.config.DefaultVisibility
	if e.config.ExtractVisibility != nil {
		visibility = e.config.ExtractVisibility(node, lang)
	}
	if e.config.ExtractVisibilityForName != nil {
		visibility = e.config.ExtractVisibilityForName(node, name, lang)
	}

	isStatic := false
	if e.config.IsStatic != nil {
		isStatic = e.config.IsStatic(node, lang)
	}
	isReadonly := false
	if e.config.IsReadonly != nil {
		isReadonly = e.config.IsReadonly(node, lang)
	}

	return &core.FieldInfo{
		Name:       name,
		Type:       typ,
		Visibility: visibility,
		IsStatic:   isStatic,
		IsReadonly: isReadonly,
		SourceFile: ctx.FilePath,
		Line:       int(node.StartPoint().Row) + 1,
	}
}

// makeStringSet creates a map[string]bool from a string slice.
func makeStringSet(items []string) map[string]bool {
	m := make(map[string]bool, len(items))
	for _, s := range items {
		m[s] = true
	}
	return m
}