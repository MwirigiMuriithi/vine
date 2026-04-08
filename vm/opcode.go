// VM opcodes and instruction definitions for the Vine Virtual Machine.
//
// ─── Instruction Set ──────────────────────────────────────────────────────────
// Each instruction has:
//   Op      — the operation code (what to do)
//   Operand — optional argument (a value, variable name, or jump target)
//
// The VM is STACK-BASED: operations read from and write to a value stack.
//   PUSH_INT 5   → stack: [5]
//   PUSH_INT 3   → stack: [5, 3]
//   ADD          → stack: [8]   (pops 5 and 3, pushes 8)
package vm

import "fmt"

// Opcode names every operation the VM understands.
type Opcode int

const (
	// ── Stack ────────────────────────────────────────────────────────────────
	OP_PUSH_INT    Opcode = iota // push integer constant    operand: int64
	OP_PUSH_FLOAT                // push float constant      operand: float64
	OP_PUSH_STRING               // push string constant     operand: string
	OP_PUSH_BOOL                 // push bool constant       operand: bool
	OP_PUSH_NULL                 // push null (ghosted)
	OP_POP                       // discard top of stack

	// ── Arithmetic ───────────────────────────────────────────────────────────
	OP_ADD // pop b, pop a → push a + b   (also string concat)
	OP_SUB // pop b, pop a → push a - b
	OP_MUL // pop b, pop a → push a * b
	OP_DIV // pop b, pop a → push a / b
	OP_MOD // pop b, pop a → push a % b
	OP_NEG // pop a        → push -a

	// ── Comparison ───────────────────────────────────────────────────────────
	OP_EQ  // == / isGiving
	OP_NEQ // !=
	OP_LT  // <
	OP_GT  // >
	OP_LEQ // <=
	OP_GEQ // >=

	// ── Logic ────────────────────────────────────────────────────────────────
	OP_AND // &&
	OP_OR  // ||
	OP_NOT // !

	// ── Variables ────────────────────────────────────────────────────────────
	OP_LOAD  // operand: name (string) → push variable's value
	OP_STORE // operand: name (string) → pop stack → store in variable

	// ── Control Flow ─────────────────────────────────────────────────────────
	OP_JUMP           // operand: int — unconditional jump
	OP_JUMP_IF_FALSE  // operand: int — jump if top is falsy; pops condition

	// ── Functions ────────────────────────────────────────────────────────────
	OP_CALL   // operand: CallInfo — call a function
	OP_RETURN // operand: bool (hasValue) — return from function

	// ── I/O ──────────────────────────────────────────────────────────────────
	OP_PRINT // pop top, print to stdout   (spill)

	// ── Error Handling ───────────────────────────────────────────────────────
	OP_THROW     // pop message, throw an error           (throwHands)
	OP_TRY_START // operand: int (catch handler address)  (attempt)
	OP_TRY_END   // pop the try frame

	// ── Misc ─────────────────────────────────────────────────────────────────
	OP_HALT // stop execution
)

// CallInfo bundles data for a CALL instruction.
type CallInfo struct {
	FuncName string
	NumArgs  int
}

// Instruction is one single VM step.
type Instruction struct {
	Op      Opcode
	Operand interface{}
}

// String returns a human-readable disassembly of an instruction.
func (i Instruction) String() string {
	switch i.Op {
	case OP_PUSH_INT:    return fmt.Sprintf("PUSH_INT      %v", i.Operand)
	case OP_PUSH_FLOAT:  return fmt.Sprintf("PUSH_FLOAT    %v", i.Operand)
	case OP_PUSH_STRING: return fmt.Sprintf("PUSH_STR      %q", i.Operand)
	case OP_PUSH_BOOL:   return fmt.Sprintf("PUSH_BOOL     %v", i.Operand)
	case OP_PUSH_NULL:   return "PUSH_NULL"
	case OP_POP:         return "POP"
	case OP_ADD:         return "ADD"
	case OP_SUB:         return "SUB"
	case OP_MUL:         return "MUL"
	case OP_DIV:         return "DIV"
	case OP_MOD:         return "MOD"
	case OP_NEG:         return "NEG"
	case OP_EQ:          return "EQ         (isGiving)"
	case OP_NEQ:         return "NEQ"
	case OP_LT:          return "LT"
	case OP_GT:          return "GT"
	case OP_LEQ:         return "LEQ"
	case OP_GEQ:         return "GEQ"
	case OP_AND:         return "AND"
	case OP_OR:          return "OR"
	case OP_NOT:         return "NOT"
	case OP_LOAD:        return fmt.Sprintf("LOAD          %v", i.Operand)
	case OP_STORE:       return fmt.Sprintf("STORE         %v", i.Operand)
	case OP_JUMP:        return fmt.Sprintf("JUMP          → %v", i.Operand)
	case OP_JUMP_IF_FALSE: return fmt.Sprintf("JUMP_FALSE    → %v", i.Operand)
	case OP_CALL:
		ci := i.Operand.(CallInfo)
		return fmt.Sprintf("CALL          %s (%d args)", ci.FuncName, ci.NumArgs)
	case OP_RETURN:      return fmt.Sprintf("RETURN        hasValue=%v", i.Operand)
	case OP_PRINT:       return "PRINT         (spill)"
	case OP_THROW:       return "THROW         (throwHands)"
	case OP_TRY_START:   return fmt.Sprintf("TRY_START     catch → %v", i.Operand)
	case OP_TRY_END:     return "TRY_END"
	case OP_HALT:        return "HALT"
	default:             return fmt.Sprintf("UNKNOWN(%d)", i.Op)
	}
}

// CompiledFunction holds the bytecode for one Vine function.
type CompiledFunction struct {
	Name         string
	ParamNames   []string
	Instructions []Instruction
}

// Disassemble returns a human-readable listing of this function's bytecode.
func (cf *CompiledFunction) Disassemble() string {
	out := fmt.Sprintf("=== forge: %s ===\n", cf.Name)
	for i, instr := range cf.Instructions {
		out += fmt.Sprintf("  %04d  %s\n", i, instr.String())
	}
	return out
}

// Program is the compiled representation of a complete Vine source file.
type Program struct {
	Functions map[string]*CompiledFunction
}

// NewProgram creates an empty Program.
func NewProgram() *Program { return &Program{Functions: make(map[string]*CompiledFunction)} }

// Disassemble returns bytecode for all functions in the program.
func (p *Program) Disassemble() string {
	out := ""
	if main, ok := p.Functions["main"]; ok { out += main.Disassemble() + "\n" }
	for name, fn := range p.Functions {
		if name != "main" { out += fn.Disassemble() + "\n" }
	}
	return out
}
