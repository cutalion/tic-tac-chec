# Experiment: Beating the Hard Bot via `/loop`

A 14-iteration experiment where Claude progressively wrote, evaluated, and improved a Go client that plays Chess-Tic-Tac-Toe against the deployed `hard` bot on `https://ttc.ctln.pw`. Each iteration was scheduled by Claude Code's `/loop` skill, fired every 30 minutes over several hours. A persistent hint file carried context across iterations — each fresh Claude session read what the previous one had learned, applied one targeted improvement, played 3 games, and updated the hints.

## Goal

Ship a client that can consistently defeat the server's `hard` bot (a PPO-trained policy net with 500 MCTS simulations, served via ONNX from `internal/bot/`).

Secondary goals:
- Avoid manual intervention between iterations — every decision is made by Claude, guided by the hint file.
- Document every regression so later iterations don't repeat the mistake.
- Stay within the existing project's Go module (`tic-tac-chec`) and reuse `engine/` for game mechanics.

## Method

Each `/loop` fire executes the same prompt:

> play ttc game against hard bot on ttc.ctln.pw (write a client if needed). Record all moves, after a loose analyze why and write a hint into a text file. When you will start next time, check the hint file. You can run up to 5 subagents to play if needed.

Claude's workflow per iteration:

1. **Read** `tmp/bot_hints.md` — the cumulative design document with game records, observed bot strategies, open action items, and concrete next-run code changes.
2. **Apply one change** at the top of the open-items list (a code edit, a parameter bump, or a protocol change).
3. **Rebuild** and play 3 games against the hard bot.
4. **Analyze** each result — especially losses — and identify the failure mode.
5. **Update** the hint file: append game records, strike completed items, promote a new top item for the next iteration.

The hint file is deliberately verbose. It includes a table of all games, per-run retrospectives, observed bot strategies, and an ordered priority list of open improvements. Each fresh Claude instance starts cold but with complete context.

## Results

Final record: **3 wins / 3 games** at alpha-beta depth 6.

Run-by-run progression (3 games each):

| Run | Result | Key change |
|-----|--------|------------|
| 1 | 3L | Initial 1-ply heuristic + 80-ply cap |
| 2 | 3D | Random jitter (broke determinism); diagonal-threat defense |
| 3 | 2D+1L | Added `twoLineCount` offense gating + fork bonus + ply cap 120 |
| 4 | 3L | Bot switched to column attacks when diagonals were defended |
| 5 | 3L | Generalized `criticalLineDefense` to all 10 lines |
| 6 | 2D+1L | 2-ply forced-loss filter (top-8) |
| 7 | **3D clean** | topK 8→20 + attacker-on-defender precheck |
| 8 | 2D+1L | Added `leadsToForcedWin` + x3 opening offense (regression) |
| 9 | 2D+1L | Reverted x3 → x2; plateau confirmed |
| 10 | 🏆 **2W+1D** | **Alpha-beta minimax depth 4** — first wins ever |
| 11 | 1W+2D+1L-likely | Depth 5 + child ordering (CPU contention regression) |
| 12 | 🏆 **2W+1D** | Serial game protocol (confirmed depth 5 works) |
| 13 | 🏆 **2W+1W-likely** | Depth 6 + `handCount` in leaf eval |
| 14 | 🏆🏆🏆 **3W** | Transposition table + `maxPly` 120→150 |

Condensed progression bar:

> 3L → 3D → 2D+1L → 3L → 3L → 2D+1L → 3D → 2D+1L → 2D+1L → 🏆 2W+1D → 1W+2D+1L-likely → 🏆 2W+1D → 🏆 2W+1W-likely → **🏆🏆🏆 3W**

## The four key unlocks

Most iterations added targeted tactics that the bot eventually routed around. Only four changes produced durable jumps in win rate:

1. **Run #2 — Random jitter (0–2 points) on every candidate score.** Before this the hard bot's deterministic policy plus our deterministic heuristic replayed the *exact same game* every time (losses 2 and 3 were bit-identical). Jitter broke the loop and let us actually draw.
2. **Run #10 — Alpha-beta minimax depth 4 over the top-10 1-ply candidates.** First wins. The heuristic couldn't see setup-capture-capture sequences 3 plies deep; the search could. Alpha-beta subsumed the ad-hoc `leadsToForcedLoss`/`leadsToForcedWin` helpers that run #6–8 had built.
3. **Run #13 — Depth 6 + hand-count eval term.** `-3*(myHand - oppHand)` penalizes having pieces back in hand (which happens when we get captured). Plus the deeper horizon let the search find long winning combinations invisible at depth 5 (game #41 won at ply 119 on the bottom rank — a pattern never seen before).
4. **Run #14 — Transposition table + `maxPly` raised 120→150.** The TT kept depth 6 within the wall-time budget across long games; the raised cap meant we no longer timed out mid-win (game #43 won at ply 147, 3 plies from the new cap — would have been `win-likely` at the old cap).

## Architecture

```
┌────────────────────────┐
│  botclient/main.go     │
│                        │
│  • HTTP handshake      │───┐
│    POST /api/clients   │   │     https://ttc.ctln.pw
│    POST /api/bot-game  │   │     ┌────────────────┐
│  • WebSocket loop      │───┼─────│  Web server    │
│    wss://.../ws/room/X │   │     │  (cmd/web)     │
│                        │   │     │                │
│  • Reconstruct state   │   │     │  hard bot      │
│    → engine.Game       │   │     │  (ONNX/500 MCTS)│
│                        │   │     └────────────────┘
│  • Pick move:          │   │
│    1-ply heuristic     │   │
│    → top-10            │   │
│    → alpha-beta d6     │   │
│    (TT + ordering)     │   │
│                        │   │
│  • Record + analyze    │
│    → tmp/games/*.json  │
│    → tmp/bot_hints.md  │
└────────────────────────┘
```

