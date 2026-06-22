package cypher

import (
	"strings"
	"unicode"
)

// TokenType represents a lexical token type
type TokenType int

const (
	TokenEOF TokenType = iota
	TokenError
	TokenIdentifier
	TokenString
	TokenNumber
	TokenColon      // :
	TokenComma      // ,
	TokenDot        // .
	TokenLeftParen  // (
	TokenRightParen // )
	TokenLeftBracket  // [
	TokenRightBracket // ]
	TokenLeftBrace    // {
	TokenRightBrace   // }
	TokenDollar       // $
	TokenStar         // *
	TokenPipe         // |
	TokenUnderscore   // _

	// Arrow tokens (for relationship patterns)
	TokenArrowRight  // ->
	TokenArrowLeft   // <-
	TokenLessThan    // <
	TokenGreaterThan // >
	TokenLessEq      // <=
	TokenGreaterEq   // >=
	TokenNotEq       // <> or !=
	TokenEquals      // =
	TokenPlus        // +
	TokenMinus       // -
	TokenSlash       // /
	TokenPercent     // %
	TokenExclamation // !

	// Keywords
	TokenMATCH
	TokenRETURN
	TokenWHERE
	TokenORDER
	TokenBY
	TokenLIMIT
	TokenSKIP
	TokenAS
	TokenAND
	TokenOR
	TokenNOT
	TokenIN
	TokenIS
	TokenNULL
	TokenTRUE
	TokenFALSE
	TokenDISTINCT
	TokenOPTIONAL
	TokenUNION
	TokenALL
	TokenWITH
	TokenSET
	TokenCREATE
	TokenDELETE
	TokenMERGE
	TokenREMOVE
	TokenCOUNT
	TokenSUM
	TokenAVG
	TokenMIN
	TokenMAX
	TokenCOLLECT
	TokenASC
	TokenDESC
	TokenUNWIND
	TokenCASE
	TokenWHEN
	TokenTHEN
	TokenELSE
	TokenEND
	TokenEXISTS
	TokenDETACH
)

// Token represents a lexical token
type Token struct {
	Type  TokenType
	Value string
	Pos   int
}

// Lexer represents a Cypher lexer
type Lexer struct {
	input  string
	pos    int
	start  int
	width  int
	tokens []Token
}

// NewLexer creates a lexer
func NewLexer(input string) *Lexer {
	return &Lexer{
		input:  input,
		tokens: make([]Token, 0),
	}
}

// Tokenize performs lexical analysis
func (l *Lexer) Tokenize() []Token {
	for {
		tok := l.nextToken()
		l.tokens = append(l.tokens, tok)
		if tok.Type == TokenEOF || tok.Type == TokenError {
			break
		}
	}
	return l.tokens
}

func (l *Lexer) nextToken() Token {
	l.skipWhitespace()

	if l.pos >= len(l.input) {
		return Token{Type: TokenEOF, Pos: l.pos}
	}

	l.start = l.pos
	ch := l.peek()

	switch {
	case isLetter(ch):
		return l.readIdentifier()
	case isDigit(ch):
		return l.readNumber()
	case ch == '"':
		return l.readString('"')
	case ch == '\'':
		return l.readString('\'')
	case ch == '-':
		l.advance()
		if l.peek() == '>' {
			l.advance()
			return Token{Type: TokenArrowRight, Value: "->", Pos: l.start}
		}
		return Token{Type: TokenMinus, Value: "-", Pos: l.start}
	case ch == '<':
		l.advance()
		switch l.peek() {
		case '-':
			l.advance()
			return Token{Type: TokenArrowLeft, Value: "<-", Pos: l.start}
		case '=':
			l.advance()
			return Token{Type: TokenLessEq, Value: "<=", Pos: l.start}
		case '>':
			l.advance()
			return Token{Type: TokenNotEq, Value: "<>", Pos: l.start}
		default:
			return Token{Type: TokenLessThan, Value: "<", Pos: l.start}
		}
	case ch == '>':
		l.advance()
		if l.peek() == '=' {
			l.advance()
			return Token{Type: TokenGreaterEq, Value: ">=", Pos: l.start}
		}
		return Token{Type: TokenGreaterThan, Value: ">", Pos: l.start}
	case ch == '=':
		l.advance()
		return Token{Type: TokenEquals, Value: "=", Pos: l.start}
	case ch == '!':
		l.advance()
		if l.peek() == '=' {
			l.advance()
			return Token{Type: TokenNotEq, Value: "!=", Pos: l.start}
		}
		return Token{Type: TokenExclamation, Value: "!", Pos: l.start}
	case ch == ':':
		l.advance()
		return Token{Type: TokenColon, Value: ":", Pos: l.start}
	case ch == ',':
		l.advance()
		return Token{Type: TokenComma, Value: ",", Pos: l.start}
	case ch == '.':
		l.advance()
		return Token{Type: TokenDot, Value: ".", Pos: l.start}
	case ch == '(':
		l.advance()
		return Token{Type: TokenLeftParen, Value: "(", Pos: l.start}
	case ch == ')':
		l.advance()
		return Token{Type: TokenRightParen, Value: ")", Pos: l.start}
	case ch == '[':
		l.advance()
		return Token{Type: TokenLeftBracket, Value: "[", Pos: l.start}
	case ch == ']':
		l.advance()
		return Token{Type: TokenRightBracket, Value: "]", Pos: l.start}
	case ch == '{':
		l.advance()
		return Token{Type: TokenLeftBrace, Value: "{", Pos: l.start}
	case ch == '}':
		l.advance()
		return Token{Type: TokenRightBrace, Value: "}", Pos: l.start}
	case ch == '$':
		l.advance()
		return Token{Type: TokenDollar, Value: "$", Pos: l.start}
	case ch == '*':
		l.advance()
		return Token{Type: TokenStar, Value: "*", Pos: l.start}
	case ch == '|':
		l.advance()
		return Token{Type: TokenPipe, Value: "|", Pos: l.start}
	case ch == '_':
		l.advance()
		return Token{Type: TokenUnderscore, Value: "_", Pos: l.start}
	case ch == '+':
		l.advance()
		return Token{Type: TokenPlus, Value: "+", Pos: l.start}
	case ch == '/':
		l.advance()
		return Token{Type: TokenSlash, Value: "/", Pos: l.start}
	case ch == '%':
		l.advance()
		return Token{Type: TokenPercent, Value: "%", Pos: l.start}
	default:
		l.advance()
		return Token{Type: TokenError, Value: string(ch), Pos: l.start}
	}
}

