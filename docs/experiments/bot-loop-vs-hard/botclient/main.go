// Archived reference copy of the bot client used for the /loop experiment.
// The live copy lives at tmp/botclient/main.go (in .gitignore). This file is
// preserved here for future readers of the experiment; the build tag keeps it
// out of `go build ./...`.

//go:build archived

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"math/rand/v2"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"tic-tac-chec/engine"

	"github.com/coder/websocket"
)

var (
	baseURL    = flag.String("base", "https://ttc.ctln.pw", "base URL")
	difficulty = flag.String("difficulty", "hard", "bot difficulty")
	logPath    = flag.String("log", "", "game log output path (defaults to tmp/games/<ts>.json)")
	hintsPath  = flag.String("hints", "tmp/bot_hints.md", "path to persistent hints file")
	timeout    = flag.Duration("timeout", 12*time.Minute, "overall game timeout")
)

type piecePayload struct {
	Color string `json:"color"`
	Kind  string `json:"kind"`
}

type gameStatePayload struct {
	Board          [engine.BoardSize][engine.BoardSize]*piecePayload `json:"board"`
	Turn           string                                            `json:"turn"`
	Status         string                                            `json:"status"`
	Winner         *string                                           `json:"winner"`
	PawnDirections struct {
		White string `json:"white"`
		Black string `json:"black"`
	} `json:"pawnDirections"`
}

type wsMessage struct {
	Type   string          `json:"type"`
	RoomID string          `json:"roomId,omitempty"`
	Color  string          `json:"color,omitempty"`
	Error  string          `json:"error,omitempty"`
	State  json.RawMessage `json:"state,omitempty"`
}

type moveRecord struct {
	Ply       int    `json:"ply"`
	Mover     string `json:"mover"` // "me" or "bot"
	Piece     string `json:"piece"` // e.g. "WR"
	To        string `json:"to"`    // e.g. "b2"
	FromCell  string `json:"from,omitempty"`
	Captured  string `json:"captured,omitempty"` // e.g. "BP"
	WasPlace  bool   `json:"wasPlace"`
	Heuristic string `json:"heuristic,omitempty"`
}

type gameLog struct {
	StartedAt  time.Time    `json:"startedAt"`
	EndedAt    time.Time    `json:"endedAt"`
	Difficulty string       `json:"difficulty"`
	MyColor    string       `json:"myColor"`
	Result     string       `json:"result"` // "win", "loss", "draw", "error"
	Reason     string       `json:"reason,omitempty"`
	Moves      []moveRecord `json:"moves"`
	HintsUsed  []string     `json:"hintsUsed,omitempty"`
	FinalBoard string       `json:"finalBoard,omitempty"`
}

