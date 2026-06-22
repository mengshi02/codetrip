package operators

import (
	"math"
	"testing"

	"github.com/mengshi02/codetrip/internal/graph"
)

// --------------- Evaluator Unit Tests ---------------

func newEval() *Evaluator {
	return NewEvaluator(nil)
}

func newEvalWithParams(params map[string]any) *Evaluator {
	return NewEvaluator(params)
}

// --- Identifier & Property Access ---

func TestEvalIdentifier(t *testing.T) {
	ev := newEval()
	row := Row{"x": 42}
	val := ev.EvalValue(&IdentifierExpr{Name: "x"}, row)
	if val != 42 {
		t.Errorf("expected 42, got %v", val)
	}
}

func TestEvalIdentifierMissing(t *testing.T) {
	ev := newEval()
	row := Row{"x": 42}
	val := ev.EvalValue(&IdentifierExpr{Name: "y"}, row)
	if val != nil {
		t.Errorf("expected nil, got %v", val)
	}
}

func TestEvalParam(t *testing.T) {
	ev := newEvalWithParams(map[string]any{"p": "hello"})
	val := ev.EvalValue(&ParameterExpr{Name: "p"}, Row{})
	if val != "hello" {
		t.Errorf("expected 'hello', got %v", val)
	}
}

func TestEvalParamMissing(t *testing.T) {
	ev := newEval()
	val := ev.EvalValue(&ParameterExpr{Name: "p"}, Row{})
	if val != nil {
		t.Errorf("expected nil, got %v", val)
	}
}

func TestEvalPropertyAccessNode(t *testing.T) {
	ev := newEval()
	node := &graph.Node{ID: "n1", Label: graph.LabelFunction, Name: "foo"}
	row := Row{"n": node}
	expr := &PropertyAccessExpr{Target: &IdentifierExpr{Name: "n"}, Prop: "name"}
	val := ev.EvalValue(expr, row)
	if val != "foo" {
		t.Errorf("expected 'foo', got %v", val)
	}
}

func TestEvalPropertyAccessEdge(t *testing.T) {
	ev := newEval()
	edge := &graph.Edge{ID: "e1", Type: graph.RelCalls, Source: "a", Target: "b"}
	row := Row{"r": edge}
	expr := &PropertyAccessExpr{Target: &IdentifierExpr{Name: "r"}, Prop: "type"}
	val := ev.EvalValue(expr, row)
	if val != "CALLS" {
		t.Errorf("expected 'CALLS', got %v", val)
	}
}

// --- Literals ---

func TestEvalStringLiteral(t *testing.T) {
	ev := newEval()
	val := ev.EvalValue(&StringLiteralExpr{Value: "hello"}, Row{})
	if val != "hello" {
		t.Errorf("expected 'hello', got %v", val)
	}
}

func TestEvalNumberLiteral(t *testing.T) {
	ev := newEval()
	val := ev.EvalValue(&NumberLiteralExpr{Value: 3.14}, Row{})
	if val != 3.14 {
		t.Errorf("expected 3.14, got %v", val)
	}
}

func TestEvalBoolLiteral(t *testing.T) {
	ev := newEval()
	if ev.EvalValue(&BooleanLiteralExpr{Value: true}, Row{}) != true {
		t.Error("expected true")
	}
	if ev.EvalValue(&BooleanLiteralExpr{Value: false}, Row{}) != false {
		t.Error("expected false")
	}
}

func TestEvalNullLiteral(t *testing.T) {
	ev := newEval()
	val := ev.EvalValue(&NullLiteralExpr{}, Row{})
	if val != nil {
		t.Errorf("expected nil, got %v", val)
	}
}

// --- Binary: comparison ---

func TestEvalEquals(t *testing.T) {
	ev := newEval()
	row := Row{"x": 42}
	expr := &BinaryExpr{Op: "=", Left: &IdentifierExpr{Name: "x"}, Right: &NumberLiteralExpr{Value: 42}}
	if !ev.EvaluateBool(expr, row) {
		t.Error("expected true for 42 = 42")
	}
}

func TestEvalNotEquals(t *testing.T) {
	ev := newEval()
	row := Row{"x": 42}
	expr := &BinaryExpr{Op: "<>", Left: &IdentifierExpr{Name: "x"}, Right: &NumberLiteralExpr{Value: 43}}
	if !ev.EvaluateBool(expr, row) {
		t.Error("expected true for 42 <> 43")
	}
}

