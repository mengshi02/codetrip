package ingest

import (
	"log"
	"path/filepath"
	"regexp"
	"strings"

	graph "github.com/mengshi02/codetrip/internal/model"
	sitter "github.com/tree-sitter/go-tree-sitter"
)

var javaPackageDeclaration = regexp.MustCompile(`(?m)^\s*package\s+([A-Za-z_$][\w$]*(?:\.[A-Za-z_$][\w$]*)*)\s*;`)
var csharpNamespaceDeclaration = regexp.MustCompile(`(?m)^\s*namespace\s+([A-Za-z_][\w]*(?:\.[A-Za-z_][\w]*)*)\s*[;{]`)
var phpNamespaceDeclaration = regexp.MustCompile(`(?m)^\s*namespace\s+([A-Za-z_][\w]*(?:\\[A-Za-z_][\w]*)*)\s*;`)

// PopulateImplicitPackageVisibility records files that are mutually visible
// without explicit imports. Java compilation units in the same package can
// reference one another directly; treating only explicit imports as visible
// creates dangling heritage targets and unresolved same-package calls.
func PopulateImplicitPackageVisibility(files []FileInput, imports ImportMap) {
	packages := make(map[string][]string)
	for _, file := range files {
		language := GetLanguageFromFilename(file.Path)
		if language == "swift" {
			if target := swiftTargetForPath(file.Path); target != "" {
				packages[language+"\x00"+target] = append(packages[language+"\x00"+target], file.Path)
			}
			continue
		}
		var expression *regexp.Regexp
		switch language {
		case "java":
			expression = javaPackageDeclaration
		case "csharp":
			expression = csharpNamespaceDeclaration
		case "php":
			expression = phpNamespaceDeclaration
		default:
			continue
		}
		match := expression.FindStringSubmatch(file.Content)
		packageName := ""
		if len(match) == 2 {
			packageName = match[1]
		}
		packages[language+"\x00"+packageName] = append(packages[language+"\x00"+packageName], file.Path)
	}
	for _, packageFiles := range packages {
		for _, source := range packageFiles {
			for _, target := range packageFiles {
				if source != target {
					imports.AddImport(source, target)
				}
			}
		}
	}
}

// SwiftPM compiles every Swift file beneath one Sources/<target> or
// Tests/<target> directory as a single module. Symbols in those files are
// visible to one another without explicit imports.
func swiftTargetForPath(path string) string {
	parts := strings.Split(filepath.ToSlash(filepath.Clean(path)), "/")
	for i := 0; i+1 < len(parts); i++ {
		if parts[i] == "Sources" || parts[i] == "Tests" {
			return strings.Join(parts[:i+2], "/")
		}
	}
	return ""
}

// SuffixIndex provides O(1) suffix-based file path lookup.
type SuffixIndex struct {
	exactMap map[string]string
	lowerMap map[string]string
	dirMap   map[string][]string
}

// BuildSuffixIndex builds a SuffixIndex from normalized and original file lists.
func BuildSuffixIndex(nfl, afl []string) *SuffixIndex {
	em := make(map[string]string)
	lm := make(map[string]string)
	dm := make(map[string][]string)
	for i, n := range nfl {
		o := afl[i]
		parts := strings.Split(n, "/")
		for j := len(parts) - 1; j >= 0; j-- {
			sf := strings.Join(parts[j:], "/")
			if _, ok := em[sf]; !ok {
				em[sf] = o
			}
			lo := strings.ToLower(sf)
			if _, ok := lm[lo]; !ok {
				lm[lo] = o
			}
		}
		// Index every directory suffix so package paths can resolve beneath
		// arbitrary source roots such as src/main/kotlin or src/main/java.
		if li := strings.LastIndex(n, "/"); li >= 0 {
			dp := parts[:len(parts)-1]
			fn := parts[len(parts)-1]
			ext := ""
			if di := strings.LastIndex(fn, "."); di >= 0 {
				ext = fn[di:]
			}
			for j := len(dp) - 1; j >= 0; j-- {
				ds := strings.Join(dp[j:], "/")
				key := ds + ":" + ext
				dm[key] = append(dm[key], o)
			}
		} else {
			// Root-level file (no "/" in path) — use "." as dirSuffix
			fn := n
			ext := ""
			if di := strings.LastIndex(fn, "."); di >= 0 {
				ext = fn[di:]
			}
			key := ".:" + ext
			dm[key] = append(dm[key], o)
		}
	}
	return &SuffixIndex{exactMap: em, lowerMap: lm, dirMap: dm}
}

func (si *SuffixIndex) get(s string) string {
	if v, ok := si.exactMap[s]; ok {
		return v
	}
	return ""
}

func (si *SuffixIndex) getInsensitive(s string) string {
	if v, ok := si.lowerMap[strings.ToLower(s)]; ok {
		return v
	}
	return ""
}

