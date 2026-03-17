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

## Milestone 3: Web UI — DONE

**What was built:**
- `cmd/web/main.go` — HTTP + WebSocket server with lobby and game bridge
- `cmd/web/static/` — browser client (HTML + JS + CSS), reactive state-based rendering
- `internal/wire/` — shared JSON serialization (extracted from CLI)
- Build-tag based static serving (`embed.FS` prod, disk dev)
- Decoupled `internal/game/room.go` from bubbletea (`chan tea.Msg` → `chan any`)
- Room color validation, initial game state broadcast
- Dockerized (`Dockerfile.web`), wss:// support for deployment

**Architecture:**
```
Browser (HTML+JS) ──WebSocket──► cmd/web/ (HTTP server)
                                  ├── embedded static files (embed.FS + fs.Sub)
                                  ├── /ws endpoint (WebSocket upgrade)
                                  ├── lobby goroutine (pairs players)
                                  └── Room.Run() (reused from internal/game)

WebSocket handler: read goroutine + write loop
  read:  browser JSON → parse.Piece/Square → moves channel → Room
  write: Room → player.Incoming → JSON envelope → browser
  coordination: done channel + sync.Once for clean shutdown
```

**Go concepts learned:**
- `net/http` — ServeMux, handlers, request context lifecycle
- WebSockets — HTTP upgrade, bidirectional messaging (`coder/websocket`)
- `embed.FS` + `fs.Sub` — single-binary static file serving
- Build tags — compile-time dev/prod switching
- Goroutine coordination — `done` channels, `sync.Once`, read/write loop pattern
- `-race` flag for detecting concurrent access bugs
- Air hot-reload for development

---

## Milestone 4: Rooms & Reconnect (web only)

**What to build:**
- Room registry — persistent rooms that outlive WebSocket disconnects
- Lobby auto-pairs into named rooms (room gets an ID, shareable URL)
- Reconnect — player token, re-attach WebSocket to existing room/slot
- Room lifecycle: waiting → playing → finished, with timeout cleanup

**Architecture:**
```
Browser ──WebSocket──► cmd/web/
                        ├── Lobby: auto-pairs → creates named room in registry
                        ├── Registry: map[string]*ManagedRoom (sync.Mutex)
                        │     └── ManagedRoom: ID, Room, player slots, state, timeout
                        ├── Reconnect: token in sessionStorage → re-attach to room
                        └── Direct join: /?room=abc123 → skip lobby, join existing room
```

**Go concepts to learn:**
- `sync.Mutex` — protecting shared state (room registry)
- `context.WithTimeout` — room cleanup after abandonment
- Session management — token-based player identity
- State machines — room lifecycle management
- Map access patterns — concurrent-safe registry

---

## Milestone 5 (future): Cross-transport play

- Shared room registry between SSH and web servers
- Adapter layer between Bubble Tea and channel-based rooms
- SSH player paired with web player in the same room

---

## Progression Map

```
Milestone 1: CLI + Claude skill              ✅ DONE
  └─ learned: JSON, file I/O, CLI design (Kong)

Milestone 2: SSH server                      ✅ DONE
  └─ learned: goroutines, channels, select, wish, CSP, deployment

Milestone 3: Web UI                          ✅ DONE
  └─ learned: net/http, WebSockets, embed.FS, build tags, goroutine coordination

Milestone 4: Rooms & Reconnect              ⬜ NEXT
  └─ teaches: sync.Mutex, context timeouts, session management, state machines

Milestone 5: Cross-transport play            ⬜ FUTURE
  └─ teaches: adapter patterns, shared state across transports
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