func TestEvalComparison(t *testing.T) {
	ev := newEval()
	row := Row{"x": 10.0}

	tests := []struct {
		op   string
		rhs  float64
		want bool
	}{
		{"<", 20.0, true},
		{"<", 5.0, false},
		{">", 5.0, true},
		{">", 20.0, false},
		{"<=", 10.0, true},
		{"<=", 5.0, false},
		{">=", 10.0, true},
		{">=", 20.0, false},
	}

	for _, tt := range tests {
		expr := &BinaryExpr{Op: tt.op, Left: &IdentifierExpr{Name: "x"}, Right: &NumberLiteralExpr{Value: tt.rhs}}
		got := ev.EvaluateBool(expr, row)
		if got != tt.want {
			t.Errorf("10 %s %v = %v, want %v", tt.op, tt.rhs, got, tt.want)
		}
	}
}

// --- Binary: logical ---

func TestEvalAnd(t *testing.T) {
	ev := newEval()
	row := Row{"a": true, "b": false}
	expr := &BinaryExpr{Op: "AND", Left: &IdentifierExpr{Name: "a"}, Right: &IdentifierExpr{Name: "b"}}
	if ev.EvaluateBool(expr, row) {
		t.Error("true AND false should be false")
	}
}

func TestEvalOr(t *testing.T) {
	ev := newEval()
	row := Row{"a": true, "b": false}
	expr := &BinaryExpr{Op: "OR", Left: &IdentifierExpr{Name: "a"}, Right: &IdentifierExpr{Name: "b"}}
	if !ev.EvaluateBool(expr, row) {
		t.Error("true OR false should be true")
	}
}

// --- Binary: string ops ---

func TestEvalStartsWith(t *testing.T) {
	ev := newEval()
	row := Row{"s": "hello world"}
	expr := &BinaryExpr{Op: "STARTS WITH", Left: &IdentifierExpr{Name: "s"}, Right: &StringLiteralExpr{Value: "hello"}}
	if !ev.EvaluateBool(expr, row) {
		t.Error("expected 'hello world' STARTS WITH 'hello'")
	}
}

func TestEvalEndsWith(t *testing.T) {
	ev := newEval()
	row := Row{"s": "hello world"}
	expr := &BinaryExpr{Op: "ENDS WITH", Left: &IdentifierExpr{Name: "s"}, Right: &StringLiteralExpr{Value: "world"}}
	if !ev.EvaluateBool(expr, row) {
		t.Error("expected 'hello world' ENDS WITH 'world'")
	}
}

func TestEvalContains(t *testing.T) {
	ev := newEval()
	row := Row{"s": "hello world"}
	expr := &BinaryExpr{Op: "CONTAINS", Left: &IdentifierExpr{Name: "s"}, Right: &StringLiteralExpr{Value: "lo wo"}}
	if !ev.EvaluateBool(expr, row) {
		t.Error("expected 'hello world' CONTAINS 'lo wo'")
	}
}

func TestEvalIn(t *testing.T) {
	ev := newEval()
	row := Row{"x": 2.0}
	expr := &BinaryExpr{Op: "IN", Left: &IdentifierExpr{Name: "x"}, Right: &ListExpr{Elements: []Expr{&NumberLiteralExpr{Value: 1}, &NumberLiteralExpr{Value: 2}, &NumberLiteralExpr{Value: 3}}}}
	if !ev.EvaluateBool(expr, row) {
		t.Error("expected 2 IN [1,2,3]")
	}
}

// --- Binary: arithmetic ---

func TestEvalArithmetic(t *testing.T) {
	ev := newEval()
	row := Row{"a": 10.0, "b": 3.0}

	tests := []struct {
		op   string
		want float64
	}{
		{"+", 13.0},
		{"-", 7.0},
		{"*", 30.0},
		{"/", 10.0 / 3.0},
		{"%", math.Mod(10.0, 3.0)},
	}

	for _, tt := range tests {
		expr := &BinaryExpr{Op: tt.op, Left: &IdentifierExpr{Name: "a"}, Right: &IdentifierExpr{Name: "b"}}
		got := ev.EvalValue(expr, row)
		f, ok := ToFloat(got)
		if !ok {
			t.Errorf("op %q: expected float, got %v", tt.op, got)
			continue
		}
		if math.Abs(f-tt.want) > 1e-9 {
			t.Errorf("op %q: expected %v, got %v", tt.op, tt.want, f)
		}
	}
}

