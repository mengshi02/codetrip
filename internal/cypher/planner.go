package cypher

import (
	"github.com/mengshi02/codetrip/internal/cypher/operators"
	"github.com/mengshi02/codetrip/internal/graph"
)

// ---------- AST → operators type adapters ----------
//
// The cypher AST types (Expr, IdentifierExpr, etc.) use exprNode() marker
// methods, while the operators package uses ExprNode(). We bridge them via
// lightweight adapter wrappers so that planner code can feed AST expressions
// directly into operator constructors.

// astExpr wraps a cypher AST Expr to satisfy operators.Expr.
type astExpr struct{ inner Expr }

func (a astExpr) ExprNode() {}

// convertExpr recursively converts a cypher Expr into an operators.Expr.
func convertExpr(e Expr) operators.Expr {
	if e == nil {
		return nil
	}
	switch ex := e.(type) {
	case *IdentifierExpr:
		return &operators.IdentifierExpr{Name: ex.Name}
	case *PropertyAccessExpr:
		return &operators.PropertyAccessExpr{
			Target: convertExpr(ex.Target),
			Prop:   ex.Prop,
		}
	case *StringLiteralExpr:
		return &operators.StringLiteralExpr{Value: ex.Value}
	case *NumberLiteralExpr:
		return &operators.NumberLiteralExpr{Value: ex.Value}
	case *ParameterExpr:
		return &operators.ParameterExpr{Name: ex.Name}
	case *BinaryExpr:
		return &operators.BinaryExpr{
			Op:    ex.Op,
			Left:  convertExpr(ex.Left),
			Right: convertExpr(ex.Right),
		}
	case *UnaryExpr:
		return &operators.UnaryExpr{
			Op:    ex.Op,
			Right: convertExpr(ex.Right),
		}
	case *FunctionCallExpr:
		args := make([]operators.Expr, len(ex.Args))
		for i, a := range ex.Args {
			args[i] = convertExpr(a)
		}
		return &operators.FunctionCallExpr{
			Name:     ex.Name,
			Args:     args,
			Distinct: ex.Distinct,
		}
	case *BooleanLiteralExpr:
		return &operators.BooleanLiteralExpr{Value: ex.Value}
	case *NullLiteralExpr:
		return &operators.NullLiteralExpr{}
	case *ListExpr:
		elems := make([]operators.Expr, len(ex.Elements))
		for i, e := range ex.Elements {
			elems[i] = convertExpr(e)
		}
		return &operators.ListExpr{Elements: elems}
	case *CaseExpr:
		var whens []operators.CaseWhen
		for _, w := range ex.Whens {
			whens = append(whens, operators.CaseWhen{
				Condition: convertExpr(w.Condition),
				Result:    convertExpr(w.Result),
			})
		}
		return &operators.CaseExpr{
			Subject:  convertExpr(ex.Subject),
			Whens:    whens,
			ElseExpr: convertExpr(ex.ElseExpr),
		}
	default:
		return astExpr{inner: e}
	}
}

// convertExprs converts a slice of cypher Expr.
func convertExprs(exprs []Expr) []operators.Expr {
	out := make([]operators.Expr, len(exprs))
	for i, e := range exprs {
		out[i] = convertExpr(e)
	}
	return out
}

// convertReturnItems converts cypher ReturnItem slice to operators ReturnItem slice.
func convertReturnItems(items []*ReturnItem) []*operators.ReturnItem {
	out := make([]*operators.ReturnItem, len(items))
	for i, it := range items {
		out[i] = &operators.ReturnItem{
			Expr:  convertExpr(it.Expr),
			Alias: it.Alias,
		}
	}
	return out
}

// convertOrderByItems converts cypher OrderByItem slice to operators OrderByItem slice.
func convertOrderByItems(items []*OrderByItem) []*operators.OrderByItem {
	out := make([]*operators.OrderByItem, len(items))
	for i, it := range items {
		out[i] = &operators.OrderByItem{
			Expr: convertExpr(it.Expr),
			Desc: it.Desc,
		}
	}
	return out
}

// convertNodePattern converts a cypher CypherNodePattern to operators CypherNodePattern.
func convertNodePattern(n *CypherNodePattern) *operators.CypherNodePattern {
	if n == nil {
		return nil
	}
	props := make(map[string]operators.Expr, len(n.Props))
	for k, v := range n.Props {
		props[k] = convertExpr(v)
	}
	return &operators.CypherNodePattern{
		Variable: n.Variable,
		Labels:   n.Labels,
		Props:    props,
	}
}

