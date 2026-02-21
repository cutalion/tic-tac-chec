package engine

type Direction [2]int

func (g *Game) pieceMoves(piece *Piece) ([]Cell, error) {
	switch piece.Kind {
	case Pawn:
		return g.pawnMoves(piece)
	case Rook:
		return g.rookMoves(piece)
	case Bishop:
		return g.bishopMoves(piece)
	case Knight:
		return g.knightMoves(piece)
	default:
		panic("invalid piece kind")
	}
}

func (g *Game) rookMoves(rook *Piece) ([]Cell, error) {
	left := Direction{0, -1}
	right := Direction{0, 1}
	up := Direction{-1, 0}
	down := Direction{1, 0}
	directions := []Direction{right, down, left, up}

	return g.slideMoves(rook, directions)
}

func (g *Game) bishopMoves(bishop *Piece) ([]Cell, error) {
	upLeft := Direction{-1, -1}
	upRight := Direction{-1, 1}
	downLeft := Direction{1, -1}
	downRight := Direction{1, 1}
	directions := []Direction{upRight, downRight, downLeft, upLeft} // clockwise from top-right

	return g.slideMoves(bishop, directions)
}

func (g *Game) knightMoves(knight *Piece) ([]Cell, error) {
	from, onBoard := g.Board.Find(knight)
	if !onBoard {
		return nil, ErrNotOnBoard
	}

	upLeft := Direction{-2, -1}
	upRight := Direction{-2, 1}
	downLeft := Direction{2, -1}
	downRight := Direction{2, 1}
	leftUp := Direction{-1, -2}
	leftDown := Direction{1, -2}
	rightUp := Direction{-1, 2}
	rightDown := Direction{1, 2}
	directions := []Direction{upRight, rightUp, rightDown, downRight, downLeft, leftDown, leftUp, upLeft}

	moves := []Cell{}
	for _, dir := range directions {
		cell := Cell{Row: from.Row + dir[0], Col: from.Col + dir[1]}

		allowed, _ := g.canMoveTo(knight, cell)
		if allowed {
			moves = append(moves, cell)
		}
	}

	return moves, nil
}

func (g *Game) pawnMoves(pawn *Piece) ([]Cell, error) {
	moves := []Cell{}
	direction := g.PawnDirections[pawn.Color]

	from, onBoard := g.Board.Find(pawn)
	if !onBoard {
		return nil, ErrNotOnBoard
	}

	cell := Cell{Row: from.Row + direction, Col: from.Col}
	if cell.Valid() && g.Board.At(cell) == nil {
		moves = append(moves, cell)
	}

	captureCells := []Cell{
		{Row: from.Row + direction, Col: from.Col - 1},
		{Row: from.Row + direction, Col: from.Col + 1},
	}
	for _, cell := range captureCells {
		if !cell.Valid() {
			continue
		}

		otherPiece := g.Board.At(cell)
		if otherPiece != nil && otherPiece.Color != pawn.Color {
			moves = append(moves, cell)
		}
	}

	return moves, nil
}

func (g *Game) slideMoves(piece *Piece, directions []Direction) ([]Cell, error) {
	moves := []Cell{}

	from, found := g.Board.Find(piece)
	if !found {
		return nil, ErrNotOnBoard
	}

	for _, dir := range directions {
		cell := from
		for range BoardSize - 1 {
			cell = Cell{cell.Row + dir[0], cell.Col + dir[1]}

			allowed, capture := g.canMoveTo(piece, cell)
			if allowed {
				moves = append(moves, cell)

				if capture {
					break
				}
			} else {
				break
			}
		}
	}

	return moves, nil
}

func (g *Game) canMoveTo(piece *Piece, cell Cell) (allowed, capture bool) {
	if !cell.Valid() {
		return false, false
	}

	otherPiece := g.Board.At(cell)
	if otherPiece == nil {
		return true, false
	}

	if piece.Color != otherPiece.Color {
		return true, true // can capture
	}

	return false, false
}
