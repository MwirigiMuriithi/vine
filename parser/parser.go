// Package parser implements SYNTACTIC ANALYSIS for the Vine compiler.
//
// ─── Technique: Recursive Descent ─────────────────────────────────────────────
// Each grammar rule is a Go method. Methods call each other, recursively, to
// handle nested constructs (expressions inside conditions inside functions).
//
// ─── Vine Grammar (simplified) ────────────────────────────────────────────────
//
//   program        = funcDecl*
//   funcDecl       = "forge" IDENT "(" params ")" type? block
//   block          = "{" statement* "}"
//   statement      = varDecl | constDecl | assign | ifStmt | whileStmt
//                  | forStmt | matchStmt | returnStmt | printStmt
//                  | breakStmt | continueStmt | tryCatch | throwStmt | exprStmt
//
//   varDecl        = "lowkey" IDENT type "=" expr
//   constDecl      = ("noCap"|"lockIn") IDENT type "=" expr
//   assign         = IDENT "=" expr
//   ifStmt         = "perchance" expr block ("otherwise" (block|ifStmt))?
//   whileStmt      = "letHimCook" expr block
//   forStmt        = "spinTheBlock" "(" (varDecl|assign) ";" expr ";" assign ")" block
//   matchStmt      = "checkTheFit" "(" expr ")" "{" case* "}"
//   case           = "style" expr ":" block | "noFilter" ":" block
//   returnStmt     = "itIsWhatItIs" expr?
//   printStmt      = "spill" "(" expr ")"
//   breakStmt      = "ghost"
//   continueStmt   = "keepItMoving"
//   throwStmt      = "throwHands" "(" expr ")"
//   tryCatch       = "attempt" block "catch" IDENT block
//
//   expr precedence (low → high):
//     || → && → == != isGiving → < > <= >= → + - → * / % → unary → primary
package parser

import (
	"fmt"
	"strconv"
	"vine/ast"
	"vine/lexer"
)

// Parser holds all parser state.
type Parser struct {
	tokens []lexer.Token
	pos    int
	errors []string
}

// New creates a Parser from a token stream.
func New(tokens []lexer.Token) *Parser {
	return &Parser{tokens: tokens}
}

// Parse runs the parser and returns the root AST node plus any errors.
func (p *Parser) Parse() (*ast.Program, []string) {
	program := &ast.Program{}
	for !p.isAtEnd() {
		fn := p.parseFuncDecl()
		if fn != nil {
			program.Functions = append(program.Functions, fn)
		}
	}
	return program, p.errors
}

// ─── Top-Level ────────────────────────────────────────────────────────────────

// parseFuncDecl: forge <n>(<params>) <returnType?> { <body> }
func (p *Parser) parseFuncDecl() *ast.FuncDecl {
	forgeTok := p.expect(lexer.TOKEN_FUNC)
	nameTok  := p.expect(lexer.TOKEN_IDENT)

	p.expect(lexer.TOKEN_LPAREN)
	params := p.parseParams()
	p.expect(lexer.TOKEN_RPAREN)

	var returnType *ast.TypeNode
	if p.isTypeKeyword(p.current()) {
		returnType = p.parseTypeNode()
	} else {
		returnType = &ast.TypeNode{Kind: ast.TypeVoid}
	}

	body := p.parseBlock()

	return &ast.FuncDecl{
		Token:      forgeTok,
		Name:       nameTok,
		Params:     params,
		ReturnType: returnType,
		Body:       body,
	}
}

// parseParams parses a comma-separated list of  name type  pairs.
func (p *Parser) parseParams() []*ast.Param {
	var params []*ast.Param
	if p.current().Type == lexer.TOKEN_RPAREN {
		return params
	}
	for {
		nameTok := p.expect(lexer.TOKEN_IDENT)
		typNode  := p.parseTypeNode()
		params    = append(params, &ast.Param{Name: nameTok, Type: typNode})
		if !p.match(lexer.TOKEN_COMMA) { break }
	}
	return params
}

// ─── Statements ───────────────────────────────────────────────────────────────

