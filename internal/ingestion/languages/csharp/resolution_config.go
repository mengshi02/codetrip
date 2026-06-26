// Package csharp — C# resolution config loader.
// Loads .csproj files to extract RootNamespace and ProjectDir for
// import-resolution configuration. The orchestrator calls this once
// per workspace pass via LoadResolutionConfig.
// Ported from TS languages/csharp/resolution-config.ts.
package csharp

import (
	"github.com/mengshi02/codetrip/internal/ingestion/core"
)

// CsharpResolutionConfig holds the C#-specific resolution configuration
// parsed from .csproj files. Used by ResolveCsharpImportTarget to
// strip the RootNamespace prefix and match project-relative paths.
type CsharpResolutionConfig struct {
	RootNamespace string   // from <RootNamespace> in .csproj or assembly name
	ProjectDir    string   // directory containing the .csproj file
	AllFilePaths  []string // workspace file paths for matching
}

// LoadCsharpResolutionConfig loads C# project resolution configuration
// from .csproj files in the repository. Extracts RootNamespace and
// ProjectDir for import path resolution.
//
// This function is called by the ScopeResolver.LoadResolutionConfig()
// hook at pipeline start, once per workspace.
//
// Mirrors TS loadCsharpResolutionConfig(repoPath).
// TODO: full implementation — currently uses core.LoadLanguageConfigs.
func LoadCsharpResolutionConfig(repoPath string) *CsharpResolutionConfig {
	// Delegate to core config loader for now.
	configs := core.LoadLanguageConfigs(repoPath)
	for _, c := range configs {
		if c.Language == "csharp" && c.CSharpProject != nil {
			return &CsharpResolutionConfig{
				RootNamespace: c.CSharpProject.RootNamespace,
				ProjectDir:    c.CSharpProject.ProjectDir,
			}
		}
	}
	return nil
}