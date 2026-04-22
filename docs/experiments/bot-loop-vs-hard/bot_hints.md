# Bot hints — lessons from playing the `hard` bot on ttc.ctln.pw

_Updated by the /loop task. Each run: read this, play a game, append observations. Next Claude instance: treat this as prior-knowledge to shape `tmp/botclient/main.go`._

## Client location
- Source: `tmp/botclient/main.go` (imports `tic-tac-chec/engine`)
- Build: `go build -o tmp/botclient/botclient ./tmp/botclient/`
- Run: `./tmp/botclient/botclient` (defaults: base=`https://ttc.ctln.pw`, difficulty=`hard`)
- Game logs: `tmp/games/<ts>.json`

## Game record
| # | date | color | result | plies | reason | log |
|---|------|-------|--------|-------|--------|-----|
| 1 | 2026-04-22 01:48 | white | draw (ply cap) | 80 | infinite b2/c3 capture cycle vs BB | `20260422-014842.json` |
| 2 | 2026-04-22 01:50 | white | **loss** | 46 | bot completed anti-diagonal a1-b2-c3-d4 (BR-BP-BB-BN) | `20260422-015004.json` |
| 3 | 2026-04-22 01:51 | white | **loss** | 46 | IDENTICAL game — same moves, same final board as game 2 | `20260422-015106.json` |
| 4 | 2026-04-22 02:07 | white | draw (ply cap) | 80 | **defended diagonal** — captured BN on d4 at ply 8, bot never recovered the anti-diagonal plan | `20260422-020715.json` |
| 5 | 2026-04-22 02:07 | white | draw (ply cap) | 80 | different game (jitter worked); heavy trade of material, ended 3W vs 4B on board | `20260422-020720.json` |
| 6 | 2026-04-22 02:07 | white | draw (ply cap) | 80 | pre-emptively captured BN on b2 at ply 2 after bot chose b2 (not d4) as anchor — wider defense covered both diagonals | `20260422-020724.json` |
| 7 | 2026-04-22 02:36 | white | draw (ply cap 120) | 120 | clean draw; my=0, opp=0 3-lines | `20260422-023641.json` |
| 8 | 2026-04-22 02:36 | white | draw (ply cap 120) | 120 | latent threat: opp 3-lines=1 at cap — likely a latent loss | `20260422-023645.json` |
| 9 | 2026-04-22 02:36 | white | **loss** | 94 | anti-diagonal AGAIN: BN-BR-BB-BP. Ply 92 was forced zugzwang; pre-ply 91 we had only 1 piece on the diag (WR@b2) and lost it | `20260422-023650.json` |
| 10 | 2026-04-22 03:06 | white | **loss** | 78 | col b = BR BN BB BP. Our diagonal defense fired but missed col b building | `20260422-030642.json` |
| 11 | 2026-04-22 03:06 | white | **loss** | 54 | col b = BR BN BB BP (same pattern) | `20260422-030647.json` |
| 12 | 2026-04-22 03:06 | white | **loss** | 42 | col a = BN BR BB BP. Bot switched from diagonals to columns | `20260422-030651.json` |
| 13 | 2026-04-22 03:36 | white | **loss** | 76 | anti-diagonal AGAIN (BR-BN-BP-BB). General line defense fired but couldn't catch 2-ply capture combo | `20260422-033644.json` |
| 14 | 2026-04-22 03:36 | white | **loss** | 12 | main diagonal (BR-BN-BB-BP); bot set up BB@b1 then captured our WN@c2 | `20260422-033650.json` |
| 15 | 2026-04-22 03:36 | white | **loss** | 16 | anti-diagonal via BB capturing into c3 | `20260422-033655.json` |
| 16 | 2026-04-22 04:07 | white | loss-likely | 120 | smoke-test: 2-ply filter survives to cap; opp 3-lines=1 on anti-diag (c3 empty) | `20260422-040722.json` |
| 17 | 2026-04-22 04:08 | white | draw (clean) | 120 | my=0, opp=0 at cap — true balanced endgame | `20260422-040802.json` |
| 18 | 2026-04-22 04:08 | white | **loss** | 42 | anti-diag (BP-BR-BB-BN); at ply 36 all top-8 candidates were already 2-ply losses (`2ply-hopeless`) | `20260422-040805.json` |
| 19 | 2026-04-22 04:08 | white | draw (ply cap) | 120 | sustained balanced play, no forced wins either side | `20260422-040810.json` |
| 20 | 2026-04-22 04:36 | white | **draw (clean)** | 120 | my=0, opp=0 at cap | `20260422-043646.json` |
| 21 | 2026-04-22 04:36 | white | **draw (clean)** | 120 | my=0, opp=0 at cap | `20260422-043650.json` |
| 22 | 2026-04-22 04:36 | white | **draw (clean)** | 120 | my=0, opp=0 at cap | `20260422-043655.json` |
| 23 | 2026-04-22 05:06 | white | **loss** | 80 | anti-diag (BR-BB-BN-BP). x3 opening captured BN at ply 4 but left us fork-vulnerable mid-game | `20260422-050654.json` |
| 24 | 2026-04-22 05:07 | white | draw (clean) | 120 | my=0, opp=0 at cap | `20260422-050659.json` |
| 25 | 2026-04-22 05:07 | white | draw (clean) | 120 | my=0, opp=0 at cap | `20260422-050703.json` |
| 26 | 2026-04-22 05:36 | white | draw (clean) | 120 | my=0, opp=0 at cap | `20260422-053628.json` |
| 27 | 2026-04-22 05:36 | white | draw (clean) | 120 | my=0, opp=0 at cap | `20260422-053633.json` |
| 28 | 2026-04-22 05:36 | white | **loss** | 66 | anti-diag (BR-BN-BB-BP); ply 60 `2ply-skip-4` then `2ply-hopeless` by ply 62 — 4-ply capture chain | `20260422-053638.json` |
| 29 | 2026-04-22 06:06 | white | draw (clean) | 120 | alpha-beta smoke test | `20260422-060633.json` |
| 30 | 2026-04-22 06:07 | white | **🏆 WIN** | 95 | col c = WN WB WP WR. Ply 92 alpha-beta promoted the 9th candidate (1-ply score -24) because `ab=99999` saw a forced win 4 plies out | `20260422-060723.json` |
| 31 | 2026-04-22 06:07 | white | **🏆 WIN** | 55 | main diag = WB WP WN WR. Ply 52 ab promoted 7th candidate (`ab=100000`); WN d4→c2 capture completed the diagonal | `20260422-060727.json` |
| 32 | 2026-04-22 06:07 | white | draw (clean) | 120 | my=0, opp=0 at cap | `20260422-060732.json` |
| 33 | 2026-04-22 06:36 | white | **🏆 WIN** | 75 | row 2 = WP WB WN WR (depth 5 smoke test, standalone). CPU: ~42s | `20260422-063633.json` |
| 34 | 2026-04-22 06:37 | white | draw (clean) | 120 | parallel with 2 others; my=0 opp=0 | `20260422-063736.json` |
| 35 | 2026-04-22 06:37 | white | draw (clean) | 120 | parallel with 2 others; my=0 opp=0 | `20260422-063742.json` |
| 36 | 2026-04-22 06:37 | white | loss-likely | 120 | opp has 3-lines=1 (row 3 BP-·-BB-BR with b1 empty) at cap | `20260422-063747.json` |
| 37 | 2026-04-22 07:06 | white | **🏆 WIN** | 91 | col d = WP WB WR WN (serial run, depth 5) | `20260422-070605.json` |
| 38 | 2026-04-22 07:07 | white | draw (clean) | 120 | serial run; my=0 opp=0 at cap | `20260422-070717.json` |
| 39 | 2026-04-22 07:08 | white | **🏆 WIN** | 79 | col a = WP WB WR WN (serial run, depth 5) | `20260422-070834.json` |
| 40 | 2026-04-22 07:36 | white | **🏆 WIN** | 71 | col a = WP WB WR WN (depth 6 smoke test) | `20260422-073640.json` |
| 41 | 2026-04-22 07:39 | white | **🏆 WIN** | 119 | row 3 = WR WP WN WB (bottom rank, new pattern) | `20260422-073935.json` |
| 42 | 2026-04-22 07:45 | white | win-likely | 120 | my 3-lines=1 (row 3 = WR-·-WN-WB, b1 empty); game C ran out of plies | `20260422-074509.json` |
| 43 | 2026-04-22 08:07 | white | **🏆 WIN** | 147 | row 2 = WP WR WB WN (TT+depth 6, maxPly 150 worked — won with 3 plies to spare) | `20260422-080704.json` |
| 44 | 2026-04-22 08:10 | white | **🏆 WIN** | 59 | row 1 = WN WB WP WR | `20260422-081059.json` |
| 45 | 2026-04-22 08:12 | white | **🏆 WIN** | 49 | row 0 (top rank) = WB WP WR WN — fastest win so far | `20260422-081248.json` |

