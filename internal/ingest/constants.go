package ingest

// Tree-sitter buffer size constants.
const (
	// TreeSitterBufferSizeSmall is the default tree-sitter parse buffer size.
	TreeSitterBufferSizeSmall = 100_000

	// TreeSitterBufferSizeLarge is the large buffer size for big files.
	TreeSitterBufferSizeLarge = 1_000_000

	// MaxTreeSitterBufferSize is the absolute maximum buffer size.
	MaxTreeSitterBufferSize = 20_000_000

	// DefaultBufferSize is the default read buffer for file walking.
	DefaultBufferSize = 65536
)

// GetTreeSitterBufferSize returns the appropriate buffer size for a given file size.
func GetTreeSitterBufferSize(fileSize int64) int {
	if fileSize > int64(TreeSitterBufferSizeSmall) {
		if fileSize > int64(MaxTreeSitterBufferSize) {
			return MaxTreeSitterBufferSize
		}
		return TreeSitterBufferSizeLarge
	}
	return TreeSitterBufferSizeSmall
}

// SupportedLanguages defines the mapping of file extensions to language IDs.
var SupportedLanguages = map[string]string{
	// Go
	".go": "go",
	// TypeScript / JavaScript
	".ts":  "typescript",
	".tsx": "tsx",
	".js":  "javascript",
	".jsx": "javascript",
	// Python
	".py":  "python",
	".pyw": "python",
	// Java
	".java": "java",
	// C/C++
	".c":   "c",
	".h":   "cpp",
	".cpp": "cpp",
	".cc":  "cpp",
	".cxx": "cpp",
	".hpp": "cpp",
	".hxx": "cpp",
	// C#
	".cs": "csharp",
	// Rust
	".rs": "rust",
	// Ruby
	".rb": "ruby",
	// PHP
	".php": "php",
	// Swift
	".swift": "swift",
	// Kotlin
	".kt":  "kotlin",
	".kts": "kotlin",
	// Scala
	".scala": "scala",
	// Dart
	".dart": "dart",
	// Lua
	".lua": "lua",
	// Zig
	".zig": "zig",
	// Shell
	".sh":   "bash",
	".bash": "bash",
	".zsh":  "bash",
}

// LanguageID returns the language identifier for a file extension, or empty string if unsupported.
func LanguageID(ext string) string {
	if lang, ok := SupportedLanguages[ext]; ok {
		return lang
	}
	return ""
}

// SkipDirectories lists directories to skip during repository walking.
var SkipDirectories = map[string]bool{
	// Version Control
	".git": true,
	".svn": true,
	".hg":  true,
	".bzr": true,
	// IDEs & Editors
	".idea":     true,
	".vscode":   true,
	".vs":       true,
	".eclipse":  true,
	".settings": true,
	// Dependencies
	"node_modules":     true,
	"bower_components": true,
	"jspm_packages":    true,
	"vendor":           true,
	"venv":             true,
	".venv":            true,
	"env":              true,
	".env":             true,
	"__pycache__":      true,
	".pytest_cache":    true,
	".mypy_cache":      true,
	"site-packages":    true,
	".tox":             true,
	"eggs":             true,
	".eggs":            true,
	"lib64":            true,
	"parts":            true,
	"sdist":            true,
	"wheels":           true,
	// Build Outputs
	"dist":          true,
	"build":         true,
	"out":           true,
	"output":        true,
	"bin":           true,
	"obj":           true,
	"target":        true,
	".next":         true,
	".nuxt":         true,
	".output":       true,
	".vercel":       true,
	".netlify":      true,
	".serverless":   true,
	"_build":        true,
	".parcel-cache": true,
	".turbo":        true,
	".svelte-kit":   true,
	// Test & Coverage
	"coverage":    true,
	".nyc_output": true,
	"htmlcov":     true,
	".coverage":   true,
	".jest":       true,
	"__tests__":   true,
	"__mocks__":   true,
	// Logs & Temp
	"logs":   true,
	"log":    true,
	"tmp":    true,
	"temp":   true,
	"cache":  true,
	".cache": true,
	".tmp":   true,
	".temp":  true,
	// Generated/Compiled
	".generated":     true,
	"generated":      true,
	"auto-generated": true,
	".terraform":     true,
	// Misc
	".husky":        true,
	".github":       true, // GitHub config, not code
	".circleci":     true,
	".gitlab":       true,
	".gitnexus":     true, // GitNexus index/cache
	".claude":       true, // Claude skills/config
	"fixtures":      true,
	"snapshots":     true,
	"__snapshots__": true,
	// Bazel
	"bazel-bin":      true,
	"bazel-out":      true,
	"bazel-testlogs": true,
}

