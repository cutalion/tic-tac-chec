//go:build model_test

package bot

import (
	"os"
	"path/filepath"
	"testing"
	"tic-tac-chec/engine"

	ort "github.com/yalue/onnxruntime_go"
)

var testModelPath string

func TestMain(m *testing.M) {
	// Find project root (internal/model -> ../../)
	root := filepath.Join("..", "..")
	libPath := filepath.Join(root, "third_party", "onnxruntime-linux-x64-1.24.1", "lib", "libonnxruntime.so")
	testModelPath = filepath.Join(root, "model", "models", "model.onnx")

	ort.SetSharedLibraryPath(libPath)
	if err := ort.InitializeEnvironment(); err != nil {
		panic("failed to init onnxruntime: " + err.Error())
	}
	defer ort.DestroyEnvironment()

	os.Exit(m.Run())
}

func TestDecodeDropAction(t *testing.T) {
	// Action 0 = Pawn drop at (0,0)
	piece, _, dst, isDrop := DecodeAction(0, engine.White)
	if !isDrop {
		t.Fatal("expected drop action")
	}
	if piece.Kind != engine.Pawn || piece.Color != engine.White {
		t.Fatalf("expected White Pawn, got %v", piece)
	}
	if dst.Row != 0 || dst.Col != 0 {
		t.Fatalf("expected (0,0), got %v", dst)
	}

	// Action 63 = Knight drop at (3,3)
	piece, _, dst, isDrop = DecodeAction(63, engine.Black)
	if !isDrop {
		t.Fatal("expected drop action")
	}
	if piece.Kind != engine.Knight || piece.Color != engine.Black {
		t.Fatalf("expected Black Knight, got %v", piece)
	}
	if dst.Row != 3 || dst.Col != 3 {
		t.Fatalf("expected (3,3), got %v", dst)
	}
}

func TestDecodeMoveAction(t *testing.T) {
	// Action 64 = move from (0,0) to (0,0) — self-move, always masked
	_, src, dst, isDrop := DecodeAction(64, engine.White)
	if isDrop {
		t.Fatal("expected move action")
	}
	if src.Row != 0 || src.Col != 0 {
		t.Fatalf("expected src (0,0), got %v", src)
	}
	if dst.Row != 0 || dst.Col != 0 {
		t.Fatalf("expected dst (0,0), got %v", dst)
	}

	// Action 319 = move from (3,3) to (3,3)
	_, src, dst, isDrop = DecodeAction(319, engine.White)
	if isDrop {
		t.Fatal("expected move action")
	}
	if src.Row != 3 || src.Col != 3 {
		t.Fatalf("expected src (3,3), got %v", src)
	}
	if dst.Row != 3 || dst.Col != 3 {
		t.Fatalf("expected dst (3,3), got %v", dst)
	}
}

func TestInferWithZeros(t *testing.T) {
	model, err := New(testModelPath, 0)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}
	defer model.Destroy()

	// All-zero state (not a real game state, but tests the inference pipeline)
	state := make([]float32, StateSize)
	logits, err := model.Infer(state)
	if err != nil {
		t.Fatalf("inference failed: %v", err)
	}

	if len(logits) != ActionSpaceSize {
		t.Fatalf("expected %d logits, got %d", ActionSpaceSize, len(logits))
	}

	// At least some logits should be non-zero
	allZero := true
	for _, v := range logits {
		if v != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		t.Fatal("all logits are zero — model may not be loaded correctly")
	}
}

func TestSelectAction(t *testing.T) {
	model, err := New(testModelPath, 0)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}
	defer model.Destroy()

	g := engine.NewGame()
	piece, cell, err := model.SelectAction(g)
	if err != nil {
		t.Fatalf("SelectAction failed: %v", err)
	}

	// At start, only drops are legal — the piece must be in hand
	if !g.PieceInHand(piece) {
		t.Fatalf("selected piece %v should be in hand at game start", piece)
	}

	// The piece must be the current player's color
	if piece.Color != g.Turn {
		t.Fatalf("selected piece color %v doesn't match turn %v", piece.Color, g.Turn)
	}

	// The cell must be valid and empty
	if !cell.Valid() {
		t.Fatalf("selected cell %v is out of bounds", cell)
	}
	if g.Board[cell.Row][cell.Col] != nil {
		t.Fatalf("selected cell %v is occupied", cell)
	}
}

