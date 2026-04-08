// integration_test.go : End-to-end tests for the Vine compiler pipeline.
//
// ─── What are integration tests? ─────────────────────────────────────────────
// Unit tests check one component at a time (just the lexer, just the parser).
// Integration tests check the ENTIRE pipeline working together:
//   source code → lexer → parser → semantic → codegen → VM → output
//
// ─── How we capture output ────────────────────────────────────────────────────
// The VM writes to os.Stdout using fmt.Fprintln. To capture that in tests we
// temporarily redirect os.Stdout to a pipe, run the program, then read back
// whatever was written.
//
// This is a standard Go testing technique for testing programs that print output.
package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	"vine/codegen"
	"vine/lexer"
	"vine/parser"
	"vine/semantic"
	"vine/vm"
)

// ─── Test Infrastructure ──────────────────────────────────────────────────────

// runProgram compiles and executes a Vine source string.
// Returns the captured stdout output and any error.
func runProgram(source string) (string, error) {
	// ── Lex ──────────────────────────────────────────────────────────────
	l := lexer.New(source)
	tokens, lexErrs := l.Tokenize()
	if len(lexErrs) > 0 {
		return "", fmt.Errorf("lex errors: %v", lexErrs)
	}

	// ── Parse ─────────────────────────────────────────────────────────────
	p := parser.New(tokens)
	prog, parseErrs := p.Parse()
	if len(parseErrs) > 0 {
		return "", fmt.Errorf("parse errors: %v", parseErrs)
	}

	// ── Semantic ──────────────────────────────────────────────────────────
	a := semantic.New()
	semErrs := a.Analyse(prog)
	if len(semErrs) > 0 {
		return "", fmt.Errorf("semantic errors: %v", semErrs)
	}

	// ── Codegen ───────────────────────────────────────────────────────────
	gen := codegen.New()
	vmProg, codeErrs := gen.Generate(prog)
	if len(codeErrs) > 0 {
		return "", fmt.Errorf("codegen errors: %v", codeErrs)
	}

	// ── Capture stdout ────────────────────────────────────────────────────
	// Redirect os.Stdout to a pipe so we can read what the VM prints.
	oldStdout := os.Stdout
	r, w, _   := os.Pipe()
	os.Stdout  = w

	machine := vm.New(vmProg)
	runErr  := machine.Run()

	// Restore stdout before checking errors (so t.Errorf still works)
	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	return strings.TrimSpace(output), runErr
}

// assertOutput is a helper that runs a program and checks the output.
func assertOutput(t *testing.T, source, expected string) {
	t.Helper()
	got, err := runProgram(source)
	if err != nil {
		t.Fatalf("runtime error: %v", err)
	}
	if got != strings.TrimSpace(expected) {
		t.Errorf("output mismatch:\n  got:  %q\n  want: %q", got, expected)
	}
}

// ─── Basic Output ─────────────────────────────────────────────────────────────

func TestHelloWorld(t *testing.T) {
	assertOutput(t,
		`forge main() void { spill("Hello, World!") }`,
		"Hello, World!")
}

func TestSpillInt(t *testing.T) {
	assertOutput(t, `forge main() void { spill(42) }`, "42")
}

func TestSpillBool(t *testing.T) {
	assertOutput(t, `forge main() void { spill(bet) }`, "true")
	assertOutput(t, `forge main() void { spill(nah) }`, "false")
}

// ─── Arithmetic ───────────────────────────────────────────────────────────────

func TestArithmetic(t *testing.T) {
	tests := []struct{ expr, want string }{
		{"10 + 3",  "13"},
		{"10 - 3",  "7"},
		{"10 * 3",  "30"},
		{"10 / 3",  "3"},
		{"10 % 3",  "1"},
		{"-5",      "-5"},
	}
	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			src := fmt.Sprintf(`forge main() void { spill(%s) }`, tt.expr)
			assertOutput(t, src, tt.want)
		})
	}
}

func TestStringConcat(t *testing.T) {
	assertOutput(t,
		`forge main() void { spill("Hello, " + "Vine!") }`,
		"Hello, Vine!")
}

// ─── Variables ────────────────────────────────────────────────────────────────

func TestLowkeyMutation(t *testing.T) {
	assertOutput(t, `forge main() void {
    lowkey x int = 0
    x = x + 1
    x = x + 1
    spill(x)
}`, "2")
}

