// Package operators implements the Volcano iterator model for Cypher query execution.
//
// Each operator implements the Iterator interface with Next()/Close() methods,
// forming a tree of iterators that can be pulled from the root to produce results.
package operators

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/mengshi02/codetrip/internal/graph"
)

// Row represents a single result row as a map of variable names to values.
type Row map[string]any

// rowPool reuses Row maps to reduce GC pressure during query execution.
var rowPool = sync.Pool{
	New: func() any {
		return make(Row, 8)
	},
}

// NewRowFromPool creates a Row from the sync.Pool.
func NewRowFromPool() Row {
	r := rowPool.Get().(Row)
	// Clear any stale entries
	for k := range r {
		delete(r, k)
	}
	return r
}

// ReleaseRow returns a Row to the pool.
func ReleaseRow(r Row) {
	if r != nil {
		rowPool.Put(r)
	}
}

// Result represents the complete query result.
type Result struct {
	Rows    []Row
	Columns []string
}

// Iterator is the core interface of the Volcano model.
// Each call to Next() produces one row; when no more rows are available
// it returns nil, nil. Close() releases any held resources.
type Iterator interface {
	Next() (Row, error)
	Close() error
}

// Evaluator provides expression evaluation shared across all operators.
// It is intentionally kept as a concrete struct (not an interface) because
// every operator needs the exact same evaluation logic.
type Evaluator struct {
	Params map[string]any
}

// NewEvaluator creates an Evaluator with the given query parameters.
func NewEvaluator(params map[string]any) *Evaluator {
	return &Evaluator{Params: params}
}

// ---------- expression evaluation ----------

// EvaluateBool evaluates an expression and returns its boolean value.
func (ev *Evaluator) EvaluateBool(expr Expr, row Row) bool {
	val := ev.EvalValue(expr, row)
	switch v := val.(type) {
	case bool:
		return v
	case nil:
		return false
	default:
		return v != nil
	}
}

// EvalValue evaluates an expression and returns its value.
func (ev *Evaluator) EvalValue(expr Expr, row Row) any {
	switch ex := expr.(type) {
	case *IdentifierExpr:
		if val, ok := row[ex.Name]; ok {
			return nodeToValue(val)
		}
		if val, ok := ev.Params[ex.Name]; ok {
			return val
		}
		return nil

	case *PropertyAccessExpr:
		target := ev.EvalValue(ex.Target, row)
		switch t := target.(type) {
		case *graph.Node:
			return GetNodeProp(t, ex.Prop)
		case *graph.Edge:
			return GetEdgeProp(t, ex.Prop)
		case map[string]any:
			return t[ex.Prop]
		}
		return nil

	case *StringLiteralExpr:
		return ex.Value

	case *NumberLiteralExpr:
		return ex.Value

	case *BooleanLiteralExpr:
		return ex.Value

	case *NullLiteralExpr:
		return nil

	case *ParameterExpr:
		if val, ok := ev.Params[ex.Name]; ok {
			return val
		}
		return nil

	case *BinaryExpr:
		return ev.evalBinary(ex, row)

	case *UnaryExpr:
		return ev.evalUnary(ex, row)

	case *FunctionCallExpr:
		return ev.evalFunction(ex, row)

	case *ListExpr:
		result := make([]any, len(ex.Elements))
		for i, elem := range ex.Elements {
			result[i] = ev.EvalValue(elem, row)
		}
		return result

	case *CaseExpr:
		return ev.evalCase(ex, row)

	default:
		return nil
	}
}

