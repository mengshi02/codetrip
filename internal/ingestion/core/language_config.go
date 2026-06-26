// Language Config — language-specific configuration for import resolution.
//
// Mirrors TS language-config.ts (241 lines) simplified for codetrip:
//   - Only 9 core languages (no PHP/Swift/Dart/Vue/Kotlin/Ruby)
//   - Go module config from go.mod (no async, direct os.ReadFile)
//   - C# root namespace from .csproj (no async)
//   - TypeScript path aliases from tsconfig.json (no async)
//   - Python: no config needed (imports are relative)
//   - C/C++: no config needed (includes are relative or via -I flags)
//   - Rust: no config needed (Cargo.toml module path handled by tree-sitter)
//
// All config loading is synchronous (no async) since codetrip has no worker pool
// and the config is loaded once at pipeline start.

package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ─── Language-specific config types ──────────────────────────

// TsconfigPaths holds TypeScript path alias config parsed from tsconfig.json.
type TsconfigPaths struct {
	Aliases map[string]string // alias prefix -> target prefix (e.g., "@" -> "src/")
	BaseURL string            // base URL for path resolution
}

// GoModuleConfig holds Go module config parsed from go.mod.
type GoModuleConfig struct {
	ModulePath string // e.g., "github.com/user/repo"
}

// CSharpProjectConfig holds C# project config parsed from .csproj files.
type CSharpProjectConfig struct {
	RootNamespace string // from <RootNamespace> or assembly name
	ProjectDir    string // directory containing the .csproj file
}

// LanguageConfig holds the per-language configuration needed for import resolution.
type LanguageConfig struct {
	// Language identifier
	Language string
	// TypeScript path aliases (nil if not TS)
	TsconfigPaths *TsconfigPaths
	// Go module path (nil if not Go)
	GoModule *GoModuleConfig
	// C# root namespace (nil if not C#)
	CSharpProject *CSharpProjectConfig
}

// ─── Config loading ──────────────────────────────────────────

// LoadLanguageConfigs loads language-specific configurations for all 9 core languages.
// Config is loaded synchronously at pipeline start.
func LoadLanguageConfigs(repoRoot string) []LanguageConfig {
	configs := []LanguageConfig{
		{
			Language:      "typescript",
			TsconfigPaths: loadTsconfigPaths(repoRoot),
			GoModule:      nil,
			CSharpProject: nil,
		},
		{
			Language:      "javascript",
			TsconfigPaths: loadTsconfigPaths(repoRoot), // JS can also use tsconfig
			GoModule:      nil,
			CSharpProject: nil,
		},
		{
			Language:      "python",
			TsconfigPaths: nil,
			GoModule:      nil,
			CSharpProject: nil,
		},
		{
			Language:      "java",
			TsconfigPaths: nil,
			GoModule:      nil,
			CSharpProject: nil,
		},
		{
			Language:      "c",
			TsconfigPaths: nil,
			GoModule:      nil,
			CSharpProject: nil,
		},
		{
			Language:      "cpp",
			TsconfigPaths: nil,
			GoModule:      nil,
			CSharpProject: nil,
		},
		{
			Language:      "csharp",
			TsconfigPaths: nil,
			GoModule:      nil,
			CSharpProject: loadCSharpProjectConfig(repoRoot),
		},
		{
			Language:      "go",
			TsconfigPaths: nil,
			GoModule:      loadGoModuleConfig(repoRoot),
			CSharpProject: nil,
		},
		{
			Language:      "rust",
			TsconfigPaths: nil,
			GoModule:      nil,
			CSharpProject: nil,
		},
	}
	return configs
}

// ─── tsconfig.json loader ────────────────────────────────────

