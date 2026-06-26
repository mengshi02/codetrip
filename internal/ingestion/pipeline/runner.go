// Pipeline runner — executes phases in dependency order.
//
// Uses Kahn's topological sort to determine execution order.
// Each phase receives typed outputs from its upstream dependencies.
//
// The runner is intentionally simple:
//   - No dynamic phase loading
//   - No plugin system
//   - Static phase graph
//   - Sequential execution (parallel support is architecturally possible
//     but most phases have linear dependencies)
//
// Ported from gitnexus pipeline-phases/runner.ts (229 lines).
package pipeline

import (
	"fmt"
	"sort"
	"time"
)

// ── Topological sort ───────────────────────────────────────────────────────

// topologicalSort validates that the phases form a valid dependency graph
// (no cycles, all deps present) and returns phases in topological execution order.
func topologicalSort(phases []PipelinePhase) ([]PipelinePhase, error) {
	phaseMap := make(map[string]PipelinePhase, len(phases))
	for _, phase := range phases {
		if _, exists := phaseMap[phase.Name()]; exists {
			return nil, fmt.Errorf("duplicate phase name: %q", phase.Name())
		}
		phaseMap[phase.Name()] = phase
	}

	// Validate all deps exist
	for _, phase := range phases {
		for _, dep := range phase.Deps() {
			if _, exists := phaseMap[dep]; !exists {
				return nil, fmt.Errorf("phase %q depends on %q, which is not registered", phase.Name(), dep)
			}
		}
	}

	// Kahn's algorithm
	inDegree := make(map[string]int, len(phases))
	reverseDeps := make(map[string][]string)

	for _, phase := range phases {
		inDegree[phase.Name()] = len(phase.Deps())
		for _, dep := range phase.Deps() {
			reverseDeps[dep] = append(reverseDeps[dep], phase.Name())
		}
	}

	var queue []string
	for name, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, name)
		}
	}
	// Sort the initial queue for deterministic order among zero-indegree nodes
	sort.Strings(queue)

	sorted := make([]PipelinePhase, 0, len(phases))
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		sorted = append(sorted, phaseMap[name])

		dependents := reverseDeps[name]
		// Sort dependents for deterministic ordering
		sort.Strings(dependents)
		for _, dependent := range dependents {
			newDeg := inDegree[dependent] - 1
			inDegree[dependent] = newDeg
			if newDeg == 0 {
				queue = append(queue, dependent)
				sort.Strings(queue) // maintain sorted order
			}
		}
	}

	if len(sorted) != len(phases) {
		remaining := make(map[string]bool)
		for name, deg := range inDegree {
			if deg > 0 {
				remaining[name] = true
			}
		}
		cyclePath := findCyclePath(remaining, phaseMap)
		return nil, fmt.Errorf("cycle detected in pipeline phases: %v", cyclePath)
	}

	return sorted, nil
}

// findCyclePath finds a concrete cycle path among the phases that Kahn's
// algorithm could not drain. DFS over the leftovers until a back-edge is
// found. Returns the cycle in order with the entry node repeated at the end.
func findCyclePath(remaining map[string]bool, phaseMap map[string]PipelinePhase) []string {
	for start := range remaining {
		var stack []string
		onStack := make(map[string]bool)
		visited := make(map[string]bool)

		var dfs func(name string) []string
		dfs = func(name string) []string {
			stack = append(stack, name)
			onStack[name] = true
			visited[name] = true

			phase := phaseMap[name]
			if phase != nil {
				for _, dep := range phase.Deps() {
					if !remaining[dep] {
						continue // dep already drained — not part of cycle
					}
					if onStack[dep] {
						// Back-edge — slice from first occurrence and close the loop
						cycleStart := -1
						for i, s := range stack {
							if s == dep {
								cycleStart = i
								break
							}
						}
						result := make([]string, len(stack)-cycleStart)
						copy(result, stack[cycleStart:])
						result = append(result, dep)
						return result
					}
					if !visited[dep] {
						if found := dfs(dep); found != nil {
							return found
						}
					}
				}
			}

			stack = stack[:len(stack)-1]
			delete(onStack, name)
			return nil
		}

		if cycle := dfs(start); cycle != nil {
			return cycle
		}
	}

	// Fallback: return sorted remaining names
	var names []string
	for name := range remaining {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ── Pipeline execution ─────────────────────────────────────────────────────

// RunPipeline executes a set of pipeline phases in dependency order.
//
// phases: all phases to execute (order doesn't matter — sorted internally)
// ctx:    shared pipeline context
// Returns: map of phase name → PhaseResult (all completed phases)
func RunPipeline(phases []PipelinePhase, ctx *PipelineContext) (map[string]*PhaseResult, error) {
	sorted, err := topologicalSort(phases)
	if err != nil {
		// Emit a terminal error progress event for graph-validation failures
		if ctx.OnProgress != nil {
			ctx.OnProgress("error", 100, fmt.Sprintf("Pipeline graph validation failed: %v", err))
		}
		return nil, err
	}

	results := make(map[string]*PhaseResult, len(sorted))

	for _, phase := range sorted {
		start := time.Now()

		// Only expose declared dependencies — prevents hidden coupling to undeclared phases
		declaredDeps := make(map[string]*PhaseResult, len(phase.Deps()))
		for _, depName := range phase.Deps() {
			if depResult, ok := results[depName]; ok {
				declaredDeps[depName] = depResult
			}
		}

		output, err := phase.Execute(ctx, declaredDeps)
		if err != nil {
			wrapped := fmt.Errorf("phase %q failed: %w", phase.Name(), err)

			// Emit a terminal error progress event
			if ctx.OnProgress != nil {
				ctx.OnProgress("error", 100, fmt.Sprintf("Phase %q failed: %v", phase.Name(), err))
			}

			return nil, wrapped
		}

		durationMs := time.Since(start).Milliseconds()
		results[phase.Name()] = &PhaseResult{
			PhaseName:  phase.Name(),
			Output:     output,
			DurationMs: durationMs,
		}
	}

	return results, nil
}