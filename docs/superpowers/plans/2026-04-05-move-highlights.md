# Move Highlights Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Highlight legal moves on the web board when a piece on the board is selected; show a capture indicator on enemy pieces.

**Architecture:** Client-side JS port of the Go engine's move logic. Computed during render (pure view), not stored in state. One new state field (`pawnDirections`) from server data.

**Tech Stack:** Vanilla JS (app.js), CSS (style.css). No build tools, no frameworks.

---

## File Map

- **Modify:** `cmd/web/static/app.js` — add move computation functions, wire up pawnDirections state, update renderBoard to apply highlights
- **Modify:** `cmd/web/static/style.css` — add capture indicator rule

---

### Task 1: Add `canMoveTo` helper

**Files:**
- Modify: `cmd/web/static/app.js` (add function before `cellNotation`, around line 971)

- [ ] **Step 1: Write `canMoveTo`**

Add this function to `app.js`, before `cellNotation`:

```javascript
function canMoveTo(board, color, row, col) {
  if (row < 0 || row > 3 || col < 0 || col > 3) {
    return { allowed: false, capture: false };
  }
  const piece = board[row][col];
  if (!piece) {
    return { allowed: true, capture: false };
  }
  if (piece.color !== color) {
    return { allowed: true, capture: true };
  }
  return { allowed: false, capture: false };
}
```

- [ ] **Step 2: Verify in browser console**

Open the game in a browser. In the console, after a `gameState` arrives:

```javascript
canMoveTo(state.board, "white", 0, 0)
```

Expected: `{allowed: true, capture: false}` (empty cell) or `{allowed: false, capture: false}` (own piece), depending on board state.

- [ ] **Step 3: Commit**

```bash
git add cmd/web/static/app.js
git commit -m "web: add canMoveTo helper for client-side move computation"
```

---

### Task 2: Add `slideMoves` helper

**Files:**
- Modify: `cmd/web/static/app.js` (add after `canMoveTo`)

- [ ] **Step 1: Write `slideMoves`**

Add after `canMoveTo`:

```javascript
function slideMoves(board, color, row, col, directions) {
  const moves = [];
  for (const [dr, dc] of directions) {
    let r = row;
    let c = col;
    for (let i = 0; i < 3; i++) {
      r += dr;
      c += dc;
      const result = canMoveTo(board, color, r, c);
      if (result.allowed) {
        moves.push({ row: r, col: c, capture: result.capture });
        if (result.capture) break;
      } else {
        break;
      }
    }
  }
  return moves;
}
```

Note: the loop runs at most 3 times (`BoardSize - 1`), matching the Go engine's `range BoardSize - 1`.

- [ ] **Step 2: Commit**

```bash
git add cmd/web/static/app.js
git commit -m "web: add slideMoves helper for rook/bishop move computation"
```

---

### Task 3: Add piece-specific move functions

**Files:**
- Modify: `cmd/web/static/app.js` (add after `slideMoves`)

- [ ] **Step 1: Write `rookMoves`**

```javascript
function rookMoves(board, color, row, col) {
  return slideMoves(board, color, row, col, [
    [0, 1], [0, -1], [-1, 0], [1, 0],
  ]);
}
```

- [ ] **Step 2: Write `bishopMoves`**

```javascript
function bishopMoves(board, color, row, col) {
  return slideMoves(board, color, row, col, [
    [-1, 1], [1, 1], [1, -1], [-1, -1],
  ]);
}
```

- [ ] **Step 3: Write `knightMoves`**

```javascript
function knightMoves(board, color, row, col) {
  const moves = [];
  const jumps = [
    [-2, -1], [-2, 1], [2, -1], [2, 1],
    [-1, -2], [-1, 2], [1, -2], [1, 2],
  ];
  for (const [dr, dc] of jumps) {
    const result = canMoveTo(board, color, row + dr, col + dc);
    if (result.allowed) {
      moves.push({ row: row + dr, col: col + dc, capture: result.capture });
    }
  }
  return moves;
}
```

- [ ] **Step 4: Write `pawnMoves`**

The `direction` parameter is a row delta: `-1` (toward row 0 / black side) or `+1` (toward row 3 / white side).