func (ev *Evaluator) evalBinary(expr *BinaryExpr, row Row) any {
	switch expr.Op {
	case "AND":
		return ev.EvaluateBool(expr.Left, row) && ev.EvaluateBool(expr.Right, row)
	case "OR":
		return ev.EvaluateBool(expr.Left, row) || ev.EvaluateBool(expr.Right, row)
	case "=":
		left := ev.EvalValue(expr.Left, row)
		right := ev.EvalValue(expr.Right, row)
		return TypedEqual(left, right)
	case "<>":
		left := ev.EvalValue(expr.Left, row)
		right := ev.EvalValue(expr.Right, row)
		return !TypedEqual(left, right)
	case "<":
		left := ev.EvalValue(expr.Left, row)
		right := ev.EvalValue(expr.Right, row)
		return TypedCompare(left, right) < 0
	case ">":
		left := ev.EvalValue(expr.Left, row)
		right := ev.EvalValue(expr.Right, row)
		return TypedCompare(left, right) > 0
	case "<=":
		left := ev.EvalValue(expr.Left, row)
		right := ev.EvalValue(expr.Right, row)
		return TypedCompare(left, right) <= 0
	case ">=":
		left := ev.EvalValue(expr.Left, row)
		right := ev.EvalValue(expr.Right, row)
		return TypedCompare(left, right) >= 0
	case "+":
		left := ev.EvalValue(expr.Left, row)
		right := ev.EvalValue(expr.Right, row)
		return typedArith(left, right, "+")
	case "-":
		left := ev.EvalValue(expr.Left, row)
		right := ev.EvalValue(expr.Right, row)
		return typedArith(left, right, "-")
	case "*":
		left := ev.EvalValue(expr.Left, row)
		right := ev.EvalValue(expr.Right, row)
		return typedArith(left, right, "*")
	case "/":
		left := ev.EvalValue(expr.Left, row)
		right := ev.EvalValue(expr.Right, row)
		return typedArith(left, right, "/")
	case "%":
		left := ev.EvalValue(expr.Left, row)
		right := ev.EvalValue(expr.Right, row)
		return typedArith(left, right, "%")
	case "STARTS WITH":
		left := toString(ev.EvalValue(expr.Left, row))
		right := toString(ev.EvalValue(expr.Right, row))
		return strings.HasPrefix(left, right)
	case "ENDS WITH":
		left := toString(ev.EvalValue(expr.Left, row))
		right := toString(ev.EvalValue(expr.Right, row))
		return strings.HasSuffix(left, right)
	case "CONTAINS":
		left := toString(ev.EvalValue(expr.Left, row))
		right := toString(ev.EvalValue(expr.Right, row))
		return strings.Contains(left, right)
	case "IN":
		left := ev.EvalValue(expr.Left, row)
		right := ev.EvalValue(expr.Right, row)
		return typedIn(left, right)
	default:
		return nil
	}
}

func (ev *Evaluator) evalUnary(expr *UnaryExpr, row Row) any {
	switch expr.Op {
	case "NOT":
		return !ev.EvaluateBool(expr.Right, row)
	case "IS NULL":
		return ev.EvalValue(expr.Right, row) == nil
	case "IS NOT NULL":
		return ev.EvalValue(expr.Right, row) != nil
	case "-":
		val := ev.EvalValue(expr.Right, row)
		if f, ok := ToFloat(val); ok {
			return -f
		}
		return nil
	default:
		return nil
	}
}

func (ev *Evaluator) evalCase(expr *CaseExpr, row Row) any {
	if expr.Subject != nil {
		// Simple CASE: CASE x WHEN a THEN 1 WHEN b THEN 2 ELSE 3 END
		subjectVal := ev.EvalValue(expr.Subject, row)
		for _, w := range expr.Whens {
			whenVal := ev.EvalValue(w.Condition, row)
			if TypedEqual(subjectVal, whenVal) {
				return ev.EvalValue(w.Result, row)
			}
		}
	} else {
		// Generic CASE: CASE WHEN cond1 THEN 1 WHEN cond2 THEN 2 ELSE 3 END
		for _, w := range expr.Whens {
			if ev.EvaluateBool(w.Condition, row) {
				return ev.EvalValue(w.Result, row)
			}
		}
	}
	if expr.ElseExpr != nil {
		return ev.EvalValue(expr.ElseExpr, row)
	}
	return nil
}

