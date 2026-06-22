package operators

// ProjectIterator projects (transforms) each row from its child iterator
// according to the RETURN clause items. It also handles DISTINCT elimination.
type ProjectIterator struct {
	child    Iterator
	items    []*ReturnItem
	ev       *Evaluator
	distinct bool
	seen     map[string]bool
	closed   bool
}

// NewProjectIterator creates a ProjectIterator for the given return items.
func NewProjectIterator(child Iterator, items []*ReturnItem, ev *Evaluator, distinct bool) *ProjectIterator {
	return &ProjectIterator{
		child:    child,
		items:    items,
		ev:       ev,
		distinct: distinct,
		seen:     make(map[string]bool),
	}
}

// ColNames returns the projected column names in order.
func (p *ProjectIterator) ColNames() []string {
	var columns []string
	for _, item := range p.items {
		colName := ExprName(item.Expr)
		if item.Alias != "" {
			colName = item.Alias
		}
		columns = append(columns, colName)
	}
	return columns
}

// Next returns the next projected row.
func (p *ProjectIterator) Next() (Row, error) {
	if p.closed {
		return nil, nil
	}
	for {
		row, err := p.child.Next()
		if err != nil {
			return nil, err
		}
		if row == nil {
			return nil, nil
		}

		newRow := Row{}
		for _, item := range p.items {
			colName := ExprName(item.Expr)
			if item.Alias != "" {
				colName = item.Alias
			}
			newRow[colName] = p.ev.EvalValue(item.Expr, row)
		}

		if p.distinct {
			columns := p.ColNames()
			key := RowKey(newRow, columns)
			if p.seen[key] {
				continue
			}
			p.seen[key] = true
		}

		return newRow, nil
	}
}

// Close releases resources held by the iterator and its child.
func (p *ProjectIterator) Close() error {
	p.closed = true
	return p.child.Close()
}