func (l *Lexer) readIdentifier() Token {
	for isAlphaNumeric(l.peek()) {
		l.advance()
	}
	value := l.input[l.start:l.pos]

	// Check keywords
	typ := lookupKeyword(value)
	return Token{Type: typ, Value: value, Pos: l.start}
}

func (l *Lexer) readNumber() Token {
	for isDigit(l.peek()) {
		l.advance()
	}
	if l.peek() == '.' {
		// Lookahead: only treat as decimal if the char after '.' is a digit
		if l.pos+1 < len(l.input) && isDigit(l.input[l.pos+1]) {
			l.advance() // consume .
			for isDigit(l.peek()) {
				l.advance()
			}
		}
	}
	return Token{Type: TokenNumber, Value: l.input[l.start:l.pos], Pos: l.start}
}

func (l *Lexer) readString(quote byte) Token {
	l.advance() // Skip opening quote
	for l.peek() != quote && l.peek() != 0 {
		if l.peek() == '\\' {
			l.advance()
		}
		l.advance()
	}
	if l.peek() == 0 {
		return Token{Type: TokenError, Value: "unterminated string", Pos: l.start}
	}
	l.advance() // Skip closing quote
	return Token{Type: TokenString, Value: l.input[l.start+1 : l.pos-1], Pos: l.start}
}

func (l *Lexer) skipWhitespace() {
	for unicode.IsSpace(rune(l.peek())) {
		l.advance()
	}
}

func (l *Lexer) peek() byte {
	if l.pos >= len(l.input) {
		return 0
	}
	return l.input[l.pos]
}

func (l *Lexer) advance() {
	if l.pos < len(l.input) {
		l.pos++
	}
}

func isLetter(ch byte) bool  { return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') }
func isDigit(ch byte) bool   { return ch >= '0' && ch <= '9' }
func isAlphaNumeric(ch byte) bool { return isLetter(ch) || isDigit(ch) || ch == '_' }

func lookupKeyword(ident string) TokenType {
	switch strings.ToUpper(ident) {
	case "MATCH":
		return TokenMATCH
	case "RETURN":
		return TokenRETURN
	case "WHERE":
		return TokenWHERE
	case "ORDER":
		return TokenORDER
	case "BY":
		return TokenBY
	case "LIMIT":
		return TokenLIMIT
	case "SKIP":
		return TokenSKIP
	case "AS":
		return TokenAS
	case "AND":
		return TokenAND
	case "OR":
		return TokenOR
	case "NOT":
		return TokenNOT
	case "IN":
		return TokenIN
	case "IS":
		return TokenIS
	case "NULL":
		return TokenNULL
	case "TRUE":
		return TokenTRUE
	case "FALSE":
		return TokenFALSE
	case "DISTINCT":
		return TokenDISTINCT
	case "OPTIONAL":
		return TokenOPTIONAL
	case "UNION":
		return TokenUNION
	case "ALL":
		return TokenALL
	case "WITH":
		return TokenWITH
	case "SET":
		return TokenSET
	case "CREATE":
		return TokenCREATE
	case "DELETE":
		return TokenDELETE
	case "MERGE":
		return TokenMERGE
	case "REMOVE":
		return TokenREMOVE
	case "COUNT":
		return TokenCOUNT
	case "SUM":
		return TokenSUM
	case "AVG":
		return TokenAVG
	case "MIN":
		return TokenMIN
	case "MAX":
		return TokenMAX
	case "COLLECT":
		return TokenCOLLECT
	case "ASC":
		return TokenASC
	case "DESC":
		return TokenDESC
	case "UNWIND":
		return TokenUNWIND
	case "CASE":
		return TokenCASE
	case "WHEN":
		return TokenWHEN
	case "THEN":
		return TokenTHEN
	case "ELSE":
		return TokenELSE
	case "END":
		return TokenEND
	case "EXISTS":
		return TokenEXISTS
	case "DETACH":
		return TokenDETACH
	default:
		return TokenIdentifier
	}
}