// ─────────────────────────────────────────────────────────────────────────────
// Definition capture keys — ordered list for extracting definition nodes from
// tree-sitter query capture maps.
// ─────────────────────────────────────────────────────────────────────────────

// DefinitionCaptureKeys is the ordered list of capture key names used to
// extract the definition node from a tree-sitter query capture map.
var DefinitionCaptureKeys = []string{
	"definition.function",
	"definition.class",
	"definition.interface",
	"definition.method",
	"definition.struct",
	"definition.enum",
	"definition.namespace",
	"definition.module",
	"definition.trait",
	"definition.impl",
	"definition.type",
	"definition.const",
	"definition.static",
	"definition.typedef",
	"definition.macro",
	"definition.union",
	"definition.property",
	"definition.record",
	"definition.delegate",
	"definition.annotation",
	"definition.constructor",
	"definition.template",
}

// ─────────────────────────────────────────────────────────────────────────────
// Function node types — AST node types that represent function/method definitions.
// ─────────────────────────────────────────────────────────────────────────────

var FunctionNodeTypes = map[string]bool{
	// TypeScript/JavaScript
	"function_declaration":           true,
	"arrow_function":                 true,
	"function_expression":            true,
	"method_definition":              true,
	"generator_function_declaration": true,
	// Python
	"function_definition": true,
	// C/C++
	// The C/C++ grammar uses the same node kind for free functions and
	// out-of-class method definitions.
	// Common async variants
	"async_function_declaration": true,
	"async_arrow_function":       true,
	// Java
	"method_declaration":      true,
	"constructor_declaration": true,
	// C#
	"local_function_statement": true,
	// Rust
	"function_item": true,
	"impl_item":     true,
	// PHP
	"anonymous_function": true,
	// Kotlin
	"lambda_literal": true,
	// Swift
	"init_declaration":   true,
	"deinit_declaration": true,
}

// FunctionDeclarationTypes are node types for standard function declarations
// that need C/C++ declarator handling.
var FunctionDeclarationTypes = map[string]bool{
	"function_declaration":           true,
	"function_definition":            true,
	"async_function_declaration":     true,
	"generator_function_declaration": true,
	"function_item":                  true,
}

// ─────────────────────────────────────────────────────────────────────────────
// Class container types — AST node types for class-like containers.
// ─────────────────────────────────────────────────────────────────────────────

var ClassContainerTypes = map[string]bool{
	"class_declaration":          true,
	"abstract_class_declaration": true,
	"interface_declaration":      true,
	"struct_declaration":         true,
	"record_declaration":         true,
	"class_specifier":            true,
	"struct_specifier":           true,
	"impl_item":                  true,
	"trait_item":                 true,
	"class_definition":           true,
	"trait_declaration":          true,
	"protocol_declaration":       true,
}

// ContainerTypeToLabel maps class container AST node types to graph labels.
var ContainerTypeToLabel = map[string]string{
	"class_declaration":          "Class",
	"abstract_class_declaration": "Class",
	"interface_declaration":      "Interface",
	"struct_declaration":         "Struct",
	"struct_specifier":           "Struct",
	"class_specifier":            "Class",
	"class_definition":           "Class",
	"impl_item":                  "Impl",
	"trait_item":                 "Trait",
	"trait_declaration":          "Trait",
	"record_declaration":         "Record",
	"protocol_declaration":       "Interface",
}

// ─────────────────────────────────────────────────────────────────────────────
// Call-related AST type sets.
// ─────────────────────────────────────────────────────────────────────────────

// CallArgumentListTypes are the AST node types for call argument containers.
var CallArgumentListTypes = map[string]bool{
	"arguments":       true,
	"argument_list":   true,
	"value_arguments": true,
}

// MemberAccessNodeTypes indicate a member-access wrapper around the callee name.
var MemberAccessNodeTypes = map[string]bool{
	"member_expression":        true, // TS/JS
	"attribute":                true, // Python
	"member_access_expression": true, // C#
	"field_expression":         true, // Rust/C++
	"selector_expression":      true, // Go
	"navigation_suffix":        true, // Kotlin/Swift
}

// ConstructorCallNodeTypes are call node types that are inherently constructor invocations.
var ConstructorCallNodeTypes = map[string]bool{
	"constructor_invocation":              true, // Kotlin
	"new_expression":                      true, // TS/JS/C++
	"object_creation_expression":          true, // Java/C#/PHP
	"implicit_object_creation_expression": true, // C# 9
	"composite_literal":                   true, // Go
	"struct_expression":                   true, // Rust
	"field_initializer":                   true, // C++ base/member constructor initializer
}