func (si *SuffixIndex) GetFilesInDir(dirSuffix, ext string) []string {
	key := dirSuffix + ":" + ext
	if v, ok := si.dirMap[key]; ok {
		return v
	}
	return nil
}

var ResolveExtensions = []string{
	"", ".tsx", ".ts", ".jsx", ".js", "/index.tsx", "/index.ts", "/index.jsx", "/index.js",
	".py", "/__init__.py", ".java", ".kt", ".go", ".cs", ".php", ".rs", ".swift",
	".c", ".cpp", ".h", ".hpp", ".m", ".mm", ".vue", ".svelte",
}

func TryResolveWithExtensions(base string, afp map[string]bool) string {
	for _, ext := range ResolveExtensions {
		if afp[base+ext] {
			return base + ext
		}
	}
	return ""
}

func SuffixResolve(path string, idx *SuffixIndex) string {
	if r := idx.get(path); r != "" {
		return r
	}
	if r := idx.getInsensitive(path); r != "" {
		return r
	}
	return ""
}

type ResolveCtx struct {
	AllFilePaths       map[string]bool
	AllFileList        []string
	NormalizedFileList []string
	Index              *SuffixIndex
	ResolveCache       map[string]string
}

type ImportResolutionContext = ResolveCtx

const resolveCacheCap = 100000

func BuildImportResolutionContext(nfl, afl []string) *ResolveCtx {
	afp := make(map[string]bool, len(afl))
	for _, f := range afl {
		afp[f] = true
	}
	return &ResolveCtx{
		AllFilePaths:       afp,
		AllFileList:        afl,
		NormalizedFileList: nfl,
		Index:              BuildSuffixIndex(nfl, afl),
		ResolveCache:       make(map[string]string),
	}
}

type ImportResultKind int

const (
	ImportResultFiles ImportResultKind = iota
	ImportResultPackage
)

type ImportResult struct {
	Kind      ImportResultKind
	Files     []string
	DirSuffix string
}

var KotlinExtensions = []string{".kt", ".kts"}

func resolveJvmWildcard(raw string, nfl, afl []string, exts []string, idx *SuffixIndex) []string {
	pkg := strings.TrimSuffix(raw, ".*")
	pkg = strings.ReplaceAll(pkg, ".", "/")
	if idx != nil {
		var files []string
		for _, ext := range exts {
			files = append(files, idx.GetFilesInDir(pkg, ext)...)
		}
		return files
	}
	var files []string
	seen := make(map[string]bool)
	for i, n := range nfl {
		packageSuffix := "/" + pkg + "/"
		if pos := strings.Index(n, packageSuffix); pos >= 0 {
			afterPackage := n[pos+len(packageSuffix):]
			if strings.Contains(afterPackage, "/") {
				continue
			}
			for _, ext := range exts {
				if strings.HasSuffix(n, ext) && !seen[afl[i]] {
					files = append(files, afl[i])
					seen[afl[i]] = true
				}
			}
		}
	}
	return files
}

func resolveJvmMemberImport(raw string, nfl, afl []string, exts []string, idx *SuffixIndex) string {
	fullPath := strings.ReplaceAll(raw, ".", "/")
	for _, ext := range exts {
		candidate := fullPath + ext
		if idx != nil {
			if resolved := idx.get(candidate); resolved != "" {
				return resolved
			}
			if resolved := idx.getInsensitive(candidate); resolved != "" {
				return resolved
			}
		}
	}

	segments := strings.Split(raw, ".")
	if len(segments) < 3 {
		return ""
	}
	lastSegment := segments[len(segments)-1]
	isAllCaps := lastSegment != ""
	for _, r := range lastSegment {
		if !((r >= 'A' && r <= 'Z') || r == '_') {
			isAllCaps = false
			break
		}
	}
	startsLower := lastSegment != "" && lastSegment[0] >= 'a' && lastSegment[0] <= 'z'
	if lastSegment != "*" && !startsLower && !isAllCaps {
		return ""
	}
	sp := strings.Join(segments[:len(segments)-1], "/")
	for _, ext := range exts {
		c := sp + ext
		if idx != nil {
			if r := idx.get(c); r != "" {
				return r
			}
			if r := idx.getInsensitive(c); r != "" {
				return r
			}
		}
	}
	return ""
}

func resolveGoPackageDir(raw string, cfg *GoModuleConfig) string {
	if cfg == nil {
		return ""
	}
	if !strings.HasPrefix(raw, cfg.ModulePath) {
		return ""
	}
	rel := strings.TrimPrefix(raw, cfg.ModulePath)
	if rel == "" {
		return "" // self-package import: skip
	}
	rel = strings.TrimPrefix(rel, "/")
	return rel
}

