// lexer_test.go : Unit tests for the Vine lexer.
//
// ─── How to run ───────────────────────────────────────────────────────────────
//   go test ./lexer/...          run just lexer tests
//   go test ./lexer/... -v       verbose output (see each test name)
//   go test ./...                run ALL package tests
//
// ─── What we test ─────────────────────────────────────────────────────────────
//   1. Individual tokens: each keyword, operator, literal type
//   2. Token positions: line and column numbers (crucial for error messages)
//   3. Edge cases: empty input, comments, unterminated strings
//   4. Full programs: ensure the lexer handles realistic Vine code
package lexer

import (
	"testing"
)

// tokenSpec describes one expected token in a test.
type tokenSpec struct {
	wantType    TokenType
	wantLiteral string
}

// ─── Helper ───────────────────────────────────────────────────────────────────

// checkTokens lexes `source` and asserts that the produced tokens match `want`.
// It ignores the final EOF token (that's always present).
func checkTokens(t *testing.T, source string, want []tokenSpec) {
	t.Helper()
	l := New(source)
	got, errs := l.Tokenize()

	if len(errs) > 0 {
		t.Errorf("unexpected lexer errors: %v", errs)
	}

	// Strip EOF from comparison
	realTokens := got
	if len(realTokens) > 0 && realTokens[len(realTokens)-1].Type == TOKEN_EOF {
		realTokens = realTokens[:len(realTokens)-1]
	}

	if len(realTokens) != len(want) {
		t.Fatalf("token count mismatch: got %d, want %d\n  got:  %v\n  want: %v",
			len(realTokens), len(want), realTokens, want)
	}

	for i, spec := range want {
		tok := realTokens[i]
		if tok.Type != spec.wantType {
			t.Errorf("token[%d] type: got %s, want %s (literal=%q)",
				i, tok.Type, spec.wantType, tok.Literal)
		}
		if tok.Literal != spec.wantLiteral {
			t.Errorf("token[%d] literal: got %q, want %q", i, tok.Literal, spec.wantLiteral)
		}
	}
}

// ─── Keyword Tests ────────────────────────────────────────────────────────────

// TestKeywords verifies that every Vine keyword produces the correct token type.
// This is important because keywords look like identifiers to the character scanner —
// LookupIdent() is what distinguishes them.
func TestKeywords(t *testing.T) {
	tests := []struct {
		source  string
		want    TokenType
	}{
		{"lowkey",       TOKEN_VAR},
		{"noCap",        TOKEN_CONST},
		{"lockIn",       TOKEN_CONST},
		{"forge",        TOKEN_FUNC},
		{"itIsWhatItIs", TOKEN_RETURN},
		{"spill",        TOKEN_PRINT},
		{"perchance",    TOKEN_IF},
		{"otherwise",    TOKEN_ELSE},
		{"letHimCook",   TOKEN_WHILE},
		{"spinTheBlock", TOKEN_FOR},
		{"ghost",        TOKEN_BREAK},
		{"keepItMoving", TOKEN_CONTINUE},
		{"bet",          TOKEN_TRUE},
		{"nah",          TOKEN_FALSE},
		{"ghosted",      TOKEN_NULL},
		{"checkTheFit",  TOKEN_MATCH},
		{"style",        TOKEN_CASE},
		{"noFilter",     TOKEN_DEFAULT},
		{"throwHands",   TOKEN_THROW},
		{"attempt",      TOKEN_TRY},
		{"catch",        TOKEN_CATCH},
		{"int",          TOKEN_INT_TYPE},
		{"float",        TOKEN_FLOAT_TYPE},
		{"string",       TOKEN_STRING_TYPE},
		{"bool",         TOKEN_BOOL_TYPE},
		{"void",         TOKEN_VOID_TYPE},
		{"isGiving",     TOKEN_EQ},
	}

	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			l := New(tt.source)
			tokens, _ := l.Tokenize()
			if len(tokens) == 0 {
				t.Fatal("no tokens produced")
			}
			if tokens[0].Type != tt.want {
				t.Errorf("keyword %q: got token type %s, want %s",
					tt.source, tokens[0].Type, tt.want)
			}
		})
	}
}