func main() {
	flag.Parse()
	log.SetFlags(log.Ltime | log.Lmicroseconds)
	initZobrist()

	hints := loadHints(*hintsPath)
	if len(hints) > 0 {
		fmt.Printf("loaded %d hint(s) from %s\n", len(hints), *hintsPath)
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	log := &gameLog{
		StartedAt:  time.Now(),
		Difficulty: *difficulty,
		HintsUsed:  hints,
	}

	if err := runGame(ctx, log); err != nil {
		log.Result = "error"
		log.Reason = err.Error()
	}
	log.EndedAt = time.Now()

	writeLog(log)
	fmt.Printf("result=%s moves=%d color=%s\n", log.Result, len(log.Moves), log.MyColor)
	if log.Result == "loss" || log.Result == "error" {
		os.Exit(2)
	}
}

func runGame(ctx context.Context, gl *gameLog) error {
	token, err := createClient(ctx)
	if err != nil {
		return fmt.Errorf("create client: %w", err)
	}

	roomID, err := createBotGame(ctx, token)
	if err != nil {
		return fmt.Errorf("create bot game: %w", err)
	}

	wsURL := strings.Replace(*baseURL, "https://", "wss://", 1)
	wsURL = strings.Replace(wsURL, "http://", "ws://", 1)
	wsURL = fmt.Sprintf("%s/ws/room/%s?token=%s", wsURL, roomID, token)

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		return fmt.Errorf("ws dial: %w", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "bye")

	var myColor engine.Color
	var haveColor bool
	var prevState *engine.Game
	var pendingMove *pending
	moveHistory := map[string]int{} // (board+piece+cell) fingerprint → times used
	// Transposition table shared across all moves in this game (HINT run #15).
	// Capped at 200k entries; cleared when full to bound memory.
	gameTT := newSyncTT(8192)
	ply := 0
	const maxPly = 150

	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return fmt.Errorf("ws read: %w", err)
		}

		var msg wsMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return fmt.Errorf("unmarshal: %w (raw=%s)", err, string(data))
		}

		switch msg.Type {
		case "roomJoined":
			myColor, err = colorFromStr(msg.Color)
			if err != nil {
				return err
			}
			haveColor = true
			gl.MyColor = msg.Color
			fmt.Printf("joined room=%s as %s\n", msg.RoomID, msg.Color)

		case "gameState":
			var payload gameStatePayload
			if err := json.Unmarshal(msg.State, &payload); err != nil {
				return fmt.Errorf("state unmarshal: %w", err)
			}
			g, err := reconstruct(payload)
			if err != nil {
				return err
			}

			if prevState != nil {
				if rec := inferLastMove(prevState, g); rec != nil {
					rec.Ply = ply
					if pendingMove != nil && rec.Piece == pieceCode(pendingMove.piece) && rec.To == cellToAlgebraic(pendingMove.cell) {
						rec.Mover = "me"
						rec.Heuristic = pendingMove.reason
						pendingMove = nil
					} else {
						rec.Mover = "bot"
					}
					gl.Moves = append(gl.Moves, *rec)
					ply++
				}
			}
			prevState = g

			if g.Status == engine.GameOver {
				gl.FinalBoard = g.Board.String()
				if g.Winner == nil {
					gl.Result = "draw"
				} else if haveColor && *g.Winner == myColor {
					gl.Result = "win"
				} else {
					gl.Result = "loss"
				}
				return nil
			}

			if !haveColor {
				continue
			}
			if g.Turn != myColor {
				continue
			}

			if ply >= maxPly {
				gl.FinalBoard = g.Board.String()
				myThreats := threats(&g.Board, myColor)
				oppThreats := threats(&g.Board, oppColor(myColor))
				if oppThreats > 0 && myThreats == 0 {
					gl.Result = "loss-likely"
				} else if myThreats > 0 && oppThreats == 0 {
					gl.Result = "win-likely"
				} else {
					gl.Result = "draw"
				}
				gl.Reason = fmt.Sprintf("ply cap reached (%d); my 3-lines=%d, opp 3-lines=%d", maxPly, myThreats, oppThreats)
				return nil
			}

			if gameTT.size() > 200000 {
				gameTT.reset(8192)
			}
			piece, cell, reason, ok := pickMoveWithHistory(g, myColor, gl.HintsUsed, moveHistory, gameTT)
			if ok {
				moveHistory[moveFingerprint(g, piece, cell)]++
			}
			if !ok {
				return fmt.Errorf("no legal moves found")
			}
			pendingMove = &pending{piece: piece, cell: cell, reason: reason}

			if err := sendMove(ctx, conn, piece, cell); err != nil {
				return fmt.Errorf("send move: %w", err)
			}

		case "error":
			return fmt.Errorf("server error: %s", msg.Error)

		default:
			// ignore opponentAway, reaction, rematchRequested, etc.
		}
	}
}

type pending struct {
	piece  engine.Piece
	cell   engine.Cell
	reason string
}

// inferLastMove diffs two board states and returns a record of the single move
// that produced curr from prev. Cases:
//   - placement onto empty cell: one added cell, no removed cell.
//   - placement-capture: impossible in this game (placements must be on empty cells).
//   - slide/step move to empty: one removed + one added (different cells).
//   - slide/step move with capture: one removed source + one target cell whose
//     occupant changed from opp to mover.
func inferLastMove(prev, curr *engine.Game) *moveRecord {
	var (
		source      *engine.Cell
		target      *engine.Cell
		targetPiece engine.Piece
		captured    *engine.Piece
	)
	for r := range engine.BoardSize {
		for c := range engine.BoardSize {
			pc, cc := prev.Board[r][c], curr.Board[r][c]
			switch {
			case pc == nil && cc == nil:
			case pc != nil && cc != nil && *pc == *cc:
			case pc != nil && cc == nil:
				cell := engine.Cell{Row: r, Col: c}
				source = &cell
			case pc == nil && cc != nil:
				cell := engine.Cell{Row: r, Col: c}
				target = &cell
				targetPiece = *cc
			case pc != nil && cc != nil && *pc != *cc:
				cell := engine.Cell{Row: r, Col: c}
				target = &cell
				targetPiece = *cc
				cap := *pc
				captured = &cap
			}
		}
	}
	if target == nil {
		return nil
	}
	rec := &moveRecord{
		Piece:    pieceCode(targetPiece),
		To:       cellToAlgebraic(*target),
		WasPlace: source == nil,
	}
	if source != nil {
		rec.FromCell = cellToAlgebraic(*source)
	}
	if captured != nil {
		rec.Captured = pieceCode(*captured)
	}
	return rec
}

// -----------------------------------------------------------------------------
// move picker: 1-ply heuristic with look-ahead for immediate wins / losses
// -----------------------------------------------------------------------------

type candidate struct {
	piece  engine.Piece
	cell   engine.Cell
	score  int
	reason string
}

