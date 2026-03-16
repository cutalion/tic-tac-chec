# Online Mode Architecture: Goroutines & Channels

## Overview

In online mode, each player runs a Bubble Tea program connected to a shared
Room via channels. The Room is the game authority — it validates moves, updates
state, and broadcasts results.

## Component Diagram

```
 Player 1 (SSH session)              Player 2 (SSH session)
┌──────────────────────┐            ┌──────────────────────┐
│   Bubble Tea Model   │            │   Bubble Tea Model   │
│                      │            │                      │
│  Moves (chan<-)  ────┼──┐    ┌────┼──── Moves (chan<-)   │
│  Incoming (<-chan) ◄─┼┐ │    │ ┌──┼──► Incoming (<-chan) │
└──────────────────────┘│ │    │ │  └──────────────────────┘
                        │ │    │ │
                        │ ▼    ▼ │
                      ┌─┴────────┴──┐
                      │    Room     │
                      │  (goroutine)│
                      │             │
                      │  Game state │
                      │  Move logic │
                      └─────────────┘
```

## Channel Types

All channels are **unbuffered**. This means every send blocks until the
receiver is ready, and vice versa.

- `Moves (chan MoveRequest)` — Player → Room. Model sends a move, Room reads it.
- `Incoming (chan tea.Msg)` — Room → Player. Room sends GameStateMsg/ErrorMsg,
  Model's waitForIncoming() reads it.

## Message Flow: A Single Move

```
1. Player presses Enter
   └─► Update() calls executeMove()
       └─► Returns tea.Cmd (a closure)

2. Bubble Tea runs the Cmd in a goroutine:
   └─► Closure sends MoveRequest on Moves channel
       └─► Room.Run() receives it via select{}
           └─► Room calls engine.Game.Move()
               ├─► Error? Send ErrorMsg on mover's Incoming
               └─► Success? Send GameStateMsg on BOTH players' Incoming

3. The same closure reads from Incoming (the Room's response)
   └─► Returns the tea.Msg to Bubble Tea
       └─► Bubble Tea calls Update(GameStateMsg)
           └─► Model updates game state, returns nextCmd()
               └─► New waitForIncoming goroutine spawned
```

## The Stale Listener Problem

### How Bubble Tea Commands Work

Every time Update() returns a `tea.Cmd`, Bubble Tea runs it in a **new
goroutine**. There is no way to cancel a previously returned Cmd.

### What Happens

In online mode, `nextCmd()` returns `waitForIncoming()` on every Update return.
Each call spawns a goroutine that blocks on `<-incoming`:

```
Key press "up"    → Update returns nextCmd() → goroutine #1 blocks on <-incoming
Key press "down"  → Update returns nextCmd() → goroutine #2 blocks on <-incoming
Key press "left"  → Update returns nextCmd() → goroutine #3 blocks on <-incoming
GameStateMsg arrives on incoming   → goroutine #1 receives it (others stay blocked)
Key press "right" → Update returns nextCmd() → goroutine #4 blocks on <-incoming
...
```

Multiple goroutines compete on the same channel. Only one receives each message.
The rest remain blocked until the channel closes.

### Why We Can't Avoid It

Bubble Tea's architecture requires returning a `tea.Cmd` from `Update()` to
schedule async work. There is no API to:
- Cancel a previously returned Cmd
- Reuse an existing listener goroutine
- Signal "no new Cmd, but keep the old one alive"

Returning `nil` would stop listening entirely (the UI freezes). So we must
return a new listener every time, even though the old one is still alive.

### Why It's Acceptable

- Each blocked goroutine costs ~2-4KB of stack memory
- A typical game has ~50-100 key presses → ~200-400KB overhead
- All stale goroutines are cleaned up when the SSH session ends (channels close,
  goroutines wake up and return OpponentDisconnectedMsg or exit)
- The leak is **per-session**, not per-server — fixed by our Room.Run() exiting
  on GameOver and channel close

### The executeMove Exception

executeMove() is special: its Cmd sends a move AND reads the response in a
single closure. This avoids spawning an extra stale listener — without this,
the Cmd would return nil after sending, and a stale waitForIncoming goroutine
would accidentally pick up the Room's response. It would work, but it's
fragile and wastes a goroutine.

## Room Lifecycle

```
Room.Run() goroutine:
  for {
    select on Player[0].Moves and Player[1].Moves
    ├─► Move received → validate → broadcast state
    │   └─► GameOver? → return (exit goroutine)
    └─► Channel closed (!ok) → notify other player → return
  }
```

Room.Run() exits when:
1. Game ends (GameOver after broadcasting final state)
2. A player disconnects (Moves channel closed)

Before the fix, Room.Run() looped forever after GameOver — a goroutine leak
that accumulated across games on a long-running server.