func (ev *Evaluator) evalFunction(expr *FunctionCallExpr, row Row) any {
	switch strings.ToUpper(expr.Name) {
	case "COUNT":
		return nil // handled by aggregate
	case "SUM", "AVG", "MIN", "MAX", "COLLECT":
		return nil // handled by aggregate
	case "TYPE":
		if len(expr.Args) > 0 {
			val := ev.EvalValue(expr.Args[0], row)
			if edge, ok := val.(*graph.Edge); ok {
				return string(edge.Type)
			}
		}
	case "LABELS":
		if len(expr.Args) > 0 {
			val := ev.EvalValue(expr.Args[0], row)
			if node, ok := val.(*graph.Node); ok {
				return []string{string(node.Label)}
			}
		}
	case "ID":
		if len(expr.Args) > 0 {
			val := ev.EvalValue(expr.Args[0], row)
			if node, ok := val.(*graph.Node); ok {
				return node.ID
			}
		}
	case "NAME":
		if len(expr.Args) > 0 {
			val := ev.EvalValue(expr.Args[0], row)
			if node, ok := val.(*graph.Node); ok {
				return node.Name
			}
		}
	case "TOFLOAT":
		if len(expr.Args) > 0 {
			val := ev.EvalValue(expr.Args[0], row)
			if f, err := strconv.ParseFloat(toString(val), 64); err == nil {
				return f
			}
		}
	case "TOSTRING":
		if len(expr.Args) > 0 {
			return toString(ev.EvalValue(expr.Args[0], row))
		}
	case "TOINTEGER":
		if len(expr.Args) > 0 {
			val := ev.EvalValue(expr.Args[0], row)
			if f, ok := ToFloat(val); ok {
				return f // keep as float64 for consistency, truncate if needed
			}
		}
	case "TRIM":
		if len(expr.Args) > 0 {
			return strings.TrimSpace(toString(ev.EvalValue(expr.Args[0], row)))
		}
	case "REPLACE":
		if len(expr.Args) >= 3 {
			s := toString(ev.EvalValue(expr.Args[0], row))
			old := toString(ev.EvalValue(expr.Args[1], row))
			newS := toString(ev.EvalValue(expr.Args[2], row))
			return strings.ReplaceAll(s, old, newS)
		}
	case "SUBSTRING":
		if len(expr.Args) >= 2 {
			s := toString(ev.EvalValue(expr.Args[0], row))
			start, _ := ToFloat(ev.EvalValue(expr.Args[1], row))
			if len(expr.Args) >= 3 {
				length, _ := ToFloat(ev.EvalValue(expr.Args[2], row))
				end := int(start) + int(length)
				if end > len(s) {
					end = len(s)
				}
				if int(start) >= len(s) {
					return ""
				}
				return s[int(start):end]
			}
			if int(start) >= len(s) {
				return ""
			}
			return s[int(start):]
		}
	case "SIZE":
		if len(expr.Args) > 0 {
			val := ev.EvalValue(expr.Args[0], row)
			switch v := val.(type) {
			case string:
				return float64(len(v))
			case []any:
				return float64(len(v))
			}
		}
	case "LENGTH":
		if len(expr.Args) > 0 {
			val := ev.EvalValue(expr.Args[0], row)
			if s, ok := val.(string); ok {
				return float64(len(s))
			}
			if arr, ok := val.([]any); ok {
				return float64(len(arr))
			}
		}
	case "UPPER", "UCASE":
		if len(expr.Args) > 0 {
			return strings.ToUpper(toString(ev.EvalValue(expr.Args[0], row)))
		}
	case "LOWER", "LCASE":
		if len(expr.Args) > 0 {
			return strings.ToLower(toString(ev.EvalValue(expr.Args[0], row)))
		}
	case "HEAD":
		if len(expr.Args) > 0 {
			val := ev.EvalValue(expr.Args[0], row)
			if arr, ok := val.([]any); ok && len(arr) > 0 {
				return arr[0]
			}
		}
	case "LAST":
		if len(expr.Args) > 0 {
			val := ev.EvalValue(expr.Args[0], row)
			if arr, ok := val.([]any); ok && len(arr) > 0 {
				return arr[len(arr)-1]
			}
		}
	case "TAIL":
		if len(expr.Args) > 0 {
			val := ev.EvalValue(expr.Args[0], row)
			if arr, ok := val.([]any); ok && len(arr) > 1 {
				return arr[1:]
			}
		}
	case "REVERSE":
		if len(expr.Args) > 0 {
			val := ev.EvalValue(expr.Args[0], row)
			switch v := val.(type) {
			case string:
				runes := []rune(v)
				for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
					runes[i], runes[j] = runes[j], runes[i]
				}
				return string(runes)
			case []any:
				result := make([]any, len(v))
				for i, j := 0, len(v)-1; j >= 0; i, j = i+1, j-1 {
					result[i] = v[j]
				}
				return result
			}
		}
	case "COALESCE":
		for _, arg := range expr.Args {
			val := ev.EvalValue(arg, row)
			if val != nil {
				return val
			}
		}
		return nil
	case "ABS":
		if len(expr.Args) > 0 {
			if f, ok := ToFloat(ev.EvalValue(expr.Args[0], row)); ok {
				return math.Abs(f)
			}
		}
	case "ROUND":
		if len(expr.Args) > 0 {
			if f, ok := ToFloat(ev.EvalValue(expr.Args[0], row)); ok {
				return math.Round(f)
			}
		}
	case "CEIL", "CEILING":
		if len(expr.Args) > 0 {
			if f, ok := ToFloat(ev.EvalValue(expr.Args[0], row)); ok {
				return math.Ceil(f)
			}
		}
	case "FLOOR":
		if len(expr.Args) > 0 {
			if f, ok := ToFloat(ev.EvalValue(expr.Args[0], row)); ok {
				return math.Floor(f)
			}
		}
	case "SQRT":
		if len(expr.Args) > 0 {
			if f, ok := ToFloat(ev.EvalValue(expr.Args[0], row)); ok {
				return math.Sqrt(f)
			}
		}
	case "EXISTS":
		if len(expr.Args) > 0 {
			val := ev.EvalValue(expr.Args[0], row)
			return val != nil
		}
	}
	return nil
}