func pickMoveWithHistory(g *engine.Game, me engine.Color, hints []string, moveHistory map[string]int, tt *syncTT) (engine.Piece, engine.Cell, string, bool) {
	cands := enumerate(g, me)
	if len(cands) == 0 {
		return engine.Piece{}, engine.Cell{}, "", false
	}
	for i := range cands {
		s, why := scoreCandidate(g, cands[i], me, hints)

		// repetition penalty: if I've already played this exact (board-state, piece, cell)
		// before, strongly penalize — avoids chasing the same capture/replace loop.
		if seen := moveHistory[moveFingerprint(g, cands[i].piece, cands[i].cell)]; seen > 0 {
			s -= 200 * seen
			why = fmt.Sprintf("%s rep-%d", why, seen)
		}

		// tiny random jitter to break ties and make games non-identical across
		// runs — the hard bot is deterministic, so any fully-deterministic
		// opponent replays the exact same game every time (hint file, games 2+3).
		s += rand.IntN(3)

		cands[i].score = s
		cands[i].reason = why
	}

	// sort candidates by score descending so we test the most promising first.
	sort.SliceStable(cands, func(i, j int) bool { return cands[i].score > cands[j].score })

	// 2-ply forced-loss filter (HINT run #5): our 1-ply heuristic missed setup
	// moves like BB→b1 followed by BB→c2 capturing our defender. Test the top
	// candidates by simulating opp's best response and checking whether every
	// response we'd have lets opp win. If yes, skip this candidate.
	// HINT run #6: raised topK 8→20 — game #18 had all top-8 as forced losses,
	// but a safe alternative may have existed deeper in the list.
	// HINT run #17: depth 7 top-K=5 fits in timeout but only draws (top-5 too
	// narrow). Depth 7 top-K=10 still times out. Production: depth 6 top-K=10.
	const topK = 10
	limit := min(topK, len(cands))

	// HINT run #19: parallelize the top-K alpha-beta searches across 4
	// goroutines sharing a mutex-protected TT. Each goroutine searches one
	// candidate independently; the shared TT accelerates repeated subtree
	// lookups across candidates.
	const workers = 4
	abScores := make([]int, limit)
	for i := range abScores {
		abScores[i] = math.MinInt
	}
	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup
	for i := 0; i < limit; i++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()
			sim := g.Clone()
			if err := sim.Move(cands[i].piece, cands[i].cell); err != nil {
				return
			}
			// depth 6 = 7-ply horizon. HINT run #20: depth 8 works but hovers
			// at the 12-min timeout ceiling; depth 7 at ~2m27s/game is the
			// robust production setting for 3-game validation runs.
			abScores[i] = abSearch(sim, me, 6, math.MinInt+1, math.MaxInt-1, false, tt)
		}(i)
	}
	wg.Wait()

	bestIdx := 0
	bestAB := math.MinInt
	for i := 0; i < limit; i++ {
		composite := abScores[i]*10 + cands[i].score/10
		if composite > bestAB {
			bestAB = composite
			bestIdx = i
		}
	}
	best := cands[bestIdx]
	tag := ""
	if bestIdx != 0 {
		tag = fmt.Sprintf(" ab-promoted-%d", bestIdx)
	}
	return best.piece, best.cell, fmt.Sprintf("score=%d %s ab=%d%s", best.score, best.reason, bestAB/10, tag), true
}

// Transposition-table entry kinds (bound flags).
const (
	ttExact = 0
	ttLower = 1 // score is a lower bound (fail-high, beta cutoff)
	ttUpper = 2 // score is an upper bound (fail-low, alpha unmoved)
)

type ttEntry struct {
	depth int
	score int
	flag  int
}

// syncTT is a mutex-protected map-based transposition table safe for
// concurrent access from multiple goroutines (HINT run #18: parallel top-K).
type syncTT struct {
	mu sync.RWMutex
	m  map[uint64]ttEntry
}

func newSyncTT(capHint int) *syncTT {
	return &syncTT{m: make(map[uint64]ttEntry, capHint)}
}

func (s *syncTT) lookup(key uint64) (ttEntry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.m[key]
	return e, ok
}

func (s *syncTT) store(key uint64, e ttEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[key] = e
}

func (s *syncTT) size() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.m)
}

func (s *syncTT) reset(capHint int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m = make(map[uint64]ttEntry, capHint)
}

// Zobrist table: per-cell × per-(color,kind) random 64-bit values + one for
// side-to-move. Seeded deterministically so game logs are reproducible.
// HINT run #16: string TT key was measurable overhead; Zobrist lets us use
// *syncTT which is significantly faster.
var (
	zobristPieces [engine.BoardSize * engine.BoardSize][int(engine.ColorCount) * int(engine.PieceKindCount)]uint64
	zobristSide   uint64
)

func initZobrist() {
	r := rand.New(rand.NewPCG(0x9E3779B97F4A7C15, 0xBF58476D1CE4E5B9))
	for i := range zobristPieces {
		for j := range zobristPieces[i] {
			zobristPieces[i][j] = r.Uint64()
		}
	}
	zobristSide = r.Uint64()
}

