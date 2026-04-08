// analyser_test.go : Unit tests for the Vine semantic analyser.
//
// ─── Strategy ─────────────────────────────────────────────────────────────────
// For semantic tests we:
//   1. Feed valid Vine programs → expect ZERO errors
//   2. Feed deliberately broken programs → expect specific error messages
//
// This verifies both that good programs pass and bad programs are caught.
package semantic

import (
	"strings"
	"testing"
	"vine/lexer"
	"vine/parser"
)

// ─── Helpers ──────────────────────────────────────────────────────────────────

// analyseSource lexes, parses, and semantically analyses the given source.
// Returns the list of semantic errors (empty = success).
func analyseSource(source string) []string {
	l   := lexer.New(source)
	tok, _ := l.Tokenize()
	p   := parser.New(tok)
	prog, _ := p.Parse()
	a   := New()
	return a.Analyse(prog)
}

// expectNoErrors fails if any errors are returned.
func expectNoErrors(t *testing.T, source string) {
	t.Helper()
	errs := analyseSource(source)
	if len(errs) > 0 {
		t.Errorf("expected no errors, got:\n  %s", strings.Join(errs, "\n  "))
	}
}

// expectError fails if NO error containing `substr` is found.
func expectError(t *testing.T, source, substr string) {
	t.Helper()
	errs := analyseSource(source)
	for _, e := range errs {
		if strings.Contains(e, substr) {
			return // found the expected error
		}
	}
	t.Errorf("expected error containing %q, but got:\n  %v", substr, errs)
}

// ─── Valid Program Tests ───────────────────────────────────────────────────────

// TestValidHello — the simplest valid program.
func TestValidHello(t *testing.T) {
	expectNoErrors(t, `forge main() void { spill("hello") }`)
}

// TestValidArithmetic — lowkey variables with arithmetic.
func TestValidArithmetic(t *testing.T) {
	expectNoErrors(t, `forge main() void {
    lowkey x int = 10
    lowkey y int = x + 5
    spill(y)
}`)
}

// TestValidIntToFloat — int value assigned to float variable (implicit widening).
func TestValidIntToFloat(t *testing.T) {
	expectNoErrors(t, `forge main() void {
    lowkey f float = 42
}`)
}

// TestValidForge — a forge calling another forge.
func TestValidForge(t *testing.T) {
	expectNoErrors(t, `
forge double(n int) int { itIsWhatItIs n * 2 }
forge main() void { spill(double(5)) }
`)
}

// TestValidRecursion — recursive forge (factorial).
func TestValidRecursion(t *testing.T) {
	expectNoErrors(t, `
forge factorial(n int) int {
    perchance (n <= 1) { itIsWhatItIs 1 }
    itIsWhatItIs n * factorial(n - 1)
}
forge main() void { spill(factorial(5)) }
`)
}

// TestValidMutualRecursion — two forges calling each other.
// This tests the two-pass approach (both must be registered before bodies are analysed).
func TestValidMutualRecursion(t *testing.T) {
	expectNoErrors(t, `
forge isEven(n int) bool {
    perchance (n isGiving 0) { itIsWhatItIs bet }
    itIsWhatItIs isOdd(n - 1)
}
forge isOdd(n int) bool {
    perchance (n isGiving 0) { itIsWhatItIs nah }
    itIsWhatItIs isEven(n - 1)
}
forge main() void { spill(isEven(4)) }
`)
}

// TestValidLetHimCook — while loop with ghost (break).
func TestValidLetHimCook(t *testing.T) {
	expectNoErrors(t, `forge main() void {
    lowkey i int = 0
    letHimCook (i < 10) {
        perchance (i isGiving 5) { ghost }
        i = i + 1
    }
}`)
}

// TestValidSpinTheBlock — for loop.
func TestValidSpinTheBlock(t *testing.T) {
	expectNoErrors(t, `forge main() void {
    spinTheBlock (lowkey i int = 0; i < 5; i = i + 1) {
        spill(i)
    }
}`)
}

