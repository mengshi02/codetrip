package taint

import (
	"fmt"
	"strings"
	"sync"
)

// TaintModelRegistry is the taint model registry
// Supports registration/lookup of multiple TaintModels
type TaintModelRegistry struct {
	mu     sync.RWMutex
	models map[string]TaintModel // key: language
}

// NewTaintModelRegistry creates a taint model registry (with built-in models)
func NewTaintModelRegistry() *TaintModelRegistry {
	r := &TaintModelRegistry{
		models: make(map[string]TaintModel),
	}
	// Register built-in models
	r.Register(&TypeScriptModel{})
	r.Register(&GoModel{})
	return r
}

// Register registers a TaintModel
func (r *TaintModelRegistry) Register(model TaintModel) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.models[model.Language()] = model
}

// GetModel gets the TaintModel for the specified language
func (r *TaintModelRegistry) GetModel(language string) (TaintModel, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m, ok := r.models[language]
	return m, ok
}

// GetModels gets all registered TaintModels
func (r *TaintModelRegistry) GetModels() []TaintModel {
	r.mu.RLock()
	defer r.mu.RUnlock()
	models := make([]TaintModel, 0, len(r.models))
	for _, m := range r.models {
		models = append(models, m)
	}
	return models
}

// AllSources returns the union of all models' sources
func (r *TaintModelRegistry) AllSources() []SourceSpec {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var sources []SourceSpec
	for _, m := range r.models {
		sources = append(sources, m.Sources()...)
	}
	return sources
}

// AllSinks returns the union of all models' sinks
func (r *TaintModelRegistry) AllSinks() []SinkSpec {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var sinks []SinkSpec
	for _, m := range r.models {
		sinks = append(sinks, m.Sinks()...)
	}
	return sinks
}

// AllSanitizers returns the union of all models' sanitizers
func (r *TaintModelRegistry) AllSanitizers() []SanitizerSpec {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var sanitizers []SanitizerSpec
	for _, m := range r.models {
		sanitizers = append(sanitizers, m.Sanitizers()...)
	}
	return sanitizers
}

// IsSource checks if a symbol matches any registered source
func (r *TaintModelRegistry) IsSource(symbol string) (SourceSpec, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, m := range r.models {
		for _, s := range m.Sources() {
			if symbolMatches(symbol, s.Name) {
				return s, true
			}
		}
	}
	return SourceSpec{}, false
}

// IsSink checks if a symbol matches any registered sink
func (r *TaintModelRegistry) IsSink(symbol string) (SinkSpec, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, m := range r.models {
		for _, s := range m.Sinks() {
			if symbolMatches(symbol, s.Name) {
				return s, true
			}
		}
	}
	return SinkSpec{}, false
}

// IsSanitizer checks if a symbol matches any registered sanitizer
func (r *TaintModelRegistry) IsSanitizer(symbol string) (SanitizerSpec, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, m := range r.models {
		for _, s := range m.Sanitizers() {
			if symbolMatches(symbol, s.Name) {
				return s, true
			}
		}
	}
	return SanitizerSpec{}, false
}

// symbolMatches performs symbol matching (supports prefix and exact matching)
func symbolMatches(symbol, pattern string) bool {
	if symbol == pattern {
		return true
	}
	// Support prefix matching: "req.query" matches "req.query.x"
	if strings.HasPrefix(symbol, pattern+".") {
		return true
	}
	// Support suffix matching: "child_process.exec" matches "exec"
	if strings.HasSuffix(symbol, "."+pattern) {
		return true
	}
	return false
}

// ============ Built-in TypeScript/JavaScript Model ============

// TypeScriptModel is the TypeScript/JavaScript standard library taint model
type TypeScriptModel struct{}

func (m *TypeScriptModel) Language() string { return "typescript" }

func (m *TypeScriptModel) Sources() []SourceSpec {
	return []SourceSpec{
		{Name: "req.query", Package: "express", Category: "http-input"},
		{Name: "req.params", Package: "express", Category: "http-input"},
		{Name: "req.body", Package: "express", Category: "http-input"},
		{Name: "req.headers", Package: "express", Category: "http-input"},
		{Name: "req.cookies", Package: "express", Category: "http-input"},
		{Name: "window.location", Package: "browser", Category: "browser-input"},
		{Name: "document.URL", Package: "browser", Category: "browser-input"},
		{Name: "document.referrer", Package: "browser", Category: "browser-input"},
		{Name: "document.cookie", Package: "browser", Category: "browser-input"},
		{Name: "location.hash", Package: "browser", Category: "browser-input"},
		{Name: "location.search", Package: "browser", Category: "browser-input"},
		{Name: "location.href", Package: "browser", Category: "browser-input"},
		{Name: "process.argv", Package: "node", Category: "cli-input"},
		{Name: "process.env", Package: "node", Category: "env-input"},
		{Name: "readline", Package: "node", Category: "cli-input"},
		{Name: "fs.readFileSync", Package: "node", Category: "file-input"},
		{Name: "fs.readFile", Package: "node", Category: "file-input"},
		{Name: "localStorage.getItem", Package: "browser", Category: "storage-input"},
		{Name: "sessionStorage.getItem", Package: "browser", Category: "storage-input"},
	}
}