// ttKey returns a Zobrist hash of (board state, side-to-move).
func ttKey(g *engine.Game, maximize bool) uint64 {
	var h uint64
	for r := range engine.BoardSize {
		for c := range engine.BoardSize {
			p := g.Board[r][c]
			if p == nil {
				continue
			}
			cellIdx := r*engine.BoardSize + c
			pieceIdx := int(p.Color)*int(engine.PieceKindCount) + int(p.Kind)
			h ^= zobristPieces[cellIdx][pieceIdx]
		}
	}
	if maximize {
		h ^= zobristSide
	}
	return h
}

// abSearch implements alpha-beta minimax with a transposition table and
// child-ordering by static leaf eval. `me` is the scoring side; `maximize`
// toggles per ply; depth counts half-plies remaining.
//
// HINT run #13: added TT with bound flags. Entries cache exact scores and
// lower/upper bounds so re-entries within the same top-level move don't
// re-search the same subtree.
func abSearch(g *engine.Game, me engine.Color, depth int, alpha, beta int, maximize bool, tt *syncTT) int {
	if g.Status == engine.GameOver || depth == 0 {
		return evaluatePosition(g, me)
	}

	key := ttKey(g, maximize)
	if e, ok := tt.lookup(key); ok && e.depth >= depth {
		switch e.flag {
		case ttExact:
			return e.score
		case ttLower:
			if e.score > alpha {
				alpha = e.score
			}
		case ttUpper:
			if e.score < beta {
				beta = e.score
			}
		}
		if alpha >= beta {
			return e.score
		}
	}

	origAlpha, origBeta := alpha, beta

	mover := me
	if !maximize {
		mover = oppColor(me)
	}
	children := enumerate(g, mover)
	if len(children) == 0 {
		return evaluatePosition(g, me)
	}

	// Order children by cheap static eval (maximize desc / minimize asc).
	type scoredChild struct {
		c candidate
		v int
	}
	scored := make([]scoredChild, 0, len(children))
	for _, c := range children {
		s := g.Clone()
		if err := s.Move(c.piece, c.cell); err != nil {
			continue
		}
		scored = append(scored, scoredChild{c: c, v: evaluatePosition(s, me)})
	}
	if maximize {
		sort.Slice(scored, func(i, j int) bool { return scored[i].v > scored[j].v })
	} else {
		sort.Slice(scored, func(i, j int) bool { return scored[i].v < scored[j].v })
	}

	var best int
	if maximize {
		best = math.MinInt
		for _, sc := range scored {
			s := g.Clone()
			if err := s.Move(sc.c.piece, sc.c.cell); err != nil {
				continue
			}
			v := abSearch(s, me, depth-1, alpha, beta, false, tt)
			if v > best {
				best = v
			}
			if best > alpha {
				alpha = best
			}
			if alpha >= beta {
				break
			}
		}
	} else {
		best = math.MaxInt
		for _, sc := range scored {
			s := g.Clone()
			if err := s.Move(sc.c.piece, sc.c.cell); err != nil {
				continue
			}
			v := abSearch(s, me, depth-1, alpha, beta, true, tt)
			if v < best {
				best = v
			}
			if best < beta {
				beta = best
			}
			if alpha >= beta {
				break
			}
		}
	}

	// Store result with appropriate bound flag.
	var flag int
	switch {
	case best <= origAlpha:
		flag = ttUpper
	case best >= origBeta:
		flag = ttLower
	default:
		flag = ttExact
	}
	tt.store(key, ttEntry{depth: depth, score: best, flag: flag})

	return best
}

// evaluatePosition is the leaf heuristic for alpha-beta. Terminal positions
// score ±100k; otherwise: my line score minus opp line score, threats
// (3-in-a-line openings) weighted, and a small penalty per piece-in-hand
// differential so we don't sacrifice material freely (HINT run #12).
func evaluatePosition(g *engine.Game, me engine.Color) int {
	opp := oppColor(me)
	if g.Status == engine.GameOver {
		if g.Winner == nil {
			return 0
		}
		if *g.Winner == me {
			return 100000
		}
		return -100000
	}
	mine := lineBuildScore(&g.Board, me)
	theirs := lineBuildScore(&g.Board, opp)
	mineThreats := threats(&g.Board, me)
	oppThreats := threats(&g.Board, opp)
	myHand := handCount(g, me)
	oppHand := handCount(g, opp)
	return (mine - theirs) + 200*(mineThreats-oppThreats) - 3*(myHand-oppHand)
}

// handCount returns the number of pieces of the given color that are NOT on
// the board (i.e., in that player's hand).
func handCount(g *engine.Game, color engine.Color) int {
	n := 0
	for kind := engine.PieceKind(0); kind < engine.PieceKindCount; kind++ {
		if g.PieceInHand(engine.Piece{Color: color, Kind: kind}) {
			n++
		}
	}
	return n
}

