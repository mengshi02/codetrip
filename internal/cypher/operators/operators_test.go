package operators

import (
	"testing"

	"github.com/mengshi02/codetrip/internal/graph"
)

// --------------- Mock GraphStore ---------------

type mockGraphStore struct {
	nodes []*graph.Node
	edges []*graph.Edge
	repo  string
}

func (m *mockGraphStore) GetNodesByLabel(repo string, label string) ([]*graph.Node, error) {
	var result []*graph.Node
	for _, n := range m.nodes {
		if string(n.Label) == label {
			result = append(result, n)
		}
	}
	return result, nil
}

func (m *mockGraphStore) GetAllNodes(repo string, limit int) []*graph.Node {
	if limit > len(m.nodes) {
		limit = len(m.nodes)
	}
	return m.nodes[:limit]
}

func (m *mockGraphStore) GetNode(id string) (*graph.Node, error) {
	for _, n := range m.nodes {
		if n.ID == id {
			return n, nil
		}
	}
	return nil, nil
}

func (m *mockGraphStore) GetAllOutEdges(nodeID string) ([]*graph.Edge, error) {
	var result []*graph.Edge
	for _, e := range m.edges {
		if e.Source == nodeID {
			result = append(result, e)
		}
	}
	return result, nil
}

func (m *mockGraphStore) GetAllInEdges(nodeID string) ([]*graph.Edge, error) {
	var result []*graph.Edge
	for _, e := range m.edges {
		if e.Target == nodeID {
			result = append(result, e)
		}
	}
	return result, nil
}

func (m *mockGraphStore) Repo() string { return m.repo }

func buildMockStore() *mockGraphStore {
	nodes := []*graph.Node{
		{ID: "n1", Label: graph.LabelFunction, Name: "foo", Repo: "test"},
		{ID: "n2", Label: graph.LabelFunction, Name: "bar", Repo: "test"},
		{ID: "n3", Label: graph.LabelClass, Name: "MyClass", Repo: "test"},
		{ID: "n4", Label: graph.LabelFunction, Name: "baz", Repo: "test"},
	}
	edges := []*graph.Edge{
		{ID: "e1", Type: graph.RelCalls, Source: "n1", Target: "n2"},
		{ID: "e2", Type: graph.RelCalls, Source: "n1", Target: "n3"},
		{ID: "e3", Type: graph.RelContains, Source: "n3", Target: "n4"},
	}
	return &mockGraphStore{nodes: nodes, edges: edges, repo: "test"}
}

// --------------- ScanIterator ---------------

func TestScanIteratorByLabel(t *testing.T) {
	gs := buildMockStore()
	ev := NewEvaluator(nil)
	scan := NewScanIterator(gs, &CypherNodePattern{Variable: "n", Labels: []string{"Function"}}, ev)

	var rows []Row
	for {
		row, err := scan.Next()
		if err != nil {
			t.Fatal(err)
		}
		if row == nil {
			break
		}
		rows = append(rows, row)
	}
	scan.Close()

	if len(rows) != 3 {
		t.Errorf("expected 3 Function nodes, got %d", len(rows))
	}
}

func TestScanIteratorAllNodes(t *testing.T) {
	gs := buildMockStore()
	ev := NewEvaluator(nil)
	scan := NewScanIterator(gs, &CypherNodePattern{Variable: "n", Labels: nil}, ev)

	var count int
	for {
		row, err := scan.Next()
		if err != nil {
			t.Fatal(err)
		}
		if row == nil {
			break
		}
		count++
	}
	scan.Close()

	if count != 4 {
		t.Errorf("expected 4 nodes, got %d", count)
	}
}

