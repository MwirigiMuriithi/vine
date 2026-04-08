// Package vm implements the VIRTUAL MACHINE that executes Vine bytecode.
//
// ─── What is a Virtual Machine? ───────────────────────────────────────────────
// Rather than compiling Vine directly to native machine code (x86, ARM, etc.),
// we compile it to BYTECODE — instructions for an imaginary "virtual" CPU.
// The VM is a program that interprets that bytecode.
//
// This approach is used by Python, Java (JVM), Lua, and many others.
// It's simpler to implement than native code generation, and bytecode is
// portable — the same .vine bytecode runs on any platform that has the VM.
//
// ─── Stack-Based Architecture ─────────────────────────────────────────────────
// Our VM is STACK-BASED. Most operations work by:
//   1. PUSHING operands onto a value stack
//   2. Executing an operation that POPS operands and PUSHES a result
//
// Example: computing  3 + 4
//   PUSH_INT 3   →  stack: [3]
//   PUSH_INT 4   →  stack: [3, 4]
//   ADD          →  pops 3 and 4, pushes 7 → stack: [7]
//
// This is in contrast to REGISTER-BASED machines (like x86) where values
// are held in named registers.  Stack machines are simpler to compile for.
//
// ─── Files in this Package ────────────────────────────────────────────────────
//   value.go  — Value type (int, float, string, bool)
//   opcode.go — Instruction set (all the operations the VM understands)
//   vm.go     — The VM itself (execution engine)
package vm

import "fmt"

// ─── Values ───────────────────────────────────────────────────────────────────

// ValueKind identifies which type of value is stored in a Value struct.
type ValueKind int

const (
	KindInt    ValueKind = iota // 64-bit signed integer
	KindFloat                   // 64-bit IEEE 754 float
	KindString                  // UTF-8 string
	KindBool                    // boolean: true or false
	KindNil                     // absence of value (used for void returns)
)

// Value is a dynamically-typed value on the VM's value stack.
//
// In a statically-typed language like Vine, we know at compile time what type
// every expression has. However, the VM uses a UNIFORM value representation:
// every slot on the stack is a Value regardless of its type.
//
// A production VM might use tagged unions or NaN-boxing for efficiency,
// but for clarity we use a plain struct with all fields.
type Value struct {
	Kind    ValueKind
	IntVal  int64
	FltVal  float64
	StrVal  string
	BoolVal bool
}

// Constructors — convenient helpers for creating each kind of Value.

func IntValue(v int64) Value    { return Value{Kind: KindInt, IntVal: v} }
func FloatValue(v float64) Value { return Value{Kind: KindFloat, FltVal: v} }
func StringValue(v string) Value { return Value{Kind: KindString, StrVal: v} }
func BoolValue(v bool) Value    { return Value{Kind: KindBool, BoolVal: v} }
func NilValue() Value           { return Value{Kind: KindNil} }

// String returns a human-readable representation of a Value.
// This is used by the PRINT instruction and for debugging.
func (v Value) String() string {
	switch v.Kind {
	case KindInt:
		return fmt.Sprintf("%d", v.IntVal)
	case KindFloat:
		// Format: remove trailing zeros for clean output
		s := fmt.Sprintf("%g", v.FltVal)
		return s
	case KindString:
		return v.StrVal
	case KindBool:
		if v.BoolVal {
			return "true"
		}
		return "false"
	case KindNil:
		return "nil"
	}
	return "<unknown>"
}

// IsTruthy returns whether a value is considered "truthy" in a boolean context.
// In Vine, only bool values participate in conditions, but this helper is useful
// for the VM's conditional jump implementation.
func (v Value) IsTruthy() bool {
	switch v.Kind {
	case KindBool:
		return v.BoolVal
	case KindInt:
		return v.IntVal != 0
	default:
		return false
	}
}
