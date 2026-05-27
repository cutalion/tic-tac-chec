# Project: Chess Tic-Tac-Toe

## User Preferences
- User is learning Go — they write ALL the code themselves
- Claude should explain concepts, architecture, and reasoning but NOT write code for the user
- User's background: Ruby, JS/TS, PHP, Python; experienced programmer learning Go deeply (not just syntax)

## Game Rules
- 4x4 board, 2 players (White/Black)
- Each player has 4 chess pieces: Pawn, Rook, Bishop, Knight
- On a turn: place a piece from hand onto any empty cell, OR move a piece on the board (chess-style movement)
- Capturing returns the captured piece to its owner's hand (shogi-style)
- Pawns reverse direction when reaching the far edge (no promotion)
- Win: 4 of your color in a row (horizontal, vertical, or diagonal)

## Development Approach
- Outside-in TDD: start from high-level use cases, let failing tests drive creation of lower-level types and functions
- For new packages/modules: build a simplified MVP first, then extend incrementally
- Avoid building infrastructure speculatively — only add what a failing test demands

## Architecture Plan
- `engine/` — pure game logic, no I/O
- `cmd/cli/` — simple console interface (first milestone)
- Later: TUI (bubbletea), HTTP server, remote terminal client

## Active Technologies
- Go 1.25+ (server), vanilla JS (frontend) + `github.com/coder/websocket` (existing) (001-emoji-reactions)
- N/A (emojis are ephemeral, not persisted) (001-emoji-reactions)
- Python 3.10+ (training), Go 1.25+ (inference & server) + PyTorch (training), `github.com/yalue/onnxruntime_go` v1.27+ (Go inference) (002-rl-bot-training)
- File-based (ONNX model files, training checkpoints) (002-rl-bot-training)
- HTML/CSS/JS (vanilla) for the frontend; Go 1.25+ for the server (no server changes anticipated beyond, at most, registering a `/fonts/` static path or adding a single `go:embed` glob line if font files need embedding in prod builds — dev already serves from `cmd/web/static/` via `http.FileServer`). + No new runtime dependencies. Fraunces (OFL 1.1) and Inter (OFL 1.1) WOFF2 files bundled as static assets. (005-mockup-restyle)
- N/A (purely presentational; no persistence changes). (005-mockup-restyle)
- Go 1.25+ (server); HTML/CSS/vanilla JS (`internal/web/static/`). + `database/sql`, `modernc.org/sqlite` (existing); `github.com/coder/websocket` unchanged. No new third-party Go deps required for MVP. (006-profile-stats-view)
- SQLite (`games`, `players`, `users`, `bots`) — **no schema migrations** for this feature; new **read** queries + indexes optional only if profiling demands (YAGNI). (006-profile-stats-view)

## Recent Changes
- 001-emoji-reactions: Added Go 1.25+ (server), vanilla JS (frontend) + `github.com/coder/websocket` (existing)
- 006-profile-stats-view: Profile + stats API and UI (read-only SQLite; authenticated JSON; `internal/web/static`); spec/plan in `specs/006-profile-stats-view/`
