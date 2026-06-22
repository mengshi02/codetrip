package cypher

import (
	"testing"
)

// --------------- Helper ---------------

// parseHelper lexes + parses a query, failing the test on error.
func parseHelper(t *testing.T, query string) *Query {
	t.Helper()
	l := NewLexer(query)
	tokens := l.Tokenize()
	if len(tokens) > 0 && tokens[0].Type == TokenError {
		t.Fatalf("lexer error: %s", tokens[0].Value)
	}
	p := NewParser(tokens)
	ast, err := p.Parse()
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	return ast
}

// --------------- Unit Tests ---------------

func TestParserSimpleMatchReturn(t *testing.T) {
	q := parseHelper(t, "MATCH (n:Person) RETURN n")
	if len(q.Clauses) != 2 {
		t.Fatalf("expected 2 clauses, got %d", len(q.Clauses))
	}
	mc, ok := q.Clauses[0].(*MatchClause)
	if !ok {
		t.Fatalf("clause 0: expected *MatchClause, got %T", q.Clauses[0])
	}
	if len(mc.Patterns) != 1 {
		t.Fatalf("expected 1 pattern, got %d", len(mc.Patterns))
	}
	rc, ok := q.Clauses[1].(*ReturnClause)
	if !ok {
		t.Fatalf("clause 1: expected *ReturnClause, got %T", q.Clauses[1])
	}
	if len(rc.Items) != 1 {
		t.Fatalf("expected 1 return item, got %d", len(rc.Items))
	}
}

func TestParserMatchWithRelationship(t *testing.T) {
	// The lexer tokenizes -[r:KNOWS]-> as: TokenMinus [ r : KNOWS ] TokenArrowRight
	// parsePattern loop sees TokenMinus after ')', calls parseRelationshipPattern.
	// It consumes '-', then sees TokenLeftBracket for [r:KNOWS], then after ]
	// sees TokenArrowRight for ->, setting DirOut. Then parseNodePattern for (m:City).
	// This should parse correctly.
	q := parseHelper(t, "MATCH (n:Person)-[r:KNOWS]->(m:City) RETURN n, m")
	mc := q.Clauses[0].(*MatchClause)
	pat := mc.Patterns[0]
	if len(pat.Elements) != 3 {
		t.Fatalf("expected 3 pattern elements, got %d", len(pat.Elements))
	}
	rel, ok := pat.Elements[1].(*RelationshipPattern)
	if !ok {
		t.Fatalf("element 1: expected *RelationshipPattern, got %T", pat.Elements[1])
	}
	if rel.Variable != "r" {
		t.Errorf("rel variable: expected 'r', got %q", rel.Variable)
	}
	if len(rel.RelTypes) != 1 || rel.RelTypes[0] != "KNOWS" {
		t.Errorf("rel types: expected [KNOWS], got %v", rel.RelTypes)
	}
	if rel.Direction != DirOut {
		t.Errorf("rel direction: expected DirOut, got %v", rel.Direction)
	}
}

func TestParserIncomingRelationship(t *testing.T) {
	q := parseHelper(t, "MATCH (n:Person)<-[r:KNOWS]-(m:City) RETURN n")
	pat := q.Clauses[0].(*MatchClause).Patterns[0]
	rel := pat.Elements[1].(*RelationshipPattern)
	if rel.Direction != DirIn {
		t.Errorf("expected DirIn, got %v", rel.Direction)
	}
}

func TestParserBidirectionalRelationship(t *testing.T) {
	q := parseHelper(t, "MATCH (n:Person)-[r:KNOWS]-(m:City) RETURN n")
	pat := q.Clauses[0].(*MatchClause).Patterns[0]
	rel := pat.Elements[1].(*RelationshipPattern)
	if rel.Direction != DirBoth {
		t.Errorf("expected DirBoth, got %v", rel.Direction)
	}
}

func TestParserVariableLengthPath(t *testing.T) {
	q := parseHelper(t, "MATCH (n)-[r:KNOWS*1..3]->(m) RETURN n, m")
	pat := q.Clauses[0].(*MatchClause).Patterns[0]
	rel := pat.Elements[1].(*RelationshipPattern)
	if rel.MinHops == nil || *rel.MinHops != 1 {
		t.Errorf("expected MinHops=1, got %v", rel.MinHops)
	}
	if rel.MaxHops == nil || *rel.MaxHops != 3 {
		t.Errorf("expected MaxHops=3, got %v", rel.MaxHops)
	}
}

