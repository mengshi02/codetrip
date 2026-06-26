package scope_resolution

import (
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// ScopeResolverRegistry holds all registered ScopeResolver instances.
// Mirrors TS scope-resolution/pipeline/registry.ts SCOPE_RESOLVERS map.
//
// Adding a language is two steps:
//  1. Implement ScopeResolver in languages/<lang>/scope_resolver.go
//  2. Register it here via Register()
//
// The scopeResolution phase iterates the registry directly — every
// registered resolver runs automatically.
type ScopeResolverRegistry struct {
	resolvers map[shared.SupportedLanguage]ScopeResolver
}

// NewScopeResolverRegistry creates an empty registry.
func NewScopeResolverRegistry() *ScopeResolverRegistry {
	return &ScopeResolverRegistry{
		resolvers: make(map[shared.SupportedLanguage]ScopeResolver),
	}
}

// Register adds a ScopeResolver to the registry.
func (r *ScopeResolverRegistry) Register(resolver ScopeResolver) {
	r.resolvers[resolver.Language()] = resolver
}

// Get returns the ScopeResolver for the given language, or nil.
func (r *ScopeResolverRegistry) Get(lang shared.SupportedLanguage) ScopeResolver {
	return r.resolvers[lang]
}

// All returns all registered resolvers.
func (r *ScopeResolverRegistry) All() map[shared.SupportedLanguage]ScopeResolver {
	return r.resolvers
}

// Languages returns the set of registered languages.
func (r *ScopeResolverRegistry) Languages() []shared.SupportedLanguage {
	langs := make([]shared.SupportedLanguage, 0, len(r.resolvers))
	for lang := range r.resolvers {
		langs = append(langs, lang)
	}
	return langs
}