func TestScanIteratorNoVariable(t *testing.T) {
	gs := buildMockStore()
	ev := NewEvaluator(nil)
	scan := NewScanIterator(gs, &CypherNodePattern{Variable: "", Labels: []string{"Function"}}, ev)

	row, err := scan.Next()
	if err != nil {
		t.Fatal(err)
	}
	if row == nil {
		t.Fatal("expected a row")
	}
	if _, ok := row[""]; ok {
		t.Error("empty variable should not produce a row entry")
	}
	scan.Close()
}

// --------------- FilterIterator ---------------

func TestFilterIterator(t *testing.T) {
	gs := buildMockStore()
	ev := NewEvaluator(nil)
	scan := NewScanIterator(gs, &CypherNodePattern{Variable: "n", Labels: []string{"Function"}}, ev)
	cond := &BinaryExpr{Op: "=", Left: &PropertyAccessExpr{Target: &IdentifierExpr{Name: "n"}, Prop: "name"}, Right: &StringLiteralExpr{Value: "foo"}}
	filter := NewFilterIterator(scan, cond, ev)

	row, err := filter.Next()
	if err != nil {
		t.Fatal(err)
	}
	if row == nil {
		t.Fatal("expected one row for name=foo")
	}

	// Only one row should match
	row2, err := filter.Next()
	if err != nil {
		t.Fatal(err)
	}
	if row2 != nil {
		t.Error("expected no more rows")
	}
	filter.Close()
}

func TestFilterIteratorNoMatch(t *testing.T) {
	gs := buildMockStore()
	ev := NewEvaluator(nil)
	scan := NewScanIterator(gs, &CypherNodePattern{Variable: "n", Labels: []string{"Function"}}, ev)
	cond := &BinaryExpr{Op: "=", Left: &PropertyAccessExpr{Target: &IdentifierExpr{Name: "n"}, Prop: "name"}, Right: &StringLiteralExpr{Value: "nonexistent"}}
	filter := NewFilterIterator(scan, cond, ev)

	row, err := filter.Next()
	if err != nil {
		t.Fatal(err)
	}
	if row != nil {
		t.Error("expected no rows for nonexistent name")
	}
	filter.Close()
}

// --------------- ProjectIterator ---------------

func TestProjectIterator(t *testing.T) {
	gs := buildMockStore()
	ev := NewEvaluator(nil)
	scan := NewScanIterator(gs, &CypherNodePattern{Variable: "n", Labels: []string{"Function"}}, ev)
	items := []*ReturnItem{
		{Expr: &PropertyAccessExpr{Target: &IdentifierExpr{Name: "n"}, Prop: "name"}, Alias: "name"},
	}
	proj := NewProjectIterator(scan, items, ev, false)

	var names []string
	for {
		row, err := proj.Next()
		if err != nil {
			t.Fatal(err)
		}
		if row == nil {
			break
		}
		names = append(names, row["name"].(string))
	}
	proj.Close()

	if len(names) != 3 {
		t.Errorf("expected 3 names, got %d", len(names))
	}
}

func TestProjectIteratorDistinct(t *testing.T) {
	ev := NewEvaluator(nil)
	// Feed rows manually via a slice iterator
	rows := []Row{
		{"x": "a"},
		{"x": "b"},
		{"x": "a"},
	}
	child := &sliceIterator{rows: rows}
	items := []*ReturnItem{{Expr: &IdentifierExpr{Name: "x"}, Alias: ""}}
	proj := NewProjectIterator(child, items, ev, true)

	var results []string
	for {
		row, err := proj.Next()
		if err != nil {
			t.Fatal(err)
		}
		if row == nil {
			break
		}
		results = append(results, row["x"].(string))
	}
	proj.Close()

	if len(results) != 2 {
		t.Errorf("expected 2 distinct values, got %d", len(results))
	}
}

// --------------- JoinIterator ---------------

