package cypher

import (
	"fmt"
	"strconv"
)

// Parser represents a Cypher syntax parser
type Parser struct {
	tokens  []Token
	pos     int
	lastErr error
}

// NewParser creates a parser
func NewParser(tokens []Token) *Parser {
	return &Parser{tokens: tokens, pos: 0}
}

// Parse parses a query
func (p *Parser) Parse() (*Query, error) {
	query := &Query{}

	for !p.atEnd() {
		clause, err := p.parseClause()
		if err != nil {
			return nil, err
		}
		if clause != nil {
			query.Clauses = append(query.Clauses, clause)
		}
	}

	if len(query.Clauses) == 0 {
		return nil, fmt.Errorf("empty query")
	}
	return query, nil
}

// parseClause parses a clause
func (p *Parser) parseClause() (Clause, error) {
	tok := p.peek()
	switch tok.Type {
	case TokenMATCH:
		return p.parseMatch()
	case TokenWHERE:
		return p.parseWhere()
	case TokenRETURN:
		return p.parseReturn()
	case TokenWITH:
		return p.parseWith()
	case TokenORDER:
		return p.parseOrderBy()
	case TokenLIMIT:
		return p.parseLimit()
	case TokenSKIP:
		return p.parseSkip()
	case TokenOPTIONAL:
		return p.parseOptionalMatch()
	case TokenUNION:
		return p.parseUnion()
	case TokenUNWIND:
		return p.parseUnwind()
	default:
		return nil, fmt.Errorf("unexpected token: %v (%s) at pos %d", tok.Type, tok.Value, tok.Pos)
	}
}

// parseMatch parses a MATCH clause
func (p *Parser) parseMatch() (*MatchClause, error) {
	p.advance() // consume MATCH

	match := &MatchClause{}

	// OPTIONAL MATCH
	if p.peek().Type == TokenOPTIONAL {
		match.Optional = true
		p.advance()
	}

	// Path variable: p=(a)-[r]->(b)
	if p.peek().Type == TokenIdentifier {
		saved := p.pos
		varName := p.advance().Value
		if p.peek().Type == TokenEquals {
			p.advance() // consume =
			match.PathVar = varName
		} else {
			// Not a path assignment, backtrack
			p.pos = saved
		}
	}

	// Parse pattern list (comma-separated)
	for {
		pattern, err := p.parsePattern()
		if err != nil {
			return nil, err
		}
		match.Patterns = append(match.Patterns, pattern)

		if p.peek().Type != TokenComma {
			break
		}
		p.advance() // consume comma
	}

	return match, nil
}

// parsePattern parses a match pattern
func (p *Parser) parsePattern() (*Pattern, error) {
	pattern := &Pattern{}

	// First node
	node, err := p.parseNodePattern()
	if err != nil {
		return nil, err
	}
	pattern.Elements = append(pattern.Elements, node)

	// Subsequent relationship-node pairs: -> or - or <-
	for p.peek().Type == TokenArrowRight || p.peek().Type == TokenArrowLeft || p.peek().Type == TokenMinus {
		rel, err := p.parseRelationshipPattern(node)
		if err != nil {
			return nil, err
		}
		pattern.Elements = append(pattern.Elements, rel)

		// Target node
		target, err := p.parseNodePattern()
		if err != nil {
			return nil, err
		}
		rel.Target = target
		node = target
		pattern.Elements = append(pattern.Elements, target)
	}

	return pattern, nil
}

// parseNodePattern parses a node pattern (n:Label {props})
func (p *Parser) parseNodePattern() (*CypherNodePattern, error) {
	n := &CypherNodePattern{
		Labels: make([]string, 0),
	}

	if p.peek().Type != TokenLeftParen {
		return nil, fmt.Errorf("expected '(' but got %v", p.peek().Type)
	}
	p.advance() // consume (

	// Variable name
	if p.peek().Type == TokenIdentifier {
		n.Variable = p.advance().Value
	}

	// Labels (can have multiple :Label)
	for p.peek().Type == TokenColon {
		p.advance() // consume :
		if p.peek().Type != TokenIdentifier {
			return nil, fmt.Errorf("expected label name after ':'")
		}
		n.Labels = append(n.Labels, p.advance().Value)
	}

	// Properties {key: value, ...}
	if p.peek().Type == TokenLeftBrace {
		props, err := p.parseProps()
		if err != nil {
			return nil, err
		}
		n.Props = props
	}

	if p.peek().Type != TokenRightParen {
		return nil, fmt.Errorf("expected ')' but got %v", p.peek().Type)
	}
	p.advance() // consume )

	return n, nil
}

