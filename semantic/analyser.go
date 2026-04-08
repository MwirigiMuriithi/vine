// Package semantic implements SEMANTIC ANALYSIS for the Vine compiler.
//
// ─── What is Semantic Analysis? ───────────────────────────────────────────────
// The semantic analyser walks the AST and checks that the program is MEANINGFUL.
// Syntax errors (caught by the parser) tell you the grammar is wrong.
// Semantic errors tell you the grammar is right but the meaning is wrong.
//
// Examples of semantic errors:
//   lowkey x int = "oops"        type mismatch
//   spill(undeclaredVar)          variable not declared
//   noCap y int = 5; y = 10      mutation of immutable (noCap/lockIn)
//   ghost outside a loop         break used outside letHimCook/spinTheBlock
//
// ─── Symbol Table ─────────────────────────────────────────────────────────────
// We use a SCOPE STACK: a list of scopes, innermost on top.
// When entering a block, we push a scope. When leaving, we pop it.
// Lookups search from innermost outward (lexical scoping).
package semantic

import (
	"fmt"
	"vine/ast"
	"vine/lexer"
)

// VarInfo stores information about a declared variable.
type VarInfo struct {
	Type    ast.TypeKind
	Name    string
	Mutable bool // false = noCap/lockIn (immutable)
}

// FuncInfo stores a function's signature.
type FuncInfo struct {
	Name       string
	ParamTypes []ast.TypeKind
	ReturnType ast.TypeKind
}

// Scope is one level of the symbol table.
type Scope map[string]VarInfo

// SymbolTable manages the scope stack.
type SymbolTable struct{ scopes []Scope }

func (st *SymbolTable) pushScope() {
	st.scopes = append(st.scopes, make(Scope))
}
func (st *SymbolTable) popScope() {
	if len(st.scopes) > 0 { st.scopes = st.scopes[:len(st.scopes)-1] }
}
func (st *SymbolTable) define(name string, kind ast.TypeKind, mutable bool) bool {
	if len(st.scopes) == 0 { return false }
	top := st.scopes[len(st.scopes)-1]
	if _, exists := top[name]; exists { return false }
	top[name] = VarInfo{Type: kind, Name: name, Mutable: mutable}
	return true
}
func (st *SymbolTable) lookup(name string) (VarInfo, bool) {
	for i := len(st.scopes) - 1; i >= 0; i-- {
		if info, ok := st.scopes[i][name]; ok { return info, true }
	}
	return VarInfo{}, false
}

// Analyser walks the AST and performs all semantic checks.
type Analyser struct {
	symbols     SymbolTable
	functions   map[string]FuncInfo
	currentFunc *FuncInfo
	loopDepth   int // tracks how deep we are inside loops (for ghost/keepItMoving check)
	errors      []string
}

// New creates a fresh Analyser.
func New() *Analyser {
	return &Analyser{functions: make(map[string]FuncInfo)}
}

// Analyse runs full semantic analysis, returning any errors.
func (a *Analyser) Analyse(program *ast.Program) []string {
	// Pass 1: register all forge (function) signatures
	// This lets functions call each other regardless of declaration order.
	for _, fn := range program.Functions {
		a.registerFunction(fn)
	}
	// Pass 2: analyse each function body
	for _, fn := range program.Functions {
		a.analyseFunc(fn)
	}
	return a.errors
}

func (a *Analyser) registerFunction(fn *ast.FuncDecl) {
	if _, exists := a.functions[fn.Name.Literal]; exists {
		a.errorf("forge %q declared more than once", fn.Name.Literal)
		return
	}
	info := FuncInfo{Name: fn.Name.Literal, ReturnType: fn.ReturnType.Kind}
	for _, p := range fn.Params {
		info.ParamTypes = append(info.ParamTypes, p.Type.Kind)
	}
	a.functions[fn.Name.Literal] = info
}

func (a *Analyser) analyseFunc(fn *ast.FuncDecl) {
	info := a.functions[fn.Name.Literal]
	a.currentFunc = &info
	a.symbols.pushScope()
	defer func() { a.symbols.popScope(); a.currentFunc = nil }()
	for _, p := range fn.Params {
		if !a.symbols.define(p.Name.Literal, p.Type.Kind, true) {
			a.errorf("duplicate parameter %q in forge %q", p.Name.Literal, fn.Name.Literal)
		}
	}
	a.analyseBlock(fn.Body)
}