func TestJoinIterator(t *testing.T) {
	left := &sliceIterator{rows: []Row{{"a": 1}, {"a": 2}}}
	right := &sliceIterator{rows: []Row{{"b": "x"}, {"b": "y"}}}
	join := NewJoinIterator(left, right)

	var count int
	for {
		row, err := join.Next()
		if err != nil {
			t.Fatal(err)
		}
		if row == nil {
			break
		}
		count++
		if _, ok := row["a"]; !ok {
			t.Error("expected 'a' in joined row")
		}
		if _, ok := row["b"]; !ok {
			t.Error("expected 'b' in joined row")
		}
	}
	join.Close()

	if count != 4 { // 2 x 2 = 4
		t.Errorf("expected 4 rows from cross product, got %d", count)
	}
}

func TestHashJoinIterator(t *testing.T) {
	left := &sliceIterator{rows: []Row{{"a": 1, "x": "foo"}, {"a": 2, "x": "bar"}, {"a": 3, "x": "baz"}}}
	right := &sliceIterator{rows: []Row{{"a": 1, "y": "alpha"}, {"a": 2, "y": "beta"}, {"a": 4, "y": "gamma"}}}
	join := NewHashJoinIterator(left, right, []string{"a"})

	var rows []Row
	for {
		row, err := join.Next()
		if err != nil {
			t.Fatal(err)
		}
		if row == nil {
			break
		}
		rows = append(rows, row)
	}
	join.Close()

	// Only rows with matching "a" values: a=1 and a=2
	if len(rows) != 2 {
		t.Errorf("expected 2 rows from hash join, got %d", len(rows))
	}
	// Verify the merged columns
	for _, row := range rows {
		if _, ok := row["x"]; !ok {
			t.Error("expected 'x' in joined row")
		}
		if _, ok := row["y"]; !ok {
			t.Error("expected 'y' in joined row")
		}
	}
}

func TestHashJoinIteratorNoCommonKeys(t *testing.T) {
	left := &sliceIterator{rows: []Row{{"a": 1}, {"a": 2}}}
	right := &sliceIterator{rows: []Row{{"b": "x"}}}
	// No matching keys → should produce 0 rows
	join := NewHashJoinIterator(left, right, []string{"a"})

	var count int
	for {
		row, err := join.Next()
		if err != nil {
			t.Fatal(err)
		}
		if row == nil {
			break
		}
		count++
	}
	join.Close()

	if count != 0 {
		t.Errorf("expected 0 rows from hash join with no matching keys, got %d", count)
	}
}

func TestJoinIteratorEmptyRight(t *testing.T) {
	left := &sliceIterator{rows: []Row{{"a": 1}}}
	right := &sliceIterator{rows: []Row{}}
	join := NewJoinIterator(left, right)

	row, err := join.Next()
	if err != nil {
		t.Fatal(err)
	}
	if row != nil {
		t.Error("expected nil row for empty right side")
	}
	join.Close()
}

// --------------- SortIterator ---------------

func TestSortIterator(t *testing.T) {
	ev := NewEvaluator(nil)
	rows := []Row{
		{"x": 3.0},
		{"x": 1.0},
		{"x": 2.0},
	}
	child := &sliceIterator{rows: rows}
	orderBy := []*OrderByItem{{Expr: &IdentifierExpr{Name: "x"}, Desc: false}}
	sort := NewSortIterator(child, orderBy, ev)

	var results []float64
	for {
		row, err := sort.Next()
		if err != nil {
			t.Fatal(err)
		}
		if row == nil {
			break
		}
		results = append(results, row["x"].(float64))
	}
	sort.Close()

	if len(results) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(results))
	}
	for i, want := range []float64{1.0, 2.0, 3.0} {
		if results[i] != want {
			t.Errorf("sorted[%d] = %v, want %v", i, results[i], want)
		}
	}
}