// leadsToForcedWin reports whether our candidate guarantees a win within 2 of
// our plies. Symmetric to leadsToForcedLoss: for EVERY opponent response, we
// must have at least one reply that wins immediately. If any opp response has
// no winning reply available to us, the candidate cannot force a win.
func leadsToForcedWin(g *engine.Game, c candidate, me engine.Color) bool {
	sim := g.Clone()
	if err := sim.Move(c.piece, c.cell); err != nil {
		return false
	}
	if sim.Status == engine.GameOver && sim.Winner != nil && *sim.Winner == me {
		return true // immediate win
	}
	if sim.Status == engine.GameOver {
		return false
	}
	opp := oppColor(me)
	oppResponses := enumerate(sim, opp)
	if len(oppResponses) == 0 {
		return false // opp has no moves — weird; treat as non-win.
	}
	for _, oc := range oppResponses {
		s1 := sim.Clone()
		if err := s1.Move(oc.piece, oc.cell); err != nil {
			continue
		}
		if s1.Status == engine.GameOver {
			if s1.Winner == nil || *s1.Winner != me {
				return false // opp won or drew instead of letting us win
			}
			continue // bizarre: opp move ended with our win
		}
		// do WE have a winning reply in this branch?
		haveWin := false
		for _, myc := range enumerate(s1, me) {
			s2 := s1.Clone()
			if err := s2.Move(myc.piece, myc.cell); err != nil {
				continue
			}
			if s2.Status == engine.GameOver && s2.Winner != nil && *s2.Winner == me {
				haveWin = true
				break
			}
		}
		if !haveWin {
			return false
		}
	}
	return true
}

// leadsToForcedLoss reports whether our candidate allows the opponent to force
// a win within the next 2 plies. Performs a constrained 3-ply search: apply
// our candidate, enumerate opp responses, for each opp response enumerate our
// responses and check if opp has an immediate win after each. If any opp
// response leaves us without a safe reply, the candidate is a forced loss.
func leadsToForcedLoss(g *engine.Game, c candidate, me engine.Color) bool {
	sim := g.Clone()
	if err := sim.Move(c.piece, c.cell); err != nil {
		return true
	}
	if sim.Status == engine.GameOver {
		return sim.Winner == nil || *sim.Winner != me
	}
	opp := oppColor(me)
	for _, oc := range enumerate(sim, opp) {
		s1 := sim.Clone()
		if err := s1.Move(oc.piece, oc.cell); err != nil {
			continue
		}
		if s1.Status == engine.GameOver && s1.Winner != nil && *s1.Winner == opp {
			return true // opp has immediate win — our candidate lets them win in 1
		}
		// does every our reply lose to an opp-wins-in-1?
		ourReplies := enumerate(s1, me)
		if len(ourReplies) == 0 {
			return true
		}
		safe := false
		for _, myc := range ourReplies {
			s2 := s1.Clone()
			if err := s2.Move(myc.piece, myc.cell); err != nil {
				continue
			}
			if s2.Status == engine.GameOver && s2.Winner != nil && *s2.Winner == me {
				safe = true
				break
			}
			oppWinsIn1 := false
			for _, oc2 := range enumerate(s2, opp) {
				s3 := s2.Clone()
				if err := s3.Move(oc2.piece, oc2.cell); err != nil {
					continue
				}
				if s3.Status == engine.GameOver && s3.Winner != nil && *s3.Winner == opp {
					oppWinsIn1 = true
					break
				}
			}
			if !oppWinsIn1 {
				safe = true
				break
			}
		}
		if !safe {
			return true
		}
	}
	return false
}

func moveFingerprint(g *engine.Game, piece engine.Piece, cell engine.Cell) string {
	var sb strings.Builder
	for r := range engine.BoardSize {
		for c := range engine.BoardSize {
			p := g.Board[r][c]
			if p == nil {
				sb.WriteByte('.')
			} else {
				sb.WriteString(pieceCode(*p))
			}
		}
	}
	sb.WriteByte('|')
	sb.WriteString(pieceCode(piece))
	sb.WriteByte('@')
	sb.WriteString(cellToAlgebraic(cell))
	return sb.String()
}

func enumerate(g *engine.Game, me engine.Color) []candidate {
	var out []candidate
	for kind := engine.PieceKind(0); kind < engine.PieceKindCount; kind++ {
		p := engine.Piece{Color: me, Kind: kind}
		if g.PieceInHand(p) {
			// placement: any empty cell
			for r := 0; r < engine.BoardSize; r++ {
				for c := 0; c < engine.BoardSize; c++ {
					if g.Board[r][c] == nil {
						out = append(out, candidate{piece: p, cell: engine.Cell{Row: r, Col: c}})
					}
				}
			}
		} else {
			for _, cell := range g.LegalMoves(p) {
				out = append(out, candidate{piece: p, cell: cell})
			}
		}
	}
	return out
}

