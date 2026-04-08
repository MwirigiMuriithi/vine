// parser_test.go : Unit tests for the Vine parser.
//
// Strategy: lex a source snippet, parse the token stream, inspect the AST.
package parser

import (
	"testing"
	"vine/ast"
	"vine/lexer"
)

// parseSource is a test helper: lex then parse a source string.
func parseSource(t *testing.T, source string) (*ast.Program, []string) {
	t.Helper()
	tokens, _ := lexer.New(source).Tokenize()
	p := New(tokens)
	return p.Parse()
}

// ─── Function Declaration ─────────────────────────────────────────────────────

func TestParseFuncDecl(t *testing.T) {
	source := `forge add(a int, b int) int { itIsWhatItIs a + b }`
	prog, errs := parseSource(t, source)
	if len(errs) > 0 { t.Fatalf("unexpected parse errors: %v", errs) }
	if len(prog.Functions) != 1 { t.Fatalf("expected 1 function, got %d", len(prog.Functions)) }

	fn := prog.Functions[0]
	if fn.Name.Literal != "add" { t.Errorf("name: got %q, want %q", fn.Name.Literal, "add") }
	if len(fn.Params) != 2 { t.Fatalf("expected 2 params, got %d", len(fn.Params)) }
	if fn.Params[0].Name.Literal != "a" { t.Errorf("param[0] name: got %q", fn.Params[0].Name.Literal) }
	if fn.Params[0].Type.Kind != ast.TypeInt { t.Errorf("param[0] type: got %s", fn.Params[0].Type.Kind) }
	if fn.ReturnType.Kind != ast.TypeInt { t.Errorf("return type: got %s", fn.ReturnType.Kind) }
}

func TestParseVoidFunction(t *testing.T) {
	prog, errs := parseSource(t, `forge greet(name string) void { spill("hi") }`)
	if len(errs) > 0 { t.Fatalf("parse errors: %v", errs) }
	if prog.Functions[0].ReturnType.Kind != ast.TypeVoid {
		t.Errorf("return type: got %s, want void", prog.Functions[0].ReturnType.Kind)
	}
}

// ─── Variable Declarations ────────────────────────────────────────────────────

func TestParseLowkey(t *testing.T) {
	prog, errs := parseSource(t, `forge main() void { lowkey x int = 42 }`)
	if len(errs) > 0 { t.Fatalf("parse errors: %v", errs) }

	vd, ok := prog.Functions[0].Body.Statements[0].(*ast.VarDeclStmt)
	if !ok { t.Fatalf("expected VarDeclStmt, got %T", prog.Functions[0].Body.Statements[0]) }
	if vd.Name.Literal != "x"     { t.Errorf("name: got %q", vd.Name.Literal) }
	if vd.Type.Kind != ast.TypeInt { t.Errorf("type: got %s", vd.Type.Kind) }
	if !vd.Mutable                 { t.Error("lowkey should be mutable=true") }

	lit, ok := vd.Value.(*ast.IntLit)
	if !ok { t.Fatalf("value: expected IntLit, got %T", vd.Value) }
	if lit.Value != 42 { t.Errorf("value: got %d, want 42", lit.Value) }
}

func TestParseNoCap(t *testing.T) {
	for _, kw := range []string{"noCap", "lockIn"} {
		t.Run(kw, func(t *testing.T) {
			prog, errs := parseSource(t, `forge main() void { `+kw+` MAX int = 100 }`)
			if len(errs) > 0 { t.Fatalf("parse errors: %v", errs) }
			vd := prog.Functions[0].Body.Statements[0].(*ast.VarDeclStmt)
			if vd.Mutable { t.Errorf("%s should be mutable=false", kw) }
		})
	}
}

// ─── Expression Precedence ────────────────────────────────────────────────────

// 2 + 3 * 4  must parse as  2 + (3 * 4)  — * has higher precedence than +
func TestBinaryExprPrecedence(t *testing.T) {
	prog, errs := parseSource(t, `forge main() void { lowkey r int = 2 + 3 * 4 }`)
	if len(errs) > 0 { t.Fatalf("parse errors: %v", errs) }

	vd   := prog.Functions[0].Body.Statements[0].(*ast.VarDeclStmt)
	root := vd.Value.(*ast.BinaryExpr)
	if root.Operator.Literal != "+" { t.Errorf("root op: got %q, want '+'", root.Operator.Literal) }
	right := root.Right.(*ast.BinaryExpr)
	if right.Operator.Literal != "*" { t.Errorf("right op: got %q, want '*'", right.Operator.Literal) }
}

