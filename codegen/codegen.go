// Package codegen implements CODE GENERATION for the Vine compiler.
//
// ─── What is Code Generation? ─────────────────────────────────────────────────
// Code generation is the final compilation phase. It walks the AST and emits
// bytecode instructions for the Vine VM.
//
// ─── Backpatching ─────────────────────────────────────────────────────────────
// For if/while/for/match, we emit JUMP instructions before knowing the target:
//   1. Emit JUMP with placeholder target (-1)
//   2. Record that instruction's index
//   3. After generating the destination, patch the placeholder with the real index
//
// ─── break (ghost) and continue (keepItMoving) ────────────────────────────────
// We maintain a stack of "loop context" objects. Each loop context records:
//   - A list of JUMP indices that need to be patched to the loop's END (ghost/break)
//   - A list of JUMP indices that need to be patched to the loop's START (keepItMoving/continue)
//
// When we see ghost/keepItMoving, we emit a JUMP placeholder and add it to the
// current loop context's list. When the loop ends, we patch all of them.
package codegen

import (
	"fmt"
	"vine/ast"
	"vine/lexer"
	"vine/vm"
)

// loopContext records pending jump patches for one loop (letHimCook / spinTheBlock).
type loopContext struct {
	breakPatches    []int // indices of JUMP instructions that need end-of-loop target
	continuePatches []int // indices of JUMP instructions that need start-of-loop target
}

// CodeGen compiles an AST into a vm.Program.
type CodeGen struct {
	program    *vm.Program
	current    *vm.CompiledFunction
	loopStack  []loopContext // stack of active loops (for ghost/keepItMoving)
	errors     []string
}

// New creates a fresh CodeGen.
func New() *CodeGen {
	return &CodeGen{program: vm.NewProgram()}
}

// Generate compiles the whole program and returns the vm.Program.
func (cg *CodeGen) Generate(program *ast.Program) (*vm.Program, []string) {
	for _, fn := range program.Functions {
		cg.compileFunc(fn)
	}
	return cg.program, cg.errors
}

// ─── Function ─────────────────────────────────────────────────────────────────

func (cg *CodeGen) compileFunc(fn *ast.FuncDecl) {
	compiled := &vm.CompiledFunction{Name: fn.Name.Literal}
	for _, p := range fn.Params {
		compiled.ParamNames = append(compiled.ParamNames, p.Name.Literal)
	}
	cg.current = compiled
	cg.compileBlock(fn.Body)
	// Implicit void return at end of function
	cg.emit(vm.Instruction{Op: vm.OP_RETURN, Operand: false})
	cg.program.Functions[fn.Name.Literal] = compiled
}

// ─── Statements ───────────────────────────────────────────────────────────────

func (cg *CodeGen) compileBlock(block *ast.BlockStmt) {
	for _, stmt := range block.Statements {
		cg.compileStatement(stmt)
	}
}

