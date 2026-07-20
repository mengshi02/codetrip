package enrich

// Process Detection Processor.
//
// Detects execution flows (Processes) in the code graph by:
// 1. Finding entry points (functions with no internal callers)
// 2. Tracing forward via CALLS edges (BFS)
// 3. Grouping and deduplicating similar paths
// 4. Labeling with heuristic names
//
// Processes help agents understand how features work through the codebase.

import (
	"fmt"
	"sort"
	"strings"

	graph "github.com/mengshi02/codetrip/internal/model"
)

// ─────────────────────────────────────────────────────────────────────────────
// Configuration
// ─────────────────────────────────────────────────────────────────────────────

// ProcessDetectionConfig holds configuration for process detection.
type ProcessDetectionConfig struct {
	MaxTraceDepth int // Maximum steps to trace (default: 10)
	MaxBranching  int // Max branches to follow per node (default: 4)
	MaxProcesses  int // Maximum processes to detect (default: 75)
	MinSteps      int // Minimum steps for a valid process (default: 3)
}

// DefaultProcessConfig returns the default configuration.
func DefaultProcessConfig() ProcessDetectionConfig {
	return ProcessDetectionConfig{
		MaxTraceDepth: 10,
		MaxBranching:  4,
		MaxProcesses:  75,
		MinSteps:      3,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Public types
// ─────────────────────────────────────────────────────────────────────────────

// ProcessNode represents a detected execution flow.
type ProcessNode struct {
	ID             string // "proc_handleLogin_createSession"
	Label          string // "HandleLogin → CreateSession"
	HeuristicLabel string
	ProcessType    string // "intra_community" or "cross_community"
	StepCount      int
	Communities    []string // Community IDs touched
	EntryPointID   string
	TerminalID     string
	Trace          []string // Ordered array of node IDs
}

// ProcessStep represents a single step in a process.
type ProcessStep struct {
	NodeID    string
	ProcessID string
	Step      int // 1-indexed position in trace
}

// ProcessDetectionResult holds the complete process detection result.
type ProcessDetectionResult struct {
	Processes []ProcessNode
	Steps     []ProcessStep
	Stats     ProcessStats
}

// ProcessStats holds statistics about detected processes.
type ProcessStats struct {
	TotalProcesses      int
	CrossCommunityCount int
	AvgStepCount        float64
	EntryPointsFound    int
}

// ─────────────────────────────────────────────────────────────────────────────
// Main processor
// ─────────────────────────────────────────────────────────────────────────────

// minTraceConfidence is the minimum edge confidence for process tracing.
// Filters out ambiguous fuzzy-global matches (0.3) that cause
// traces to jump across unrelated code areas.
const minTraceConfidence = 0.5

// ProcessProcesses detects processes (execution flows) in the knowledge graph.
// This runs AFTER community detection, using CALLS edges to trace flows.
func ProcessProcesses(
	kg *graph.KnowledgeGraph,
	memberships []CommunityMembership,
	cfg ProcessDetectionConfig,
) ProcessDetectionResult {
	// Build lookup maps
	membershipMap := make(map[string]string)
	for _, m := range memberships {
		membershipMap[m.NodeID] = m.CommunityID
	}

	callsEdges := buildCallsGraph(kg)
	reverseCallsEdges := buildReverseCallsGraph(kg)
	nodeMap := make(map[string]*graph.GraphNode)
	kg.ForEachNode(func(n *graph.GraphNode) {
		nodeMap[n.ID] = n
	})

	// Step 1: Find entry points
	entryPoints := findEntryPoints(kg, reverseCallsEdges, callsEdges)

	// Step 2: Trace processes from each entry point
	allTraces := [][]string{}
	maxTraces := cfg.MaxProcesses * 2

	for i := 0; i < len(entryPoints) && len(allTraces) < maxTraces; i++ {
		entryID := entryPoints[i]
		traces := traceFromEntryPoint(entryID, callsEdges, cfg)

		for _, t := range traces {
			if len(t) >= cfg.MinSteps {
				allTraces = append(allTraces, t)
			}
		}
	}

	// Step 3: Deduplicate similar traces (subset removal)
	uniqueTraces := deduplicateTraces(allTraces)

	// Step 3b: Deduplicate by entry+terminal pair (keep longest path per pair)
	endpointDeduped := deduplicateByEndpoints(uniqueTraces)

	// Step 4: Limit to max processes (prioritize longer traces)
	sort.Slice(endpointDeduped, func(i, j int) bool {
		return len(endpointDeduped[i]) > len(endpointDeduped[j])
	})
	if len(endpointDeduped) > cfg.MaxProcesses {
		endpointDeduped = endpointDeduped[:cfg.MaxProcesses]
	}

	// Step 5: Create process nodes
	processes := []ProcessNode{}
	steps := []ProcessStep{}

	for idx, trace := range endpointDeduped {
		entryPointID := trace[0]
		terminalID := trace[len(trace)-1]

		// Get communities touched
		commSet := make(map[string]bool)
		for _, nodeID := range trace {
			if comm, ok := membershipMap[nodeID]; ok {
				commSet[comm] = true
			}
		}
		communities := []string{}
		for c := range commSet {
			communities = append(communities, c)
		}
		sort.Strings(communities)

		// Determine process type
		processType := "intra_community"
		if len(communities) > 1 {
			processType = "cross_community"
		}

		// Generate label
		entryName := "Unknown"
		terminalName := "Unknown"
		if entryNode, ok := nodeMap[entryPointID]; ok {
			if entryNode.Properties.Name != "" {
				entryName = entryNode.Properties.Name
			}
		}
		if terminalNode, ok := nodeMap[terminalID]; ok {
			if terminalNode.Properties.Name != "" {
				terminalName = terminalNode.Properties.Name
			}
		}
		heuristicLabel := fmt.Sprintf("%s → %s", capitalize(entryName), capitalize(terminalName))

		processID := fmt.Sprintf("proc_%d_%s", idx, sanitizeID(entryName))

		processes = append(processes, ProcessNode{
			ID:             processID,
			Label:          heuristicLabel,
			HeuristicLabel: heuristicLabel,
			ProcessType:    processType,
			StepCount:      len(trace),
			Communities:    communities,
			EntryPointID:   entryPointID,
			TerminalID:     terminalID,
			Trace:          trace,
		})

		// Create step relationships
		for stepIdx, nodeID := range trace {
			steps = append(steps, ProcessStep{
				NodeID:    nodeID,
				ProcessID: processID,
				Step:      stepIdx + 1,
			})
		}
	}

	// Calculate stats
	crossCommunityCount := 0
	totalSteps := 0
	for _, p := range processes {
		if p.ProcessType == "cross_community" {
			crossCommunityCount++
		}
		totalSteps += p.StepCount
	}
	avgStepCount := 0.0
	if len(processes) > 0 {
		avgStepCount = float64(totalSteps) / float64(len(processes))
		avgStepCount = float64(int(avgStepCount*10)) / 10.0 // round to 1 decimal
	}

	return ProcessDetectionResult{
		Processes: processes,
		Steps:     steps,
		Stats: ProcessStats{
			TotalProcesses:      len(processes),
			CrossCommunityCount: crossCommunityCount,
			AvgStepCount:        avgStepCount,
			EntryPointsFound:    len(entryPoints),
		},
	}
}

// ApplyProcessesToGraph adds Process nodes and STEP_IN_PROCESS relationships to the KG.
func ApplyProcessesToGraph(kg *graph.KnowledgeGraph, result ProcessDetectionResult) {
	for _, proc := range result.Processes {
		stepCount := float64(proc.StepCount)
		procNode := &graph.GraphNode{
			ID:    proc.ID,
			Label: graph.LabelProcess,
			Properties: graph.NodeProperties{
				Name:           proc.Label,
				HeuristicLabel: proc.HeuristicLabel,
				ProcessType:    proc.ProcessType,
				StepCount:      &stepCount,
				Communities:    proc.Communities,
				EntryPointID:   proc.EntryPointID,
				TerminalID:     proc.TerminalID,
			},
		}
		kg.AddNode(procNode)
	}

	for _, step := range result.Steps {
		relID := graph.GenerateID("STEP_IN_PROCESS", fmt.Sprintf("%s_%d_%s", step.ProcessID, step.Step, step.NodeID))
		stepVal := step.Step
		kg.AddRelationship(&graph.GraphRelationship{
			ID:         relID,
			SourceID:   step.NodeID,
			TargetID:   step.ProcessID,
			Type:       graph.RelSTEP_IN_PROCESS,
			Confidence: 1.0,
			Reason:     "process-trace",
			Step:       &stepVal,
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Helper: Build CALLS adjacency list
// ─────────────────────────────────────────────────────────────────────────────

func buildCallsGraph(kg *graph.KnowledgeGraph) map[string][]string {
	adj := make(map[string][]string)
	kg.ForEachRelationship(func(rel *graph.GraphRelationship) {
		if rel.Type == graph.RelCALLS && rel.Confidence >= minTraceConfidence {
			adj[rel.SourceID] = append(adj[rel.SourceID], rel.TargetID)
		}
	})
	return adj
}

func buildReverseCallsGraph(kg *graph.KnowledgeGraph) map[string][]string {
	adj := make(map[string][]string)
	kg.ForEachRelationship(func(rel *graph.GraphRelationship) {
		if rel.Type == graph.RelCALLS && rel.Confidence >= minTraceConfidence {
			adj[rel.TargetID] = append(adj[rel.TargetID], rel.SourceID)
		}
	})
	return adj
}

// ─────────────────────────────────────────────────────────────────────────────
// Helper: Find entry points
// ─────────────────────────────────────────────────────────────────────────────

type entryPointCandidate struct {
	ID      string
	Score   float64
	Reasons []string
}

// findEntryPoints finds functions/methods that are good entry points for tracing.
// Entry points are scored based on call ratio, export status, name patterns, and framework.
// Test files are excluded entirely.
func findEntryPoints(
	kg *graph.KnowledgeGraph,
	reverseCallsEdges map[string][]string,
	callsEdges map[string][]string,
) []string {
	symbolTypes := map[graph.NodeLabel]bool{
		graph.LabelFunction: true,
		graph.LabelMethod:   true,
	}

	candidates := []entryPointCandidate{}

	kg.ForEachNode(func(node *graph.GraphNode) {
		if !symbolTypes[node.Label] {
			return
		}

		filePath := node.Properties.FilePath

		// Skip test files entirely
		if IsTestFile(filePath) {
			return
		}

		callers := reverseCallsEdges[node.ID]
		callees := callsEdges[node.ID]

		// Must have at least 1 outgoing call to trace forward
		if len(callees) == 0 {
			return
		}

		// Calculate entry point score
		isExported := node.Properties.IsExported != nil && *node.Properties.IsExported
		language := node.Properties.Language
		if language == "" {
			language = "javascript" // default fallback
		}

		result := CalculateEntryPointScore(
			node.Properties.Name,
			language,
			isExported,
			len(callers),
			len(callees),
			filePath,
		)

		// Apply AST framework multiplier if present
		score := result.Score
		reasons := result.Reasons

		if score > 0 {
			candidates = append(candidates, entryPointCandidate{
				ID:      node.ID,
				Score:   score,
				Reasons: reasons,
			})
		}
	})

	// Sort by score descending
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})

	// Limit to prevent explosion (top 200)
	limit := 200
	if len(candidates) < limit {
		limit = len(candidates)
	}

	result := make([]string, limit)
	for i, c := range candidates[:limit] {
		result[i] = c.ID
	}
	return result
}

// ─────────────────────────────────────────────────────────────────────────────
// Helper: Trace from entry point (BFS)
// ─────────────────────────────────────────────────────────────────────────────

// traceFromEntryPoint traces forward from an entry point using BFS.
// Returns all distinct paths up to maxDepth.
func traceFromEntryPoint(
	entryID string,
	callsEdges map[string][]string,
	cfg ProcessDetectionConfig,
) [][]string {
	traces := [][]string{}

	// BFS with path tracking
	// Each queue item: [currentNodeId, pathSoFar]
	type queueItem struct {
		currentID string
		path      []string
	}
	queue := []queueItem{{currentID: entryID, path: []string{entryID}}}
	maxTraces := cfg.MaxBranching * 3

	for len(queue) > 0 && len(traces) < maxTraces {
		item := queue[0]
		queue = queue[1:]

		callees := callsEdges[item.currentID]

		if len(callees) == 0 {
			// Terminal node - this is a complete trace
			if len(item.path) >= cfg.MinSteps {
				traces = append(traces, append([]string{}, item.path...))
			}
		} else if len(item.path) >= cfg.MaxTraceDepth {
			// Max depth reached - save what we have
			if len(item.path) >= cfg.MinSteps {
				traces = append(traces, append([]string{}, item.path...))
			}
		} else {
			// Continue tracing - limit branching
			limitedCallees := callees
			if len(limitedCallees) > cfg.MaxBranching {
				limitedCallees = limitedCallees[:cfg.MaxBranching]
			}

			addedBranch := false
			for _, calleeID := range limitedCallees {
				// Avoid cycles
				if !containsString(item.path, calleeID) {
					newPath := append(append([]string{}, item.path...), calleeID)
					queue = append(queue, queueItem{currentID: calleeID, path: newPath})
					addedBranch = true
				}
			}

			// If all branches were cycles, save current path as terminal
			if !addedBranch && len(item.path) >= cfg.MinSteps {
				traces = append(traces, append([]string{}, item.path...))
			}
		}
	}

	return traces
}

// ─────────────────────────────────────────────────────────────────────────────
// Helper: Deduplicate traces
// ─────────────────────────────────────────────────────────────────────────────

// deduplicateTraces merges traces that are subsets of other traces.
// Keep longer traces, remove redundant shorter ones.
func deduplicateTraces(traces [][]string) [][]string {
	if len(traces) == 0 {
		return nil
	}

	// Sort by length descending
	sorted := make([][]string, len(traces))
	copy(sorted, traces)
	sort.Slice(sorted, func(i, j int) bool {
		return len(sorted[i]) > len(sorted[j])
	})

	unique := [][]string{}
	for _, trace := range sorted {
		traceKey := strings.Join(trace, "->")
		isSubset := false
		for _, existing := range unique {
			existingKey := strings.Join(existing, "->")
			if strings.Contains(existingKey, traceKey) {
				isSubset = true
				break
			}
		}
		if !isSubset {
			unique = append(unique, trace)
		}
	}

	return unique
}

// deduplicateByEndpoints keeps only the longest trace per unique entry→terminal pair.
func deduplicateByEndpoints(traces [][]string) [][]string {
	if len(traces) == 0 {
		return nil
	}

	// Sort longest first so the first seen per key is the longest
	sorted := make([][]string, len(traces))
	copy(sorted, traces)
	sort.Slice(sorted, func(i, j int) bool {
		return len(sorted[i]) > len(sorted[j])
	})

	byEndpoints := make(map[string][]string)
	for _, trace := range sorted {
		key := trace[0] + "::" + trace[len(trace)-1]
		if _, exists := byEndpoints[key]; !exists {
			byEndpoints[key] = trace
		}
	}

	result := [][]string{}
	for _, trace := range byEndpoints {
		result = append(result, trace)
	}
	return result
}

// ─────────────────────────────────────────────────────────────────────────────
// String utilities
// ─────────────────────────────────────────────────────────────────────────────

// sanitizeID replaces non-alphanumeric characters with underscores and truncates.
func sanitizeID(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	result := b.String()
	if len(result) > 20 {
		result = result[:20]
	}
	return strings.ToLower(result)
}

func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