// parseRelationshipPattern parses a relationship pattern:
//   -[r:TYPE*1..3]->   (outgoing, bracket form)
//   -[r:TYPE]->        (outgoing, bracket form)
//   <-[r:TYPE]-        (incoming, bracket form)
//   -[r:TYPE]-         (bidirectional, bracket form)
//   ->                 (shorthand outgoing)
//   -                  (shorthand bidirectional)
func (p *Parser) parseRelationshipPattern(source *CypherNodePattern) (*RelationshipPattern, error) {
	rel := &RelationshipPattern{
		RelTypes: make([]string, 0),
	}

	first := p.peek()

	switch first.Type {
	case TokenArrowRight:
		// Shorthand: ->
		p.advance()
		rel.Direction = DirOut
		return rel, nil

	case TokenArrowLeft:
		// <-[r:TYPE]- form: incoming relationship
		p.advance() // consume <-
		rel.Direction = DirIn

	case TokenMinus:
		// -[r:TYPE]-> or -[r:TYPE]- or just -
		p.advance() // consume -
		rel.Direction = DirBoth // default for dash-started patterns; overridden by trailing arrow

	default:
		return nil, fmt.Errorf("expected '-' or '->' or '<-' in relationship pattern")
	}

	// Parse bracket contents if present
	if p.peek().Type == TokenLeftBracket {
		p.advance() // consume [

		// Variable name
		if p.peek().Type == TokenIdentifier {
			rel.Variable = p.advance().Value
		}

		// Relationship types (can have multiple :TYPE)
		for p.peek().Type == TokenColon {
			p.advance() // consume :
			if p.peek().Type == TokenPipe {
				p.advance() // consume | (multi-type separator)
				continue
			}
			if p.peek().Type != TokenIdentifier {
				return nil, fmt.Errorf("expected relationship type after ':'")
			}
			rel.RelTypes = append(rel.RelTypes, p.advance().Value)

			// Support TYPE1|TYPE2
			for p.peek().Type == TokenPipe {
				p.advance()
				if p.peek().Type == TokenIdentifier {
					rel.RelTypes = append(rel.RelTypes, p.advance().Value)
				}
			}
		}

		// Variable-length path *min..max
		if p.peek().Type == TokenStar {
			p.advance()
			rel.MinHops = intPtr(1)
			rel.MaxHops = intPtr(1)

			if p.peek().Type == TokenDot {
				// *1..3 format
				p.advance() // first .
				p.advance() // second .
				if p.peek().Type == TokenNumber {
					maxVal, _ := strconv.Atoi(p.advance().Value)
					rel.MaxHops = intPtr(maxVal)
				}
			} else if p.peek().Type == TokenNumber {
				minVal, _ := strconv.Atoi(p.advance().Value)
				rel.MinHops = intPtr(minVal)
				rel.MaxHops = intPtr(minVal)
				if p.peek().Type == TokenDot {
					p.advance() // first .
					p.advance() // second .
					if p.peek().Type == TokenNumber {
						maxVal, _ := strconv.Atoi(p.advance().Value)
						rel.MaxHops = intPtr(maxVal)
					}
				}
			} else {
				// * means any length
				rel.MinHops = intPtr(1)
				rel.MaxHops = nil
			}
		}

		// Properties
		if p.peek().Type == TokenLeftBrace {
			props, err := p.parseProps()
			if err != nil {
				return nil, err
			}
			rel.Props = props
		}

		if p.peek().Type != TokenRightBracket {
			return nil, fmt.Errorf("expected ']' but got %v", p.peek().Type)
		}
		p.advance() // consume ]
	}

	// Determine direction from trailing arrow
	// For DirIn (started with <-), we expect trailing -
	// For DirOut (started with -), we expect trailing ->
	// For no bracket, we already set direction
	switch rel.Direction {
	case DirIn:
		// <-[r:TYPE]- : consume trailing -
		if p.peek().Type == TokenMinus {
			p.advance()
		}
	case DirOut:
		// already set, no trailing needed (shorthand form)
	default:
		// Started with -, check trailing
		if p.peek().Type == TokenArrowRight {
			p.advance()
			rel.Direction = DirOut
		} else if p.peek().Type == TokenMinus {
			p.advance()
			// -[...]- means bidirectional
			rel.Direction = DirBoth
		} else {
			rel.Direction = DirBoth
		}
	}

	return rel, nil
}