func TestSortIteratorDesc(t *testing.T) {
	ev := NewEvaluator(nil)
	rows := []Row{{"x": 1.0}, {"x": 3.0}, {"x": 2.0}}
	child := &sliceIterator{rows: rows}
	orderBy := []*OrderByItem{{Expr: &IdentifierExpr{Name: "x"}, Desc: true}}
	sort := NewSortIterator(child, orderBy, ev)

	var results []float64
	for {
		row, err := sort.Next()
		if err != nil {
			t.Fatal(err)
		}
		if row == nil {
			break
		}
		results = append(results, row["x"].(float64))
	}
	sort.Close()

	for i, want := range []float64{3.0, 2.0, 1.0} {
		if results[i] != want {
			t.Errorf("sorted desc[%d] = %v, want %v", i, results[i], want)
		}
	}
}

// --------------- LimitIterator ---------------

func TestLimitIterator(t *testing.T) {
	rows := []Row{{"x": 1}, {"x": 2}, {"x": 3}, {"x": 4}, {"x": 5}}
	child := &sliceIterator{rows: rows}
	limit := 3
	limIt := NewLimitIterator(child, nil, &limit)

	var count int
	for {
		row, err := limIt.Next()
		if err != nil {
			t.Fatal(err)
		}
		if row == nil {
			break
		}
		count++
	}
	limIt.Close()

	if count != 3 {
		t.Errorf("expected 3 rows, got %d", count)
	}
}

func TestLimitIteratorSkipAndLimit(t *testing.T) {
	rows := []Row{{"x": 1}, {"x": 2}, {"x": 3}, {"x": 4}, {"x": 5}}
	child := &sliceIterator{rows: rows}
	skip := 2
	limit := 2
	limIt := NewLimitIterator(child, &skip, &limit)

	var results []any
	for {
		row, err := limIt.Next()
		if err != nil {
			t.Fatal(err)
		}
		if row == nil {
			break
		}
		results = append(results, row["x"])
	}
	limIt.Close()

	if len(results) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(results))
	}
	if results[0] != 3 || results[1] != 4 {
		t.Errorf("expected [3 4], got %v", results)
	}
}

// --------------- AggregateIterator ---------------

func TestAggregateIteratorCount(t *testing.T) {
	ev := NewEvaluator(nil)
	rows := []Row{{"x": 1}, {"x": 2}, {"x": 3}}
	child := &sliceIterator{rows: rows}
	items := []*ReturnItem{
		{Expr: &FunctionCallExpr{Name: "count", Args: []Expr{&IdentifierExpr{Name: "x"}}}, Alias: "cnt"},
	}
	agg := NewAggregateIterator(child, items, ev, nil, false, nil, nil)

	row, err := agg.Next()
	if err != nil {
		t.Fatal(err)
	}
	if row == nil {
		t.Fatal("expected a row")
	}
	if row["cnt"] != 3 {
		t.Errorf("expected count=3, got %v", row["cnt"])
	}
	agg.Close()
}

func TestAggregateIteratorGroupBy(t *testing.T) {
	ev := NewEvaluator(nil)
	rows := []Row{
		{"label": "A", "val": 1.0},
		{"label": "B", "val": 2.0},
		{"label": "A", "val": 3.0},
	}
	child := &sliceIterator{rows: rows}
	items := []*ReturnItem{
		{Expr: &IdentifierExpr{Name: "label"}, Alias: ""},
		{Expr: &FunctionCallExpr{Name: "sum", Args: []Expr{&IdentifierExpr{Name: "val"}}}, Alias: "total"},
	}
	agg := NewAggregateIterator(child, items, ev, nil, false, nil, nil)

	var results []Row
	for {
		row, err := agg.Next()
		if err != nil {
			t.Fatal(err)
		}
		if row == nil {
			break
		}
		results = append(results, row)
	}
	agg.Close()

	if len(results) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(results))
	}
}

// --------------- ExpandIterator ---------------

