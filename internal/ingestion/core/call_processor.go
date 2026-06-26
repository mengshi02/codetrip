// Call Processor — extracts call-graph edges from parsed symbols.
//
// Mirrors TS call-processor.ts, skeleton for codetrip.
// After parsing, call_processor scans function bodies for call expressions
// and creates CALLS edges between the caller and callee symbols.
// Deferred to Phase 3 when tree-sitter parsing produces call-site data.

package core

import "github.com/mengshi02/codetrip/internal/ingestion/shared"

// CallSite represents a function call found in source code.
type CallSite struct {
	CallerScopeID string
	CalleeName     string // raw text of the callee (may be qualified like obj.method)
	File           string
	Line           int
	IsMethodCall   bool
	ReceiverType   string // for method calls, the inferred receiver type
}

// CallProcessorResult holds the extracted call-graph edges.
type CallProcessorResult struct {
	CallSites []CallSite
	// Resolved calls: calleeName → ScopeID mapping (after resolution)
	ResolvedCalls map[string][]string // calleeName → list of possible target ScopeIDs
}

// ProcessCalls extracts call sites from a parsed file's symbol data.
//
// Current status: skeleton — full implementation deferred to Phase 3.
func ProcessCalls(filePath string, parsedData interface{}) (*CallProcessorResult, error) {
	// TODO(Phase 3): iterate parsed symbols, find call expressions
	return &CallProcessorResult{
		CallSites:     []CallSite{},
		ResolvedCalls: map[string][]string{},
	}, nil
}

// AddCallEdges writes resolved CALLS edges into the KnowledgeGraph.
//
// Current status: skeleton — full implementation deferred to Phase 3.
func AddCallEdges(graph shared.KnowledgeGraph, result *CallProcessorResult) error {
	// TODO(Phase 3): create CALLS edges with Evidence
	return nil
}