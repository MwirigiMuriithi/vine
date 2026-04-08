// Package eval implements a TREE-WALK INTERPRETER for Vine.
//
// ─── What is a Tree-Walk Interpreter? ────────────────────────────────────────
// A tree-walk interpreter evaluates the AST DIRECTLY, without compiling it to
// bytecode first. It literally walks the tree and computes values as it goes.
//
// ─── Two approaches compared ─────────────────────────────────────────────────
//
//   Approach A (this file)  — Tree-Walk Interpreter
//     AST → Interpreter → output
//     ✓ Simpler to implement
//     ✓ Easier to understand
//     ✗ Slower (re-traverses the tree on every loop iteration)
//     Used by: early Ruby, early PHP
//
//   Approach B (vm/ package) — Bytecode Compiler + VM
//     AST → Bytecode → VM → output
//     ✓ Faster (bytecode is compact and cache-friendly)
//     ✓ Easier to optimise
//     ✗ More complex (two phases instead of one)
//     Used by: Python, Java, Lua, JavaScript engines
//
// Having BOTH implementations in this project lets you see the trade-offs
// first-hand — a great way to understand compiler design.
//
// ─── Control Flow via Go panics ───────────────────────────────────────────────
// The tree-walk interpreter uses Go's panic/recover mechanism to implement
// non-local control flow (return, break, continue, throw).
// When we see  itIsWhatItIs 42  inside a nested expression, we need to
// immediately unwind the call stack back to the function boundary.
// We do this by panicking with a special sentinel value, then recovering it
// at the function call site.
package eval

import (
	"fmt"
	"math"
	"os"
	"strings"

	"vine/ast"
)

// ─── Values ───────────────────────────────────────────────────────────────────

// Value represents any runtime value in the tree-walk interpreter.
// Unlike the VM's typed Value struct, we use Go's interface{} for simplicity.
type Value interface{}

// Type helpers
func isInt(v Value) bool    { _, ok := v.(int64);   return ok }
func isFloat(v Value) bool  { _, ok := v.(float64); return ok }
func isString(v Value) bool { _, ok := v.(string);  return ok }
func isBool(v Value) bool   { _, ok := v.(bool);    return ok }

func toFloat64(v Value) float64 {
	switch x := v.(type) {
	case int64:   return float64(x)
	case float64: return x
	}
	return 0
}

func valueString(v Value) string {
	if v == nil { return "ghosted" }
	switch x := v.(type) {
	case bool:    if x { return "true" } else { return "false" }
	case float64: return fmt.Sprintf("%g", x)
	default:      return fmt.Sprintf("%v", x)
	}
}

// ─── Control Flow Sentinels ───────────────────────────────────────────────────
// These special types are used with panic/recover to implement non-local jumps.

type returnSignal  struct{ value Value }    // itIsWhatItIs
type breakSignal   struct{}                  // ghost
type continueSignal struct{}                 // keepItMoving
type throwSignal   struct{ message string }  // throwHands

// ─── Environment (Scope) ─────────────────────────────────────────────────────

// Env is one scope level: a map of variable names to their values.
type Env struct {
	vars   map[string]Value
	parent *Env // enclosing scope (nil at top level)
}

// newEnv creates a child scope inside `parent`.
func newEnv(parent *Env) *Env {
	return &Env{vars: make(map[string]Value), parent: parent}
}

// get looks up a variable, searching from this scope outward.
func (e *Env) get(name string) (Value, bool) {
	if v, ok := e.vars[name]; ok { return v, true }
	if e.parent != nil { return e.parent.get(name) }
	return nil, false
}

// set sets a variable in the innermost scope where it already exists,
// or creates it in the current scope if not found.
func (e *Env) set(name string, val Value) {
	// Check if it already exists somewhere in scope chain
	if env := e.find(name); env != nil {
		env.vars[name] = val
		return
	}
	e.vars[name] = val
}

// define creates a new variable in the CURRENT scope.
func (e *Env) define(name string, val Value) { e.vars[name] = val }

// find returns the scope that contains `name`, or nil.
func (e *Env) find(name string) *Env {
	if _, ok := e.vars[name]; ok { return e }
	if e.parent != nil { return e.parent.find(name) }
	return nil
}

