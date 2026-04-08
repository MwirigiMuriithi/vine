// Lexer converts Vine source code (a stream of characters) into a stream of tokens.
//
// ─── How It Works ────────────────────────────────────────────────────────────
// The lexer is a state machine. It reads one character at a time, and based on
// what it sees, decides which token to produce.
//
// Special note on multi-word keywords:
//   Some Vine keywords contain spaces conceptually, but are written as one
//   camelCase identifier (e.g. "itIsWhatItIs", "letHimCook", "spinTheBlock").
//   The lexer treats them exactly like normal identifiers — they are just
//   looked up in the keyword table and produce their keyword token type.
//
// ─── Comment Syntax ──────────────────────────────────────────────────────────
//   // single-line comment (dropped entirely, never reaches the parser)
package lexer

import (
	"fmt"
	"strings"
)

// Lexer holds all state for the scanning process.
type Lexer struct {
	source string
	pos    int
	line   int
	column int
	tokens []Token
	errors []string
}

// New creates a fresh Lexer ready to scan the given source code.
func New(source string) *Lexer {
	return &Lexer{source: source, pos: 0, line: 1, column: 1}
}

// Tokenize scans the entire source and returns:
//   - All tokens (ending with TOKEN_EOF)
//   - Any error messages (lexer continues on error for better diagnostics)
func (l *Lexer) Tokenize() ([]Token, []string) {
	for !l.isAtEnd() {
		l.scanToken()
	}
	l.emit(TOKEN_EOF, "")
	return l.tokens, l.errors
}

// scanToken reads the next token from the current position.
func (l *Lexer) scanToken() {
	startLine := l.line
	startCol  := l.column
	ch        := l.advance()

	switch ch {
	// ── Single-character tokens ──────────────────────────────────────────
	case '(':  l.emitAt(TOKEN_LPAREN,   "(", startLine, startCol)
	case ')':  l.emitAt(TOKEN_RPAREN,   ")", startLine, startCol)
	case '{':  l.emitAt(TOKEN_LBRACE,   "{", startLine, startCol)
	case '}':  l.emitAt(TOKEN_RBRACE,   "}", startLine, startCol)
	case '[':  l.emitAt(TOKEN_LBRACKET, "[", startLine, startCol)
	case ']':  l.emitAt(TOKEN_RBRACKET, "]", startLine, startCol)
	case ',':  l.emitAt(TOKEN_COMMA,    ",", startLine, startCol)
	case '.':  l.emitAt(TOKEN_DOT,      ".", startLine, startCol)
	case ':':  l.emitAt(TOKEN_COLON,    ":", startLine, startCol)
	case ';':  l.emitAt(TOKEN_SEMICOLON,";", startLine, startCol)
	case '%':  l.emitAt(TOKEN_PERCENT,  "%", startLine, startCol)
	case '+':  l.emitAt(TOKEN_PLUS,     "+", startLine, startCol)
	case '*':  l.emitAt(TOKEN_STAR,     "*", startLine, startCol)
	case '-':  l.emitAt(TOKEN_MINUS,    "-", startLine, startCol)

	// ── One-or-two-character tokens ──────────────────────────────────────
	case '/':
		if l.peek() == '/' {
			// Line comment — consume until end of line, discard entirely
			for !l.isAtEnd() && l.peek() != '\n' {
				l.advance()
			}
		} else {
			l.emitAt(TOKEN_SLASH, "/", startLine, startCol)
		}

	case '=':
		if l.match('=') {
			l.emitAt(TOKEN_EQ, "==", startLine, startCol)
		} else if l.match('>') {
			l.emitAt(TOKEN_ARROW, "=>", startLine, startCol) // lambda arrow
		} else {
			l.emitAt(TOKEN_ASSIGN, "=", startLine, startCol)
		}

	case '!':
		if l.match('=') {
			l.emitAt(TOKEN_NEQ, "!=", startLine, startCol)
		} else {
			l.emitAt(TOKEN_NOT, "!", startLine, startCol)
		}

	case '<':
		if l.match('=') {
			l.emitAt(TOKEN_LEQ, "<=", startLine, startCol)
		} else {
			l.emitAt(TOKEN_LT, "<", startLine, startCol)
		}

	case '>':
		if l.match('=') {
			l.emitAt(TOKEN_GEQ, ">=", startLine, startCol)
		} else {
			l.emitAt(TOKEN_GT, ">", startLine, startCol)
		}

	case '&':
		if l.match('&') {
			l.emitAt(TOKEN_AND, "&&", startLine, startCol)
		} else {
			l.addError(startLine, startCol, "expected '&&', got single '&'")
		}

	case '|':
		if l.match('|') {
			l.emitAt(TOKEN_OR, "||", startLine, startCol)
		} else {
			l.addError(startLine, startCol, "expected '||', got single '|'")
		}

	// ── Whitespace ───────────────────────────────────────────────────────
	case ' ', '\t', '\r':
		// insignificant — skip silently

	case '\n':
		l.line++
		l.column = 1

	// ── String Literals ──────────────────────────────────────────────────
	case '"':
		l.scanString(startLine, startCol)

	default:
		switch {
		case isDigit(ch):
			l.scanNumber(ch, startLine, startCol)
		case isAlpha(ch):
			l.scanIdent(ch, startLine, startCol)
		default:
			l.addError(startLine, startCol, fmt.Sprintf("unexpected character %q", ch))
			l.emitAt(TOKEN_ILLEGAL, string(ch), startLine, startCol)
		}
	}
}

