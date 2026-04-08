// Package ast defines the Abstract Syntax Tree (AST) for the Vine language.
//
// ─── What is an AST? ─────────────────────────────────────────────────────────
// After the Lexer produces a flat token stream, the Parser organises tokens
// into a TREE that reflects the grammatical structure of the program.
//
// Every node in the tree is either a STATEMENT (does something) or an
// EXPRESSION (produces a value).
//
// ─── Vine Keyword → AST Node mapping ────────────────────────────────────────
//   lowkey x int = 5          → VarDeclStmt
//   noCap / lockIn x int = 5  → ConstDeclStmt
//   forge f(a int) int { }    → FuncDecl
//   perchance / otherwise     → IfStmt
//   letHimCook (cond) { }     → WhileStmt
//   spinTheBlock (;;) { }     → ForStmt
//   ghost                     → BreakStmt
//   keepItMoving               → ContinueStmt
//   checkTheFit (x) { style } → MatchStmt
//   itIsWhatItIs expr          → ReturnStmt
//   spill(expr)               → PrintStmt
//   throwHands("msg")         → ThrowStmt
//   attempt { } catch e { }   → TryCatchStmt
package ast

import (
	"fmt"
	"strings"
	"vine/lexer"
)

// ─── Base Interfaces ──────────────────────────────────────────────────────────

// Node is the root interface for every AST node.
type Node interface {
	TokenLiteral() string
	String() string
}

// Statement does something but doesn't produce a value directly.
type Statement interface {
	Node
	statementNode()
}

// Expression computes and returns a value.
type Expression interface {
	Node
	expressionNode()
}

// ─── Program ──────────────────────────────────────────────────────────────────

// Program is the root of the AST — the entire Vine source file.
// At the top level, a Vine program is a sequence of function definitions.
type Program struct {
	Functions []*FuncDecl
}

func (p *Program) TokenLiteral() string {
	if len(p.Functions) > 0 { return p.Functions[0].TokenLiteral() }
	return ""
}
func (p *Program) String() string {
	var sb strings.Builder
	for _, fn := range p.Functions {
		sb.WriteString(fn.String())
		sb.WriteString("\n")
	}
	return sb.String()
}

// ─── Types ────────────────────────────────────────────────────────────────────

type TypeKind int

const (
	TypeInt     TypeKind = iota
	TypeFloat
	TypeString
	TypeBool
	TypeVoid
	TypeUnknown
)

func (t TypeKind) String() string {
	switch t {
	case TypeInt:    return "int"
	case TypeFloat:  return "float"
	case TypeString: return "string"
	case TypeBool:   return "bool"
	case TypeVoid:   return "void"
	default:         return "unknown"
	}
}

type TypeNode struct {
	Token lexer.Token
	Kind  TypeKind
}
func (tn *TypeNode) String() string { return tn.Kind.String() }

// ─── Top-Level: Function Declaration ─────────────────────────────────────────

// Param is one parameter in a forge (function) signature.
type Param struct {
	Name lexer.Token
	Type *TypeNode
}

// FuncDecl represents a forge definition:
//   forge add(a int, b int) int { itIsWhatItIs a + b }
type FuncDecl struct {
	Token      lexer.Token // the 'forge' token
	Name       lexer.Token
	Params     []*Param
	ReturnType *TypeNode
	Body       *BlockStmt
}
func (fd *FuncDecl) statementNode()       {}
func (fd *FuncDecl) TokenLiteral() string { return fd.Token.Literal }
func (fd *FuncDecl) String() string {
	params := []string{}
	for _, p := range fd.Params {
		params = append(params, p.Name.Literal+" "+p.Type.String())
	}
	ret := ""
	if fd.ReturnType != nil { ret = " " + fd.ReturnType.String() }
	return fmt.Sprintf("forge %s(%s)%s %s", fd.Name.Literal, strings.Join(params, ", "), ret, fd.Body.String())
}

// ─── Statements ───────────────────────────────────────────────────────────────

// BlockStmt is a { } block containing a list of statements.
type BlockStmt struct {
	Token      lexer.Token
	Statements []Statement
}
func (bs *BlockStmt) statementNode()       {}
func (bs *BlockStmt) TokenLiteral() string { return bs.Token.Literal }
func (bs *BlockStmt) String() string {
	var sb strings.Builder
	sb.WriteString("{\n")
	for _, s := range bs.Statements {
		sb.WriteString("  ")
		sb.WriteString(s.String())
		sb.WriteString("\n")
	}
	sb.WriteString("}")
	return sb.String()
}