```javascript
function pawnMoves(board, color, row, col, direction) {
  const moves = [];
  const forwardRow = row + direction;

  if (forwardRow >= 0 && forwardRow <= 3 && !board[forwardRow][col]) {
    moves.push({ row: forwardRow, col: col, capture: false });
  }

  for (const dc of [-1, 1]) {
    const captureCol = col + dc;
    if (forwardRow < 0 || forwardRow > 3 || captureCol < 0 || captureCol > 3) continue;
    const target = board[forwardRow][captureCol];
    if (target && target.color !== color) {
      moves.push({ row: forwardRow, col: captureCol, capture: true });
    }
  }

  return moves;
}
```

- [ ] **Step 5: Commit**

```bash
git add cmd/web/static/app.js
git commit -m "web: add rook, bishop, knight, pawn move functions"
```

---

### Task 4: Add `computeMoves` dispatcher

**Files:**
- Modify: `cmd/web/static/app.js` (add after `pawnMoves`)

- [ ] **Step 1: Write `computeMoves`**

```javascript
function computeMoves(board, piece, row, col, pawnDirections) {
  switch (piece.kind) {
    case "rook":   return rookMoves(board, piece.color, row, col);
    case "bishop": return bishopMoves(board, piece.color, row, col);
    case "knight": return knightMoves(board, piece.color, row, col);
    case "pawn":   return pawnMoves(board, piece.color, row, col, pawnDirections);
    default:       return [];
  }
}
```

The `pawnDirections` parameter is already a numeric row delta (`-1` or `+1`) — the mapping from the server's string format happens elsewhere (Task 5).

- [ ] **Step 2: Commit**

```bash
git add cmd/web/static/app.js
git commit -m "web: add computeMoves dispatcher"
```

---

### Task 5: Store pawn directions from server

**Files:**
- Modify: `cmd/web/static/app.js` — `state` object (line 18), `handleRoomMessage` (line 321), `resetBoardState` (line 915)

- [ ] **Step 1: Add `pawnDirections` to state**

In the `state` object (around line 18), add after `selectedPiece: null,`:

```javascript
  pawnDirections: null,
```

- [ ] **Step 2: Store pawn directions in `handleRoomMessage`**

In the `case "gameState":` handler (around line 322), add after `state.winner = data.state.winner;`:

```javascript
      state.pawnDirections = data.state.pawnDirections;
```

The server sends `{ white: "toBlackSide", black: "toWhiteSide" }`. This is stored as-is; the mapping to numeric deltas happens at use sites.

- [ ] **Step 3: Clear in `resetBoardState`**

In `resetBoardState()` (around line 915), add after `state.winner = null;`:

```javascript
  state.pawnDirections = null;
```

- [ ] **Step 4: Commit**

```bash
git add cmd/web/static/app.js
git commit -m "web: store pawnDirections from gameState messages"
```

---

### Task 6: Add pawn direction mapping helper

**Files:**
- Modify: `cmd/web/static/app.js` (add before `computeMoves`)

- [ ] **Step 1: Write `pawnDirection`**

This converts the server's string format to a numeric row delta for the given color:

```javascript
function pawnDirection(pawnDirections, color) {
  const dir = pawnDirections[color];
  return dir === "toBlackSide" ? -1 : 1;
}
```

- [ ] **Step 2: Update `computeMoves` to use it**

Change the pawn case in `computeMoves`:

```javascript
    case "pawn":   return pawnMoves(board, piece.color, row, col, pawnDirection(pawnDirections, piece.color));
```

- [ ] **Step 3: Commit**

```bash
git add cmd/web/static/app.js
git commit -m "web: add pawn direction string-to-delta mapping"
```

---

### Task 7: Add `findPiecePosition` helper

**Files:**
- Modify: `cmd/web/static/app.js` (add before `computeMoves`)

- [ ] **Step 1: Write `findPiecePosition`**

Scans the board to find a piece matching the given code (e.g., `"WR"` for white rook). Each piece is unique, so there's at most one match.

```javascript
function findPiecePosition(board, code) {
  for (let row = 0; row < 4; row++) {
    for (let col = 0; col < 4; col++) {
      const piece = board[row][col];
      if (piece && PIECE_CODES[piece.color][piece.kind] === code) {
        return { row, col };
      }
    }
  }
  return null;
}
```

- [ ] **Step 2: Commit**

```bash
git add cmd/web/static/app.js
git commit -m "web: add findPiecePosition helper"
```

---

### Task 8: Wire up highlights in `renderBoard`

**Files:**
- Modify: `cmd/web/static/app.js` — `renderBoard` function (line 654)

- [ ] **Step 1: Compute moves at the top of `renderBoard`**

At the beginning of `renderBoard(flipped)`, after the `board` element is created (after line 656), add:

