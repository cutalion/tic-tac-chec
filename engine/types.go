package engine

type Game struct {
	Board          Board
	Pieces         Pieces
	Turn           Color
	PawnDirections PawnDirections
	Status         GameStatus
	Winner         *Color
}

const BoardSize = 4

type Board [BoardSize][BoardSize]*Piece
type Line []*Piece

type Cell struct {
	Row int
	Col int
}

type (
	Color     int
	PieceKind int
)

const (
	Pawn PieceKind = iota
	Rook
	Bishop
	Knight
	PieceKindCount
)

func (k PieceKind) String() string {
	switch k {
	case Pawn:
		return "P"
	case Rook:
		return "R"
	case Bishop:
		return "B"
	case Knight:
		return "N" // K is used for King in chess. We don't have King, but anyway
	}

	panic("unknown piece kind")
}

const (
	White Color = iota
	Black
	ColorCount
)

func (c Color) String() string {
	switch c {
	case White:
		return "W"
	case Black:
		return "B"
	}

	panic("unknown color")
}

const PieceCount = int(PieceKindCount) * int(ColorCount)

type Piece struct {
	Color Color
	Kind  PieceKind
}

type Pieces [ColorCount][PieceKindCount]Piece

type BoardSide int

const (
	BlackSide BoardSide = 0 // 0 row at the top
	WhiteSide BoardSide = BoardSize - 1
)

type GameStatus int

const (
	GameStarted GameStatus = iota
	GameOver
)

type PawnDirections [ColorCount]PawnDirection
type PawnDirection int

const (
	ToBlackSide PawnDirection = -1
	ToWhiteSide PawnDirection = 1
)
