// Package lexer implements the LEXICAL ANALYSIS phase of the Vine compiler.
//
// ─── What is Lexical Analysis? ────────────────────────────────────────────────
// The lexer reads raw source code and groups characters into TOKENS —
// the smallest meaningful units of the language (like words in a sentence).
//
// Vine uses internet-culture keywords to make the language fun:
//
//   lowkey x int = 42          → var x int = 42
//   forge add(a int, b int) int → func add(a int, b int) int
//   perchance (x isGiving 0)   → if (x == 0)
//   letHimCook (i < 10)        → while (i < 10)
//   itIsWhatItIs x             → return x
//   spill("hello")             → print("hello")
package lexer

import "fmt"

// TokenType is an integer tag identifying what kind of token this is.
type TokenType int

const (
	// ── Literals ────────────────────────────────────────────────────────────
	TOKEN_INT_LIT    TokenType = iota // integer literal   e.g. 42
	TOKEN_FLOAT_LIT                   // float literal     e.g. 3.14
	TOKEN_STRING_LIT                  // string literal    e.g. "hello"
	TOKEN_TRUE                        // bet   (true)
	TOKEN_FALSE                       // nah   (false)

	// ── Identifiers ─────────────────────────────────────────────────────────
	TOKEN_IDENT // programmer-defined name e.g. x, myVar

	// ── Keywords ────────────────────────────────────────────────────────────
	TOKEN_VAR      // lowkey   — declare a mutable variable
	TOKEN_CONST    // noCap / lockIn — declare an immutable constant
	TOKEN_FUNC     // forge    — define a function
	TOKEN_RETURN   // itIsWhatItIs — return a value
	TOKEN_IF       // perchance — if statement
	TOKEN_ELSE     // otherwise — else branch
	TOKEN_WHILE    // letHimCook — while loop
	TOKEN_FOR      // spinTheBlock — for loop
	TOKEN_BREAK    // ghost    — break out of a loop
	TOKEN_CONTINUE // keepItMoving — continue to next iteration
	TOKEN_PRINT    // spill    — print to stdout
	TOKEN_MATCH    // checkTheFit — switch/match statement
	TOKEN_CASE     // style    — a case inside checkTheFit
	TOKEN_DEFAULT  // noFilter — default case
	TOKEN_NEW      // summon   — instantiate a blueprint
	TOKEN_CLASS    // blueprint — class definition
	TOKEN_THIS     // theVibe  — reference to current instance
	TOKEN_EXTENDS  // evolvedFrom — inheritance
	TOKEN_NULL     // ghosted  — null / no value
	TOKEN_TRY      // attempt  — try block
	TOKEN_CATCH    // catch    — catch block
	TOKEN_THROW    // throwHands — throw an error

	// ── Type Keywords ────────────────────────────────────────────────────────
	TOKEN_INT_TYPE    // int
	TOKEN_FLOAT_TYPE  // float
	TOKEN_STRING_TYPE // string
	TOKEN_BOOL_TYPE   // bool
	TOKEN_VOID_TYPE   // void

	// ── Arithmetic Operators ─────────────────────────────────────────────────
	TOKEN_PLUS    // +
	TOKEN_MINUS   // -
	TOKEN_STAR    // *
	TOKEN_SLASH   // /
	TOKEN_PERCENT // %

	// ── Comparison Operators ─────────────────────────────────────────────────
	TOKEN_EQ  // == or isGiving
	TOKEN_NEQ // !=
	TOKEN_LT  // <
	TOKEN_GT  // >
	TOKEN_LEQ // <=
	TOKEN_GEQ // >=

	// ── Assignment & Arrow ───────────────────────────────────────────────────
	TOKEN_ASSIGN // =
	TOKEN_ARROW  // => (for lambdas / mood)

	// ── Logical Operators ────────────────────────────────────────────────────
	TOKEN_AND // &&
	TOKEN_OR  // ||
	TOKEN_NOT // !

	// ── Delimiters ───────────────────────────────────────────────────────────
	TOKEN_LPAREN    // (
	TOKEN_RPAREN    // )
	TOKEN_LBRACE    // {
	TOKEN_RBRACE    // }
	TOKEN_LBRACKET  // [
	TOKEN_RBRACKET  // ]
	TOKEN_COMMA     // ,
	TOKEN_DOT       // .
	TOKEN_COLON     // :
	TOKEN_SEMICOLON // ;

	// ── Special ──────────────────────────────────────────────────────────────
	TOKEN_EOF     // end of file
	TOKEN_ILLEGAL // unrecognised character
)