func TestParserVariableLengthStar(t *testing.T) {
	q := parseHelper(t, "MATCH (n)-[r:KNOWS*]->(m) RETURN n, m")
	pat := q.Clauses[0].(*MatchClause).Patterns[0]
	rel := pat.Elements[1].(*RelationshipPattern)
	if rel.MinHops == nil || *rel.MinHops != 1 {
		t.Errorf("expected MinHops=1, got %v", rel.MinHops)
	}
	if rel.MaxHops != nil {
		t.Errorf("expected MaxHops=nil (unbounded), got %v", *rel.MaxHops)
	}
}

func TestParserWhereClause(t *testing.T) {
	q := parseHelper(t, "MATCH (n:Person) WHERE n.name = 'Alice' RETURN n")
	if len(q.Clauses) != 3 {
		t.Fatalf("expected 3 clauses, got %d", len(q.Clauses))
	}
	wc, ok := q.Clauses[1].(*WhereClause)
	if !ok {
		t.Fatalf("clause 1: expected *WhereClause, got %T", q.Clauses[1])
	}
	bin, ok := wc.Condition.(*BinaryExpr)
	if !ok {
		t.Fatalf("condition: expected *BinaryExpr, got %T", wc.Condition)
	}
	if bin.Op != "=" {
		t.Errorf("expected op '=', got %q", bin.Op)
	}
}

func TestParserReturnDistinct(t *testing.T) {
	q := parseHelper(t, "MATCH (n) RETURN DISTINCT n.name")
	rc := q.Clauses[1].(*ReturnClause)
	if !rc.Distinct {
		t.Error("expected DISTINCT=true")
	}
}

func TestParserReturnWithAlias(t *testing.T) {
	q := parseHelper(t, "MATCH (n) RETURN n.name AS name")
	rc := q.Clauses[1].(*ReturnClause)
	if len(rc.Items) != 1 {
		t.Fatalf("expected 1 return item, got %d", len(rc.Items))
	}
	if rc.Items[0].Alias != "name" {
		t.Errorf("expected alias 'name', got %q", rc.Items[0].Alias)
	}
}

func TestParserOrderByLimit(t *testing.T) {
	q := parseHelper(t, "MATCH (n) RETURN n.name ORDER BY n.name DESC LIMIT 10")
	rc := q.Clauses[1].(*ReturnClause)
	if len(rc.OrderBy) != 1 {
		t.Fatalf("expected 1 order by item, got %d", len(rc.OrderBy))
	}
	if !rc.OrderBy[0].Desc {
		t.Error("expected DESC=true")
	}
	if rc.Limit == nil || *rc.Limit != 10 {
		t.Errorf("expected LIMIT=10, got %v", rc.Limit)
	}
}

func TestParserSkipLimit(t *testing.T) {
	q := parseHelper(t, "MATCH (n) RETURN n SKIP 5 LIMIT 10")
	rc := q.Clauses[1].(*ReturnClause)
	if rc.Skip == nil || *rc.Skip != 5 {
		t.Errorf("expected SKIP=5, got %v", rc.Skip)
	}
	if rc.Limit == nil || *rc.Limit != 10 {
		t.Errorf("expected LIMIT=10, got %v", rc.Limit)
	}
}

func TestParserOptionalMatch(t *testing.T) {
	q := parseHelper(t, "OPTIONAL MATCH (n:Person) RETURN n")
	mc := q.Clauses[0].(*MatchClause)
	if !mc.Optional {
		t.Error("expected Optional=true")
	}
}

func TestParserMultiPattern(t *testing.T) {
	q := parseHelper(t, "MATCH (a:Person), (b:City) RETURN a, b")
	mc := q.Clauses[0].(*MatchClause)
	if len(mc.Patterns) != 2 {
		t.Fatalf("expected 2 patterns, got %d", len(mc.Patterns))
	}
}

func TestParserPathVariable(t *testing.T) {
	q := parseHelper(t, "MATCH p=(n:Person)-[r:KNOWS]->(m:City) RETURN p")
	mc := q.Clauses[0].(*MatchClause)
	if mc.PathVar != "p" {
		t.Errorf("expected PathVar='p', got %q", mc.PathVar)
	}
}

