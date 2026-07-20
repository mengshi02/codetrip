package ingest

// Language-specific config types and loaders.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// TsconfigPaths holds TypeScript path alias configuration.
type TsconfigPaths struct {
	Aliases map[string]string // alias prefix -> target prefix (e.g., "@/" -> "src/")
	BaseURL string
}

// GoModuleConfig holds Go module configuration.
type GoModuleConfig struct {
	ModulePath string // e.g., "github.com/user/repo"
}

// ComposerConfig holds PHP Composer PSR-4 autoload configuration.
type ComposerConfig struct {
	PSR4 map[string]string // namespace prefix -> directory (e.g., "App\" -> "app/")
}

// CSharpProjectConfig holds C# project configuration.
type CSharpProjectConfig struct {
	RootNamespace string
	ProjectDir    string
}

// SwiftPackageConfig holds Swift Package Manager configuration.
type SwiftPackageConfig struct {
	Targets map[string]string // target name -> source directory path
}

// LanguageConfigs bundles all language-specific configs loaded once per ingestion run.
type LanguageConfigs struct {
	TsconfigPaths      *TsconfigPaths
	GoModule           *GoModuleConfig
	ComposerConfig     *ComposerConfig
	SwiftPackageConfig *SwiftPackageConfig
	CsharpConfigs      []CSharpProjectConfig
}

// LoadTsconfigPaths parses tsconfig.json to extract path aliases.
// Tries tsconfig.json, tsconfig.app.json, tsconfig.base.json in order.
func LoadTsconfigPaths(repoRoot string) *TsconfigPaths {
	candidates := []string{"tsconfig.json", "tsconfig.app.json", "tsconfig.base.json"}

	for _, filename := range candidates {
		tsconfigPath := filepath.Join(repoRoot, filename)
		raw, err := os.ReadFile(tsconfigPath)
		if err != nil {
			continue
		}
		// Strip JSON comments
		stripped := stripJSONComments(string(raw))
		var tsconfig map[string]interface{}
		if err := json.Unmarshal([]byte(stripped), &tsconfig); err != nil {
			continue
		}
		compilerOptions, ok := tsconfig["compilerOptions"].(map[string]interface{})
		if !ok {
			continue
		}
		pathsRaw, ok := compilerOptions["paths"]
		if !ok {
			continue
		}
		pathsMap, ok := pathsRaw.(map[string]interface{})
		if !ok {
			continue
		}

		baseURL := "."
		if bu, ok := compilerOptions["baseUrl"].(string); ok {
			baseURL = bu
		}

		aliases := make(map[string]string)
		for pattern, targets := range pathsMap {
			targetsList, ok := targets.([]interface{})
			if !ok || len(targetsList) == 0 {
				continue
			}
			target, ok := targetsList[0].(string)
			if !ok {
				continue
			}
			aliasPrefix := pattern
			if strings.HasSuffix(aliasPrefix, "/*") {
				aliasPrefix = aliasPrefix[:len(aliasPrefix)-1]
			}
			targetPrefix := target
			if strings.HasSuffix(targetPrefix, "/*") {
				targetPrefix = targetPrefix[:len(targetPrefix)-1]
			}
			aliases[aliasPrefix] = targetPrefix
		}

		if len(aliases) > 0 {
			return &TsconfigPaths{Aliases: aliases, BaseURL: baseURL}
		}
	}
	return nil
}

var goModuleRegex = regexp.MustCompile(`(?m)^module\s+(\S+)`)

// LoadGoModulePath parses go.mod to extract module path.
func LoadGoModulePath(repoRoot string) *GoModuleConfig {
	goModPath := filepath.Join(repoRoot, "go.mod")
	content, err := os.ReadFile(goModPath)
	if err != nil {
		return nil
	}
	match := goModuleRegex.FindSubmatch(content)
	if len(match) >= 2 {
		return &GoModuleConfig{ModulePath: string(match[1])}
	}
	return nil
}