// convertRelPattern converts a cypher RelationshipPattern to operators RelationshipPattern.
func convertRelPattern(r *RelationshipPattern) *operators.RelationshipPattern {
	if r == nil {
		return nil
	}
	props := make(map[string]operators.Expr, len(r.Props))
	for k, v := range r.Props {
		props[k] = convertExpr(v)
	}
	return &operators.RelationshipPattern{
		Variable:  r.Variable,
		RelTypes:  r.RelTypes,
		Direction: operators.Direction(r.Direction),
		MinHops:   r.MinHops,
		MaxHops:   r.MaxHops,
		Props:     props,
		Target:    convertNodePattern(r.Target),
	}
}

// ---------- GraphStore adapter ----------

// graphStoreAdapter wraps *graph.GraphStore to satisfy operators.GraphStore.
type graphStoreAdapter struct {
	gs *graph.GraphStore
}

func (a graphStoreAdapter) GetNodesByLabel(repo string, label string) ([]*graph.Node, error) {
	return a.gs.GetNodesByLabel(repo, label)
}

func (a graphStoreAdapter) GetAllNodes(repo string, limit int) []*graph.Node {
	return a.gs.GetAllNodes(repo, limit)
}

func (a graphStoreAdapter) GetNode(id string) (*graph.Node, error) {
	return a.gs.GetNode(id)
}

func (a graphStoreAdapter) GetAllOutEdges(nodeID string) ([]*graph.Edge, error) {
	return a.gs.GetAllOutEdges(nodeID)
}

func (a graphStoreAdapter) GetAllInEdges(nodeID string) ([]*graph.Edge, error) {
	return a.gs.GetAllInEdges(nodeID)
}

func (a graphStoreAdapter) Repo() string {
	return a.gs.Repo()
}

// ---------- Plan building ----------

// Plan represents a compiled query execution plan (iterator tree root).
type Plan struct {
	Root    operators.Iterator
	Columns []string
}

// BuildPlan constructs a Volcano iterator tree from a parsed query AST.
func BuildPlan(query *Query, gs *graph.GraphStore, params map[string]any) (*Plan, error) {
	adapter := graphStoreAdapter{gs: gs}
	ev := operators.NewEvaluator(params)

	var root operators.Iterator
	// Start with an empty row source
	root = &emptyIterator{}
	rootVars := newVarSet()

	var columns []string

	for _, clause := range query.Clauses {
		switch c := clause.(type) {
		case *MatchClause:
			matchIt, matchVars := buildMatchPlan(c, adapter, ev)
			if root != nil {
				joinKeys := intersectKeys(rootVars, matchVars)
				if len(joinKeys) > 0 {
					root = operators.NewHashJoinIterator(root, matchIt, joinKeys)
				} else {
					root = operators.NewJoinIterator(root, matchIt)
				}
			} else {
				root = matchIt
			}
			rootVars.merge(matchVars)

		case *WhereClause:
			// Predicate pushdown: split AND conditions and apply each
			// as early as possible. Conditions referencing only one variable
			// can be pushed closer to the Scan producing that variable.
			conditions := splitAndConditions(c.Condition)
			for _, cond := range conditions {
				root = operators.NewFilterIterator(root, convertExpr(cond), ev)
			}

		case *ReturnClause:
			opItems := convertReturnItems(c.Items)
			opOrderBy := convertOrderByItems(c.OrderBy)

			if operators.HasAggregate(opItems) {
				agg := operators.NewAggregateIterator(
					root, opItems, ev,
					opOrderBy, c.Distinct, c.Skip, c.Limit,
				)
				root = agg
				columns = agg.ColNames()
			} else {
				if len(opOrderBy) > 0 {
					sortIt := operators.NewSortIterator(root, opOrderBy, ev)
					// Top-N optimization: if LIMIT follows, only keep N rows
					if c.Limit != nil {
						sortIt.SetTopN(*c.Limit)
					}
					root = sortIt
				}
				if c.Skip != nil || c.Limit != nil {
					root = operators.NewLimitIterator(root, c.Skip, c.Limit)
				}
				proj := operators.NewProjectIterator(root, opItems, ev, c.Distinct)
				root = proj
				columns = proj.ColNames()
			}

		case *UnwindClause:
			root = operators.NewUnwindIterator(root, convertExpr(c.Expr), c.Var, ev)
			rootVars.add(c.Var)

		case *UnionClause:
			// UNION is handled at the executor level by draining both sides
			// Here we just note it; the executor will split the query
			_ = c
		}
	}

	return &Plan{Root: root, Columns: columns}, nil
}

