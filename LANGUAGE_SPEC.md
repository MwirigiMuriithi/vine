# The Vine Programming Language Specification
## Version 0.1.0

---

## 1. Overview

Vine is a statically-typed, procedural programming language designed for
learning compiler design & construction.<br>
It uses internet-culture gen-z keywords to make
the syntax memorable and fun.

**Design goals:**
- Simple enough to learn the course
- Expressive enough to write real algorithms
- Every keyword is memorable and maps to a standard concept

---

## 2. Lexical Structure

### 2.1 Source File Encoding
Vine source files are UTF-8 text with the `.vine` extension.

### 2.2 Comments
Only line comments are supported:
```vine
// This is a comment
lowkey x int = 5   // inline comment
```

### 2.3 Keywords

| Vine Keyword     | Meaning                  | Category       |
|-----------------|--------------------------|----------------|
| `lowkey`         | mutable variable         | Declaration    |
| `noCap`          | immutable constant       | Declaration    |
| `lockIn`         | same as noCap            | Declaration    |
| `forge`          | function definition      | Declaration    |
| `itIsWhatItIs`   | return statement         | Control Flow   |
| `spill`          | print to stdout          | I/O            |
| `perchance`      | if statement             | Control Flow   |
| `otherwise`      | else branch              | Control Flow   |
| `letHimCook`     | while loop               | Control Flow   |
| `spinTheBlock`   | for loop                 | Control Flow   |
| `ghost`          | break out of loop        | Control Flow   |
| `keepItMoving`   | continue to next iter    | Control Flow   |
| `checkTheFit`    | match / switch           | Control Flow   |
| `style`          | case arm                 | Control Flow   |
| `noFilter`       | default case             | Control Flow   |
| `attempt`        | try block                | Error Handling |
| `catch`          | catch block              | Error Handling |
| `throwHands`     | throw an error           | Error Handling |
| `bet`            | boolean true             | Literal        |
| `nah`            | boolean false            | Literal        |
| `ghosted`        | null value               | Literal        |
| `isGiving`       | equality operator (==)   | Operator       |
| `int`            | integer type             | Type           |
| `float`          | float type               | Type           |
| `string`         | string type              | Type           |
| `bool`           | boolean type             | Type           |
| `void`           | no-return type           | Type           |

### 2.4 Identifiers
An identifier starts with a letter (`a-z`, `A-Z`) or underscore `_`,
followed by any combination of letters, digits, or underscores.

```
identifier = (letter | '_') (letter | digit | '_')*
```

Keywords are reserved and cannot be used as identifiers.

### 2.5 Literals

**Integer:**  `42`, `0`, `1000000`

**Float:**  `3.14`, `0.5`, `100.0`  
  A float must have at least one digit on each side of the decimal point.

**String:**  `"hello world"`  
  Supported escape sequences: `\"`, `\\`, `\n`, `\t`

**Boolean:**  `bet` (true) or `nah` (false)

**Null:**  `ghosted`

---

## 3. Types

Vine is **statically typed**, every variable has a type, determined at
declaration time.

| Type     | Description               | Example Literal |
|----------|---------------------------|-----------------|
| `int`    | 64-bit signed integer     | `42`, `-7`      |
| `float`  | 64-bit IEEE 754 double    | `3.14`, `0.5`   |
| `string` | UTF-8 character sequence  | `"hello"`       |
| `bool`   | boolean value             | `bet`, `nah`    |
| `void`   | no value (functions only) | —               |

**Type compatibility:**
- An `int` value may be used where `float` is expected (implicit widening).
- No other implicit conversions exist.

---

## 4. Declarations

### 4.1 Mutable Variables: `lowkey`
```vine
lowkey <name> <type> = <expression>
```
A `lowkey` variable can be reassigned after declaration.

```vine
lowkey count int = 0
count = count + 1   // ✓ allowed
```

### 4.2 Immutable Constants:`noCap` / `lockIn`
```vine
noCap <name> <type> = <expression>
lockIn <name> <type> = <expression>   // same as noCap
```
A `noCap` / `lockIn` binding cannot be reassigned. Attempting to do so
is a **semantic error**.

```vine
noCap MAX int = 100
MAX = 200   // ✗ semantic error: cannot reassign noCap, it's locked in fr
```

### 4.3 Functions: `forge`
```vine
forge <name>(<params>) <returnType> {
    <body>
}
```

