# Chess Tic-Tac-Toe (Tic Tac Chec)

A pet project to learn Go — a hybrid board game combining chess piece movement with tic-tac-toe win conditions.

## Rules

- 4×4 board, 2 players: White and Black
- Each player has 4 pieces: Pawn, Rook, Bishop, Knight
- On your turn: **place** a piece from hand onto any empty cell, or **move** a piece already on the board (chess-style movement)
- Capturing a piece returns it to its **owner's** hand (shogi-style)
- Pawns reverse direction when reaching the far edge
- **Win**: get 4 of your pieces in a row — horizontal, vertical, or diagonal

## How to Run

```bash
go run ./cmd/tui/
```

## Controls

| Key | Action |
|-----|--------|
| ↑ ↓ ← → | Move cursor |
| Enter | Select piece / confirm move |
| Esc | Deselect |
| N | New game (after game over) |
| C | Cycle color scheme |
| S | Toggle status overlay |
| Q | Quit |
