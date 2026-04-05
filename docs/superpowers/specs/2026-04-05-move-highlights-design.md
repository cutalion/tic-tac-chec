# Move Highlights — Web UI

Display legal moves when a board piece is selected. No highlighting for hand pieces (any empty cell is valid).

## Approach

Client-side move computation — duplicate the Go engine's move logic in JavaScript. Pure view concern: moves are computed during render, not stored in state.

## Move Computation Functions (app.js)

Port of `engine/moves.go`. All functions are pure — no side effects, no state mutation.

- `computeMoves(board, piece, row, col, pawnDirections)` — dispatcher by piece kind, returns `[{row, col, capture}]`
- `rookMoves(board, color, row, col)` — slide in 4 cardinal directions
- `bishopMoves(board, color, row, col)` — slide in 4 diagonals
- `knightMoves(board, color, row, col)` — 8 L-shaped jumps
- `pawnMoves(board, color, row, col, direction)` — 1 forward if empty, 2 diagonal captures
- `slideMoves(board, color, row, col, directions)` — shared helper for rook/bishop, walks each direction until blocked or capture
- `canMoveTo(board, color, row, col)` — returns `{allowed, capture}`, checks bounds and occupancy

### Pawn directions

The server sends `pawnDirections` in `gameState` messages (wire format: `"toBlackSide"` / `"toWhiteSide"`). Store as `state.pawnDirections`. Map to row deltas: `toBlackSide` = `-1`, `toWhiteSide` = `+1`.

All move computation uses engine coordinates. The board flip for the black player only affects display order — engine row/col on cells are unchanged, so move logic is unaffected.

## Rendering (app.js — renderBoard)

When `state.selectedPiece` exists with `source === "board"`:

1. Find the selected piece's engine position by scanning `state.board` for a matching piece code
2. Call `computeMoves()` once at the top of `renderBoard()`
3. For each board cell, check membership in the moves list:
   - Non-capture target: add `.target` class (green border)
   - Capture target: add `.target` class + reduce enemy piece glyph opacity

When `source === "hand"` or no piece selected: no move highlighting.

## CSS (style.css)

One new rule for the capture indicator:

```css
.board-cell.target .piece-glyph {
  opacity: 0.5;
}
```

The `.board-cell.target` rule already exists, using `var(--target)` for the border color (dark: `#66cc99`, light: `#186830`).

## State Changes

Add `state.pawnDirections` — set from `data.state.pawnDirections` in the `gameState` handler. Clear in `resetBoardState()`.

No other state changes. `possibleMoves` is not stored — computed in render.

## Scope

- Web frontend only (`cmd/web/static/app.js`, `cmd/web/static/style.css`)
- No server/engine changes
- No HTML changes
- No TUI changes
- No new WebSocket messages or API endpoints
