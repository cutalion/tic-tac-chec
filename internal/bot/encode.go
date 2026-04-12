package bot

import (
	"tic-tac-chec/engine"
)

const (
	NumChannels     = 19
	BoardSize       = engine.BoardSize
	StateSize       = NumChannels * BoardSize * BoardSize // 19 * 4 * 4 = 304
	ActionSpaceSize = 320
)

// pieceOrder defines the channel index for each piece.
// Must match Python's ALL_PIECES order exactly.
var pieceOrder = [8]engine.Piece{
	{Color: engine.White, Kind: engine.Pawn},   // channel 0, 8
	{Color: engine.White, Kind: engine.Rook},   // channel 1, 9
	{Color: engine.White, Kind: engine.Bishop}, // channel 2, 10
	{Color: engine.White, Kind: engine.Knight}, // channel 3, 11
	{Color: engine.Black, Kind: engine.Pawn},   // channel 4, 12
	{Color: engine.Black, Kind: engine.Rook},   // channel 5, 13
	{Color: engine.Black, Kind: engine.Bishop}, // channel 6, 14
	{Color: engine.Black, Kind: engine.Knight}, // channel 7, 15
}

type StateEncoder struct {
	state []float32
}

func NewStateEncoder() *StateEncoder {
	return &StateEncoder{
		state: make([]float32, StateSize),
	}
}

// Encode converts an engine.Game to a flat float32 slice of length 304
// (19 channels × 4 rows × 4 cols) matching the Python encode_state() layout.
//
// Channel layout (from data-model.md):
//
//	0-7:   Piece positions (1.0 at piece location, 0.0 elsewhere)
//	8-15:  Hand status (all 1.0 if piece in hand, all 0.0 if on board)
//	16:    Turn indicator (all 1.0 if White's turn, all 0.0 if Black's)
//	17:    White pawn direction (all 1.0 if ToBlackSide, all 0.0 if ToWhiteSide)
//	18:    Black pawn direction (all 1.0 if ToBlackSide, all 0.0 if ToWhiteSide)
//
// Memory layout: channel-major order [ch][row][col] flattened as:
//
//	index = ch * 16 + row * 4 + col
func (e *StateEncoder) Encode(g *engine.Game) []float32 {
	// Piece positions
	// ch - index in pieceOrder, 0-7
	for ch, p := range pieceOrder {
		if piece := g.Piece(p); piece != nil {
			cell, ok := g.Board.Find(piece)
			if ok { // piece is on board
				e.setChannelBit(ch, cell.Row, cell.Col, 1.0)
			} else { // piece is in hand
				e.fillChannel(ch+8, 1.0)
			}
		} else {
			panic("piece not found")
		}
	}

	if g.Turn == engine.White {
		e.fillChannel(16, 1.0)
	}

	if g.PawnDirections[engine.White] == engine.ToBlackSide {
		e.fillChannel(17, 1.0)
	}
	if g.PawnDirections[engine.Black] == engine.ToBlackSide {
		e.fillChannel(18, 1.0)
	}

	return e.state
}

func (e *StateEncoder) fillChannel(ch int, val float32) {
	for row := range 4 {
		for col := range 4 {
			e.state[ch*16+row*4+col] = val
		}
	}
}

func (e *StateEncoder) setChannelBit(ch, row, col int, val float32) {
	e.state[ch*16+row*4+col] = val
}
