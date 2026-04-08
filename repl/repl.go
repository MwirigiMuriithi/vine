// Package repl implements an interactive Read-Eval-Print Loop (REPL) for Vine.
//
// ─── What is a REPL? ─────────────────────────────────────────────────────────
// A REPL lets you type Vine code one line at a time and see results immediately.
// It's the same experience as Python's `>>>` prompt or Node.js's interactive mode.
//
// ─── How it works ─────────────────────────────────────────────────────────────
// The REPL maintains a SESSION: a collection of forges (functions) declared so far.
// When you type a statement (not wrapped in a forge), the REPL automatically
// wraps it in a temporary  forge __repl__() void { }  and runs it.
// If you type a full forge definition, it adds it to the session.
//
// This lets you build up functions incrementally:
//   vine> forge double(n int) int { itIsWhatItIs n * 2 }
//   vine> spill(double(5))
//   10
//
// ─── Special commands ─────────────────────────────────────────────────────────
//   :quit / :q / :exit    exit the REPL
//   :help / :h            show help
//   :clear                clear the session (forget all declared forges)
//   :session              show all forges declared this session
package repl

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"vine/codegen"
	"vine/lexer"
	"vine/parser"
	"vine/semantic"
	"vine/vm"
)

const banner = `
🌿 Vine REPL — Interactive Mode
  Type Vine expressions or statements, press Enter to run.
  Type a full forge definition to add it to the session.
  Type :help for commands, :quit to exit.
──────────────────────────────────────────────────────────
`

const helpText = `
REPL Commands:
  :quit / :q / :exit      Exit the REPL
  :clear                  Clear all declared forges from the session
  :session                Show all forges declared this session
  :help / :h              Show this help

Vine Quick Reference:
  lowkey x int = 5                 mutable variable
  noCap PI float = 3.14            immutable constant
  spill(expr)                      print a value
  forge f(a int, b int) int { }    define a function
  perchance (cond) { } otherwise { }
  letHimCook (cond) { }
  spinTheBlock (init; cond; post) { }
  ghost                            break
  keepItMoving                     continue
  bet / nah                        true / false
  isGiving                         ==

Examples:
  spill("Hello, World!")
  lowkey x int = 10 + 5 * 2
  forge square(n int) int { itIsWhatItIs n * n }
  spill(square(7))
`

// Session holds the state accumulated during a REPL session.
type Session struct {
	// forgeSource stores the raw source of each declared forge.
	// When we run a new statement, we prefix the program with all of these.
	forgeSources []string
}

// addForge records a forge definition in the session.
func (s *Session) addForge(src string) {
	s.forgeSources = append(s.forgeSources, src)
}

// program returns the full source for the current session,
// optionally appending a one-shot  forge __repl__() void { <stmts> }
func (s *Session) program(stmts string) string {
	var sb strings.Builder
	for _, f := range s.forgeSources {
		sb.WriteString(f)
		sb.WriteString("\n")
	}
	if stmts != "" {
		sb.WriteString("forge __repl__() void {\n")
		sb.WriteString(stmts)
		sb.WriteString("\n}\n")
	}
	sb.WriteString("forge main() void { __repl__() }\n")
	return sb.String()
}

// Run starts the REPL and blocks until the user exits.
func Run() {
	fmt.Print(banner)

	session := &Session{}
	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Print("vine> ")

		if !scanner.Scan() {
			// EOF (Ctrl-D)
			fmt.Println("\nGoodbye! Keep it lowkey 🌿")
			return
		}

		line := strings.TrimSpace(scanner.Text())

		if line == "" {
			continue
		}

		// ── Special REPL commands ──────────────────────────────────────────
		switch strings.ToLower(line) {
		case ":quit", ":q", ":exit":
			fmt.Println("Goodbye! Keep it lowkey 🌿")
			return
		case ":help", ":h":
			fmt.Println(helpText)
			continue
		case ":clear":
			session = &Session{}
			fmt.Println("Session cleared — fresh start fr.")
			continue
		case ":session":
			if len(session.forgeSources) == 0 {
				fmt.Println("No forges declared this session.")
			} else {
				fmt.Println("── Declared forges ──────────────────────────────")
				for _, f := range session.forgeSources {
					fmt.Println(f)
				}
			}
			continue
		}

		// ── Multi-line input support ───────────────────────────────────────
		// If the line ends with '{', keep reading until we see a matching '}'
		input := line
		if strings.HasSuffix(line, "{") {
			depth := strings.Count(line, "{") - strings.Count(line, "}")
			for depth > 0 {
				fmt.Print("....  ")
				if !scanner.Scan() { break }
				next := scanner.Text()
				input += "\n" + next
				depth += strings.Count(next, "{") - strings.Count(next, "}")
			}
		}

		// ── Decide: is this a forge definition or a statement? ────────────
		trimmed := strings.TrimSpace(input)
		if strings.HasPrefix(trimmed, "forge ") {
			// It's a forge definition — add to session, don't run immediately
			session.addForge(trimmed)
			fmt.Println("✓ forge added to session")
		} else {
			// It's a statement — wrap in __repl__ and run
			evalInSession(session, input)
		}
	}
}

// evalInSession wraps `stmts` in a temporary forge and runs it with
// all the forges accumulated in `session` available.
func evalInSession(session *Session, stmts string) {
	// If there are no declared forges, we need a simpler main
	var source string
	if len(session.forgeSources) == 0 {
		source = "forge __repl__() void {\n" + stmts + "\n}\nforge main() void { __repl__() }\n"
	} else {
		source = session.program(stmts)
	}

	output, err := runSource(source)
	if err != nil {
		fmt.Fprintf(os.Stderr, "✗ %v\n", err)
		return
	}
	if output != "" {
		fmt.Println(output)
	}
}

// runSource runs a complete Vine source string through the full pipeline.
// Returns the stdout output and any error.
func runSource(source string) (string, error) {
	// Lex
	l := lexer.New(source)
	tokens, lexErrs := l.Tokenize()
	if len(lexErrs) > 0 {
		return "", fmt.Errorf("lexer: %s", strings.Join(lexErrs, "; "))
	}

	// Parse
	p := parser.New(tokens)
	prog, parseErrs := p.Parse()
	if len(parseErrs) > 0 {
		return "", fmt.Errorf("parser: %s", strings.Join(parseErrs, "; "))
	}

	// Semantic
	a := semantic.New()
	semErrs := a.Analyse(prog)
	if len(semErrs) > 0 {
		return "", fmt.Errorf("semantic: %s", strings.Join(semErrs, "; "))
	}

	// Codegen
	gen := codegen.New()
	vmProg, codeErrs := gen.Generate(prog)
	if len(codeErrs) > 0 {
		return "", fmt.Errorf("codegen: %s", strings.Join(codeErrs, "; "))
	}

	// Capture stdout
	oldOut := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	machine := vm.New(vmProg)
	runErr := machine.Run()

	w.Close()
	os.Stdout = oldOut

	buf := new(strings.Builder)
	buf2 := bufio.NewReader(r)
	for {
		b := make([]byte, 4096)
		n, err := buf2.Read(b)
		if n > 0 { buf.Write(b[:n]) }
		if err != nil { break }
	}

	output := strings.TrimRight(buf.String(), "\n")
	return output, runErr
}
