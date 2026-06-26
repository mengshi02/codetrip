package utils

import (
	"os"
	"strings"
)

// IsDev reports whether NODE_ENV is set to "development".
var IsDev = os.Getenv("NODE_ENV") == "development"

// ParseTruthyEnv interprets an environment variable value as a boolean.
// Returns false for empty/missing values.
// Truthy values: "1", "true", "yes" (case-insensitive, whitespace-trimmed).
func ParseTruthyEnv(raw string) bool {
	if raw == "" {
		return false
	}
	value := strings.TrimSpace(strings.ToLower(raw))
	return value == "1" || value == "true" || value == "yes"
}

// IsSemanticModelValidatorEnabled reports whether the semantic model validator
// should run. Disabled by VALIDATE_SEMANTIC_MODEL=0; enabled in dev mode
// or when VALIDATE_SEMANTIC_MODEL=1.
func IsSemanticModelValidatorEnabled() bool {
	if os.Getenv("VALIDATE_SEMANTIC_MODEL") == "0" {
		return false
	}
	return IsDev || ParseTruthyEnv(os.Getenv("VALIDATE_SEMANTIC_MODEL"))
}