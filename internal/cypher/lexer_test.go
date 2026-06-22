package cypher

import (
	"testing"
)

// --------------- Unit Tests ---------------

func TestLexerEmpty(t *testing.T) {
	l := NewLexer("")
	tokens := l.Tokenize()
	if len(tokens) != 1 {
		t.Fatalf("expected 1 token (EOF), got %d", len(tokens))
	}
	if tokens[0].Type != TokenEOF {
		t.Fatalf("expected TokenEOF, got %v", tokens[0].Type)
	}
}

func TestLexerSimpleQuery(t *testing.T) {
	input := "MATCH (n:Person) RETURN n"
	l := NewLexer(input)
	tokens := l.Tokenize()

	expected := []struct {
		typ  TokenType
		val  string
	}{
		{TokenMATCH, "MATCH"},
		{TokenLeftParen, "("},
		{TokenIdentifier, "n"},
		{TokenColon, ":"},
		{TokenIdentifier, "Person"},
		{TokenRightParen, ")"},
		{TokenRETURN, "RETURN"},
		{TokenIdentifier, "n"},
	}

	if len(tokens) < len(expected)+1 { // +1 for EOF
		t.Fatalf("expected at least %d tokens, got %d", len(expected)+1, len(tokens))
	}
	for i, exp := range expected {
		if tokens[i].Type != exp.typ {
			t.Errorf("token %d: expected type %v, got %v", i, exp.typ, tokens[i].Type)
		}
		if tokens[i].Value != exp.val {
			t.Errorf("token %d: expected value %q, got %q", i, exp.val, tokens[i].Value)
		}
	}
	if tokens[len(expected)].Type != TokenEOF {
		t.Errorf("last token should be EOF, got %v", tokens[len(expected)].Type)
	}
}

func TestLexerStringLiterals(t *testing.T) {
	tests := []struct {
		input string
		val   string
	}{
		{`"hello"`, "hello"},
		{`'world'`, "world"},
		{`"escape\"d"`, `escape\"d`},
	}

	for _, tt := range tests {
		l := NewLexer(tt.input)
		tokens := l.Tokenize()
		if len(tokens) < 2 {
			t.Fatalf("input %q: expected at least 2 tokens", tt.input)
		}
		if tokens[0].Type != TokenString {
			t.Errorf("input %q: expected TokenString, got %v", tt.input, tokens[0].Type)
		}
		if tokens[0].Value != tt.val {
			t.Errorf("input %q: expected value %q, got %q", tt.input, tt.val, tokens[0].Value)
		}
	}
}

func TestLexerNumbers(t *testing.T) {
	tests := []struct {
		input string
		val   string
	}{
		{"42", "42"},
		{"3.14", "3.14"},
		{"0", "0"},
		{"100.5", "100.5"},
	}

	for _, tt := range tests {
		l := NewLexer(tt.input)
		tokens := l.Tokenize()
		if len(tokens) < 2 {
			t.Fatalf("input %q: expected at least 2 tokens", tt.input)
		}
		if tokens[0].Type != TokenNumber {
			t.Errorf("input %q: expected TokenNumber, got %v", tt.input, tokens[0].Type)
		}
		if tokens[0].Value != tt.val {
			t.Errorf("input %q: expected value %q, got %q", tt.input, tt.val, tokens[0].Value)
		}
	}
}

func TestLexerArrows(t *testing.T) {
	tests := []struct {
		input   string
		tokType TokenType
		val     string
	}{
		{"->", TokenArrowRight, "->"},
		{"<-", TokenArrowLeft, "<-"},
		{"<=", TokenLessEq, "<="},
		{">=", TokenGreaterEq, ">="},
		{"<>", TokenNotEq, "<>"},
		{"!=", TokenNotEq, "!="},
	}

	for _, tt := range tests {
		l := NewLexer(tt.input)
		tokens := l.Tokenize()
		if len(tokens) < 2 {
			t.Fatalf("input %q: expected at least 2 tokens", tt.input)
		}
		if tokens[0].Type != tt.tokType {
			t.Errorf("input %q: expected type %v, got %v", tt.input, tt.tokType, tokens[0].Type)
		}
		if tokens[0].Value != tt.val {
			t.Errorf("input %q: expected value %q, got %q", tt.input, tt.val, tokens[0].Value)
		}
	}
}

