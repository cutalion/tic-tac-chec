# Chess Tic-Tac-Toe (Tic Tac Chec)

A pet project to learn Go — a hybrid board game combining chess piece movement with tic-tac-toe win conditions.

## Rules

- 4×4 board, 2 players: White and Black
- Each player has 4 pieces: Pawn, Rook, Bishop, Knight
- On your turn: **place** a piece from hand onto any empty cell, or **move** a piece already on the board (chess-style movement)
- Capturing a piece returns it to its **owner's** hand (shogi-style)
- Pawns reverse direction when reaching the far edge
- **Win**: get 4 of your pieces in a row — horizontal, vertical, or diagonal

## Example

![Gameplay](play.png)

## How to Run

```bash
go run ./cmd/tui/
```

## Claude Code Skill

Play against Claude in your terminal using the [Claude Code](https://docs.anthropic.com/en/docs/claude-code) skill.

### Requirements

- Go 1.25+
- [Claude Code](https://docs.anthropic.com/en/docs/claude-code) CLI

### Install

```bash
make install-skill
```

Then restart Claude Code and say `/play-tic-tac-chec`.

The skill self-improves: when Claude loses, it analyzes the game and updates its strategy in `~/.claude/skills/play-tic-tac-chec/SKILL.md`.

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