func TestEvalArithmeticDivByZero(t *testing.T) {
	ev := newEval()
	row := Row{"a": 10.0, "b": 0.0}
	expr := &BinaryExpr{Op: "/", Left: &IdentifierExpr{Name: "a"}, Right: &IdentifierExpr{Name: "b"}}
	val := ev.EvalValue(expr, row)
	if val != nil {
		t.Errorf("expected nil for div by zero, got %v", val)
	}
}

// --- Unary ---

func TestEvalNot(t *testing.T) {
	ev := newEval()
	row := Row{"x": true}
	expr := &UnaryExpr{Op: "NOT", Right: &IdentifierExpr{Name: "x"}}
	if ev.EvaluateBool(expr, row) {
		t.Error("NOT true should be false")
	}
}

func TestEvalIsNull(t *testing.T) {
	ev := newEval()
	row := Row{"x": nil}
	expr := &UnaryExpr{Op: "IS NULL", Right: &IdentifierExpr{Name: "x"}}
	if !ev.EvaluateBool(expr, row) {
		t.Error("nil IS NULL should be true")
	}
}

func TestEvalIsNotNull(t *testing.T) {
	ev := newEval()
	row := Row{"x": 42}
	expr := &UnaryExpr{Op: "IS NOT NULL", Right: &IdentifierExpr{Name: "x"}}
	if !ev.EvaluateBool(expr, row) {
		t.Error("42 IS NOT NULL should be true")
	}
}

func TestEvalNegate(t *testing.T) {
	ev := newEval()
	row := Row{"x": 5.0}
	expr := &UnaryExpr{Op: "-", Right: &IdentifierExpr{Name: "x"}}
	val := ev.EvalValue(expr, row)
	if val != -5.0 {
		t.Errorf("expected -5.0, got %v", val)
	}
}

// --- List ---

func TestEvalList(t *testing.T) {
	ev := newEval()
	expr := &ListExpr{Elements: []Expr{
		&NumberLiteralExpr{Value: 1},
		&NumberLiteralExpr{Value: 2},
		&NumberLiteralExpr{Value: 3},
	}}
	val := ev.EvalValue(expr, Row{})
	arr, ok := val.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", val)
	}
	if len(arr) != 3 {
		t.Errorf("expected 3 elements, got %d", len(arr))
	}
}

// --- CASE ---

func TestEvalCaseSimple(t *testing.T) {
	ev := newEval()
	row := Row{"x": 2.0}
	expr := &CaseExpr{
		Subject: &IdentifierExpr{Name: "x"},
		Whens: []CaseWhen{
			{Condition: &NumberLiteralExpr{Value: 1}, Result: &StringLiteralExpr{Value: "one"}},
			{Condition: &NumberLiteralExpr{Value: 2}, Result: &StringLiteralExpr{Value: "two"}},
		},
		ElseExpr: &StringLiteralExpr{Value: "other"},
	}
	val := ev.EvalValue(expr, row)
	if val != "two" {
		t.Errorf("expected 'two', got %v", val)
	}
}

func TestEvalCaseGeneric(t *testing.T) {
	ev := newEval()
	row := Row{"x": 20.0}
	expr := &CaseExpr{
		Whens: []CaseWhen{
			{Condition: &BinaryExpr{Op: ">", Left: &IdentifierExpr{Name: "x"}, Right: &NumberLiteralExpr{Value: 18}}, Result: &StringLiteralExpr{Value: "adult"}},
		},
		ElseExpr: &StringLiteralExpr{Value: "minor"},
	}
	val := ev.EvalValue(expr, row)
	if val != "adult" {
		t.Errorf("expected 'adult', got %v", val)
	}
}

// --- Functions ---

func TestEvalFunctionType(t *testing.T) {
	ev := newEval()
	edge := &graph.Edge{Type: graph.RelCalls}
	row := Row{"r": edge}
	val := ev.EvalValue(&FunctionCallExpr{Name: "type", Args: []Expr{&IdentifierExpr{Name: "r"}}}, row)
	if val != "CALLS" {
		t.Errorf("expected 'CALLS', got %v", val)
	}
}

func TestEvalFunctionLabels(t *testing.T) {
	ev := newEval()
	node := &graph.Node{Label: graph.LabelFunction}
	row := Row{"n": node}
	val := ev.EvalValue(&FunctionCallExpr{Name: "labels", Args: []Expr{&IdentifierExpr{Name: "n"}}}, row)
	arr, ok := val.([]string)
	if !ok {
		t.Fatalf("expected []string, got %T", val)
	}
	if len(arr) != 1 || arr[0] != "Function" {
		t.Errorf("expected [Function], got %v", arr)
	}
}

