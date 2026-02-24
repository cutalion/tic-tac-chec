package engine

type direction [2]int

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
	left := direction{0, -1}
	right := direction{0, 1}
	up := direction{-1, 0}
	down := direction{1, 0}
	directions := []direction{right, down, left, up}

	return g.slideMoves(rook, directions)
}

func (g *Game) bishopMoves(bishop *Piece) ([]Cell, error) {
	upLeft := direction{-1, -1}
	upRight := direction{-1, 1}
	downLeft := direction{1, -1}
	downRight := direction{1, 1}
	directions := []direction{upRight, downRight, downLeft, upLeft} // clockwise from top-right

	return g.slideMoves(bishop, directions)
}

func (g *Game) knightMoves(knight *Piece) ([]Cell, error) {
	from, onBoard := g.Board.Find(knight)
	if !onBoard {
		return nil, ErrNotOnBoard
	}

	upLeft := direction{-2, -1}
	upRight := direction{-2, 1}
	downLeft := direction{2, -1}
	downRight := direction{2, 1}
	leftUp := direction{-1, -2}
	leftDown := direction{1, -2}
	rightUp := direction{-1, 2}
	rightDown := direction{1, 2}
	directions := []direction{upRight, rightUp, rightDown, downRight, downLeft, leftDown, leftUp, upLeft}

	var moves []Cell
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
	var moves []Cell
	direction := int(g.PawnDirections[pawn.Color])

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

func (g *Game) slideMoves(piece *Piece, directions []direction) ([]Cell, error) {
	var moves []Cell

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