func TestParserUnion(t *testing.T) {
	q := parseHelper(t, "MATCH (n:Person) RETURN n.name UNION MATCH (m:City) RETURN m.name")
	// Clauses: [Match, Return, Union, Match, Return]
	uc, ok := q.Clauses[2].(*UnionClause)
	if !ok {
		t.Fatalf("clause 2: expected *UnionClause, got %T", q.Clauses[2])
	}
	if uc.All {
		t.Error("expected UnionClause.All=false")
	}
}

func TestParserUnionAll(t *testing.T) {
	q := parseHelper(t, "MATCH (n:Person) RETURN n.name UNION ALL MATCH (m:City) RETURN m.name")
	// Clauses: [Match, Return, Union(All=true), Match, Return]
	uc, ok := q.Clauses[2].(*UnionClause)
	if !ok {
		t.Fatalf("clause 2: expected *UnionClause, got %T", q.Clauses[2])
	}
	if !uc.All {
		t.Error("expected UnionClause.All=true")
	}
}

func TestParserUnwind(t *testing.T) {
	q := parseHelper(t, "UNWIND [1, 2, 3] AS x RETURN x")
	uc, ok := q.Clauses[0].(*UnwindClause)
	if !ok {
		t.Fatalf("clause 0: expected *UnwindClause, got %T", q.Clauses[0])
	}
	if uc.Var != "x" {
		t.Errorf("expected var 'x', got %q", uc.Var)
	}
}

func TestParserWithClause(t *testing.T) {
	q := parseHelper(t, "MATCH (n) WITH n.name AS name RETURN name")
	if len(q.Clauses) != 3 {
		t.Fatalf("expected 3 clauses, got %d", len(q.Clauses))
	}
	_, ok := q.Clauses[1].(*ReturnClause) // WITH is parsed as ReturnClause
	if !ok {
		t.Fatalf("clause 1: expected *ReturnClause (WITH), got %T", q.Clauses[1])
	}
}

func TestParserBinaryExpressions(t *testing.T) {
	tests := []struct {
		query string
		op    string
	}{
		{"MATCH (n) WHERE n.age > 18 RETURN n", ">"},
		{"MATCH (n) WHERE n.age >= 18 RETURN n", ">="},
		{"MATCH (n) WHERE n.age < 18 RETURN n", "<"},
		{"MATCH (n) WHERE n.age <= 18 RETURN n", "<="},
		{"MATCH (n) WHERE n.age <> 18 RETURN n", "<>"},
		{"MATCH (n) WHERE n.name = 'Alice' RETURN n", "="},
		{"MATCH (n) WHERE n.active AND n.verified RETURN n", "AND"},
		{"MATCH (n) WHERE n.active OR n.verified RETURN n", "OR"},
	}

	for _, tt := range tests {
		q := parseHelper(t, tt.query)
		wc := q.Clauses[1].(*WhereClause)
		bin, ok := wc.Condition.(*BinaryExpr)
		if !ok {
			t.Errorf("query %q: expected *BinaryExpr, got %T", tt.query, wc.Condition)
			continue
		}
		if bin.Op != tt.op {
			t.Errorf("query %q: expected op %q, got %q", tt.query, tt.op, bin.Op)
		}
	}
}

func TestParserNotExpression(t *testing.T) {
	q := parseHelper(t, "MATCH (n) WHERE NOT n.active RETURN n")
	wc := q.Clauses[1].(*WhereClause)
	unary, ok := wc.Condition.(*UnaryExpr)
	if !ok {
		t.Fatalf("expected *UnaryExpr, got %T", wc.Condition)
	}
	if unary.Op != "NOT" {
		t.Errorf("expected op 'NOT', got %q", unary.Op)
	}
}

func TestParserIsNull(t *testing.T) {
	q := parseHelper(t, "MATCH (n) WHERE n.name IS NULL RETURN n")
	wc := q.Clauses[1].(*WhereClause)
	unary, ok := wc.Condition.(*UnaryExpr)
	if !ok {
		t.Fatalf("expected *UnaryExpr, got %T", wc.Condition)
	}
	if unary.Op != "IS NULL" {
		t.Errorf("expected op 'IS NULL', got %q", unary.Op)
	}
}

