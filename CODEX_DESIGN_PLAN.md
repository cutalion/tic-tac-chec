# Codex Design Plan

Fresh web server redesign for Tic-Tac-Chec.

This plan intentionally ignores old web/server architecture notes and starts from scratch.

## Goal

Rebuild the web server with clear subsystem boundaries and a simple external API.

Target flow:

1. Client gets identity
2. Client connects to lobby
3. Server pairs two live lobby clients
4. Server creates room
5. Game runs inside the room
6. Room and game survive WebSocket disconnects

## Scope Decision

Lobby does not survive disconnects.

If a player disconnects before being paired, remove them from matchmaking immediately.

Room and game do survive disconnects.

If a player disconnects after room creation, the room stays alive and the player can reconnect to their existing seat.

This keeps the design simple:

- no ghost players in lobby
- no lobby TTLs
- no reconnect confirmation protocol for matchmaking
- reconnect logic exists only in rooms

## Core Design Rule

WebSocket connections are ephemeral.

Room state is durable.

A room must never depend on a particular socket remaining open.

## Subsystems

### 1. Identity

Responsibility:

- issue stable client tokens
- authenticate requests and WebSocket connections

Notes:

- one token per browser/device
- token is stored by the client and reused on reconnect
- token identifies seat ownership in a room

### 2. Matchmaker

Responsibility:

- track currently connected lobby players only
- pair two active players
- remove player from queue on disconnect

Notes:

- no durable waiting queue
- if lobby socket closes before pairing, matchmaking is cancelled
- after pairing, matchmaker is no longer involved

### 3. Room Manager

Responsibility:

- create rooms
- find rooms by ID
- own room lifecycle

Notes:

- room IDs are stable
- room manager maps `roomID -> room`

### 4. Room

Responsibility:

- own the game state
- own seat assignment
- accept commands from players
- broadcast snapshots/events
- handle disconnect/reconnect

Notes:

- seat ownership is tied to `clientID`, not socket
- a room can exist with zero, one, or two connected players

### 5. Transport

Responsibility:

- HTTP handlers for REST endpoints
- WebSocket handlers for lobby and rooms
- JSON decoding/encoding
- attach/detach live connections to subsystems

Notes:

- transport should not contain game rules
- transport should not own room state

## Recommended Package Shape

One possible layout:

```text
internal/auth
internal/matchmaking
internal/room
internal/api
internal/protocol
engine
cmd/web
```

Suggested responsibility split:

- `internal/auth`: client tokens and auth helpers
- `internal/matchmaking`: lobby queue and pairing
- `internal/room`: room manager, room, seat, commands, events
- `internal/api`: HTTP and WebSocket handlers
- `internal/protocol`: request/response message types
- `engine`: pure game rules

## Durable vs Ephemeral State

### Durable

- client identity token
- room ID
- room seats
- game state
- reconnect eligibility for room participants

### Ephemeral

- lobby WebSocket connection
- room WebSocket connection
- whether a player is currently attached live

## Data Model

### Client

```go
type Client struct {
    ID string
}
```

### Lobby Participant

This exists only while the lobby WebSocket is alive.

```go
type LobbyParticipant struct {
    ClientID string
    Session  *LobbySession
}
```

### Room Seat

```go
type Seat struct {
    ClientID  string
    Color     Color
    Session   *RoomSession // nil when disconnected
    Connected bool
}
```

### Room

```go
type Room struct {
    ID    string
    White Seat
    Black Seat
    Game  *engine.Game
}
```

Important distinction:

- `ClientID` is durable seat ownership
- `Session` is current live attachment

## Runtime Flow

### 1. Client Identity

Client calls:

```text
POST /clients
```

Server returns:

```json
{"token":"..."}
```

The client stores this token and uses it in future requests.

### 2. Lobby

Client opens:

```text
GET /ws/lobby?token=...
```

Server behavior:

1. authenticate token
2. register client in matchmaker
3. send `waiting`
4. if socket closes before pairing, remove client from matchmaking

### 3. Pairing

When two live lobby clients are available:

1. matchmaker removes both from queue
2. room manager creates a room
3. one client becomes white, the other black
4. server sends `matched` message to both clients

Example:

```json
{"type":"matched","roomId":"r123","color":"white"}
```

After that, clients leave lobby flow and connect to the room.

### 4. Room Attach

Client opens:

```text
GET /ws/rooms/{roomID}?token=...
```

Server behavior:

1. authenticate token
2. load room by ID
3. verify token belongs to white or black seat
4. attach this socket to that seat
5. replace any previous live socket for that seat
6. send full room snapshot immediately

### 5. In-Game Commands

Client sends:

```json
{"type":"move","piece":"WR","to":"b3"}
```

Server room logic:

1. verify sender owns the seat
2. verify turn/rules
3. apply move through `engine`
4. broadcast full snapshot to connected players

### 6. Room Disconnect

If room WebSocket closes:

1. detach session from seat
2. mark seat disconnected
3. keep room alive
4. notify opponent that the player is away