// parseBlock: { statement* }
func (p *Parser) parseBlock() *ast.BlockStmt {
	lbrace := p.expect(lexer.TOKEN_LBRACE)
	block  := &ast.BlockStmt{Token: lbrace}
	for !p.isAtEnd() && p.current().Type != lexer.TOKEN_RBRACE {
		stmt := p.parseStatement()
		if stmt != nil {
			block.Statements = append(block.Statements, stmt)
		}
	}
	p.expect(lexer.TOKEN_RBRACE)
	return block
}

func (p *Parser) parseStatement() ast.Statement {
	switch p.current().Type {

	case lexer.TOKEN_VAR:
		return p.parseVarDecl(true) // lowkey → mutable

	case lexer.TOKEN_CONST:
		return p.parseVarDecl(false) // noCap/lockIn → immutable

	case lexer.TOKEN_IF:
		return p.parseIfStmt()

	case lexer.TOKEN_WHILE:
		return p.parseWhileStmt()

	case lexer.TOKEN_FOR:
		return p.parseForStmt()

	case lexer.TOKEN_RETURN:
		return p.parseReturnStmt()

	case lexer.TOKEN_PRINT:
		return p.parsePrintStmt()

	case lexer.TOKEN_BREAK:
		tok := p.advance()
		return &ast.BreakStmt{Token: tok}

	case lexer.TOKEN_CONTINUE:
		tok := p.advance()
		return &ast.ContinueStmt{Token: tok}

	case lexer.TOKEN_THROW:
		return p.parseThrowStmt()

	case lexer.TOKEN_TRY:
		return p.parseTryCatch()

	case lexer.TOKEN_MATCH:
		return p.parseMatchStmt()

	case lexer.TOKEN_IDENT:
		// assignment  x = expr   vs  expression statement  f(args)
		if p.peek().Type == lexer.TOKEN_ASSIGN {
			return p.parseAssignStmt()
		}
		return p.parseExprStmt()

	default:
		p.addError(p.current(), fmt.Sprintf("unexpected token %s at start of statement", p.current().Type))
		p.advance()
		return nil
	}
}

// lowkey <n> <type> = <expr>   or   noCap <n> <type> = <expr>
func (p *Parser) parseVarDecl(mutable bool) *ast.VarDeclStmt {
	varTok  := p.advance()
	nameTok := p.expect(lexer.TOKEN_IDENT)
	typNode := p.parseTypeNode()
	p.expect(lexer.TOKEN_ASSIGN)
	value := p.parseExpression()
	return &ast.VarDeclStmt{Token: varTok, Name: nameTok, Type: typNode, Value: value, Mutable: mutable}
}

// <n> = <expr>
func (p *Parser) parseAssignStmt() *ast.AssignStmt {
	nameTok   := p.advance()
	assignTok := p.expect(lexer.TOKEN_ASSIGN)
	value     := p.parseExpression()
	return &ast.AssignStmt{Token: assignTok, Name: nameTok, Value: value}
}

// perchance <cond> { } (otherwise { })?
func (p *Parser) parseIfStmt() *ast.IfStmt {
	ifTok       := p.advance()
	cond        := p.parseExpression()
	consequence := p.parseBlock()

	var alternative ast.Statement
	if p.match(lexer.TOKEN_ELSE) {
		if p.current().Type == lexer.TOKEN_IF {
			alternative = p.parseIfStmt()  // else-if chain
		} else {
			alternative = p.parseBlock()   // plain else block
		}
	}

	return &ast.IfStmt{Token: ifTok, Condition: cond, Consequence: consequence, Alternative: alternative}
}

// letHimCook <cond> { }
func (p *Parser) parseWhileStmt() *ast.WhileStmt {
	whileTok := p.advance()
	cond     := p.parseExpression()
	body     := p.parseBlock()
	return &ast.WhileStmt{Token: whileTok, Condition: cond, Body: body}
}

