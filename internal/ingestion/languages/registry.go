// Package languages — registry of all language providers and scope resolvers.
//
// This package serves as the central registry for the 9 core languages,
// mapping SupportedLanguage values to their LanguageProvider and ScopeResolver
// implementations.
//
// Ported from TS languages/index.ts.
package languages

import (
	"github.com/mengshi02/codetrip/internal/ingestion/core"
	"github.com/mengshi02/codetrip/internal/ingestion/languages/c"
	"github.com/mengshi02/codetrip/internal/ingestion/languages/cpp"
	"github.com/mengshi02/codetrip/internal/ingestion/languages/csharp"
	"github.com/mengshi02/codetrip/internal/ingestion/languages/golang"
	"github.com/mengshi02/codetrip/internal/ingestion/languages/java"
	"github.com/mengshi02/codetrip/internal/ingestion/languages/javascript"
	"github.com/mengshi02/codetrip/internal/ingestion/languages/python"
	"github.com/mengshi02/codetrip/internal/ingestion/languages/rust"
	"github.com/mengshi02/codetrip/internal/ingestion/languages/typescript"
	"github.com/mengshi02/codetrip/internal/ingestion/scope_resolution"
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// ProviderRegistry maps each SupportedLanguage to its LanguageProvider.
var ProviderRegistry map[shared.SupportedLanguage]core.LanguageProvider

// ResolverRegistry maps each SupportedLanguage to its ScopeResolver.
var ResolverRegistry map[shared.SupportedLanguage]scope_resolution.ScopeResolver

func init() {
	ProviderRegistry = map[shared.SupportedLanguage]core.LanguageProvider{
		shared.SupportedLanguageC:          c.CProvider(),
		shared.SupportedLanguageCpp:        cpp.CppProvider(),
		shared.SupportedLanguageCSharp:     csharp.CSharpProvider(),
		shared.SupportedLanguageGo:         golang.GoProvider(),
		shared.SupportedLanguageJava:       java.JavaProvider(),
		shared.SupportedLanguageJavaScript: javascript.JavaScriptProvider(),
		shared.SupportedLanguagePython:     python.PythonProvider(),
		shared.SupportedLanguageRust:       rust.RustProvider(),
		shared.SupportedLanguageTypeScript: typescript.TypeScriptProvider(),
	}

	ResolverRegistry = map[shared.SupportedLanguage]scope_resolution.ScopeResolver{
		shared.SupportedLanguageC:          c.CScopeResolver,
		shared.SupportedLanguageCpp:        cpp.CppScopeResolver,
		shared.SupportedLanguageCSharp:     csharp.CSharpScopeResolver,
		shared.SupportedLanguageGo:         golang.GoScopeResolver,
		shared.SupportedLanguageJava:       java.JavaScopeResolver,
		shared.SupportedLanguageJavaScript: javascript.JavaScriptScopeResolver,
		shared.SupportedLanguagePython:     python.PythonScopeResolver,
		shared.SupportedLanguageRust:       rust.RustScopeResolver,
		shared.SupportedLanguageTypeScript: typescript.TypeScriptScopeResolver,
	}
}

// GetProvider returns the LanguageProvider for the given language, or nil.
func GetProvider(lang shared.SupportedLanguage) core.LanguageProvider {
	return ProviderRegistry[lang]
}

// GetResolver returns the ScopeResolver for the given language, or nil.
func GetResolver(lang shared.SupportedLanguage) scope_resolution.ScopeResolver {
	return ResolverRegistry[lang]
}