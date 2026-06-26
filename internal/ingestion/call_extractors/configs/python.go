package configs

import (
	"github.com/mengshi02/codetrip/internal/ingestion/core"
)

// PythonCallConfig is the call extraction configuration for Python.
var PythonCallConfig = core.CallExtractionConfig{
	Language: core.LangPython,
}