## Observed bot strategies

### 1. Anti-diagonal build (a1-b2-c3-d4) — PRIMARY THREAT
Opening: **BN → d4** at ply 1 looks like a bad corner knight, but it's the FIRST anchor of the anti-diagonal. The bot then slowly drops BP/BB/BR onto b2, c3, a1 (in any order) and closes it out.

**Defense priority (next iteration):** if opponent has ≥2 pieces on {a1, b2, c3, d4} with none of ours there, treat this as a tier-1 threat. Occupy one empty cell on that line immediately — placement cost is worth stopping the diagonal.

### 2. Material trades toward a winning line
The bot captures aggressively when it also keeps its line-building intact. It will sacrifice tempo to keep a piece on the diagonal (e.g., ply 11: BN from d4 to b3 — *looks* like it breaks the diagonal, but then BN goes back to d4 on ply 19). Don't assume a capture has broken their plan.

### 3. Exploiting our b2/c3 oscillation
When we keep replacing a rook on b2/c3 to "block", the bot's bishop just sweeps it off. Each re-placement costs us a piece + a tempo. Stop placing rooks on squares that a bishop on the same diagonal will immediately capture.

## Heuristic bugs found (and partly fixed)

- `engine.Piece{}` equals `WhitePawn` (Color=0, Kind=0) → "no legal moves" guard tripped falsely. **Fixed** with explicit `ok bool`.
- Rep-penalty by post-move signature never fired because we recorded pre-move signatures. **Fixed** by tracking (board-before, piece, cell) move fingerprints with -200 per prior use.
- `inferLastMove` lost the source cell on move-with-capture. **Fixed** with disjoint `source`/`target`/`captured` tracking.

