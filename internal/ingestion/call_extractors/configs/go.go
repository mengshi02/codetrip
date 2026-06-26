package configs

import (
	"github.com/mengshi02/codetrip/internal/ingestion/core"
)

// GoCallConfig is the call extraction configuration for Go.
var GoCallConfig = core.CallExtractionConfig{
	Language: core.LangGo,
}