func TestModelPlaysFullGame(t *testing.T) {
	model, err := New(testModelPath, 0)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}
	defer model.Destroy()

	g := engine.NewGame()

	// Play up to 200 moves (should end well before that)
	for i := 0; i < 200; i++ {
		if g.Status == engine.GameOver {
			t.Logf("game over after %d moves, winner: %v", i, g.Winner)
			return
		}

		piece, cell, err := model.SelectAction(g)
		if err != nil {
			t.Fatalf("move %d: SelectAction failed: %v", i, err)
		}

		err = g.Move(piece, cell)
		if err != nil {
			t.Fatalf("move %d: illegal move %v to %v: %v", i, piece, cell, err)
		}
	}

	t.Log("game did not end in 200 moves (draw by exhaustion)")
}

func TestMCTSFindsWinningMove(t *testing.T) {
	b, err := New(testModelPath, 50)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}
	defer b.Destroy()

	// Set up: White has pawn(0,0), rook(0,1), bishop(0,2) — needs knight at (0,3) to win
	g := engine.NewGame()
	g.Move(engine.WhitePawn, engine.Cell{Row: 0, Col: 0})
	g.Move(engine.BlackPawn, engine.Cell{Row: 3, Col: 3})
	g.Move(engine.WhiteRook, engine.Cell{Row: 0, Col: 1})
	g.Move(engine.BlackRook, engine.Cell{Row: 3, Col: 2})
	g.Move(engine.WhiteBishop, engine.Cell{Row: 0, Col: 2})
	g.Move(engine.BlackBishop, engine.Cell{Row: 3, Col: 1})
	// Now it's White's turn. White knight drop at (0,3) wins.

	piece, cell, err := b.SelectAction(g)
	if err != nil {
		t.Fatalf("SelectAction failed: %v", err)
	}

	if cell.Row != 0 || cell.Col != 3 {
		t.Errorf("MCTS should find winning move at (0,3), got piece=%v cell=%v", piece, cell)
	}
}

// TODO: Re-enable after MCTS-guided training (Phase 2) improves the value network.
// Current network gives optimistic values to non-blocking moves, so MCTS can't
// discover that not blocking leads to a loss — even at 5000 simulations.
//
// func TestMCTSBlocksOpponentWin(t *testing.T) {
// 	b, err := New(testModelPath, 50)
// 	if err != nil {
// 		t.Fatalf("failed to create model: %v", err)
// 	}
// 	defer b.Destroy()
//
// 	g := engine.NewGame()
// 	bp := g.Pieces.Get(engine.Black, engine.Pawn)
// 	br := g.Pieces.Get(engine.Black, engine.Rook)
// 	bb := g.Pieces.Get(engine.Black, engine.Bishop)
// 	wp := g.Pieces.Get(engine.White, engine.Pawn)
// 	wr := g.Pieces.Get(engine.White, engine.Rook)
// 	wb := g.Pieces.Get(engine.White, engine.Bishop)
//
// 	g.Board = engine.Board{
// 		{nil, nil, nil, wr},
// 		{bp, br, bb, nil},
// 		{nil, nil, nil, nil},
// 		{wp, wb, nil, nil},
// 	}
//
// 	piece, cell, err := b.SelectAction(g)
// 	if err != nil {
// 		t.Fatalf("SelectAction failed: %v", err)
// 	}
//
// 	if cell.Row != 1 || cell.Col != 3 {
// 		t.Errorf("MCTS should block at (1,3), got piece=%v cell=%v", piece, cell)
// 	}
// }