## Heuristic gaps still present (action items for next run)

### Resolved this iteration
- ✅ Diagonals weighted 2× in `lineBuildScore` via `isDiagonal()` helper.
- ✅ Explicit diagonal-threat term: if opponent has ≥2 on a diagonal with 0 mine, +80/piece bonus for landing on an empty diagonal cell; -50 penalty for ignoring.
- ✅ `math/rand/v2` jitter of 0–2 points on every score → breaks determinism.
- ✅ Offense boost: `ours *= 2` when `threats(&g.Board, opp) == 0`.
- ✅ Fork bonus: +40 if a candidate creates ≥2 new 2-lines.
- ✅ `maxPly` raised to 120 with near-win diagnostic (reports `my 3-lines` + `opp 3-lines` on cap).

### Regression this iteration
- ❌ Run #3 = 2 draws + 1 loss (vs. run #2 = 3 draws). Offense bias distracted us from defense.
- Game #8 hit cap with `opp 3-lines=1` — a latent loss that cap hid.
- Game #9 lost the anti-diagonal AGAIN (a1-b2-c3-d4 = BN-BR-BB-BP) on ply 94. At ply 91 bot had 2 on anti-diag (c3=BB, d4=BP) plus our WR at b2. Because `mineOnDiag > 0`, the defense term didn't fire, so we happily moved WR→b1 on ply 92, vacating b2; bot moved BR→b2 and completed the diagonal. **Gating bug**: `mineOnDiag > 0` is too permissive — a single defender on a contested line is fragile.

