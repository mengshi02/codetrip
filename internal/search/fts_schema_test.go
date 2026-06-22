package search

import (
	"testing"

	"github.com/mengshi02/codetrip/internal/graph"
)

func TestNewSearchDocument(t *testing.T) {
	node := graph.NewNode("testrepo", graph.LabelFunction, "myFunc")
	node.FilePath = "pkg/handler.go"
	node.Props = graph.NodePropsFromMap(map[string]any{
		"startLine": 10,
		"endLine":   25,
	})

	doc := NewSearchDocument(node)
	if doc == nil {
		t.Fatal("document is nil")
	}
}

func TestTokenize(t *testing.T) {
	tests := []struct {
		input string
		want  int // minimum expected tokens
	}{
		{"helloWorld", 1}, // lowercase: helloworld → 1 token
		{"snake_case_name", 1}, // underscore kept: snake_case_name → 1 token
		{"HTTPRequest", 1}, // lowercase: httprequest → 1 token
		{"simple", 1},
		{"a", 0}, // stop word or too short
		{"the", 0},
		{"getUserByID", 1}, // lowercase: getuserbyid → 1 token
		{"hello world", 2}, // space-separated
		{"foo-bar", 2}, // dash splits
	}

	for _, tt := range tests {
		tokens := tokenize(tt.input)
		if len(tokens) < tt.want {
			t.Errorf("tokenize(%q) = %v, want at least %d tokens", tt.input, tokens, tt.want)
		}
	}
}

func TestPrepareSearchText(t *testing.T) {
	result := prepareSearchText("getUserByID")
	if result == "" {
		t.Error("expected non-empty result")
	}
	// Should contain split tokens
	if len(result) < 5 {
		t.Errorf("prepareSearchText too short: %q", result)
	}
}

func TestSplitCamelCase(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"helloWorld", []string{"hello", "World"}},
		{"HTTPRequest", []string{"H", "T", "T", "P", "Request"}}, // consecutive uppercase split individually
		{"simple", []string{"simple"}},
		{"ABC", []string{"A", "B", "C"}},
	}

	for _, tt := range tests {
		got := splitCamelCase(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("splitCamelCase(%q) = %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i, w := range got {
			if w != tt.want[i] {
				t.Errorf("splitCamelCase(%q)[%d] = %q, want %q", tt.input, i, w, tt.want[i])
			}
		}
	}
}

func TestIsStopWord(t *testing.T) {
	if !isStopWord("the") {
		t.Error("'the' should be a stop word")
	}
	if !isStopWord("func") {
		t.Error("'func' should be a stop word")
	}
	if isStopWord("function") {
		t.Error("'function' should not be a stop word")
	}
}

func TestIsAlphaNumeric(t *testing.T) {
	if !isAlphaNumeric('a') || !isAlphaNumeric('Z') || !isAlphaNumeric('5') || !isAlphaNumeric('_') {
		t.Error("expected alphanumeric chars to return true")
	}
	if isAlphaNumeric('-') || isAlphaNumeric('.') || isAlphaNumeric(' ') {
		t.Error("expected non-alphanumeric chars to return false")
	}
}

func TestFieldConstants(t *testing.T) {
	if FieldNodeID != "nodeID" {
		t.Errorf("FieldNodeID = %q, want nodeID", FieldNodeID)
	}
	if FieldName != "name" {
		t.Errorf("FieldName = %q, want name", FieldName)
	}
	if FieldContent != "content" {
		t.Errorf("FieldContent = %q, want content", FieldContent)
	}
}