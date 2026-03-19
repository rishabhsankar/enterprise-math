// Package expression provides a recursive-descent parser for mathematical
// expressions, producing an AST that can be evaluated through the operation system.
package expression

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/rishabhsankar/enterprise-math/pkg/operations"
)

// TokenType classifies lexer tokens.
type TokenType int

const (
	TokenNumber TokenType = iota
	TokenPlus
	TokenMinus
	TokenStar
	TokenSlash
	TokenPercent
	TokenCaret
	TokenLParen
	TokenRParen
	TokenComma
	TokenIdent
	TokenEOF
)

// Token represents a single lexical token.
type Token struct {
	Type    TokenType
	Value   string
	Pos     int
}

// Lexer tokenizes mathematical expression strings.
type Lexer struct {
	input   string
	pos     int
	tokens  []Token
}

// NewLexer creates a new lexer for the given input.
func NewLexer(input string) *Lexer {
	return &Lexer{input: input}
}

// Tokenize breaks the input string into tokens.
func (l *Lexer) Tokenize() ([]Token, error) {
	l.tokens = make([]Token, 0)

	for l.pos < len(l.input) {
		ch := rune(l.input[l.pos])

		if unicode.IsSpace(ch) {
			l.pos++
			continue
		}

		if unicode.IsDigit(ch) || ch == '.' {
			l.readNumber()
			continue
		}

		if unicode.IsLetter(ch) || ch == '_' {
			l.readIdent()
			continue
		}

		switch ch {
		case '+':
			l.tokens = append(l.tokens, Token{Type: TokenPlus, Value: "+", Pos: l.pos})
		case '-':
			l.tokens = append(l.tokens, Token{Type: TokenMinus, Value: "-", Pos: l.pos})
		case '*':
			l.tokens = append(l.tokens, Token{Type: TokenStar, Value: "*", Pos: l.pos})
		case '/':
			l.tokens = append(l.tokens, Token{Type: TokenSlash, Value: "/", Pos: l.pos})
		case '%':
			l.tokens = append(l.tokens, Token{Type: TokenPercent, Value: "%", Pos: l.pos})
		case '^':
			l.tokens = append(l.tokens, Token{Type: TokenCaret, Value: "^", Pos: l.pos})
		case '(':
			l.tokens = append(l.tokens, Token{Type: TokenLParen, Value: "(", Pos: l.pos})
		case ')':
			l.tokens = append(l.tokens, Token{Type: TokenRParen, Value: ")", Pos: l.pos})
		case ',':
			l.tokens = append(l.tokens, Token{Type: TokenComma, Value: ",", Pos: l.pos})
		default:
			return nil, fmt.Errorf("unexpected character '%c' at position %d", ch, l.pos)
		}
		l.pos++
	}

	l.tokens = append(l.tokens, Token{Type: TokenEOF, Pos: l.pos})
	return l.tokens, nil
}

func (l *Lexer) readNumber() {
	start := l.pos
	hasDot := false
	for l.pos < len(l.input) {
		ch := rune(l.input[l.pos])
		if ch == '.' && !hasDot {
			hasDot = true
			l.pos++
		} else if unicode.IsDigit(ch) {
			l.pos++
		} else {
			break
		}
	}
	l.tokens = append(l.tokens, Token{Type: TokenNumber, Value: l.input[start:l.pos], Pos: start})
}

func (l *Lexer) readIdent() {
	start := l.pos
	for l.pos < len(l.input) {
		ch := rune(l.input[l.pos])
		if unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == '_' {
			l.pos++
		} else {
			break
		}
	}
	l.tokens = append(l.tokens, Token{Type: TokenIdent, Value: l.input[start:l.pos], Pos: start})
}

// Node represents a node in the expression AST.
type Node interface {
	String() string
	Evaluate(ctx context.Context, factory operations.OperationFactory) (*operations.OperationResult, error)
	Complexity() int
}

// NumberNode represents a literal number.
type NumberNode struct {
	Value float64
}

func (n *NumberNode) String() string { return fmt.Sprintf("%g", n.Value) }
func (n *NumberNode) Complexity() int { return 1 }

