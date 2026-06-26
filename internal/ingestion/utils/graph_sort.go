package utils

// IndependentFileGroup is a group of files with no mutual dependencies,
// safe to process in parallel.
type IndependentFileGroup []string

// TopologicalLevelSortResult holds the sorted levels and cycle count.
type TopologicalLevelSortResult struct {
	Levels     []IndependentFileGroup
	CycleCount int
}

// TopologicalLevelSort groups files by topological level using Kahn's
// algorithm on the **reverse** import graph.
//
// Files in the same level have no mutual dependencies — safe to process
// in parallel. Files involved in import cycles are appended as a final
// level and processed last in an undefined order (best-effort propagation).
//
// ## Why the counter is named pendingImportsPerFile (not inDegree)
//
// Cross-file binding propagation must process **leaves first** — a file's
// imports must be resolved before the file itself is re-resolved. To get
// leaves first from Kahn's algorithm, we run Kahn's on the **reverse** of
// the import graph:
//
// - importMap is importer → {imports} (forward edges point at deps).
// - The reverse graph has edges dep → {importers}, materialized in
//   reverseDeps.
// - On the reverse graph, "in-degree of node X" equals "number of imports X
//   has in the forward graph" — i.e. X's forward out-degree.
//
// So pendingImportsPerFile[file] counts how many of file's imports
// are still un-emitted. A file is ready (level 0) once all its imports
// have been emitted in earlier levels — that is, once its pending-imports
// count drops to 0.
//
// **Do not rename this back to inDegree and do not invert the counting
// direction.** Doing either flips the emission order from leaves-first to
// roots-first, which silently breaks cross-file binding propagation.
func TopologicalLevelSort(importMap map[string]map[string]bool) TopologicalLevelSortResult {
	// Per-file count of imports not yet emitted in an earlier level.
	pendingImportsPerFile := make(map[string]int)
	reverseDeps := make(map[string][]string)

	// Initialize counters
	for file, deps := range importMap {
		if _, exists := pendingImportsPerFile[file]; !exists {
			pendingImportsPerFile[file] = 0
		}
		for dep := range deps {
			if _, exists := pendingImportsPerFile[dep]; !exists {
				pendingImportsPerFile[dep] = 0
			}
			pendingImportsPerFile[file]++
			reverseDeps[dep] = append(reverseDeps[dep], file)
		}
	}

	var levels []IndependentFileGroup

	// Level 0: files with no un-emitted imports (true leaves).
	var currentLevel IndependentFileGroup
	for file, count := range pendingImportsPerFile {
		if count == 0 {
			currentLevel = append(currentLevel, file)
		}
	}

	for len(currentLevel) > 0 {
		levels = append(levels, currentLevel)
		var nextLevel IndependentFileGroup
		for _, file := range currentLevel {
			// For each importer of file, one of its pending imports just got
			// emitted — decrement. If it hits 0, the importer is ready.
			for _, dependent := range reverseDeps[file] {
				pendingImportsPerFile[dependent]--
				if pendingImportsPerFile[dependent] == 0 {
					nextLevel = append(nextLevel, dependent)
				}
			}
		}
		currentLevel = nextLevel
	}

	// Anything still > 0 participates in a cycle.
	var cycleFiles IndependentFileGroup
	for file, count := range pendingImportsPerFile {
		if count > 0 {
			cycleFiles = append(cycleFiles, file)
		}
	}
	cycleCount := len(cycleFiles)
	if cycleCount > 0 {
		levels = append(levels, cycleFiles)
	}

	return TopologicalLevelSortResult{
		Levels:     levels,
		CycleCount: cycleCount,
	}
}