func TestParserIsNotNull(t *testing.T) {
	q := parseHelper(t, "MATCH (n) WHERE n.name IS NOT NULL RETURN n")
	wc := q.Clauses[1].(*WhereClause)
	unary, ok := wc.Condition.(*UnaryExpr)
	if !ok {
		t.Fatalf("expected *UnaryExpr, got %T", wc.Condition)
	}
	if unary.Op != "IS NOT NULL" {
		t.Errorf("expected op 'IS NOT NULL', got %q", unary.Op)
	}
}

func TestParserStartsWith(t *testing.T) {
	q := parseHelper(t, "MATCH (n) WHERE n.name STARTS WITH 'Al' RETURN n")
	wc := q.Clauses[1].(*WhereClause)
	bin, ok := wc.Condition.(*BinaryExpr)
	if !ok {
		t.Fatalf("expected *BinaryExpr, got %T", wc.Condition)
	}
	if bin.Op != "STARTS WITH" {
		t.Errorf("expected op 'STARTS WITH', got %q", bin.Op)
	}
}

func TestParserEndsWith(t *testing.T) {
	q := parseHelper(t, "MATCH (n) WHERE n.name ENDS WITH 'ce' RETURN n")
	wc := q.Clauses[1].(*WhereClause)
	bin, ok := wc.Condition.(*BinaryExpr)
	if !ok {
		t.Fatalf("expected *BinaryExpr, got %T", wc.Condition)
	}
	if bin.Op != "ENDS WITH" {
		t.Errorf("expected op 'ENDS WITH', got %q", bin.Op)
	}
}

func TestParserContains(t *testing.T) {
	q := parseHelper(t, "MATCH (n) WHERE n.name CONTAINS 'li' RETURN n")
	wc := q.Clauses[1].(*WhereClause)
	bin, ok := wc.Condition.(*BinaryExpr)
	if !ok {
		t.Fatalf("expected *BinaryExpr, got %T", wc.Condition)
	}
	if bin.Op != "CONTAINS" {
		t.Errorf("expected op 'CONTAINS', got %q", bin.Op)
	}
}

func TestParserInExpression(t *testing.T) {
	q := parseHelper(t, "MATCH (n) WHERE n.age IN [1, 2, 3] RETURN n")
	wc := q.Clauses[1].(*WhereClause)
	bin, ok := wc.Condition.(*BinaryExpr)
	if !ok {
		t.Fatalf("expected *BinaryExpr, got %T", wc.Condition)
	}
	if bin.Op != "IN" {
		t.Errorf("expected op 'IN', got %q", bin.Op)
	}
}

func TestParserAggregateFunctions(t *testing.T) {
	tests := []struct {
		query   string
		funcName string
	}{
		{"MATCH (n) RETURN count(n)", "count"},
		{"MATCH (n) RETURN sum(n.age)", "sum"},
		{"MATCH (n) RETURN avg(n.age)", "avg"},
		{"MATCH (n) RETURN min(n.age)", "min"},
		{"MATCH (n) RETURN max(n.age)", "max"},
		{"MATCH (n) RETURN collect(n.name)", "collect"},
	}

	for _, tt := range tests {
		q := parseHelper(t, tt.query)
		rc := q.Clauses[1].(*ReturnClause)
		fn, ok := rc.Items[0].Expr.(*FunctionCallExpr)
		if !ok {
			t.Errorf("query %q: expected *FunctionCallExpr, got %T", tt.query, rc.Items[0].Expr)
			continue
		}
		if fn.Name != tt.funcName {
			t.Errorf("query %q: expected funcName %q, got %q", tt.query, tt.funcName, fn.Name)
		}
	}
}

func TestParserCountDistinct(t *testing.T) {
	q := parseHelper(t, "MATCH (n) RETURN count(DISTINCT n.name)")
	rc := q.Clauses[1].(*ReturnClause)
	fn, ok := rc.Items[0].Expr.(*FunctionCallExpr)
	if !ok {
		t.Fatalf("expected *FunctionCallExpr, got %T", rc.Items[0].Expr)
	}
	if !fn.Distinct {
		t.Error("expected Distinct=true")
	}
}

