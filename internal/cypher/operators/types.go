package operators

// This file re-exports AST types from the parent cypher package so that
// operator implementations can reference them without creating a circular
// import. The cypher package aliases these via type aliases for backward
// compatibility.

import "github.com/mengshi02/codetrip/internal/graph"

// ---------- Expression types (mirroring cypher AST) ----------
// These are defined here to avoid circular imports between cypher and
// operators. The cypher package provides type aliases that map to these.

// Expr is the interface for all expression nodes.
type Expr interface {
	ExprNode()
}

// IdentifierExpr represents a variable name like n or r.
type IdentifierExpr struct {
	Name string
}

func (i *IdentifierExpr) ExprNode() {}

// PropertyAccessExpr represents n.name style access.
type PropertyAccessExpr struct {
	Target Expr
	Prop   string
}

func (p *PropertyAccessExpr) ExprNode() {}

// StringLiteralExpr represents a string literal.
type StringLiteralExpr struct {
	Value string
}

func (s *StringLiteralExpr) ExprNode() {}

// NumberLiteralExpr represents a numeric literal.
type NumberLiteralExpr struct {
	Value float64
}

func (n *NumberLiteralExpr) ExprNode() {}

// ParameterExpr represents a query parameter like $param.
type ParameterExpr struct {
	Name string
}

func (p *ParameterExpr) ExprNode() {}

// BinaryExpr represents a binary operation (AND, OR, =, etc.).
type BinaryExpr struct {
	Op    string
	Left  Expr
	Right Expr
}

func (b *BinaryExpr) ExprNode() {}

// FunctionCallExpr represents a function call like count(n).
type FunctionCallExpr struct {
	Name     string
	Args     []Expr
	Distinct bool
}

func (f *FunctionCallExpr) ExprNode() {}

// UnaryExpr represents a unary operation (NOT, IS NULL, etc.).
type UnaryExpr struct {
	Op    string
	Right Expr
}

func (u *UnaryExpr) ExprNode() {}

// BooleanLiteralExpr represents a boolean literal (true/false).
type BooleanLiteralExpr struct {
	Value bool
}

func (b *BooleanLiteralExpr) ExprNode() {}

// NullLiteralExpr represents a null literal.
type NullLiteralExpr struct{}

func (n *NullLiteralExpr) ExprNode() {}

// ListExpr represents a list literal [1, 2, 3].
type ListExpr struct {
	Elements []Expr
}

func (l *ListExpr) ExprNode() {}

// CaseExpr represents a CASE expression.
type CaseExpr struct {
	Subject  Expr        // nil for generic CASE WHEN form
	Whens    []CaseWhen  // WHEN ... THEN ... branches
	ElseExpr Expr        // optional ELSE
}

func (c *CaseExpr) ExprNode() {}

// CaseWhen represents one WHEN ... THEN ... branch inside a CASE.
type CaseWhen struct {
	Condition Expr
	Result    Expr
}

// ---------- Clause / pattern types ----------

// ReturnItem represents one item in a RETURN clause.
type ReturnItem struct {
	Expr  Expr
	Alias string
}

// OrderByItem represents one item in an ORDER BY clause.
type OrderByItem struct {
	Expr Expr
	Desc bool
}

// Direction represents the direction of a relationship.
type Direction int

const (
	DirOut    Direction = iota // ->
	DirIn                      // <-
	DirBoth                    // -
)

// CypherNodePattern represents a node pattern like (n:Label {props}).
type CypherNodePattern struct {
	Variable string
	Labels   []string
	Props    map[string]Expr
}

// RelationshipPattern represents a relationship pattern like -[r:TYPE]->.
type RelationshipPattern struct {
	Variable  string
	RelTypes  []string
	Direction Direction
	MinHops   *int
	MaxHops   *int
	Props     map[string]Expr
	Target    *CypherNodePattern
}

// GraphStore abstracts the graph storage operations needed by operators.
type GraphStore interface {
	GetNodesByLabel(repo string, label string) ([]*graph.Node, error)
	GetAllNodes(repo string, limit int) []*graph.Node
	GetNode(id string) (*graph.Node, error)
	GetAllOutEdges(nodeID string) ([]*graph.Edge, error)
	GetAllInEdges(nodeID string) ([]*graph.Edge, error)
	Repo() string
}