func TestEvalFunctionID(t *testing.T) {
	ev := newEval()
	node := &graph.Node{ID: "n1"}
	row := Row{"n": node}
	val := ev.EvalValue(&FunctionCallExpr{Name: "id", Args: []Expr{&IdentifierExpr{Name: "n"}}}, row)
	if val != "n1" {
		t.Errorf("expected 'n1', got %v", val)
	}
}

func TestEvalFunctionName(t *testing.T) {
	ev := newEval()
	node := &graph.Node{Name: "foo"}
	row := Row{"n": node}
	val := ev.EvalValue(&FunctionCallExpr{Name: "name", Args: []Expr{&IdentifierExpr{Name: "n"}}}, row)
	if val != "foo" {
		t.Errorf("expected 'foo', got %v", val)
	}
}

func TestEvalFunctionToUpper(t *testing.T) {
	ev := newEval()
	row := Row{"s": "hello"}
	val := ev.EvalValue(&FunctionCallExpr{Name: "upper", Args: []Expr{&IdentifierExpr{Name: "s"}}}, row)
	if val != "HELLO" {
		t.Errorf("expected 'HELLO', got %v", val)
	}
}

func TestEvalFunctionToLower(t *testing.T) {
	ev := newEval()
	row := Row{"s": "HELLO"}
	val := ev.EvalValue(&FunctionCallExpr{Name: "lower", Args: []Expr{&IdentifierExpr{Name: "s"}}}, row)
	if val != "hello" {
		t.Errorf("expected 'hello', got %v", val)
	}
}

func TestEvalFunctionSize(t *testing.T) {
	ev := newEval()
	row := Row{"s": "hello"}
	val := ev.EvalValue(&FunctionCallExpr{Name: "size", Args: []Expr{&IdentifierExpr{Name: "s"}}}, row)
	if val != 5.0 {
		t.Errorf("expected 5.0, got %v", val)
	}
}

func TestEvalFunctionTrim(t *testing.T) {
	ev := newEval()
	row := Row{"s": "  hi  "}
	val := ev.EvalValue(&FunctionCallExpr{Name: "trim", Args: []Expr{&IdentifierExpr{Name: "s"}}}, row)
	if val != "hi" {
		t.Errorf("expected 'hi', got %v", val)
	}
}

func TestEvalFunctionReplace(t *testing.T) {
	ev := newEval()
	row := Row{"s": "hello world"}
	val := ev.EvalValue(&FunctionCallExpr{Name: "replace", Args: []Expr{&IdentifierExpr{Name: "s"}, &StringLiteralExpr{Value: "world"}, &StringLiteralExpr{Value: "Go"}}}, row)
	if val != "hello Go" {
		t.Errorf("expected 'hello Go', got %v", val)
	}
}

func TestEvalFunctionSubstring(t *testing.T) {
	ev := newEval()
	row := Row{"s": "hello"}
	val := ev.EvalValue(&FunctionCallExpr{Name: "substring", Args: []Expr{&IdentifierExpr{Name: "s"}, &NumberLiteralExpr{Value: 1}, &NumberLiteralExpr{Value: 3}}}, row)
	if val != "ell" {
		t.Errorf("expected 'ell', got %v", val)
	}
}

func TestEvalFunctionAbs(t *testing.T) {
	ev := newEval()
	row := Row{"x": -5.0}
	val := ev.EvalValue(&FunctionCallExpr{Name: "abs", Args: []Expr{&IdentifierExpr{Name: "x"}}}, row)
	if val != 5.0 {
		t.Errorf("expected 5.0, got %v", val)
	}
}

func TestEvalFunctionRound(t *testing.T) {
	ev := newEval()
	row := Row{"x": 3.5}
	val := ev.EvalValue(&FunctionCallExpr{Name: "round", Args: []Expr{&IdentifierExpr{Name: "x"}}}, row)
	if val != 4.0 {
		t.Errorf("expected 4.0, got %v", val)
	}
}

func TestEvalFunctionCoalesce(t *testing.T) {
	ev := newEval()
	row := Row{"a": nil, "b": nil, "c": "found"}
	val := ev.EvalValue(&FunctionCallExpr{Name: "coalesce", Args: []Expr{&IdentifierExpr{Name: "a"}, &IdentifierExpr{Name: "b"}, &IdentifierExpr{Name: "c"}}}, row)
	if val != "found" {
		t.Errorf("expected 'found', got %v", val)
	}
}