func resolveGoPackage(raw string, cfg *GoModuleConfig, nfl, afl []string) []string {
	ds := resolveGoPackageDir(raw, cfg)
	log.Printf("[import-processor] resolveGoPackage: raw=%q modulePath=%q dirSuffix=%q", raw, cfg.ModulePath, ds)
	if ds == "" {
		return nil
	}
	var files []string
	for i, n := range nfl {
		dir := ""
		if li := strings.LastIndex(n, "/"); li >= 0 {
			dir = n[:li]
		}
		// Root package: ds=="." matches files with no directory component
		if dir == ds || strings.HasPrefix(dir, ds+"/") || (ds == "." && dir == "") {
			if strings.HasSuffix(n, ".go") && !strings.HasSuffix(n, "_test.go") {
				files = append(files, afl[i])
			}
		}
	}
	return files
}

func resolveCSharpImport(raw string, cfgs []CSharpProjectConfig, nfl, afl []string, idx *SuffixIndex) []string {
	namespacePath := strings.ReplaceAll(raw, ".", "/")
	var results []string

	for _, cfg := range cfgs {
		nsPath := strings.ReplaceAll(cfg.RootNamespace, ".", "/")
		var relative string
		if strings.HasPrefix(namespacePath, nsPath+"/") {
			relative = namespacePath[len(nsPath)+1:]
		} else if namespacePath == nsPath {
			// The import IS the root namespace — resolve to all .cs files in project dir
			relative = ""
		} else {
			continue
		}

		dirPrefix := ""
		if cfg.ProjectDir != "" {
			if relative != "" {
				dirPrefix = cfg.ProjectDir + "/" + relative
			} else {
				dirPrefix = cfg.ProjectDir
			}
		} else {
			dirPrefix = relative
		}

		// 1. Try as single file: dirPrefix.cs (e.g., "Models/DlqMessage.cs")
		if relative != "" {
			candidate := dirPrefix + ".cs"
			if idx != nil {
				if r := idx.get(candidate); r != "" {
					return []string{r}
				}
				if r := idx.getInsensitive(candidate); r != "" {
					return []string{r}
				}
			}
			// Also try suffix match on the relative part
			if idx != nil {
				if r := idx.get(relative + ".cs"); r != "" {
					return []string{r}
				}
				if r := idx.getInsensitive(relative + ".cs"); r != "" {
					return []string{r}
				}
			}
		}

		// 2. Try as directory: all .cs files directly inside (namespace import)
		if idx != nil && dirPrefix != "" {
			dirFiles := idx.GetFilesInDir(dirPrefix, ".cs")
			for _, f := range dirFiles {
				normalized := strings.ReplaceAll(f, "\\", "/")
				prefixIdx := strings.Index(normalized, dirPrefix+"/")
				if prefixIdx < 0 {
					continue
				}
				afterDir := normalized[prefixIdx+len(dirPrefix)+1:]
				if !strings.Contains(afterDir, "/") {
					results = append(results, f)
				}
			}
			if len(results) > 0 {
				return results
			}
		}

		// 3. Linear scan fallback for directory matching
		if len(results) == 0 && dirPrefix != "" {
			dirTrail := dirPrefix + "/"
			for i, n := range nfl {
				if !strings.HasSuffix(n, ".cs") {
					continue
				}
				prefixIdx := strings.Index(n, dirTrail)
				if prefixIdx < 0 {
					continue
				}
				afterDir := n[prefixIdx+len(dirTrail):]
				if !strings.Contains(afterDir, "/") {
					results = append(results, afl[i])
				}
			}
			if len(results) > 0 {
				return results
			}
		}
	}

	// Fallback: suffix matching without namespace stripping (single file)
	pathParts := strings.Split(namespacePath, "/")
	var nonEmpty []string
	for _, p := range pathParts {
		if p != "" {
			nonEmpty = append(nonEmpty, p)
		}
	}
	if len(nonEmpty) > 0 {
		// Try last segment as filename
		lastPart := nonEmpty[len(nonEmpty)-1]
		if idx != nil {
			if r := idx.get(lastPart + ".cs"); r != "" {
				return []string{r}
			}
			if r := idx.getInsensitive(lastPart + ".cs"); r != "" {
				return []string{r}
			}
		}
	}

	return nil
}

func resolveCSharpNamespaceDir(raw string, cfgs []CSharpProjectConfig) string {
	namespacePath := strings.ReplaceAll(raw, ".", "/")
	for _, cfg := range cfgs {
		nsPath := strings.ReplaceAll(cfg.RootNamespace, ".", "/")
		var relative string
		if strings.HasPrefix(namespacePath, nsPath+"/") {
			relative = namespacePath[len(nsPath)+1:]
		} else if namespacePath == nsPath {
			relative = ""
		} else {
			continue
		}
		dirPrefix := ""
		if cfg.ProjectDir != "" {
			if relative != "" {
				dirPrefix = cfg.ProjectDir + "/" + relative
			} else {
				dirPrefix = cfg.ProjectDir
			}
		} else {
			dirPrefix = relative
		}
		if dirPrefix == "" {
			continue
		}
		return "/" + dirPrefix + "/"
	}
	return ""
}

