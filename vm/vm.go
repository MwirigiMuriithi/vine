// VM — the Vine Virtual Machine execution engine.
//
// ─── Execution Model ──────────────────────────────────────────────────────────
// The VM maintains:
//
//   VALUE STACK  — a LIFO stack of Values. Operations push operands, pop them
//                  to compute results, and push results back.
//
//   CALL STACK   — one CallFrame per active function invocation. Each frame
//                  tracks: current function, instruction pointer (IP), locals.
//
//   TRY STACK    — records active attempt/catch handlers. When throwHands is
//                  executed, we unwind to the nearest try frame.
//
// ─── Instruction Cycle ────────────────────────────────────────────────────────
//   1. Read instruction at current IP
//   2. Increment IP (so jumps can override it)
//   3. Execute instruction
//   Repeat until call stack is empty or HALT.
package vm

import (
	"fmt"
	"math"
	"os"
)

// ─── Call Frame ───────────────────────────────────────────────────────────────

// CallFrame represents one active function call.
type CallFrame struct {
	fn     *CompiledFunction
	ip     int
	locals map[string]Value
}

// tryFrame records an active attempt/catch handler.
type tryFrame struct {
	callStackDepth  int // how many call frames were on the stack when try started
	valueStackDepth int // value stack depth at try entry (for unwinding)
	catchAddr       int // instruction index of the catch handler
	frame           *CallFrame // the call frame that owns this try
}

// ─── VM ───────────────────────────────────────────────────────────────────────

// VM is the execution engine.
type VM struct {
	program    *Program
	valueStack []Value
	callStack  []CallFrame
	tryStack   []tryFrame
}

// New creates a VM loaded with the given compiled program.
func New(program *Program) *VM { return &VM{program: program} }

// Run executes the program starting from "main".
func (v *VM) Run() error {
	mainFn, ok := v.program.Functions["main"]
	if !ok { return fmt.Errorf("no 'main' forge defined") }
	v.pushFrame(mainFn, []Value{})

	for len(v.callStack) > 0 {
		frame := v.currentFrame()
		if frame.ip >= len(frame.fn.Instructions) {
			v.popFrame()
			continue
		}
		instr := frame.fn.Instructions[frame.ip]
		frame.ip++
		if err := v.execute(instr, frame); err != nil {
			return err
		}
	}
	return nil
}