func TestExpandIteratorOutgoing(t *testing.T) {
	gs := buildMockStore()
	ev := NewEvaluator(nil)

	// Start with node n1, expand outgoing CALLS edges
	scan := NewScanIterator(gs, &CypherNodePattern{Variable: "n", Labels: []string{"Function"}}, ev)
	rel := &RelationshipPattern{
		Variable:  "r",
		RelTypes:  []string{"CALLS"},
		Direction: DirOut,
		Target:    &CypherNodePattern{Variable: "m"},
	}
	expand := NewExpandIterator(scan, rel, gs, ev)

	var count int
	for {
		row, err := expand.Next()
		if err != nil {
			t.Fatal(err)
		}
		if row == nil {
			break
		}
		count++
		// n1 has 2 outgoing CALLS edges (to n2 and n3)
		// n2 and n4 have no outgoing CALLS, n3 has no outgoing CALLS (CONTAINS != CALLS)
		if _, ok := row["r"]; !ok {
			t.Error("expected 'r' in expanded row")
		}
		if _, ok := row["m"]; !ok {
			t.Error("expected 'm' in expanded row")
		}
	}
	expand.Close()

	if count != 2 {
		t.Errorf("expected 2 expanded rows (n1->n2, n1->n3 via CALLS), got %d", count)
	}
}

func TestExpandIteratorIncoming(t *testing.T) {
	gs := buildMockStore()
	ev := NewEvaluator(nil)

	// Scan n2, expand incoming CALLS edges
	row := Row{"n": gs.nodes[1]} // n2 (bar)
	rel := &RelationshipPattern{
		Variable:  "r",
		RelTypes:  []string{"CALLS"},
		Direction: DirIn,
		Target:    &CypherNodePattern{Variable: "m"},
	}
	child := &sliceIterator{rows: []Row{row}}
	expand := NewExpandIterator(child, rel, gs, ev)

	r, err := expand.Next()
	if err != nil {
		t.Fatal(err)
	}
	if r == nil {
		t.Fatal("expected one row for incoming CALLS to n2")
	}
	expand.Close()
}

// --------------- UnwindIterator ---------------

func TestUnwindIterator(t *testing.T) {
	ev := NewEvaluator(nil)
	row := Row{"arr": []any{1.0, 2.0, 3.0}}
	child := &sliceIterator{rows: []Row{row}}
	expr := &IdentifierExpr{Name: "arr"}
	unwind := NewUnwindIterator(child, expr, "x", ev)

	var results []any
	for {
		r, err := unwind.Next()
		if err != nil {
			t.Fatal(err)
		}
		if r == nil {
			break
		}
		results = append(results, r["x"])
	}
	unwind.Close()

	if len(results) != 3 {
		t.Errorf("expected 3 unwound rows, got %d", len(results))
	}
	if results[0] != 1.0 || results[1] != 2.0 || results[2] != 3.0 {
		t.Errorf("expected [1 2 3], got %v", results)
	}
}

// --------------- sliceIterator helper ---------------

type sliceIterator struct {
	rows []Row
	idx  int
}

func (s *sliceIterator) Next() (Row, error) {
	if s.idx >= len(s.rows) {
		return nil, nil
	}
	row := s.rows[s.idx]
	s.idx++
	return row, nil
}

func (s *sliceIterator) Close() error { return nil }

// --------------- Benchmarks ---------------

func BenchmarkScanIterator(b *testing.B) {
	gs := buildMockStore()
	ev := NewEvaluator(nil)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scan := NewScanIterator(gs, &CypherNodePattern{Variable: "n", Labels: []string{"Function"}}, ev)
		for {
			row, err := scan.Next()
			if err != nil || row == nil {
				break
			}
		}
		scan.Close()
	}
}

func BenchmarkFilterIterator(b *testing.B) {
	gs := buildMockStore()
	ev := NewEvaluator(nil)
	cond := &BinaryExpr{Op: "=", Left: &PropertyAccessExpr{Target: &IdentifierExpr{Name: "n"}, Prop: "name"}, Right: &StringLiteralExpr{Value: "foo"}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scan := NewScanIterator(gs, &CypherNodePattern{Variable: "n", Labels: []string{"Function"}}, ev)
		filter := NewFilterIterator(scan, cond, ev)
		for {
			row, err := filter.Next()
			if err != nil || row == nil {
				break
			}
		}
		filter.Close()
	}
}

