package configs

import (
	"github.com/mengshi02/codetrip/internal/ingestion/core"
)

// CSharpCallConfig is the call extraction configuration for C#.
var CSharpCallConfig = core.CallExtractionConfig{
	Language:                core.LangCSharp,
	TypeAsReceiverHeuristic: true,
}