// parseWhere parses a WHERE clause
func (p *Parser) parseWhere() (*WhereClause, error) {
	p.advance() // consume WHERE

	cond, err := p.parseExpr()
	if err != nil {
		return nil, err
	}

	return &WhereClause{Condition: cond}, nil
}

// parseReturn parses a RETURN clause
func (p *Parser) parseReturn() (*ReturnClause, error) {
	p.advance() // consume RETURN

	ret := &ReturnClause{}

	// DISTINCT
	if p.peek().Type == TokenDISTINCT {
		ret.Distinct = true
		p.advance()
	}

	// Return items list
	for {
		item, err := p.parseReturnItem()
		if err != nil {
			return nil, err
		}
		ret.Items = append(ret.Items, item)

		if p.peek().Type != TokenComma {
			break
		}
		p.advance() // consume comma
	}

	ret.OrderBy, ret.Skip, ret.Limit = p.parseOrderBySkipLimit()

	return ret, nil
}

// parseWith parses a WITH clause (similar to RETURN)
func (p *Parser) parseWith() (Clause, error) {
	p.advance() // consume WITH

	ret := &ReturnClause{}

	for {
		item, err := p.parseReturnItem()
		if err != nil {
			return nil, err
		}
		ret.Items = append(ret.Items, item)

		if p.peek().Type != TokenComma {
			break
		}
		p.advance()
	}

	ret.OrderBy, ret.Skip, ret.Limit = p.parseOrderBySkipLimit()

	return ret, nil
}

// parseOrderBySkipLimit parses optional ORDER BY, SKIP, LIMIT after RETURN/WITH
func (p *Parser) parseOrderBySkipLimit() ([]*OrderByItem, *int, *int) {
	var orderBy []*OrderByItem
	var skip, limit *int

	// ORDER BY
	if p.peek().Type == TokenORDER {
		p.advance()
		if p.peek().Type != TokenBY {
			return nil, nil, nil
		}
		p.advance()

		for {
			item, err := p.parseOrderByItem()
			if err != nil {
				break
			}
			orderBy = append(orderBy, item)

			if p.peek().Type != TokenComma {
				break
			}
			p.advance()
		}
	}

	// SKIP
	if p.peek().Type == TokenSKIP {
		p.advance()
		if p.peek().Type == TokenNumber {
			val, _ := strconv.Atoi(p.advance().Value)
			skip = &val
		}
	}

	// LIMIT
	if p.peek().Type == TokenLIMIT {
		p.advance()
		if p.peek().Type == TokenNumber {
			val, _ := strconv.Atoi(p.advance().Value)
			limit = &val
		}
	}

	return orderBy, skip, limit
}

// parseOptionalMatch parses an OPTIONAL MATCH clause
func (p *Parser) parseOptionalMatch() (Clause, error) {
	p.advance() // consume OPTIONAL
	match, err := p.parseMatch()
	if err != nil {
		return nil, err
	}
	match.Optional = true
	return match, nil
}

// parseUnion parses a UNION [ALL] clause
func (p *Parser) parseUnion() (Clause, error) {
	p.advance() // consume UNION
	all := false
	if p.peek().Type == TokenALL {
		all = true
		p.advance()
	}
	return &UnionClause{All: all}, nil
}