// execute performs one instruction.
func (v *VM) execute(instr Instruction, frame *CallFrame) error {
	switch instr.Op {

	// ── Push Literals ─────────────────────────────────────────────────────
	case OP_PUSH_INT:    v.push(IntValue(instr.Operand.(int64)))
	case OP_PUSH_FLOAT:  v.push(FloatValue(instr.Operand.(float64)))
	case OP_PUSH_STRING: v.push(StringValue(instr.Operand.(string)))
	case OP_PUSH_BOOL:   v.push(BoolValue(instr.Operand.(bool)))
	case OP_PUSH_NULL:   v.push(NilValue())
	case OP_POP:         v.pop()

	// ── Arithmetic ────────────────────────────────────────────────────────
	case OP_ADD:
		b, a := v.pop(), v.pop()
		v.push(v.add(a, b))
	case OP_SUB:
		b, a := v.pop(), v.pop()
		v.push(v.numericOp(a, b, func(x, y int64) Value { return IntValue(x-y) }, func(x, y float64) Value { return FloatValue(x-y) }))
	case OP_MUL:
		b, a := v.pop(), v.pop()
		v.push(v.numericOp(a, b, func(x, y int64) Value { return IntValue(x*y) }, func(x, y float64) Value { return FloatValue(x*y) }))
	case OP_DIV:
		b, a := v.pop(), v.pop()
		if b.Kind == KindInt && b.IntVal == 0   { return v.throwError("division by zero — no cap") }
		if b.Kind == KindFloat && b.FltVal == 0 { return v.throwError("division by zero — no cap") }
		v.push(v.numericOp(a, b, func(x, y int64) Value { return IntValue(x/y) }, func(x, y float64) Value { return FloatValue(x/y) }))
	case OP_MOD:
		b, a := v.pop(), v.pop()
		if b.IntVal == 0 { return v.throwError("modulo by zero") }
		v.push(IntValue(a.IntVal % b.IntVal))
	case OP_NEG:
		a := v.pop()
		if a.Kind == KindFloat { v.push(FloatValue(-a.FltVal)) } else { v.push(IntValue(-a.IntVal)) }

	// ── Comparison ────────────────────────────────────────────────────────
	case OP_EQ:  b, a := v.pop(), v.pop(); v.push(BoolValue(v.valuesEqual(a, b)))
	case OP_NEQ: b, a := v.pop(), v.pop(); v.push(BoolValue(!v.valuesEqual(a, b)))
	case OP_LT:  b, a := v.pop(), v.pop(); v.push(BoolValue(v.compareNums(a, b) < 0))
	case OP_GT:  b, a := v.pop(), v.pop(); v.push(BoolValue(v.compareNums(a, b) > 0))
	case OP_LEQ: b, a := v.pop(), v.pop(); v.push(BoolValue(v.compareNums(a, b) <= 0))
	case OP_GEQ: b, a := v.pop(), v.pop(); v.push(BoolValue(v.compareNums(a, b) >= 0))

	// ── Logic ─────────────────────────────────────────────────────────────
	case OP_AND: b, a := v.pop(), v.pop(); v.push(BoolValue(a.BoolVal && b.BoolVal))
	case OP_OR:  b, a := v.pop(), v.pop(); v.push(BoolValue(a.BoolVal || b.BoolVal))
	case OP_NOT: a := v.pop(); v.push(BoolValue(!a.BoolVal))

	// ── Variables ─────────────────────────────────────────────────────────
	case OP_LOAD:
		name := instr.Operand.(string)
		val, ok := frame.locals[name]
		if !ok { return v.throwError(fmt.Sprintf("undefined variable %q", name)) }
		v.push(val)
	case OP_STORE:
		frame.locals[instr.Operand.(string)] = v.pop()

	// ── Control Flow ──────────────────────────────────────────────────────
	case OP_JUMP:
		frame.ip = instr.Operand.(int)
	case OP_JUMP_IF_FALSE:
		if !v.pop().IsTruthy() { frame.ip = instr.Operand.(int) }

	// ── Functions ─────────────────────────────────────────────────────────
	case OP_CALL:
		ci := instr.Operand.(CallInfo)
		fn, ok := v.program.Functions[ci.FuncName]
		if !ok { return v.throwError(fmt.Sprintf("undefined forge %q", ci.FuncName)) }
		args := make([]Value, ci.NumArgs)
		for i := ci.NumArgs - 1; i >= 0; i-- { args[i] = v.pop() }
		v.pushFrame(fn, args)

	case OP_RETURN:
		hasValue := instr.Operand.(bool)
		var retVal Value
		if hasValue { retVal = v.pop() } else { retVal = NilValue() }
		v.popFrame()
		if len(v.callStack) > 0 && hasValue { v.push(retVal) }

	// ── I/O ───────────────────────────────────────────────────────────────
	case OP_PRINT:
		fmt.Fprintln(os.Stdout, v.pop().String())

	// ── Error Handling (throwHands / attempt / catch) ─────────────────────
	case OP_THROW:
		msg := v.pop().String()
		return v.throwError(msg)

	case OP_TRY_START:
		catchAddr := instr.Operand.(int)
		v.tryStack = append(v.tryStack, tryFrame{
			callStackDepth:  len(v.callStack),
			valueStackDepth: len(v.valueStack),
			catchAddr:       catchAddr,
			frame:           frame,
		})

	case OP_TRY_END:
		if len(v.tryStack) > 0 {
			v.tryStack = v.tryStack[:len(v.tryStack)-1]
		}

	// ── Halt ──────────────────────────────────────────────────────────────
	case OP_HALT:
		v.callStack = v.callStack[:0]

	default:
		return fmt.Errorf("runtime error: unknown opcode %d", instr.Op)
	}
	return nil
}