func (a *Analyser) analyseBlock(block *ast.BlockStmt) {
	for _, stmt := range block.Statements {
		a.analyseStatement(stmt)
	}
}

func (a *Analyser) analyseStatement(stmt ast.Statement) {
	switch s := stmt.(type) {
	case *ast.VarDeclStmt:
		a.analyseVarDecl(s)
	case *ast.AssignStmt:
		a.analyseAssign(s)
	case *ast.IfStmt:
		a.analyseIf(s)
	case *ast.WhileStmt:
		a.analyseWhile(s)
	case *ast.ForStmt:
		a.analyseFor(s)
	case *ast.ReturnStmt:
		a.analyseReturn(s)
	case *ast.PrintStmt:
		a.analyseExpr(s.Value)
	case *ast.ThrowStmt:
		a.analyseExpr(s.Message)
	case *ast.TryCatchStmt:
		a.analyseTryCatch(s)
	case *ast.MatchStmt:
		a.analyseMatch(s)
	case *ast.ExprStmt:
		a.analyseExpr(s.Expr)
	case *ast.BlockStmt:
		// ── Bug fix: plain else blocks arrive here as *ast.BlockStmt ─────────
		// When the parser stores a plain 'otherwise { }' branch, it stores it
		// directly as a *ast.BlockStmt. We push a new scope and analyse it.
		a.symbols.pushScope()
		a.analyseBlock(s)
		a.symbols.popScope()
	case *ast.BreakStmt:
		if a.loopDepth == 0 {
			a.errorf("line %d: 'ghost' (break) used outside of a loop", s.Token.Line)
		}
	case *ast.ContinueStmt:
		if a.loopDepth == 0 {
			a.errorf("line %d: 'keepItMoving' (continue) used outside of a loop", s.Token.Line)
		}
	}
}

func (a *Analyser) analyseVarDecl(stmt *ast.VarDeclStmt) {
	valueType := a.analyseExpr(stmt.Value)
	declared  := stmt.Type.Kind
	if valueType != ast.TypeUnknown && declared != ast.TypeUnknown {
		if !typesCompatible(declared, valueType) {
			a.errorf("line %d: type mismatch: %s %q declared as %s but got %s",
				stmt.Token.Line, stmt.Token.Literal, stmt.Name.Literal, declared, valueType)
		}
	}
	if !a.symbols.define(stmt.Name.Literal, declared, stmt.Mutable) {
		a.errorf("line %d: %q already declared in this scope", stmt.Token.Line, stmt.Name.Literal)
	}
}

func (a *Analyser) analyseAssign(stmt *ast.AssignStmt) {
	info, ok := a.symbols.lookup(stmt.Name.Literal)
	if !ok {
		a.errorf("line %d: assignment to undeclared variable %q", stmt.Token.Line, stmt.Name.Literal)
		a.analyseExpr(stmt.Value)
		return
	}
	// Immutability check: noCap/lockIn variables cannot be reassigned
	if !info.Mutable {
		a.errorf("line %d: cannot reassign noCap/lockIn variable %q — it's locked in fr", stmt.Token.Line, stmt.Name.Literal)
	}
	vt := a.analyseExpr(stmt.Value)
	if vt != ast.TypeUnknown && info.Type != ast.TypeUnknown && !typesCompatible(info.Type, vt) {
		a.errorf("line %d: cannot assign %s to %q (type %s)", stmt.Token.Line, vt, stmt.Name.Literal, info.Type)
	}
}

func (a *Analyser) analyseIf(stmt *ast.IfStmt) {
	ct := a.analyseExpr(stmt.Condition)
	if ct != ast.TypeBool && ct != ast.TypeUnknown {
		a.errorf("line %d: perchance condition must be bool, got %s", stmt.Token.Line, ct)
	}
	a.symbols.pushScope()
	a.analyseBlock(stmt.Consequence)
	a.symbols.popScope()
	if stmt.Alternative != nil {
		a.symbols.pushScope()
		a.analyseStatement(stmt.Alternative)
		a.symbols.popScope()
	}
}

