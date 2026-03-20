# Web API Design (Draft)

Redesign of the web layer: separate API server (Go) from SPA client,
with a clean REST + WebSocket protocol reusable by mobile apps.

## Architecture

```
Browser (SPA)  ──►  Caddy  ──►  Go API server
Mobile app     ──►          ──►  (same API)

Caddy routes:
  /api/*          → reverse_proxy go-server:8080
  /api/*/ws       → reverse_proxy go-server:8080 (WebSocket)
  /*              → file_server /srv/static (SPA)
  /room/*         → rewrite / → file_server (SPA catch-all)
  /lobby          → rewrite / → file_server (SPA catch-all)
```

Go server is a pure API server — no HTML, no static files, no embed.FS.

SPA handles client-side routing:
- `/` — landing page (project info, rules, links to play)
- `/lobby` — auto-match with strangers
- `/room/:id` — game room

---

## Client Identity

A **client token** identifies a player across all rooms and sessions.

- Issued once on first interaction, stored in localStorage forever
- Sent as `Authorization: Bearer <token>` header on REST requests
- Sent as query param on WebSocket: `/api/rooms/:id/ws?token=<token>`
- Format: 32 random bytes, base62-encoded (~43 chars)
- A client can be in multiple rooms simultaneously

### Token Lifecycle

```
First visit → POST /api/clients → {token} → localStorage
All subsequent requests include token
Token lost → new token → old games inaccessible (acceptable)
```

Client creation is explicit — the SPA calls `POST /api/clients` on first
visit and stores the token. All subsequent API calls require it.

---

## Room IDs

- UUIDv7, base62-encoded (~22 chars)
- Time-sortable, no collision handling needed
- URL: `/room/5vGHs0dMwCqiVNhTk0eRsP`

---

## REST API

All endpoints under `/api/`. Request/response bodies are JSON.
Auth via `Authorization: Bearer <token>` where noted.

### Clients

#### `POST /api/clients`

Create a new anonymous client identity.

**Response** `201 Created`:
```json
{
  "token": "5vGHs0dMwCqiVNhTk0eRsP..."
}
```

### Rooms

#### `POST /api/rooms`

Create a new room. Creator auto-joins as a player.

**Auth:** required

**Response** `201 Created`:
```json
{
  "roomId": "2kF9xQm4vR7nJpLw3hBcYt",
  "color": "white",
  "status": "waiting"
}
```

#### `GET /api/rooms/:id`

Get room info. No auth required (public info for the join screen).

**Response** `200 OK`:
```json
{
  "roomId": "2kF9xQm4vR7nJpLw3hBcYt",
  "status": "waiting|playing|finished",
  "players": {
    "white": {"connected": true},
    "black": null
  },
  "canJoin": true
}
```

`canJoin` is true when: room has an empty slot AND the requesting client
is not already in this room. If the client is already a participant,
response includes their `color` instead:

```json
{
  "roomId": "...",
  "status": "playing",
  "players": {
    "white": {"connected": true},
    "black": {"connected": false}
  },
  "canJoin": false,
  "yourColor": "black"
}
```

**`404 Not Found`** if room doesn't exist.

#### `POST /api/rooms/:id/join`

Join an existing room as the second player.

**Auth:** required

**Response** `200 OK`:
```json
{
  "color": "black"
}
```

**Error responses:**
- `404` — room not found
- `409` — room is full
- `409` — you're already in this room

---

## WebSocket API

### Lobby

#### `GET /api/lobby/ws?token=<token>`

Connect to auto-matchmaking. Server pairs you with the next available player.

**Server → Client:**

```json
{"type": "waiting"}
```
Sent immediately on connect. Client shows "Waiting for opponent..."

```json
{"type": "matched", "roomId": "2kF9xQm4vR7nJpLw3hBcYt", "color": "white"}
```
Matched with an opponent. Client redirects to `/room/:roomId`.
Server closes the WebSocket after sending this.

**Client → Server:**

No messages. Connection itself is the "I want to play" signal.
Client can close the WebSocket to cancel matchmaking.

### Room

#### `GET /api/rooms/:id/ws?token=<token>`

Connect to a game room for real-time play.

**Prerequisite:** Client must be a participant in the room (via create or join).
If not a participant, server closes with `4403 Forbidden`.
If room doesn't exist, server closes with `4404 Not Found`.

#### Server → Client Messages

**`gameState`** — Full game state snapshot. Sent on connect, after each move,
and after rematch start.