### Major regression run #4 (3 losses / 3 games)
- ❌❌❌ **Bot switched from diagonals to columns.** When we hardened the diagonal defense, the bot stopped targeting diagonals — all 3 losses were **4-in-a-column** (col b in games #10–11, col a in game #12).
- This is the single biggest lesson so far: **the diagonal-specific code became blinkers.** In game #10, heuristic logged `diag-reinforce+120` while col b was the actual winning line. We were watching the wrong target.
- The heuristic is specialized where it should be generalized: every defensive term we have (`countDiag`, `diagonals()`, `diag-defend`/`diag-abandon`/`diag-reinforce`) is diagonal-only. Rows and columns have no analogous protection.

### Run #5 (3 losses / 3 games) — generalized defense, still losing
- Generalized `criticalLineDefense` scans all 10 lines now. `line-defend+160` and `line-reinforce+120` did fire correctly.
- But **all 3 games still lost** — bot reverted to diagonals (2 anti-diag + 1 main diag wins).
- **Root cause now visible: 2-ply capture combos.** Pattern in all 3 games:
  - Bot has 2-3 pieces on line L, our 1 defender is on L at cell C.
  - Bot places/moves piece X to an attacking square near C (setup move).
  - Our 1-ply heuristic evaluates this bot move and sees no *immediate* win — so we pursue offense.
  - Next turn, bot captures our defender at C with piece X, landing ON L, completing the line.
- Example (game #14, ply 8→11): we defended c2 with WN on main diag. Bot placed BB at b1 (non-winning move). Bot then swept BB b1→c2 capturing WN, completing BR-BN-BB-BP. Our code saw this only at ply 10 when it was already unavoidable (`-79998 opponent wins next` on every candidate).
- Progression: 3L → 3D → 2D+1L → 3L → 3L. The generalization helped us notice more threats but didn't buy us lookahead.

### Run #6 (2 draws + 1 loss) — 2-ply forced-loss filter LANDED
- Added `leadsToForcedLoss(g, c, me)`: 3-ply search (our move → opp response → our reply → check opp immediate wins). If any opp response has NO safe reply for us, the candidate is flagged.
- `pickMoveWithHistory` now sorts by 1-ply score then tests top-8; first non-forced-loss is chosen (`2ply-skip-N` in heuristic log).
- Smoke test + 2 real games went to ply cap 120. Only one loss at ply 42 — and the log shows `2ply-hopeless` already at ply 36, meaning the bot constructed a 4-ply trap faster than our 2-ply filter could react.
- 1-ply per move went from ~100ms to ~250ms (smoke game took 31s for 120 plies). Sustainable.
- **Remaining failure modes:**
  - 4-ply setups (need deeper search or wider top-K).
  - "2ply-hopeless" cases where even top-8 all lose — we could check top-20 or all candidates; at 250ms/move extra, full enumeration doubles game time.

### Run #7 (3 CLEAN DRAWS) — defense milestone reached ✨
- Raised `topK` from 8→20 and added **attacker-on-defender 1-ply precheck**: for each critical line with `mine==1 && opp>=2`, scan on-board opp pieces for any whose `LegalMoves` include our defender's cell. If found:
  - candidate captures the attacker (c.cell == attackerCell): +120
  - candidate reinforces the line (landsOnLine): +80
  - candidate ignores: -60
- **All 3 games drew at ply cap with `my=0 opp=0` 3-lines** — genuine balanced endgames, no latent threats masked by the cap. This is the first run where every game closed cleanly.
- The bot never got a 3-in-a-line either; our combined filters prevent even the *setup* moves from gaining traction.
- The draw state is now stable: we defend perfectly at our strength level, but we also can't *win*. The remaining barrier is offense, not defense.

### Run #8 (2 draws + 1 loss) — offense push caused minor regression
- Added `leadsToForcedWin(g, c, me)`: mirror of `leadsToForcedLoss`. For every opp response, requires that at least one of our replies wins immediately. If so, pick this candidate with a `FORCED-WIN` tag.
- Added cold-start offense boost: `ours *= 3` when total pieces on board ≤ 4 (vs. x2 otherwise).
- **Results: 2 clean draws + 1 loss.** The loss followed x3 aggression at ply 4 that captured BN but left defensive reserves thin; by ply 74 we had two simultaneous `attacker-left-60` penalties — a fork we couldn't resolve.
- Critically: **`leadsToForcedWin` never fired in any of 3 games.** We don't have winning tactics available, only equal-strength positional play. Forced wins require existing 3-in-a-line threats that only appear when the opponent blunders — which the hard bot doesn't.

### Run #9 (2 draws + 1 loss) — revert didn't help; plateau confirmed
- Reverted the x3 opening boost back to x2.
- **Results: 2 clean draws + 1 loss at ply 66.** The loss was anti-diag (BR-BN-BB-BP) via a 4-ply capture chain: bot's BR repeatedly captured our WR as we cycled it between a1/a2/a3/a4, ending with BR completing the anti-diagonal. Our 2-ply filter flagged 4 of our top candidates as forced losses at ply 60 (`2ply-skip-4`); the 5th we picked set up the collapse.
- **Plateau proven.** Runs 7–9 have been "≥ 2 clean draws + 0-or-1 loss" regardless of offense tweaks. The remaining barrier is search depth.

### Run #10 (🏆 2 WINS + 1 DRAW) — ALPHA-BETA DEPTH 4 BROKE THE PLATEAU
- Replaced the 2-ply forced-loss / forced-win filters with proper **alpha-beta minimax depth 4** over the top-10 candidates (by 1-ply heuristic score). Leaf eval: `lineBuildScore(me) - lineBuildScore(opp) + 200*(threats(me)-threats(opp))`; terminal = ±100k.
- Per-game cost: ~35s wall clock, well under the 6-min timeout.
- **Results: 2 WINS + 1 clean DRAW** — the first wins against the hard bot across 32 games.
  - Game #30: win on column c (WN-WB-WP-WR). At ply 92 the 1-ply heuristic scored the winning move at -24 (negative), but alpha-beta valued it at 99999 — `ab-promoted-9` means we picked the 9th-ranked 1-ply candidate because deeper search saw the forced win.
  - Game #31: win on main diagonal (WB-WP-WN-WR). Ply 52 `ab-promoted-7` with `ab=100000`. Ply 54 winning move: WN d4→c2 capture.
- **Key finding:** the bot does blunder — but only against tactics 3+ plies deep. Our 1-ply heuristic couldn't see any forced wins in 3 full runs; alpha-beta depth 4 found winning lines in 2 of 3 games.
- Progression: 3L → 3D → 2D+1L → 3L → 3L → 2D+1L → 3D → 2D+1L → 2D+1L → **2W+1D** ✨

### Run #11 (1W + 2D + 1L-likely) — depth 5 + move-ordering; parallel regression
- Bumped `abSearch` depth 3→4 (so total horizon is 5 plies) and added child ordering inside the search: sort children by cheap leaf eval (maximize desc / minimize asc) for better alpha-beta pruning.
- **Standalone smoke test: WIN** at ply 75 on row 2 (WP-WB-WN-WR) — different pattern than run #10's wins.
- **3 parallel games: 2 clean draws + 1 loss-likely** at cap (opp 3-lines=1 on row 3). Aggregate run #11 = 1W+2D+1L-likely across 4 games.
- **Hypothesis: CPU contention under parallel play.** 3× depth-5 alpha-betas compete for cores, per-move time spikes unevenly, one game's search may have stalled / returned sub-optimal moves. Depth 4 is more robust under the parallel 3-game protocol.

### Run #12 (🏆 2 WINS + 1 DRAW, serial) — confirms parallel was the problem
- Same code as run #11 (depth 5 + ordering), but played games **one at a time** (≈60s each).
- **Results: 2 WINS + 1 clean DRAW** — matching run #10 (depth 4 parallel) exactly.
- Winning patterns: column d (game A), column a (game C). Both winning columns had the piece order WP-WB-WR-WN top-to-bottom, reflecting our heuristic's tendency to stack whites on a single file.
- **Hypothesis confirmed**: CPU contention — NOT search depth — caused run #11's regression. Depth 4 parallel ≈ depth 5 serial in win rate. No additional wins from depth increase at this architecture.
- Plateau conclusion: the hard bot and our alpha-beta-depth-5 are evenly matched modulo opening variance. 2/3 wins looks consistent.

### Run #13 (🏆 2 WINS + 1 WIN-LIKELY) — DEPTH 6 unlocks harder wins
- Bumped `abSearch` depth 4→5 (6-ply horizon total). Added `handCount(g, color)` to leaf eval: `-3*(myHand - oppHand)` discourages getting captured (captured pieces return to hand, raising myHand).
- Per-game cost: ~2m46s standalone (vs. ~60s at depth 5). Well within 6-min timeout.
- **Results — best so far: 2 WINS + 1 win-likely at cap.**
  - Game #40: win at ply 71, col a (WP-WB-WR-WN).
  - Game #41: win at ply 119, **row 3 bottom rank (WR-WP-WN-WB)** — new pattern never seen in earlier runs; depth 6 saw this long combination.
  - Game #42: cap with `my 3-lines=1` on row 3 (WR-·-WN-WB, b1 empty). One move from a third win.
- Depth 6 doesn't just repeat depth 5's wins — it finds *new* winning patterns, especially long combinations on rows. The hand-count eval term also helps: we less often sacrifice pieces unnecessarily.

### Run #14 (🏆🏆🏆 3 WINS / 3 GAMES) — FIRST PERFECT RUN
- Added a **transposition table** with bound flags (EXACT / LOWER / UPPER) scoped to each pickMoveWithHistory call, keyed by `boardSignature + side-to-move`. Raised `maxPly` 120 → 150.
- Wall time per game: 60s–3m40s (fluctuates with game length). TT didn't speed us up in absolute terms, but it did allow depth 6 to finish within 150-ply budgets consistently.
- **🎉 First 3-win sweep of the whole experiment.**
  - Game #43: row 2 (WP-WR-WB-WN) at ply 147. The `maxPly` raise was critical — at the old 120 cap this would have been win-likely.
  - Game #44: row 1 (WN-WB-WP-WR) at ply 59.
  - Game #45: row 0 top rank (WB-WP-WR-WN) at **ply 49, fastest win ever**.
- **Three different rows, three different games.** The hand-count term (`-3*(myHand-oppHand)`) seems to be the key offensive push: discouraging sacrifices keeps all four white pieces on the board, which is a prerequisite for any row completion.
- **Final progression:** 3L → 3D → 2D+1L → 3L → 3L → 2D+1L → 3D → 2D+1L → 2D+1L → 🏆 2W+1D → 1W+2D+1L-likely → 🏆 2W+1D → 🏆 2W+1W-likely → **🏆🏆🏆 3W/3G**.

### Open (priority order)

1. ✅ Generalized line defense landed.
2. ✅ 2-ply forced-loss filter landed.
3. ✅ Top-K expanded 8→20.
4. ✅ Attacker-on-defender 1-ply precheck landed.
5. ✅ `leadsToForcedWin` superseded by alpha-beta.
6. ✅ x3 opening offense reverted.
7. ✅ **Alpha-beta minimax depth 4 landed.** 2W+1D in run #10.
8. ✅ **Depth 5 + child ordering landed.** Mixed results under parallel, solid under serial.
9. ✅ Serial 3-game protocol.
10. ✅ Depth 6 landed.
11. ✅ Hand-count added to leaf eval.
12. ✅ maxPly raised 120 → 150.
13. ✅ Transposition table with bound flags.
14. **3 WINS / 3 GAMES. Goal achieved.** Subsequent work is polish.
15. **Try playing as BLACK.** Unexplored; could reveal asymmetry in the bot's strength.
16. **Iterative deepening.** Currently fixed depth 6. Under tight time budgets iterative deepening is more robust.
17. **Hints programmatic consumption.** Still `_ = hints`; now largely unnecessary since minimax subsumes most tactical logic.
18. **Celebrate + refactor.** Much of the 1-ply heuristic code is now dead weight (subsumed by alpha-beta). Could clean up lineDefend/reinforce/attacker logic — alpha-beta handles these via search.

## Protocol notes (so future runs don't re-discover)

- Auth: `POST /api/clients` returns `{"token": "..."}` (no body needed).
- Create game: `POST /api/bot-game?difficulty=hard` with `Authorization: Bearer <token>` returns `{"roomId": "..."}`.
- WS: `wss://ttc.ctln.pw/ws/room/{roomId}?token=<token>`.
- First message: `{"type":"roomJoined","roomId":...,"color":"white"|"black"}`.
- State messages: `{"type":"gameState","state":{board,turn,status,winner,pawnDirections}}` — **no MoveCount field** (differs from `internal/wire.GameState`).
- Move format: `{"type":"move","piece":"WR","to":"b2"}`. Placement vs. move is inferred by server (piece already on board ⇒ move; else placement).
- Square notation: `a1`–`d4`, white side at rank 1.
- Piece codes: WP/WR/WB/WN, BP/BR/BB/BN (knight is N, not K).

## Meta

- **Determinism trap (mitigated):** added 0–2-point jitter to every score, games are now genuinely different. But jitter is a band-aid — a fuller fix is minimax with a tie-break rule using hash of the move.
- **Progress measurement:** 3 losses in run #1 → 3 draws in run #2. Next target: first win (or at least 1 win in 3 games). If run #3 is again 3 draws at ply cap, the heuristic is plateauing and it's time to drop in the local ONNX model via `internal/bot/` + `bot/models/bot_hard.onnx` as the move picker (need `ORT_LIB_PATH` exported).
- **Concrete code-changes for run #15 (polish phase):**
  - Since we achieved 3W/3G, subsequent runs are about robustness and calibration rather than pushing the bar.
  - **Try depth 7** now that TT is in place. Might be too slow, but worth a smoke test.
  - **Try as BLACK**. Inspect server: `GET /api/me` or lobby flow might allow choosing color. Current `/api/bot-game?difficulty=hard` always returns white (server picks randomly? need to check). Repeat 3-game to see if black is disadvantaged.
  - **Refactor cleanup**: the `line-defend/-reinforce/-abandon` terms and the 2-ply forced-loss/win helpers are now redundant — alpha-beta subsumes them. Could delete to simplify, but the 1-ply heuristic is still used for child-ordering, so keep it lean.
- **Summary of the journey (14 iterations):**
  - Started with 3 losses in 46-plies, ended with **3 wins in 3 games**.
  - Key unlocks:
    1. Run #2: random jitter broke determinism (losses → draws).
    2. Run #10: alpha-beta search depth 4 (draws → wins).
    3. Run #13: depth 6 + hand-count eval (2W → 2W+winlikely).
    4. Run #14: maxPly 150 + transposition table (2W+winlikely → **3W**).
  - Regressions (runs #3, #4, #8, #11) each taught specific lessons — offensive boosts, defense specialization, CPU contention.
- **For future exploration:**
  - Try as BLACK.
  - Mirror-match the local ONNX bot vs. our alpha-beta to calibrate strength against the hard bot.
  - Self-play tournaments to gather training data for a learned eval function.