// ─── Interpreter ─────────────────────────────────────────────────────────────

// Interpreter is the tree-walk evaluator.
type Interpreter struct {
	globals   *Env               // top-level (global) environment
	functions map[string]*ast.FuncDecl // all declared forges
}

// New creates a fresh Interpreter.
func New() *Interpreter {
	return &Interpreter{
		globals:   newEnv(nil),
		functions: make(map[string]*ast.FuncDecl),
	}
}

// Interpret runs a complete Vine program.
func (interp *Interpreter) Interpret(program *ast.Program) error {
	// Register all functions first (two-pass, like the semantic analyser)
	for _, fn := range program.Functions {
		interp.functions[fn.Name.Literal] = fn
	}

	// Find and call main
	mainFn, ok := interp.functions["main"]
	if !ok {
		return fmt.Errorf("no 'main' forge defined")
	}

	env := newEnv(interp.globals)
	return interp.callFunc(mainFn, []Value{}, env)
}

// callFunc invokes a forge with the given argument values.
func (interp *Interpreter) callFunc(fn *ast.FuncDecl, args []Value, _ *Env) (err error) {
	env := newEnv(interp.globals)

	// Bind arguments to parameter names
	for i, param := range fn.Params {
		if i < len(args) {
			env.define(param.Name.Literal, args[i])
		}
	}

	// Use panic/recover to implement 'itIsWhatItIs' (return)
	defer func() {
		if r := recover(); r != nil {
			switch sig := r.(type) {
			case returnSignal:
				// Normal return — not an error
				_ = sig.value
			case throwSignal:
				// Unhandled throw propagates as an error
				err = fmt.Errorf("💀 throwHands: %s", sig.message)
			default:
				// Real panic — re-panic
				panic(r)
			}
		}
	}()

	interp.execBlock(fn.Body, env)
	return nil
}

// ─── Statement Execution ─────────────────────────────────────────────────────

func (interp *Interpreter) execBlock(block *ast.BlockStmt, env *Env) {
	for _, stmt := range block.Statements {
		interp.execStmt(stmt, env)
	}
}

func (interp *Interpreter) execStmt(stmt ast.Statement, env *Env) {
	switch s := stmt.(type) {

	case *ast.VarDeclStmt:
		val := interp.evalExpr(s.Value, env)
		env.define(s.Name.Literal, val)

	case *ast.AssignStmt:
		val := interp.evalExpr(s.Value, env)
		env.set(s.Name.Literal, val)

	case *ast.PrintStmt:
		val := interp.evalExpr(s.Value, env)
		fmt.Fprintln(os.Stdout, valueString(val))

	case *ast.ReturnStmt:
		var val Value
		if s.Value != nil { val = interp.evalExpr(s.Value, env) }
		panic(returnSignal{value: val})

	case *ast.IfStmt:
		cond := interp.evalExpr(s.Condition, env)
		if isTruthy(cond) {
			childEnv := newEnv(env)
			interp.execBlock(s.Consequence, childEnv)
		} else if s.Alternative != nil {
			childEnv := newEnv(env)
			interp.execStmt(s.Alternative, childEnv)
		}

	case *ast.BlockStmt:
		// Plain else block
		interp.execBlock(s, env)

	case *ast.WhileStmt:
		broken := false
		for !broken {
			cond := interp.evalExpr(s.Condition, env)
			if !isTruthy(cond) { break }
			func() {
				defer func() {
					if r := recover(); r != nil {
						switch r.(type) {
						case breakSignal:    broken = true   // ghost — exit loop
						case continueSignal:                 // keepItMoving — next iteration (do nothing, loop continues)
						default:            panic(r)         // propagate everything else
						}
					}
				}()
				childEnv := newEnv(env)
				interp.execBlock(s.Body, childEnv)
			}()
		}

	case *ast.ForStmt:
		forEnv := newEnv(env)
		interp.execStmt(s.Init, forEnv)
		broken := false
		for !broken {
			cond := interp.evalExpr(s.Cond, forEnv)
			if !isTruthy(cond) { break }
			continued := false
			func() {
				defer func() {
					if r := recover(); r != nil {
						switch r.(type) {
						case breakSignal:    broken    = true
						case continueSignal: continued = true
						default: panic(r)
						}
					}
				}()
				childEnv := newEnv(forEnv)
				interp.execBlock(s.Body, childEnv)
			}()
			if !broken {
				interp.execStmt(s.Post, forEnv)
			}
			_ = continued
		}

	case *ast.BreakStmt:
		panic(breakSignal{})

	case *ast.ContinueStmt:
		panic(continueSignal{})

	case *ast.ThrowStmt:
		msg := valueString(interp.evalExpr(s.Message, env))
		panic(throwSignal{message: msg})

	case *ast.TryCatchStmt:
		func() {
			defer func() {
				if r := recover(); r != nil {
					switch sig := r.(type) {
					case throwSignal:
						catchEnv := newEnv(env)
						catchEnv.define(s.CatchVar.Literal, sig.message)
						interp.execBlock(s.CatchBlock, catchEnv)
					default:
						panic(r) // re-panic non-throw signals
					}
				}
			}()
			interp.execBlock(s.TryBlock, newEnv(env))
		}()

	case *ast.MatchStmt:
		subject := interp.evalExpr(s.Subject, env)
		matched := false
		for _, c := range s.Cases {
			if c.Value == nil && !matched {
				// noFilter (default)
				interp.execBlock(c.Body, newEnv(env))
				matched = true
				break
			}
			if c.Value != nil {
				caseVal := interp.evalExpr(c.Value, env)
				if valuesEqual(subject, caseVal) {
					interp.execBlock(c.Body, newEnv(env))
					matched = true
					break
				}
			}
		}
		// If nothing matched and there's a default, run it
		if !matched {
			for _, c := range s.Cases {
				if c.Value == nil {
					interp.execBlock(c.Body, newEnv(env))
					break
				}
			}
		}

	case *ast.ExprStmt:
		interp.evalExpr(s.Expr, env)
	}
}

