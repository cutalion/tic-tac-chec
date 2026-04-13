# Tic-Tac-Chec WebSocket Protocol

## Overview

Tic-Tac-Chec is a 2-player board game on a 4x4 grid combining chess piece movement with tic-tac-toe win conditions. This document specifies the WebSocket protocol for playing the game programmatically.

**Server:** `ttc.ctln.pw`

## Game Rules

- **Board:** 4x4 grid. Columns a-d, rows 1-4. Row 1 = White's home side (bottom), row 4 = Black's home side (top).
- **Players:** White and Black. White moves first.
- **Pieces:** Each player has 4 chess pieces: Pawn (P), Rook (R), Bishop (B), Knight (N).
- **Turn actions:** Either **place** a piece from hand onto any empty cell, OR **move** a piece already on the board using standard chess movement rules.
- **Capturing:** Landing on an opponent's piece captures it. The captured piece returns to its **owner's hand** (shogi-style) and can be placed again on a future turn.
- **Pawn movement:** Pawns move one square forward (no initial double move). They capture diagonally forward. When a pawn reaches the far edge, its direction reverses (no promotion).
- **Rook movement:** Slides horizontally or vertically, any number of squares (blocked by pieces, can capture).
- **Bishop movement:** Slides diagonally, any number of squares (blocked by pieces, can capture).
- **Knight movement:** Standard chess L-shape (2+1), jumps over pieces, can capture.
- **Win condition:** Get all 4 of your pieces in a row — horizontal, vertical, or diagonal.

## Authentication

All WebSocket and API endpoints require a client token.

### Obtain a Token

```
POST /api/clients
→ 201 {"token": "<client-id>"}
```

### Use the Token

Either method works for all authenticated endpoints:
- Query parameter: `?token=<client-id>`
- HTTP header: `Authorization: Bearer <client-id>`

### Verify Token

```
GET /api/me?token=<client-id>
→ 200 {"token": "<client-id>"}
```

## Starting a Game

### Option A: Bot Game

```
POST /api/bot-game?token=<token>&difficulty=easy|medium|hard
→ 201 {"roomId": "<room-id>"}
```

Then connect to the room WebSocket (see below).

### Option B: Matchmaking (Lobby)

1. Connect to lobby WebSocket:
   ```
   GET /ws/lobby?token=<token>       (default lobby)
   GET /ws/lobby/<id>?token=<token>  (specific lobby)
   ```

2. Receive waiting confirmation:
   ```json
   {"type": "waiting"}
   ```

3. When a second player joins, both receive:
   ```json
   {"type": "paired", "roomId": "<room-id>"}
   ```

4. Disconnect from lobby, connect to room.

### Option C: Private Lobby

```
POST /api/lobbies
→ 201 {"id": "<lobby-id>"}
```

Share the lobby ID. Both players connect to `/ws/lobby/<id>`.

## Room Connection

```
GET /ws/room/<room-id>?token=<token>
```

On connect, you receive:

```json
{"type": "roomJoined", "roomId": "<room-id>", "color": "white"}
```

Followed immediately by the initial game state.

## Game State

Sent after every move and on room join:

```json
{
  "type": "gameState",
  "state": {
    "board": [
      [null, null, null, null],
      [null, {"color": "white", "kind": "pawn"}, null, null],
      [null, null, null, null],
      [null, null, null, null]
    ],
    "turn": "white",
    "status": "started",
    "winner": null,
    "pawnDirections": {
      "white": "toBlackSide",
      "black": "toWhiteSide"
    }
  }
}
```

### Board Layout

The board is a 4x4 array: `board[row][col]`.

```
         col 0 (a)  col 1 (b)  col 2 (c)  col 3 (d)
row 0 →   a4         b4         c4         d4        ← Black's side (rank 4)
row 1 →   a3         b3         c3         d3
row 2 →   a2         b2         c2         d2
row 3 →   a1         b1         c1         d1        ← White's side (rank 1)
```

Each cell is either `null` (empty) or `{"color": "white"|"black", "kind": "pawn"|"rook"|"bishop"|"knight"}`.

### Determining Hand Pieces

Pieces not on the board are in the player's hand. Each player always has exactly 4 pieces (pawn, rook, bishop, knight). Scan the board to find which are placed; the rest are in hand and available for placement.

### Pawn Directions

- `"toBlackSide"` — pawn moves toward row 0 (upward in the array).
- `"toWhiteSide"` — pawn moves toward row 3 (downward in the array).

White's pawn starts moving `toBlackSide`. When it reaches row 0, direction flips to `toWhiteSide`. When it reaches row 3, direction flips back. Same logic for Black's pawn. If a pawn is captured and returns to hand, its direction resets to the initial value.

### Game Status

- `"started"` — game in progress.
- `"over"` — game finished. Check `winner` field for `"white"` or `"black"`.

## Making Moves

Send a move message:

```json
{"type": "move", "piece": "WP", "to": "b3"}
```

### Piece Codes

| Code | Piece         |
|------|---------------|
| WP   | White Pawn    |
| WR   | White Rook    |
| WB   | White Bishop  |
| WN   | White Knight  |
| BP   | Black Pawn    |
| BR   | Black Rook    |
| BB   | Black Bishop  |
| BN   | Black Knight  |

### Square Notation

Chess-style: file (a-d) + rank (1-4). Examples: `a1` (bottom-left, White's side), `d4` (top-right, Black's side).

Mapping to board array indices:
- File: a=col 0, b=col 1, c=col 2, d=col 3
- Rank: 4=row 0, 3=row 1, 2=row 2, 1=row 3

### Move Semantics

The server automatically determines whether a move is a **placement** or a **board move**:
- If the piece is in hand → places it on the target cell (must be empty).
- If the piece is on the board → moves it to the target cell (must be a legal chess move; can capture opponent pieces).

### Error Response

If a move is invalid:

```json
{"type": "error", "error": "White Pawn can't move there — illegal move"}
```

## Other Messages

### Rematch

After a game ends, either player can request a rematch:

```json
{"type": "rematch"}
```

The opponent receives:

```json
{"type": "rematchRequested"}
```

When both players request rematch, both receive:

```json
{"type": "rematchStarted", "color": "white"}
```

Colors may swap. A new game state follows.

### Reactions

Send an emoji reaction:

```json
{"type": "reaction", "reaction": "👍"}
```

Opponent receives:

```json
{"type": "reaction", "reaction": "👍", "player": "white", "roomId": "<room-id>"}
```

### Connection Events

```json
{"type": "opponentAway"}
{"type": "opponentDisconnected"}
{"type": "opponentReconnected"}
```

## Full Game Example

```
1. POST /api/clients → {"token": "abc123"}
2. POST /api/bot-game?token=abc123&difficulty=easy → {"roomId": "room-xyz"}
3. WS /ws/room/room-xyz?token=abc123
   ← {"type": "roomJoined", "roomId": "room-xyz", "color": "white"}
   ← {"type": "gameState", "state": {"board": [[null,null,null,null],[null,null,null,null],[null,null,null,null],[null,null,null,null]], "turn": "white", "status": "started", "winner": null, "pawnDirections": {"white": "toBlackSide", "black": "toWhiteSide"}}}
   → {"type": "move", "piece": "WR", "to": "b2"}
   ← {"type": "gameState", "state": {"board": [...], "turn": "black", ...}}
   ... (alternating moves)
   ← {"type": "gameState", "state": {"board": [...], "turn": "white", "status": "over", "winner": "white", ...}}
```
