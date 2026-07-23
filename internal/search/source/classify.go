package source

import (
	"bytes"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/go-enry/go-enry/v2"
)

// Scope controls which repository text is searched.
type Scope string

const (
	ScopeCode Scope = "code"
	ScopeDocs Scope = "docs"
	ScopeAll  Scope = "all"
)

// FileKind is the stable classification stored with source-index documents.
type FileKind string

const (
	FileCode   FileKind = "code"
	FileConfig FileKind = "config"
	FileDocs   FileKind = "docs"
)

var documentationExtensions = map[string]bool{
	".md": true, ".mdx": true, ".markdown": true, ".rst": true, ".adoc": true, ".asciidoc": true, ".txt": true,
}

var configurationExtensions = map[string]bool{
	".json": true, ".jsonc": true, ".yaml": true, ".yml": true, ".toml": true, ".ini": true, ".cfg": true, ".conf": true,
	".xml": true, ".properties": true, ".env": true, ".proto": true, ".sql": true, ".graphql": true, ".gql": true,
}

var codeExtensions = map[string]bool{
	".go": true, ".ts": true, ".tsx": true, ".js": true, ".jsx": true, ".mjs": true, ".cjs": true,
	".py": true, ".pyw": true, ".java": true, ".c": true, ".h": true, ".cpp": true, ".cc": true, ".cxx": true,
	".hpp": true, ".hxx": true, ".cs": true, ".rs": true, ".php": true, ".swift": true, ".kt": true,
	".kts": true, ".scala": true, ".dart": true, ".lua": true, ".zig": true, ".sh": true, ".bash": true, ".zsh": true,
	".ps1": true, ".pl": true, ".r": true, ".ex": true, ".exs": true, ".erl": true, ".hrl": true, ".fs": true, ".fsx": true,
	".vue": true, ".svelte": true, ".css": true, ".scss": true, ".sass": true, ".less": true, ".html": true, ".htm": true,
}

var codeFileNames = map[string]bool{
	"dockerfile": true, "makefile": true, "rakefile": true, "gemfile": true, "procfile": true, "justfile": true,
	"cmakelists.txt": true, "build": true, "workspace": true,
}

var documentationFileNames = map[string]bool{
	"readme": true, "authors": true, "maintainers": true, "notice": true, "license": true,
}

var ignoredSourceDirectories = map[string]bool{
	".git": true, ".codetrip": true, ".svn": true, ".hg": true, "node_modules": true, "vendor": true,
	"dist": true, "build": true, "out": true, "target": true, "bin": true, "obj": true, "coverage": true,
	".cache": true, "__pycache__": true, ".venv": true, "venv": true,
}

// ClassifyFile returns a searchable kind or an empty kind for ignored content.
func ClassifyFile(path string, content []byte) FileKind {
	if bytes.IndexByte(content, 0) >= 0 || !utf8.Valid(content) {
		return ""
	}
	name := strings.ToLower(filepath.Base(path))
	if codeFileNames[name] {
		return FileConfig
	}
	if documentationFileNames[name] {
		return FileDocs
	}
	ext := strings.ToLower(filepath.Ext(name))
	if documentationExtensions[ext] {
		return FileDocs
	}
	if configurationExtensions[ext] {
		return FileConfig
	}
	if codeExtensions[ext] {
		return FileCode
	}
	language := strings.ToLower(enry.GetLanguage(path, content))
	switch language {
	case "markdown", "asciidoc", "restructuredtext", "text":
		return FileDocs
	case "json", "yaml", "toml", "xml", "protocol buffer", "sql", "graphql", "dockerfile", "makefile":
		return FileConfig
	case "":
		return ""
	default:
		return FileCode
	}
}

func kindMatchesScope(kind FileKind, scope Scope) bool {
	switch scope {
	case ScopeDocs:
		return kind == FileDocs
	case ScopeAll:
		return kind != ""
	default:
		return kind == FileCode || kind == FileConfig
	}
}

func shouldSkipDirectory(name string) bool {
	return ignoredSourceDirectories[strings.ToLower(name)]
}

func scopePathPattern(scope Scope) string {
	extensions := make([]string, 0)
	add := func(values map[string]bool) {
		for extension := range values {
			extensions = append(extensions, regexp.QuoteMeta(extension))
		}
	}
	switch scope {
	case ScopeDocs:
		add(documentationExtensions)
	case ScopeCode:
		add(codeExtensions)
		add(configurationExtensions)
	default:
		return ""
	}
	sort.Strings(extensions)
	namesMap := documentationFileNames
	if scope == ScopeCode {
		namesMap = codeFileNames
	}
	names := make([]string, 0, len(namesMap))
	for name := range namesMap {
		names = append(names, regexp.QuoteMeta(name))
	}
	sort.Strings(names)
	return "(?i)(?:" + strings.Join(extensions, "|") + ")$|(?:^|/)(?:" + strings.Join(names, "|") + ")$"
}

func codeFileNamePattern() string {
	names := make([]string, 0, len(codeFileNames))
	for name := range codeFileNames {
		names = append(names, regexp.QuoteMeta(name))
	}
	sort.Strings(names)
	return "(?i)(?:^|/)(?:" + strings.Join(names, "|") + ")$"
}
