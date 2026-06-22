package cypher

import (
	"context"
	"fmt"

	"github.com/mengshi02/codetrip/internal/cypher/operators"
	"github.com/mengshi02/codetrip/internal/graph"
)

// Row is a type alias for operators.Row, preserving backward compatibility.
// Existing code that references cypher.Row will continue to work.
type Row = operators.Row

// Result is a type alias for operators.Result, preserving backward compatibility.
// Existing code that references cypher.Result will continue to work.
type Result = operators.Result

// Executor represents a Cypher query executor (Volcano iterator model)
type Executor struct {
	gs *graph.GraphStore
}

// NewExecutor creates an executor
func NewExecutor(gs *graph.GraphStore) *Executor {
	return &Executor{gs: gs}
}

// Execute executes a Cypher query with the given context for timeout/cancellation.
// Every 1000 rows produced, the executor checks ctx.Err() and returns ErrQueryTimeout
// if the context has expired. This prevents runaway queries on large graphs.
func (e *Executor) Execute(ctx context.Context, query string, params map[string]any) (*Result, error) {
	lexer := NewLexer(query)
	tokens := lexer.Tokenize()
	if len(tokens) > 0 && tokens[0].Type == TokenError {
		return nil, fmt.Errorf("lexer error: %s", tokens[0].Value)
	}

	parser := NewParser(tokens)
	ast, err := parser.Parse()
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	return e.executeAST(ctx, ast, params)
}

// executeAST builds a Volcano iterator plan and drains it to produce results.
// It checks ctx.Err() every 1000 rows to support query timeout/cancellation.
func (e *Executor) executeAST(ctx context.Context, query *Query, params map[string]any) (*Result, error) {
	// Check for UNION: split at UNION clause and execute both sides
	var leftClauses, rightClauses []Clause
	var unionAll bool
	for i, clause := range query.Clauses {
		if uc, ok := clause.(*UnionClause); ok {
			unionAll = uc.All
			leftClauses = query.Clauses[:i]
			rightClauses = query.Clauses[i+1:]
			break
		}
	}

	if len(leftClauses) > 0 && len(rightClauses) > 0 {
		return e.executeUnion(ctx, leftClauses, rightClauses, unionAll, params)
	}

	plan, err := BuildPlan(query, e.gs, params)
	if err != nil {
		return nil, err
	}
	defer plan.Root.Close()

	var rows []Row
	for i := 0; ; i++ {
		row, err := plan.Root.Next()
		if err != nil {
			return nil, err
		}
		if row == nil {
			break
		}
		rows = append(rows, row)

		// Check context every 1000 rows to support timeout/cancellation
		if i%1000 == 0 && i > 0 {
			if ctx.Err() != nil {
				return nil, fmt.Errorf("cypher query: %w", ctx.Err())
			}
		}
	}

	columns := plan.Columns
	if columns == nil {
		columns = operators.CollectColumns(rows)
	}

	return &Result{Rows: rows, Columns: columns}, nil
}

// executeUnion executes a UNION [ALL] query by running both sides separately.
func (e *Executor) executeUnion(ctx context.Context, leftClauses, rightClauses []Clause, unionAll bool, params map[string]any) (*Result, error) {
	leftQuery := &Query{Clauses: leftClauses}
	rightQuery := &Query{Clauses: rightClauses}

	leftResult, err := e.executeAST(ctx, leftQuery, params)
	if err != nil {
		return nil, err
	}

	rightResult, err := e.executeAST(ctx, rightQuery, params)
	if err != nil {
		return nil, err
	}

	var rows []Row
	rows = append(rows, leftResult.Rows...)
	rows = append(rows, rightResult.Rows...)

	if !unionAll {
		// UNION (without ALL) removes duplicates
		rows = operators.DeduplicateRows(rows, leftResult.Columns)
	}

	columns := leftResult.Columns
	if columns == nil {
		columns = operators.CollectColumns(rows)
	}

	return &Result{Rows: rows, Columns: columns}, nil
}