func resolvePhpImport(raw string, cfg *ComposerConfig, afp map[string]bool, nfl, afl []string, idx *SuffixIndex) string {
	if cfg != nil && len(cfg.PSR4) > 0 {
		matchedLocalPrefix := false
		for prefix, dir := range cfg.PSR4 {
			if strings.HasPrefix(raw, prefix) {
				matchedLocalPrefix = true
				rel := strings.TrimLeft(strings.TrimPrefix(raw, prefix), `\\/`)
				rel = strings.ReplaceAll(rel, "\\", "/")
				sp := pathNorm(dir + "/" + rel)
				if afp[sp+".php"] {
					return sp + ".php"
				}
				if idx != nil {
					if r := idx.get(sp + ".php"); r != "" {
						return r
					}
				}
			}
		}
		// Composer is authoritative about namespaces owned by this package.
		// Imports outside those prefixes are external; a local-prefix import
		// whose file is absent is unresolved/generated. Do not bind either one
		// to an unrelated repository class by simple filename.
		if !matchedLocalPrefix {
			return ""
		}
		return ""
	}
	sp := strings.ReplaceAll(raw, "\\", "/")
	for _, ext := range []string{".php"} {
		if afp[sp+ext] {
			return sp + ext
		}
		if idx != nil {
			if r := idx.get(sp + ext); r != "" {
				return r
			}
		}
	}
	// Class-name suffix fallback: extract the last segment of the fully-qualified name
	// (e.g. "LaravelLang\Publisher\Plugins\Plugin" → "Plugin") and search for "Plugin.php"
	// prefix does not match any PSR-4 mapping.
	if idx != nil {
		parts := strings.Split(raw, "\\")
		if len(parts) > 0 {
			className := parts[len(parts)-1]
			if className != "" {
				if r := idx.get(className + ".php"); r != "" {
					return r
				}
				if r := idx.getInsensitive(className + ".php"); r != "" {
					return r
				}
			}
		}
	}
	return ""
}

func resolveRustImport(fp string, raw string, afp map[string]bool) string {
	raw = normalizeRustUsePath(raw)
	if raw == "" {
		return ""
	}
	var modulePath string
	switch {
	case strings.HasPrefix(raw, "crate::"):
		modulePath = strings.ReplaceAll(strings.TrimPrefix(raw, "crate::"), "::", "/")
		crateRoot := rustCrateSourceRoot(fp)
		if resolved := tryRustModulePath(filepath.ToSlash(filepath.Join(crateRoot, modulePath)), afp); resolved != "" {
			return resolved
		}
		// Keep the repository-root fallback for unusual layouts where source
		// files are generated beneath a workspace but refer to the root crate.
		if crateRoot != "src" {
			if resolved := tryRustModulePath("src/"+modulePath, afp); resolved != "" {
				return resolved
			}
		}
		return tryRustModulePath(modulePath, afp)
	case strings.HasPrefix(raw, "self::"):
		modulePath = filepath.ToSlash(filepath.Join(filepath.Dir(fp), strings.ReplaceAll(strings.TrimPrefix(raw, "self::"), "::", "/")))
		return tryRustModulePath(modulePath, afp)
	case strings.HasPrefix(raw, "super::"):
		modulePath = filepath.ToSlash(filepath.Join(filepath.Dir(filepath.Dir(fp)), strings.ReplaceAll(strings.TrimPrefix(raw, "super::"), "::", "/")))
		return tryRustModulePath(modulePath, afp)
	case strings.Contains(raw, "::"):
		modulePath = strings.ReplaceAll(raw, "::", "/")
		return tryRustModulePath(modulePath, afp)
	default:
		return ""
	}
}

// normalizeRustUsePath reduces a grouped use declaration to the module that
// makes all bindings visible. For example, crate::process::{Reader, Builder}
// imports both symbols from crate::process, so resolving the containing module
// is sufficient for scoped semantic lookup.
func normalizeRustUsePath(raw string) string {
	raw = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(raw), ";"))
	if grouped := strings.Index(raw, "::{"); grouped >= 0 {
		raw = raw[:grouped]
	}
	return strings.TrimSpace(raw)
}

func rustCrateSourceRoot(fp string) string {
	fp = filepath.ToSlash(filepath.Clean(fp))
	if fp == "src" || strings.HasPrefix(fp, "src/") {
		return "src"
	}
	if marker := strings.LastIndex(fp, "/src/"); marker >= 0 {
		return fp[:marker+len("/src")]
	}
	return "src"
}

func tryRustModulePath(modulePath string, afp map[string]bool) string {
	if afp[modulePath+".rs"] {
		return modulePath + ".rs"
	}
	if afp[modulePath+"/mod.rs"] {
		return modulePath + "/mod.rs"
	}
	if afp[modulePath+"/lib.rs"] {
		return modulePath + "/lib.rs"
	}
	if lastSlash := strings.LastIndex(modulePath, "/"); lastSlash > 0 {
		parentPath := modulePath[:lastSlash]
		if afp[parentPath+".rs"] {
			return parentPath + ".rs"
		}
		if afp[parentPath+"/mod.rs"] {
			return parentPath + "/mod.rs"
		}
	}
	return ""
}

