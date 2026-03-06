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

## Play Online

Connect via SSH — no install needed:

```bash
ssh tramway.proxy.rlwy.net -p 17014
```

You'll be paired with the next player who connects. Features:
- Auto-pairing lobby
- Turn indicator and board flip (Black sees the board from their side)
- In-game rules screen (`?`)

## Self-Hosting

Run the SSH server with Docker:

```bash
docker build -t tic-tac-chec .
docker run -p 2222:2222 tic-tac-chec
```

Players connect with `ssh localhost -p 2222`.

To keep the host key stable across redeploys, set the `HOST_KEY_PEM` env var to an Ed25519 private key in PEM format:

```bash
ssh-keygen -t ed25519 -f host_key -N "" && rm host_key.pub
docker run -p 2222:2222 -e HOST_KEY_PEM="$(cat host_key)" tic-tac-chec
```

Without `HOST_KEY_PEM`, keys are auto-generated on startup (clients will see a host key change warning after each redeploy).

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
| ↑ ↓ ← → / h j k l | Move cursor |
| Enter / Space | Select piece / confirm move |
| ? | Rules screen |
| N | New game (after game over) |
| C | Cycle color scheme |
| S | Toggle status overlay |
| Q | Quit |