func scoreCandidate(g *engine.Game, c candidate, me engine.Color, hints []string) (int, string) {
	// simulate
	sim := g.Clone()
	if err := sim.Move(c.piece, c.cell); err != nil {
		return math.MinInt, "illegal"
	}
	// immediate win
	if sim.Status == engine.GameOver && sim.Winner != nil && *sim.Winner == me {
		return 100000, "wins"
	}
	score := 0
	reasons := []string{}

	// capture bonus
	if target := g.Board.At(c.cell); target != nil && target.Color != me {
		score += 60
		reasons = append(reasons, "capture "+pieceCode(*target))
	}

	// opponent response check — can they win on their next move?
	opp := oppColor(me)
	oppCands := enumerate(sim, opp)
	oppWins := false
	oppBestScore := 0
	for _, oc := range oppCands {
		os := sim.Clone()
		if err := os.Move(oc.piece, oc.cell); err != nil {
			continue
		}
		if os.Status == engine.GameOver && os.Winner != nil && *os.Winner == opp {
			oppWins = true
			break
		}
		// rough: does opponent create 3-in-a-line with empty 4th?
		s := lineBuildScore(&os.Board, opp)
		if s > oppBestScore {
			oppBestScore = s
		}
	}
	if oppWins {
		// this move lets opponent win; catastrophic
		return -80000, "opponent wins next"
	}
	// penalize strong opponent responses
	score -= oppBestScore

	// our line-building (HINT run #8: reverted x3 opening boost — caused 1 loss
	// by thinning defensive reserves mid-game). Simple: x2 when opp has no
	// 2-in-a-line anywhere, else x1.
	ours := lineBuildScore(&sim.Board, me)
	if twoLineCount(&g.Board, opp) == 0 {
		ours *= 2
	}
	score += ours
	if ours > 0 {
		reasons = append(reasons, fmt.Sprintf("ours+%d", ours))
	}

	// fork bonus: if this move creates ≥2 new lines of exactly 2 mine with
	// 0 opp, give +40. Forks force the opponent to choose which to block.
	before2s := twoLineCount(&g.Board, me)
	after2s := twoLineCount(&sim.Board, me)
	if after2s-before2s >= 2 {
		score += 40
		reasons = append(reasons, fmt.Sprintf("fork+40(+%d 2-lines)", after2s-before2s))
	}

	// blocks an opponent threat? check if before our move there was a 3-in-a-line
	// for opp and now that cell is ours
	beforeOpp := threats(&g.Board, opp)
	afterOpp := threats(&sim.Board, opp)
	if afterOpp < beforeOpp {
		score += 40 * (beforeOpp - afterOpp)
		reasons = append(reasons, "blocks")
	}

	// center bonus (inner 2x2)
	if (c.cell.Row == 1 || c.cell.Row == 2) && (c.cell.Col == 1 || c.cell.Col == 2) {
		score += 4
	}

	// Critical-line defense (HINT run #4: bot switched from diagonals to columns
	// when we hardened only the diagonal. Generalized across all 10 lines.)
	// For each line where opp has ≥2 pieces:
	//   - mine == 0: LAND on an empty cell (+80/opp); ignoring (-50/(opp-1)).
	//   - mine == 1 (FRAGILE): abandoning the sole defender → -200;
	//     landing on an empty cell reinforces (+60/opp).
	for _, line := range allLines() {
		oppOnLine, emptyOnLine, mineOnLine := countLine(&g.Board, line, opp, me)
		if oppOnLine < 2 {
			continue
		}
		landsOnLine := false
		for _, cell := range line {
			if cell == c.cell {
				landsOnLine = true
				break
			}
		}

		switch mineOnLine {
		case 0:
			if landsOnLine {
				bonus := 80 * oppOnLine
				score += bonus
				reasons = append(reasons, fmt.Sprintf("line-defend+%d", bonus))
			} else if emptyOnLine > 0 {
				penalty := 50 * (oppOnLine - 1)
				score -= penalty
				reasons = append(reasons, fmt.Sprintf("line-ignored-%d", penalty))
			}
		case 1:
			var defenderCell engine.Cell
			for _, cell := range line {
				if p := g.Board.At(cell); p != nil && p.Color == me {
					defenderCell = cell
					break
				}
			}
			defenderAfter := sim.Board.At(defenderCell)
			if defenderAfter == nil || defenderAfter.Color != me {
				score -= 200
				reasons = append(reasons, "line-abandon-200")
			} else if landsOnLine {
				bonus := 60 * oppOnLine
				score += bonus
				reasons = append(reasons, fmt.Sprintf("line-reinforce+%d", bonus))
			}

			// Attacker-on-defender precheck (HINT run #6): if any opp piece on
			// the board can capture our defender next move, this line is on the
			// verge of completion. Boost moves that remove the attacker or
			// reinforce the line.
			var attackerCell engine.Cell
			attackerFound := false
			for kind := engine.PieceKind(0); kind < engine.PieceKindCount; kind++ {
				oppPiece := engine.Piece{Color: opp, Kind: kind}
				if g.PieceInHand(oppPiece) {
					continue
				}
				for _, target := range g.LegalMoves(oppPiece) {
					if target == defenderCell {
						if cell, ok := g.Board.Find(g.Piece(oppPiece)); ok {
							attackerCell = cell
							attackerFound = true
						}
						break
					}
				}
				if attackerFound {
					break
				}
			}
			if attackerFound {
				switch {
				case c.cell == attackerCell:
					score += 120
					reasons = append(reasons, "attacker-capture+120")
				case landsOnLine:
					score += 80
					reasons = append(reasons, "attacker-reinforce+80")
				default:
					score -= 60
					reasons = append(reasons, "attacker-left-60")
				}
			}
		}
	}

	// knight on corner: slight negative (fewer moves)
	if c.piece.Kind == engine.Knight {
		corners := map[engine.Cell]bool{
			{Row: 0, Col: 0}: true,
			{Row: 0, Col: 3}: true,
			{Row: 3, Col: 0}: true,
			{Row: 3, Col: 3}: true,
		}
		if corners[c.cell] {
			score -= 5
		}
	}
	_ = hints

	return score, strings.Join(reasons, ",")
}