// ScopedCallNodeTypes are AST node types for scoped/qualified calls.
var ScopedCallNodeTypes = map[string]bool{
	"scoped_identifier":    true, // Rust: Foo::new()
	"qualified_identifier": true, // C++: ns::func()
}

// SimpleReceiverTypes are the AST node types for simple receiver identifiers.
var SimpleReceiverTypes = map[string]bool{
	"identifier":        true,
	"simple_identifier": true,
	"variable_name":     true, // PHP $variable
	"name":              true, // PHP name node
	"this":              true,
	"self":              true,
}

// CallableSymbolTypes are the symbol types that can be call targets.
var CallableSymbolTypes = map[string]bool{
	"Function":    true,
	"Method":      true,
	"Constructor": true,
	"Macro":       true,
}

// SkipExtensions lists file extensions to skip during repository walking.
var SkipExtensions = map[string]bool{
	// Images
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".svg": true,
	".ico": true, ".webp": true, ".bmp": true, ".tiff": true, ".tif": true,
	".psd": true, ".ai": true, ".sketch": true, ".fig": true, ".xd": true,
	// Archives
	".zip": true, ".tar": true, ".gz": true, ".rar": true, ".7z": true,
	".bz2": true, ".xz": true, ".tgz": true,
	// Binary/Compiled
	".exe": true, ".dll": true, ".so": true, ".dylib": true, ".a": true,
	".lib": true, ".o": true, ".obj": true,
	".class": true, ".jar": true, ".war": true, ".ear": true,
	".pyc": true, ".pyo": true, ".pyd": true,
	".beam": true, ".wasm": true, ".node": true,
	// Documents
	".pdf": true, ".doc": true, ".docx": true, ".xls": true, ".xlsx": true,
	".ppt": true, ".pptx": true, ".odt": true, ".ods": true, ".odp": true,
	// Media
	".mp4": true, ".mp3": true, ".wav": true, ".mov": true, ".avi": true,
	".mkv": true, ".flv": true, ".wmv": true, ".ogg": true, ".webm": true,
	".flac": true, ".aac": true, ".m4a": true,
	// Fonts
	".woff": true, ".woff2": true, ".ttf": true, ".eot": true, ".otf": true,
	// Databases
	".db": true, ".sqlite": true, ".sqlite3": true, ".mdb": true, ".accdb": true,
	// Source maps
	".map": true,
	// Lock files (also in SkipFiles, but double coverage)
	".lock": true,
	// Certificates & Keys (security — don't index!)
	".pem": true, ".key": true, ".crt": true, ".cer": true, ".p12": true, ".pfx": true,
	// Data files (often large/binary)
	".csv": true, ".tsv": true, ".parquet": true, ".avro": true, ".feather": true,
	".npy": true, ".npz": true, ".pkl": true, ".pickle": true, ".h5": true, ".hdf5": true,
	// Misc binary
	".bin": true, ".dat": true, ".data": true, ".raw": true,
	".iso": true, ".img": true, ".dmg": true,
	// Compound extensions (checked separately)
	".min.js": true, ".min.css": true, ".bundle.js": true, ".chunk.js": true,
	// TypeScript declaration files
	".d.ts": true,
}

// SkipFiles lists files to skip during repository walking.
var SkipFiles = map[string]bool{
	".DS_Store":         true,
	"package-lock.json": true,
	"yarn.lock":         true,
	"pnpm-lock.yaml":    true,
	"composer.lock":     true,
	"Gopkg.lock":        true,
	"go.sum":            true,
	"Cargo.lock":        true,
	"poetry.lock":       true,
	"Gemfile.lock":      true,
	".gitignore":        true,
	".gitattributes":    true,
	".npmrc":            true,
	".yarnrc":           true,
	".editorconfig":     true,
	".prettierrc":       true,
	".prettierignore":   true,
	".eslintignore":     true,
	".dockerignore":     true,
	"Thumbs.db":         true,
	".env":              true,
	".env.local":        true,
	".env.development":  true,
	".env.production":   true,
	".env.test":         true,
	".env.example":      true,
	// License & Changelog files
	"LICENSE":            true,
	"LICENSE.md":         true,
	"LICENSE.txt":        true,
	"CHANGELOG.md":       true,
	"CHANGELOG":          true,
	"CONTRIBUTING.md":    true,
	"CODE_OF_CONDUCT.md": true,
	"SECURITY.md":        true,
}
