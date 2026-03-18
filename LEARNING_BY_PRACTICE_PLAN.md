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

**Goal:** Games survive network disconnects — subway tunnels, elevators, closing
laptop and reopening. A player can lose connection and resume their game.

**Acknowledged limitation:** All state is in-memory. Server restart = all games lost.
Acceptable for now.

### Step 1: Room.Run() survives disconnects

The core change. Currently, when a player's `Moves` channel closes, `Room.Run()` exits
and the game is lost. Instead:

**Nil channel pattern:** When a player disconnects, set **both** their `Moves` and
`Incoming` to `nil`. Nil channels block forever in `select`, disabling that case.
All sends to Incoming must be nil-guarded (skip if nil) to prevent deadlocks — otherwise
Room.Run() blocks trying to send GameStateMsg to a disconnected player and never reaches
the reconnect select case.

**Reconnect channels:** Room.Run() selects on two additional unbuffered channels (one per
player slot) that receive reconnection events. A reconnect event carries both a new Moves
channel and a new Incoming channel. On reconnect, close the old Incoming (so any lingering
goroutines reading it can drain) before swapping in the new one. Then send current
GameState to the reconnected player.

**Opponent notifications:** When a player disconnects, send `opponentAway` to the other
player (only if their Incoming is non-nil). When they reconnect, send `opponentReturned`
to the other player + current `GameState` to both.

```
Room.Run() select cases:
  - white.Moves        (nil when disconnected)
  - black.Moves        (nil when disconnected)
  - white.Reconnect    (receives new Moves + Incoming channels)
  - black.Reconnect    (receives new Moves + Incoming channels)
```

### Step 2: Registry

Simple `map[token]*Room` protected by `sync.Mutex` in `cmd/web/`.
Two tokens per room (one per player). Lookup by token returns room + color.
Tokens are 16-byte random hex strings via `crypto/rand`.

No room IDs in URLs for now — token-only reconnect is sufficient.
Room IDs / shareable URLs can be added later if needed.

### Step 3: WebSocket handler

Single `/ws` endpoint. Client sends a first message to indicate intent:
- `{"type": "join"}` → enter lobby → get paired → receive `{token, color, gameState}`
- `{"type": "reconnect", "token": "..."}` → look up room → reattach → receive `{color, gameState}`

If reconnect fails (room expired, invalid token), server sends `{type: "roomExpired"}`.
Client clears stored token and can start a new game.

If the token's slot already has an active connection (e.g., two tabs), reject the second
connection with `{type: "alreadyConnected"}` — client shows "Game is open in another tab."
The active-connection check should go through Room.Run() via the reconnect channel
response (not by peeking at state from the handler), keeping Room.Run() as the single
owner of player state.

**Lobby disconnect:** If a player disconnects while waiting in the lobby (before being
paired), they have no room and no token. Use `select` with context cancellation when
sending to the lobby channel so the goroutine doesn't leak. On reconnect with no token,
they simply join the lobby again as a fresh player.

Both paths converge into the same read/write loop, attached to the room's player slot.

### Step 4: Client (JS)

- Store `{token, color}` in **localStorage** (not sessionStorage — sessionStorage
  doesn't survive tab close, which breaks the core use case)
- On page load: if token exists in localStorage, send reconnect message
- If reconnect fails, clear localStorage, send join
- **Reconnect triggers:** Both `WebSocket.onclose`/`onerror` AND `visibilitychange`
  (page becomes visible → check if WS is alive) should trigger reconnect. Without
  `visibilitychange`, mobile feels broken (browser can take 30-60s to fire `onclose`).
  Without `onclose`, WiFi drops mid-game go undetected until a tab switch.
- Reconnect with exponential backoff. After N failures, show "Could not reconnect.
  [Try again]" button.
- UI states: "Waiting for opponent..." (lobby), "Opponent reconnecting..." (opponentAway),
  "Reconnected!" (opponentReturned), "Game expired" (roomExpired),
  "Game is open in another tab" (alreadyConnected)
- Delay the "opponent reconnecting" banner by ~1s to avoid flashing on flaky connections.
  If opponent reconnects within that window, skip the banner entirely.
- Client-side opponent timeout: after ~60s of "opponent reconnecting," show "Opponent
  appears to have left. [Start a new game]" — a stopgap until server TTL exists.
- On game over: clear localStorage, offer "Play again" (fresh matchmake via lobby,
  not a rematch with same opponent)

### Follow-up (not in initial scope)

- **Room cleanup with TTL:** Timer starts when both players disconnect. Sends on a
  channel that Room.Run() selects on (timer goroutine must never directly mutate state).
  ~30 min TTL for abandoned rooms. Until then, server restart is the only cleanup.
  Add a quit channel to Room.Run() select when implementing this.
- **Room IDs / shareable URLs:** `/?room=abc` for direct join or spectating.
- **Lobby timeout:** Cancel waiting after N minutes if no opponent shows up.
- **AFK / move timeout:** Notify opponent when the other player has been idle too long.
- **Rematch:** "Play again with same opponent" (reuse room, reset game state).

### Go concepts to learn

- **Nil channel in select** — dynamically disabling select cases
- **`sync.Mutex`** — protecting the registry (shared map across goroutines)
- **`crypto/rand`** — generating secure random tokens
- **Channel of channels** — reconnect channel carries new channels as payload
- **Goroutine lifecycle** — Room.Run() as single owner of all state transitions

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

Milestone 4: Rooms & Reconnect              ⬜ NEXT
  └─ teaches: nil channel pattern, sync.Mutex, channel of channels, goroutine lifecycle

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
