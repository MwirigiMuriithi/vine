# Vine Learning Compiler

> A complete, heavily-documented compiler built for the **Compiler Construction & Design** course,<br>
> written in Go, using internet-culture gen-z keywords .

---

## Vine Keyword Reference

| Vine Keyword | Meaning | Traditional Equivalent |
|---|---|---|
| `lowkey x int = 5` | mutable variable | `var x int = 5` |
| `noCap x int = 5` | immutable constant | `const x int = 5` |
| `lockIn x int = 5` | same as noCap | `const x int = 5` |
| `forge f(a int) int {}` | function definition | `func f(a int) int {}` |
| `itIsWhatItIs expr` | return | `return expr` |
| `spill(expr)` | print to stdout | `print(expr)` |
| `perchance (cond) {}` | if | `if (cond) {}` |
| `otherwise {}` | else | `else {}` |
| `isGiving` | equality operator | `==` |
| `letHimCook (cond) {}` | while loop | `while (cond) {}` |
| `spinTheBlock (;;) {}` | for loop | `for (;;) {}` |
| `ghost` | break out of loop | `break` |
| `keepItMoving` | continue to next iter | `continue` |
| `bet` | boolean true | `true` |
| `nah` | boolean false | `false` |
| `ghosted` | null value | `null` |
| `checkTheFit (x) {}` | switch/match | `switch (x) {}` |
| `style val: {}` | case arm | `case val:` |
| `noFilter: {}` | default arm | `default:` |
| `attempt {} catch e {}` | try/catch | `try {} catch(e) {}` |
| `throwHands("msg")` | throw error | `throw "msg"` |

---

## Quick Start

```bash
cd vine
go build -o vine .

./vine examples/hello.vine
./vine examples/variables.vine
./vine examples/control_flow.vine
./vine examples/functions.vine
./vine examples/advanced.vine

# Debug flags:
./vine --dump-tokens   examples/hello.vine
./vine --dump-ast      examples/functions.vine
./vine --dump-bytecode examples/control_flow.vine
```

---

## Example Program

```vine
// A complete Vine program showing the core features

forge isPrime(n int) bool {
    perchance (n <= 1) { itIsWhatItIs nah }
    lowkey d int = 2
    letHimCook (d < n) {
        perchance (n % d isGiving 0) { itIsWhatItIs nah }
        d = d + 1
    }
    itIsWhatItIs bet
}

forge safeDivide(a int, b int) int {
    perchance (b isGiving 0) {
        throwHands("can't divide by zero bestie")
    }
    itIsWhatItIs a / b
}

forge main() void {
    // noCap constant
    noCap LIMIT int = 20

    // spinTheBlock (for loop) + checkTheFit (switch)
    spinTheBlock (lowkey i int = 1; i <= LIMIT; i = i + 1) {
        checkTheFit (i % 15) {
            style 0:  { spill("FizzBuzz") }
            noFilter: {
                perchance (i % 3 isGiving 0) { spill("Fizz") }
                perchance (i % 5 isGiving 0) { spill("Buzz") }
                perchance (isPrime(i))        { spill(i) }
            }
        }
    }

    // attempt / catch error handling
    attempt {
        spill(safeDivide(10, 2))    // 5
        spill(safeDivide(10, 0))    // throwHands!
    } catch err {
        spill("caught: " + err)
    }
}
```

---

## Project Structure

```
vine/
├── main.go                  ← Compiler driver (orchestrates all phases)
├── go.mod
├── lexer/
│   ├── token.go             ← Token types + keyword table (all the slang lives here)
│   └── lexer.go             ← Lexer: characters → token stream
├── ast/
│   └── ast.go               ← AST node definitions
├── parser/
│   └── parser.go            ← Recursive-descent parser: tokens → AST
├── semantic/
│   └── analyser.go          ← Type checker + scope stack + immutability rules
├── codegen/
│   └── codegen.go           ← Code generator: AST → bytecode (with backpatching)
├── vm/
│   ├── value.go             ← Value type (int/float/string/bool/nil)
│   ├── opcode.go            ← Instruction set definitions
│   └── vm.go                ← VM execution engine with try/catch support
└── examples/
    ├── hello.vine
    ├── variables.vine       ← lowkey, noCap, lockIn
    ├── control_flow.vine    ← perchance, letHimCook, spinTheBlock, ghost, checkTheFit
    ├── functions.vine       ← forge, itIsWhatItIs, recursion
    └── advanced.vine        ← everything together + attempt/catch
```

---

## Compiler Phases & CS Concepts

| Phase | File | Key Concept |
|---|---|---|
| Lexical Analysis | `lexer/` | Finite State Machine, keyword table |
| Parsing | `parser/` | Recursive Descent, operator precedence via grammar |
| Semantic Analysis | `semantic/` | Symbol Table, Scope Stack, Two-Pass, Immutability |
| Code Generation | `codegen/` | Tree-Walk, Backpatching, Loop Context Stack |
| Execution | `vm/` | Stack Machine, Call Frames, Try Stack |

## Contributing

Contributions are welcome.

If you want to help improve Vine, please:

1. Fork the repository
2. Create a feature branch
3. Make your changes with clear commits
4. Add or update tests where relevant
5. Run the project and make sure it still works
6. Open a pull request with a clear description of what changed



---

## Areas for Contribution

### Suggested areas to contribute
- Lexer, parser, semantic analysis, and code generation improvements
- New language features and keyword support
- Better error messages and diagnostics
- More example `.vine` programs
- Documentation and compiler-construction explanations
- VS Code extension improvements

### Language Features
- Arrays / slices
- Structs or user-defined types
- Pattern matching improvements (`checkTheFit`)
- String interpolation
- Standard library functions

### Compiler Improvements
- Better error recovery in the parser
- Type inference (partial or full)
- Constant folding / simple optimizations
- Dead code elimination
- Improved symbol table design

### Runtime / VM
- Performance improvements
- Debugging support (step execution, stack inspection)
- Better error stack traces
- Memory management enhancements

### Testing
- More unit tests across all phases
- Edge-case programs
- Invalid syntax / error-case tests

### Documentation
- Expand `LANGUAGE_SPEC.md`
- introduce doc folder
- Add “how the compiler works” walkthroughs
- Inline comments explaining algorithms and design decisions

### VS Code Extension
- Semantic highlighting
- Diagnostics (error reporting from compiler)
- Code completion / IntelliSense
- Formatting support

## Project Structure Contributions
Improvements to the project structure are welcome.

If you believe the current layout can be improved (better separation of concerns, clearer module boundaries, or cleaner architecture), feel free to propose changes.

Guidelines:
- Keep the compiler pipeline clear *(lexer → parser → semantic → codegen → VM)*
- Avoid unnecessary complexity or over-engineering
- Ensure changes improve readability and maintainability
- Explain the reasoning behind structural changes in your PR

Refactors that make the project easier to understand, extend, or teach from are highly encouraged.
---

## Pull Request Expectations

- Keep PRs **small and focused**
- Include a clear description of:
  - what you changed
  - why you changed it
- Link related issues if applicable
- Ensure the project builds and runs

---

## Philosophy

Vine is:
- a **teaching tool** for compiler construction
- a **playground** for language design
- a **fun experiment** using internet-culture syntax

Contributions should balance **correctness**, **clarity**, and **fun**.
<br>

*Vine Programming Language and Compiler v0.1.0*