func TestIsGivingOperator(t *testing.T) {
	prog, errs := parseSource(t, `forge main() void { perchance (x isGiving 5) { spill("y") } }`)
	if len(errs) > 0 { t.Fatalf("parse errors: %v", errs) }
	ifStmt := prog.Functions[0].Body.Statements[0].(*ast.IfStmt)
	cond   := ifStmt.Condition.(*ast.BinaryExpr)
	if cond.Operator.Literal != "isGiving" {
		t.Errorf("operator: got %q, want 'isGiving'", cond.Operator.Literal)
	}
}

// ─── Control Flow ─────────────────────────────────────────────────────────────

func TestParseIfElse(t *testing.T) {
	prog, errs := parseSource(t, `forge main() void {
    perchance (bet) { spill("y") } otherwise { spill("n") }
}`)
	if len(errs) > 0 { t.Fatalf("parse errors: %v", errs) }
	ifStmt := prog.Functions[0].Body.Statements[0].(*ast.IfStmt)
	if ifStmt.Alternative == nil { t.Error("expected Alternative, got nil") }
}

func TestParseLetHimCook(t *testing.T) {
	prog, errs := parseSource(t, `forge main() void { letHimCook (i < 10) { i = i + 1 } }`)
	if len(errs) > 0 { t.Fatalf("parse errors: %v", errs) }
	if _, ok := prog.Functions[0].Body.Statements[0].(*ast.WhileStmt); !ok {
		t.Errorf("expected WhileStmt")
	}
}

func TestParseSpinTheBlock(t *testing.T) {
	prog, errs := parseSource(t, `forge main() void {
    spinTheBlock (lowkey i int = 0; i < 5; i = i + 1) { spill(i) }
}`)
	if len(errs) > 0 { t.Fatalf("parse errors: %v", errs) }
	fs, ok := prog.Functions[0].Body.Statements[0].(*ast.ForStmt)
	if !ok { t.Fatalf("expected ForStmt, got %T", prog.Functions[0].Body.Statements[0]) }
	if fs.Init == nil { t.Error("ForStmt.Init is nil") }
	if fs.Cond == nil { t.Error("ForStmt.Cond is nil") }
	if fs.Post == nil { t.Error("ForStmt.Post is nil") }
}

func TestParseGhost(t *testing.T) {
	prog, errs := parseSource(t, `forge main() void { letHimCook (bet) { ghost } }`)
	if len(errs) > 0 { t.Fatalf("parse errors: %v", errs) }
	ws := prog.Functions[0].Body.Statements[0].(*ast.WhileStmt)
	if _, ok := ws.Body.Statements[0].(*ast.BreakStmt); !ok {
		t.Errorf("expected BreakStmt in loop body")
	}
}

func TestParseCheckTheFit(t *testing.T) {
	prog, errs := parseSource(t, `forge main() void {
    checkTheFit (x) {
        style 1: { spill("one") }
        style 2: { spill("two") }
        noFilter: { spill("other") }
    }
}`)
	if len(errs) > 0 { t.Fatalf("parse errors: %v", errs) }
	ms, ok := prog.Functions[0].Body.Statements[0].(*ast.MatchStmt)
	if !ok { t.Fatalf("expected MatchStmt") }
	if len(ms.Cases) != 3 { t.Errorf("expected 3 cases, got %d", len(ms.Cases)) }
	if ms.Cases[2].Value != nil { t.Error("noFilter case should have nil Value") }
}

func TestParseAttemptCatch(t *testing.T) {
	prog, errs := parseSource(t, `forge main() void {
    attempt { spill("ok") } catch err { spill(err) }
}`)
	if len(errs) > 0 { t.Fatalf("parse errors: %v", errs) }
	tc, ok := prog.Functions[0].Body.Statements[0].(*ast.TryCatchStmt)
	if !ok { t.Fatalf("expected TryCatchStmt") }
	if tc.CatchVar.Literal != "err" { t.Errorf("catch var: got %q", tc.CatchVar.Literal) }
}

func TestParseThrowHands(t *testing.T) {
	prog, errs := parseSource(t, `forge main() void { throwHands("oops") }`)
	if len(errs) > 0 { t.Fatalf("parse errors: %v", errs) }
	if _, ok := prog.Functions[0].Body.Statements[0].(*ast.ThrowStmt); !ok {
		t.Errorf("expected ThrowStmt")
	}
}