// parseUnwind parses an UNWIND clause
func (p *Parser) parseUnwind() (Clause, error) {
	p.advance() // consume UNWIND

	expr, err := p.parseExpr()
	if err != nil {
		return nil, err
	}

	if p.peek().Type != TokenAS {
		return nil, fmt.Errorf("expected AS after UNWIND expression")
	}
	p.advance()

	if p.peek().Type != TokenIdentifier {
		return nil, fmt.Errorf("expected variable name after AS in UNWIND")
	}
	varName := p.advance().Value

	return &UnwindClause{Expr: expr, Var: varName}, nil
}

// parseOrderBy parses a standalone ORDER BY
func (p *Parser) parseOrderBy() (Clause, error) {
	p.advance() // ORDER
	if p.peek().Type != TokenBY {
		return nil, fmt.Errorf("expected BY after ORDER")
	}
	p.advance()

	var items []*OrderByItem
	for {
		item, err := p.parseOrderByItem()
		if err != nil {
			return nil, err
		}
		items = append(items, item)

		if p.peek().Type != TokenComma {
			break
		}
		p.advance()
	}

	ret := &ReturnClause{OrderBy: items}
	return ret, nil
}

// parseLimit parses a standalone LIMIT
func (p *Parser) parseLimit() (Clause, error) {
	p.advance() // LIMIT
	val, err := strconv.Atoi(p.advance().Value)
	if err != nil {
		return nil, fmt.Errorf("invalid LIMIT value")
	}
	ret := &ReturnClause{Limit: &val}
	return ret, nil
}

// parseSkip parses a standalone SKIP
func (p *Parser) parseSkip() (Clause, error) {
	p.advance() // SKIP
	val, err := strconv.Atoi(p.advance().Value)
	if err != nil {
		return nil, fmt.Errorf("invalid SKIP value")
	}
	ret := &ReturnClause{Skip: &val}
	return ret, nil
}

// ============ Expression parsing ============
// Precedence (low to high):
//   OR
//   AND
//   NOT (unary)
//   comparison: =, <>, !=, <, >, <=, >=, STARTS WITH, ENDS WITH, CONTAINS, IN, IS NULL, IS NOT NULL
//   addition/subtraction: +, -
//   multiplication/division/modulo: *, /, %
//   unary: -, NOT
//   primary: literal, identifier, function, (expr), list, CASE

func (p *Parser) parseExpr() (Expr, error) {
	return p.parseOr()
}

func (p *Parser) parseOr() (Expr, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.peek().Type == TokenOR {
		p.advance()
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Op: "OR", Left: left, Right: right}
	}
	return left, nil
}

func (p *Parser) parseAnd() (Expr, error) {
	left, err := p.parseNot()
	if err != nil {
		return nil, err
	}
	for p.peek().Type == TokenAND {
		p.advance()
		right, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Op: "AND", Left: left, Right: right}
	}
	return left, nil
}

func (p *Parser) parseNot() (Expr, error) {
	if p.peek().Type == TokenNOT {
		p.advance()
		right, err := p.parseComparison()
		if err != nil {
			return nil, err
		}
		return &UnaryExpr{Op: "NOT", Right: right}, nil
	}
	return p.parseComparison()
}