// scanString scans a double-quoted string literal, processing escape sequences.
func (l *Lexer) scanString(startLine, startCol int) {
	var sb strings.Builder
	for !l.isAtEnd() && l.peek() != '"' {
		ch := l.advance()
		if ch == '\n' {
			l.addError(startLine, startCol, "unterminated string (newline inside string)")
			return
		}
		if ch == '\\' {
			if l.isAtEnd() { break }
			esc := l.advance()
			switch esc {
			case '"':  sb.WriteByte('"')
			case '\\': sb.WriteByte('\\')
			case 'n':  sb.WriteByte('\n')
			case 't':  sb.WriteByte('\t')
			default:
				sb.WriteByte('\\')
				sb.WriteByte(esc)
			}
		} else {
			sb.WriteByte(ch)
		}
	}
	if l.isAtEnd() {
		l.addError(startLine, startCol, "unterminated string literal")
		return
	}
	l.advance() // consume closing "
	l.emitAt(TOKEN_STRING_LIT, sb.String(), startLine, startCol)
}

// scanNumber scans an integer or float literal.
// `first` is the digit already consumed.
func (l *Lexer) scanNumber(first byte, startLine, startCol int) {
	var sb strings.Builder
	sb.WriteByte(first)
	isFloat := false

	for !l.isAtEnd() && isDigit(l.peek()) {
		sb.WriteByte(l.advance())
	}
	// Decimal point → float
	if !l.isAtEnd() && l.peek() == '.' && isDigit(l.peekNext()) {
		isFloat = true
		sb.WriteByte(l.advance()) // consume '.'
		for !l.isAtEnd() && isDigit(l.peek()) {
			sb.WriteByte(l.advance())
		}
	}
	if isFloat {
		l.emitAt(TOKEN_FLOAT_LIT, sb.String(), startLine, startCol)
	} else {
		l.emitAt(TOKEN_INT_LIT, sb.String(), startLine, startCol)
	}
}

// scanIdent scans an identifier or keyword.
// `first` is the letter/underscore already consumed.
// After scanning the full word, LookupIdent decides if it's a keyword.
func (l *Lexer) scanIdent(first byte, startLine, startCol int) {
	var sb strings.Builder
	sb.WriteByte(first)
	for !l.isAtEnd() && isAlphaNumeric(l.peek()) {
		sb.WriteByte(l.advance())
	}
	word    := sb.String()
	tokType := LookupIdent(word)
	l.emitAt(tokType, word, startLine, startCol)
}

// ─── Low-level helpers ────────────────────────────────────────────────────────

func (l *Lexer) advance() byte {
	ch := l.source[l.pos]
	l.pos++
	l.column++
	return ch
}

func (l *Lexer) peek() byte {
	if l.isAtEnd() { return 0 }
	return l.source[l.pos]
}

func (l *Lexer) peekNext() byte {
	if l.pos+1 >= len(l.source) { return 0 }
	return l.source[l.pos+1]
}

func (l *Lexer) match(expected byte) bool {
	if l.isAtEnd() || l.source[l.pos] != expected { return false }
	l.pos++
	l.column++
	return true
}

func (l *Lexer) isAtEnd() bool { return l.pos >= len(l.source) }

func (l *Lexer) emit(tokType TokenType, literal string) {
	l.tokens = append(l.tokens, Token{Type: tokType, Literal: literal, Line: l.line, Column: l.column})
}

func (l *Lexer) emitAt(tokType TokenType, literal string, line, col int) {
	l.tokens = append(l.tokens, Token{Type: tokType, Literal: literal, Line: line, Column: col})
}

func (l *Lexer) addError(line, col int, msg string) {
	l.errors = append(l.errors, fmt.Sprintf("lexer error at line %d, col %d: %s", line, col, msg))
}

func isDigit(ch byte) bool        { return ch >= '0' && ch <= '9' }
func isAlpha(ch byte) bool        { return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_' }
func isAlphaNumeric(ch byte) bool { return isAlpha(ch) || isDigit(ch) }