func (a *Analyser) analyseWhile(stmt *ast.WhileStmt) {
	ct := a.analyseExpr(stmt.Condition)
	if ct != ast.TypeBool && ct != ast.TypeUnknown {
		a.errorf("line %d: letHimCook condition must be bool, got %s", stmt.Token.Line, ct)
	}
	a.symbols.pushScope()
	a.loopDepth++
	a.analyseBlock(stmt.Body)
	a.loopDepth--
	a.symbols.popScope()
}

func (a *Analyser) analyseFor(stmt *ast.ForStmt) {
	a.symbols.pushScope()
	a.analyseStatement(stmt.Init)
	ct := a.analyseExpr(stmt.Cond)
	if ct != ast.TypeBool && ct != ast.TypeUnknown {
		a.errorf("line %d: spinTheBlock condition must be bool, got %s", stmt.Token.Line, ct)
	}
	a.analyseStatement(stmt.Post)
	a.loopDepth++
	a.analyseBlock(stmt.Body)
	a.loopDepth--
	a.symbols.popScope()
}

func (a *Analyser) analyseReturn(stmt *ast.ReturnStmt) {
	if a.currentFunc == nil {
		a.errorf("line %d: itIsWhatItIs outside of a forge", stmt.Token.Line)
		return
	}
	expected := a.currentFunc.ReturnType
	if stmt.Value == nil {
		if expected != ast.TypeVoid {
			a.errorf("line %d: forge %q must return %s", stmt.Token.Line, a.currentFunc.Name, expected)
		}
		return
	}
	actual := a.analyseExpr(stmt.Value)
	if actual != ast.TypeUnknown && expected != ast.TypeUnknown && !typesCompatible(expected, actual) {
		a.errorf("line %d: forge %q returns %s but expression is %s",
			stmt.Token.Line, a.currentFunc.Name, expected, actual)
	}
}

func (a *Analyser) analyseTryCatch(stmt *ast.TryCatchStmt) {
	a.symbols.pushScope()
	a.analyseBlock(stmt.TryBlock)
	a.symbols.popScope()
	a.symbols.pushScope()
	// The caught error is a string variable in the catch block
	a.symbols.define(stmt.CatchVar.Literal, ast.TypeString, true)
	a.analyseBlock(stmt.CatchBlock)
	a.symbols.popScope()
}

func (a *Analyser) analyseMatch(stmt *ast.MatchStmt) {
	subjectType := a.analyseExpr(stmt.Subject)
	for _, c := range stmt.Cases {
		if c.Value != nil {
			caseType := a.analyseExpr(c.Value)
			if !typesCompatible(subjectType, caseType) && !typesCompatible(caseType, subjectType) {
				a.errorf("checkTheFit case type %s doesn't match subject type %s", caseType, subjectType)
			}
		}
		a.symbols.pushScope()
		a.analyseBlock(c.Body)
		a.symbols.popScope()
	}
}

// analyseExpr returns the type of an expression (TypeUnknown on error).
func (a *Analyser) analyseExpr(expr ast.Expression) ast.TypeKind {
	switch e := expr.(type) {
	case *ast.IntLit:    return ast.TypeInt
	case *ast.FloatLit:  return ast.TypeFloat
	case *ast.StringLit: return ast.TypeString
	case *ast.BoolLit:   return ast.TypeBool
	case *ast.NullLit:   return ast.TypeUnknown // ghosted is typeless
	case *ast.Identifier:
		info, ok := a.symbols.lookup(e.Name)
		if !ok {
			a.errorf("line %d: undeclared variable %q — it's ghosted fr", e.Token.Line, e.Name)
			return ast.TypeUnknown
		}
		return info.Type
	case *ast.UnaryExpr:  return a.analyseUnary(e)
	case *ast.BinaryExpr: return a.analyseBinary(e)
	case *ast.CallExpr:   return a.analyseCall(e)
	}
	return ast.TypeUnknown
}

