<!--
Sync Impact Report
- Version change: 0.0.0 → 1.0.0 (initial ratification)
- Added principles:
  - I. Outside-In TDD
  - II. Engine Purity
  - III. Human-Written Go
  - IV. Incremental MVP
  - V. YAGNI
- Added sections:
  - Architecture Constraints
  - Development Workflow
  - Governance
- Templates requiring updates:
  - .specify/templates/plan-template.md ✅ no changes needed (Constitution Check section is generic)
  - .specify/templates/spec-template.md ✅ no changes needed (user stories structure compatible)
  - .specify/templates/tasks-template.md ✅ no changes needed (phase structure compatible)
  - .specify/templates/commands/*.md — no command files exist
- Follow-up TODOs: none
-->

# Tic-Tac-Chec Constitution

## Core Principles

### I. Outside-In TDD (NON-NEGOTIABLE)

Development MUST follow outside-in test-driven development:

- Start from high-level use cases; let failing tests drive creation
  of lower-level types and functions
- Red-Green-Refactor cycle strictly enforced
- No production code without a failing test that demands it
- Tests define the public API before implementation exists

Rationale: Outside-in TDD ensures the design is driven by actual
usage rather than speculative abstractions. It keeps the codebase
lean and every line of code justified.

### II. Engine Purity

The `engine/` package MUST contain pure game logic with zero I/O:

- No `fmt.Print`, no `os` calls, no network access
- All state transitions expressed as pure functions or methods on
  value types
- Engine MUST be independently testable without mocks for
  external systems

Rationale: A pure engine enables multiple frontends (CLI, TUI, SSH,
Web) to share identical game logic without coupling to any
presentation layer.

### III. Human-Written Go

The user MUST write all Go code themselves as a learning exercise:

- Claude explains concepts, architecture, and reasoning but MUST
  NOT generate Go code for the user
- Exception: boring/mechanical tasks (string literals, boilerplate,
  formatting) may be delegated to Claude
- The boundary is Go concepts vs. busywork — if writing it teaches
  something, the user writes it

Rationale: The project exists to learn Go deeply. Writing code
yourself is the only way to build real fluency — not just syntax,
but idioms, patterns, and the standard library.

### IV. Incremental MVP

For new packages and modules, build a simplified MVP first, then
extend incrementally:

- Each milestone MUST deliver a working, testable artifact
- Features are sliced thin: the smallest useful thing ships first
- New capabilities build on proven foundations, not speculative
  designs

Rationale: Incremental delivery surfaces integration issues early
and keeps feedback loops tight. Each step validates assumptions
before the next one builds on them.

### V. YAGNI

Only add what a failing test demands:

- No speculative infrastructure, no "just in case" abstractions
- Complexity MUST be justified by a concrete, current need
- Three similar lines of code are better than a premature
  abstraction

Rationale: Speculative code creates maintenance burden and obscures
the actual design. When you need it, you'll know — and the tests
will tell you.

## Architecture Constraints

- **Layered packages**: `engine/` (pure logic) → `internal/`
  (shared infrastructure: game, wire, ui, display, parse) →
  `cmd/` (entry points: cli, tui, ssh, web)
- **Dependency direction**: `cmd/` depends on `internal/` and
  `engine/`; `internal/` depends on `engine/`; `engine/` depends
  on nothing outside the standard library
- **Concurrency model**: channel-based CSP where Room owns game
  state; players communicate via channels
- **Deployment**: Docker Compose + Caddy; SSH host keys persisted
  in volumes

## Development Workflow

- **Branch strategy**: feature branches off `main`
- **Commit discipline**: commit after each task or logical group;
  no Co-Authored-By trailer
- **Testing tools**: `go test`, `go-cmp` for equality comparisons
- **CLI framework**: Kong (not cobra, not flag)
- **Testability pattern**: `NewApp(out, err io.Writer)` —
  inject writers, use `io.Discard` in tests

## Governance

This constitution supersedes all other development practices for
the Tic-Tac-Chec project. Amendments require:

1. Documentation of the proposed change and rationale
2. Version bump following semantic versioning:
   - MAJOR: principle removal or incompatible redefinition
   - MINOR: new principle or materially expanded guidance
   - PATCH: clarifications, wording, typo fixes
3. Update of the Sync Impact Report at the top of this file
4. Propagation check against all `.specify/templates/` files

All plans and specs MUST verify compliance with these principles
via the Constitution Check section before implementation begins.

**Version**: 1.0.0 | **Ratified**: 2026-04-05 | **Last Amended**: 2026-04-05
