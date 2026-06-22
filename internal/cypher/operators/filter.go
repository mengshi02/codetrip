package operators

// FilterIterator filters rows from its child iterator based on a condition
// expression (WHERE clause).
type FilterIterator struct {
	child  Iterator
	cond   Expr
	ev     *Evaluator
	closed bool
}

// NewFilterIterator creates a FilterIterator that passes only rows satisfying cond.
func NewFilterIterator(child Iterator, cond Expr, ev *Evaluator) *FilterIterator {
	return &FilterIterator{
		child: child,
		cond:  cond,
		ev:    ev,
	}
}

// Next returns the next row that satisfies the filter condition.
func (f *FilterIterator) Next() (Row, error) {
	if f.closed {
		return nil, nil
	}
	for {
		row, err := f.child.Next()
		if err != nil {
			return nil, err
		}
		if row == nil {
			return nil, nil
		}
		if f.ev.EvaluateBool(f.cond, row) {
			return row, nil
		}
	}
}

// Close releases resources held by the iterator and its child.
func (f *FilterIterator) Close() error {
	f.closed = true
	return f.child.Close()
}