func (cg *CodeGen) compileStatement(stmt ast.Statement) {
	switch s := stmt.(type) {
	case *ast.VarDeclStmt:
		cg.compileExpr(s.Value)
		cg.emit(vm.Instruction{Op: vm.OP_STORE, Operand: s.Name.Literal})

	case *ast.AssignStmt:
		cg.compileExpr(s.Value)
		cg.emit(vm.Instruction{Op: vm.OP_STORE, Operand: s.Name.Literal})

	case *ast.IfStmt:
		cg.compileIf(s)

	case *ast.WhileStmt:
		cg.compileWhile(s)

	case *ast.ForStmt:
		cg.compileFor(s)

	case *ast.ReturnStmt:
		hasValue := s.Value != nil
		if hasValue { cg.compileExpr(s.Value) }
		cg.emit(vm.Instruction{Op: vm.OP_RETURN, Operand: hasValue})

	case *ast.PrintStmt:
		cg.compileExpr(s.Value)
		cg.emit(vm.Instruction{Op: vm.OP_PRINT})

	case *ast.ThrowStmt:
		cg.compileExpr(s.Message)
		cg.emit(vm.Instruction{Op: vm.OP_THROW})

	case *ast.TryCatchStmt:
		cg.compileTryCatch(s)

	case *ast.MatchStmt:
		cg.compileMatch(s)

	case *ast.ExprStmt:
		cg.compileExpr(s.Expr)
		cg.emit(vm.Instruction{Op: vm.OP_POP}) // discard expression result

	case *ast.BlockStmt:
		// ── Bug fix: plain 'otherwise' branches arrive as *ast.BlockStmt ────
		// The IfStmt's Alternative can be a *ast.BlockStmt when the else has
		// no further else-if chaining. We just compile it as a block.
		cg.compileBlock(s)

	case *ast.BreakStmt:
		// ghost: emit a JUMP with placeholder; record it for end-of-loop patching
		idx := cg.emitPlaceholder(vm.OP_JUMP)
		if len(cg.loopStack) > 0 {
			top := &cg.loopStack[len(cg.loopStack)-1]
			top.breakPatches = append(top.breakPatches, idx)
		}

	case *ast.ContinueStmt:
		// keepItMoving: emit JUMP to start of loop; patched when loop is closed
		idx := cg.emitPlaceholder(vm.OP_JUMP)
		if len(cg.loopStack) > 0 {
			top := &cg.loopStack[len(cg.loopStack)-1]
			top.continuePatches = append(top.continuePatches, idx)
		}

	default:
		cg.errorf("unknown statement type: %T", stmt)
	}
}

// ─── Control Flow Compilation ─────────────────────────────────────────────────

// compileIf: perchance cond { then } otherwise { else }
//
//   [cond]
//   JUMP_IF_FALSE → elseLabel
//   [then]
//   JUMP          → endLabel
//   elseLabel:
//   [else]
//   endLabel:
func (cg *CodeGen) compileIf(stmt *ast.IfStmt) {
	cg.compileExpr(stmt.Condition)
	jumpFalseIdx := cg.emitPlaceholder(vm.OP_JUMP_IF_FALSE)
	cg.compileBlock(stmt.Consequence)

	if stmt.Alternative != nil {
		jumpEndIdx := cg.emitPlaceholder(vm.OP_JUMP)
		cg.patch(jumpFalseIdx, cg.nextIndex())
		cg.compileStatement(stmt.Alternative) // handles *ast.BlockStmt or *ast.IfStmt
		cg.patch(jumpEndIdx, cg.nextIndex())
	} else {
		cg.patch(jumpFalseIdx, cg.nextIndex())
	}
}

// compileWhile: letHimCook cond { body }
//
//   startLabel:
//   [cond]
//   JUMP_IF_FALSE → endLabel
//   [body]
//   JUMP          → startLabel
//   endLabel:
func (cg *CodeGen) compileWhile(stmt *ast.WhileStmt) {
	startLabel := cg.nextIndex()
	cg.loopStack = append(cg.loopStack, loopContext{})

	cg.compileExpr(stmt.Condition)
	jumpFalseIdx := cg.emitPlaceholder(vm.OP_JUMP_IF_FALSE)
	cg.compileBlock(stmt.Body)
	cg.emit(vm.Instruction{Op: vm.OP_JUMP, Operand: startLabel})
	endLabel := cg.nextIndex()
	cg.patch(jumpFalseIdx, endLabel)

	// Patch all ghost (break) jumps to endLabel
	// Patch all keepItMoving (continue) jumps to startLabel
	ctx := cg.loopStack[len(cg.loopStack)-1]
	cg.loopStack = cg.loopStack[:len(cg.loopStack)-1]
	for _, idx := range ctx.breakPatches    { cg.patch(idx, endLabel) }
	for _, idx := range ctx.continuePatches { cg.patch(idx, startLabel) }
}

