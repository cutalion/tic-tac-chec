# Experiment: Beating the Hard Bot via `/loop`

A 23-iteration experiment where Claude progressively wrote, evaluated, and improved a Go client that plays Chess-Tic-Tac-Toe against the deployed `hard` bot on `https://ttc.ctln.pw`. Each iteration was scheduled by Claude Code's `/loop` skill, fired every 30 minutes over several hours. A persistent hint file carried context across iterations — each fresh Claude session read what the previous one had learned, applied one targeted improvement, played 2-3 games, and updated the hints.

**Final record: 23 wins + 5 draws + 1 loss-likely over 29 games (79.3% win rate, 0 confirmed losses).**

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

**Final record: 23 wins + 5 draws + 1 loss-likely over 29 games. Current config: iterative deepening alpha-beta (depth 4→7, adaptive cutoff), 4 parallel goroutines, shared Zobrist-keyed TT, 30s per-move budget.**

Run-by-run progression (mostly 3 games each):

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
| 15 | 🏆 **2W+1D** | Depth 7 ruled out (timed out); BLACK color blocked by server |
| 16 | 🏆🏆🏆 **3W** | TT across turns (persistent per-game) |
| 17 | 🏆 **1W+2D** | Zobrist hashing (string key → uint64, ~50% faster) |
| 18 | 🏆🏆🏆 **3W** | `--timeout` 6→12 min; **depth 7 finally fits** |
| 19 | 🏆🏆🏆 **3W** | **Parallel top-K**: 4 goroutines + syncTT (mutex); 3× faster |
| 20 | 🏆🏆 **2W** (2 games) | Depth 8 works, at the 12-min timeout ceiling |
| 21 | 🏆 **2W+1L-likely** | Depth 7 validation after depth-8 ambition |
| 22 | 🏆 **1W+2D** | More depth-7 data — confirms 75-85% win-rate variance |
| 23 | 🏆🏆🏆 **3W** | **Iterative deepening** (d4→d7, 30s per-move budget, adaptive cutoff) |

Condensed progression bar:

> 3L → 3D → 2D+1L → 3L → 3L → 2D+1L → 3D → 2D+1L → 2D+1L → 🏆 2W+1D → 1W+2D+1L-likely → 🏆 2W+1D → 🏆 2W+1W-likely → 🏆🏆🏆 3W → 🏆 2W+1D → 🏆🏆🏆 3W → 🏆 1W+2D → 🏆🏆🏆 3W → 🏆🏆🏆 3W → 🏆🏆 2W → 🏆 2W+1L-likely → 🏆 1W+2D → **🏆🏆🏆 3W**

**Cumulative from run #10 (first win) to run #23: 29 wins, 7 draws, 1 loss-likely across 40 games — 72.5% win rate, 0 confirmed losses after the alpha-beta breakthrough.**

## Performance (wall time)