// ─────────────────────────────────────────────────────────────────────────────
// Standard resolver (relative/alias/suffix matching).
// ─────────────────────────────────────────────────────────────────────────────

func resolveImportPath(fp string, raw string, lang string, cfgs *LanguageConfigs, rctx *ResolveCtx) *ImportResult {
	if raw == "" {
		return nil
	}
	// Check cache
	if r, ok := rctx.ResolveCache[fp+"::"+raw]; ok {
		if r == "" {
			return nil
		}
		return &ImportResult{Kind: ImportResultFiles, Files: []string{r}}
	}

	// Step 1: TypeScript path alias rewrite
	if cfgs != nil && cfgs.TsconfigPaths != nil && len(cfgs.TsconfigPaths.Aliases) > 0 {
		for alias, target := range cfgs.TsconfigPaths.Aliases {
			if strings.HasPrefix(raw, alias) {
				rewritten := strings.Replace(raw, alias, target, 1)
				base := rewritten
				if cfgs.TsconfigPaths.BaseURL != "" {
					base = cfgs.TsconfigPaths.BaseURL + "/" + rewritten
				}
				if r := tryResolveIdx(base, rctx); r != "" {
					rctx.ResolveCache[fp+"::"+raw] = r
					return &ImportResult{Kind: ImportResultFiles, Files: []string{r}}
				}
			}
		}
	}

	// Step 2: Rust crate::/self::/super:: resolution
	if strings.HasPrefix(raw, "crate::") || strings.HasPrefix(raw, "self::") || strings.HasPrefix(raw, "super::") {
		if r := resolveRustImport(fp, raw, rctx.AllFilePaths); r != "" {
			rctx.ResolveCache[fp+"::"+raw] = r
			return &ImportResult{Kind: ImportResultFiles, Files: []string{r}}
		}
	}

	// Step 3: Python relative import
	if strings.HasPrefix(raw, ".") {
		if r := resolvePythonRel(fp, raw, rctx); r != "" {
			rctx.ResolveCache[fp+"::"+raw] = r
			return &ImportResult{Kind: ImportResultFiles, Files: []string{r}}
		}
	}

	// Step 4: Generic relative (./ or ../)
	if raw == "." || strings.HasPrefix(raw, "./") || strings.HasPrefix(raw, "../") {
		dir := filepath.Dir(fp)
		clean := pathNorm(dir + "/" + raw)
		resolve := tryResolveIdx
		if lang == "javascript" || lang == "typescript" || lang == "tsx" {
			resolve = tryResolveRelativeExact
		}
		if r := resolve(clean, rctx); r != "" {
			rctx.ResolveCache[fp+"::"+raw] = r
			return &ImportResult{Kind: ImportResultFiles, Files: []string{r}}
		}
		// For JS/TS relative imports, do not reinterpret a missing relative
		// target as an unrelated suffix elsewhere in the repository. The
		// reference resolver leaves such imports unresolved.
		if lang == "javascript" || lang == "typescript" || lang == "tsx" {
			return nil
		}
	}

	// Step 5: Package/absolute resolution (dot-to-slash + suffix matching)
	// C/C++ includes use actual file paths (e.g. "ngx_config.h") — don't convert dots to slashes.
	var sp string
	if lang == "c" || lang == "cpp" || strings.Contains(raw, "/") {
		sp = raw
	} else {
		sp = strings.ReplaceAll(raw, ".", "/")
	}
	if r := tryResolveIdx(sp, rctx); r != "" {
		rctx.ResolveCache[fp+"::"+raw] = r
		return &ImportResult{Kind: ImportResultFiles, Files: []string{r}}
	}

	// Cache miss
	if len(rctx.ResolveCache) < resolveCacheCap {
		rctx.ResolveCache[fp+"::"+raw] = ""
	}
	return nil
}

func tryResolveRelativeExact(base string, rctx *ResolveCtx) string {
	for _, ext := range ResolveExtensions {
		if rctx.AllFilePaths[base+ext] {
			return base + ext
		}
	}
	return ""
}

func tryResolveIdx(base string, rctx *ResolveCtx) string {
	// Try with extensions first
	if r := TryResolveWithExtensions(base, rctx.AllFilePaths); r != "" {
		return r
	}
	// Progressively drop leading path segments. Package roots frequently differ
	// from repository source roots, so the first resolvable suffix wins.
	parts := strings.FieldsFunc(base, func(r rune) bool { return r == '/' })
	for i := 0; i < len(parts); i++ {
		suffix := strings.Join(parts[i:], "/")
		for _, ext := range ResolveExtensions {
			candidate := suffix + ext
			if r := SuffixResolve(candidate, rctx.Index); r != "" {
				return r
			}
		}
	}
	// Try with /index suffixes
	for _, ext := range []string{".ts", ".tsx", ".js", ".jsx"} {
		idx := base + "/index" + ext
		if rctx.AllFilePaths[idx] {
			return idx
		}
		if r := SuffixResolve(idx, rctx.Index); r != "" {
			return r
		}
	}
	return ""
}