func (ev *Evaluator) collectFunctionArgs(fn *FunctionCallExpr, row Row) []any {
	args := make([]any, 0)
	for _, arg := range fn.Args {
		args = append(args, ev.EvalValue(arg, row))
	}
	return args
}

// ---------- aggregate helpers ----------

// ComputeAggregate computes an aggregate value over rows.
func (ev *Evaluator) ComputeAggregate(expr Expr, rows []Row) any {
	fn, ok := expr.(*FunctionCallExpr)
	if !ok {
		return ev.EvalValue(expr, rows[0])
	}

	values := make([]any, 0, len(rows))
	for _, row := range rows {
		if len(fn.Args) > 0 {
			if ident, ok := fn.Args[0].(*IdentifierExpr); ok && ident.Name == "*" {
				values = append(values, 1) // COUNT(*)
			} else {
				values = append(values, ev.EvalValue(fn.Args[0], row))
			}
		}
	}

	switch strings.ToUpper(fn.Name) {
	case "COUNT":
		return len(values)
	case "SUM":
		sum := 0.0
		for _, v := range values {
			if f, ok := ToFloat(v); ok {
				sum += f
			}
		}
		return sum
	case "AVG":
		sum := 0.0
		count := 0
		for _, v := range values {
			if f, ok := ToFloat(v); ok {
				sum += f
				count++
			}
		}
		if count == 0 {
			return nil
		}
		return sum / float64(count)
	case "MIN":
		minVal := math.MaxFloat64
		for _, v := range values {
			if f, ok := ToFloat(v); ok && f < minVal {
				minVal = f
			}
		}
		if minVal == math.MaxFloat64 {
			return nil
		}
		return minVal
	case "MAX":
		maxVal := -math.MaxFloat64
		for _, v := range values {
			if f, ok := ToFloat(v); ok && f > maxVal {
				maxVal = f
			}
		}
		if maxVal == -math.MaxFloat64 {
			return nil
		}
		return maxVal
	case "COLLECT":
		return values
	}
	return nil
}

// IsAggregateExpr returns true if the expression is an aggregate function call.
func IsAggregateExpr(expr Expr) bool {
	if fn, ok := expr.(*FunctionCallExpr); ok {
		switch strings.ToUpper(fn.Name) {
		case "COUNT", "SUM", "AVG", "MIN", "MAX", "COLLECT":
			return true
		}
	}
	return false
}

// HasAggregate returns true if any return item contains an aggregate expression.
func HasAggregate(items []*ReturnItem) bool {
	for _, item := range items {
		if IsAggregateExpr(item.Expr) {
			return true
		}
	}
	return false
}

// ExprName returns a human-readable name for an expression.
func ExprName(expr Expr) string {
	switch ex := expr.(type) {
	case *IdentifierExpr:
		return ex.Name
	case *PropertyAccessExpr:
		return ExprName(ex.Target) + "." + ex.Prop
	case *FunctionCallExpr:
		args := make([]string, len(ex.Args))
		for i, a := range ex.Args {
			args[i] = ExprName(a)
		}
		return fmt.Sprintf("%s(%s)", ex.Name, strings.Join(args, ", "))
	default:
		return fmt.Sprintf("%v", expr)
	}
}