- **Parameters:** zero or more `name type` pairs, comma-separated.
- **Return type:** any type keyword, or `void` if the function returns nothing.
- **Body:** a block of statements.
- Functions can call themselves (**recursion**) and call each other in any order
  (the semantic analyser uses a two-pass approach).

```vine
forge add(a int, b int) int {
    itIsWhatItIs a + b
}

forge greet(name string) void {
    spill("Hello, " + name + "!")
}
```

---

## 5. Statements

### 5.1 Print: `spill`
```vine
spill(<expression>)
```
Prints the expression to standard output followed by a newline.
Any type can be printed.

### 5.2 Assignment
```vine
<name> = <expression>
```
Reassigns an existing variable. The variable must have been declared with
`lowkey` (not `noCap`/`lockIn`).

### 5.3 Return: `itIsWhatItIs`
```vine
itIsWhatItIs <expression>   // return with a value
itIsWhatItIs                // void return
```

### 5.4 If Statement: `perchance` / `otherwise`
```vine
perchance (<condition>) {
    <then-body>
} otherwise {
    <else-body>
}

// else-if chain:
perchance (<cond1>) {
    ...
} otherwise perchance (<cond2>) {
    ...
} otherwise {
    ...
}
```
The condition **must** be of type `bool`.

### 5.5 While Loop: `letHimCook`
```vine
letHimCook (<condition>) {
    <body>
}
```
Executes the body repeatedly while the condition is `bet` (true).

### 5.6 For Loop: `spinTheBlock`
```vine
spinTheBlock (<init>; <condition>; <post>) {
    <body>
}
```
- `init`: a variable declaration (`lowkey`) or assignment
- `condition`: a `bool` expression
- `post`: an assignment statement (the loop update step)

```vine
spinTheBlock (lowkey i int = 0; i < 10; i = i + 1) {
    spill(i)
}
```

### 5.7 Break: `ghost`
```vine
ghost
```
Exits the **innermost** `letHimCook` or `spinTheBlock` loop immediately.
Using `ghost` outside a loop is a **semantic error**.

### 5.8 Continue: `keepItMoving`
```vine
keepItMoving
```
Skips to the next iteration of the innermost loop.

### 5.9 Match / Switch: `checkTheFit`
```vine
checkTheFit (<expression>) {
    style <value1>: {
        <body1>
    }
    style <value2>: {
        <body2>
    }
    noFilter: {
        <default-body>
    }
}
```
Evaluates the subject expression, then checks each `style` arm for equality.
The first matching arm executes. `noFilter` is the default if nothing matches.

### 5.10 Error Handling: `attempt` / `catch` / `throwHands`
```vine
// Throw an error:
throwHands(<message-expression>)

// Handle errors:
attempt {
    <try-body>
} catch <errorVar> {
    <catch-body>
}
```
- `throwHands` stops normal execution and unwinds to the nearest `attempt` block.
- The caught error message is bound as a `string` to `<errorVar>`.
- If no `attempt` block catches the error, the program exits with an error message.
- The VM also throws automatically on division by zero.

---

## 6. Expressions

### 6.1 Operator Precedence (lowest to highest)
```
||                          logical OR
&&                          logical AND
== isGiving !=              equality
< > <= >=                   comparison
+ -                         addition / subtraction
* / %                       multiplication / division / modulo
- !                         unary negation / logical NOT
f(args)  (expr)  literal    function call / grouped / primary
```

### 6.2 Arithmetic Operators
| Operator | Types          | Result |
|----------|---------------|--------|
| `+`      | int, float    | numeric addition |
| `+`      | string, string | string concatenation |
| `-`      | int, float    | subtraction |
| `*`      | int, float    | multiplication |
| `/`      | int, float    | division (integer truncates) |
| `%`      | int           | modulo (remainder) |
| `-x`     | int, float    | unary negation |

If one operand is `float` and the other is `int`, the result is `float`.

### 6.3 Comparison Operators
All return `bool`.

| Operator   | Meaning       |
|------------|---------------|
| `==` / `isGiving` | equal to |
| `!=`       | not equal to  |
| `<`        | less than     |
| `>`        | greater than  |
| `<=`       | less than or equal to |
| `>=`       | greater than or equal to |

### 6.4 Logical Operators
Operands must be `bool`. Result is `bool`.

| Operator | Meaning     |
|----------|-------------|
| `&&`     | logical AND |
| `\|\|`   | logical OR  |
| `!`      | logical NOT (unary) |

