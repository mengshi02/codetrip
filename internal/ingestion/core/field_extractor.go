package core

import (
	"regexp"
	"strings"

	"github.com/odvcencio/gotreesitter"
)

// FieldExtractor extracts field declarations from AST nodes.
// Per-language implementations embed BaseFieldExtractor for shared normalize/resolve logic.
type FieldExtractor interface {
	Language() SupportedLanguage
	Extract(node *gotreesitter.Node, ctx *FieldExtractorContext, source []byte, lang *gotreesitter.Language) *ExtractedFields
	IsTypeDeclaration(node *gotreesitter.Node, lang *gotreesitter.Language) bool
}

// whitespaceRe matches runs of whitespace (spaces, tabs, newlines) for type normalization.
var whitespaceRe = regexp.MustCompile(`\s+`)

// BaseFieldExtractor provides shared field extraction helpers.
// Per-language structs embed this to inherit NormalizeType and ResolveType,
// then implement ExtractVisibility and the remaining FieldExtractor methods.
type BaseFieldExtractor struct {
	LanguageTag SupportedLanguage
}

// Language returns the language this extractor handles.
func (b *BaseFieldExtractor) Language() SupportedLanguage {
	return b.LanguageTag
}

// NormalizeType cleans up a raw type string:
//  1. Trim leading/trailing whitespace
//  2. Collapse internal whitespace runs into single spaces
//  3. Remove surrounding parentheses for simple types
//     (keeps parens for tuple-like types: "(int, string)" stays)
func (b *BaseFieldExtractor) NormalizeType(rawType string) string {
	if rawType == "" {
		return ""
	}
	// Collapse whitespace runs → single space
	result := whitespaceRe.ReplaceAllString(rawType, " ")
	result = strings.TrimSpace(result)

	// Strip surrounding parens only if no comma inside (simple type, not tuple)
	if strings.HasPrefix(result, "(") && strings.HasSuffix(result, ")") {
		inner := result[1 : len(result)-1]
		if !strings.Contains(inner, ",") {
			result = strings.TrimSpace(inner)
		}
	}

	return result
}

// ResolveType resolves a normalized type string to its canonical form.
// Strategy:
//  1. Check TypeEnv for an exact binding → return canonical name
//  2. Check SymbolTable for an exact binding → return canonical name
//  3. Neither found → return the normalized type as-is
func (b *BaseFieldExtractor) ResolveType(normalizedType string, ctx *FieldExtractorContext) string {
	if normalizedType == "" {
		return ""
	}

	// Try TypeEnv first (fast path)
	if ctx.TypeEnv != nil {
		resolved := ctx.TypeEnv.LookupExact(normalizedType)
		if resolved != "" {
			return resolved
		}
	}

	// Try SymbolTable (slow path, full resolution)
	if ctx.SymbolTable != nil {
		def := ctx.SymbolTable.LookupExactFull(ctx.FilePath, normalizedType)
		if def != nil && def.QualifiedName != nil {
			return *def.QualifiedName
		}
	}

	// Neither found → keep normalized form
	return normalizedType
}