func TestEvalFunctionReverse(t *testing.T) {
	ev := newEval()
	row := Row{"s": "abc"}
	val := ev.EvalValue(&FunctionCallExpr{Name: "reverse", Args: []Expr{&IdentifierExpr{Name: "s"}}}, row)
	if val != "cba" {
		t.Errorf("expected 'cba', got %v", val)
	}
}

func TestEvalFunctionExists(t *testing.T) {
	ev := newEval()
	row := Row{"x": 42}
	val := ev.EvalValue(&FunctionCallExpr{Name: "exists", Args: []Expr{&IdentifierExpr{Name: "x"}}}, row)
	if val != true {
		t.Errorf("expected true, got %v", val)
	}
}

func TestEvalFunctionHeadTailLast(t *testing.T) {
	ev := newEval()
	list := []any{1.0, 2.0, 3.0}
	row := Row{"arr": list}

	head := ev.EvalValue(&FunctionCallExpr{Name: "head", Args: []Expr{&IdentifierExpr{Name: "arr"}}}, row)
	if head != 1.0 {
		t.Errorf("head: expected 1.0, got %v", head)
	}

	last := ev.EvalValue(&FunctionCallExpr{Name: "last", Args: []Expr{&IdentifierExpr{Name: "arr"}}}, row)
	if last != 3.0 {
		t.Errorf("last: expected 3.0, got %v", last)
	}

	tail := ev.EvalValue(&FunctionCallExpr{Name: "tail", Args: []Expr{&IdentifierExpr{Name: "arr"}}}, row)
	tailArr, ok := tail.([]any)
	if !ok || len(tailArr) != 2 {
		t.Errorf("tail: expected 2 elements, got %v", tail)
	}
}

// --- ComputeAggregate ---

func TestComputeAggregateCount(t *testing.T) {
	ev := newEval()
	rows := []Row{{"x": 1}, {"x": 2}, {"x": 3}}
	expr := &FunctionCallExpr{Name: "count", Args: []Expr{&IdentifierExpr{Name: "x"}}}
	val := ev.ComputeAggregate(expr, rows)
	if val != 3 {
		t.Errorf("expected 3, got %v", val)
	}
}

func TestComputeAggregateSum(t *testing.T) {
	ev := newEval()
	rows := []Row{{"x": 1.0}, {"x": 2.0}, {"x": 3.0}}
	expr := &FunctionCallExpr{Name: "sum", Args: []Expr{&IdentifierExpr{Name: "x"}}}
	val := ev.ComputeAggregate(expr, rows)
	if val != 6.0 {
		t.Errorf("expected 6.0, got %v", val)
	}
}

func TestComputeAggregateAvg(t *testing.T) {
	ev := newEval()
	rows := []Row{{"x": 10.0}, {"x": 20.0}}
	expr := &FunctionCallExpr{Name: "avg", Args: []Expr{&IdentifierExpr{Name: "x"}}}
	val := ev.ComputeAggregate(expr, rows)
	if val != 15.0 {
		t.Errorf("expected 15.0, got %v", val)
	}
}

func TestComputeAggregateMinMax(t *testing.T) {
	ev := newEval()
	rows := []Row{{"x": 3.0}, {"x": 1.0}, {"x": 2.0}}

	minVal := ev.ComputeAggregate(&FunctionCallExpr{Name: "min", Args: []Expr{&IdentifierExpr{Name: "x"}}}, rows)
	if minVal != 1.0 {
		t.Errorf("min: expected 1.0, got %v", minVal)
	}

	maxVal := ev.ComputeAggregate(&FunctionCallExpr{Name: "max", Args: []Expr{&IdentifierExpr{Name: "x"}}}, rows)
	if maxVal != 3.0 {
		t.Errorf("max: expected 3.0, got %v", maxVal)
	}
}

func TestComputeAggregateCollect(t *testing.T) {
	ev := newEval()
	rows := []Row{{"x": "a"}, {"x": "b"}}
	expr := &FunctionCallExpr{Name: "collect", Args: []Expr{&IdentifierExpr{Name: "x"}}}
	val := ev.ComputeAggregate(expr, rows)
	arr, ok := val.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", val)
	}
	if len(arr) != 2 || arr[0] != "a" || arr[1] != "b" {
		t.Errorf("expected [a b], got %v", arr)
	}
}

