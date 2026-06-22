package taint

import (
	"github.com/mengshi02/codetrip/internal/cfg"
	"github.com/mengshi02/codetrip/internal/pipeline"
)

// SourceSpec is the taint source specification
type SourceSpec struct {
	Name     string // Source name (e.g., req.query, r.URL.Query)
	Package  string // Package name
	Category string // Category (e.g., http-input, cli-input, env)
}

// SinkSpec is the taint sink specification
type SinkSpec struct {
	Name     string // Sink name (e.g., eval, exec, template.HTML)
	Package  string // Package name
	Category string // Category (e.g., code-exec, file-write, xss, sql-injection)
}

// SanitizerSpec is the sanitizer specification
type SanitizerSpec struct {
	Name     string // Sanitizer name (e.g., escapeHtml, html.EscapeString)
	Package  string // Package name
	Category string // Category (e.g., html-escape, url-encode, json-encode)
}

// TaintModel is the taint model interface
// Defines source/sink/sanitizer sets for specific languages/frameworks
type TaintModel interface {
	// Language returns the language identifier for this model
	Language() string
	// Sources returns all taint sources defined by this model
	Sources() []SourceSpec
	// Sinks returns all taint sinks defined by this model
	Sinks() []SinkSpec
	// Sanitizers returns all sanitizers defined by this model
	Sanitizers() []SanitizerSpec
}

// TaintResult is the taint analysis result
// Compatible with pipeline.TaintFinding, with additional fields
type TaintResult struct {
	Category    string               // Vulnerability category
	SourceName  string               // Source name
	SourceLine  int                  // Source line number
	SinkName    string               // Sink name
	SinkLine    int                  // Sink line number
	HopPath    []pipeline.HopInfo   // Propagation path
	Sanitized  bool                 // Whether there's a sanitizer on the path
	Confidence float64              // Confidence score (0.0-1.0)
}

// ToFinding converts TaintResult to pipeline.TaintFinding
func (r *TaintResult) ToFinding() pipeline.TaintFinding {
	return pipeline.TaintFinding{
		Category:   r.Category,
		SourceLine: r.SourceLine,
		SinkLine:   r.SinkLine,
		HopPath:    r.HopPath,
	}
}

// TaintAnalyzer is the taint analyzer interface
type TaintAnalyzer interface {
	Analyze(fcfg *cfg.FunctionCFG, registry *TaintModelRegistry) ([]TaintResult, error)
}