```javascript
  let moves = [];
  if (state.selectedPiece && state.selectedPiece.source === "board") {
    const pos = findPiecePosition(state.board, state.selectedPiece.code);
    if (pos) {
      moves = computeMoves(state.board, state.selectedPiece, pos.row, pos.col, state.pawnDirections);
    }
  }
```

- [ ] **Step 2: Apply `.target` class and capture indicator in the cell loop**

Inside the inner `for` loop (around line 667), after the cell element is created and before the click handler, add:

```javascript
      const move = moves.find((m) => m.row === engineRow && m.col === col);
      if (move) {
        cell.classList.add("target");
      }
```

Then, inside the existing block where the piece glyph `span` is created (around line 676, after `cell.appendChild(span);`), add the capture opacity. Change the piece-rendering block to:

```javascript
      if (piece) {
        const span = document.createElement("span");
        span.className = `piece-glyph piece-${piece.color}`;
        span.textContent = PIECE_SYMBOLS[piece.kind];

        if (
          state.selectedPiece &&
          state.selectedPiece.code === PIECE_CODES[piece.color][piece.kind]
        ) {
          cell.classList.add("selected");
        }

        if (move && move.capture) {
          span.style.opacity = "0.5";
        }

        cell.appendChild(span);
      }
```

Note: the `move` variable is already in scope from the earlier addition. The `move.capture` check applies the inline opacity to enemy pieces that can be captured.

- [ ] **Step 3: Test in browser**

1. Open a game, place some pieces on the board
2. Select a piece on the board (click it) — verify:
   - The selected piece's cell has yellow border (existing `.selected`)
   - Legal move destinations have green border (`.target`)
   - Enemy pieces on capturable cells have green border AND reduced opacity
3. Select a piece from the hand — verify no cells are highlighted
4. Deselect (click elsewhere) — verify highlights disappear
5. Test each piece type: pawn (forward + diagonal captures), rook (straight lines), bishop (diagonals), knight (L-shapes)
6. Verify moves are blocked by own pieces and stop at board edges

- [ ] **Step 4: Commit**

```bash
git add cmd/web/static/app.js
git commit -m "web: highlight legal moves when board piece is selected"
```

---

### Task 9: Add CSS capture indicator rule

**Files:**
- Modify: `cmd/web/static/style.css` (after `.board-cell.target` rule, around line 529)

- [ ] **Step 1: Add the rule**

After the existing `.board-cell.target` rule (line 527-529), add:

```css
.board-cell.target .piece-glyph {
  opacity: 0.5;
}
```

- [ ] **Step 2: Remove inline opacity from Task 8**

Since CSS now handles capture opacity, remove the inline style from `renderBoard`. In the piece rendering block, remove:

```javascript
        if (move && move.capture) {
          span.style.opacity = "0.5";
        }
```

The CSS rule `.board-cell.target .piece-glyph` already targets exactly the right elements — enemy piece glyphs inside target cells.

- [ ] **Step 3: Test in browser**

Verify capture targets still show reduced opacity on the enemy piece glyph, now via CSS rather than inline style.

- [ ] **Step 4: Commit**

```bash
git add cmd/web/static/style.css cmd/web/static/app.js
git commit -m "web: use CSS rule for capture indicator opacity"
```

---

### Task 10: End-to-end verification

No code changes — just manual testing.

- [ ] **Step 1: Test full game flow**

Open two browser tabs, start a game:

1. **Place phase:** Select a hand piece — confirm no board cells are highlighted
2. **Move phase:** Select a board piece — confirm legal moves are highlighted green, captures show dimmed enemy piece
3. **Piece switching:** Click a different own piece on the board — highlights update to new piece's moves
4. **Deselection:** Click empty cell without a selected piece — no highlights
5. **After move:** Make a move — highlights clear (selectedPiece resets on gameState)
6. **Opponent's turn:** Confirm clicking during opponent's turn does nothing (existing guard)

- [ ] **Step 2: Test pawn direction reversal**

1. Move a pawn to the far edge (row 0 for white, row 3 for black)
2. On the next gameState, the server sends updated pawnDirections
3. Select that pawn again — verify it now highlights moves in the reversed direction

- [ ] **Step 3: Test edge cases**

1. Piece with no legal moves (fully surrounded by own pieces) — no cells highlighted, just selected border
2. Knight near board edge — only valid L-shapes shown, no out-of-bounds highlights
3. Rook/bishop blocked by own piece — line stops before own piece