func TestLexerKeywords(t *testing.T) {
	tests := []struct {
		input   string
		tokType TokenType
	}{
		{"MATCH", TokenMATCH},
		{"RETURN", TokenRETURN},
		{"WHERE", TokenWHERE},
		{"ORDER", TokenORDER},
		{"BY", TokenBY},
		{"LIMIT", TokenLIMIT},
		{"SKIP", TokenSKIP},
		{"AND", TokenAND},
		{"OR", TokenOR},
		{"NOT", TokenNOT},
		{"IN", TokenIN},
		{"IS", TokenIS},
		{"NULL", TokenNULL},
		{"TRUE", TokenTRUE},
		{"FALSE", TokenFALSE},
		{"DISTINCT", TokenDISTINCT},
		{"OPTIONAL", TokenOPTIONAL},
		{"UNION", TokenUNION},
		{"ALL", TokenALL},
		{"WITH", TokenWITH},
		{"SET", TokenSET},
		{"CREATE", TokenCREATE},
		{"DELETE", TokenDELETE},
		{"MERGE", TokenMERGE},
		{"REMOVE", TokenREMOVE},
		{"COUNT", TokenCOUNT},
		{"SUM", TokenSUM},
		{"AVG", TokenAVG},
		{"MIN", TokenMIN},
		{"MAX", TokenMAX},
		{"COLLECT", TokenCOLLECT},
		{"ASC", TokenASC},
		{"DESC", TokenDESC},
		{"UNWIND", TokenUNWIND},
		{"CASE", TokenCASE},
		{"WHEN", TokenWHEN},
		{"THEN", TokenTHEN},
		{"ELSE", TokenELSE},
		{"END", TokenEND},
		{"EXISTS", TokenEXISTS},
		{"DETACH", TokenDETACH},
	}

	for _, tt := range tests {
		l := NewLexer(tt.input)
		tokens := l.Tokenize()
		if len(tokens) < 2 {
			t.Fatalf("keyword %q: expected at least 2 tokens", tt.input)
		}
		if tokens[0].Type != tt.tokType {
			t.Errorf("keyword %q: expected type %v, got %v (value=%q)", tt.input, tt.tokType, tokens[0].Type, tokens[0].Value)
		}
	}
}

func TestLexerKeywordsCaseInsensitive(t *testing.T) {
	tests := []struct {
		input   string
		tokType TokenType
	}{
		{"match", TokenMATCH},
		{"Match", TokenMATCH},
		{"RETURN", TokenRETURN},
		{"return", TokenRETURN},
	}

	for _, tt := range tests {
		l := NewLexer(tt.input)
		tokens := l.Tokenize()
		if len(tokens) < 2 {
			t.Fatalf("keyword %q: expected at least 2 tokens", tt.input)
		}
		if tokens[0].Type != tt.tokType {
			t.Errorf("keyword %q: expected type %v, got %v", tt.input, tt.tokType, tokens[0].Type)
		}
	}
}

func TestLexerPunctuation(t *testing.T) {
	tests := []struct {
		input   string
		tokType TokenType
	}{
		{":", TokenColon},
		{",", TokenComma},
		{".", TokenDot},
		{"(", TokenLeftParen},
		{")", TokenRightParen},
		{"[", TokenLeftBracket},
		{"]", TokenRightBracket},
		{"{", TokenLeftBrace},
		{"}", TokenRightBrace},
		{"$", TokenDollar},
		{"*", TokenStar},
		{"|", TokenPipe},
		{"+", TokenPlus},
		{"/", TokenSlash},
		{"%", TokenPercent},
		{"=", TokenEquals},
		{"<", TokenLessThan},
		{">", TokenGreaterThan},
	}

	for _, tt := range tests {
		l := NewLexer(tt.input)
		tokens := l.Tokenize()
		if len(tokens) < 2 {
			t.Fatalf("punct %q: expected at least 2 tokens", tt.input)
		}
		if tokens[0].Type != tt.tokType {
			t.Errorf("punct %q: expected type %v, got %v", tt.input, tt.tokType, tokens[0].Type)
		}
	}
}