// compileFor: spinTheBlock (init; cond; post) { body }
//
//   [init]
//   startLabel:
//   [cond]
//   JUMP_IF_FALSE → endLabel
//   [body]
//   continueLabel:
//   [post]
//   JUMP          → startLabel
//   endLabel:
func (cg *CodeGen) compileFor(stmt *ast.ForStmt) {
	cg.compileStatement(stmt.Init)
	startLabel := cg.nextIndex()
	cg.loopStack = append(cg.loopStack, loopContext{})

	cg.compileExpr(stmt.Cond)
	jumpFalseIdx := cg.emitPlaceholder(vm.OP_JUMP_IF_FALSE)
	cg.compileBlock(stmt.Body)

	continueLabel := cg.nextIndex() // keepItMoving lands here (before post)
	cg.compileStatement(stmt.Post)
	cg.emit(vm.Instruction{Op: vm.OP_JUMP, Operand: startLabel})
	endLabel := cg.nextIndex()
	cg.patch(jumpFalseIdx, endLabel)

	ctx := cg.loopStack[len(cg.loopStack)-1]
	cg.loopStack = cg.loopStack[:len(cg.loopStack)-1]
	for _, idx := range ctx.breakPatches    { cg.patch(idx, endLabel) }
	for _, idx := range ctx.continuePatches { cg.patch(idx, continueLabel) }
}

// compileTryCatch: attempt { } catch <var> { }
func (cg *CodeGen) compileTryCatch(stmt *ast.TryCatchStmt) {
	// Emit TRY_START with placeholder for catch handler address
	tryStartIdx := cg.emitPlaceholder(vm.OP_TRY_START)

	cg.compileBlock(stmt.TryBlock)

	// After the try block, jump over the catch handler
	jumpOverCatch := cg.emitPlaceholder(vm.OP_JUMP)
	catchLabel := cg.nextIndex()
	cg.patch(tryStartIdx, catchLabel)

	// In the catch block, the thrown value is on the stack → store as catchVar
	cg.emit(vm.Instruction{Op: vm.OP_STORE, Operand: stmt.CatchVar.Literal})
	cg.compileBlock(stmt.CatchBlock)
	cg.patch(jumpOverCatch, cg.nextIndex())
	cg.emit(vm.Instruction{Op: vm.OP_TRY_END})
}

// compileMatch: checkTheFit (subject) { style val: {} ... noFilter: {} }
//
// Strategy: chain of equality checks, similar to if-else-if.
func (cg *CodeGen) compileMatch(stmt *ast.MatchStmt) {
	// We compile the subject once and store it in a temp variable
	// Then compare it against each case value.
	tempName := "__match_subject__"
	cg.compileExpr(stmt.Subject)
	cg.emit(vm.Instruction{Op: vm.OP_STORE, Operand: tempName})

	var jumpEnds []int

	for _, c := range stmt.Cases {
		if c.Value == nil {
			// noFilter (default) — always execute, no condition check
			cg.compileBlock(c.Body)
			jumpEnds = append(jumpEnds, cg.emitPlaceholder(vm.OP_JUMP))
		} else {
			// style <value>: — compare subject to value
			cg.emit(vm.Instruction{Op: vm.OP_LOAD, Operand: tempName})
			cg.compileExpr(c.Value)
			cg.emit(vm.Instruction{Op: vm.OP_EQ})
			skipIdx := cg.emitPlaceholder(vm.OP_JUMP_IF_FALSE)
			cg.compileBlock(c.Body)
			jumpEnds = append(jumpEnds, cg.emitPlaceholder(vm.OP_JUMP))
			cg.patch(skipIdx, cg.nextIndex())
		}
	}

	endLabel := cg.nextIndex()
	for _, idx := range jumpEnds {
		cg.patch(idx, endLabel)
	}
}

// ─── Expression Compilation ───────────────────────────────────────────────────

