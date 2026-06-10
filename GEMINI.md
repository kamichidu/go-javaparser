# go-javaparser Development Guidelines (GEMINI.md)

This document establishes the repository-wide architectural conventions, standards, and workflows for `go-javaparser` (`github.com/kamichidu/go-javaparser`), a standalone, highly efficient Java parser backend.
All future development, AI agents, and developers must strictly adhere to these mandates to preserve the core values and performant nature of the codebase.

---

## Core Philosophy

The absolute axiom driving all design decisions in `go-javaparser` is:

> **Complete knowledge is optional. Observation latency is not.**

We prioritize immediate, non-blocking responsiveness. Rather than performing heavy type resolutions or semantic interpretation, this library focuses strictly on scanning and streaming out "observed" declarations within milliseconds.

---

## Core Values (Design Constitution)

To uphold the Core Philosophy, all implementations must conform to the following four Core Values:

Observation Before Interpretation
: **Prioritize observation over interpretation.**
  The parser strictly observes physical syntax declarations (e.g., classes, methods, fields, annotations, physical coordinates). It never attempts to resolve imports, perform name binding, or do type checks. Interpretation is the responsibility of the consumer.

Streaming Before Materialization
: **Prioritize streaming over materialization.**
  Instead of constructing a massive syntax tree in memory (materialization) and returning it at the very end, we prioritize emitting observed declaration facts on the fly as a stream of events (`event.SourceEvent`). This keeps memory allocation near zero and supports extremely fast parallel processing.

Incremental Friendly
: **Natively support incremental processing.**
  To support keystroke-level low-latency inside editors, we avoid re-parsing the entire file for every keystroke. We support local enclosing scope parsing (e.g., parsing only a specific method body block) to perform extremely light and localized updates.

Consumer-Driven Projections
: **Driven strictly by consumer projections.**
  The parser's complexity and scanning depth are driven strictly by the consumer's subscription policies. If the consumer does not request local variable occurrences, method bodies are treated as conceptually non-existent: the `Parser Core` skips them entirely without tokenizing or allocating memory for them.

---

## Triple Projections Architecture

`go-javaparser` decouples the `Parser Core` from how the parsed data is materialized or projected. The core supports three completely independent projection packages:

```text
               +-----------------------------+
               |         Parser Core         | (Handwritten LL(k) Parser)
               +--------------+--------------+
                              |
         +--------------------+--------------------+
         |                    |                    |
         v                    v                    v
+-----------------+  +-----------------+  +-----------------+
|  AST Projection |  | Event Projection|  |  Completion     |
|                 |  |                 |  |  Projection     |
+-----------------+  +-----------------+  +-----------------+
| ast/            |  | event/          |  | completion/     |
| (Declaration    |  | (Direct         |  | (Syntax Context |
|  Oriented AST)  |  |  Event Stream)  |  |  Observation)   |
+-----------------+  +-----------------+  +-----------------+
```

All three projections are entirely independent; none of them import or depend on the others. Consumers may choose to subscribe to **any, all, or a specific combination** of these projections depending on their performance and structural requirements.

AST Projection (`ast/` package)
: Materializes the parsed tokens into a lightweight, structural, declaration-oriented tree representation (`ast.File`).
  * **Best for**: Class outline displays, code structure visualizers, code formatters, and static analysis tools that need to traverse a tree.

Event Projection (`event/` package)
: Direct streaming. Emits chronological `SourceEvent` events on the fly directly to an `EventSink` without allocating or constructing any intermediate AST nodes.
  * **Best for**: Ultra-low-latency background indexers, large-scale asynchronous workspace discovery, and any context where memory footprints must be kept near zero.

Completion Projection (`completion/` package)
: Real-time syntax context observation. Parses the file's live buffer and projects a `CompletionContext` containing the current package name, active imports, local variables in scope, and a parsed/classified `ReceiverExpr` (extracting the receiver expression and member prefix using unnested-dot backtracking).
  * **Best for**: Highly performant, context-aware auto-completion systems inside Language Server Protocol servers without introducing resolving overhead.

---

## Package Layout & Dependency Boundaries

To prevent circular package imports and maintain clean package boundaries, the dependency graph is strictly unidirectional, layered over a shared syntax package:

```text
               +------------+
               |   parser   | (Parser Core - imports syntax, ast, and event)
               +---/-----\--+
                  /       \
                 v         v
            +---+---+   +---\---+   +------------+
            |  ast  |   | event |   | completion | (Completely independent of each other)
            +---\---+   +---/---+   +-----/------+
                 \         /             /
                  v       v             /
               +---\-----/-------------v+
               |         syntax         | (Shared vocabulary - Range, Position, TypeKind, Parameter)
               +------------------------+
```

