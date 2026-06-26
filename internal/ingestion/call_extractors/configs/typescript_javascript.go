package configs

import (
	"github.com/mengshi02/codetrip/internal/ingestion/core"
)

// TypeScriptCallConfig is the call extraction configuration for TypeScript.
var TypeScriptCallConfig = core.CallExtractionConfig{
	Language: core.LangTypeScript,
}

// JavaScriptCallConfig is the call extraction configuration for JavaScript.
var JavaScriptCallConfig = core.CallExtractionConfig{
	Language: core.LangJavaScript,
}