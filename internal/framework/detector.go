package framework

import (
	"path/filepath"
	"strings"
	"sync"
)

// FrameworkDetector is the framework detector interface
type FrameworkDetector interface {
	DetectFromPath(filePath string) *FrameworkHint
	DetectFromAST(filePath string, symbols []SymbolMatch) *FrameworkHint
}

// SymbolMatch represents symbol match information (simplified version to avoid dependency on pipeline package)
type SymbolMatch struct {
	Name     string
	Label    string
	Receiver string
	FilePath string
	Line     int
}

// FrameworkHint represents framework detection hint
type FrameworkHint struct {
	Framework            string
	EntryPointMultiplier float64 // entry point score multiplier
	Reason               string
}

// FrameworkRegistry is the framework detector registry
type FrameworkRegistry struct {
	mu          sync.RWMutex
	detectors   []FrameworkDetector
	pathRules   []PathRule
	symbolRules []SymbolRule
}

// PathRule represents file path pattern matching rule
type PathRule struct {
	Pattern    string // regex pattern
	Framework  string
	Multiplier float64
	Reason     string
}

// SymbolRule represents symbol/decorator pattern matching rule
type SymbolRule struct {
	Name       string // symbol name (supports suffix matching)
	Label      string // symbol label filter (optional)
	Receiver   string // receiver filter (optional)
	Framework  string
	Multiplier float64
	Reason     string
}

// NewFrameworkRegistry creates a new framework detector registry
func NewFrameworkRegistry() *FrameworkRegistry {
	r := &FrameworkRegistry{
		detectors:   make([]FrameworkDetector, 0),
		pathRules:   defaultPathRules(),
		symbolRules: defaultSymbolRules(),
	}
	return r
}

// Register registers a framework detector
func (r *FrameworkRegistry) Register(detector FrameworkDetector) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.detectors = append(r.detectors, detector)
}

// AddPathRule adds a path matching rule
func (r *FrameworkRegistry) AddPathRule(rule PathRule) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pathRules = append(r.pathRules, rule)
}

// AddSymbolRule adds a symbol matching rule
func (r *FrameworkRegistry) AddSymbolRule(rule SymbolRule) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.symbolRules = append(r.symbolRules, rule)
}

// DetectFromPath detects framework from file path
func (r *FrameworkRegistry) DetectFromPath(filePath string) *FrameworkHint {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// 1. Check custom detectors first
	for _, d := range r.detectors {
		if hint := d.DetectFromPath(filePath); hint != nil {
			return hint
		}
	}

	// 2. Check path rules
	normalized := filepath.ToSlash(filePath)
	for _, rule := range r.pathRules {
		if matchPathRule(normalized, rule) {
			return &FrameworkHint{
				Framework:            rule.Framework,
				EntryPointMultiplier: rule.Multiplier,
				Reason:               rule.Reason,
			}
		}
	}

	return nil
}

// DetectFromAST detects framework from AST symbols
func (r *FrameworkRegistry) DetectFromAST(filePath string, symbols []SymbolMatch) *FrameworkHint {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// 1. Check custom detectors first
	for _, d := range r.detectors {
		if hint := d.DetectFromAST(filePath, symbols); hint != nil {
			return hint
		}
	}

	// 2. Check symbol rules
	for _, sym := range symbols {
		for _, rule := range r.symbolRules {
			if matchSymbolRule(sym, rule) {
				return &FrameworkHint{
					Framework:            rule.Framework,
					EntryPointMultiplier: rule.Multiplier,
					Reason:               rule.Reason,
				}
			}
		}
	}

	return nil
}

// DetectAll performs comprehensive path+symbol detection and returns all matching framework hints
func (r *FrameworkRegistry) DetectAll(filePath string, symbols []SymbolMatch) []*FrameworkHint {
	r.mu.RLock()
	defer r.mu.RUnlock()

	seen := make(map[string]bool)
	var hints []*FrameworkHint

	// Path detection
	normalized := filepath.ToSlash(filePath)
	for _, rule := range r.pathRules {
		if matchPathRule(normalized, rule) && !seen[rule.Framework] {
			seen[rule.Framework] = true
			hints = append(hints, &FrameworkHint{
				Framework:            rule.Framework,
				EntryPointMultiplier: rule.Multiplier,
				Reason:               rule.Reason,
			})
		}
	}

	// Symbol detection
	for _, sym := range symbols {
		for _, rule := range r.symbolRules {
			if matchSymbolRule(sym, rule) && !seen[rule.Framework] {
				seen[rule.Framework] = true
				hints = append(hints, &FrameworkHint{
					Framework:            rule.Framework,
					EntryPointMultiplier: rule.Multiplier,
					Reason:               rule.Reason,
				})
			}
		}
	}

	// Custom detectors
	for _, d := range r.detectors {
		if hint := d.DetectFromPath(filePath); hint != nil && !seen[hint.Framework] {
			seen[hint.Framework] = true
			hints = append(hints, hint)
		}
		if hint := d.DetectFromAST(filePath, symbols); hint != nil && !seen[hint.Framework] {
			seen[hint.Framework] = true
			hints = append(hints, hint)
		}
	}

	return hints
}

// matchPathRule checks if file path matches the rule
func matchPathRule(filePath string, rule PathRule) bool {
	// Simple substring/suffix matching (high performance)
	return strings.Contains(filePath, rule.Pattern) ||
		strings.HasSuffix(filePath, rule.Pattern) ||
		matchGlob(filePath, rule.Pattern)
}

// matchGlob performs simple glob matching
func matchGlob(path, pattern string) bool {
	matched, _ := filepath.Match(pattern, filepath.Base(path))
	if matched {
		return true
	}
	// Try to match each level in the path
	parts := strings.Split(path, "/")
	for _, part := range parts {
		matched, _ = filepath.Match(pattern, part)
		if matched {
			return true
		}
	}
	return false
}

// matchSymbolRule checks if symbol matches the rule
func matchSymbolRule(sym SymbolMatch, rule SymbolRule) bool {
	// Name matching
	if rule.Name != "" {
		if !strings.Contains(sym.Name, rule.Name) && !strings.HasSuffix(sym.Name, rule.Name) {
			return false
		}
	}

	// Label filtering
	if rule.Label != "" && sym.Label != rule.Label {
		return false
	}

	// Receiver filtering
	if rule.Receiver != "" && sym.Receiver != rule.Receiver {
		return false
	}

	return true
}