// throwError implements the throwHands / attempt-catch mechanism.
// If there's an active try frame, unwind to it and jump to the catch handler.
// Otherwise return a runtime error (program exits).
func (v *VM) throwError(msg string) error {
	if len(v.tryStack) > 0 {
		tf := v.tryStack[len(v.tryStack)-1]
		v.tryStack = v.tryStack[:len(v.tryStack)-1]

		// Unwind call stack to the frame that owns the try
		v.callStack = v.callStack[:tf.callStackDepth]
		// Unwind value stack
		v.valueStack = v.valueStack[:tf.valueStackDepth]
		// Push the error message as the caught value
		v.push(StringValue(msg))
		// Jump to catch handler
		tf.frame.ip = tf.catchAddr
		return nil
	}
	return fmt.Errorf("💀 throwHands: %s", msg)
}

// ─── Stack Operations ─────────────────────────────────────────────────────────

func (v *VM) push(val Value) { v.valueStack = append(v.valueStack, val) }

func (v *VM) pop() Value {
	if len(v.valueStack) == 0 {
		fmt.Fprintln(os.Stderr, "runtime error: value stack underflow")
		os.Exit(1)
	}
	n := len(v.valueStack)
	val := v.valueStack[n-1]
	v.valueStack = v.valueStack[:n-1]
	return val
}

func (v *VM) pushFrame(fn *CompiledFunction, args []Value) {
	frame := CallFrame{fn: fn, ip: 0, locals: make(map[string]Value)}
	for i, name := range fn.ParamNames {
		if i < len(args) { frame.locals[name] = args[i] }
	}
	v.callStack = append(v.callStack, frame)
}

func (v *VM) popFrame() {
	if len(v.callStack) > 0 { v.callStack = v.callStack[:len(v.callStack)-1] }
}

func (v *VM) currentFrame() *CallFrame { return &v.callStack[len(v.callStack)-1] }

// ─── Arithmetic Helpers ───────────────────────────────────────────────────────

func (v *VM) add(a, b Value) Value {
	if a.Kind == KindString && b.Kind == KindString {
		return StringValue(a.StrVal + b.StrVal)
	}
	return v.numericOp(a, b, func(x, y int64) Value { return IntValue(x+y) }, func(x, y float64) Value { return FloatValue(x+y) })
}

func (v *VM) numericOp(a, b Value, intOp func(int64, int64) Value, fltOp func(float64, float64) Value) Value {
	if a.Kind == KindFloat || b.Kind == KindFloat {
		return fltOp(toFloat(a), toFloat(b))
	}
	return intOp(a.IntVal, b.IntVal)
}

func toFloat(v Value) float64 {
	if v.Kind == KindFloat { return v.FltVal }
	return float64(v.IntVal)
}

func (v *VM) valuesEqual(a, b Value) bool {
	if a.Kind != b.Kind {
		if (a.Kind == KindInt || a.Kind == KindFloat) && (b.Kind == KindInt || b.Kind == KindFloat) {
			return toFloat(a) == toFloat(b)
		}
		return false
	}
	switch a.Kind {
	case KindInt:    return a.IntVal == b.IntVal
	case KindFloat:  return math.Abs(a.FltVal-b.FltVal) < 1e-15
	case KindString: return a.StrVal == b.StrVal
	case KindBool:   return a.BoolVal == b.BoolVal
	case KindNil:    return true
	}
	return false
}

func (v *VM) compareNums(a, b Value) float64 { return toFloat(a) - toFloat(b) }