The room must remain fully usable for reconnect.

### 7. Room Reconnect

If same client reconnects with same token:

1. server finds their seat by `clientID`
2. server attaches new session
3. server sends full snapshot
4. server notifies opponent that player is back

## Message Model

Keep message design simple and path-scoped.

Do not build a generic multi-channel socket protocol first.

Lobby socket messages:

### Server -> Client

```json
{"type":"waiting"}
{"type":"matched","roomId":"r123","color":"white"}
```

### Client -> Server

No lobby messages required in the first version.

Connection itself means "I want matchmaking".

Room socket messages:

### Client -> Server

```json
{"type":"move","piece":"WR","to":"b3"}
{"type":"rematch"}
```

### Server -> Client

```json
{"type":"snapshot", "...":"full room state"}
{"type":"opponent_status","status":"away"}
{"type":"opponent_status","status":"connected"}
{"type":"error","message":"not your turn"}
```

## Snapshot-First Strategy

On every room connect or reconnect, send a full snapshot.

Do not try to resume from missed incremental events.

This simplifies reconnection a lot:

- no replay buffer
- no event sequence IDs
- no partial synchronization logic

The room is the source of truth, so snapshot recovery is enough.

## Room Invariants

These invariants should always hold:

1. Every room has exactly two seats.
2. Each seat has exactly one owner `ClientID`.
3. A seat may have zero or one live session attached.
4. A player may reconnect only to the seat they own.
5. The game state lives inside the room, never inside the WebSocket handler.
6. Matchmaker never owns game state.
7. Disconnect in lobby cancels matchmaking.
8. Disconnect in room does not destroy the room.

## Concurrency Model

Recommended model:

- one goroutine owns the matchmaker queue
- one goroutine owns each room
- handlers communicate with them through commands/events

Why:

- clear ownership
- less locking complexity
- easier reasoning about reconnect and move ordering

Avoid spreading room state across multiple mutex-protected structs too early.

## Suggested Room Command Model

The room should accept app-level commands, not WebSocket objects.

Example:

```go
type Command interface{}

type Attach struct {
    ClientID string
    Session  *RoomSession
}

type Detach struct {
    ClientID string
}

type Move struct {
    ClientID string
    Piece    engine.Piece
    To       engine.Cell
}

type RequestRematch struct {
    ClientID string
}
```

The room should emit app-level events:

```go
type Event interface{}

type SnapshotEvent struct {
    Room RoomView
}

type OpponentStatusEvent struct {
    Status string
}

type ErrorEvent struct {
    Message string
}
```

This keeps transport isolated from game logic.

## Minimal External API

Start with only what is needed:

### REST

```text
POST /clients
```

### WebSocket

```text
GET /ws/lobby?token=...
GET /ws/rooms/{roomID}?token=...
```

This is enough for:

- identity
- live matchmaking
- durable room reconnect

More REST endpoints can come later if needed.

## Implementation Order

Start with the client identity endpoint.

### Step 1. Clients Endpoint

Implement:

```text
POST /clients
```

Requirements:

- generate a stable opaque token
- return JSON
- keep handler logic small

At this step there is no lobby or room logic yet.

### Step 2. Auth Extraction

Add shared auth helper:

- read token from request
- validate format
- convert request to `clientID`

Use the same auth path for future REST and WebSocket handlers.

### Step 3. Matchmaker

Implement:

- register connected lobby client
- unregister on disconnect
- pair two live clients

Output of pairing:

- `roomID`
- white client ID
- black client ID

### Step 4. Lobby WebSocket

Implement:

```text
GET /ws/lobby?token=...
```

Behavior:

- authenticate
- add player to matchmaker
- send `waiting`
- on pair, send `matched`
- on disconnect, remove player if still waiting

### Step 5. Room Manager

Implement:

- create room from pair result
- lookup room by ID
- keep rooms in memory

### Step 6. Room Core

Implement room state and command loop:

- durable seats
- durable `engine.Game`
- attach/detach session
- move validation
- snapshot generation

### Step 7. Room WebSocket

Implement:

```text
GET /ws/rooms/{roomID}?token=...
```

Behavior:

- authenticate
- authorize room membership
- attach session to owned seat
- send snapshot
- forward move/rematch commands to room
- detach on disconnect

### Step 8. Reconnect Tests

Add tests for:

1. lobby disconnect removes player from queue
2. matched players create room correctly
3. room survives one player disconnect
4. reconnect reattaches correct seat
5. reconnect sends fresh snapshot
6. opponent gets away/connected updates

## First Version Non-Goals

Do not build these in the first pass:

- persistent storage
- durable lobby across disconnects
- replaying missed events
- spectators
- multiple simultaneous room memberships
- analytics or admin APIs

## Summary

First version should optimize for clean ownership:

- identity is stable
- lobby is live-presence only
- room is durable
- reconnect is room-only
- handlers translate protocol, not business logic

That gives a simple architecture with clear boundaries and a clean path to incremental growth.