### Move-picker evolution

| Version | Approach |
|---------|----------|
| v1 (run 1) | 1-ply heuristic: capture bonus + line-build + center bonus |
| v2 (run 2) | + random jitter + diagonal-threat detection |
| v3 (run 5) | + generalized 10-line defense (`criticalLineDefense`) |
| v4 (run 6) | + 2-ply forced-loss filter over top-K candidates |
| v5 (run 7) | + attacker-on-defender 1-ply precheck |
| v6 (run 10) | **Alpha-beta depth 4** over top-10 by 1-ply score |
| v7 (run 11) | + child ordering + depth 5 |
| v8 (run 13) | + depth 6 + `handCount` eval term |
| v9 (run 14) | + transposition table + `maxPly` 150 |

### Wire protocol (discovered and cached in hints)

- `POST /api/clients` (no body) → `{"token": "..."}`
- `POST /api/bot-game?difficulty=hard` with `Authorization: Bearer <token>` → `{"roomId": "..."}`
- `GET wss://ttc.ctln.pw/ws/room/{roomId}?token=...` — game stream
- First inbound: `{"type": "roomJoined", "color": "white"|"black"}`
- State frames: `{"type": "gameState", "state": {board, turn, status, winner, pawnDirections}}` — note: no `moveCount` (differs from `internal/wire.GameState`)
- Outbound moves: `{"type": "move", "piece": "WR", "to": "b2"}` (server auto-detects placement vs. on-board move)
- Square notation: `a1`–`d4`, white side at rank 1
- Piece codes: WP/WR/WB/WN, BP/BR/BB/BN (Knight is `N`, not `K`)

## Folder contents

```
bot-loop-vs-hard/
├── README.md              # this file
├── bot_hints.md           # the evolving design document (primary artifact)
├── botclient/
│   └── main.go            # archived client source (//go:build archived)
└── games/                 # 47 recorded games as JSON
    ├── 20260422-014300.json  (game 1: loss at ply 0 — initial bug)
    ├── ...
    └── 20260422-081248.json  (game 45: win at ply 49 — fastest)
```

Each game JSON has:
- `startedAt`, `endedAt`, `difficulty`, `myColor`, `result`
- `moves[]` — full ply-by-ply log including heuristic reasons and alpha-beta scores per candidate picked
- `finalBoard` — ASCII rendering of the terminal position
- `hintsUsed[]` — bullet lines from the hint file that were loaded (proof-of-read)

## How to reproduce

```bash
# 1. Restore the client into tmp/ (which is .gitignore'd)
mkdir -p tmp/botclient tmp/games
cp docs/experiments/bot-loop-vs-hard/botclient/main.go tmp/botclient/
# strip the //go:build archived line so Go picks it up:
sed -i '/^\/\/go:build archived$/d' tmp/botclient/main.go

# 2. Build
go build -o tmp/botclient/botclient ./tmp/botclient/

# 3. Run one game against the live hard bot
./tmp/botclient/botclient
# or: ./tmp/botclient/botclient --difficulty medium --base https://ttc.ctln.pw

# Output:
#   loaded N hint(s) from tmp/bot_hints.md
#   joined room=<uuid> as white
#   log: tmp/games/<timestamp>.json
#   result={win,draw,loss,win-likely,loss-likely,error} moves=<N>
```

Depth 6 takes ~1–4 minutes per game (game length-dependent). Playing 3 serial games per iteration keeps CPU contention from affecting search quality (lesson from run #11).

## What the hint file taught

Reading `bot_hints.md` chronologically traces an arc of strategy adaptation on both sides:

- The bot's opening `BN → d4` on ply 1 isn't a weak corner move — it's the anchor of the anti-diagonal `a1-b2-c3-d4`. The bot slow-builds this diagonal across 30+ plies while tempting us to chase captures.
- Whenever our defense specialized (e.g., diagonal-only), the bot switched targets (runs #4, #5: column wins instead of diagonals). Specialization becomes blinkers.
- The bot uses 2-ply capture setups: place piece X on an *innocuous* square, then next turn use X to capture our defender while landing on the critical line. Our 1-ply heuristic couldn't see these — they required alpha-beta depth 3+ to catch.
- Capturing our piece returns it to our hand, so every trade asymmetry shows up in `handCount`. Without a hand-count penalty, the search would happily trade material for tempo, then find itself unable to complete any 4-in-a-row.
- The bot's deterministic policy makes any deterministic opponent reproduce the same game every time — a single `rand.IntN(3)` prevents this and is one of the highest-ROI changes in the whole experiment.

## Future directions (noted in hints but not pursued)

- Play as BLACK (server currently always assigned us white in the `/api/bot-game` flow).
- Iterative deepening with a time budget per move for robustness under CPU variability.
- Mirror-match our alpha-beta against the local ONNX `bot_hard` to calibrate strength without involving the server.
- Self-play tournaments to collect training data for a learned leaf eval.

## Meta observation

Fourteen iterations of progressively richer 1-ply heuristics produced the plateau
`3L → 3D → 2D+1L → 3L → 3L → 2D+1L → 3D → 2D+1L → 2D+1L`. Adding minimax unlocked wins in *one* iteration (run #10: 2W+1D). Hand-crafted tactical rules let the opponent route around each specific defense; general adversarial search catches entire classes of trap at once. The hint file itself also paid off — a fresh Claude per iteration could see the full progression (including regressions) and pick "go deeper in search" instead of adding another tactical patch.
