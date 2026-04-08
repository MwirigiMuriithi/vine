# Vine Language VS Code Extension

Official language support for the **Vine programming language**, a statically-typed, procedural language built for learning compiler design, using internet-culture keywords.

---

## Features

###  Syntax Highlighting
Full semantic coloring for every Vine keyword, type, operator, and literal. The grammar distinguishes between:
- **Declaration keywords** (`lowkey`, `noCap`, `lockIn`, `forge`)
- **Control flow** (`perchance`, `otherwise`, `checkTheFit`, `style`, `noFilter`)
- **Loop keywords** (`letHimCook`, `spinTheBlock`, `ghost`, `keepItMoving`)
- **Error handling** (`attempt`, `catch`, `throwHands`)
- **Built-in I/O** (`spill`)
- **Literals** (`bet`, `nah`, `ghosted`)
- **Types** (`int`, `float`, `string`, `bool`, `void`)
- **Operators** (`isGiving`, `==`, `!=`, `<`, `>`, `&&`, `||`, etc.)

###  Vine Dark Theme
A bespoke dark theme built specifically around Vine's keyword palette:
- Muted dark background (`#0f1117`) for long coding sessions
- Green accent (`#4ade80`) for the Vine brand color
- Each keyword *category* gets a distinct hue, loops are green, conditionals are blue, error handling is red/amber, `forge` declarations are purple

###  Language Configuration
- Auto-closing brackets, braces, and quotes
- Block folding (`{` / `}`)
- Smart indentation on `Enter` after `{`
- Line comment toggle (`// ...`)
- Word-boundary detection for Vine identifiers

###  Snippets
Type a keyword prefix and hit `Tab` to expand:

| Prefix | Expands to |
|--------|-----------|
| `lowkey` | `lowkey name int = value` |
| `noCap` | `noCap name int = value` |
| `lockIn` | `lockIn name int = value` |
| `forge` | Full function scaffold |
| `main` | `forge main() void { }` |
| `spill` | `spill(value)` |
| `perchance` | If block |
| `perchance-otherwise` | If-else block |
| `perchance-chain` | If / else-if / else chain |
| `letHimCook` | While loop |
| `spinTheBlock` | For loop (with `lowkey i` init) |
| `checkTheFit` | Switch/match with 2 arms + `noFilter` |
| `attempt` | Try-catch block |
| `throwHands` | Throw statement |
| `itIsWhatItIs` | Return statement |
| `vine-starter` | Full minimal starter program |
| `forge-recursive` | Recursive function template |

---

## Installation

### From the Marketplace
Search **"Vine Language"** in the VS Code Extensions panel (`Ctrl+Shift+X`) and click Install.

### From a `.vsix` file
```bash
code --install-extension vine-language-1.0.0.vsix
```

### From source
```bash
git clone https://github.com/MwirigiMuriithi/vine/
cd vine-vscode
pnpm install
pnpm vsce package # produces vine-language-1.0.0.vsix
code --install-extension vine-language-1.0.0.vsix
```

---

## Using the Vine Dark Theme
1. Open the Command Palette (`Ctrl+Shift+P`)
2. Run `Preferences: Color Theme`
3. Select **Vine Dark**

---

## Vine Keyword Quick Reference

```
lowkey x int = 5          // mutable variable     (var)
noCap MAX int = 100        // immutable constant   (const)
forge add(a int, b int) int { }  // function       (func)
itIsWhatItIs a + b         // return               (return)
spill("hello")             // print                (print)
perchance (x > 0) { }     // if                   (if)
otherwise { }             // else                 (else)
letHimCook (x > 0) { }    // while loop           (while)
spinTheBlock (lowkey i int = 0; i < 10; i = i + 1) { }  // for
ghost                     // break                (break)
keepItMoving              // continue             (continue)
checkTheFit (x) { style 1: { } noFilter: { } }  // switch
attempt { } catch e { }  // try/catch
throwHands("msg")         // throw                (throw)
bet / nah                 // true / false
ghosted                   // null
isGiving                  // == (equality)
```

---


## Contributing

Issues and PRs welcome at [https://github.com/MwirigiMuriithi/vine](https://github.com/MwirigiMuriithi/vine).

---