// spinTheBlock (init; cond; post) { }
// Example: spinTheBlock (lowkey i int = 0; i < 10; i = i + 1) { }
func (p *Parser) parseForStmt() *ast.ForStmt {
	forTok := p.advance() // consume 'spinTheBlock'
	p.expect(lexer.TOKEN_LPAREN)

	// init: lowkey i int = 0  (or  i = 0  for an existing variable)
	var init ast.Statement
	if p.current().Type == lexer.TOKEN_VAR {
		init = p.parseVarDecl(true)
	} else if p.current().Type == lexer.TOKEN_CONST {
		init = p.parseVarDecl(false)
	} else {
		init = p.parseAssignStmt()
	}
	p.expect(lexer.TOKEN_SEMICOLON)

	// condition
	cond := p.parseExpression()
	p.expect(lexer.TOKEN_SEMICOLON)

	// post: i = i + 1
	post := p.parseAssignStmt()
	p.expect(lexer.TOKEN_RPAREN)

	body := p.parseBlock()
	return &ast.ForStmt{Token: forTok, Init: init, Cond: cond, Post: post, Body: body}
}

// itIsWhatItIs <expr>?
func (p *Parser) parseReturnStmt() *ast.ReturnStmt {
	retTok := p.advance()
	var value ast.Expression
	if p.current().Type != lexer.TOKEN_RBRACE && !p.isAtEnd() {
		value = p.parseExpression()
	}
	return &ast.ReturnStmt{Token: retTok, Value: value}
}

// spill(<expr>)
func (p *Parser) parsePrintStmt() *ast.PrintStmt {
	printTok := p.advance()
	p.expect(lexer.TOKEN_LPAREN)
	value := p.parseExpression()
	p.expect(lexer.TOKEN_RPAREN)
	return &ast.PrintStmt{Token: printTok, Value: value}
}

// throwHands(<expr>)
func (p *Parser) parseThrowStmt() *ast.ThrowStmt {
	throwTok := p.advance()
	p.expect(lexer.TOKEN_LPAREN)
	msg := p.parseExpression()
	p.expect(lexer.TOKEN_RPAREN)
	return &ast.ThrowStmt{Token: throwTok, Message: msg}
}

// attempt { } catch <varName> { }
func (p *Parser) parseTryCatch() *ast.TryCatchStmt {
	tryTok   := p.advance()
	tryBlock := p.parseBlock()
	p.expect(lexer.TOKEN_CATCH)
	catchVar := p.expect(lexer.TOKEN_IDENT)
	catchBlock := p.parseBlock()
	return &ast.TryCatchStmt{Token: tryTok, TryBlock: tryBlock, CatchVar: catchVar, CatchBlock: catchBlock}
}

// checkTheFit (<expr>) { style <val>: { } ... noFilter: { } }
func (p *Parser) parseMatchStmt() *ast.MatchStmt {
	matchTok := p.advance()
	p.expect(lexer.TOKEN_LPAREN)
	subject := p.parseExpression()
	p.expect(lexer.TOKEN_RPAREN)
	p.expect(lexer.TOKEN_LBRACE)

	var cases []*ast.MatchCase
	for !p.isAtEnd() && p.current().Type != lexer.TOKEN_RBRACE {
		if p.current().Type == lexer.TOKEN_CASE {
			p.advance() // consume 'style'
			val  := p.parseExpression()
			p.expect(lexer.TOKEN_COLON)
			body := p.parseBlock()
			cases = append(cases, &ast.MatchCase{Value: val, Body: body})
		} else if p.current().Type == lexer.TOKEN_DEFAULT {
			p.advance() // consume 'noFilter'
			p.expect(lexer.TOKEN_COLON)
			body := p.parseBlock()
			cases = append(cases, &ast.MatchCase{Value: nil, Body: body})
		} else {
			p.addError(p.current(), "expected 'style' or 'noFilter' inside checkTheFit")
			p.advance()
		}
	}
	p.expect(lexer.TOKEN_RBRACE)
	return &ast.MatchStmt{Token: matchTok, Subject: subject, Cases: cases}
}

