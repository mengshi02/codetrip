package process

// ProcessNode represents a process node
type ProcessNode struct {
	ID             string
	HeuristicLabel string
	ProcessType    string // intra_community | cross_community
	StepCount      int
	Communities    []string
	EntryPointID   string
	TerminalID     string
	Trace          []string // ordered node ID path
}

// ProcessStep represents a process step
type ProcessStep struct {
	ProcessID string
	NodeID    string
	Step      int // 1-indexed
}

// ProcessResult represents process detection result
type ProcessResult struct {
	Processes []ProcessNode
	Steps     []ProcessStep
	Stats     ProcessStats
}

// ProcessStats represents process detection statistics
type ProcessStats struct {
	ProcessCount       int
	IntraCommunity     int
	CrossCommunity     int
	AvgSteps           float64
	TotalTracedSymbols int
}

// ProcessConfig represents process detection configuration
type ProcessConfig struct {
	MaxTraceDepth int // maximum trace depth (default 10)
	MaxBranching  int // maximum branches per node (default 4)
	MaxProcesses  int // maximum process count: max(20, min(300, symbolCount/10))
	MinSteps      int // minimum step count (default 3)
}