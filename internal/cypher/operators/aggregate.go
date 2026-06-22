package operators

// fmt import removed — not needed

// AggregateIterator groups rows and computes aggregate values.
// It materializes all rows from its child, groups them by non-aggregate
// expressions, and then produces one output row per group.
type AggregateIterator struct {
	child     Iterator
	items     []*ReturnItem
	ev        *Evaluator
	orderBy   []*OrderByItem
	distinct  bool
	skip      *int
	limit     *int

	// output state
	outRows []Row
	outIdx  int
	closed  bool
}

// NewAggregateIterator creates an AggregateIterator.
func NewAggregateIterator(
	child Iterator,
	items []*ReturnItem,
	ev *Evaluator,
	orderBy []*OrderByItem,
	distinct bool,
	skip, limit *int,
) *AggregateIterator {
	return &AggregateIterator{
		child:    child,
		items:    items,
		ev:       ev,
		orderBy:  orderBy,
		distinct: distinct,
		skip:     skip,
		limit:    limit,
	}
}

// ColNames returns the output column names in order.
func (a *AggregateIterator) ColNames() []string {
	var columns []string
	for _, item := range a.items {
		colName := ExprName(item.Expr)
		if item.Alias != "" {
			colName = item.Alias
		}
		columns = append(columns, colName)
	}
	return columns
}

func (a *AggregateIterator) materialize() {
	if a.outRows != nil {
		return
	}

	// Collect all input rows
	var allRows []Row
	for {
		row, err := a.child.Next()
		if err != nil || row == nil {
			break
		}
		allRows = append(allRows, row)
	}

	// Separate group-by expressions from aggregate items
	var groupExprs []Expr
	var aggItems []*ReturnItem
	for _, item := range a.items {
		if IsAggregateExpr(item.Expr) {
			aggItems = append(aggItems, item)
		} else {
			groupExprs = append(groupExprs, item.Expr)
		}
	}

	columns := a.ColNames()

	if len(groupExprs) == 0 {
		// Whole table is one group
		row := Row{}
		for _, item := range aggItems {
			colName := ExprName(item.Expr)
			if item.Alias != "" {
				colName = item.Alias
			}
			row[colName] = a.ev.ComputeAggregate(item.Expr, allRows)
		}
		a.outRows = []Row{row}
		return
	}

	// Group by expressions
	groups := make(map[string][]Row, len(allRows)/2+1)
	groupOrder := make([]string, 0, len(allRows)/2+1)
	for _, row := range allRows {
		key := a.ev.GroupKey(groupExprs, row)
		if _, exists := groups[key]; !exists {
			groupOrder = append(groupOrder, key)
		}
		groups[key] = append(groups[key], row)
	}

	var result []Row
	for _, key := range groupOrder {
		groupRows := groups[key]
		newRow := Row{}

		// Group key values
		for _, expr := range groupExprs {
			colName := ExprName(expr)
			newRow[colName] = a.ev.EvalValue(expr, groupRows[0])
		}

		// Aggregate values
		for _, item := range aggItems {
			colName := ExprName(item.Expr)
			if item.Alias != "" {
				colName = item.Alias
			}
			newRow[colName] = a.ev.ComputeAggregate(item.Expr, groupRows)
		}

		result = append(result, newRow)
	}

	// ORDER BY
	if len(a.orderBy) > 0 {
		SortRows(result, a.orderBy, a.ev)
	}

	// DISTINCT
	if a.distinct {
		result = DeduplicateRows(result, columns)
	}

	// SKIP + LIMIT
	result = ApplySkipLimit(result, a.skip, a.limit)

	a.outRows = result
}

// Next returns the next aggregated row.
func (a *AggregateIterator) Next() (Row, error) {
	if a.closed {
		return nil, nil
	}
	a.materialize()

	if a.outIdx >= len(a.outRows) {
		return nil, nil
	}

	row := a.outRows[a.outIdx]
	a.outIdx++
	return row, nil
}

// Close releases resources held by the iterator and its child.
func (a *AggregateIterator) Close() error {
	a.closed = true
	return a.child.Close()
}

// ensure columns variable is used
var _ = ExprName