# Learning by Practice Plan: Tic-Tac-Chec Multiplayer

## Goal
Build multiplayer features progressively, each teaching core Go concepts.
Play order is designed so each milestone builds on the previous one.

---

## Milestone 1: CLI + Claude Skill (AI opponent) — DONE

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

## Milestone 2: SSH Server (real multiplayer) — DONE

**What was built:**
- `cmd/server/` — plain TCP game server (legacy, text protocol on :9090)
- `cmd/ssh/main.go` — SSH game server using `charmbracelet/wish` + bubbletea middleware on :2222
- `internal/game/room.go` — Room type multiplexes moves from both players via channels
- `internal/ui/` — Bubble Tea TUI with online mode, board flipping, color schemes
- Lobby auto-pairing, disconnect handling, host key management
- Deployed to Railway at `tramway.proxy.rlwy.net:17014`

**Architecture:**
```
ssh player1@localhost -p 2222
ssh player2@localhost -p 2222

SSH Server (wish + bubbletea middleware)
├── Lobby goroutine: pairs connecting players via unbuffered channel
├── Room goroutine: owns game state, multiplexes moves via select
│       ┌──────────────────────────────┐
│  P1 ──┤  chan MoveRequest (in)       ├── P2
│       │  chan tea.Msg (out, buf 1)   │
│       └──────────────────────────────┘
└── Each SSH session: Bubble Tea TUI with online mode
                      sends MoveRequest → Room
                      receives GameStateMsg/ErrorMsg ← Room
```

**Go concepts learned:**
- Goroutines and the "goroutine per connection" pattern
- Channels: unidirectional (`chan<-`, `<-chan`), buffered vs unbuffered
- `select` statement for multiplexing moves from two players
- Channel-based state ownership (CSP style)
- SSH server with `charmbracelet/wish` + bubbletea middleware
- Disconnect handling via channel close detection

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
Milestone 1: CLI + Claude skill              ✅ DONE
  └─ learned: JSON, file I/O, CLI design (Kong)

Milestone 2: SSH server                      ✅ DONE
  └─ learned: goroutines, channels, select, wish, CSP, deployment

Milestone 3: Web UI                          ⬜ NEXT
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

## Next Up: Milestone 3

Build the Web UI — HTTP server + WebSocket + minimal browser client.
Reuse the `internal/game` Room/Player pattern from the SSH server, with a WebSocket transport instead of SSH sessions.