func (p *Parser) parseComparison() (Expr, error) {
	left, err := p.parseAddition()
	if err != nil {
		return nil, err
	}

	for {
		switch p.peek().Type {
		case TokenEquals:
			p.advance()
			right, err := p.parseAddition()
			if err != nil {
				return nil, err
			}
			left = &BinaryExpr{Op: "=", Left: left, Right: right}
		case TokenNotEq: // <> or !=
			p.advance()
			right, err := p.parseAddition()
			if err != nil {
				return nil, err
			}
			left = &BinaryExpr{Op: "<>", Left: left, Right: right}
		case TokenLessThan:
			p.advance()
			right, err := p.parseAddition()
			if err != nil {
				return nil, err
			}
			left = &BinaryExpr{Op: "<", Left: left, Right: right}
		case TokenGreaterThan:
			p.advance()
			right, err := p.parseAddition()
			if err != nil {
				return nil, err
			}
			left = &BinaryExpr{Op: ">", Left: left, Right: right}
		case TokenLessEq:
			p.advance()
			right, err := p.parseAddition()
			if err != nil {
				return nil, err
			}
			left = &BinaryExpr{Op: "<=", Left: left, Right: right}
		case TokenGreaterEq:
			p.advance()
			right, err := p.parseAddition()
			if err != nil {
				return nil, err
			}
			left = &BinaryExpr{Op: ">=", Left: left, Right: right}
		case TokenIS:
			p.advance()
			if p.peek().Type == TokenNOT {
				p.advance()
				if p.peek().Type == TokenNULL {
					p.advance()
					left = &UnaryExpr{Op: "IS NOT NULL", Right: left}
				} else {
					return nil, fmt.Errorf("expected NULL after IS NOT")
				}
			} else if p.peek().Type == TokenNULL {
				p.advance()
				left = &UnaryExpr{Op: "IS NULL", Right: left}
			} else {
				return nil, fmt.Errorf("expected NULL after IS")
			}
		case TokenIdentifier:
			switch p.peek().Value {
			case "STARTS", "starts", "Starts":
				op := p.advance().Value
				if p.peek().Type != TokenIdentifier && p.peek().Type != TokenWITH {
					return nil, fmt.Errorf("expected WITH after %s", op)
				}
				p.advance()
				right, err := p.parseAddition()
				if err != nil {
					return nil, err
				}
				left = &BinaryExpr{Op: "STARTS WITH", Left: left, Right: right}
			case "ENDS", "ends", "Ends":
				op := p.advance().Value
				if p.peek().Type != TokenIdentifier && p.peek().Type != TokenWITH {
					return nil, fmt.Errorf("expected WITH after %s", op)
				}
				p.advance()
				right, err := p.parseAddition()
				if err != nil {
					return nil, err
				}
				left = &BinaryExpr{Op: "ENDS WITH", Left: left, Right: right}
			case "CONTAINS", "contains", "Contains":
				p.advance()
				right, err := p.parseAddition()
				if err != nil {
					return nil, err
				}
				left = &BinaryExpr{Op: "CONTAINS", Left: left, Right: right}
			default:
				return left, nil
			}
		case TokenIN:
			p.advance()
			right, err := p.parseAddition()
			if err != nil {
				return nil, err
			}
			left = &BinaryExpr{Op: "IN", Left: left, Right: right}
		default:
			return left, nil
		}
	}
}

func (p *Parser) parseAddition() (Expr, error) {
	left, err := p.parseMultiplication()
	if err != nil {
		return nil, err
	}
	for p.peek().Type == TokenPlus || p.peek().Type == TokenMinus {
		op := "+"
		if p.peek().Type == TokenMinus {
			op = "-"
		}
		p.advance()
		right, err := p.parseMultiplication()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Op: op, Left: left, Right: right}
	}
	return left, nil
}

func (p *Parser) parseMultiplication() (Expr, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for p.peek().Type == TokenStar || p.peek().Type == TokenSlash || p.peek().Type == TokenPercent {
		op := "*"
		switch p.peek().Type {
		case TokenSlash:
			op = "/"
		case TokenPercent:
			op = "%"
		}
		p.advance()
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Op: op, Left: left, Right: right}
	}
	return left, nil
}

func (p *Parser) parseUnary() (Expr, error) {
	if p.peek().Type == TokenMinus {
		p.advance()
		right, err := p.parsePrimary()
		if err != nil {
			return nil, err
		}
		return &UnaryExpr{Op: "-", Right: right}, nil
	}
	return p.parsePrimary()
}

