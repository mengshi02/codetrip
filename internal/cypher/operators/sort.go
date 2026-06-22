package operators

// SortIterator materializes all rows from its child, sorts them according
// to ORDER BY specifications, and then returns them in sorted order.
//
// When a topN limit is set (via SetTopN), only the top-N rows are kept
// during materialization, avoiding full sort of large intermediate results.
type SortIterator struct {
	child   Iterator
	orderBy []*OrderByItem
	ev      *Evaluator

	// Top-N optimization: if set, only keep the best N rows during sort.
	// This avoids O(K*log(K)) full sort when K is large and only N rows
	// are needed (e.g., ORDER BY x LIMIT 10).
	topN int

	// output state
	outRows []Row
	outIdx  int
	closed  bool
}

// NewSortIterator creates a SortIterator with the given ordering.
func NewSortIterator(child Iterator, orderBy []*OrderByItem, ev *Evaluator) *SortIterator {
	return &SortIterator{
		child:   child,
		orderBy: orderBy,
		ev:      ev,
	}
}

// SetTopN enables top-N optimization: only the first N sorted rows are kept.
// Call this when a LIMIT clause follows the ORDER BY.
func (s *SortIterator) SetTopN(n int) {
	s.topN = n
}

func (s *SortIterator) materialize() {
	if s.outRows != nil {
		return
	}

	var rows []Row
	for {
		row, err := s.child.Next()
		if err != nil || row == nil {
			break
		}
		rows = append(rows, row)

		// Top-N: periodically sort and truncate to avoid unbounded growth
		if s.topN > 0 && len(rows) > s.topN*2 {
			SortRows(rows, s.orderBy, s.ev)
			rows = rows[:s.topN]
		}
	}

	SortRows(rows, s.orderBy, s.ev)

	if s.topN > 0 && len(rows) > s.topN {
		rows = rows[:s.topN]
	}

	s.outRows = rows
}

// Next returns the next row in sorted order.
func (s *SortIterator) Next() (Row, error) {
	if s.closed {
		return nil, nil
	}
	s.materialize()

	if s.outIdx >= len(s.outRows) {
		return nil, nil
	}

	row := s.outRows[s.outIdx]
	s.outIdx++
	return row, nil
}

// Close releases resources held by the iterator and its child.
func (s *SortIterator) Close() error {
	s.closed = true
	return s.child.Close()
}

// LimitIterator applies SKIP and LIMIT to its child iterator.
// It materializes rows up to the required amount.
type LimitIterator struct {
	child Iterator
	skip  *int
	limit *int

	// state
	skipped int
	returned int
	closed  bool
}

// NewLimitIterator creates a LimitIterator with the given skip and limit.
func NewLimitIterator(child Iterator, skip, limit *int) *LimitIterator {
	return &LimitIterator{
		child: child,
		skip:  skip,
		limit: limit,
	}
}

// Next returns the next row within the skip/limit window.
func (l *LimitIterator) Next() (Row, error) {
	if l.closed {
		return nil, nil
	}

	skipN := 0
	if l.skip != nil {
		skipN = *l.skip
	}
	limitN := -1 // unlimited
	if l.limit != nil {
		limitN = *l.limit
	}

	for {
		row, err := l.child.Next()
		if err != nil {
			return nil, err
		}
		if row == nil {
			return nil, nil
		}

		// Skip rows
		if l.skipped < skipN {
			l.skipped++
			continue
		}

		// Check limit
		if limitN >= 0 && l.returned >= limitN {
			return nil, nil
		}

		l.returned++
		return row, nil
	}
}

// Close releases resources held by the iterator and its child.
func (l *LimitIterator) Close() error {
	l.closed = true
	return l.child.Close()
}