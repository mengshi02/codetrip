package configs

import (
	"github.com/mengshi02/codetrip/internal/ingestion/core"
)

// RustCallConfig is the call extraction configuration for Rust.
var RustCallConfig = core.CallExtractionConfig{
	Language: core.LangRust,
}