// VarDeclStmt: lowkey <name> <type> = <expr>
type VarDeclStmt struct {
	Token   lexer.Token // 'lowkey' token
	Name    lexer.Token
	Type    *TypeNode
	Value   Expression
	Mutable bool // true for lowkey, false for noCap/lockIn
}
func (vd *VarDeclStmt) statementNode()       {}
func (vd *VarDeclStmt) TokenLiteral() string { return vd.Token.Literal }
func (vd *VarDeclStmt) String() string {
	kw := "lowkey"
	if !vd.Mutable { kw = "noCap" }
	return fmt.Sprintf("%s %s %s = %s", kw, vd.Name.Literal, vd.Type, vd.Value.String())
}

// AssignStmt: <name> = <expr>
type AssignStmt struct {
	Token lexer.Token
	Name  lexer.Token
	Value Expression
}
func (as *AssignStmt) statementNode()       {}
func (as *AssignStmt) TokenLiteral() string { return as.Token.Literal }
func (as *AssignStmt) String() string {
	return fmt.Sprintf("%s = %s", as.Name.Literal, as.Value.String())
}

// IfStmt: perchance <cond> { } otherwise { }
type IfStmt struct {
	Token       lexer.Token // 'perchance' token
	Condition   Expression
	Consequence *BlockStmt
	Alternative Statement  // nil, *BlockStmt (plain else), or *IfStmt (else-if)
}
func (is *IfStmt) statementNode()       {}
func (is *IfStmt) TokenLiteral() string { return is.Token.Literal }
func (is *IfStmt) String() string {
	s := fmt.Sprintf("perchance %s %s", is.Condition.String(), is.Consequence.String())
	if is.Alternative != nil { s += " otherwise " + is.Alternative.String() }
	return s
}

// WhileStmt: letHimCook <cond> { }
type WhileStmt struct {
	Token     lexer.Token // 'letHimCook' token
	Condition Expression
	Body      *BlockStmt
}
func (ws *WhileStmt) statementNode()       {}
func (ws *WhileStmt) TokenLiteral() string { return ws.Token.Literal }
func (ws *WhileStmt) String() string {
	return fmt.Sprintf("letHimCook %s %s", ws.Condition.String(), ws.Body.String())
}

// ForStmt: spinTheBlock (init; cond; post) { }
// Classic C-style for loop:
//   spinTheBlock (lowkey i int = 0; i < 10; i = i + 1) { }
type ForStmt struct {
	Token  lexer.Token  // 'spinTheBlock' token
	Init   Statement    // initialiser (VarDeclStmt or AssignStmt)
	Cond   Expression   // loop condition
	Post   Statement    // post-iteration update (AssignStmt)
	Body   *BlockStmt
}
func (fs *ForStmt) statementNode()       {}
func (fs *ForStmt) TokenLiteral() string { return fs.Token.Literal }
func (fs *ForStmt) String() string {
	return fmt.Sprintf("spinTheBlock (%s; %s; %s) %s",
		fs.Init.String(), fs.Cond.String(), fs.Post.String(), fs.Body.String())
}

// BreakStmt: ghost
type BreakStmt struct {
	Token lexer.Token
}
func (bs *BreakStmt) statementNode()       {}
func (bs *BreakStmt) TokenLiteral() string { return bs.Token.Literal }
func (bs *BreakStmt) String() string       { return "ghost" }

// ContinueStmt: keepItMoving
type ContinueStmt struct {
	Token lexer.Token
}
func (cs *ContinueStmt) statementNode()       {}
func (cs *ContinueStmt) TokenLiteral() string { return cs.Token.Literal }
func (cs *ContinueStmt) String() string       { return "keepItMoving" }

// ReturnStmt: itIsWhatItIs <expr>?
type ReturnStmt struct {
	Token lexer.Token // 'itIsWhatItIs' token
	Value Expression  // nil for void returns
}
func (rs *ReturnStmt) statementNode()       {}
func (rs *ReturnStmt) TokenLiteral() string { return rs.Token.Literal }
func (rs *ReturnStmt) String() string {
	if rs.Value == nil { return "itIsWhatItIs" }
	return "itIsWhatItIs " + rs.Value.String()
}

// PrintStmt: spill(<expr>)
type PrintStmt struct {
	Token lexer.Token
	Value Expression
}
func (ps *PrintStmt) statementNode()       {}
func (ps *PrintStmt) TokenLiteral() string { return ps.Token.Literal }
func (ps *PrintStmt) String() string       { return fmt.Sprintf("spill(%s)", ps.Value.String()) }

// ThrowStmt: throwHands(<expr>)
type ThrowStmt struct {
	Token   lexer.Token
	Message Expression
}
func (ts *ThrowStmt) statementNode()       {}
func (ts *ThrowStmt) TokenLiteral() string { return ts.Token.Literal }
func (ts *ThrowStmt) String() string {
	return fmt.Sprintf("throwHands(%s)", ts.Message.String())
}