func (n *NumberNode) Evaluate(_ context.Context, _ operations.OperationFactory) (*operations.OperationResult, error) {
	return &operations.OperationResult{
		Value:    operations.NewNumber(n.Value),
		Strategy: "literal",
	}, nil
}

// BinaryNode represents a binary operation (a op b).
type BinaryNode struct {
	Op    string
	Left  Node
	Right Node
}

func (n *BinaryNode) String() string {
	return fmt.Sprintf("(%s %s %s)", n.Left.String(), n.Op, n.Right.String())
}

func (n *BinaryNode) Complexity() int {
	return 1 + n.Left.Complexity() + n.Right.Complexity()
}

func (n *BinaryNode) Evaluate(ctx context.Context, factory operations.OperationFactory) (*operations.OperationResult, error) {
	start := time.Now()

	leftResult, err := n.Left.Evaluate(ctx, factory)
	if err != nil {
		return nil, fmt.Errorf("left operand: %w", err)
	}

	rightResult, err := n.Right.Evaluate(ctx, factory)
	if err != nil {
		return nil, fmt.Errorf("right operand: %w", err)
	}

	opMap := map[string]string{
		"+": "add", "-": "subtract", "*": "multiply",
		"/": "divide", "%": "modulo", "^": "power",
	}

	opName, exists := opMap[n.Op]
	if !exists {
		return nil, fmt.Errorf("unknown operator: %s", n.Op)
	}

	op, err := factory.Create(opName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create operation %s: %w", opName, err)
	}

	result, err := op.Execute(ctx, leftResult.Value, rightResult.Value)
	if err != nil {
		return nil, err
	}

	result.Duration = time.Since(start)
	return result, nil
}

// UnaryNode represents a unary operation (-x).
type UnaryNode struct {
	Op      string
	Operand Node
}

func (n *UnaryNode) String() string {
	return fmt.Sprintf("(%s%s)", n.Op, n.Operand.String())
}

func (n *UnaryNode) Complexity() int { return 1 + n.Operand.Complexity() }

func (n *UnaryNode) Evaluate(ctx context.Context, factory operations.OperationFactory) (*operations.OperationResult, error) {
	result, err := n.Operand.Evaluate(ctx, factory)
	if err != nil {
		return nil, err
	}
	if n.Op == "-" {
		result.Value.Value = -result.Value.Value
	}
	return result, nil
}

// FunctionCallNode represents a function call like sin(x).
type FunctionCallNode struct {
	Name string
	Args []Node
}

func (n *FunctionCallNode) String() string {
	args := make([]string, len(n.Args))
	for i, a := range n.Args {
		args[i] = a.String()
	}
	return fmt.Sprintf("%s(%s)", n.Name, strings.Join(args, ", "))
}

func (n *FunctionCallNode) Complexity() int {
	c := 2
	for _, arg := range n.Args {
		c += arg.Complexity()
	}
	return c
}

func (n *FunctionCallNode) Evaluate(ctx context.Context, factory operations.OperationFactory) (*operations.OperationResult, error) {
	start := time.Now()

	argResults := make([]operations.Number, len(n.Args))
	for i, arg := range n.Args {
		result, err := arg.Evaluate(ctx, factory)
		if err != nil {
			return nil, fmt.Errorf("argument %d of %s: %w", i, n.Name, err)
		}
		argResults[i] = result.Value
	}

	op, err := factory.Create(n.Name, nil)
	if err != nil {
		return nil, fmt.Errorf("unknown function: %s", n.Name)
	}

	result, err := op.Execute(ctx, argResults...)
	if err != nil {
		return nil, err
	}

	result.Duration = time.Since(start)
	return result, nil
}

// Parser performs recursive-descent parsing of tokenized expressions.
type Parser struct {
	tokens []Token
	pos    int
}

// NewParser creates a parser from a token stream.
func NewParser(tokens []Token) *Parser {
	return &Parser{tokens: tokens}
}

// Parse parses the token stream into an AST.
func (p *Parser) Parse() (Node, error) {
	node, err := p.parseExpression()
	if err != nil {
		return nil, err
	}

	if p.current().Type != TokenEOF {
		return nil, fmt.Errorf("unexpected token at position %d: %s", p.current().Pos, p.current().Value)
	}

	return node, nil
}

