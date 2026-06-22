package search

import (
	"strings"

	"github.com/mengshi02/codetrip/internal/graph"
)

// FTS index field name constants
const (
	FieldNodeID    = "nodeID"    // Node ID (stored field)
	FieldName      = "name"      // Symbol name (text, tokenized index + stored)
	FieldLabel     = "label"     // Node type label (keyword, stored)
	FieldFilePath  = "filePath"  // File path (text, tokenized index + stored)
	FieldContent   = "content"   // Composite search text (text, index only not stored)
	FieldStartLine = "startLine" // Start line number (numeric, stored)
	FieldEndLine   = "endLine"   // End line number (numeric, stored)
)

// SearchDocument represents an index document structure
type SearchDocument struct {
	NodeID    string
	Name      string
	Label     string
	FilePath  string
	Content   string
	StartLine int
	EndLine   int
}

// NewSearchDocument converts graph.Node to a bluge index document
// Field design:
//   - FieldNodeID: stored in document _id, used for exact matching and result retrieval
//   - FieldName: text, tokenized index + stored, boost=3.0 during search
//   - FieldLabel: keyword, stored, used for filtering
//   - FieldFilePath: text, tokenized index + stored, boost=1.5 during search
//   - FieldContent: text, index only not stored, aggregates all searchable text
//   - FieldStartLine/FieldEndLine: numeric, stored
func NewSearchDocument(node *graph.Node) *SearchDocument {
	doc := &SearchDocument{
		NodeID: node.ID,
		Name:   node.Name,
		Label:  string(node.Label),
	}

	if node.FilePath != "" {
		doc.FilePath = node.FilePath
	}

	doc.StartLine = node.GetPropInt("startLine")
	doc.EndLine = node.GetPropInt("endLine")

	// Composite search text: aggregate name + label + filePath + property strings
	// Pre-tokenize (camelCase/snake_case splitting) so the analyzer can index correctly
	var content strings.Builder
	content.WriteString(prepareSearchText(node.Name))
	content.WriteString(" ")
	content.WriteString(prepareSearchText(string(node.Label)))
	if node.FilePath != "" {
		content.WriteString(" ")
		content.WriteString(prepareSearchText(node.FilePath))
	}
	for _, k := range node.Props.Keys() {
		v, _ := node.Props.GetProp(k)
		if s, ok := v.(string); ok {
			content.WriteString(" ")
			content.WriteString(prepareSearchText(s))
		}
	}
	doc.Content = content.String()

	return doc
}

// prepareSearchText pre-tokenizes: splits camelCase/snake_case and rejoins
// So the analyzer can correctly index code identifiers
func prepareSearchText(text string) string {
	tokens := tokenize(text)
	return strings.Join(tokens, " ")
}

// tokenize tokenizes: supports camelCase splitting, snake_case splitting, punctuation splitting
// IMPORTANT: camelCase split must happen BEFORE lowercasing, because the split
// relies on uppercase letters as word boundaries.
func tokenize(text string) []string {
	// Replace punctuation with spaces first (preserve case for camelCase split)
	var b strings.Builder
	for _, r := range text {
		if isAlphaNumeric(r) {
			b.WriteRune(r)
		} else {
			b.WriteRune(' ')
		}
	}
	text = b.String()

	// camelCase splitting (must be done BEFORE lowercasing)
	words := make([]string, 0)
	for _, word := range strings.Fields(text) {
		words = append(words, splitCamelCase(word)...)
	}

	// Lowercase after splitting
	lowered := make([]string, 0, len(words))
	for _, w := range words {
		lowered = append(lowered, strings.ToLower(w))
	}

	// Filter stop words and short words
	filtered := make([]string, 0, len(lowered))
	for _, w := range lowered {
		if len(w) >= 2 && !isStopWord(w) {
			filtered = append(filtered, w)
		}
	}
	return filtered
}

// splitCamelCase splits camelCase/PascalCase
func splitCamelCase(word string) []string {
	var parts []string
	current := strings.Builder{}

	for i, r := range word {
		if i > 0 && isUpper(r) && current.Len() > 0 {
			parts = append(parts, current.String())
			current.Reset()
		}
		current.WriteRune(r)
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

func isUpper(r rune) bool { return r >= 'A' && r <= 'Z' }

func isAlphaNumeric(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_'
}

// isStopWord checks if a word is a stop word
func isStopWord(word string) bool {
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "is": true, "are": true,
		"was": true, "were": true, "be": true, "been": true,
		"being": true, "have": true, "has": true, "had": true,
		"do": true, "does": true, "did": true, "will": true,
		"would": true, "could": true, "should": true, "may": true,
		"might": true, "can": true, "shall": true, "to": true,
		"of": true, "in": true, "for": true, "on": true, "with": true,
		"at": true, "by": true, "from": true, "as": true, "it": true,
		"its": true, "this": true, "that": true, "and": true, "or": true,
		"not": true, "no": true, "if": true, "fn": true, "func": true,
		"var": true, "let": true, "const": true, "type": true,
		"return": true, "new": true, "self": true,
	}
	return stopWords[word]
}