func TestParserCountStar(t *testing.T) {
	q := parseHelper(t, "MATCH (n) RETURN count(*)")
	rc := q.Clauses[1].(*ReturnClause)
	fn, ok := rc.Items[0].Expr.(*FunctionCallExpr)
	if !ok {
		t.Fatalf("expected *FunctionCallExpr, got %T", rc.Items[0].Expr)
	}
	if len(fn.Args) != 1 {
		t.Fatalf("expected 1 arg, got %d", len(fn.Args))
	}
	id, ok := fn.Args[0].(*IdentifierExpr)
	if !ok || id.Name != "*" {
		t.Errorf("expected arg '*', got %v", fn.Args[0])
	}
}

func TestParserCaseExpression(t *testing.T) {
	q := parseHelper(t, "MATCH (n) RETURN CASE n.age WHEN 1 THEN 'one' ELSE 'other' END")
	rc := q.Clauses[1].(*ReturnClause)
	ce, ok := rc.Items[0].Expr.(*CaseExpr)
	if !ok {
		t.Fatalf("expected *CaseExpr, got %T", rc.Items[0].Expr)
	}
	if ce.Subject == nil {
		t.Error("expected Subject (simple CASE form)")
	}
	if len(ce.Whens) != 1 {
		t.Errorf("expected 1 WHEN, got %d", len(ce.Whens))
	}
	if ce.ElseExpr == nil {
		t.Error("expected ELSE expression")
	}
}

func TestParserGenericCaseExpression(t *testing.T) {
	q := parseHelper(t, "MATCH (n) RETURN CASE WHEN n.age > 18 THEN 'adult' ELSE 'minor' END")
	rc := q.Clauses[1].(*ReturnClause)
	ce, ok := rc.Items[0].Expr.(*CaseExpr)
	if !ok {
		t.Fatalf("expected *CaseExpr, got %T", rc.Items[0].Expr)
	}
	if ce.Subject != nil {
		t.Error("expected nil Subject (generic CASE form)")
	}
}

func TestParserParameter(t *testing.T) {
	q := parseHelper(t, "MATCH (n) WHERE n.name = $name RETURN n")
	wc := q.Clauses[1].(*WhereClause)
	bin := wc.Condition.(*BinaryExpr)
	param, ok := bin.Right.(*ParameterExpr)
	if !ok {
		t.Fatalf("expected *ParameterExpr, got %T", bin.Right)
	}
	if param.Name != "name" {
		t.Errorf("expected param name 'name', got %q", param.Name)
	}
}

func TestParserListLiteral(t *testing.T) {
	q := parseHelper(t, "UNWIND [1, 2, 3] AS x RETURN x")
	uc := q.Clauses[0].(*UnwindClause)
	list, ok := uc.Expr.(*ListExpr)
	if !ok {
		t.Fatalf("expected *ListExpr, got %T", uc.Expr)
	}
	if len(list.Elements) != 3 {
		t.Errorf("expected 3 list elements, got %d", len(list.Elements))
	}
}

func TestParserNodeWithProps(t *testing.T) {
	q := parseHelper(t, "MATCH (n:Person {name: 'Alice'}) RETURN n")
	mc := q.Clauses[0].(*MatchClause)
	node := mc.Patterns[0].Elements[0].(*CypherNodePattern)
	if len(node.Props) != 1 {
		t.Fatalf("expected 1 prop, got %d", len(node.Props))
	}
	if _, ok := node.Props["name"]; !ok {
		t.Error("expected prop 'name'")
	}
}

func TestParserMultiLabelNode(t *testing.T) {
	q := parseHelper(t, "MATCH (n:Person:Employee) RETURN n")
	mc := q.Clauses[0].(*MatchClause)
	node := mc.Patterns[0].Elements[0].(*CypherNodePattern)
	if len(node.Labels) != 2 {
		t.Fatalf("expected 2 labels, got %d", len(node.Labels))
	}
	if node.Labels[0] != "Person" || node.Labels[1] != "Employee" {
		t.Errorf("expected labels [Person Employee], got %v", node.Labels)
	}
}