// ─── Expression Evaluation ────────────────────────────────────────────────────

func (interp *Interpreter) evalExpr(expr ast.Expression, env *Env) Value {
	switch e := expr.(type) {
	case *ast.IntLit:    return e.Value
	case *ast.FloatLit:  return e.Value
	case *ast.StringLit: return e.Value
	case *ast.BoolLit:   return e.Value
	case *ast.NullLit:   return nil

	case *ast.Identifier:
		val, ok := env.get(e.Name)
		if !ok {
			fmt.Fprintf(os.Stderr, "runtime error: undefined variable %q\n", e.Name)
			os.Exit(1)
		}
		return val

	case *ast.UnaryExpr:
		op  := interp.evalExpr(e.Operand, env)
		switch e.Operator.Literal {
		case "-":
			if isFloat(op) { return -op.(float64) }
			return -op.(int64)
		case "!":
			return !isTruthy(op)
		}

	case *ast.BinaryExpr:
		return interp.evalBinary(e, env)

	case *ast.CallExpr:
		return interp.evalCall(e, env)
	}
	return nil
}

func (interp *Interpreter) evalBinary(e *ast.BinaryExpr, env *Env) Value {
	left  := interp.evalExpr(e.Left, env)
	right := interp.evalExpr(e.Right, env)
	op    := e.Operator.Literal

	// String concatenation
	if op == "+" && isString(left) && isString(right) {
		return left.(string) + right.(string)
	}

	// Numeric operations
	switch op {
	case "+":  return numericOp(left, right, func(a, b int64) Value { return a+b }, func(a, b float64) Value { return a+b })
	case "-":  return numericOp(left, right, func(a, b int64) Value { return a-b }, func(a, b float64) Value { return a-b })
	case "*":  return numericOp(left, right, func(a, b int64) Value { return a*b }, func(a, b float64) Value { return a*b })
	case "/":
		if isZero(right) { panic(throwSignal{message: "division by zero — no cap"}) }
		return numericOp(left, right, func(a, b int64) Value { return a/b }, func(a, b float64) Value { return a/b })
	case "%":
		return left.(int64) % right.(int64)
	case "==", "isGiving":  return valuesEqual(left, right)
	case "!=":              return !valuesEqual(left, right)
	case "<":   return toFloat64(left) < toFloat64(right)
	case ">":   return toFloat64(left) > toFloat64(right)
	case "<=":  return toFloat64(left) <= toFloat64(right)
	case ">=":  return toFloat64(left) >= toFloat64(right)
	case "&&":  return isTruthy(left) && isTruthy(right)
	case "||":  return isTruthy(left) || isTruthy(right)
	}
	return nil
}

