package model

// ---------------------------------------------------------------------------
// GatherAncestors -- BFS/topological-order ancestor collection
// ---------------------------------------------------------------------------

// GatherAncestors -- collect all ancestor IDs of classId (excluding classId itself),
// returned in BFS/topological order.
// Uses head-pointer BFS (avoids O(n) re-indexing from Array.shift).
// Exported for reuse by mro-processor.go (graph-level MRO emission).
func GatherAncestors(classID string, parentMap map[string][]string) []string {
	visited := make(map[string]bool)
	order := make([]string, 0)
	queue := make([]string, 0)

	// Initial queue = classID's direct parents
	parents, ok := parentMap[classID]
	if ok {
		queue = append(queue, parents...)
	}

	head := 0
	for head < len(queue) {
		id := queue[head]
		head++
		if visited[id] {
			continue
		}
		visited[id] = true
		order = append(order, id)
		grandparents, ok := parentMap[id]
		if ok {
			for _, gp := range grandparents {
				if !visited[gp] {
					queue = append(queue, gp)
				}
			}
		}
	}
	return order
}

// ---------------------------------------------------------------------------
// C3Linearize -- iterative C3 linearization
// ---------------------------------------------------------------------------

// c3Frame -- work stack frame for iterative C3.
// Phase 0 (ENTER): check cache/cycle, push parent frames
// Phase 1 (MERGE): all parent linearizations cached, execute C3 merge
type c3Frame struct {
	id    string
	phase int // 0=ENTER, 1=MERGE
}

const (
	c3Enter = 0
	c3Merge = 1
)

// C3Linearize -- compute the C3 linearization of classId.
// Returns ancestor IDs in C3 order (excluding classId itself).
// Returns nil if linearization fails (inconsistent/cyclic hierarchy).
//
// Uses an iterative stack to avoid stack overflow on deep inheritance chains (10K+ levels in Android/Java).
// cache: completed linearizations (reused across calls).
// inProgress: optional cycle detection set (reused across recursive calls).
func C3Linearize(
	classID string,
	parentMap map[string][]string,
	cache map[string][]string, // value nil = failed
	inProgress map[string]bool, // optional
) []string {
	if result, ok := cache[classID]; ok {
		return result // nil or actual result
	}

	visiting := inProgress
	if visiting == nil {
		visiting = make(map[string]bool)
	}

	stack := []c3Frame{{id: classID, phase: c3Enter}}

	for len(stack) > 0 {
		frame := &stack[len(stack)-1]

		if frame.phase == c3Enter {
			// -- ENTER phase --
			// Check cache
			if _, ok := cache[frame.id]; ok {
				stack = stack[:len(stack)-1]
				continue
			}

			// Cycle detection
			if visiting[frame.id] {
				cache[frame.id] = nil // mark as failed
				stack = stack[:len(stack)-1]
				continue
			}
			visiting[frame.id] = true

			// No parents -> empty linearization
			directParents, ok := parentMap[frame.id]
			if !ok || len(directParents) == 0 {
				visiting[frame.id] = false
				cache[frame.id] = []string{} // empty slice (not nil)
				stack = stack[:len(stack)-1]
				continue
			}

			// Switch to MERGE phase, push uncached parent frames
			frame.phase = c3Merge
			allParentsCached := true
			for i := len(directParents) - 1; i >= 0; i-- {
				pid := directParents[i]
				if _, cached := cache[pid]; !cached {
					stack = append(stack, c3Frame{id: pid, phase: c3Enter})
					allParentsCached = false
				}
			}
			if !allParentsCached {
				continue // process parent frames first
			}
			// All parents cached -> proceed directly to MERGE phase
		}

		// -- MERGE phase --
		// directParents confirmed non-empty in ENTER phase
		stack = stack[:len(stack)-1] // pop current frame

		directParents := parentMap[frame.id]

		// Build parent linearizations from cache
		parentLinearizations := make([][]string, 0, len(directParents))
		failed := false
		for _, pid := range directParents {
			pLin, ok := cache[pid]
			if !ok {
				failed = true
				break
			}
			if pLin == nil {
				failed = true
				break
			}
			// [pid, ...pLin] -- parent itself + its linearization
			merged := make([]string, 0, 1+len(pLin))
			merged = append(merged, pid)
			merged = append(merged, pLin...)
			parentLinearizations = append(parentLinearizations, merged)
		}

		if failed {
			visiting[frame.id] = false
			cache[frame.id] = nil
			continue
		}

		// C3 merge algorithm
		// sequences = parentLinearizations + [directParents]
		sequences := make([][]string, len(parentLinearizations)+1)
		copy(sequences, parentLinearizations)
		sequences[len(parentLinearizations)] = directParents

		heads := make([]int, len(sequences)) // head pointer for each sequence
		result := make([]string, 0)

		// tailCount: O(1) membership check replacing O(n) indexOf
		tailCount := make(map[string]int)
		for _, seq := range sequences {
			for i := 1; i < len(seq); i++ {
				tailCount[seq[i]]++
			}
		}

		remaining := 0
		for _, s := range sequences {
			remaining += len(s)
		}

		inconsistent := false

		for remaining > 0 {
			var head string
			found := false

			// Find the first candidate not in any sequence's tail
			for si := 0; si < len(sequences); si++ {
				if heads[si] >= len(sequences[si]) {
					continue
				}
				candidate := sequences[si][heads[si]]
				if tailCount[candidate] == 0 {
					head = candidate
					found = true
					break
				}
			}

			if !found {
				inconsistent = true
				break
			}

			result = append(result, head)

			// Advance head pointer for all sequences containing head
			for si := 0; si < len(sequences); si++ {
				if heads[si] >= len(sequences[si]) {
					continue
				}
				if sequences[si][heads[si]] == head {
					heads[si]++
					remaining--
					// Newly promoted head from tail -> decrement tailCount
					if heads[si] < len(sequences[si]) {
						promoted := sequences[si][heads[si]]
						prev := tailCount[promoted]
						if prev <= 1 {
							delete(tailCount, promoted)
						} else {
							tailCount[promoted] = prev - 1
						}
					}
				}
			}
		}

		visiting[frame.id] = false
		if inconsistent {
			cache[frame.id] = nil
		} else {
			cache[frame.id] = result
		}
	}

	result, ok := cache[classID]
	if !ok {
		return nil
	}
	return result
}