func BenchmarkJoinIterator(b *testing.B) {
	leftRows := make([]Row, 100)
	rightRows := make([]Row, 100)
	for i := range leftRows {
		leftRows[i] = Row{"a": i}
		rightRows[i] = Row{"b": i}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		left := &sliceIterator{rows: leftRows}
		right := &sliceIterator{rows: rightRows}
		join := NewJoinIterator(left, right)
		for {
			row, err := join.Next()
			if err != nil || row == nil {
				break
			}
		}
		join.Close()
	}
}

func BenchmarkHashJoinIterator(b *testing.B) {
	leftRows := make([]Row, 100)
	rightRows := make([]Row, 100)
	for i := range leftRows {
		leftRows[i] = Row{"a": i, "x": i}
		rightRows[i] = Row{"a": i, "y": i}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		left := &sliceIterator{rows: leftRows}
		right := &sliceIterator{rows: rightRows}
		join := NewHashJoinIterator(left, right, []string{"a"})
		for {
			row, err := join.Next()
			if err != nil || row == nil {
				break
			}
		}
		join.Close()
	}
}

func BenchmarkAggregateIterator(b *testing.B) {
	ev := NewEvaluator(nil)
	rows := make([]Row, 1000)
	for i := range rows {
		rows[i] = Row{"grp": float64(i % 10), "val": float64(i)}
	}
	items := []*ReturnItem{
		{Expr: &IdentifierExpr{Name: "grp"}, Alias: ""},
		{Expr: &FunctionCallExpr{Name: "sum", Args: []Expr{&IdentifierExpr{Name: "val"}}}, Alias: "total"},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		child := &sliceIterator{rows: rows}
		agg := NewAggregateIterator(child, items, ev, nil, false, nil, nil)
		for {
			row, err := agg.Next()
			if err != nil || row == nil {
				break
			}
		}
		agg.Close()
	}
}

func BenchmarkSortIterator(b *testing.B) {
	ev := NewEvaluator(nil)
	rows := make([]Row, 1000)
	for i := 999; i >= 0; i-- {
		rows[999-i] = Row{"x": float64(i)}
	}
	orderBy := []*OrderByItem{{Expr: &IdentifierExpr{Name: "x"}, Desc: false}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		child := &sliceIterator{rows: rows}
		sort := NewSortIterator(child, orderBy, ev)
		for {
			row, err := sort.Next()
			if err != nil || row == nil {
				break
			}
		}
		sort.Close()
	}
}

func BenchmarkExpandIterator(b *testing.B) {
	gs := buildMockStore()
	ev := NewEvaluator(nil)
	rel := &RelationshipPattern{
		Variable:  "r",
		RelTypes:  []string{"CALLS"},
		Direction: DirOut,
		Target:    &CypherNodePattern{Variable: "m"},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scan := NewScanIterator(gs, &CypherNodePattern{Variable: "n", Labels: []string{"Function"}}, ev)
		expand := NewExpandIterator(scan, rel, gs, ev)
		for {
			row, err := expand.Next()
			if err != nil || row == nil {
				break
			}
		}
		expand.Close()
	}
}

func BenchmarkUnwindIterator(b *testing.B) {
	ev := NewEvaluator(nil)
	list := make([]any, 100)
	for i := range list {
		list[i] = float64(i)
	}
	row := Row{"arr": list}
	expr := &IdentifierExpr{Name: "arr"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		child := &sliceIterator{rows: []Row{row}}
		unwind := NewUnwindIterator(child, expr, "x", ev)
		for {
			r, err := unwind.Next()
			if err != nil || r == nil {
				break
			}
		}
		unwind.Close()
	}
}
