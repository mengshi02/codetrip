package ingest

// Import-related types used across multiple processors.
//
// ImportMap:  Map<FilePath, Set<ResolvedFilePath>>  — exact file-to-file imports
// PackageMap: Map<FilePath, Set<PackageDirSuffix>> — directory-suffix-level imports (Go packages, C# namespaces)
// NamedImportMap: Map<FilePath, Map<LocalName, NamedImportBinding>> — defined in named_binding.go

// ImportMap maps each file to the set of files it imports.
type ImportMap map[string]map[string]bool

// ImportOrderMap preserves source declaration order for languages where
// duplicate symbols across headers are resolved by the first include.
type ImportOrderMap map[string][]string

// NewImportMap creates an empty ImportMap.
func NewImportMap() ImportMap {
	return make(ImportMap)
}

// AddImport adds an import relationship: sourceFile imports targetFile.
func (im ImportMap) AddImport(sourceFile, targetFile string) {
	if im[sourceFile] == nil {
		im[sourceFile] = make(map[string]bool)
	}
	im[sourceFile][targetFile] = true
}

// PackageMap maps each file to a set of package directory suffixes.
// Used for Go packages and C# namespaces where the import is at directory level.
type PackageMap map[string]map[string]bool

// NewPackageMap creates an empty PackageMap.
func NewPackageMap() PackageMap {
	return make(PackageMap)
}

// AddPackage adds a package directory suffix for a file.
func (pm PackageMap) AddPackage(filePath, dirSuffix string) {
	if pm[filePath] == nil {
		pm[filePath] = make(map[string]bool)
	}
	pm[filePath][dirSuffix] = true
}

// ─────────────────────────────────────────────────────────────────────────────
// Extracted data structures for fast-path processing.
// ─────────────────────────────────────────────────────────────────────────────

// ExtractedImport represents a single import captured during parsing.
type ExtractedImport struct {
	FilePath   string
	ImportPath string
	Language   string
	ImportNode string // serialized AST node info for named binding extraction
}

// ExtractedCall represents a single call captured during parsing.
type ExtractedCall struct {
	FilePath               string
	Language               string
	SourceID               string
	CallName               string
	ReceiverName           string
	ReceiverTypeName       string   // e.g. "Consumer" for consumer.Stop()
	ReceiverChain          []string // intermediate fluent methods from base receiver to terminal call
	ReceiverChainArgCounts []int    // argument count for each intermediate fluent method
	CallForm               CallForm
	ArgCount               int
	StartByte              uint
	Source                 []byte // source code slice for type env lookup
	CallNodeKind           string // AST node kind
}

// ExtractedHeritage represents a heritage (extends/implements) captured during parsing.
type ExtractedHeritage struct {
	FilePath     string
	ChildID      string
	ParentName   string
	HeritageType string // "extends", "implements", "trait"
	StartByte    uint
}

// ExtractedRoute represents a Laravel-style route extracted during parsing.
type ExtractedRoute struct {
	FilePath   string
	Controller string
	Method     string
	RoutePath  string
	HTTPMethod string
}

// FrameworkDetected represents a framework detection result.
type FrameworkDetected struct {
	FilePath  string
	Framework string
}

// ExtractedData holds all extracted data from the parsing phase.
type ExtractedData struct {
	Imports    []ExtractedImport
	Calls      []ExtractedCall
	Heritage   []ExtractedHeritage
	Routes     []ExtractedRoute
	Frameworks []FrameworkDetected
}