func (p *Parser) parseExprStmt() *ast.ExprStmt {
	return &ast.ExprStmt{Expr: p.parseExpression()}
}

// ─── Expressions (Recursive Descent, lowest → highest precedence) ────────────

func (p *Parser) parseExpression() ast.Expression  { return p.parseLogicOr() }

func (p *Parser) parseLogicOr() ast.Expression {
	left := p.parseLogicAnd()
	for p.current().Type == lexer.TOKEN_OR {
		op    := p.advance()
		right := p.parseLogicAnd()
		left   = &ast.BinaryExpr{Left: left, Operator: op, Right: right}
	}
	return left
}

func (p *Parser) parseLogicAnd() ast.Expression {
	left := p.parseEquality()
	for p.current().Type == lexer.TOKEN_AND {
		op    := p.advance()
		right := p.parseEquality()
		left   = &ast.BinaryExpr{Left: left, Operator: op, Right: right}
	}
	return left
}

// parseEquality handles == / isGiving / !=
func (p *Parser) parseEquality() ast.Expression {
	left := p.parseComparison()
	for p.current().Type == lexer.TOKEN_EQ || p.current().Type == lexer.TOKEN_NEQ {
		op    := p.advance()
		right := p.parseComparison()
		left   = &ast.BinaryExpr{Left: left, Operator: op, Right: right}
	}
	return left
}

func (p *Parser) parseComparison() ast.Expression {
	left := p.parseAddition()
	for p.current().Type == lexer.TOKEN_LT  || p.current().Type == lexer.TOKEN_GT ||
		p.current().Type == lexer.TOKEN_LEQ || p.current().Type == lexer.TOKEN_GEQ {
		op    := p.advance()
		right := p.parseAddition()
		left   = &ast.BinaryExpr{Left: left, Operator: op, Right: right}
	}
	return left
}

func (p *Parser) parseAddition() ast.Expression {
	left := p.parseMultiply()
	for p.current().Type == lexer.TOKEN_PLUS || p.current().Type == lexer.TOKEN_MINUS {
		op    := p.advance()
		right := p.parseMultiply()
		left   = &ast.BinaryExpr{Left: left, Operator: op, Right: right}
	}
	return left
}

func (p *Parser) parseMultiply() ast.Expression {
	left := p.parseUnary()
	for p.current().Type == lexer.TOKEN_STAR   ||
		p.current().Type == lexer.TOKEN_SLASH  ||
		p.current().Type == lexer.TOKEN_PERCENT {
		op    := p.advance()
		right := p.parseUnary()
		left   = &ast.BinaryExpr{Left: left, Operator: op, Right: right}
	}
	return left
}

func (p *Parser) parseUnary() ast.Expression {
	if p.current().Type == lexer.TOKEN_MINUS || p.current().Type == lexer.TOKEN_NOT {
		op      := p.advance()
		operand := p.parseUnary()
		return &ast.UnaryExpr{Operator: op, Operand: operand}
	}
	return p.parsePrimary()
}

func (p *Parser) parsePrimary() ast.Expression {
	tok := p.current()
	switch tok.Type {

	case lexer.TOKEN_INT_LIT:
		p.advance()
		val, _ := strconv.ParseInt(tok.Literal, 10, 64)
		return &ast.IntLit{Token: tok, Value: val}

	case lexer.TOKEN_FLOAT_LIT:
		p.advance()
		val, _ := strconv.ParseFloat(tok.Literal, 64)
		return &ast.FloatLit{Token: tok, Value: val}

	case lexer.TOKEN_STRING_LIT:
		p.advance()
		return &ast.StringLit{Token: tok, Value: tok.Literal}

	case lexer.TOKEN_TRUE:
		p.advance()
		return &ast.BoolLit{Token: tok, Value: true}

	case lexer.TOKEN_FALSE:
		p.advance()
		return &ast.BoolLit{Token: tok, Value: false}

	case lexer.TOKEN_NULL:
		p.advance()
		return &ast.NullLit{Token: tok}

	case lexer.TOKEN_IDENT:
		p.advance()
		if p.current().Type == lexer.TOKEN_LPAREN {
			return p.parseCallExpr(tok)
		}
		return &ast.Identifier{Token: tok, Name: tok.Literal}

	case lexer.TOKEN_LPAREN:
		p.advance()
		expr := p.parseExpression()
		p.expect(lexer.TOKEN_RPAREN)
		return expr

	default:
		p.addError(tok, fmt.Sprintf("expected expression, got %s", tok.Type))
		p.advance()
		return &ast.IntLit{Token: tok, Value: 0}
	}
}

