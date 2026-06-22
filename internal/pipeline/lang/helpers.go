package lang

import (
	"sort"
	"strings"

	"github.com/mengshi02/codetrip/internal/pipeline"
)

// ============ Capture Helper Functions ============
//
// These helpers extract text and metadata from LangCapture slices
// produced by tree-sitter query execution. They avoid interface{}
// type assertions by working directly with the concrete LangCapture type.
//
// IMPORTANT: Captures now carry a MatchIndex field that identifies which
// query match they belong to. Helpers that look up sub-captures MUST
// filter by MatchIndex to avoid cross-match contamination.

// findCaptureTextInMatch returns the Text field of the first capture
// with the given Name within the specified matchIndex.
// Returns "" if not found.
func findCaptureTextInMatch(captures []pipeline.LangCapture, matchIndex int, name string) string {
	for i := range captures {
		if captures[i].MatchIndex == matchIndex && captures[i].Name == name {
			return captures[i].Text
		}
	}
	return ""
}

// findCaptureText returns the Text field of the first capture with the given Name.
// Searches across all matches — use only for global/unique captures.
// Returns "" if not found.
func findCaptureText(captures []pipeline.LangCapture, name string) string {
	for i := range captures {
		if captures[i].Name == name {
			return captures[i].Text
		}
	}
	return ""
}

// capturesInMatch returns all captures that belong to the specified matchIndex.
func capturesInMatch(captures []pipeline.LangCapture, matchIndex int) []pipeline.LangCapture {
	var result []pipeline.LangCapture
	for i := range captures {
		if captures[i].MatchIndex == matchIndex {
			result = append(result, captures[i])
		}
	}
	return result
}

// findChildText returns the Text field of the first child capture with the given NodeType.
// Returns "" if not found.
func findChildText(captures []pipeline.LangCapture, nodeType string) string {
	for i := range captures {
		if captures[i].NodeType == nodeType {
			return captures[i].Text
		}
	}
	return ""
}

// findChildrenForNode returns all captures that have the given NodeType as their parent.
// This is used to find sub-captures belonging to a specific match.
func findChildrenForNode(captures []pipeline.LangCapture, nodeType string) []pipeline.LangCapture {
	var result []pipeline.LangCapture
	for i := range captures {
		if captures[i].NodeType == nodeType {
			result = append(result, captures[i])
		}
	}
	return result
}

// captureByName returns captures filtered by Name.
func captureByName(captures []pipeline.LangCapture, name string) []pipeline.LangCapture {
	var result []pipeline.LangCapture
	for i := range captures {
		if captures[i].Name == name {
			result = append(result, captures[i])
		}
	}
	return result
}

// captureByNameInMatch returns captures filtered by Name within a specific match.
func captureByNameInMatch(captures []pipeline.LangCapture, matchIndex int, name string) []pipeline.LangCapture {
	var result []pipeline.LangCapture
	for i := range captures {
		if captures[i].MatchIndex == matchIndex && captures[i].Name == name {
			result = append(result, captures[i])
		}
	}
	return result
}

// firstLineOfCapture extracts the first line of a capture's text.
// The source parameter is kept for API consistency but the text is
// read from cap.Text which was populated during query execution.
func firstLineOfCapture(cap pipeline.LangCapture, source []byte) string {
	text := cap.Text
	if idx := strings.IndexByte(text, '\n'); idx >= 0 {
		return text[:idx]
	}
	return text
}

// buildScopeParentIDs computes the ParentID field for each ScopeInfo by
// line-range nesting: a child scope's [StartLine, EndLine] is strictly
// within the parent scope's range. We use a stack-based algorithm:
//  1. Sort scopes by StartLine ascending, then by EndLine descending
//     (wider scopes first for same start line).
//  2. Walk sorted scopes, maintaining a stack of "open" scopes.
//  3. For each scope, pop the stack until the top contains the current scope.
//  4. The top of the stack (if any) is the parent.
func buildScopeParentIDs(scopes []*pipeline.ScopeInfo) {
	if len(scopes) == 0 {
		return
	}

	// Sort by StartLine ascending, EndLine descending (wider first)
	sort.Slice(scopes, func(i, j int) bool {
		if scopes[i].StartLine != scopes[j].StartLine {
			return scopes[i].StartLine < scopes[j].StartLine
		}
		return scopes[i].EndLine > scopes[j].EndLine
	})

	// Stack of scope indices
	stack := make([]int, 0, len(scopes))

	for i, s := range scopes {
		// Pop scopes that end before this one starts
		for len(stack) > 0 {
			top := scopes[stack[len(stack)-1]]
			if top.EndLine < s.StartLine {
				stack = stack[:len(stack)-1]
			} else {
				break
			}
		}

		// The top of the stack is the parent (if any)
		if len(stack) > 0 {
			s.ParentID = scopes[stack[len(stack)-1]].ID
		}

		stack = append(stack, i)
	}
}