// TestIdentifier checks that non-keyword words produce TOKEN_IDENT.
func TestIdentifier(t *testing.T) {
	tests := []string{"x", "myVar", "count_1", "CamelCase", "_private"}
	for _, ident := range tests {
		t.Run(ident, func(t *testing.T) {
			l := New(ident)
			tokens, _ := l.Tokenize()
			if tokens[0].Type != TOKEN_IDENT {
				t.Errorf("%q: got %s, want IDENT", ident, tokens[0].Type)
			}
			if tokens[0].Literal != ident {
				t.Errorf("%q: literal mismatch: got %q", ident, tokens[0].Literal)
			}
		})
	}
}

// ─── Literal Tests ────────────────────────────────────────────────────────────

// TestIntegerLiterals checks integer scanning.
func TestIntegerLiterals(t *testing.T) {
	tests := []struct{ input, literal string }{
		{"0", "0"},
		{"42", "42"},
		{"1000000", "1000000"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			checkTokens(t, tt.input, []tokenSpec{{TOKEN_INT_LIT, tt.literal}})
		})
	}
}

// TestFloatLiterals checks that  3.14  and  0.5  produce TOKEN_FLOAT_LIT.
func TestFloatLiterals(t *testing.T) {
	tests := []string{"3.14", "0.5", "100.0", "1.23456"}
	for _, src := range tests {
		t.Run(src, func(t *testing.T) {
			checkTokens(t, src, []tokenSpec{{TOKEN_FLOAT_LIT, src}})
		})
	}
}

// TestStringLiterals checks that double-quoted strings scan correctly.
func TestStringLiterals(t *testing.T) {
	tests := []struct {
		source  string
		literal string
	}{
		{`"hello"`,         "hello"},
		{`"hello world"`,   "hello world"},
		{`""`,              ""},
		{`"tab\there"`,     "tab\there"},
		{`"new\nline"`,     "new\nline"},
		{`"quote\"inside"`, `quote"inside`},
	}
	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			checkTokens(t, tt.source, []tokenSpec{{TOKEN_STRING_LIT, tt.literal}})
		})
	}
}

// TestBoolLiterals checks bet and nah.
func TestBoolLiterals(t *testing.T) {
	checkTokens(t, "bet", []tokenSpec{{TOKEN_TRUE, "bet"}})
	checkTokens(t, "nah", []tokenSpec{{TOKEN_FALSE, "nah"}})
}

// ─── Operator Tests ───────────────────────────────────────────────────────────

// TestOperators verifies every operator token.
func TestOperators(t *testing.T) {
	tests := []struct {
		source string
		want   TokenType
	}{
		{"+",  TOKEN_PLUS},
		{"-",  TOKEN_MINUS},
		{"*",  TOKEN_STAR},
		{"/",  TOKEN_SLASH},
		{"%",  TOKEN_PERCENT},
		{"==", TOKEN_EQ},
		{"!=", TOKEN_NEQ},
		{"<",  TOKEN_LT},
		{">",  TOKEN_GT},
		{"<=", TOKEN_LEQ},
		{">=", TOKEN_GEQ},
		{"=",  TOKEN_ASSIGN},
		{"=>", TOKEN_ARROW},
		{"&&", TOKEN_AND},
		{"||", TOKEN_OR},
		{"!",  TOKEN_NOT},
	}
	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			l := New(tt.source)
			tokens, _ := l.Tokenize()
			if tokens[0].Type != tt.want {
				t.Errorf("operator %q: got %s, want %s", tt.source, tokens[0].Type, tt.want)
			}
		})
	}
}

// ─── Comment Tests ────────────────────────────────────────────────────────────