// GroupKey builds a composite key from a set of grouping expressions.
func (ev *Evaluator) GroupKey(exprs []Expr, row Row) string {
	parts := make([]string, len(exprs))
	for i, expr := range exprs {
		parts[i] = fmt.Sprintf("%v", ev.EvalValue(expr, row))
	}
	return strings.Join(parts, "\x00")
}

// ---------- node / edge property access ----------

// GetNodeProp returns a property value from a graph node.
func GetNodeProp(node *graph.Node, prop string) any {
	switch prop {
	case "id", "ID":
		return node.ID
	case "name", "Name":
		return node.Name
	case "label", "Label":
		return string(node.Label)
	case "file", "filePath", "FilePath":
		return node.FilePath
	case "uid", "UID":
		return node.UID
	default:
		if v, ok := node.Props.GetProp(prop); ok {
			return v
		}
		return nil
	}
}

// GetEdgeProp returns a property value from a graph edge.
func GetEdgeProp(edge *graph.Edge, prop string) any {
	switch prop {
	case "type", "Type":
		return string(edge.Type)
	case "source", "Source":
		return edge.Source
	case "target", "Target":
		return edge.Target
	case "confidence":
		return edge.Confidence()
	default:
		if v, ok := edge.Props.GetProp(prop); ok {
			return v
		}
		return nil
	}
}

// ---------- row utilities ----------

func nodeToValue(val any) any { return val }

// CopyRow creates a shallow copy of a row.
// Uses pre-allocated map with the correct size hint to avoid resizing.
func CopyRow(row Row) Row {
	if row == nil {
		return Row{}
	}
	newRow := make(Row, len(row))
	for k, v := range row {
		newRow[k] = v
	}
	return newRow
}

// CollectColumns returns the unique column names across all rows.
func CollectColumns(rows []Row) []string {
	seen := make(map[string]bool)
	var columns []string
	for _, row := range rows {
		for k := range row {
			if !seen[k] {
				seen[k] = true
				columns = append(columns, k)
			}
		}
	}
	return columns
}

// DeduplicateRows removes duplicate rows based on the given columns.
func DeduplicateRows(rows []Row, columns []string) []Row {
	seen := make(map[string]bool)
	var result []Row
	for _, row := range rows {
		key := RowKey(row, columns)
		if !seen[key] {
			seen[key] = true
			result = append(result, row)
		}
	}
	return result
}

// RowKey builds a composite string key from the given columns.
func RowKey(row Row, columns []string) string {
	parts := make([]string, len(columns))
	for i, col := range columns {
		parts[i] = fmt.Sprintf("%v", row[col])
	}
	return strings.Join(parts, "\x00")
}

// SortRows sorts rows in place according to the ORDER BY specification.
// Uses Evaluator to compute sort key values from expressions, supporting
// property accesses like n.name that may not yet be projected as columns.
func SortRows(rows []Row, orderBy []*OrderByItem, ev *Evaluator) {
	sort.SliceStable(rows, func(i, j int) bool {
		for _, item := range orderBy {
			vi := ev.EvalValue(item.Expr, rows[i])
			vj := ev.EvalValue(item.Expr, rows[j])

			cmp := TypedCompare(vi, vj)
			if cmp != 0 {
				if item.Desc {
					return cmp > 0
				}
				return cmp < 0
			}
		}
		return false
	})
}

// ApplySkipLimit applies SKIP and LIMIT to a row slice.
func ApplySkipLimit(rows []Row, skip, limit *int) []Row {
	s := 0
	if skip != nil {
		s = *skip
	}
	if s >= len(rows) {
		return nil
	}
	rows = rows[s:]

	if limit != nil && *limit < len(rows) {
		rows = rows[:*limit]
	}
	return rows
}

// FilterNodesByProps filters nodes by matching property expressions.
func FilterNodesByProps(nodes []*graph.Node, props map[string]Expr, ev *Evaluator) []*graph.Node {
	result := make([]*graph.Node, 0, len(nodes))
	for _, node := range nodes {
		match := true
		for key, expr := range props {
			expected := ev.EvalValue(expr, nil)
			actual := GetNodeProp(node, key)
			if !TypedEqual(actual, expected) {
				match = false
				break
			}
		}
		if match {
			result = append(result, node)
		}
	}
	return result
}

// ---------- numeric utilities ----------

// ToFloat converts a value to float64 if possible.
func ToFloat(v any) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case string:
		f, err := strconv.ParseFloat(val, 64)
		return f, err == nil
	default:
		return 0, false
	}
}

