// main.go — The Vine Compiler / Interpreter Driver
//
// ─── Pipeline ─────────────────────────────────────────────────────────────────
//
//   Source (.vine)
//       │
//       ▼  LEXER      characters  →  token stream
//       ▼  PARSER     tokens      →  AST
//       ▼  SEMANTIC   AST         →  type-checked AST
//       ▼  CODEGEN    AST         →  VM bytecode      (default path)
//       ▼  VM         bytecode    →  output
//
//   OR (with --interpret flag):
//       ▼  EVAL       AST         →  output  (tree-walk interpreter)
//
// ─── Usage ────────────────────────────────────────────────────────────────────
//   vine                          Start the interactive REPL
//   vine <file.vine>              Compile and run a Vine source file
//   vine --interpret <file>       Run via the tree-walk interpreter instead
//   vine --dump-tokens   <file>   Print the token stream
//   vine --dump-ast      <file>   Print the AST
//   vine --dump-bytecode <file>   Print the VM bytecode disassembly
package main

import (
	"fmt"
	"os"
	"strings"

	"vine/codegen"
	"vine/eval"
	"vine/lexer"
	"vine/parser"
	"vine/repl"
	"vine/semantic"
	"vine/vm"
)

func main() {
	// No arguments → start the REPL
	if len(os.Args) == 1 {
		repl.Run()
		return
	}

	dumpTokens   := false
	dumpAST      := false
	dumpBytecode := false
	useInterp    := false
	var filePath string

	for _, arg := range os.Args[1:] {
		switch arg {
		case "--dump-tokens":   dumpTokens   = true
		case "--dump-ast":      dumpAST      = true
		case "--dump-bytecode": dumpBytecode = true
		case "--interpret":     useInterp    = true
		case "--help", "-h":    printHelp(); os.Exit(0)
		default:
			if strings.HasPrefix(arg, "--") {
				fmt.Fprintf(os.Stderr, "Unknown flag: %s\n", arg)
				os.Exit(1)
			}
			filePath = arg
		}
	}

	if filePath == "" {
		fmt.Fprintln(os.Stderr, "Error: no source file specified.")
		printHelp()
		os.Exit(1)
	}

	source, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading %q: %v\n", filePath, err)
		os.Exit(1)
	}

	compile(string(source), filePath, dumpTokens, dumpAST, dumpBytecode, useInterp)
}

