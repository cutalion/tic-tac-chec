package bot

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"tic-tac-chec/engine"
	"tic-tac-chec/internal/game"
	"time"

	ort "github.com/yalue/onnxruntime_go"
)

// Bot plays Tic Tac Chec using an ONNX neural network model.
type Bot struct {
	session *ort.DynamicAdvancedSession
}

// New creates a Bot that loads the ONNX model from the given path.
// Call ort.InitializeEnvironment() before creating a Bot,
// and ort.DestroyEnvironment() when done.
func New(modelPath string) (*Bot, error) {
	session, err := ort.NewDynamicAdvancedSession(
		modelPath,
		[]string{"state"},
		[]string{"action_logits", "state_value"},
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("bot: load model: %w", err)
	}

	return &Bot{session: session}, nil
}

// Infer runs the model on a game state and returns action logits (320 floats).
func (b *Bot) Infer(state []float32) ([]float32, error) {
	inputShape := ort.Shape{1, NumChannels, BoardSize, BoardSize}
	input, err := ort.NewTensor(inputShape, state)
	if err != nil {
		return nil, fmt.Errorf("bot: create input tensor: %w", err)
	}
	defer input.Destroy()

	outputShape := ort.Shape{1, ActionSpaceSize}
	output, err := ort.NewEmptyTensor[float32](outputShape)
	if err != nil {
		return nil, fmt.Errorf("bot: create output tensor: %w", err)
	}
	defer output.Destroy()

	// We need a second output for state_value even though we don't use it
	valueShape := ort.Shape{1, 1}
	valueOutput, err := ort.NewEmptyTensor[float32](valueShape)
	if err != nil {
		return nil, fmt.Errorf("bot: create value tensor: %w", err)
	}
	defer valueOutput.Destroy()

	err = b.session.Run([]ort.ArbitraryTensor{input}, []ort.ArbitraryTensor{output, valueOutput})
	if err != nil {
		return nil, fmt.Errorf("bot: run inference: %w", err)
	}

	logits := make([]float32, ActionSpaceSize)
	copy(logits, output.GetData())
	return logits, nil
}

// InferWithValue runs the model and returns both action logits (320 floats)
// and the state value estimate (single float).
func (b *Bot) InferWithValue(state []float32) ([]float32, float32, error) {
	inputShape := ort.Shape{1, NumChannels, BoardSize, BoardSize}
	input, err := ort.NewTensor(inputShape, state)
	if err != nil {
		return nil, 0, fmt.Errorf("bot: create input tensor: %w", err)
	}
	defer input.Destroy()

	outputShape := ort.Shape{1, ActionSpaceSize}
	output, err := ort.NewEmptyTensor[float32](outputShape)
	if err != nil {
		return nil, 0, fmt.Errorf("bot: create output tensor: %w", err)
	}
	defer output.Destroy()

	valueShape := ort.Shape{1, 1}
	valueOutput, err := ort.NewEmptyTensor[float32](valueShape)
	if err != nil {
		return nil, 0, fmt.Errorf("bot: create value tensor: %w", err)
	}
	defer valueOutput.Destroy()

	err = b.session.Run([]ort.ArbitraryTensor{input}, []ort.ArbitraryTensor{output, valueOutput})
	if err != nil {
		return nil, 0, fmt.Errorf("bot: run inference: %w", err)
	}

	logits := make([]float32, ActionSpaceSize)
	copy(logits, output.GetData())
	value := valueOutput.GetData()[0]
	return logits, value, nil
}

// SelectAction picks the best legal action given logits and a game state.
// Applies action masking: illegal actions get -inf, then picks argmax.
func (b *Bot) SelectAction(g *engine.Game) (engine.Piece, engine.Cell, error) {
	state := NewStateEncoder().Encode(g)

	logits, err := b.Infer(state)
	if err != nil {
		return engine.Piece{}, engine.Cell{}, err
	}

	// Build legal action set
	legal := legalActions(g)

	// Mask illegal actions and find argmax
	bestAction := -1
	bestScore := float32(math.Inf(-1))
	for _, action := range legal {
		if logits[action] > bestScore {
			bestScore = logits[action]
			bestAction = action
		}
	}

	if bestAction == -1 {
		return engine.Piece{}, engine.Cell{}, fmt.Errorf("bot: no legal actions")
	}

	piece, src, dst, isDrop := DecodeAction(bestAction, g.Turn)
	if isDrop {
		return piece, dst, nil
	}

	// For moves, find the piece at the source cell
	boardPiece := g.Board[src.Row][src.Col]
	if boardPiece == nil {
		return engine.Piece{}, engine.Cell{}, fmt.Errorf("bot: no piece at source %v", src)
	}

	return *boardPiece, dst, nil
}

// legalActions returns valid action indices for the current player.
func legalActions(g *engine.Game) []int {
	var actions []int

	// Drop actions (0-63)
	for kindIdx := range int(engine.PieceKindCount) {
		piece := engine.Piece{Color: g.Turn, Kind: engine.PieceKind(kindIdx)}
		if !g.PieceInHand(piece) {
			continue
		}
		for row := range BoardSize {
			for col := range BoardSize {
				if g.Board[row][col] == nil {
					actions = append(actions, kindIdx*16+row*4+col)
				}
			}
		}
	}

	// Move actions (64-319)
	for row := range BoardSize {
		for col := range BoardSize {
			p := g.Board[row][col]
			if p == nil || p.Color != g.Turn {
				continue
			}

			moves := g.LegalMoves(*p)
			srcIdx := row*BoardSize + col
			for _, target := range moves {
				dstIdx := target.Row*BoardSize + target.Col
				actions = append(actions, 64+srcIdx*16+dstIdx)
			}
		}
	}

	return actions
}

// RunPlayer creates a game.Player backed by the bot and starts a goroutine
// that listens for game events and responds with moves.
func (b *Bot) RunPlayer() game.Player {
	commands := make(chan game.Command, 2)
	player := game.NewPlayer(commands)

	go b.playLoop(&player, commands)

	return player
}

// playLoop is the bot's event loop goroutine. It listens for game events
// and responds with moves when it's the bot's turn.
func (b *Bot) playLoop(player *game.Player, commands chan game.Command) {
	botColor := engine.White
	botPlayerID := player.ID

	for event := range player.Updates {
		switch e := event.(type) {
		case game.PairedEvent:
			botColor = e.Color
		case game.SnapshotEvent:
			if e.Game.Status == engine.GameOver {
				time.Sleep(500 * time.Millisecond)
				emoji := game.ReactionEmojis[rand.Intn(len(game.ReactionEmojis))]
				commands <- game.ReactionCommand{PlayerID: botPlayerID, Reaction: emoji}
				commands <- game.RematchCommand{PlayerID: botPlayerID}
				continue
			}

			if e.Game.Turn == botColor {
				time.Sleep(time.Duration(300+rand.Intn(700)) * time.Millisecond)
				piece, cell, err := b.SelectAction(&e.Game)
				if err != nil {
					log.Printf("bot: SelectAction error: %v", err)
					continue
				}
				commands <- game.MoveCommand{Piece: piece, To: cell}
			}
		case game.RematchRequestedEvent:
			commands <- game.RematchCommand{PlayerID: botPlayerID}
		default:
			// ignore
		}
	}
}

func (b *Bot) Destroy() {
	if b.session != nil {
		b.session.Destroy()
	}
}