// TestValidAttemptCatch — try/catch.
func TestValidAttemptCatch(t *testing.T) {
	expectNoErrors(t, `
forge risky(n int) int {
    perchance (n isGiving 0) { throwHands("zero!") }
    itIsWhatItIs n * 2
}
forge main() void {
    attempt {
        spill(risky(0))
    } catch err {
        spill("caught: " + err)
    }
}`)
}

// TestValidCheckTheFit — match/switch.
func TestValidCheckTheFit(t *testing.T) {
	expectNoErrors(t, `forge main() void {
    lowkey x int = 2
    checkTheFit (x) {
        style 1: { spill("one") }
        style 2: { spill("two") }
        noFilter: { spill("other") }
    }
}`)
}

// TestValidNoCap — immutable constant, not reassigned.
func TestValidNoCap(t *testing.T) {
	expectNoErrors(t, `forge main() void {
    noCap PI float = 3.14
    spill(PI)
}`)
}

// ─── Invalid Program Tests ────────────────────────────────────────────────────

// TestUndeclaredVariable — using a variable that was never declared.
func TestUndeclaredVariable(t *testing.T) {
	expectError(t,
		`forge main() void { spill(x) }`,
		"undeclared",
	)
}

// TestTypeMismatch — assigning a string to an int variable.
func TestTypeMismatch(t *testing.T) {
	expectError(t,
		`forge main() void { lowkey x int = "oops" }`,
		"type mismatch",
	)
}

// TestAssignToNoCap — reassigning an immutable noCap variable.
func TestAssignToNoCap(t *testing.T) {
	expectError(t,
		`forge main() void {
    noCap x int = 5
    x = 10
}`,
		"locked in",
	)
}

// TestUndeclaredFunction — calling a forge that doesn't exist.
func TestUndeclaredFunction(t *testing.T) {
	expectError(t,
		`forge main() void { spill(doesntExist(1, 2)) }`,
		"ghosted forge",
	)
}

// TestWrongArgCount — calling a forge with the wrong number of arguments.
func TestWrongArgCount(t *testing.T) {
	expectError(t, `
forge add(a int, b int) int { itIsWhatItIs a + b }
forge main() void { spill(add(1)) }`,
		"expects 2 arg",
	)
}

// TestWrongArgType — passing an int where a string is expected.
func TestWrongArgType(t *testing.T) {
	expectError(t, `
forge greet(name string) void { spill(name) }
forge main() void { greet(42) }`,
		"expected string",
	)
}

// TestWrongReturnType — returning an int from a bool forge.
func TestWrongReturnType(t *testing.T) {
	expectError(t, `
forge check() bool { itIsWhatItIs 42 }
forge main() void { spill(check()) }`,
		"returns bool",
	)
}

// TestIfConditionNotBool — using an int as a perchance condition.
func TestIfConditionNotBool(t *testing.T) {
	expectError(t,
		`forge main() void { perchance (42) { spill("hi") } }`,
		"must be bool",
	)
}

// TestWhileConditionNotBool — using a string as a letHimCook condition.
func TestWhileConditionNotBool(t *testing.T) {
	expectError(t,
		`forge main() void { letHimCook ("forever") { spill("oops") } }`,
		"must be bool",
	)
}

// TestGhostOutsideLoop — ghost (break) used outside any loop.
func TestGhostOutsideLoop(t *testing.T) {
	expectError(t,
		`forge main() void { ghost }`,
		"ghost",
	)
}

// TestDuplicateVariable — declaring the same variable twice in the same scope.
func TestDuplicateVariable(t *testing.T) {
	expectError(t,
		`forge main() void {
    lowkey x int = 1
    lowkey x int = 2
}`,
		"already declared",
	)
}

// TestDuplicateFunction — defining the same forge twice.
func TestDuplicateFunction(t *testing.T) {
	expectError(t, `
forge foo() void {}
forge foo() void {}
forge main() void {}`,
		"declared more than once",
	)
}

// TestStringPlusInt — adding a string and an int (invalid).
func TestStringPlusInt(t *testing.T) {
	expectError(t,
		`forge main() void { lowkey x string = "hi" + 1 }`,
		"two numbers or two strings",
	)
}
