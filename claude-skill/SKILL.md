---
name: play-tic-tac-chec
description: Play a game of Chess Tic-Tac-Toe against Claude
allowed-tools: Bash, Read, Edit
---

# Skill: Play Tic-Tac-Chec

You are playing Tic-Tac-Chec as **Black** against the human (White).
The game runs through a CLI binary built from this project.

## Game Rules

- 4x4 board, 2 players: White (human) and Black (you)
- Each player has 4 chess pieces: Pawn (P), Rook (R), Bishop (B), Knight (N)
- On your turn you may either:
  - **Place** a piece from your hand onto any empty cell
  - **Move** a piece already on the board using chess-style movement
- **Capturing** an opponent's piece returns it to the opponent's hand (shogi-style)
- **Pawns** reverse direction when reaching the far edge (no promotion)
- **Win condition:** get 4 of your color in a row (horizontal, vertical, or diagonal)

## Chess Movement Rules

- **Rook:** slides horizontally or vertically, any number of squares (up to 3 on this board)
- **Bishop:** slides diagonally, any number of squares
- **Knight:** L-shape (2+1), can jump over pieces
- **Pawn:** moves 1 square forward (toward opponent's side), captures diagonally forward

## CLI Reference

Binary location: `__SKILL_DIR__/tic-tac-chec-cli` (build with `go build -o tic-tac-chec-cli ./cmd/cli` if needed)

| Command | Example |
|---------|---------|
| Start new game | `__SKILL_DIR__/tic-tac-chec-cli start` |
| Make a move | `__SKILL_DIR__/tic-tac-chec-cli --game=<path> move <piece> <square>` |

- **Piece codes:** WP, WR, WB, WN (White) / BP, BR, BB, BN (Black)
- **Squares:** columns a-d, rows 1-4 (e.g., `a1` = bottom-left, `d4` = top-right)
- The `start` command prints the game state file path in its output. Capture it for subsequent moves.

## Board Layout

```
  a  b  c  d
4 .  .  .  .  4    ← Black's home side (row 4 = top)
3 .  .  .  .  3
2 .  .  .  .  2
1 .  .  .  .  1    ← White's home side (row 1 = bottom)
  a  b  c  d
White hand: WP WR WB WN
Black hand: BP BR BB BN
Next turn: W
```

## Game Loop

1. **Start:** Run `__SKILL_DIR__/tic-tac-chec-cli start` and note the `--game=<path>` from the output.
2. **Show the board** to the human from the CLI output.
3. **Human's turn (White):** Ask the human for their move. They should tell you something like "pawn to a3" or "WP a3". Execute: `__SKILL_DIR__/tic-tac-chec-cli --game=<path> move <piece> <square>`
4. **Show the board** after the human's move so they can see the result.
5. **Your turn (Black):** Analyze the board, decide your move, and execute it.
6. **Show updated board** after your move.
7. **Repeat** from step 3 until someone wins or the human wants to stop.

If a move fails (illegal move, wrong turn, etc.), show the error and ask for a different move.

## Output Style

- **Be concise.** Show the board, state your move, ask for theirs.
- **Always wrap the board in a markdown code block** (triple backticks) so column alignment is preserved. Always put a text line before the code block (e.g., your move description or "Board:") so the CLI bullet point (`●`) doesn't merge with the first row of the board.
- After your move, just say what you did: "BP b3. Your move!" Do NOT reveal your strategy, which line you're building, or why you chose a move. Keep it secret.
- Only explain strategy if the human asks "why?" or similar.

## Strategy

When choosing your move, think through these priorities in order:

1. **Win if you can.** If placing or moving a piece completes 4-in-a-row, do it.
2. **Block immediate threats.** If the human has 3-in-a-row with the 4th cell open, check whether they can actually fill it in one legal move. If their hand is empty, the only way to fill the cell is to move an existing piece there — but if that piece is already part of the 3-in-a-row, moving it vacates its cell and doesn't complete the line. Only block if the opponent has a real way to complete it (a piece in hand, or a piece NOT in the line that can legally reach the open cell). **Critical:** also verify that the opponent cannot capture your blocker with a piece that would itself complete the line. If they can, the block fails — instead break the line by capturing one of their pieces.
3. **Check ALL lines through every cell before moving.** There are 10 winning lines (4 rows, 4 columns, 2 diagonals). Before each move, scan every line — don't fixate on one threat and miss another. Especially check whether your own move opens a winning line for the opponent.
4. **Don't hand the opponent a win.** Before capturing or vacating a cell, check if you're opening a line for them. Moving off a cell is as important as moving onto one.
5. **Build two-way threats.** Place or move to create a position where you threaten to complete a line in two different ways. The human can only block one.
6. **Be deceptive — don't build lines obviously.** Avoid placing pieces in a straight line where the opponent can see the threat forming. Instead, build lines indirectly:
   - Place a knight away from your target line — it can jump into position later.
   - Capture an opponent's piece that blocks your line, framing it as a defensive move.
   - Place pieces that serve multiple lines simultaneously, so your true goal isn't clear.
   - Set up a line where the final piece can be moved into place (e.g., a rook sliding across a row, a bishop moving diagonally into the last cell).
7. **Control the center.** Cells b2, b3, c2, c3 appear in the most lines. Prefer placing pieces there early.
8. **Prefer placing over moving** in the early game (first 3-4 turns). Move pieces later when repositioning creates better alignments.
9. **Capture only with purpose.** Capturing removes your piece from its current line AND returns a piece to the opponent's hand. Only capture to block a winning threat or land in a strong position.
10. **Consult Lessons Learned.** Before choosing a move, review the Lessons Learned section below. These are patterns from past losses — avoid repeating them.

## Post-Game Analysis (on loss)

When you lose a game, **before saying anything else**, do the following:

1. **Reconstruct the game.** List every move in order (both players), noting the board state at key turning points.
2. **Identify the losing move.** Find the specific move (or failure to move) that allowed the opponent to win. Was it:
   - A missed block? (opponent had 3-in-a-row and you didn't stop it)
   - A bad trade? (you captured a piece but opened a winning line)
   - A positional mistake? (you ignored a developing threat)
   - A missed win? (you had a winning move but didn't see it)
3. **Extract a concrete lesson.** Write a specific, actionable rule — not vague advice. Bad: "Be more careful." Good: "When opponent has pieces on a1 and a2, check if a rook or pawn in hand can reach a3/a4 before placing elsewhere."
4. **Update the skill file:**
   - Read `__SKILL_DIR__/SKILL.md`
   - Check if a similar lesson already exists in "Lessons Learned"
   - If **no similar lesson exists**: add the new lesson to the "Lessons Learned" section with a tally of `(1)`
   - If **a similar lesson exists**: increment its tally. If the tally reaches `(3)`, **promote it** — move it into the Strategy section as a new numbered rule and remove it from Lessons Learned.
   - When promoting, generalize the lesson into a reusable principle (combine specifics from all instances).
5. **Tell the human** what you learned (briefly), e.g., "GG! I noticed I missed your rook threat on column c — I've updated my strategy to watch for that."

## Lessons Learned

<!-- Lessons from lost games. Format: - **Lesson** (tally) -->
<!-- When a lesson reaches (3), promote it to the Strategy section above. -->
- **Before committing to an offensive move, count the opponent's pieces on every row, column, and diagonal. If any line has 3 of their pieces, check whether the 4th cell can be reached (by a piece in hand OR a knight/rook/bishop move). If so, block the gap immediately — do not play offense when a defensive emergency exists.** (1)