func (p *Parser) current() Token {
	if p.pos >= len(p.tokens) {
		return Token{Type: TokenEOF}
	}
	return p.tokens[p.pos]
}

func (p *Parser) advance() Token {
	tok := p.current()
	p.pos++
	return tok
}

func (p *Parser) expect(t TokenType) (Token, error) {
	tok := p.current()
	if tok.Type != t {
		return tok, fmt.Errorf("expected token type %d, got %d at position %d", t, tok.Type, tok.Pos)
	}
	p.advance()
	return tok, nil
}

func (p *Parser) parseExpression() (Node, error) {
	return p.parseAdditive()
}

func (p *Parser) parseAdditive() (Node, error) {
	left, err := p.parseMultiplicative()
	if err != nil {
		return nil, err
	}

	for p.current().Type == TokenPlus || p.current().Type == TokenMinus {
		op := p.advance().Value
		right, err := p.parseMultiplicative()
		if err != nil {
			return nil, err
		}
		left = &BinaryNode{Op: op, Left: left, Right: right}
	}

	return left, nil
}

func (p *Parser) parseMultiplicative() (Node, error) {
	left, err := p.parseExponent()
	if err != nil {
		return nil, err
	}

	for p.current().Type == TokenStar || p.current().Type == TokenSlash || p.current().Type == TokenPercent {
		op := p.advance().Value
		right, err := p.parseExponent()
		if err != nil {
			return nil, err
		}
		left = &BinaryNode{Op: op, Left: left, Right: right}
	}

	return left, nil
}

func (p *Parser) parseExponent() (Node, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}

	if p.current().Type == TokenCaret {
		p.advance()
		right, err := p.parseExponent() // right-associative
		if err != nil {
			return nil, err
		}
		left = &BinaryNode{Op: "^", Left: left, Right: right}
	}

	return left, nil
}

func (p *Parser) parseUnary() (Node, error) {
	if p.current().Type == TokenMinus {
		p.advance()
		operand, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &UnaryNode{Op: "-", Operand: operand}, nil
	}
	if p.current().Type == TokenPlus {
		p.advance()
		return p.parseUnary()
	}
	return p.parsePrimary()
}

func (p *Parser) parsePrimary() (Node, error) {
	tok := p.current()

	switch tok.Type {
	case TokenNumber:
		p.advance()
		val, err := strconv.ParseFloat(tok.Value, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid number: %s", tok.Value)
		}
		return &NumberNode{Value: val}, nil

	case TokenIdent:
		p.advance()
		if p.current().Type == TokenLParen {
			return p.parseFunctionCall(tok.Value)
		}
		// Treat as constant
		constants := map[string]float64{
			"pi": 3.141592653589793,
			"e":  2.718281828459045,
			"phi": 1.618033988749895,
			"tau": 6.283185307179586,
		}
		if val, exists := constants[strings.ToLower(tok.Value)]; exists {
			return &NumberNode{Value: val}, nil
		}
		return nil, fmt.Errorf("unknown identifier: %s", tok.Value)

	case TokenLParen:
		p.advance()
		expr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(TokenRParen); err != nil {
			return nil, fmt.Errorf("missing closing parenthesis")
		}
		return expr, nil

	default:
		return nil, fmt.Errorf("unexpected token: %s at position %d", tok.Value, tok.Pos)
	}
}

func (p *Parser) parseFunctionCall(name string) (Node, error) {
	p.advance() // consume '('

	args := make([]Node, 0)
	if p.current().Type != TokenRParen {
		arg, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		args = append(args, arg)

		for p.current().Type == TokenComma {
			p.advance()
			arg, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			args = append(args, arg)
		}
	}

	if _, err := p.expect(TokenRParen); err != nil {
		return nil, fmt.Errorf("missing closing parenthesis in function call %s", name)
	}

	return &FunctionCallNode{Name: name, Args: args}, nil
}

// ParseExpression parses a string expression into an evaluable AST.
func ParseExpression(input string) (Node, error) {
	lexer := NewLexer(input)
	tokens, err := lexer.Tokenize()
	if err != nil {
		return nil, fmt.Errorf("lexer error: %w", err)
	}

	parser := NewParser(tokens)
	return parser.Parse()
}