func (m *TypeScriptModel) Sinks() []SinkSpec {
	return []SinkSpec{
		{Name: "eval", Package: "builtin", Category: "code-exec"},
		{Name: "Function", Package: "builtin", Category: "code-exec"},
		{Name: "setTimeout", Package: "builtin", Category: "code-exec"},
		{Name: "setInterval", Package: "builtin", Category: "code-exec"},
		{Name: "exec", Package: "child_process", Category: "code-exec"},
		{Name: "execSync", Package: "child_process", Category: "code-exec"},
		{Name: "spawn", Package: "child_process", Category: "code-exec"},
		{Name: "child_process.exec", Package: "child_process", Category: "code-exec"},
		{Name: "fs.writeFile", Package: "node", Category: "file-write"},
		{Name: "fs.writeFileSync", Package: "node", Category: "file-write"},
		{Name: "fs.appendFile", Package: "node", Category: "file-write"},
		{Name: "response.write", Package: "express", Category: "xss"},
		{Name: "res.send", Package: "express", Category: "xss"},
		{Name: "res.write", Package: "http", Category: "xss"},
		{Name: "innerHTML", Package: "browser", Category: "xss"},
		{Name: "outerHTML", Package: "browser", Category: "xss"},
		{Name: "document.write", Package: "browser", Category: "xss"},
		{Name: "document.writeln", Package: "browser", Category: "xss"},
	}
}

func (m *TypeScriptModel) Sanitizers() []SanitizerSpec {
	return []SanitizerSpec{
		{Name: "escapeHtml", Package: "builtin", Category: "html-escape"},
		{Name: "sanitize", Package: "dompurify", Category: "html-sanitize"},
		{Name: "DOMPurify.sanitize", Package: "dompurify", Category: "html-sanitize"},
		{Name: "encodeURIComponent", Package: "builtin", Category: "url-encode"},
		{Name: "decodeURIComponent", Package: "builtin", Category: "url-encode"},
		{Name: "encodeURI", Package: "builtin", Category: "url-encode"},
		{Name: "JSON.stringify", Package: "builtin", Category: "json-encode"},
		{Name: "validator.escape", Package: "validator", Category: "html-escape"},
		{Name: "xss", Package: "xss", Category: "html-sanitize"},
	}
}

// ============ Built-in Go Model ============

// GoModel is the Go standard library taint model
type GoModel struct{}

func (m *GoModel) Language() string { return "go" }

func (m *GoModel) Sources() []SourceSpec {
	return []SourceSpec{
		{Name: "r.URL.Query", Package: "net/http", Category: "http-input"},
		{Name: "r.FormValue", Package: "net/http", Category: "http-input"},
		{Name: "r.Form", Package: "net/http", Category: "http-input"},
		{Name: "r.PostForm", Package: "net/http", Category: "http-input"},
		{Name: "r.Body", Package: "net/http", Category: "http-input"},
		{Name: "r.Header.Get", Package: "net/http", Category: "http-input"},
		{Name: "r.Cookie", Package: "net/http", Category: "http-input"},
		{Name: "r.URL.Path", Package: "net/http", Category: "http-input"},
		{Name: "os.Args", Package: "os", Category: "cli-input"},
		{Name: "os.Getenv", Package: "os", Category: "env-input"},
		{Name: "flag.Arg", Package: "flag", Category: "cli-input"},
		{Name: "flag.String", Package: "flag", Category: "cli-input"},
		{Name: "bufio.Scanner.Scan", Package: "bufio", Category: "stdin-input"},
		{Name: "io.ReadAll", Package: "io", Category: "file-input"},
		{Name: "os.ReadFile", Package: "os", Category: "file-input"},
	}
}

