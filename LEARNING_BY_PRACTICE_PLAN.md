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
- Deployed to `ttc.ctln.pw` (Docker Compose + Caddy on VPS)

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

## Milestone 4: Rooms, Lobbies & Reconnect (web only) — DONE

**Goal:** Move from a single anonymous web match into a real browser game with
stable client identity, shareable room URLs, reconnect-capable rooms, and a
better multiplayer loop.

**Acknowledged limitation:** All state is still in-memory. Server restart = all
games and lobbies lost. Acceptable for now.

**What was built:**
- `cmd/web/main.go` now serves route-based pages for `/`, `/lobby`, `/lobby/{id}`,
  and `/room/{id}`, plus REST endpoints and dedicated WebSocket endpoints
- `cmd/web/client.go` issues stable client IDs and validates them through `/api/me`
- `cmd/web/lobby.go` + `cmd/web/lobby_registry.go` implement a default matchmaking
  lobby plus private invite lobbies
- `cmd/web/room_registry.go` tracks room IDs and maps browser clients to room participants
- `internal/game/room.go` survives disconnects, supports reconnect, and handles rematch
  with color swap
- `cmd/web/static/app.js` handles browser routing, local token storage, invite links,
  room joins, reconnect-on-page-load, rematch UI, connection-lost UI, and board rendering

**Architecture:**
```
Browser SPA-ish client
├── Home page (`/`)
├── Default lobby (`/lobby`)
├── Private invite lobby (`/lobby/:id`)
└── Room page (`/room/:id`)

Go web server (`cmd/web`)
├── `POST /api/clients`     create stable client identity
├── `GET /api/me`           validate stored client token
├── `POST /api/lobbies`     create invite-only lobby
├── `GET /ws/lobby`         default matchmaking
├── `GET /ws/lobby/{id}`    private lobby
└── `GET /ws/room/{id}`     authenticated room connection

Game runtime
├── lobby registry    owns join/pairing state
├── room registry     owns room lookup by room ID
└── Room.Run()        owns game state, disconnect/reconnect, rematch
```

### What is already done

- **Stable browser identity:** the browser keeps a client token in `localStorage`
  and reuses it across page loads
- **Room IDs in URLs:** pairing now produces a room ID and the client navigates to
  `/room/:id`
- **Private invite lobbies:** `/api/lobbies` creates a shareable lobby link for a friend
- **Dedicated WebSocket endpoints:** lobby and room traffic are now separated
- **Participant auth:** only room participants can connect to `/ws/room/:id`
- **Reconnect-capable room runtime:** if a player disconnects, `Room.Run()` stays alive
  and accepts a new command/update channel pair for the same player ID
- **Opponent presence events:** room sends away / reconnected notifications
- **Rematch with color swap:** after game over, either player can request a rematch

### What is partially done / still missing

- **Automatic reconnect in the browser:** the room runtime supports reconnect, but the
  browser client still mostly falls back to a "Connection lost" state instead of doing
  backoff + retry + `visibilitychange` recovery
- **Lifecycle cleanup:** room and lobby registries keep data in memory; there is no TTL
  or garbage collection for abandoned rooms/lobbies yet
- **Persistence across server restarts:** still intentionally absent

### Go concepts learned

- **ServeMux path patterns** — `GET /room/{id}`, `GET /ws/room/{id}`, etc.
- **Stable identity vs transport session** — browser client ID is durable; WebSocket
  connection is ephemeral
- **Registry types + `sync.Mutex`** — lobby/room/client lookup shared across handlers
- **Reconnect through channel replacement** — handlers attach fresh channels to the same
  in-memory room instead of rebuilding game state
- **Client-side routing without a framework** — URL-driven state with plain JS
- **Incremental web architecture** — add lobbies, invite flow, rooms, and rematch without
  throwing away the existing server

---

## Milestone 5: AI Agent Arena

**Idea:** Run AI agents that play tic-tac-chec against humans (and each other) via the web UI. Track game results to compare how different agents perform and how they improve over time as their skill prompts evolve.

**Basic flow:**
1. An AI agent runs on the server (in a Docker container), connects to the web server as a player
2. Human opens the web UI and gets paired with the agent (or another human)
3. Game results (winner, moves, agent identity/version) are persisted
4. A stats dashboard shows win rates, agent comparisons, and performance over time

**The interesting question:** Can an agent "learn" by iterating on its skill prompt? Play N games → analyze losses → update prompt → play again → measure improvement.

**Open design questions (to revisit):**
- Agent runtime: Claude Code with skill, Claude Agent SDK, or direct API calls?
- How agents connect: WebSocket client, HTTP API, or in-process?
- Matchmaking: "Play vs AI" button, separate lobby, or automated batch games?
- Storage: SQLite, PostgreSQL, or flat files?
- Dashboard: server-rendered or static JS?

---

## Milestone 6 (future): Cross-transport play

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

Milestone 4: Rooms, Lobbies & Reconnect      ✅ DONE
  └─ learned: path routing, stable identity, registries, reconnect-by-channel-swap, plain JS routing
  └─ follow-up: auto reconnect, registry cleanup, optional persistence

Milestone 5: AI Agent Arena                  ⬜ FUTURE
  └─ teaches: LLM API integration, game result persistence, stats/dashboards

Milestone 6: Cross-transport play            ⬜ FUTURE
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