func TestNoCapConstant(t *testing.T) {
	assertOutput(t, `forge main() void {
    noCap N int = 99
    spill(N)
}`, "99")
}

// ─── Control Flow ─────────────────────────────────────────────────────────────

func TestPerchance(t *testing.T) {
	assertOutput(t, `forge main() void {
    perchance (bet) { spill("yes") } otherwise { spill("no") }
}`, "yes")
}

func TestOtherwise(t *testing.T) {
	assertOutput(t, `forge main() void {
    perchance (nah) { spill("yes") } otherwise { spill("no") }
}`, "no")
}

func TestIsGiving(t *testing.T) {
	assertOutput(t, `forge main() void {
    lowkey x int = 5
    perchance (x isGiving 5) { spill("match") } otherwise { spill("no match") }
}`, "match")
}

func TestLetHimCook(t *testing.T) {
	assertOutput(t, `forge main() void {
    lowkey i int = 0
    lowkey sum int = 0
    letHimCook (i < 5) {
        sum = sum + i
        i = i + 1
    }
    spill(sum)
}`, "10") // 0+1+2+3+4 = 10
}

func TestSpinTheBlock(t *testing.T) {
	assertOutput(t, `forge main() void {
    lowkey total int = 0
    spinTheBlock (lowkey i int = 1; i <= 5; i = i + 1) {
        total = total + i
    }
    spill(total)
}`, "15") // 1+2+3+4+5 = 15
}

func TestGhost(t *testing.T) {
	// ghost should break out when i == 3; so we only print 0,1,2
	assertOutput(t, `forge main() void {
    spinTheBlock (lowkey i int = 0; i < 10; i = i + 1) {
        perchance (i isGiving 3) { ghost }
        spill(i)
    }
}`, "0\n1\n2")
}

// ─── Functions ────────────────────────────────────────────────────────────────

func TestFunctionCall(t *testing.T) {
	assertOutput(t, `
forge add(a int, b int) int { itIsWhatItIs a + b }
forge main() void { spill(add(3, 4)) }
`, "7")
}

func TestRecursiveFactorial(t *testing.T) {
	assertOutput(t, `
forge factorial(n int) int {
    perchance (n <= 1) { itIsWhatItIs 1 }
    itIsWhatItIs n * factorial(n - 1)
}
forge main() void { spill(factorial(5)) }
`, "120")
}

func TestFibonacci(t *testing.T) {
	assertOutput(t, `
forge fib(n int) int {
    perchance (n <= 0) { itIsWhatItIs 0 }
    perchance (n isGiving 1) { itIsWhatItIs 1 }
    itIsWhatItIs fib(n-1) + fib(n-2)
}
forge main() void { spill(fib(10)) }
`, "55")
}

// ─── checkTheFit (match/switch) ───────────────────────────────────────────────

func TestCheckTheFit(t *testing.T) {
	assertOutput(t, `forge main() void {
    lowkey x int = 2
    checkTheFit (x) {
        style 1: { spill("one") }
        style 2: { spill("two") }
        noFilter: { spill("other") }
    }
}`, "two")
}

func TestCheckTheFitDefault(t *testing.T) {
	assertOutput(t, `forge main() void {
    lowkey x int = 99
    checkTheFit (x) {
        style 1: { spill("one") }
        noFilter: { spill("default") }
    }
}`, "default")
}

// ─── attempt / catch ──────────────────────────────────────────────────────────

func TestAttemptNoCatch(t *testing.T) {
	// When no error is thrown, the catch block should not run
	assertOutput(t, `forge main() void {
    attempt {
        spill("ok")
    } catch err {
        spill("caught: " + err)
    }
}`, "ok")
}

func TestAttemptCatch(t *testing.T) {
	// When throwHands fires, the catch block should run
	assertOutput(t, `forge main() void {
    attempt {
        throwHands("oops")
        spill("never")
    } catch err {
        spill("caught: " + err)
    }
}`, "caught: oops")
}

// ─── Division by Zero ─────────────────────────────────────────────────────────

func TestDivisionByZeroCaught(t *testing.T) {
	// The VM throws on div-by-zero; attempt/catch should handle it
	assertOutput(t, `forge main() void {
    attempt {
        lowkey x int = 10 / 0
        spill(x)
    } catch err {
        spill("safe")
    }
}`, "safe")
}