func (cg *CodeGen) compileExpr(expr ast.Expression) {
	switch e := expr.(type) {
	case *ast.IntLit:
		cg.emit(vm.Instruction{Op: vm.OP_PUSH_INT, Operand: e.Value})
	case *ast.FloatLit:
		cg.emit(vm.Instruction{Op: vm.OP_PUSH_FLOAT, Operand: e.Value})
	case *ast.StringLit:
		cg.emit(vm.Instruction{Op: vm.OP_PUSH_STRING, Operand: e.Value})
	case *ast.BoolLit:
		cg.emit(vm.Instruction{Op: vm.OP_PUSH_BOOL, Operand: e.Value})
	case *ast.NullLit:
		cg.emit(vm.Instruction{Op: vm.OP_PUSH_NULL})
	case *ast.Identifier:
		cg.emit(vm.Instruction{Op: vm.OP_LOAD, Operand: e.Name})
	case *ast.UnaryExpr:
		cg.compileExpr(e.Operand)
		switch e.Operator.Type {
		case lexer.TOKEN_MINUS: cg.emit(vm.Instruction{Op: vm.OP_NEG})
		case lexer.TOKEN_NOT:   cg.emit(vm.Instruction{Op: vm.OP_NOT})
		}
	case *ast.BinaryExpr:
		cg.compileBinary(e)
	case *ast.CallExpr:
		for _, arg := range e.Args { cg.compileExpr(arg) }
		cg.emit(vm.Instruction{Op: vm.OP_CALL, Operand: vm.CallInfo{FuncName: e.Function.Literal, NumArgs: len(e.Args)}})
	default:
		cg.errorf("unknown expression type: %T", expr)
	}
}

func (cg *CodeGen) compileBinary(e *ast.BinaryExpr) {
	cg.compileExpr(e.Left)
	cg.compileExpr(e.Right)
	switch e.Operator.Type {
	case lexer.TOKEN_PLUS:    cg.emit(vm.Instruction{Op: vm.OP_ADD})
	case lexer.TOKEN_MINUS:   cg.emit(vm.Instruction{Op: vm.OP_SUB})
	case lexer.TOKEN_STAR:    cg.emit(vm.Instruction{Op: vm.OP_MUL})
	case lexer.TOKEN_SLASH:   cg.emit(vm.Instruction{Op: vm.OP_DIV})
	case lexer.TOKEN_PERCENT: cg.emit(vm.Instruction{Op: vm.OP_MOD})
	case lexer.TOKEN_EQ:      cg.emit(vm.Instruction{Op: vm.OP_EQ})
	case lexer.TOKEN_NEQ:     cg.emit(vm.Instruction{Op: vm.OP_NEQ})
	case lexer.TOKEN_LT:      cg.emit(vm.Instruction{Op: vm.OP_LT})
	case lexer.TOKEN_GT:      cg.emit(vm.Instruction{Op: vm.OP_GT})
	case lexer.TOKEN_LEQ:     cg.emit(vm.Instruction{Op: vm.OP_LEQ})
	case lexer.TOKEN_GEQ:     cg.emit(vm.Instruction{Op: vm.OP_GEQ})
	case lexer.TOKEN_AND:     cg.emit(vm.Instruction{Op: vm.OP_AND})
	case lexer.TOKEN_OR:      cg.emit(vm.Instruction{Op: vm.OP_OR})
	default:
		cg.errorf("unknown binary operator: %s", e.Operator.Literal)
	}
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func (cg *CodeGen) emit(instr vm.Instruction) {
	cg.current.Instructions = append(cg.current.Instructions, instr)
}
func (cg *CodeGen) emitPlaceholder(op vm.Opcode) int {
	idx := cg.nextIndex()
	cg.emit(vm.Instruction{Op: op, Operand: -1})
	return idx
}
func (cg *CodeGen) patch(idx, target int) {
	cg.current.Instructions[idx].Operand = target
}
func (cg *CodeGen) nextIndex() int { return len(cg.current.Instructions) }
func (cg *CodeGen) errorf(format string, args ...interface{}) {
	cg.errors = append(cg.errors, "codegen error: "+fmt.Sprintf(format, args...))
}