func (m *GoModel) Sinks() []SinkSpec {
	return []SinkSpec{
		{Name: "os.exec", Package: "os/exec", Category: "code-exec"},
		{Name: "exec.Command", Package: "os/exec", Category: "code-exec"},
		{Name: "exec.CommandContext", Package: "os/exec", Category: "code-exec"},
		{Name: "template.HTML", Package: "html/template", Category: "xss"},
		{Name: "template.JS", Package: "html/template", Category: "xss"},
		{Name: "template.URL", Package: "html/template", Category: "xss"},
		{Name: "fmt.Fprintf", Package: "fmt", Category: "xss"},
		{Name: "fmt.Sprintf", Package: "fmt", Category: "xss"},
		{Name: "db.Exec", Package: "database/sql", Category: "sql-injection"},
		{Name: "db.Query", Package: "database/sql", Category: "sql-injection"},
		{Name: "db.QueryRow", Package: "database/sql", Category: "sql-injection"},
		{Name: "os.WriteFile", Package: "os", Category: "file-write"},
		{Name: "os.Create", Package: "os", Category: "file-write"},
		{Name: "w.Write", Package: "net/http", Category: "xss"},
		{Name: "json.Unmarshal", Package: "encoding/json", Category: "deserialization"},
		{Name: "xml.Unmarshal", Package: "encoding/xml", Category: "deserialization"},
	}
}

func (m *GoModel) Sanitizers() []SanitizerSpec {
	return []SanitizerSpec{
		{Name: "html.EscapeString", Package: "html", Category: "html-escape"},
		{Name: "url.QueryEscape", Package: "net/url", Category: "url-encode"},
		{Name: "url.PathEscape", Package: "net/url", Category: "url-encode"},
		{Name: "json.Marshal", Package: "encoding/json", Category: "json-encode"},
		{Name: "json.NewEncoder.Encode", Package: "encoding/json", Category: "json-encode"},
		{Name: "base64.StdEncoding.EncodeToString", Package: "encoding/base64", Category: "base64-encode"},
		{Name: "regexp.QuoteMeta", Package: "regexp", Category: "regex-escape"},
		{Name: "strconv.Quote", Package: "strconv", Category: "string-escape"},
	}
}

// ============ Custom Model Builder ============

// ModelBuilder helps build custom TaintModels
type ModelBuilder struct {
	language    string
	sources     []SourceSpec
	sinks       []SinkSpec
	sanitizers  []SanitizerSpec
}

// NewModelBuilder creates a custom model builder
func NewModelBuilder(language string) *ModelBuilder {
	return &ModelBuilder{
		language: language,
	}
}

// AddSource adds a source
func (b *ModelBuilder) AddSource(name, pkg, category string) *ModelBuilder {
	b.sources = append(b.sources, SourceSpec{Name: name, Package: pkg, Category: category})
	return b
}

// AddSink adds a sink
func (b *ModelBuilder) AddSink(name, pkg, category string) *ModelBuilder {
	b.sinks = append(b.sinks, SinkSpec{Name: name, Package: pkg, Category: category})
	return b
}

// AddSanitizer adds a sanitizer
func (b *ModelBuilder) AddSanitizer(name, pkg, category string) *ModelBuilder {
	b.sanitizers = append(b.sanitizers, SanitizerSpec{Name: name, Package: pkg, Category: category})
	return b
}

// Build builds a custom TaintModel
func (b *ModelBuilder) Build() TaintModel {
	return &customModel{
		language:   b.language,
		sources:    b.sources,
		sinks:      b.sinks,
		sanitizers: b.sanitizers,
	}
}

// customModel is a custom TaintModel implementation
type customModel struct {
	language   string
	sources    []SourceSpec
	sinks      []SinkSpec
	sanitizers []SanitizerSpec
}

func (m *customModel) Language() string            { return m.language }
func (m *customModel) Sources() []SourceSpec       { return m.sources }
func (m *customModel) Sinks() []SinkSpec           { return m.sinks }
func (m *customModel) Sanitizers() []SanitizerSpec { return m.sanitizers }

// ValidateModel validates the integrity of a TaintModel
func ValidateModel(model TaintModel) error {
	if model.Language() == "" {
		return fmt.Errorf("taint model: language must not be empty")
	}
	for _, s := range model.Sources() {
		if s.Name == "" {
			return fmt.Errorf("taint model %s: source name must not be empty", model.Language())
		}
	}
	for _, s := range model.Sinks() {
		if s.Name == "" {
			return fmt.Errorf("taint model %s: sink name must not be empty", model.Language())
		}
	}
	for _, s := range model.Sanitizers() {
		if s.Name == "" {
			return fmt.Errorf("taint model %s: sanitizer name must not be empty", model.Language())
		}
	}
	return nil
}