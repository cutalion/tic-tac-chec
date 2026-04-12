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
	// Find project root (internal/bot -> ../../)
	root := filepath.Join("..", "..")
	libPath := filepath.Join(root, "third_party", "onnxruntime-linux-x64-1.24.1", "lib", "libonnxruntime.so")
	testModelPath = filepath.Join(root, "bot", "models", "bot.onnx")

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
	bot, err := New(testModelPath)
	if err != nil {
		t.Fatalf("failed to create bot: %v", err)
	}
	defer bot.Destroy()

	// All-zero state (not a real game state, but tests the inference pipeline)
	state := make([]float32, StateSize)
	logits, err := bot.Infer(state)
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
	bot, err := New(testModelPath)
	if err != nil {
		t.Fatalf("failed to create bot: %v", err)
	}
	defer bot.Destroy()

	g := engine.NewGame()
	piece, cell, err := bot.SelectAction(g)
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

func TestBotPlaysFullGame(t *testing.T) {
	bot, err := New(testModelPath)
	if err != nil {
		t.Fatalf("failed to create bot: %v", err)
	}
	defer bot.Destroy()

	g := engine.NewGame()

	// Play up to 200 moves (should end well before that)
	for i := 0; i < 200; i++ {
		if g.Status == engine.GameOver {
			t.Logf("game over after %d moves, winner: %v", i, g.Winner)
			return
		}

		piece, cell, err := bot.SelectAction(g)
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
