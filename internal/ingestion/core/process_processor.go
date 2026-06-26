// Process Processor — extracts process/workflow patterns from code (e.g., request handlers).
//
// Mirrors TS process-processor.ts, skeleton for codetrip.
// Detects named processes like HTTP handlers, event listeners, CLI commands,
// and creates Process nodes with HANDLES edges linking them to their triggers.
// Deferred to Phase 7 when the full pipeline is operational.

package core

import "github.com/mengshi02/codetrip/internal/ingestion/shared"

// ProcessDefinition represents a named process/workflow in the codebase.
type ProcessDefinition struct {
	Name        string
	TriggerKind string // "http_handler", "event_listener", "cli_command", "cron_job"
	TriggerPath string // e.g., "/api/users" for HTTP handlers
	ScopeID     string
	File        string
	Line        int
}

// ProcessProcessorResult holds all discovered processes.
type ProcessProcessorResult struct {
	Processes []ProcessDefinition
	Stats     map[string]int // triggerKind → count
}

// DetectProcesses scans the graph for process patterns.
//
// Current status: skeleton — full implementation deferred to Phase 7.
func DetectProcesses(graph shared.KnowledgeGraph, frameworkHints []FrameworkHintExt) (*ProcessProcessorResult, error) {
	// TODO(Phase 7): detect HTTP handlers, event listeners, CLI commands etc.
	return &ProcessProcessorResult{
		Processes: []ProcessDefinition{},
		Stats:     map[string]int{},
	}, nil
}

// AddProcessNodesToGraph creates Process nodes and HANDLES edges.
//
// Current status: skeleton — full implementation deferred to Phase 7.
func AddProcessNodesToGraph(graph shared.KnowledgeGraph, result *ProcessProcessorResult) error {
	// TODO(Phase 7): create Process + HANDLES edges
	return nil
}