// keywords maps every reserved word to its TokenType.
// Multi-word keywords (like "itIsWhatItIs") are stored as single entries.
var keywords = map[string]TokenType{
	// ── Core statements ──────────────────────────────────────────────────────
	"lowkey":       TOKEN_VAR,     // mutable variable declaration
	"noCap":        TOKEN_CONST,   // immutable constant (alias: lockIn)
	"lockIn":       TOKEN_CONST,   // immutable constant (alias: noCap)
	"forge":        TOKEN_FUNC,    // function definition
	"itIsWhatItIs": TOKEN_RETURN,  // return statement
	"spill":        TOKEN_PRINT,   // print/output
	"ghost":        TOKEN_BREAK,   // break out of loop
	"keepItMoving": TOKEN_CONTINUE,// continue loop

	// ── Control flow ─────────────────────────────────────────────────────────
	"perchance":    TOKEN_IF,      // if
	"otherwise":    TOKEN_ELSE,    // else
	"letHimCook":   TOKEN_WHILE,   // while loop
	"spinTheBlock": TOKEN_FOR,     // for loop

	// ── Pattern matching ─────────────────────────────────────────────────────
	"checkTheFit":  TOKEN_MATCH,   // switch/match
	"style":        TOKEN_CASE,    // case
	"noFilter":     TOKEN_DEFAULT, // default case

	// ── Boolean literals ─────────────────────────────────────────────────────
	"bet":          TOKEN_TRUE,    // true
	"nah":          TOKEN_FALSE,   // false
	"ghosted":      TOKEN_NULL,    // null / none

	// ── OOP ──────────────────────────────────────────────────────────────────
	"blueprint":    TOKEN_CLASS,   // class definition
	"summon":       TOKEN_NEW,     // new / instantiate
	"theVibe":      TOKEN_THIS,    // this / self
	"evolvedFrom":  TOKEN_EXTENDS, // extends / inherits

	// ── Error handling ────────────────────────────────────────────────────────
	"attempt":      TOKEN_TRY,     // try
	"catch":        TOKEN_CATCH,   // catch
	"throwHands":   TOKEN_THROW,   // throw

	// ── Comparison operator keyword ───────────────────────────────────────────
	// 'isGiving' is the word form of ==
	// e.g.  x isGiving 5   means   x == 5
	"isGiving":     TOKEN_EQ,

	// ── Types ────────────────────────────────────────────────────────────────
	"int":    TOKEN_INT_TYPE,
	"float":  TOKEN_FLOAT_TYPE,
	"string": TOKEN_STRING_TYPE,
	"bool":   TOKEN_BOOL_TYPE,
	"void":   TOKEN_VOID_TYPE,
}

// LookupIdent returns TOKEN_IDENT for user-defined names,
// or the keyword's TokenType if the word is reserved.
func LookupIdent(ident string) TokenType {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return TOKEN_IDENT
}

// Token is one atomic unit of source code produced by the lexer.
type Token struct {
	Type    TokenType
	Literal string // the exact text from the source
	Line    int
	Column  int
}

func (t Token) String() string {
	return fmt.Sprintf("Token{%-14s %-30q line:%d col:%d}",
		t.Type.String(), t.Literal, t.Line, t.Column)
}

// String returns the display name for a TokenType.
func (tt TokenType) String() string {
	names := map[TokenType]string{
		TOKEN_INT_LIT:    "INT_LIT",
		TOKEN_FLOAT_LIT:  "FLOAT_LIT",
		TOKEN_STRING_LIT: "STRING_LIT",
		TOKEN_TRUE:       "bet",
		TOKEN_FALSE:      "nah",
		TOKEN_IDENT:      "IDENT",
		TOKEN_VAR:        "lowkey",
		TOKEN_CONST:      "noCap",
		TOKEN_FUNC:       "forge",
		TOKEN_RETURN:     "itIsWhatItIs",
		TOKEN_IF:         "perchance",
		TOKEN_ELSE:       "otherwise",
		TOKEN_WHILE:      "letHimCook",
		TOKEN_FOR:        "spinTheBlock",
		TOKEN_BREAK:      "ghost",
		TOKEN_CONTINUE:   "keepItMoving",
		TOKEN_PRINT:      "spill",
		TOKEN_MATCH:      "checkTheFit",
		TOKEN_CASE:       "style",
		TOKEN_DEFAULT:    "noFilter",
		TOKEN_NEW:        "summon",
		TOKEN_CLASS:      "blueprint",
		TOKEN_THIS:       "theVibe",
		TOKEN_EXTENDS:    "evolvedFrom",
		TOKEN_NULL:       "ghosted",
		TOKEN_TRY:        "attempt",
		TOKEN_CATCH:      "catch",
		TOKEN_THROW:      "throwHands",
		TOKEN_INT_TYPE:   "int",
		TOKEN_FLOAT_TYPE: "float",
		TOKEN_STRING_TYPE:"string",
		TOKEN_BOOL_TYPE:  "bool",
		TOKEN_VOID_TYPE:  "void",
		TOKEN_PLUS:       "+",
		TOKEN_MINUS:      "-",
		TOKEN_STAR:       "*",
		TOKEN_SLASH:      "/",
		TOKEN_PERCENT:    "%",
		TOKEN_EQ:         "isGiving(==)",
		TOKEN_NEQ:        "!=",
		TOKEN_LT:         "<",
		TOKEN_GT:         ">",
		TOKEN_LEQ:        "<=",
		TOKEN_GEQ:        ">=",
		TOKEN_ASSIGN:     "=",
		TOKEN_ARROW:      "=>",
		TOKEN_AND:        "&&",
		TOKEN_OR:         "||",
		TOKEN_NOT:        "!",
		TOKEN_LPAREN:     "(",
		TOKEN_RPAREN:     ")",
		TOKEN_LBRACE:     "{",
		TOKEN_RBRACE:     "}",
		TOKEN_LBRACKET:   "[",
		TOKEN_RBRACKET:   "]",
		TOKEN_COMMA:      ",",
		TOKEN_DOT:        ".",
		TOKEN_COLON:      ":",
		TOKEN_SEMICOLON:  ";",
		TOKEN_EOF:        "EOF",
		TOKEN_ILLEGAL:    "ILLEGAL",
	}
	if name, ok := names[tt]; ok {
		return name
	}
	return fmt.Sprintf("TOKEN(%d)", int(tt))
}