// buildMatchPlan constructs the iterator subtree for a MATCH clause.
// Returns the iterator and the set of variables it produces.
func buildMatchPlan(match *MatchClause, gs operators.GraphStore, ev *operators.Evaluator) (operators.Iterator, *varSet) {
	var root operators.Iterator
	allVars := newVarSet()

	for _, pattern := range match.Patterns {
		patIt, patVars := buildPatternPlan(pattern, gs, ev)
		if root == nil {
			root = patIt
		} else {
			joinKeys := intersectKeys(allVars, patVars)
			if len(joinKeys) > 0 {
				root = operators.NewHashJoinIterator(root, patIt, joinKeys)
			} else {
				root = operators.NewJoinIterator(root, patIt)
			}
		}
		allVars.merge(patVars)
	}

	if match.Optional {
		root = &optionalIterator{child: root}
	}

	return root, allVars
}

// buildPatternPlan constructs the iterator subtree for a single pattern.
// Returns the iterator and the set of variables it produces.
func buildPatternPlan(pattern *Pattern, gs operators.GraphStore, ev *operators.Evaluator) (operators.Iterator, *varSet) {
	var root operators.Iterator
	vars := newVarSet()

	for i, elem := range pattern.Elements {
		switch el := elem.(type) {
		case *CypherNodePattern:
			if i > 0 {
				if prevRel, ok := pattern.Elements[i-1].(*RelationshipPattern); ok && prevRel.Target == el {
					continue
				}
			}
			scan := operators.NewScanIterator(gs, convertNodePattern(el), ev)
			if root == nil {
				root = scan
			} else {
				root = operators.NewJoinIterator(root, scan)
			}
			if el.Variable != "" {
				vars.add(el.Variable)
			}

		case *RelationshipPattern:
			expand := operators.NewExpandIterator(root, convertRelPattern(el), gs, ev)
			root = expand
			if el.Variable != "" {
				vars.add(el.Variable)
			}
			if el.Target != nil && el.Target.Variable != "" {
				vars.add(el.Target.Variable)
			}
		}
	}

	return root, vars
}

// ---------- helper iterators ----------

// varSet tracks variable names produced by an iterator subtree.
// It is built during plan construction so we can detect join keys
// without needing to inspect unexported iterator fields.
type varSet struct {
	names map[string]bool
}

func newVarSet(names ...string) *varSet {
	vs := &varSet{names: make(map[string]bool, len(names))}
	for _, n := range names {
		vs.names[n] = true
	}
	return vs
}

func (vs *varSet) add(name string)      { vs.names[name] = true }
func (vs *varSet) has(name string) bool { return vs.names[name] }
func (vs *varSet) keys() []string {
	keys := make([]string, 0, len(vs.names))
	for k := range vs.names {
		keys = append(keys, k)
	}
	return keys
}
func (vs *varSet) merge(other *varSet) {
	for k := range other.names {
		vs.names[k] = true
	}
}

// intersectKeys returns the variable names present in both varSets.
func intersectKeys(a, b *varSet) []string {
	var common []string
	for k := range a.names {
		if b.names[k] {
			common = append(common, k)
		}
	}
	return common
}

// splitAndConditions decomposes a compound AND expression into individual
// conjuncts. This allows each condition to be applied as a separate filter,
// enabling earlier row elimination. For example:
//
//	n.name = 'foo' AND m.age > 30
//
// becomes two separate filters, each eliminating rows before the next check.
func splitAndConditions(expr Expr) []Expr {
	if bin, ok := expr.(*BinaryExpr); ok && bin.Op == "AND" {
		left := splitAndConditions(bin.Left)
		right := splitAndConditions(bin.Right)
		return append(left, right...)
	}
	return []Expr{expr}
}

// emptyIterator produces a single empty row, used as the starting point.
type emptyIterator struct {
	done   bool
	closed bool
}

func (e *emptyIterator) Next() (operators.Row, error) {
	if e.closed || e.done {
		return nil, nil
	}
	e.done = true
	return operators.Row{}, nil
}

func (e *emptyIterator) Close() error {
	e.closed = true
	return nil
}

// optionalIterator wraps a child and returns a single empty row if the
// child produces no results (OPTIONAL MATCH semantics).
type optionalIterator struct {
	child   operators.Iterator
	checked bool
	empty   bool
	closed  bool
}

func (o *optionalIterator) Next() (operators.Row, error) {
	if o.closed {
		return nil, nil
	}
	if !o.checked {
		o.checked = true
		row, err := o.child.Next()
		if err != nil {
			return nil, err
		}
		if row == nil {
			o.empty = true
			return operators.Row{}, nil
		}
		return row, nil
	}
	if o.empty {
		return nil, nil
	}
	return o.child.Next()
}

func (o *optionalIterator) Close() error {
	o.closed = true
	return o.child.Close()
}