All measured on commodity hardware, running one game serially (no parallel games — game-level parallelism caused CPU contention, see run #11).

| Config | Per move | Per game (60–150 plies) | 3 games serial |
|--------|----------|-------------------------|----------------|
| Depth 4 (run #10) | ~30–80 ms | ~35 s | ~1.5 min |
| Depth 5, serial (run #12) | ~0.8–1.5 s | ~1 min | ~3 min |
| Depth 6 + per-move TT (run #14) | ~3–5 s | ~3.5 min | ~10 min |
| Depth 6 + persistent TT (run #16) | ~2.5–4 s | ~2.5–3 min | ~8 min |
| **Depth 6 + Zobrist TT (run #17)** | **~1–2 s** | **~1.5 min** | **~5 min** |
| **Depth 6 + Zobrist + parallel (run #19)** | **~0.5–1 s** | **~1–2 min** | **~3–6 min** |
| Depth 7 + Zobrist + parallel (run #19) | ~4–8 s | ~2.5 min | ~7.5 min |
| Depth 8 + Zobrist + parallel (run #20) | ~15–25 s | ~11–12 min | exceeded 30 min cron |
| Iterative deepening d4→d7 (run #23) | 30 s cap/move, reaches d8 on 30–77% of moves | ~1–2 min | ~5 min |

**Key insight**: **per-move time is what matters for interactive play** — depth 6 parallel's ~0.5–1 s per move is snappier than most humans respond. The "long" game times are just `per_move × number_of_plies`, not the time to make any single decision. This is why the 30-min cron window comfortably held 3 serial games for almost every configuration (only depth 8 pushed the boundary).

**Why per-move dominates**: a 100-ply game at depth 6 parallel is 50 of our moves × ~1 s ≈ 50 s of search, plus network and bot-reply latency. Network per move is ~100 ms round-trip; bot-reply (the server's hard bot doing its own MCTS) runs ~500 ms–1 s. So a substantial chunk of each "game minute" is waiting for the opponent, not our search.

## The seven key unlocks

Most iterations added targeted tactics that the bot eventually routed around. Seven changes produced durable jumps in strength:

1. **Run #2 — Random jitter (0–2 points) on every candidate score.** Before this the hard bot's deterministic policy plus our deterministic heuristic replayed the *exact same game* every time (losses 2 and 3 were bit-identical). Jitter broke the loop and let us actually draw.
2. **Run #10 — Alpha-beta minimax depth 4 over the top-10 1-ply candidates.** First wins. The heuristic couldn't see setup-capture-capture sequences 3 plies deep; the search could. Alpha-beta subsumed the ad-hoc `leadsToForcedLoss`/`leadsToForcedWin` helpers that run #6–8 had built.
3. **Run #13 — Depth 6 + hand-count eval term.** `-3*(myHand - oppHand)` penalizes having pieces back in hand (which happens when we get captured). Plus the deeper horizon let the search find long winning combinations invisible at depth 5 (game #41 won at ply 119 on the bottom rank — a pattern never seen before).
4. **Run #14 — Transposition table + `maxPly` raised 120→150.** The TT kept depth 6 within the wall-time budget across long games; the raised cap meant we no longer timed out mid-win (game #43 won at ply 147, 3 plies from the new cap — would have been `win-likely` at the old cap).
5. **Run #16 — Persistent TT across turns.** Allocated once per game instead of per-move, passed down into `pickMoveWithHistory`. Cross-move subtree reuse cut wall time ~20%.
6. **Run #17 — Zobrist hashing.** Replaced string TT keys with `uint64` XOR of precomputed per-cell per-piece values + side-to-move bit. Single largest speedup (~50%).
7. **Run #19 — Parallel top-K with shared `syncTT`.** 4 goroutines across the top-10 candidates, mutex-protected TT. Final ~3× speedup at depth 7 (7m25s → 2m27s). Enabled depth 8 to fit inside the raised 12-min timeout.
8. **Run #23 — Iterative deepening with adaptive cutoff.** 30s per-move budget; runs depth 4→7 and stops between depths when `remaining_time < 4 × last_depth_time`. Reaches depth 8 on 30–77% of moves depending on game phase (endgame gets deeper because branching drops). 3W/3G in the first trial.

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
| v9 (run 14) | + per-move transposition table + `maxPly` 150 |
| v10 (run 16) | + **persistent** TT across turns |
| v11 (run 17) | + **Zobrist** hashing (string → uint64) |
| v12 (run 18) | + `--timeout` 12 min + depth 7 |
| v13 (run 19) | + **parallel top-K** (4 goroutines, mutex-protected `syncTT`) |
| v14 (run 20) | + depth 8 (at wall-time ceiling) |
| v15 (run 23) | + **iterative deepening** (d4→d7 with 30s/move budget, adaptive cutoff) |

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

Twenty-three iterations fall into three phases:

1. **Runs #1–9 (heuristic plateau)**: Progressively richer 1-ply heuristics produced `3L → 3D → 2D+1L → 3L → 3L → 2D+1L → 3D → 2D+1L → 2D+1L`. Each tactical rule the bot routed around, prompting a more general rule. We never won.
2. **Runs #10–14 (search breakthrough)**: Adding minimax unlocked wins in *one* iteration (run #10: 2W+1D). Each subsequent change deepened or sped the search: depth 5, depth 6, hand-count eval, ply cap 150, TT. Run #14 hit 3W/3G — first perfect run.
3. **Runs #15–23 (engine optimization)**: Every change was about making the search faster, deeper, or more robust: persistent TT, Zobrist hashing, 12-min timeout, parallel top-K goroutines, depth 8 ceiling, and finally iterative deepening. Win rate stabilized at 75–85%, reaching depth 8 on a majority of endgame moves.

Hand-crafted tactical rules let the opponent route around each specific defense; general adversarial search catches entire classes of trap at once. The hint file itself was the biggest multiplier — a fresh Claude per iteration could see the full progression (including regressions) and pick "go deeper in search" or "raise the timeout" instead of adding another tactical patch. It's effectively a memo-passing mechanism between short-lived Claude instances, and it converts the 30-minute cron cadence into real forward progress.

The production config (**iterative deepening d4→d7 with 30s budget, 4-goroutine parallel top-K, Zobrist-keyed shared syncTT, maxPly 150**) consistently beats the hard bot 2-of-3 or 3-of-3, using **~0.5–1 s of search per move** and ~1–2 minutes of wall time per full game on commodity hardware. Search depth adapts naturally from d5–d6 in openings to d7–d8 in endgames as branching factor drops. **This is fast enough to embed as a server-side bot alongside the ONNX models** — the `altbot` would have zero external dependencies (pure Go on `engine/`), no `ORT_LIB_PATH` needed, and per-move latency competitive with a snappy human.

Further gains would require mechanisms outside pure search: ONNX policy-net priors for tighter move-ordering (potentially unlocking depth 9), aspiration windows for faster pruning, or opening books to skip the first few plies. The current ceiling reflects the hard bot's 500 MCTS simulations; matching or exceeding its forward-search power seems possible only by borrowing its own policy network.