```text
/github.com/kamichidu/go-javaparser/
|-- go.mod                  # Go module definition (Go 1.25.1, zero third-party deps)
|-- syntax/
|   `-- syntax.go           # [Shared] Ranges, Positions, TypeKinds, Parameter DTOs
|-- ast/
|   `-- ast.go              # Declaration-Oriented AST node representations (depends on syntax)
|-- lexer/
|   |-- lexer.go            # High-performance state-based lexical scanner
|   `-- token.go            # Lexical Token definitions
|-- parser/
|   |-- errors.go           # Parse error representations (ParseError)
|   `-- parser.go           # Handwritten LL(k) Parser Core (depends on syntax, ast, and event)
|-- event/
|   `-- event.go            # SourceEvent and EventSink definitions (depends on syntax, zero ast dependency)
`-- completion/
    `-- completion.go       # CompletionContext, ReceiverExpr and context extractor (depends on syntax)
```

---

## Technical Standards & Design Contracts

### 1. Zero Third-Party Dependencies
The module must **never** introduce any external third-party dependencies (e.g., ANTLR, goyacc, external loggers). Only Go's standard library is permitted to maintain immediate starts, ultra-fast compilation, and compact binary sizes.

### 2. V1 Range Parsing & Failover Contract
- **No Diff calculation**: The parser itself does not compute diffs between previous and current syntax states.
- **Consumer-Driven Range**: The consumer (e.g. LSP server) calculates and isolates the exact local range (e.g., byte offsets or line ranges of the method body) that needs re-evaluation and feeds only that substring to the parser.
- **Parser Core Simplicity**: Parser Core simply executes the parser blindly on the given text range without attempting to locate the boundaries or "guess" the surrounding block.
- **Partial Parse Failure fallback**: If local re-parsing fails inside `ParseLocalScope` due to syntax mismatches, the `Parser Core` simply reports a `Partial Parse Failure` event and returns an error. The Parser Core **never** automatically runs a fallback complete parse internally. The consumer decides how to handle the failure:
  1. **Maintain stale facts**: Keep using the last successfully observed facts inside the buffer to prevent editor symbols from suddenly vanishing.
  2. **Trigger asynchronous complete parse**: Queue a background job to perform a full-file compile-parse asynchronously without blocking the editor thread.
  3. **Synchronous full fallback**: Only perform a blocking, synchronous file compile-parse if the current context (e.g., CLI or deep reference query) accepts latency.

### 3. Javadoc & String Block Scanning
- When performing body skipping, the `Lexer` must properly classify string literals (`"hello"`), character literals, multi-line comment blocks (`/*...*/`), and Javadoc blocks (`/**... */`).
- This prevents the braces-counting engine from being tripped up by unmatched curly braces enclosed within string literals or comment blocks inside method bodies.

### 4. Completion Context Observation Rules
- **Pure Observation**: The `completion` package strictly observes and projects the syntax context (e.g. extracting `System.out` and `f` from `System.out.f`). It **must never** attempt to resolve imports, do type checks, or perform name binding. It remains completely stateless.
- **Unnested-dot Backtracking**: It must scan backwards from the cursor to find the nearest unnested dot `.` (handling nested parentheses, brackets, or generic angle brackets) to cleanly segment the Receiver Expression from the Member Prefix.
- **Generics Variable Recognition**: It must correctly recognize local variable declarations with generics (e.g. `List<String> list;`) by parsing and skipping balanced generic tags, ensuring live local variables are always captured cleanly.

---

## Implementation & Integration Lessons

### 1. Go Struct Field-Method Name Collisions
In Go, a struct cannot declare a field and a method with the exact same name (they share the same namespace within the struct).
Since our `event.SourceEvent` interface mandates the `Type() EventType` method, any event struct (like `FieldDeclEvent` or `LocalDeclEvent`) must **never** name its inner string field `Type`. Instead, they are named `DataType` to prevent compilation failures.

### 2. LSP Coordinate Mapping Policy (Consumer's Responsibility)
`go-javaparser` is designed as a standard standalone compiler-style parser, operating strictly on **1-based text coordinates** (both lines and columns begin at 1). It remains completely unaware of downstream presentation formats or protocol wires.
**The responsibility of coordinate mapping (such as translating 1-based coordinates to 0-based formats for LSP-centric consumers) belongs entirely to the consumer.** The `Parser Core` remains pure, stateless, and fully decoupled from any coordinate translation logic.

### 3. Parameter List Parser Synchronizer
In `Parser Core`'s recursive-descent parameter parsing, encountering `{` or `;` inside a parameter list before seeing the closing parenthesis `)` represents an unclosed parameter syntax error.
To prevent the parser from swallowing the subsequent method body block as parameters, the parser must report a clean syntax `ParseError` and immediately break out of the parameter parsing loop.

---

## Development Workflows

### 1. Verification Tests
To verify that all changes are compile-clean and functionally correct, execute the tests in the `./javaparser` directory:

1. **Run All Tests**:
   ```bash
   go test -v ./...
   ```
2. **Run Scanner Only**:
   ```bash
   go test -v ./lexer/...
   ```
3. **Run Parser Core & Projections Only**:
   ```bash
   go test -v ./parser/...
   ```

### 2. Code Formatting
Always format Go source files before committing using standard Go formatting:
```bash
go fmt ./...
```
