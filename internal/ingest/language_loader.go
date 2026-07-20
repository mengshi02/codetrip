package ingest

// Language loader — registers go-tree-sitter Language objects for each supported language.
// Uses CGO-based grammar bindings from tree-sitter-*-go packages.
// Covers all supported parsers (Go, JavaScript/JSX, TypeScript/TSX, Python,
// Java, C, C++, C#, Rust, Ruby, PHP, Kotlin, and Swift).

import (
	"fmt"
	"sync"
	"unsafe"

	tree_sitter_swift "github.com/alex-pinkus/tree-sitter-swift/bindings/go"
	tree_sitter_kotlin "github.com/fwcd/tree-sitter-kotlin/bindings/go"
	sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_c_sharp "github.com/tree-sitter/tree-sitter-c-sharp/bindings/go"
	tree_sitter_c "github.com/tree-sitter/tree-sitter-c/bindings/go"
	tree_sitter_cpp "github.com/tree-sitter/tree-sitter-cpp/bindings/go"
	tree_sitter_go "github.com/tree-sitter/tree-sitter-go/bindings/go"
	tree_sitter_java "github.com/tree-sitter/tree-sitter-java/bindings/go"
	tree_sitter_javascript "github.com/tree-sitter/tree-sitter-javascript/bindings/go"
	tree_sitter_php "github.com/tree-sitter/tree-sitter-php/bindings/go"
	tree_sitter_python "github.com/tree-sitter/tree-sitter-python/bindings/go"
	tree_sitter_ruby "github.com/tree-sitter/tree-sitter-ruby/bindings/go"
	tree_sitter_rust "github.com/tree-sitter/tree-sitter-rust/bindings/go"
	tree_sitter_typescript "github.com/tree-sitter/tree-sitter-typescript/bindings/go"
)

// LanguageRegistry holds lazily-loaded *sitter.Language instances keyed by language ID.
type LanguageRegistry struct {
	mu    sync.RWMutex
	cache map[string]*sitter.Language
}

// NewLanguageRegistry creates a new language registry.
func NewLanguageRegistry() *LanguageRegistry {
	return &LanguageRegistry{
		cache: make(map[string]*sitter.Language),
	}
}

// GetLanguage returns the *sitter.Language for the given language ID.
// Languages are loaded lazily on first access and cached.
func (r *LanguageRegistry) GetLanguage(langID string) (*sitter.Language, error) {
	r.mu.RLock()
	if lang, ok := r.cache[langID]; ok {
		r.mu.RUnlock()
		return lang, nil
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check after acquiring write lock
	if lang, ok := r.cache[langID]; ok {
		return lang, nil
	}

	lang, err := r.loadLanguage(langID)
	if err != nil {
		return nil, err
	}
	r.cache[langID] = lang
	return lang, nil
}

// loadLanguage loads the tree-sitter language for the given ID.
func (r *LanguageRegistry) loadLanguage(langID string) (*sitter.Language, error) {
	var ptr unsafe.Pointer

	switch langID {
	case "go":
		ptr = tree_sitter_go.Language()
	case "javascript":
		ptr = tree_sitter_javascript.Language()
	case "typescript":
		ptr = tree_sitter_typescript.LanguageTypescript()
	case "tsx":
		ptr = tree_sitter_typescript.LanguageTSX()
	case "python":
		ptr = tree_sitter_python.Language()
	case "java":
		ptr = tree_sitter_java.Language()
	case "c":
		ptr = tree_sitter_c.Language()
	case "cpp":
		ptr = tree_sitter_cpp.Language()
	case "csharp":
		ptr = tree_sitter_c_sharp.Language()
	case "rust":
		ptr = tree_sitter_rust.Language()
	case "ruby":
		ptr = tree_sitter_ruby.Language()
	case "php":
		ptr = tree_sitter_php.LanguagePHP()
	case "kotlin":
		ptr = tree_sitter_kotlin.Language()
	case "swift":
		ptr = tree_sitter_swift.Language()
	default:
		return nil, fmt.Errorf("unsupported language: %q", langID)
	}

	lang := sitter.NewLanguage(ptr)
	if lang == nil {
		return nil, fmt.Errorf("failed to create Language for %q", langID)
	}
	return lang, nil
}

// SupportedLanguageIDs returns the list of language IDs that have go-tree-sitter bindings.
func (r *LanguageRegistry) SupportedLanguageIDs() []string {
	return []string{"go", "javascript", "typescript", "tsx", "python", "java", "c", "cpp", "csharp", "rust", "ruby", "php", "kotlin", "swift"}
}

// HasBinding returns true if the language ID has a go-tree-sitter binding.
func (r *LanguageRegistry) HasBinding(langID string) bool {
	switch langID {
	case "go", "javascript", "typescript", "tsx", "python", "java", "c", "cpp", "csharp", "rust", "ruby", "php", "kotlin", "swift":
		return true
	default:
		return false
	}
}