func (a *Analyser) analyseUnary(e *ast.UnaryExpr) ast.TypeKind {
	ot := a.analyseExpr(e.Operand)
	switch e.Operator.Type {
	case lexer.TOKEN_MINUS:
		if ot != ast.TypeInt && ot != ast.TypeFloat && ot != ast.TypeUnknown {
			a.errorf("line %d: unary '-' needs numeric type, got %s", e.Operator.Line, ot)
			return ast.TypeUnknown
		}
		return ot
	case lexer.TOKEN_NOT:
		if ot != ast.TypeBool && ot != ast.TypeUnknown {
			a.errorf("line %d: '!' needs bool, got %s", e.Operator.Line, ot)
			return ast.TypeUnknown
		}
		return ast.TypeBool
	}
	return ast.TypeUnknown
}

func (a *Analyser) analyseBinary(e *ast.BinaryExpr) ast.TypeKind {
	lt := a.analyseExpr(e.Left)
	rt := a.analyseExpr(e.Right)
	if lt == ast.TypeUnknown || rt == ast.TypeUnknown { return ast.TypeUnknown }

	switch e.Operator.Type {
	case lexer.TOKEN_PLUS:
		if lt == ast.TypeString && rt == ast.TypeString { return ast.TypeString }
		if isNumeric(lt) && isNumeric(rt) { return numericResult(lt, rt) }
		a.errorf("line %d: '+' needs two numbers or two strings, got %s + %s", e.Operator.Line, lt, rt)
		return ast.TypeUnknown
	case lexer.TOKEN_MINUS, lexer.TOKEN_STAR, lexer.TOKEN_SLASH, lexer.TOKEN_PERCENT:
		if !isNumeric(lt) || !isNumeric(rt) {
			a.errorf("line %d: '%s' needs numeric operands, got %s and %s", e.Operator.Line, e.Operator.Literal, lt, rt)
			return ast.TypeUnknown
		}
		return numericResult(lt, rt)
	case lexer.TOKEN_EQ, lexer.TOKEN_NEQ:
		if !typesCompatible(lt, rt) && !typesCompatible(rt, lt) {
			a.errorf("line %d: no cap, can't compare %s and %s", e.Operator.Line, lt, rt)
		}
		return ast.TypeBool
	case lexer.TOKEN_LT, lexer.TOKEN_GT, lexer.TOKEN_LEQ, lexer.TOKEN_GEQ:
		if !isNumeric(lt) || !isNumeric(rt) {
			a.errorf("line %d: '%s' needs numeric operands", e.Operator.Line, e.Operator.Literal)
			return ast.TypeUnknown
		}
		return ast.TypeBool
	case lexer.TOKEN_AND, lexer.TOKEN_OR:
		if lt != ast.TypeBool || rt != ast.TypeBool {
			a.errorf("line %d: '%s' needs bool operands, got %s and %s", e.Operator.Line, e.Operator.Literal, lt, rt)
			return ast.TypeUnknown
		}
		return ast.TypeBool
	}
	return ast.TypeUnknown
}

func (a *Analyser) analyseCall(e *ast.CallExpr) ast.TypeKind {
	fn, ok := a.functions[e.Function.Literal]
	if !ok {
		a.errorf("line %d: calling ghosted forge %q — it doesn't exist", e.Token.Line, e.Function.Literal)
		return ast.TypeUnknown
	}
	if len(e.Args) != len(fn.ParamTypes) {
		a.errorf("line %d: %q expects %d arg(s), got %d", e.Token.Line, fn.Name, len(fn.ParamTypes), len(e.Args))
		return fn.ReturnType
	}
	for i, arg := range e.Args {
		at  := a.analyseExpr(arg)
		exp := fn.ParamTypes[i]
		if at != ast.TypeUnknown && !typesCompatible(exp, at) {
			a.errorf("line %d: arg %d of %q: expected %s, got %s", e.Token.Line, i+1, fn.Name, exp, at)
		}
	}
	return fn.ReturnType
}

func typesCompatible(to, from ast.TypeKind) bool {
	if to == from { return true }
	if to == ast.TypeFloat && from == ast.TypeInt { return true }
	return false
}
func isNumeric(t ast.TypeKind) bool { return t == ast.TypeInt || t == ast.TypeFloat }
func numericResult(a, b ast.TypeKind) ast.TypeKind {
	if a == ast.TypeFloat || b == ast.TypeFloat { return ast.TypeFloat }
	return ast.TypeInt
}
func (a *Analyser) errorf(format string, args ...interface{}) {
	a.errors = append(a.errors, "semantic error: "+fmt.Sprintf(format, args...))
}