// TryCatchStmt: attempt { } catch <name> { }
type TryCatchStmt struct {
	Token     lexer.Token // 'attempt' token
	TryBlock  *BlockStmt
	CatchVar  lexer.Token // the caught error variable name
	CatchBlock *BlockStmt
}
func (tc *TryCatchStmt) statementNode()       {}
func (tc *TryCatchStmt) TokenLiteral() string { return tc.Token.Literal }
func (tc *TryCatchStmt) String() string {
	return fmt.Sprintf("attempt %s catch %s %s",
		tc.TryBlock.String(), tc.CatchVar.Literal, tc.CatchBlock.String())
}

// MatchCase is one arm of a checkTheFit statement:
//   style <value>: { <body> }
type MatchCase struct {
	Value Expression  // nil for the 'noFilter' default case
	Body  *BlockStmt
}

// MatchStmt: checkTheFit (<expr>) { style val: { } ... noFilter: { } }
type MatchStmt struct {
	Token   lexer.Token // 'checkTheFit' token
	Subject Expression
	Cases   []*MatchCase
}
func (ms *MatchStmt) statementNode()       {}
func (ms *MatchStmt) TokenLiteral() string { return ms.Token.Literal }
func (ms *MatchStmt) String() string {
	return fmt.Sprintf("checkTheFit (%s) { ... }", ms.Subject.String())
}

// ExprStmt wraps an expression used as a statement (e.g. a function call).
type ExprStmt struct {
	Expr Expression
}
func (es *ExprStmt) statementNode()       {}
func (es *ExprStmt) TokenLiteral() string { return es.Expr.TokenLiteral() }
func (es *ExprStmt) String() string       { return es.Expr.String() }

// ─── Expressions ──────────────────────────────────────────────────────────────

type IntLit struct {
	Token lexer.Token
	Value int64
}
func (il *IntLit) expressionNode()      {}
func (il *IntLit) TokenLiteral() string { return il.Token.Literal }
func (il *IntLit) String() string       { return il.Token.Literal }

type FloatLit struct {
	Token lexer.Token
	Value float64
}
func (fl *FloatLit) expressionNode()      {}
func (fl *FloatLit) TokenLiteral() string { return fl.Token.Literal }
func (fl *FloatLit) String() string       { return fl.Token.Literal }

type StringLit struct {
	Token lexer.Token
	Value string
}
func (sl *StringLit) expressionNode()      {}
func (sl *StringLit) TokenLiteral() string { return sl.Token.Literal }
func (sl *StringLit) String() string       { return fmt.Sprintf("%q", sl.Value) }

// BoolLit: bet (true) or nah (false)
type BoolLit struct {
	Token lexer.Token
	Value bool
}
func (bl *BoolLit) expressionNode()      {}
func (bl *BoolLit) TokenLiteral() string { return bl.Token.Literal }
func (bl *BoolLit) String() string       { return bl.Token.Literal }

// NullLit: ghosted
type NullLit struct {
	Token lexer.Token
}
func (nl *NullLit) expressionNode()      {}
func (nl *NullLit) TokenLiteral() string { return nl.Token.Literal }
func (nl *NullLit) String() string       { return "ghosted" }

// Identifier is a reference to a variable or parameter.
type Identifier struct {
	Token lexer.Token
	Name  string
}
func (id *Identifier) expressionNode()      {}
func (id *Identifier) TokenLiteral() string { return id.Token.Literal }
func (id *Identifier) String() string       { return id.Name }

// BinaryExpr: left op right
// Operators include +, -, *, /, %, ==, isGiving, !=, <, >, <=, >=, &&, ||
type BinaryExpr struct {
	Left     Expression
	Operator lexer.Token
	Right    Expression
}
func (be *BinaryExpr) expressionNode()      {}
func (be *BinaryExpr) TokenLiteral() string { return be.Operator.Literal }
func (be *BinaryExpr) String() string {
	return fmt.Sprintf("(%s %s %s)", be.Left.String(), be.Operator.Literal, be.Right.String())
}

// UnaryExpr: op operand   (e.g. -x or !flag)
type UnaryExpr struct {
	Operator lexer.Token
	Operand  Expression
}
func (ue *UnaryExpr) expressionNode()      {}
func (ue *UnaryExpr) TokenLiteral() string { return ue.Operator.Literal }
func (ue *UnaryExpr) String() string {
	return fmt.Sprintf("(%s%s)", ue.Operator.Literal, ue.Operand.String())
}

// CallExpr: funcName(arg1, arg2, ...)
type CallExpr struct {
	Token    lexer.Token // '(' token
	Function lexer.Token // function name
	Args     []Expression
}
func (ce *CallExpr) expressionNode()      {}
func (ce *CallExpr) TokenLiteral() string { return ce.Token.Literal }
func (ce *CallExpr) String() string {
	args := []string{}
	for _, a := range ce.Args { args = append(args, a.String()) }
	return fmt.Sprintf("%s(%s)", ce.Function.Literal, strings.Join(args, ", "))
}