func TestComputeAggregateCountStar(t *testing.T) {
	ev := newEval()
	rows := []Row{{"x": 1}, {"x": 2}}
	expr := &FunctionCallExpr{Name: "count", Args: []Expr{&IdentifierExpr{Name: "*"}}}
	val := ev.ComputeAggregate(expr, rows)
	if val != 2 {
		t.Errorf("expected 2, got %v", val)
	}
}

// --- Utility functions ---

func TestTypedEqual(t *testing.T) {
	tests := []struct {
		a, b any
		want bool
	}{
		{nil, nil, true},
		{1, nil, false},
		{nil, 1, false},
		{1.0, 1, true},  // numeric cross-type
		{"a", "a", true},
		{"a", "b", false},
		{true, true, true},
		{true, false, false},
	}

	for _, tt := range tests {
		got := TypedEqual(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("TypedEqual(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestTypedCompare(t *testing.T) {
	tests := []struct {
		a, b any
		want int
	}{
		{nil, nil, 0},
		{nil, 1, -1},
		{1, nil, 1},
		{1.0, 2.0, -1},
		{2.0, 1.0, 1},
		{1.0, 1.0, 0},
		{"a", "b", -1},
		{"b", "a", 1},
	}

	for _, tt := range tests {
		got := TypedCompare(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("TypedCompare(%v, %v) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestToFloat(t *testing.T) {
	tests := []struct {
		input any
		want  float64
		ok    bool
	}{
		{42, 42.0, true},
		{3.14, 3.14, true},
		{"3.14", 3.14, true},
		{"abc", 0, false},
		{true, 0, false},
	}

	for _, tt := range tests {
		got, ok := ToFloat(tt.input)
		if ok != tt.ok {
			t.Errorf("ToFloat(%v): ok=%v, want %v", tt.input, ok, tt.ok)
		}
		if ok && got != tt.want {
			t.Errorf("ToFloat(%v) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

// --------------- Benchmarks ---------------

func BenchmarkEvalIdentifier(b *testing.B) {
	ev := newEval()
	row := Row{"x": 42}
	expr := &IdentifierExpr{Name: "x"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ev.EvalValue(expr, row)
	}
}

func BenchmarkEvalBinaryComparison(b *testing.B) {
	ev := newEval()
	row := Row{"x": 42.0}
	expr := &BinaryExpr{Op: ">", Left: &IdentifierExpr{Name: "x"}, Right: &NumberLiteralExpr{Value: 10.0}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ev.EvaluateBool(expr, row)
	}
}

func BenchmarkEvalArithmetic(b *testing.B) {
	ev := newEval()
	row := Row{"a": 10.0, "b": 3.0}
	expr := &BinaryExpr{Op: "+", Left: &IdentifierExpr{Name: "a"}, Right: &IdentifierExpr{Name: "b"}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ev.EvalValue(expr, row)
	}
}

func BenchmarkEvalStringContains(b *testing.B) {
	ev := newEval()
	row := Row{"s": "the quick brown fox jumps over the lazy dog"}
	expr := &BinaryExpr{Op: "CONTAINS", Left: &IdentifierExpr{Name: "s"}, Right: &StringLiteralExpr{Value: "fox"}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ev.EvaluateBool(expr, row)
	}
}

func BenchmarkEvalCaseExpression(b *testing.B) {
	ev := newEval()
	row := Row{"x": 3.0}
	expr := &CaseExpr{
		Subject: &IdentifierExpr{Name: "x"},
		Whens: []CaseWhen{
			{Condition: &NumberLiteralExpr{Value: 1}, Result: &StringLiteralExpr{Value: "one"}},
			{Condition: &NumberLiteralExpr{Value: 2}, Result: &StringLiteralExpr{Value: "two"}},
			{Condition: &NumberLiteralExpr{Value: 3}, Result: &StringLiteralExpr{Value: "three"}},
			{Condition: &NumberLiteralExpr{Value: 4}, Result: &StringLiteralExpr{Value: "four"}},
		},
		ElseExpr: &StringLiteralExpr{Value: "other"},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ev.EvalValue(expr, row)
	}
}

func BenchmarkComputeAggregate(b *testing.B) {
	ev := newEval()
	rows := make([]Row, 1000)
	for i := range rows {
		rows[i] = Row{"x": float64(i % 100)}
	}
	expr := &FunctionCallExpr{Name: "avg", Args: []Expr{&IdentifierExpr{Name: "x"}}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ev.ComputeAggregate(expr, rows)
	}
}