```json
{
  "type": "gameState",
  "color": "white",
  "state": {
    "board": [
      [null, null, null, null],
      [null, {"color": "white", "kind": "rook"}, null, null],
      [null, null, null, null],
      [null, null, null, null]
    ],
    "turn": "black",
    "status": "playing",
    "winner": null,
    "winLine": null
  }
}
```

**`error`** — Invalid move or other error.

```json
{"type": "error", "message": "not your turn"}
```

**`opponentStatus`** — Opponent connection state changed.

```json
{"type": "opponentStatus", "status": "connected|away|disconnected"}
```

- `away` — opponent's WebSocket closed, may reconnect
- `disconnected` — opponent left permanently (future: after TTL)
- `connected` — opponent (re)connected

**`rematchRequested`** — Opponent wants a rematch.

```json
{"type": "rematchRequested"}
```

**`rematchStarted`** — Both players accepted. New game begins.

```json
{"type": "rematchStarted", "color": "black"}
```

Colors swap on rematch. Followed by a `gameState` message.

#### Client → Server Messages

**`move`** — Make a move.

```json
{"type": "move", "piece": "WR", "to": "b3"}
```

`piece`: color+kind code (`WP`, `WR`, `WB`, `WN`, `BP`, `BR`, `BB`, `BN`).
`to`: target square in algebraic notation (`a1`–`d4`).

A move is either a placement (piece from hand) or a board move (piece
already on the board moves to target). The server resolves which based
on whether the piece is in hand or on the board.

**`rematch`** — Request a rematch after game ends.

```json
{"type": "rematch"}
```

#### Presence Messages (bidirectional)

Relayed to the opponent as-is by the server. Not persisted, not validated
beyond basic structure. Server adds no fields — just forwards.

**`cursor`** — Cell the player is hovering/selecting.

```json
{"type": "cursor", "cell": "b3"}
```

`cell` is `null` when deselected. Sent on cursor move, throttled client-side
(~100ms). Server relays to opponent only.

**`emoji`** — Reaction emoji.

```json
{"type": "emoji", "emoji": "👍"}
```

Client displays the emoji as a bubble animation on the opponent's side.
Server relays to opponent only. Rate-limited server-side (e.g., max 3/sec).

These messages follow a **relay pattern**: the server forwards them to the
other player in the room. Two levels of handling:

- **Known types** (`cursor`, `emoji`): server validates structure and content
  (e.g., emoji must be in the allow-list). Invalid messages are dropped silently.
- **Unknown types**: relayed as-is with just a size limit (e.g., 256 bytes).
  This allows clients to experiment with new presence features without
  server changes.

---

## Data Model (Server)

```
Client {
  Token    string            // base62, primary key
  Rooms    map[roomId]*Room  // rooms this client participates in
}

Room {
  ID       string            // UUIDv7 base62
  Status   waiting|playing|finished
  White    *Slot
  Black    *Slot
  Game     engine.Game
}

Slot {
  Client   *Client
  State    connected|away|disconnected
  Channels (moves, updates, rematch, reconnect)
}
```

**Ownership:** Room.Run() goroutine owns all mutable room state.
External access (REST handlers) reads only immutable/atomic fields
or communicates via channels.

---

## Migration from Current Design

**What changes:**
- Token model: per-room tokens → single client token
- Routing: single `/ws` → separate REST endpoints + typed WS endpoints
- Lobby: channel in WS handler → dedicated `/api/lobby/ws`
- Static serving: embed.FS in Go → Caddy serves SPA separately
- Room creation: implicit (lobby pairs) → explicit `POST /api/rooms` + lobby

**What stays:**
- Room.Run() goroutine with select (core pattern unchanged)
- Channel-based state ownership (CSP)
- Nil channel pattern for disconnects
- Engine is untouched

**Internal refactoring needed:**
- `cmd/web/` → pure API server (no static files, no embed.FS)
- `participants.go` → `clients.go` (client registry, not per-room tokens)
- New: room registry (map[roomId]*Room)
- Room.Run() player slots need to reference Client instead of raw channels
- Lobby becomes its own goroutine writing to a response channel per waiter

---

## Open Questions

1. ~~**Hands in gameState**~~ — **No.** Clients derive hands from the board
   (full piece set minus pieces on board). Keeps wire format minimal.

2. **Spectators** — should `/api/rooms/:id/ws` without a token allow
   read-only spectating? Defer for now?

3. **Client expiry** — clients with no active rooms for >N days?
   Not needed yet (server restart clears everything).

4. **Rate limiting** — room creation spam? Defer until it's a problem.

5. **Room list** — `GET /api/rooms` to list open rooms? Or keep discovery
   through lobby + direct links only?
