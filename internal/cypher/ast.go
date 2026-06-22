package cypher

// AST node types
type NodeType int

const (
	NodeQuery NodeType = iota
	NodeMatchClause
	NodeReturnClause
	NodeWhereClause
	NodeOrderByClause
	NodeLimitClause
	NodePattern
	NodeCypherNode    // (n:Label {props})
	NodeCypherRel     // -[r:TYPE]->
	NodePathPattern
	NodePropertyAccess // n.name
	NodeFunctionCall   // count(n)
	NodeBinaryExpr     // AND, OR, =, <>, +, -, etc.
	NodeUnaryExpr      // NOT
	NodeIdentifier
	NodeStringLiteral
	NodeNumberLiteral
	NodeParameter      // $param
	NodeDistinct
	NodeAliasedExpr    // expr AS alias
	NodeAggregateExpr
)

// ASTNode represents an AST node
type ASTNode struct {
	Type     NodeType
	Children []*ASTNode
	Value    string    // identifier name / literal value
	Props    map[string]any
}

// NewASTNode creates an AST node
func NewASTNode(typ NodeType) *ASTNode {
	return &ASTNode{
		Type:     typ,
		Children: make([]*ASTNode, 0),
		Props:    make(map[string]any),
	}
}

// AddChild adds a child node
func (n *ASTNode) AddChild(child *ASTNode) *ASTNode {
	n.Children = append(n.Children, child)
	return n
}

// Query represents a query AST
type Query struct {
	Clauses     []Clause
	UnionAll    bool   // UNION ALL flag
	UnionRight  *Query // Right side of UNION
}

// Clause represents a query clause
type Clause interface {
	ClauseType() string
}

// MatchClause represents a MATCH clause
type MatchClause struct {
	Optional bool
	Patterns []*Pattern
	PathVar  string // path variable name e.g. "p" in "p=(a)-[r]->(b)"
}

func (m *MatchClause) ClauseType() string { return "MATCH" }

// ReturnClause represents a RETURN clause
type ReturnClause struct {
	Items    []*ReturnItem
	Distinct bool
	OrderBy  []*OrderByItem
	Limit    *int
	Skip     *int
}

func (r *ReturnClause) ClauseType() string { return "RETURN" }

// WhereClause represents a WHERE clause
type WhereClause struct {
	Condition Expr
}

func (w *WhereClause) ClauseType() string { return "WHERE" }

// UnwindClause represents an UNWIND clause
type UnwindClause struct {
	Expr   Expr   // list expression
	Var    string // AS variable
}

func (u *UnwindClause) ClauseType() string { return "UNWIND" }

// UnionClause represents a UNION [ALL] clause
type UnionClause struct {
	All bool
}

func (u *UnionClause) ClauseType() string { return "UNION" }

// Pattern represents a match pattern
type Pattern struct {
	Elements []PatternElement
}

// PatternElement represents a pattern element
type PatternElement interface {
	patternElement()
}

// CypherNodePattern represents a node pattern (n:Label {props})
type CypherNodePattern struct {
	Variable string
	Labels   []string
	Props    map[string]Expr
}

func (n *CypherNodePattern) patternElement() {}

// RelationshipPattern represents a relationship pattern -[r:TYPE]->
type RelationshipPattern struct {
	Variable  string
	RelTypes  []string
	Direction Direction
	MinHops   *int
	MaxHops   *int
	Props     map[string]Expr
	Target    *CypherNodePattern
}

func (r *RelationshipPattern) patternElement() {}

// Direction represents relationship direction
type Direction int

const (
	DirOut    Direction = iota // ->
	DirIn                      // <-
	DirBoth                    // -
)

// ReturnItem represents a RETURN item
type ReturnItem struct {
	Expr  Expr
	Alias string
}

// OrderByItem represents an ORDER BY item
type OrderByItem struct {
	Expr Expr
	Desc bool
}

// ============ Expression types ============

// Expr represents an expression interface
type Expr interface {
	exprNode()
}

// IdentifierExpr represents an identifier expression
type IdentifierExpr struct {
	Name string
}

func (i *IdentifierExpr) exprNode() {}

// PropertyAccessExpr represents a property access expression
type PropertyAccessExpr struct {
	Target Expr
	Prop   string
}

func (p *PropertyAccessExpr) exprNode() {}

// StringLiteralExpr represents a string literal
type StringLiteralExpr struct {
	Value string
}

func (s *StringLiteralExpr) exprNode() {}

// NumberLiteralExpr represents a number literal
type NumberLiteralExpr struct {
	Value float64
}

func (n *NumberLiteralExpr) exprNode() {}

// BooleanLiteralExpr represents a boolean literal (true/false)
type BooleanLiteralExpr struct {
	Value bool
}

func (b *BooleanLiteralExpr) exprNode() {}

// NullLiteralExpr represents a null literal
type NullLiteralExpr struct{}

func (n *NullLiteralExpr) exprNode() {}

// ParameterExpr represents a parameter expression
type ParameterExpr struct {
	Name string
}

func (p *ParameterExpr) exprNode() {}

// BinaryExpr represents a binary expression
type BinaryExpr struct {
	Op    string // AND, OR, =, <>, <, >, <=, >=, +, -, *, /, %, STARTS WITH, ENDS WITH, CONTAINS, IN
	Left  Expr
	Right Expr
}

func (b *BinaryExpr) exprNode() {}

// UnaryExpr represents a unary expression
type UnaryExpr struct {
	Op    string // NOT, IS NULL, IS NOT NULL
	Right Expr
}

func (u *UnaryExpr) exprNode() {}

// FunctionCallExpr represents a function call expression
type FunctionCallExpr struct {
	Name      string
	Args      []Expr
	Distinct  bool // e.g. COUNT(DISTINCT x)
}

func (f *FunctionCallExpr) exprNode() {}

// ListExpr represents a list literal [1, 2, 3]
type ListExpr struct {
	Elements []Expr
}

func (l *ListExpr) exprNode() {}

// CaseExpr represents a CASE expression
type CaseExpr struct {
	Subject   Expr          // optional: CASE x WHEN ... (simple form)
	Whens     []CaseWhen    // WHEN ... THEN ...
	ElseExpr  Expr          // optional: ELSE ...
}

// CaseWhen represents one WHEN ... THEN ... branch
type CaseWhen struct {
	Condition Expr
	Result    Expr
}

func (c *CaseExpr) exprNode() {}