// lineBuildScore: sum over lines of weighted piece count if no opposing piece blocks.
// Diagonals are weighted 2× because the hard bot's primary plan is an anti-diagonal
// build-up (hint file, game #2 loss analysis).
func lineBuildScore(b *engine.Board, color engine.Color) int {
	score := 0
	lines := b.Lines()
	for i, line := range lines {
		myCount, oppCount := 0, 0
		for _, p := range line {
			if p == nil {
				continue
			}
			if p.Color == color {
				myCount++
			} else {
				oppCount++
			}
		}
		if oppCount > 0 {
			continue
		}
		var base int
		switch myCount {
		case 2:
			base = 8
		case 3:
			base = 25
		case 4:
			base = 10000
		}
		if isDiagonal(i) {
			base *= 2
		}
		score += base
	}
	return score
}

// isDiagonal reports whether the line index from Board.Lines() is one of the
// two diagonals (last two entries). Board.Lines() returns 4 rows, 4 cols, then
// main + anti diagonals.
func isDiagonal(lineIndex int) bool {
	return lineIndex >= 2*engine.BoardSize
}

// allLines returns all 10 winning lines (4 rows + 4 cols + 2 diagonals) as
// explicit Cell slices, parallel to engine.Board.Lines() but with coordinates
// so we can query c.cell membership and defender positions.
func allLines() [][engine.BoardSize]engine.Cell {
	lines := make([][engine.BoardSize]engine.Cell, 0, 2*engine.BoardSize+2)
	for r := range engine.BoardSize {
		var line [engine.BoardSize]engine.Cell
		for c := range engine.BoardSize {
			line[c] = engine.Cell{Row: r, Col: c}
		}
		lines = append(lines, line)
	}
	for c := range engine.BoardSize {
		var line [engine.BoardSize]engine.Cell
		for r := range engine.BoardSize {
			line[r] = engine.Cell{Row: r, Col: c}
		}
		lines = append(lines, line)
	}
	var main, anti [engine.BoardSize]engine.Cell
	for i := range engine.BoardSize {
		main[i] = engine.Cell{Row: i, Col: i}
		anti[i] = engine.Cell{Row: i, Col: engine.BoardSize - i - 1}
	}
	lines = append(lines, main, anti)
	return lines
}

// countLine returns (opp, empty, mine) counts on the given line cells.
func countLine(b *engine.Board, line [engine.BoardSize]engine.Cell, opp, me engine.Color) (oppCount, emptyCount, mineCount int) {
	for _, c := range line {
		p := b.At(c)
		switch {
		case p == nil:
			emptyCount++
		case p.Color == opp:
			oppCount++
		case p.Color == me:
			mineCount++
		}
	}
	return
}

// threats: number of lines where color has 3 pieces and none opposing.
func threats(b *engine.Board, color engine.Color) int {
	n := 0
	for _, line := range b.Lines() {
		myCount, oppCount := 0, 0
		for _, p := range line {
			if p == nil {
				continue
			}
			if p.Color == color {
				myCount++
			} else {
				oppCount++
			}
		}
		if myCount == 3 && oppCount == 0 {
			n++
		}
	}
	return n
}

// countPiecesOnBoard returns the total number of pieces (any color) on the board.
func countPiecesOnBoard(b *engine.Board) int {
	n := 0
	for r := range engine.BoardSize {
		for c := range engine.BoardSize {
			if b[r][c] != nil {
				n++
			}
		}
	}
	return n
}