func (p *Parser) parsePrimary() (Expr, error) {
	tok := p.peek()

	switch tok.Type {
	case TokenIdentifier:
		p.advance()
		expr := Expr(&IdentifierExpr{Name: tok.Value})

		// Property access n.name
		for p.peek().Type == TokenDot {
			p.advance()
			prop := p.advance()
			if prop.Type != TokenIdentifier {
				return nil, fmt.Errorf("expected property name after '.'")
			}
			expr = &PropertyAccessExpr{Target: expr, Prop: prop.Value}
		}

		// Function call count(n)
		if p.peek().Type == TokenLeftParen {
			p.advance()
			args := make([]Expr, 0)
			distinct := false

			if p.peek().Type == TokenStar {
				p.advance()
				args = append(args, &IdentifierExpr{Name: "*"})
			} else if p.peek().Type == TokenDISTINCT {
				distinct = true
				p.advance()
				if p.peek().Type == TokenStar {
					p.advance()
					args = append(args, &IdentifierExpr{Name: "*"})
				} else if p.peek().Type != TokenRightParen {
					arg, err := p.parseExpr()
					if err != nil {
						return nil, err
					}
					args = append(args, arg)
					for p.peek().Type == TokenComma {
						p.advance()
						arg, err := p.parseExpr()
						if err != nil {
							return nil, err
						}
						args = append(args, arg)
					}
				}
			} else if p.peek().Type != TokenRightParen {
				arg, err := p.parseExpr()
				if err != nil {
					return nil, err
				}
				args = append(args, arg)

				for p.peek().Type == TokenComma {
					p.advance()
					arg, err := p.parseExpr()
					if err != nil {
						return nil, err
					}
					args = append(args, arg)
				}
			}
			if p.peek().Type != TokenRightParen {
				return nil, fmt.Errorf("expected ')' in function call")
			}
			p.advance()
			return &FunctionCallExpr{Name: tok.Value, Args: args, Distinct: distinct}, nil
		}

		return expr, nil

	case TokenString:
		p.advance()
		return &StringLiteralExpr{Value: tok.Value}, nil

	case TokenNumber:
		p.advance()
		val, _ := strconv.ParseFloat(tok.Value, 64)
		return &NumberLiteralExpr{Value: val}, nil

	case TokenTRUE:
		p.advance()
		return &BooleanLiteralExpr{Value: true}, nil

	case TokenFALSE:
		p.advance()
		return &BooleanLiteralExpr{Value: false}, nil

	case TokenNULL:
		p.advance()
		return &NullLiteralExpr{}, nil

	case TokenDollar:
		p.advance()
		if p.peek().Type != TokenIdentifier {
			return nil, fmt.Errorf("expected parameter name after '$'")
		}
		name := p.advance().Value
		return &ParameterExpr{Name: name}, nil

	case TokenLeftParen:
		p.advance()
		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if p.peek().Type != TokenRightParen {
			return nil, fmt.Errorf("expected ')'")
		}
		p.advance()
		return expr, nil

	case TokenLeftBracket:
		return p.parseListLiteral()

	case TokenCASE:
		return p.parseCaseExpr()

	case TokenCOUNT, TokenSUM, TokenAVG, TokenMIN, TokenMAX, TokenCOLLECT:
		// Aggregate functions as keywords
		funcName := p.advance().Value
		if p.peek().Type != TokenLeftParen {
			return nil, fmt.Errorf("expected '(' after aggregate function %s", funcName)
		}
		p.advance()

		distinct := false
		if p.peek().Type == TokenDISTINCT {
			distinct = true
			p.advance()
		}

		var arg Expr
		if p.peek().Type == TokenStar {
			p.advance()
			arg = &IdentifierExpr{Name: "*"}
		} else {
			var err error
			arg, err = p.parseExpr()
			if err != nil {
				return nil, err
			}
		}

		if p.peek().Type != TokenRightParen {
			return nil, fmt.Errorf("expected ')' after aggregate function")
		}
		p.advance()

		return &FunctionCallExpr{
			Name:     funcName,
			Args:     []Expr{arg},
			Distinct: distinct,
		}, nil

	case TokenEXISTS:
		// EXISTS { pattern } or EXISTS(expr)
		p.advance()
		if p.peek().Type == TokenLeftParen {
			p.advance()
			arg, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			if p.peek().Type != TokenRightParen {
				return nil, fmt.Errorf("expected ')' after EXISTS")
			}
			p.advance()
			return &FunctionCallExpr{Name: "exists", Args: []Expr{arg}}, nil
		}
		return nil, fmt.Errorf("expected '(' after EXISTS")

	default:
		return nil, fmt.Errorf("unexpected token in expression: %v (%s)", tok.Type, tok.Value)
	}
}

