// Package importresolvers provides import path resolution for all supported languages.
// Ported from GitNexus import-resolvers/ (TypeScript).
//
// Each language declares an ordered list of resolution strategies.
// The factory chains them: first non-null result wins.
package importresolvers

import (
	"github.com/mengshi02/codetrip/internal/ingestion/core"
)

// ImportResult is the result of resolving an import via language-specific dispatch.
//   - Kind "files": resolved to one or more files -> add to ImportMap
//   - Kind "package": resolved to a directory -> add graph edges + store dirSuffix in PackageMap
//   - nil: no resolution (external dependency, etc.)
type ImportResult struct {
	Kind      string   // "files" or "package"
	Files     []string // resolved file paths
	DirSuffix string   // for Kind=="package": directory suffix
}

// ImportConfigs holds bundled language-specific configs loaded once per ingestion run.
type ImportConfigs struct {
	TsconfigPaths      *TsconfigPaths
	GoModule           *GoModuleConfig
	CSharpConfigs      []CSharpProjectConfig
	CSharpNamespaces   *CSharpNamespaceEvidence
	ComposerConfig     *ComposerConfig
	SwiftPackageConfig *SwiftPackageConfig
}

// ImportResolutionContext holds pre-built lookup structures for import resolution.
// Build once, reuse across chunks.
type ImportResolutionContext struct {
	AllFilePaths      map[string]bool
	AllFileList       []string
	NormalizedFileList []string
	Index             *SuffixIndex
	ResolveCache      map[string]*string
}

// ResolveCtx is the full context for import resolution: file lookups + language configs.
type ResolveCtx struct {
	ImportResolutionContext
	Configs ImportConfigs
}

// ImportResolverFn is the per-language import resolver function signature.
type ImportResolverFn func(rawImportPath string, filePath string, ctx *ResolveCtx) *ImportResult

// ImportResolverStrategy is a single import resolution strategy — one step in a composable chain.
// Same signature as ImportResolverFn. Returns nil to let the next strategy try.
type ImportResolverStrategy func(rawImportPath string, filePath string, ctx *ResolveCtx) *ImportResult

// ImportResolutionConfig is the declarative config for composable import resolution.
// Mirrors the MethodExtractionConfig / CallExtractionConfig pattern.
type ImportResolutionConfig struct {
	// Language is documentation-only metadata identifying which language this config serves.
	Language   core.SupportedLanguage
	Strategies []ImportResolverStrategy
}

// ---- Language config types (ported from language-config.ts) ----

// TsconfigPaths holds TypeScript path alias configuration.
type TsconfigPaths struct {
	BaseUrl  string
	Aliases  [][2]string // [aliasPrefix, targetPrefix]
}

// GoModuleConfig holds Go module configuration.
type GoModuleConfig struct {
	ModulePath string
	RootDir    string
}

// CSharpProjectConfig holds C# project configuration.
type CSharpProjectConfig struct {
	ProjectDir    string
	RootNamespace string
}

// CSharpNamespaceEvidence holds in-repo namespace evidence for C# suffix-fallback resolution.
type CSharpNamespaceEvidence struct {
	Namespaces map[string]bool
}

// ComposerConfig holds PHP Composer configuration (kept for interface compat).
type ComposerConfig struct {
	Autoload map[string]string
}

// SwiftPackageConfig holds Swift package configuration.
type SwiftPackageConfig struct {
	Name string
	Path string
}