// LoadComposerConfig parses composer.json to extract PSR-4 autoload mappings.
func LoadComposerConfig(repoRoot string) *ComposerConfig {
	composerPath := filepath.Join(repoRoot, "composer.json")
	raw, err := os.ReadFile(composerPath)
	if err != nil {
		return nil
	}
	var composer map[string]interface{}
	if err := json.Unmarshal(raw, &composer); err != nil {
		return nil
	}

	psr4 := make(map[string]string)

	// Merge autoload and autoload-dev psr-4
	for _, key := range []string{"autoload", "autoload-dev"} {
		autoload, ok := composer[key].(map[string]interface{})
		if !ok {
			continue
		}
		psr4Raw, ok := autoload["psr-4"].(map[string]interface{})
		if !ok {
			continue
		}
		for ns, dir := range psr4Raw {
			nsStr := strings.TrimRight(ns, `\`)
			dirStr, ok := dir.(string)
			if !ok {
				continue
			}
			dirStr = strings.ReplaceAll(dirStr, `\`, "/")
			dirStr = strings.TrimRight(dirStr, "/")
			psr4[nsStr] = dirStr
		}
	}

	if len(psr4) > 0 {
		return &ComposerConfig{PSR4: psr4}
	}
	return nil
}

// LoadCSharpProjectConfigs scans the repo root for .csproj files and extracts RootNamespace.
func LoadCSharpProjectConfigs(repoRoot string) []CSharpProjectConfig {
	var configs []CSharpProjectConfig
	maxDepth := 5
	maxDirs := 100
	dirsScanned := 0

	type queueItem struct {
		dir   string
		depth int
	}
	queue := []queueItem{{dir: repoRoot, depth: 0}}

	for len(queue) > 0 && dirsScanned < maxDirs {
		item := queue[0]
		queue = queue[1:]
		dirsScanned++

		entries, err := os.ReadDir(item.dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() && item.depth < maxDepth {
				name := entry.Name()
				if name == "node_modules" || name == ".git" || name == "bin" || name == "obj" {
					continue
				}
				queue = append(queue, queueItem{dir: filepath.Join(item.dir, name), depth: item.depth + 1})
			}
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".csproj") {
				csprojPath := filepath.Join(item.dir, entry.Name())
				content, err := os.ReadFile(csprojPath)
				if err != nil {
					continue
				}
				rootNamespace := strings.TrimSuffix(entry.Name(), ".csproj")
				if nsMatch := regexp.MustCompile(`<RootNamespace>\s*([^<]+)\s*</RootNamespace>`).FindSubmatch(content); len(nsMatch) >= 2 {
					rootNamespace = strings.TrimSpace(string(nsMatch[1]))
				}
				relDir, _ := filepath.Rel(repoRoot, item.dir)
				projectDir := strings.ReplaceAll(relDir, `\`, "/")
				configs = append(configs, CSharpProjectConfig{
					RootNamespace: rootNamespace,
					ProjectDir:    projectDir,
				})
			}
		}
	}
	return configs
}

// LoadSwiftPackageConfig scans for Swift Package Manager target directories.
func LoadSwiftPackageConfig(repoRoot string) *SwiftPackageConfig {
	targets := make(map[string]string)
	sourceDirs := []string{"Sources", "Package/Sources", "src"}

	for _, sourceDir := range sourceDirs {
		fullPath := filepath.Join(repoRoot, sourceDir)
		entries, err := os.ReadDir(fullPath)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				targets[entry.Name()] = sourceDir + "/" + entry.Name()
			}
		}
	}

	if len(targets) > 0 {
		return &SwiftPackageConfig{Targets: targets}
	}
	return nil
}

// LoadAllLanguageConfigs loads all language configs for a repository.
func LoadAllLanguageConfigs(repoRoot string) *LanguageConfigs {
	return &LanguageConfigs{
		TsconfigPaths:      LoadTsconfigPaths(repoRoot),
		GoModule:           LoadGoModulePath(repoRoot),
		ComposerConfig:     LoadComposerConfig(repoRoot),
		SwiftPackageConfig: LoadSwiftPackageConfig(repoRoot),
		CsharpConfigs:      LoadCSharpProjectConfigs(repoRoot),
	}
}

// stripJSONComments removes // and /* */ style comments from JSON-like content.
func stripJSONComments(s string) string {
	// Remove single-line comments
	s = regexp.MustCompile(`//.*$`).ReplaceAllString(s, "")
	// Remove multi-line comments
	s = regexp.MustCompile(`/\*[\s\S]*?\*/`).ReplaceAllString(s, "")
	return s
}