### 6.5 Function Calls
```vine
<name>(<arg1>, <arg2>, ...)
```
Arguments are evaluated left-to-right before the function is called.

---

## 7. Scoping Rules

Vine uses **lexical scoping** (also called "static scoping"):
- Each block `{ }` creates a new scope.
- Names are looked up from the innermost scope outward.
- A name in an inner scope can shadow the same name in an outer scope.

```vine
lowkey x int = 10
perchance (bet) {
    lowkey x int = 20   // shadows the outer x
    spill(x)            // prints 20
}
spill(x)                // prints 10
```

---

## 8. Semantics Summary

| Rule | Description |
|------|-------------|
| **Type safety** | Every expression has a statically-determined type |
| **Immutability** | `noCap`/`lockIn` variables cannot be reassigned |
| **Loop control** | `ghost`/`keepItMoving` only valid inside `letHimCook`/`spinTheBlock` |
| **Return** | `itIsWhatItIs` type must match the forge's declared return type |
| **Declaration before use** | Variables must be declared before they are referenced |
| **Two-pass functions** | All `forge` declarations are visible everywhere in the program |
| **Int→Float coercion** | An `int` may be implicitly used where `float` is expected |

---

## 9. Grammar (Formal BNF)

```
program        = funcDecl*

funcDecl       = "forge" IDENT "(" params ")" type? block

params         = ( IDENT type ("," IDENT type)* )?
type           = "int" | "float" | "string" | "bool" | "void"

block          = "{" statement* "}"

statement      = varDecl
               | constDecl
               | assign
               | ifStmt
               | whileStmt
               | forStmt
               | matchStmt
               | returnStmt
               | printStmt
               | breakStmt
               | continueStmt
               | throwStmt
               | tryCatch
               | exprStmt

varDecl        = "lowkey" IDENT type "=" expr
constDecl      = ("noCap" | "lockIn") IDENT type "=" expr
assign         = IDENT "=" expr
ifStmt         = "perchance" expr block ("otherwise" (block | ifStmt))?
whileStmt      = "letHimCook" expr block
forStmt        = "spinTheBlock" "(" (varDecl | assign) ";" expr ";" assign ")" block
matchStmt      = "checkTheFit" "(" expr ")" "{" caseArm* "}"
caseArm        = ("style" expr | "noFilter") ":" block
returnStmt     = "itIsWhatItIs" expr?
printStmt      = "spill" "(" expr ")"
breakStmt      = "ghost"
continueStmt   = "keepItMoving"
throwStmt      = "throwHands" "(" expr ")"
tryCatch       = "attempt" block "catch" IDENT block
exprStmt       = expr

expr           = logicOr
logicOr        = logicAnd ("||" logicAnd)*
logicAnd       = equality ("&&" equality)*
equality       = comparison (("==" | "isGiving" | "!=") comparison)*
comparison     = addition (("<" | ">" | "<=" | ">=") addition)*
addition       = multiply (("+" | "-") multiply)*
multiply       = unary (("*" | "/" | "%") unary)*
unary          = ("-" | "!") unary | primary
primary        = INT_LIT | FLOAT_LIT | STRING_LIT
               | "bet" | "nah" | "ghosted"
               | IDENT "(" args ")"
               | IDENT
               | "(" expr ")"

args           = (expr ("," expr)*)?
```

---

## 10. Compiler Architecture

```
Source Code
    │
    ▼ ─────────────── Phase 1: Lexical Analysis ──────────────────────
    │  lexer/lexer.go      Characters → Tokens
    │  lexer/token.go      Token type definitions
    │
    ▼ ─────────────── Phase 2: Parsing ───────────────────────────────
    │  parser/parser.go    Tokens → Abstract Syntax Tree
    │  ast/ast.go          AST node type definitions
    │
    ▼ ─────────────── Phase 3: Semantic Analysis ─────────────────────
    │  semantic/analyser.go  Type checking, scope analysis, immutability
    │
    ▼ ─────────────── Phase 4A: Code Generation ──────────────────────
    │  codegen/codegen.go    AST → VM Bytecode
    │  vm/opcode.go          Instruction set definitions
    │
    ▼ ─────────────── Phase 4B: Tree-Walk Interpretation (alt.) ──────
    │  eval/eval.go          AST → Direct execution (no bytecode)
    │
    ▼ ─────────────── Phase 5: Virtual Machine ───────────────────────
       vm/vm.go              Bytecode → Program Output
       vm/value.go           Runtime value type
```

---


*Vine Language Specification v0.1.0*