func resolvePythonRel(fp string, raw string, rctx *ResolveCtx) string {
	dir := filepath.Dir(fp)
	parts := strings.Split(dir, "/")
	dots := 0
	for _, ch := range raw {
		if ch == '.' {
			dots++
		} else {
			break
		}
	}
	level := dots - 1 // first dot is current dir
	if level < 0 {
		level = 0
	}
	if level > len(parts)-1 {
		level = len(parts) - 1
	}
	base := strings.Join(parts[:len(parts)-level], "/")
	mod := strings.TrimLeft(raw, ".")
	mod = strings.ReplaceAll(mod, ".", "/")
	if mod != "" {
		base = base + "/" + mod
	}
	// Try __init__.py
	if rctx.AllFilePaths[base+"/__init__.py"] {
		return base + "/__init__.py"
	}
	// Try direct module
	if rctx.AllFilePaths[base+".py"] {
		return base + ".py"
	}
	return ""
}

func pathNorm(p string) string {
	p = strings.ReplaceAll(p, "\\", "/")
	parts := strings.Split(p, "/")
	var stack []string
	for _, part := range parts {
		switch part {
		case "..":
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		case ".", "":
			// skip
		default:
			stack = append(stack, part)
		}
	}
	return strings.Join(stack, "/")
}

// ─────────────────────────────────────────────────────────────────────────────
// resolveLanguageImport — language-specific dispatcher.
// ─────────────────────────────────────────────────────────────────────────────

func resolveLanguageImport(fp string, raw string, lang string, cfgs *LanguageConfigs, rctx *ResolveCtx) *ImportResult {
	switch lang {
	case "java":
		if strings.HasSuffix(raw, ".*") {
			if files := resolveJvmWildcard(raw, rctx.NormalizedFileList, rctx.AllFileList, []string{".java"}, rctx.Index); len(files) > 0 {
				return &ImportResult{Kind: ImportResultFiles, Files: files}
			}
		}
		if r := resolveJvmMemberImport(raw, rctx.NormalizedFileList, rctx.AllFileList, []string{".java"}, rctx.Index); r != "" {
			return &ImportResult{Kind: ImportResultFiles, Files: []string{r}}
		}
		if r := resolveJvmSimpleNameFallback(raw, rctx.AllFileList); r != "" {
			return &ImportResult{Kind: ImportResultFiles, Files: []string{r}}
		}
	case "kotlin":
		if strings.HasSuffix(raw, ".*") {
			if files := resolveJvmWildcard(raw, rctx.NormalizedFileList, rctx.AllFileList, KotlinExtensions, rctx.Index); len(files) > 0 {
				return &ImportResult{Kind: ImportResultFiles, Files: files}
			}
			if files := resolveJvmWildcard(raw, rctx.NormalizedFileList, rctx.AllFileList, []string{".java"}, rctx.Index); len(files) > 0 {
				return &ImportResult{Kind: ImportResultFiles, Files: files}
			}
		}
		if r := resolveJvmMemberImport(raw, rctx.NormalizedFileList, rctx.AllFileList, KotlinExtensions, rctx.Index); r != "" {
			return &ImportResult{Kind: ImportResultFiles, Files: []string{r}}
		}
		if r := resolveJvmMemberImport(raw, rctx.NormalizedFileList, rctx.AllFileList, []string{".java"}, rctx.Index); r != "" {
			return &ImportResult{Kind: ImportResultFiles, Files: []string{r}}
		}
	case "go":
		if cfgs != nil && cfgs.GoModule != nil && strings.HasPrefix(raw, cfgs.GoModule.ModulePath) {
			if files := resolveGoPackage(raw, cfgs.GoModule, rctx.NormalizedFileList, rctx.AllFileList); len(files) > 0 {
				ds := resolveGoPackageDir(raw, cfgs.GoModule)
				return &ImportResult{Kind: ImportResultPackage, Files: files, DirSuffix: ds}
			}
		}
	case "csharp":
		if cfgs != nil && len(cfgs.CsharpConfigs) > 0 {
			if files := resolveCSharpImport(raw, cfgs.CsharpConfigs, rctx.NormalizedFileList, rctx.AllFileList, rctx.Index); len(files) > 0 {
				if len(files) > 1 {
					dirSuffix := resolveCSharpNamespaceDir(raw, cfgs.CsharpConfigs)
					if dirSuffix != "" {
						return &ImportResult{Kind: ImportResultPackage, Files: files, DirSuffix: dirSuffix}
					}
				}
				return &ImportResult{Kind: ImportResultFiles, Files: files}
			}
			return nil // C# imports that don't resolve within project are external — don't fall through
		}
	case "php":
		if cfgs != nil {
			if r := resolvePhpImport(raw, cfgs.ComposerConfig, rctx.AllFilePaths, rctx.NormalizedFileList, rctx.AllFileList, rctx.Index); r != "" {
				return &ImportResult{Kind: ImportResultFiles, Files: []string{r}}
			}
		}
	case "swift":
		if cfgs != nil && cfgs.SwiftPackageConfig != nil {
			if targetDir, ok := cfgs.SwiftPackageConfig.Targets[raw]; ok {
				dirPrefix := strings.TrimSuffix(pathNorm(targetDir), "/") + "/"
				files := make([]string, 0)
				for i, normalizedPath := range rctx.NormalizedFileList {
					if strings.HasPrefix(normalizedPath, dirPrefix) && strings.HasSuffix(normalizedPath, ".swift") {
						files = append(files, rctx.AllFileList[i])
					}
				}
				if len(files) > 0 {
					return &ImportResult{Kind: ImportResultFiles, Files: files}
				}
			}
			return nil
		}
	case "rust":
		if r := resolveRustImport(fp, raw, rctx.AllFilePaths); r != "" {
			return &ImportResult{Kind: ImportResultFiles, Files: []string{r}}
		}
	}
	// Fallback: standard resolver
	return resolveImportPath(fp, raw, lang, cfgs, rctx)
}