// <name>(<arg1>, <arg2>, ...)
func (p *Parser) parseCallExpr(nameTok lexer.Token) *ast.CallExpr {
	lparen := p.advance()
	args   := p.parseArgs()
	p.expect(lexer.TOKEN_RPAREN)
	return &ast.CallExpr{Token: lparen, Function: nameTok, Args: args}
}

func (p *Parser) parseArgs() []ast.Expression {
	var args []ast.Expression
	if p.current().Type == lexer.TOKEN_RPAREN { return args }
	for {
		args = append(args, p.parseExpression())
		if !p.match(lexer.TOKEN_COMMA) { break }
	}
	return args
}

// ─── Type Parsing ─────────────────────────────────────────────────────────────

func (p *Parser) parseTypeNode() *ast.TypeNode {
	tok := p.current()
	switch tok.Type {
	case lexer.TOKEN_INT_TYPE:    p.advance(); return &ast.TypeNode{Token: tok, Kind: ast.TypeInt}
	case lexer.TOKEN_FLOAT_TYPE:  p.advance(); return &ast.TypeNode{Token: tok, Kind: ast.TypeFloat}
	case lexer.TOKEN_STRING_TYPE: p.advance(); return &ast.TypeNode{Token: tok, Kind: ast.TypeString}
	case lexer.TOKEN_BOOL_TYPE:   p.advance(); return &ast.TypeNode{Token: tok, Kind: ast.TypeBool}
	case lexer.TOKEN_VOID_TYPE:   p.advance(); return &ast.TypeNode{Token: tok, Kind: ast.TypeVoid}
	default:
		p.addError(tok, fmt.Sprintf("expected type, got %s", tok.Type))
		return &ast.TypeNode{Kind: ast.TypeUnknown}
	}
}

func (p *Parser) isTypeKeyword(tok lexer.Token) bool {
	switch tok.Type {
	case lexer.TOKEN_INT_TYPE, lexer.TOKEN_FLOAT_TYPE,
		lexer.TOKEN_STRING_TYPE, lexer.TOKEN_BOOL_TYPE, lexer.TOKEN_VOID_TYPE:
		return true
	}
	return false
}

// ─── Token Navigation ─────────────────────────────────────────────────────────

func (p *Parser) current() lexer.Token {
	if p.pos < len(p.tokens) { return p.tokens[p.pos] }
	return lexer.Token{Type: lexer.TOKEN_EOF}
}
func (p *Parser) peek() lexer.Token {
	if p.pos+1 < len(p.tokens) { return p.tokens[p.pos+1] }
	return lexer.Token{Type: lexer.TOKEN_EOF}
}
func (p *Parser) advance() lexer.Token {
	tok := p.current()
	if !p.isAtEnd() { p.pos++ }
	return tok
}
func (p *Parser) match(t lexer.TokenType) bool {
	if p.current().Type == t { p.advance(); return true }
	return false
}
func (p *Parser) expect(t lexer.TokenType) lexer.Token {
	tok := p.current()
	if tok.Type != t {
		p.addError(tok, fmt.Sprintf("expected %s, got %s (%q)", t, tok.Type, tok.Literal))
	} else {
		p.advance()
	}
	return tok
}
func (p *Parser) isAtEnd() bool { return p.current().Type == lexer.TOKEN_EOF }
func (p *Parser) addError(tok lexer.Token, msg string) {
	p.errors = append(p.errors, fmt.Sprintf("parse error at line %d, col %d: %s", tok.Line, tok.Column, msg))
}