// TestLineComments ensures that // comments produce NO tokens.
func TestLineComments(t *testing.T) {
	source := `// this whole line is a comment
42`
	checkTokens(t, source, []tokenSpec{{TOKEN_INT_LIT, "42"}})
}

// TestInlineComment checks that comments at the end of a line are stripped.
func TestInlineComment(t *testing.T) {
	source := `lowkey x int = 5 // declare x`
	checkTokens(t, source, []tokenSpec{
		{TOKEN_VAR,      "lowkey"},
		{TOKEN_IDENT,    "x"},
		{TOKEN_INT_TYPE, "int"},
		{TOKEN_ASSIGN,   "="},
		{TOKEN_INT_LIT,  "5"},
	})
}

// ─── Line/Column Tracking ─────────────────────────────────────────────────────

// TestLineNumbers verifies that multi-line source produces correct line numbers.
// Accurate line numbers are essential for helpful error messages.
func TestLineNumbers(t *testing.T) {
	source := "lowkey\nx\nint"
	l := New(source)
	tokens, _ := l.Tokenize()

	if tokens[0].Line != 1 { t.Errorf("token 0 line: got %d, want 1", tokens[0].Line) }
	if tokens[1].Line != 2 { t.Errorf("token 1 line: got %d, want 2", tokens[1].Line) }
	if tokens[2].Line != 3 { t.Errorf("token 2 line: got %d, want 3", tokens[2].Line) }
}

// ─── Error Cases ──────────────────────────────────────────────────────────────

// TestUnterminatedString checks that a missing closing " produces an error.
func TestUnterminatedString(t *testing.T) {
	l := New(`"hello`)
	_, errs := l.Tokenize()
	if len(errs) == 0 {
		t.Error("expected error for unterminated string, got none")
	}
}

// TestIllegalCharacter checks that an unknown character produces TOKEN_ILLEGAL and an error.
func TestIllegalCharacter(t *testing.T) {
	l := New("@")
	tokens, errs := l.Tokenize()
	if len(errs) == 0 {
		t.Error("expected error for illegal character '@'")
	}
	if tokens[0].Type != TOKEN_ILLEGAL {
		t.Errorf("expected TOKEN_ILLEGAL, got %s", tokens[0].Type)
	}
}

// TestEmptyInput checks that scanning empty source produces only EOF.
func TestEmptyInput(t *testing.T) {
	l := New("")
	tokens, errs := l.Tokenize()
	if len(errs) > 0 {
		t.Errorf("unexpected errors on empty input: %v", errs)
	}
	if len(tokens) != 1 || tokens[0].Type != TOKEN_EOF {
		t.Errorf("expected single EOF token, got %v", tokens)
	}
}

// ─── Full Program Test ────────────────────────────────────────────────────────

// TestFullProgram tokenises a small but complete Vine program.
// This is an integration test for the lexer — it exercises many paths at once.
func TestFullProgram(t *testing.T) {
	source := `forge add(a int, b int) int {
    itIsWhatItIs a + b
}`
	expected := []tokenSpec{
		{TOKEN_FUNC,     "forge"},
		{TOKEN_IDENT,    "add"},
		{TOKEN_LPAREN,   "("},
		{TOKEN_IDENT,    "a"},
		{TOKEN_INT_TYPE, "int"},
		{TOKEN_COMMA,    ","},
		{TOKEN_IDENT,    "b"},
		{TOKEN_INT_TYPE, "int"},
		{TOKEN_RPAREN,   ")"},
		{TOKEN_INT_TYPE, "int"},
		{TOKEN_LBRACE,   "{"},
		{TOKEN_RETURN,   "itIsWhatItIs"},
		{TOKEN_IDENT,    "a"},
		{TOKEN_PLUS,     "+"},
		{TOKEN_IDENT,    "b"},
		{TOKEN_RBRACE,   "}"},
	}
	checkTokens(t, source, expected)
}