// TypedEqual performs type-aware equality comparison.
// Prefers numeric comparison when both sides are numeric;
// falls back to string comparison otherwise.
func TypedEqual(a, b any) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	// Try numeric comparison first
	af, aok := ToFloat(a)
	bf, bok := ToFloat(b)
	if aok && bok {
		return af == bf
	}
	// Try direct type-matched comparison
	switch av := a.(type) {
	case string:
		if bv, ok := b.(string); ok {
			return av == bv
		}
	case bool:
		if bv, ok := b.(bool); ok {
			return av == bv
		}
	case float64:
		if bv, ok := b.(float64); ok {
			return av == bv
		}
	}
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

// TypedCompare performs type-aware comparison.
// Returns -1, 0, or 1. Prefers numeric comparison.
func TypedCompare(a, b any) int {
	if a == nil && b == nil {
		return 0
	}
	if a == nil {
		return -1
	}
	if b == nil {
		return 1
	}
	af, aok := ToFloat(a)
	bf, bok := ToFloat(b)
	if aok && bok {
		if af < bf {
			return -1
		}
		if af > bf {
			return 1
		}
		return 0
	}
	// String comparison
	as := toString(a)
	bs := toString(b)
	if as < bs {
		return -1
	}
	if as > bs {
		return 1
	}
	return 0
}

// typedArith performs arithmetic operations on two values.
func typedArith(a, b any, op string) any {
	af, aok := ToFloat(a)
	bf, bok := ToFloat(b)
	if !aok || !bok {
		return nil
	}
	switch op {
	case "+":
		return af + bf
	case "-":
		return af - bf
	case "*":
		return af * bf
	case "/":
		if bf == 0 {
			return nil
		}
		return af / bf
	case "%":
		if bf == 0 {
			return nil
		}
		return math.Mod(af, bf)
	}
	return nil
}

// typedIn checks if a value is contained in a list.
func typedIn(elem, list any) bool {
	arr, ok := list.([]any)
	if !ok {
		return false
	}
	for _, v := range arr {
		if TypedEqual(elem, v) {
			return true
		}
	}
	return false
}

// toString converts a value to its string representation efficiently.
func toString(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case nil:
		return ""
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case int:
		return strconv.Itoa(val)
	case bool:
		if val {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", val)
	}
}

// FindSourceNode scans a row for a *graph.Node value.
func FindSourceNode(row Row) (*graph.Node, bool) {
	for _, v := range row {
		if n, ok := v.(*graph.Node); ok {
			return n, true
		}
	}
	return nil, false
}

// ---------- UNWIND iterator ----------

// UnwindIterator expands a list expression into individual rows.
// Each element in the list produces one output row with the variable
// bound to that element.
type UnwindIterator struct {
	child  Iterator
	expr   Expr
	varName string
	ev     *Evaluator
	buffer []Row
	pos    int
	closed bool
}

// NewUnwindIterator creates an UNWIND iterator.
func NewUnwindIterator(child Iterator, expr Expr, varName string, ev *Evaluator) *UnwindIterator {
	return &UnwindIterator{
		child:   child,
		expr:    expr,
		varName: varName,
		ev:      ev,
		buffer:  make([]Row, 0),
	}
}

func (u *UnwindIterator) Next() (Row, error) {
	if u.closed {
		return nil, nil
	}

	// If we have buffered rows, return them first
	if u.pos < len(u.buffer) {
		row := u.buffer[u.pos]
		u.pos++
		return row, nil
	}

	// Get next input row
	inputRow, err := u.child.Next()
	if err != nil {
		return nil, err
	}
	if inputRow == nil {
		return nil, nil
	}

	// Evaluate the list expression
	val := u.ev.EvalValue(u.expr, inputRow)
	list, ok := val.([]any)
	if !ok {
		// Not a list, skip
		return u.Next()
	}

	// Buffer one row per element
	u.buffer = u.buffer[:0]
	u.pos = 0
	for _, elem := range list {
		row := CopyRow(inputRow)
		row[u.varName] = elem
		u.buffer = append(u.buffer, row)
	}

	if len(u.buffer) == 0 {
		return u.Next()
	}

	row := u.buffer[0]
	u.pos = 1
	return row, nil
}

func (u *UnwindIterator) Close() error {
	u.closed = true
	return u.child.Close()
}