// twoLineCount: lines where color has exactly 2 pieces and none opposing.
// Used for fork detection.
func twoLineCount(b *engine.Board, color engine.Color) int {
	n := 0
	for _, line := range b.Lines() {
		myCount, oppCount := 0, 0
		for _, p := range line {
			if p == nil {
				continue
			}
			if p.Color == color {
				myCount++
			} else {
				oppCount++
			}
		}
		if myCount == 2 && oppCount == 0 {
			n++
		}
	}
	return n
}

func oppColor(c engine.Color) engine.Color {
	if c == engine.White {
		return engine.Black
	}
	return engine.White
}

// -----------------------------------------------------------------------------
// conversions
// -----------------------------------------------------------------------------

func reconstruct(p gameStatePayload) (*engine.Game, error) {
	g := engine.NewGame()
	for r, row := range p.Board {
		for c, cell := range row {
			if cell == nil {
				continue
			}
			col, err := colorFromStr(cell.Color)
			if err != nil {
				return nil, err
			}
			kind, err := kindFromStr(cell.Kind)
			if err != nil {
				return nil, err
			}
			g.Board[r][c] = g.Pieces.Get(col, kind)
		}
	}
	turn, err := colorFromStr(p.Turn)
	if err != nil {
		return nil, err
	}
	g.Turn = turn
	switch p.Status {
	case "started":
		g.Status = engine.GameStarted
	case "over":
		g.Status = engine.GameOver
	default:
		return nil, fmt.Errorf("unknown status: %s", p.Status)
	}
	if p.Winner != nil {
		w, err := colorFromStr(*p.Winner)
		if err != nil {
			return nil, err
		}
		g.Winner = &w
	}
	if d, err := pawnDirFromStr(p.PawnDirections.White); err == nil {
		g.PawnDirections[engine.White] = d
	}
	if d, err := pawnDirFromStr(p.PawnDirections.Black); err == nil {
		g.PawnDirections[engine.Black] = d
	}
	return g, nil
}

func colorFromStr(s string) (engine.Color, error) {
	switch s {
	case "white":
		return engine.White, nil
	case "black":
		return engine.Black, nil
	}
	return 0, fmt.Errorf("bad color %q", s)
}

func kindFromStr(s string) (engine.PieceKind, error) {
	switch s {
	case "pawn":
		return engine.Pawn, nil
	case "rook":
		return engine.Rook, nil
	case "bishop":
		return engine.Bishop, nil
	case "knight":
		return engine.Knight, nil
	}
	return 0, fmt.Errorf("bad kind %q", s)
}

func pawnDirFromStr(s string) (engine.PawnDirection, error) {
	switch s {
	case "toBlackSide":
		return engine.ToBlackSide, nil
	case "toWhiteSide":
		return engine.ToWhiteSide, nil
	}
	return 0, fmt.Errorf("bad dir %q", s)
}

func pieceCode(p engine.Piece) string {
	var c, k byte
	if p.Color == engine.White {
		c = 'W'
	} else {
		c = 'B'
	}
	switch p.Kind {
	case engine.Pawn:
		k = 'P'
	case engine.Rook:
		k = 'R'
	case engine.Bishop:
		k = 'B'
	case engine.Knight:
		k = 'N'
	}
	return string([]byte{c, k})
}

func cellToAlgebraic(c engine.Cell) string {
	file := 'a' + c.Col
	rank := '4' - c.Row
	return string([]byte{byte(file), byte(rank)})
}

// -----------------------------------------------------------------------------
// HTTP + WS wiring
// -----------------------------------------------------------------------------

func createClient(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", *baseURL+"/api/clients", nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("status=%d body=%s", resp.StatusCode, string(body))
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return out.Token, nil
}

func createBotGame(ctx context.Context, token string) (string, error) {
	url := fmt.Sprintf("%s/api/bot-game?difficulty=%s", *baseURL, *difficulty)
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("status=%d body=%s", resp.StatusCode, string(body))
	}
	var out struct {
		RoomID string `json:"roomId"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return out.RoomID, nil
}

func sendMove(ctx context.Context, conn *websocket.Conn, p engine.Piece, cell engine.Cell) error {
	payload := map[string]string{
		"type":  "move",
		"piece": pieceCode(p),
		"to":    cellToAlgebraic(cell),
	}
	buf, _ := json.Marshal(payload)
	return conn.Write(ctx, websocket.MessageText, buf)
}

// -----------------------------------------------------------------------------
// hints + log persistence
// -----------------------------------------------------------------------------

func loadHints(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var hints []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "- ") {
			hints = append(hints, strings.TrimPrefix(line, "- "))
		}
	}
	return hints
}

func writeLog(gl *gameLog) {
	path := *logPath
	if path == "" {
		path = fmt.Sprintf("tmp/games/%s.json", gl.StartedAt.Format("20060102-150405"))
	}
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetIndent("", "  ")
	_ = enc.Encode(gl)
	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "write log: %v\n", err)
		return
	}
	fmt.Printf("log: %s\n", path)
}