// parseListLiteral parses a list literal [1, 2, 3]
func (p *Parser) parseListLiteral() (Expr, error) {
	p.advance() // consume [

	elements := make([]Expr, 0)
	for p.peek().Type != TokenRightBracket && p.peek().Type != TokenEOF {
		elem, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		elements = append(elements, elem)

		if p.peek().Type == TokenComma {
			p.advance()
		} else {
			break
		}
	}

	if p.peek().Type != TokenRightBracket {
		return nil, fmt.Errorf("expected ']' in list literal")
	}
	p.advance() // consume ]

	return &ListExpr{Elements: elements}, nil
}

// parseCaseExpr parses a CASE [x] WHEN ... THEN ... [ELSE ...] END expression
func (p *Parser) parseCaseExpr() (Expr, error) {
	p.advance() // consume CASE

	ce := &CaseExpr{}

	// Simple CASE: CASE x WHEN ... THEN ...
	// Generic CASE: CASE WHEN ... THEN ...
	// If the next token is WHEN, it's the generic form; otherwise simple form
	if p.peek().Type != TokenWHEN {
		subject, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		ce.Subject = subject
	}

	// WHEN ... THEN ... branches
	for p.peek().Type == TokenWHEN {
		p.advance() // consume WHEN
		cond, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if p.peek().Type != TokenTHEN {
			return nil, fmt.Errorf("expected THEN after WHEN condition")
		}
		p.advance() // consume THEN
		result, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		ce.Whens = append(ce.Whens, CaseWhen{Condition: cond, Result: result})
	}

	// ELSE
	if p.peek().Type == TokenELSE {
		p.advance()
		elseExpr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		ce.ElseExpr = elseExpr
	}

	if p.peek().Type != TokenEND {
		return nil, fmt.Errorf("expected END after CASE expression")
	}
	p.advance() // consume END

	return ce, nil
}

// parseReturnItem parses a RETURN item
func (p *Parser) parseReturnItem() (*ReturnItem, error) {
	expr, err := p.parseExpr()
	if err != nil {
		return nil, err
	}

	item := &ReturnItem{Expr: expr}

	// AS alias
	if p.peek().Type == TokenAS {
		p.advance()
		if p.peek().Type != TokenIdentifier {
			return nil, fmt.Errorf("expected alias after AS")
		}
		item.Alias = p.advance().Value
	}

	return item, nil
}

// parseOrderByItem parses an ORDER BY item
func (p *Parser) parseOrderByItem() (*OrderByItem, error) {
	expr, err := p.parseExpr()
	if err != nil {
		return nil, err
	}

	item := &OrderByItem{Expr: expr}

	if p.peek().Type == TokenDESC {
		item.Desc = true
		p.advance()
	} else if p.peek().Type == TokenASC {
		p.advance()
	}

	return item, nil
}

// parseProps parses properties {key: value, ...}
func (p *Parser) parseProps() (map[string]Expr, error) {
	props := make(map[string]Expr)

	if p.peek().Type != TokenLeftBrace {
		return nil, fmt.Errorf("expected '{'")
	}
	p.advance()

	for p.peek().Type != TokenRightBrace && p.peek().Type != TokenEOF {
		// key
		if p.peek().Type != TokenIdentifier {
			return nil, fmt.Errorf("expected property key")
		}
		key := p.advance().Value

		// :
		if p.peek().Type != TokenColon {
			return nil, fmt.Errorf("expected ':' after property key")
		}
		p.advance()

		// value
		val, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		props[key] = val

		if p.peek().Type == TokenComma {
			p.advance()
		}
	}

	if p.peek().Type != TokenRightBrace {
		return nil, fmt.Errorf("expected '}'")
	}
	p.advance()

	return props, nil
}

// ============ Helper methods ============

func (p *Parser) peek() Token {
	if p.pos >= len(p.tokens) {
		return Token{Type: TokenEOF}
	}
	return p.tokens[p.pos]
}

func (p *Parser) advance() Token {
	tok := p.peek()
	if p.pos < len(p.tokens) {
		p.pos++
	}
	return tok
}

func (p *Parser) atEnd() bool {
	return p.pos >= len(p.tokens) || p.tokens[p.pos].Type == TokenEOF
}

func intPtr(v int) *int { return &v }

// stringsEqualCI checks if two strings are equal case-insensitively
func stringsEqualCI(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 32
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 32
		}
		if ca != cb {
			return false
		}
	}
	return true
}