func TestLexerUnterminatedString(t *testing.T) {
	l := NewLexer(`"unterminated`)
	tokens := l.Tokenize()
	if len(tokens) < 1 {
		t.Fatal("expected at least 1 token")
	}
	if tokens[0].Type != TokenError {
		t.Errorf("expected TokenError for unterminated string, got %v", tokens[0].Type)
	}
}

func TestLexerComplexQuery(t *testing.T) {
	input := `MATCH (n:Person)-[r:KNOWS]->(m:Person) WHERE n.name = 'Alice' RETURN n.name, m.name ORDER BY n.name LIMIT 10`
	l := NewLexer(input)
	tokens := l.Tokenize()

	expectedTypes := []TokenType{
		TokenMATCH, TokenLeftParen, TokenIdentifier, TokenColon, TokenIdentifier,
		TokenRightParen, TokenMinus, TokenLeftBracket, TokenIdentifier, TokenColon,
		TokenIdentifier, TokenRightBracket, TokenArrowRight, TokenLeftParen,
		TokenIdentifier, TokenColon, TokenIdentifier, TokenRightParen, TokenWHERE,
		TokenIdentifier, TokenDot, TokenIdentifier, TokenEquals, TokenString,
		TokenRETURN, TokenIdentifier, TokenDot, TokenIdentifier, TokenComma,
		TokenIdentifier, TokenDot, TokenIdentifier, TokenORDER, TokenBY,
		TokenIdentifier, TokenDot, TokenIdentifier, TokenLIMIT, TokenNumber,
		TokenEOF,
	}

	if len(tokens) != len(expectedTypes) {
		t.Fatalf("expected %d tokens, got %d", len(expectedTypes), len(tokens))
	}
	for i, exp := range expectedTypes {
		if tokens[i].Type != exp {
			t.Errorf("token %d: expected type %v, got %v (value=%q)", i, exp, tokens[i].Type, tokens[i].Value)
		}
	}
}

func TestLexerParameter(t *testing.T) {
	input := "$name"
	l := NewLexer(input)
	tokens := l.Tokenize()
	if len(tokens) < 3 {
		t.Fatal("expected at least 3 tokens ($ name EOF)")
	}
	if tokens[0].Type != TokenDollar {
		t.Errorf("expected TokenDollar, got %v", tokens[0].Type)
	}
	if tokens[1].Type != TokenIdentifier || tokens[1].Value != "name" {
		t.Errorf("expected identifier 'name', got %v (%q)", tokens[1].Type, tokens[1].Value)
	}
}

func TestLexerDecimalNotDangling(t *testing.T) {
	// "3." should NOT be treated as 3.0 — the lexer only treats as decimal
	// when the char after '.' is also a digit.
	input := "3.x"
	l := NewLexer(input)
	tokens := l.Tokenize()
	if tokens[0].Type != TokenNumber || tokens[0].Value != "3" {
		t.Errorf("expected number '3', got %v (%q)", tokens[0].Type, tokens[0].Value)
	}
	if tokens[1].Type != TokenDot {
		t.Errorf("expected dot, got %v", tokens[1].Type)
	}
	if tokens[2].Type != TokenIdentifier || tokens[2].Value != "x" {
		t.Errorf("expected identifier 'x', got %v (%q)", tokens[2].Type, tokens[2].Value)
	}
}

// --------------- Benchmarks ---------------

func BenchmarkLexerSimple(b *testing.B) {
	input := "MATCH (n:Person) RETURN n"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l := NewLexer(input)
		l.Tokenize()
	}
}

func BenchmarkLexerComplex(b *testing.B) {
	input := "MATCH (n:Person)-[r:KNOWS*1..3]->(m:City) WHERE n.name STARTS WITH 'A' AND m.population > 1000000 RETURN n.name AS name, count(r) AS cnt ORDER BY cnt DESC LIMIT 50"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l := NewLexer(input)
		l.Tokenize()
	}
}

func BenchmarkLexerKeywords(b *testing.B) {
	input := "MATCH OPTIONAL UNION ALL WHERE ORDER BY LIMIT SKIP DISTINCT CREATE DELETE MERGE REMOVE SET WITH RETURN AND OR NOT IN IS NULL TRUE FALSE"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l := NewLexer(input)
		l.Tokenize()
	}
}