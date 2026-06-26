package configs

import (
	"github.com/mengshi02/codetrip/internal/ingestion/core"
)

// PythonClassConfig extracts class definitions from Python source code.
// Python only has class_definition; nesting is handled via ancestor scoping.
var PythonClassConfig = core.ClassExtractionConfig{
	Language:               core.LangPython,
	TypeDeclarationNodes:   []string{"class_definition"},
	AncestorScopeNodeTypes: []string{"class_definition"},
}