func (interp *Interpreter) evalCall(e *ast.CallExpr, env *Env) Value {
	fn, ok := interp.functions[e.Function.Literal]
	if !ok {
		fmt.Fprintf(os.Stderr, "runtime error: undefined forge %q\n", e.Function.Literal)
		os.Exit(1)
	}

	args := make([]Value, len(e.Args))
	for i, arg := range e.Args {
		args[i] = interp.evalExpr(arg, env)
	}

	// Build a new scope and call
	callEnv := newEnv(interp.globals)
	for i, param := range fn.Params {
		if i < len(args) { callEnv.define(param.Name.Literal, args[i]) }
	}

	var retVal Value
	func() {
		defer func() {
			if r := recover(); r != nil {
				switch sig := r.(type) {
				case returnSignal:
					retVal = sig.value
				default:
					panic(r) // propagate throw/break/continue up
				}
			}
		}()
		interp.execBlock(fn.Body, callEnv)
	}()
	return retVal
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func isTruthy(v Value) bool {
	if v == nil { return false }
	switch x := v.(type) {
	case bool:  return x
	case int64: return x != 0
	default:    return true
	}
}

func isZero(v Value) bool {
	switch x := v.(type) {
	case int64:   return x == 0
	case float64: return x == 0.0
	}
	return false
}

func valuesEqual(a, b Value) bool {
	if a == nil && b == nil { return true }
	if a == nil || b == nil { return false }
	// Numeric comparison across int/float
	if (isInt(a) || isFloat(a)) && (isInt(b) || isFloat(b)) {
		return math.Abs(toFloat64(a) - toFloat64(b)) < 1e-15
	}
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

func numericOp(a, b Value,
	intOp   func(int64, int64) Value,
	floatOp func(float64, float64) Value) Value {

	if isFloat(a) || isFloat(b) {
		return floatOp(toFloat64(a), toFloat64(b))
	}
	return intOp(a.(int64), b.(int64))
}

// ─── Pretty-printer for debugging ─────────────────────────────────────────────

// PrintAST prints a simplified view of the AST for debugging.
// Useful in the REPL with a :ast command.
func PrintAST(node ast.Node, indent int) {
	prefix := strings.Repeat("  ", indent)
	switch n := node.(type) {
	case *ast.Program:
		fmt.Printf("%sProgram\n", prefix)
		for _, fn := range n.Functions { PrintAST(fn, indent+1) }
	case *ast.FuncDecl:
		fmt.Printf("%sForge %s\n", prefix, n.Name.Literal)
		PrintAST(n.Body, indent+1)
	case *ast.BlockStmt:
		for _, s := range n.Statements { PrintAST(s, indent) }
	case *ast.VarDeclStmt:
		fmt.Printf("%sVarDecl %s %s\n", prefix, n.Name.Literal, n.Type.Kind)
	case *ast.IfStmt:
		fmt.Printf("%sPerchance\n", prefix)
		PrintAST(n.Consequence, indent+1)
	case *ast.WhileStmt:
		fmt.Printf("%sLetHimCook\n", prefix)
		PrintAST(n.Body, indent+1)
	case *ast.ReturnStmt:
		fmt.Printf("%sItIsWhatItIs\n", prefix)
	case *ast.PrintStmt:
		fmt.Printf("%sSpill\n", prefix)
	case *ast.BinaryExpr:
		fmt.Printf("%sBinary %s\n", prefix, n.Operator.Literal)
		PrintAST(n.Left, indent+1)
		PrintAST(n.Right, indent+1)
	case *ast.IntLit:
		fmt.Printf("%sIntLit %d\n", prefix, n.Value)
	case *ast.Identifier:
		fmt.Printf("%sIdent %s\n", prefix, n.Name)
	default:
		fmt.Printf("%s%T\n", prefix, node)
	}
}
