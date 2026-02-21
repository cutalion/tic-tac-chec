# Learning by Practice Plan: Tic-Tac-Chec Multiplayer

## Goal
Build multiplayer features progressively, each teaching core Go concepts.
Play order is designed so each milestone builds on the previous one.

---

## Milestone 1: CLI + Claude Skill (AI opponent)

**What you build:**
- `cmd/cli/main.go` — a command-line interface to the engine
  - `ttt move WR b3` — apply a move
  - `ttt status` — print board + whose turn
  - `ttt new` — start a new game
- `state.json` — serialized game state persisted between CLI calls
- `.claude/skills/play-tic-tac-chec.md` — skill prompt that defines Claude's behavior as opponent

**How it works:**
```
/play-tic-tac-chec
     │
     ├─ Claude reads state.json (Read tool)
     ├─ Displays board
     ├─ Asks for your move ("move rook to b3")
     ├─ Calls: ./ttt move WR b3  (Bash tool)
     ├─ Claude thinks about its move (AI logic from skill prompt)
     ├─ Calls: ./ttt move BN a2  (Bash tool)
     └─ Displays updated board, loops
```

**Go concepts learned:**
- `encoding/json` — Marshal/Unmarshal for game state persistence
- `os.Args` or `flag` package — CLI argument parsing
- `os.ReadFile` / `os.WriteFile` — simple file I/O
- Package design: CLI as a thin wrapper over the engine

---

## Milestone 2: SSH Server (real multiplayer)

**What you build:**
- `cmd/server/main.go` — SSH game server using `charmbracelet/wish`
- Two players SSH in, both get the existing Bubbletea TUI
- A game room ties them together via channels

**Architecture:**
```
ssh player1@localhost -p 2222
ssh player2@localhost -p 2222

Game Server
├── Lobby goroutine: pairs up connecting players
├── GameRoom goroutine: owns game state, processes moves
│       ┌─────────────────────────┐
│  P1 ──┤  chan Move (in)         ├── P2
│       │  chan GameState (out)   │
│       └─────────────────────────┘
└── Each player goroutine: reads input → sends to room
                           receives state → renders TUI
```

**Go concepts learned:**
- Goroutines and the "goroutine per connection" pattern
- Channels: unidirectional (`chan<-`, `<-chan`), buffered vs unbuffered
- `select` statement for multiplexing
- `sync.Mutex` vs channel-based state ownership (CSP style)
- SSH server with `charmbracelet/wish`
- Context cancellation for cleanup

---

## Milestone 3: Web UI

**What you build:**
- `cmd/webserver/main.go` — HTTP + WebSocket server
- Minimal browser client (HTML + JS) — no framework needed
- The engine runs server-side; browser just renders state and sends moves

**Architecture:**
```
Browser ──WebSocket──► Go HTTP server ──► Game engine
                      (net/http)
                      one goroutine per WS connection
                      channels connect players in a room
```

**Go concepts learned:**
- `net/http` — handlers, middleware, routing
- WebSockets — full-duplex, `gorilla/websocket` or stdlib
- JSON as wire protocol
- Graceful shutdown with `context.Context`
- CORS and HTTP basics

---

## Progression Map

```
Milestone 1: CLI + Claude skill
  └─ teaches: JSON, file I/O, CLI design, os.Args

Milestone 2: SSH server
  └─ teaches: goroutines, channels, select, wish, CSP

Milestone 3: Web UI
  └─ teaches: net/http, WebSockets, context, JSON API
```

Each milestone leaves the engine untouched — it's pure logic.
The transport layer (CLI / SSH / HTTP) is always a separate cmd/.

---

## Distribution Strategy

**Milestone 1:** Project-local skill only
- Binary: `go build -o ttt ./cmd/cli` — run from project root
- Skill: `.claude/skills/play-tic-tac-chec.md` — loaded automatically by Claude Code
- No install step needed

**After Milestone 2:** Add a Makefile (`make install`)
- Installs binary to `~/.local/bin/ttt-chec` (in $PATH)
- Copies skill to `~/.claude/skills/ttt-chec.md`
- Skill references binary by name, not hardcoded path

---

## Starting Point: Milestone 1

First task: build the CLI in `cmd/cli/main.go`.

The CLI needs to:
1. Load game state from `state.json` (or start fresh if file missing)
2. Parse a command from `os.Args`
3. Apply the command to the engine
4. Save updated state back to `state.json`
5. Print the result

The game state to serialize is the `engine.Game` struct.
Key question to explore: what does Go's `encoding/json` require of a struct to serialize it?
(Hint: exported fields, and pointer fields need special thought.)
