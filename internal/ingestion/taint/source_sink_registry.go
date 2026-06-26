package taint

import "sync"

// Source/sink/sanitizer registry — mirrors TS taint/source-sink-registry.ts.
//
// A keyed registry of SourceSinkSanitizerSpec by language id. Register
// built-in models via the explicit RegisterBuiltinTaintModels() seam in
// typescript_model.go — deliberately not an init side-effect. The taint emit
// path must call it once before the pdg window consumes the registry
// (idempotent; the registry itself stays empty until then).

var (
	registryMu sync.RWMutex
	registry   = make(map[string]SourceSinkSanitizerSpec)
)

// RegisterSourceSinkConfig registers the taint config for a language.
// Last-write-wins: re-registering the same languageId overwrites the previous
// spec.
func RegisterSourceSinkConfig(languageID string, spec SourceSinkSanitizerSpec) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[languageID] = spec
}

// GetSourceSinkConfig looks up the taint config for a language.
// Returns nil when no spec is registered.
func GetSourceSinkConfig(languageID string) *SourceSinkSanitizerSpec {
	registryMu.RLock()
	defer registryMu.RUnlock()
	if spec, ok := registry[languageID]; ok {
		return &spec
	}
	return nil
}

// RegisteredTaintLanguages returns language ids that currently have a
// registered spec.
func RegisteredTaintLanguages() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	languages := make([]string, 0, len(registry))
	for id := range registry {
		languages = append(languages, id)
	}
	return languages
}

// ClearSourceSinkRegistry resets the registry. Primarily for test isolation.
func ClearSourceSinkRegistry() {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry = make(map[string]SourceSinkSanitizerSpec)
}