// loadTsconfigPaths parses tsconfig.json to extract path aliases.
// Tries tsconfig.json, tsconfig.app.json, tsconfig.base.json in order.
func loadTsconfigPaths(repoRoot string) *TsconfigPaths {
	candidates := []string{"tsconfig.json", "tsconfig.app.json", "tsconfig.base.json"}

	for _, filename := range candidates {
		tsconfigPath := filepath.Join(repoRoot, filename)
		data, err := os.ReadFile(tsconfigPath)
		if err != nil {
			continue // file doesn't exist
		}

		// Strip JSON comments for robustness
		stripped := stripJSONComments(string(data))
		var tsconfig map[string]any
		if err := json.Unmarshal([]byte(stripped), &tsconfig); err != nil {
			continue // not valid JSON
		}

		compilerOptions, ok := tsconfig["compilerOptions"].(map[string]any)
		if !ok {
			continue
		}
		paths, ok := compilerOptions["paths"].(map[string]any)
		if !ok {
			continue
		}

		baseURL, _ := compilerOptions["baseUrl"].(string)
		if baseURL == "" {
			baseURL = "."
		}

		aliases := make(map[string]string)
		for pattern, targets := range paths {
			targetList, ok := targets.([]any)
			if !ok || len(targetList) == 0 {
				continue
			}
			target, ok := targetList[0].(string)
			if !ok {
				continue
			}
			// Convert glob patterns: "@/*" -> "@/", "src/*" -> "src/"
			aliasPrefix := pattern
			if strings.HasSuffix(pattern, "/*") {
				aliasPrefix = pattern[:len(pattern)-1]
			}
			targetPrefix := target
			if strings.HasSuffix(target, "/*") {
				targetPrefix = target[:len(target)-1]
			}
			aliases[aliasPrefix] = targetPrefix
		}

		if len(aliases) > 0 {
			return &TsconfigPaths{Aliases: aliases, BaseURL: baseURL}
		}
	}
	return nil
}

// stripJSONComments removes single-line (// ...) and multi-line (/* ... */) comments
// from JSON content for robust parsing.
func stripJSONComments(content string) string {
	// Single-line comments: // ...
	content = regexp.MustCompile(`//.*`).ReplaceAllString(content, "")
	// Multi-line comments: /* ... */
	content = regexp.MustCompile(`/\*[\s\S]*?\*/`).ReplaceAllString(content, "")
	return content
}

// ─── go.mod loader ──────────────────────────────────────────

// loadGoModuleConfig parses go.mod to extract the module path.
func loadGoModuleConfig(repoRoot string) *GoModuleConfig {
	goModPath := filepath.Join(repoRoot, "go.mod")
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return nil
	}
	re := regexp.MustCompile(`^module\s+(\S+)`)
	matches := re.FindAllSubmatch(data, -1)
	if len(matches) == 0 {
		return nil
	}
	return &GoModuleConfig{ModulePath: string(matches[0][1])}
}

// ─── .csproj loader ──────────────────────────────────────────

// loadCSharpProjectConfig parses .csproj files to extract the root namespace.
func loadCSharpProjectConfig(repoRoot string) *CSharpProjectConfig {
	// Find .csproj files in repo root
	var csprojPath string
	err := filepath.WalkDir(repoRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			// Only walk top-level directories, not recursively
			if path != repoRoot {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), ".csproj") {
			csprojPath = path
			return nil // found one, stop walking
		}
		return nil
	})

	if csprojPath == "" {
		return nil
	}
	data, err := os.ReadFile(csprojPath)
	if err != nil {
		return nil
	}
	re := regexp.MustCompile(`<RootNamespace>(.*?)</RootNamespace>`)
	matches := re.FindAllSubmatch(data, 1)
	if len(matches) > 0 {
		return &CSharpProjectConfig{
			RootNamespace: string(matches[0][1]),
			ProjectDir:    filepath.Dir(csprojPath),
		}
	}
	// Fallback: use project directory name as root namespace
	projectDir := filepath.Dir(csprojPath)
	dirName := filepath.Base(projectDir)
	return &CSharpProjectConfig{
		RootNamespace: dirName,
		ProjectDir:    projectDir,
	}
}