func TestParserMultiRelTypes(t *testing.T) {
	q := parseHelper(t, "MATCH (n)-[r:KNOWS|LOVES]->(m) RETURN n, m")
	pat := q.Clauses[0].(*MatchClause).Patterns[0]
	rel := pat.Elements[1].(*RelationshipPattern)
	if len(rel.RelTypes) != 2 {
		t.Fatalf("expected 2 rel types, got %d", len(rel.RelTypes))
	}
	if rel.RelTypes[0] != "KNOWS" || rel.RelTypes[1] != "LOVES" {
		t.Errorf("expected [KNOWS LOVES], got %v", rel.RelTypes)
	}
}

func TestParserArithmetic(t *testing.T) {
	q := parseHelper(t, "MATCH (n) RETURN n.a + n.b * n.c - n.d / n.e % n.f AS result")
	rc := q.Clauses[1].(*ReturnClause)
	// Just verify it parses without error — precedence is exercised through evaluation
	if len(rc.Items) != 1 {
		t.Fatalf("expected 1 return item, got %d", len(rc.Items))
	}
}

func TestParserBoolLiterals(t *testing.T) {
	q := parseHelper(t, "MATCH (n) WHERE n.active = true AND n.deleted = false RETURN n")
	wc := q.Clauses[1].(*WhereClause)
	bin := wc.Condition.(*BinaryExpr)
	// Left side: n.active = true
	left := bin.Left.(*BinaryExpr)
	_, ok1 := left.Right.(*BooleanLiteralExpr)
	if !ok1 {
		t.Errorf("expected *BooleanLiteralExpr for true, got %T", left.Right)
	}
	// Right side: n.deleted = false
	right := bin.Right.(*BinaryExpr)
	_, ok2 := right.Right.(*BooleanLiteralExpr)
	if !ok2 {
		t.Errorf("expected *BooleanLiteralExpr for false, got %T", right.Right)
	}
}

func TestParserNullLiteral(t *testing.T) {
	q := parseHelper(t, "MATCH (n) WHERE n.name = null RETURN n")
	wc := q.Clauses[1].(*WhereClause)
	bin := wc.Condition.(*BinaryExpr)
	_, ok := bin.Right.(*NullLiteralExpr)
	if !ok {
		t.Errorf("expected *NullLiteralExpr, got %T", bin.Right)
	}
}

func TestParserEmptyQuery(t *testing.T) {
	p := NewParser([]Token{{Type: TokenEOF}})
	_, err := p.Parse()
	if err == nil {
		t.Error("expected error for empty query")
	}
}

func TestParserUnexpectedToken(t *testing.T) {
	p := NewParser([]Token{{Type: TokenPlus, Value: "+"}})
	_, err := p.Parse()
	if err == nil {
		t.Error("expected error for unexpected token")
	}
}

func TestParserExistsFunction(t *testing.T) {
	q := parseHelper(t, "MATCH (n) WHERE EXISTS(n.name) RETURN n")
	wc := q.Clauses[1].(*WhereClause)
	fn, ok := wc.Condition.(*FunctionCallExpr)
	if !ok {
		t.Fatalf("expected *FunctionCallExpr, got %T", wc.Condition)
	}
	if fn.Name != "exists" {
		t.Errorf("expected func name 'exists', got %q", fn.Name)
	}
}

// --------------- Benchmarks ---------------

func BenchmarkParserSimple(b *testing.B) {
	input := "MATCH (n:Person) RETURN n"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l := NewLexer(input)
		tokens := l.Tokenize()
		p := NewParser(tokens)
		p.Parse()
	}
}

func BenchmarkParserComplex(b *testing.B) {
	input := "MATCH (n:Person)-[r:KNOWS*1..3]->(m:City) WHERE n.name STARTS WITH 'A' AND m.population > 1000000 RETURN n.name AS name, count(r) AS cnt ORDER BY cnt DESC LIMIT 50"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l := NewLexer(input)
		tokens := l.Tokenize()
		p := NewParser(tokens)
		p.Parse()
	}
}

func BenchmarkParserAggregate(b *testing.B) {
	input := "MATCH (n:Person) RETURN n.label AS label, count(n) AS cnt, avg(n.age) AS avg_age, collect(n.name) AS names ORDER BY cnt DESC LIMIT 10"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l := NewLexer(input)
		tokens := l.Tokenize()
		p := NewParser(tokens)
		p.Parse()
	}
}