// resolveJvmSimpleNameFallback mirrors the reference's deterministic fallback
// when a malformed/unmatched qualified import names a class that exists in
// multiple packages. Keep it Java-only; valid package paths resolve above.
func resolveJvmSimpleNameFallback(raw string, files []string) string {
	parts := strings.Split(raw, ".")
	if len(parts) < 2 {
		return ""
	}
	base := parts[len(parts)-1] + ".java"
	result := ""
	for _, file := range files {
		if strings.HasSuffix(filepath.ToSlash(file), "/"+base) {
			result = file
		}
	}
	return result
}

// ─────────────────────────────────────────────────────────────────────────────
// Graph edge helpers.
// ─────────────────────────────────────────────────────────────────────────────

func makeImportRelID(src, tgt string) string {
	return "IMPORTS:" + src + "->" + tgt
}

func applyImportResult(g *graph.KnowledgeGraph, src string, ir *ImportResult, im ImportMap, pm PackageMap, nim NamedImportMap) {
	if ir == nil {
		return
	}
	fileSrc := "File:" + src
	switch ir.Kind {
	case ImportResultFiles:
		for _, tgt := range ir.Files {
			if tgt == src {
				continue
			}
			fileTgt := "File:" + tgt
			im.AddImport(src, tgt)
			g.AddRelationship(&graph.GraphRelationship{
				ID:         makeImportRelID(fileSrc, fileTgt),
				SourceID:   fileSrc,
				TargetID:   fileTgt,
				Type:       graph.RelIMPORTS,
				Confidence: 1.0,
			})
		}
	case ImportResultPackage:
		for _, tgt := range ir.Files {
			if tgt == src {
				continue
			}
			fileTgt := "File:" + tgt
			im.AddImport(src, tgt)
			g.AddRelationship(&graph.GraphRelationship{
				ID:         makeImportRelID(fileSrc, fileTgt),
				SourceID:   fileSrc,
				TargetID:   fileTgt,
				Type:       graph.RelIMPORTS,
				Confidence: 1.0,
			})
		}
		if ir.DirSuffix != "" {
			pm.AddPackage(src, ir.DirSuffix) // store dirSuffix for Tier 2c directory-level matching
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Import query capture and ProcessImports.
// ─────────────────────────────────────────────────────────────────────────────

// ImportQueryCapture holds a single import captured from tree-sitter query.
type ImportQueryCapture struct {
	FilePath     string
	ImportPath   string
	Language     string
	NamedBinding string
	ExportedName string
}

// ProcessImports runs import resolution for all files.
func ProcessImports(
	g *graph.KnowledgeGraph,
	reg *LanguageRegistry,
	files []string,
	langMap map[string]string,
	astCache map[string]*sitter.Tree,
	srcCache map[string][]byte,
	parser *sitter.Parser,
	im ImportMap,
	pm PackageMap,
	nim NamedImportMap,
	cfgs *LanguageConfigs,
	rctx *ResolveCtx,
) {
	captures := extractImportCaptures(reg, files, langMap, astCache, srcCache, parser)
	ProcessImportsFromExtracted(g, captures, im, pm, nim, cfgs, rctx, nil)
}

func extractImportCaptures(
	reg *LanguageRegistry,
	files []string,
	langMap map[string]string,
	astCache map[string]*sitter.Tree,
	srcCache map[string][]byte,
	parser *sitter.Parser,
) []ImportQueryCapture {
	var captures []ImportQueryCapture
	for _, fp := range files {
		lang, ok := langMap[fp]
		if !ok {
			continue
		}
		tree, ok := astCache[fp]
		if !ok {
			continue
		}
		src, ok := srcCache[fp]
		if !ok {
			continue
		}
		l, err := reg.GetLanguage(lang)
		if err != nil {
			continue
		}
		qs := getImportQuery(lang)
		if qs == "" {
			continue
		}
		q, queryErr := sitter.NewQuery(l, qs)
		if queryErr != nil {
			continue
		}
		captureNames := q.CaptureNames()
		qc := sitter.NewQueryCursor()
		matches := qc.Matches(q, tree.RootNode(), src)
		for {
			m := matches.Next()
			if m == nil {
				break
			}
			var impPath, impName, expName string
			for _, c := range m.Captures {
				idx := int(c.Index)
				name := ""
				if idx < len(captureNames) {
					name = captureNames[idx]
				}
				switch name {
				case "import.path":
					impPath = c.Node.Utf8Text(src)
				case "import.name":
					impName = c.Node.Utf8Text(src)
				case "import.exported_name":
					expName = c.Node.Utf8Text(src)
				}
			}
			if impPath == "" {
				impPath = impName
			}
			if impPath == "" {
				continue
			}
			impPath = cleanImportPath(impPath)
			if lang == "kotlin" && impName == "" && !strings.HasSuffix(impPath, ".*") {
				if idx := strings.LastIndex(impPath, "."); idx >= 0 {
					impName = impPath[idx+1:]
				} else {
					impName = impPath
				}
				expName = impName
			}
			captures = append(captures, ImportQueryCapture{
				FilePath:     fp,
				ImportPath:   impPath,
				Language:     lang,
				NamedBinding: impName,
				ExportedName: expName,
			})
		}
		qc.Close()
		q.Close()
	}
	return captures
}

func cleanImportPath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.TrimPrefix(p, "\"")
	p = strings.TrimSuffix(p, "\"")
	p = strings.TrimPrefix(p, "'")
	p = strings.TrimSuffix(p, "'")
	p = strings.TrimPrefix(p, "`")
	p = strings.TrimSuffix(p, "`")
	// C/C++ #include <header.h> — strip angle brackets
	p = strings.TrimPrefix(p, "<")
	p = strings.TrimSuffix(p, ">")
	return p
}

func getImportQuery(lang string) string {
	switch lang {
	case "typescript", "tsx":
		return TypeScriptImportQuery
	case "javascript":
		return JavaScriptImportQuery
	case "python":
		return PythonImportQuery
	case "go":
		return GoImportQuery
	case "java":
		return JavaImportQuery
	case "kotlin":
		return KotlinImportQuery
	case "csharp":
		return CSharpImportQuery
	case "php":
		return PHPImportQuery
	case "rust":
		return RustImportQuery
	case "swift":
		return SwiftImportQuery
	case "c":
		return CImportQuery
	default:
		return ""
	}
}

// ProcessImportsFromExtracted resolves import captures and updates graph/maps.
func ProcessImportsFromExtracted(
	g *graph.KnowledgeGraph,
	captures []ImportQueryCapture,
	im ImportMap,
	pm PackageMap,
	nim NamedImportMap,
	cfgs *LanguageConfigs,
	rctx *ResolveCtx,
	order ImportOrderMap,
) {
	log.Printf("[import-processor] ProcessImportsFromExtracted: %d captures, GoModule=%v", len(captures), cfgs.GoModule)
	for _, c := range captures {
		ir := resolveLanguageImport(c.FilePath, c.ImportPath, c.Language, cfgs, rctx)
		applyImportResult(g, c.FilePath, ir, im, pm, nim)
		if order != nil && ir != nil {
			for _, target := range ir.Files {
				seen := false
				for _, existing := range order[c.FilePath] {
					if existing == target {
						seen = true
						break
					}
				}
				if !seen {
					order[c.FilePath] = append(order[c.FilePath], target)
				}
			}
		}
		// Record named import binding. An unresolved explicit import is retained
		// with an empty source path so call resolution will not guess a repository
		// target with the same name as an external dependency.
		if c.NamedBinding != "" && ir == nil {
			if nim[c.FilePath] == nil {
				nim[c.FilePath] = make(map[string]NamedImportBinding)
			}
			nim[c.FilePath][c.NamedBinding] = NamedImportBinding{
				SourcePath: "@unresolved:" + c.ImportPath, ExportedName: c.ExportedName,
			}
		} else if c.NamedBinding != "" && ir != nil {
			for _, tgt := range ir.Files {
				if nim[c.FilePath] == nil {
					nim[c.FilePath] = make(map[string]NamedImportBinding)
				}
				nim[c.FilePath][c.NamedBinding] = NamedImportBinding{
					SourcePath:   tgt,
					ExportedName: c.ExportedName,
				}
			}
		}
	}
}

func isFileInPackageDir(fp string, ds string) bool {
	dir := filepath.Dir(fp)
	return dir == ds || strings.HasPrefix(dir, ds+"/")
}
