package cfg

// ReachingDefsAnalyzer is the reaching definitions analyzer
// Uses classic data flow analysis algorithm (forward propagation) to compute reachable definitions at each program point
type ReachingDefsAnalyzer struct{}

// NewReachingDefsAnalyzer creates a reaching definitions analyzer
func NewReachingDefsAnalyzer() *ReachingDefsAnalyzer {
	return &ReachingDefsAnalyzer{}
}

// Analyze performs reaching definitions analysis on FunctionCFG
// Input: FunctionCFG
// Output: map[string][]string (definition point → set of reachable block IDs)
func (a *ReachingDefsAnalyzer) Analyze(fcfg *FunctionCFG) map[string][]string {
	if fcfg == nil || len(fcfg.Blocks) == 0 {
		return make(map[string][]string)
	}

	// Build predecessor map
	predMap := make(map[string][]string, len(fcfg.Blocks))
	for _, edge := range fcfg.Edges {
		predMap[edge.To] = append(predMap[edge.To], edge.From)
	}

	// Build statement to block map
	stmtBlockMap := make(map[string]string) // statementID → blockID
	for _, block := range fcfg.Blocks {
		for _, stmtID := range block.StatementIDs {
			stmtBlockMap[stmtID] = block.ID
		}
	}

	// gen and kill sets
	// gen[B]: definitions generated in block B (variable → set of definition points)
	// kill[B]: definitions killed in block B (variable → set of killed definition points)
	type defPoint struct {
		varName string
		stmtID  string
		blockID string
	}

	// Collect all definition points
	allDefs := make(map[string][]defPoint) // varName → list of definition points
	for _, stmt := range fcfg.Statements {
		blockID := stmtBlockMap[stmt.StatementID]
		for _, varName := range stmt.Defines {
			allDefs[varName] = append(allDefs[varName], defPoint{
				varName: varName,
				stmtID:  stmt.StatementID,
				blockID: blockID,
			})
		}
	}

	// Compute gen and kill for each block
	type blockSets struct {
		gen  map[string]bool // generated definition points (stmtID)
		kill map[string]bool // killed definition points (stmtID)
	}

	blockSetsMap := make(map[string]*blockSets, len(fcfg.Blocks))
	for _, block := range fcfg.Blocks {
		bs := &blockSets{
			gen:  make(map[string]bool),
			kill: make(map[string]bool),
		}

		// Traverse statements in the block (in order)
		for _, stmtID := range block.StatementIDs {
			// Find statement facts
			var stmt *StatementFacts
			for i := range fcfg.Statements {
				if fcfg.Statements[i].StatementID == stmtID {
					stmt = &fcfg.Statements[i]
					break
				}
			}
			if stmt == nil {
				continue
			}

			// For each defined variable
			for _, varName := range stmt.Defines {
				// Kill other definitions of the same variable
				for _, dp := range allDefs[varName] {
					if dp.stmtID != stmtID {
						bs.kill[dp.stmtID] = true
					}
				}
				// Current definition is generated
				bs.gen[stmtID] = true
				// Current definition is not killed by itself
				delete(bs.kill, stmtID)
			}
		}

		blockSetsMap[block.ID] = bs
	}

	// Iterative data flow analysis (forward propagation)
	// in[B] = ∪ out[P] for all predecessors P of B
	// out[B] = gen[B] ∪ (in[B] - kill[B])

	inSets := make(map[string]map[string]bool, len(fcfg.Blocks))
	outSets := make(map[string]map[string]bool, len(fcfg.Blocks))

	// Initialize
	for _, block := range fcfg.Blocks {
		inSets[block.ID] = make(map[string]bool)
		outSets[block.ID] = make(map[string]bool)
		// Initial out = gen
		for k := range blockSetsMap[block.ID].gen {
			outSets[block.ID][k] = true
		}
	}

	// Iterate until fixpoint
	changed := true
	maxIter := 100 // Prevent infinite loop
	for changed && maxIter > 0 {
		changed = false
		maxIter--

		for _, block := range fcfg.Blocks {
			bs := blockSetsMap[block.ID]

			// Compute in[B]
			newIn := make(map[string]bool)
			for _, predID := range predMap[block.ID] {
				for k := range outSets[predID] {
					newIn[k] = true
				}
			}

			// Compute out[B] = gen[B] ∪ (in[B] - kill[B])
			newOut := make(map[string]bool)
			// gen[B]
			for k := range bs.gen {
				newOut[k] = true
			}
			// in[B] - kill[B]
			for k := range newIn {
				if !bs.kill[k] {
					newOut[k] = true
				}
			}

			// Check if changed
			if !setEqual(outSets[block.ID], newOut) {
				changed = true
				inSets[block.ID] = newIn
				outSets[block.ID] = newOut
			}
		}
	}

	// Build result: definition point → set of reachable blocks
	result := make(map[string][]string)
	for _, block := range fcfg.Blocks {
		for defPoint := range outSets[block.ID] {
			result[defPoint] = append(result[defPoint], block.ID)
		}
	}

	return result
}

// setEqual checks if two string sets are equal
func setEqual(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}