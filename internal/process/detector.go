package process

import (
	"fmt"

	"github.com/mengshi02/codetrip/internal/community"
	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/util"
)

// ProcessDetector execution flow detector
// High-performance design: parallel entry scoring + BFS tracing
type ProcessDetector struct {
	config ProcessConfig
}

// NewProcessDetector creates a process detector
func NewProcessDetector(opts ...ProcessOption) *ProcessDetector {
	cfg := ProcessConfig{
		MaxTraceDepth: 10,
		MaxBranching:  4,
		MaxProcesses:  20,
		MinSteps:      3,
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	return &ProcessDetector{config: cfg}
}

// ProcessOption process detection configuration option
type ProcessOption func(*ProcessConfig)

// WithMaxTraceDepth sets maximum trace depth
func WithMaxTraceDepth(d int) ProcessOption {
	return func(c *ProcessConfig) { c.MaxTraceDepth = d }
}

// Detect detects processes
func (d *ProcessDetector) Detect(gs *graph.GraphStore, repo string, memberships []community.CommunityMembership) (*ProcessResult, error) {
	// 1. Build community mapping
	nodeToComm := make(map[string]string)
	for _, m := range memberships {
		nodeToComm[m.NodeID] = m.CommunityID
	}

	// 2. Collect entry point candidates
	functions, err := gs.GetNodesByLabel(repo, string(graph.LabelFunction))
	if err != nil {
		return nil, err
	}
	methods, _ := gs.GetNodesByLabel(repo, string(graph.LabelMethod))
	candidates := append(functions, methods...)

	// 3. Score entry points
	type scoredEntry struct {
		node  *graph.Node
		score float64
	}
	var entries []scoredEntry

	for _, node := range candidates {
		score := d.scoreEntryPoint(gs, node)
		if score > 0 {
			entries = append(entries, scoredEntry{node, score})
		}
	}

	// Sort (high score first)
	for i := 0; i < len(entries)-1; i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[j].score > entries[i].score {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}

	// 4. BFS forward tracing
	maxProcs := d.config.MaxProcesses
	if len(entries) < maxProcs {
		maxProcs = len(entries)
	}

	var processes []ProcessNode
	var steps []ProcessStep

	for i := 0; i < maxProcs && len(processes) < d.config.MaxProcesses; i++ {
		entry := entries[i]
		trace := d.bfsTrace(gs, entry.node.ID, d.config.MaxTraceDepth)

		if len(trace) < d.config.MinSteps {
			continue
		}

		// Determine process type
		procType := "intra_community"
		commSet := make(map[string]bool)
		for _, id := range trace {
			if comm, ok := nodeToComm[id]; ok {
				commSet[comm] = true
			}
		}
		if len(commSet) > 1 {
			procType = "cross_community"
		}

		procID := util.GenerateID(repo, "Process", fmt.Sprintf("proc_%d", i))
		pn := ProcessNode{
			ID:             procID,
			HeuristicLabel: d.generateLabel(gs, trace),
			ProcessType:    procType,
			StepCount:      len(trace),
			Communities:    keys(commSet),
			EntryPointID:   trace[0],
			TerminalID:     trace[len(trace)-1],
			Trace:          trace,
		}
		processes = append(processes, pn)

		// Create steps
		for stepIdx, nodeID := range trace {
			steps = append(steps, ProcessStep{
				ProcessID: procID,
				NodeID:    nodeID,
				Step:      stepIdx + 1,
			})
		}
	}

	// Statistics
	intraCount := 0
	crossCount := 0
	totalSteps := 0
	for _, p := range processes {
		if p.ProcessType == "intra_community" {
			intraCount++
		} else {
			crossCount++
		}
		totalSteps += p.StepCount
	}

	avgSteps := 0.0
	if len(processes) > 0 {
		avgSteps = float64(totalSteps) / float64(len(processes))
	}

	return &ProcessResult{
		Processes: processes,
		Steps:     steps,
		Stats: ProcessStats{
			ProcessCount:   len(processes),
			IntraCommunity: intraCount,
			CrossCommunity: crossCount,
			AvgSteps:       avgSteps,
		},
	}, nil
}

// scoreEntryPoint scores an entry point
func (d *ProcessDetector) scoreEntryPoint(gs *graph.GraphStore, node *graph.Node) float64 {
	score := 0.0

	// No internal callers → more likely to be an entry point
	inEdges, _ := gs.GetInEdges(node.ID, string(graph.RelCalls))
	if len(inEdges) == 0 {
		score += 1.0
	}

	// Framework bonus
	if node.GetPropBool("isExported") {
		score += 0.5
	}
	if node.GetPropBool("isAsync") {
		score += 0.3
	}

	// Function name heuristics
	name := node.Name
	if len(name) > 4 {
		prefix := name[:4]
		switch prefix {
		case "Hand", "hand", "Proc", "proc", "Main", "main", "Run ", "RunM":
			score += 0.5
		}
	}

	return score
}

// bfsTrace BFS forward tracing
func (d *ProcessDetector) bfsTrace(gs *graph.GraphStore, startID string, maxDepth int) []string {
	visited := make(map[string]bool)
	trace := []string{startID}
	visited[startID] = true
	currentID := startID

	for depth := 0; depth < maxDepth; depth++ {
		outEdges, _ := gs.GetOutEdges(currentID, string(graph.RelCalls))
		if len(outEdges) == 0 {
			break
		}

		// Limit branching
		branchCount := 0
		var nextID string
		for _, edge := range outEdges {
			if visited[edge.Target] {
				continue
			}
			if branchCount >= d.config.MaxBranching {
				break
			}
			branchCount++
			nextID = edge.Target
		}

		if nextID == "" || visited[nextID] {
			break
		}

		visited[nextID] = true
		trace = append(trace, nextID)
		currentID = nextID
	}

	return trace
}

// generateLabel generates process heuristic name
func (d *ProcessDetector) generateLabel(gs *graph.GraphStore, trace []string) string {
	if len(trace) == 0 {
		return "empty_process"
	}
	first, _ := gs.GetNode(trace[0])
	if first != nil {
		return first.Name + "_process"
	}
	return "process"
}

// keys returns map keys
func keys(m map[string]bool) []string {
	result := make([]string, 0, len(m))
	for k := range m {
		result = append(result, k)
	}
	return result
}