func compile(source, filename string, dumpTokens, dumpAST, dumpBytecode, useInterp bool) {
	sep := "──────────────────────────────────────────────────────────"
	mode := "VM (bytecode)"
	if useInterp { mode = "Tree-Walk Interpreter" }
	fmt.Printf("🌿 Vine Compiler  [%s]\n   file: %s\n%s\n\n", mode, filename, sep)

	// ═══════════════ PHASE 1: LEXICAL ANALYSIS ═══════════════
	fmt.Println("Phase 1: Lexical Analysis...")
	lex := lexer.New(source)
	tokens, lexErrors := lex.Tokenize()

	if dumpTokens {
		fmt.Println("\n── Token Stream ──────────────────────────────────────────")
		for i, tok := range tokens {
			fmt.Printf("  [%04d]  %s\n", i, tok.String())
		}
		fmt.Println()
	}
	if len(lexErrors) > 0 { printErrors("Lexer", lexErrors); os.Exit(1) }
	fmt.Printf("  ✓ Produced %d tokens\n\n", len(tokens))

	// ═══════════════ PHASE 2: PARSING ═══════════════
	fmt.Println("Phase 2: Parsing (building AST)...")
	p := parser.New(tokens)
	program, parseErrors := p.Parse()

	if dumpAST {
		fmt.Println("\n── Abstract Syntax Tree ──────────────────────────────────")
		eval.PrintAST(program, 0)
		fmt.Println()
	}
	if len(parseErrors) > 0 { printErrors("Parser", parseErrors); os.Exit(1) }
	fmt.Printf("  ✓ Parsed %d forge(s)\n\n", len(program.Functions))

	// ═══════════════ PHASE 3: SEMANTIC ANALYSIS ═══════════════
	fmt.Println("Phase 3: Semantic Analysis (type checking)...")
	analyser := semantic.New()
	semErrors := analyser.Analyse(program)
	if len(semErrors) > 0 { printErrors("Semantic Analyser", semErrors); os.Exit(1) }
	fmt.Println("  ✓ No cap — no type errors\n")

	// ═══════════════ PATH A: TREE-WALK INTERPRETER ═══════════════
	if useInterp {
		fmt.Println("Phase 4: Interpreting (tree-walk)...\n")
		fmt.Println("── Program Output ────────────────────────────────────────")
		interpreter := eval.New()
		if err := interpreter.Interpret(program); err != nil {
			fmt.Fprintf(os.Stderr, "\nRuntime Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("\n" + sep)
		fmt.Println("✓ Program completed — that's a W.")
		return
	}

	// ═══════════════ PATH B: BYTECODE COMPILER + VM ═══════════════

	// ── Phase 4: Code Generation ────────────────────────────────
	fmt.Println("Phase 4: Code Generation (emitting bytecode)...")
	gen := codegen.New()
	vmProgram, codegenErrors := gen.Generate(program)
	if len(codegenErrors) > 0 { printErrors("Code Generator", codegenErrors); os.Exit(1) }
	fmt.Printf("  ✓ Compiled %d forge(s)\n\n", len(vmProgram.Functions))

	if dumpBytecode {
		fmt.Println("── Bytecode Disassembly ──────────────────────────────────")
		fmt.Print(vmProgram.Disassemble())
		fmt.Println()
	}

	// ── Phase 5: VM Execution ────────────────────────────────────
	fmt.Println("Phase 5: Executing...\n")
	fmt.Println("── Program Output ────────────────────────────────────────")

	machine := vm.New(vmProgram)
	if err := machine.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "\nRuntime Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n" + sep)
	fmt.Println("✓ Program completed — that's a W.")
}

func printErrors(phase string, errors []string) {
	fmt.Fprintf(os.Stderr, "\n✗ %s Errors:\n", phase)
	for _, e := range errors {
		fmt.Fprintf(os.Stderr, "  → %s\n", e)
	}
}

func printHelp() {
	fmt.Print(`
🌿 Vine Programming Language
==============================
A learning compiler & interpreter written in Go, using internet-culture keywords.

Usage:
  vine                              Start the interactive REPL
  vine <file.vine>                  Compile and run via the VM
  vine --interpret <file.vine>      Run via the tree-walk interpreter (simpler, slower)
  vine --dump-tokens   <file.vine>  Print the lexer token stream
  vine --dump-ast      <file.vine>  Print the AST structure
  vine --dump-bytecode <file.vine>  Print the VM bytecode disassembly
  vine --help                       Show this help

Vine Keyword Reference:
  lowkey x int = 5           mutable variable           (var)
  noCap / lockIn x int = 5   immutable constant         (const)
  forge f(a int) int { }     function definition        (func)
  itIsWhatItIs expr           return                     (return)
  spill(expr)                print to stdout             (print)
  perchance (cond) { }       if statement               (if)
  otherwise { }              else branch                (else)
  isGiving                   equality operator          (==)
  letHimCook (cond) { }      while loop                 (while)
  spinTheBlock (;;) { }      for loop                   (for)
  ghost                      break out of loop          (break)
  keepItMoving               continue loop              (continue)
  bet / nah                  true / false               (true/false)
  ghosted                    null                       (null)
  checkTheFit (x) { }        match/switch               (switch)
  style val: { }             case arm                   (case)
  noFilter: { }              default case               (default)
  attempt { } catch e { }    try/catch                  (try/catch)
  throwHands("msg")          throw error                (throw)

Examples:
  vine                              (REPL mode)
  vine examples/hello.vine
  vine examples/functions.vine
  vine --interpret examples/advanced.vine
  vine --dump-bytecode examples/control_flow.vine
`)
}
