package ui

import (
	"fmt"

	"tic-tac-chec/engine"

	"github.com/charmbracelet/lipgloss"
)

type ColorScheme struct {
	Name string
	P1   lipgloss.Color // "White" player
	P2   lipgloss.Color // "Black" player
}

func toLipglossColor(scheme ColorScheme, color engine.Color) lipgloss.Color {
	if color == engine.White {
		return scheme.P1
	}

	return scheme.P2
}

// Using dark-mode colors since SSH clients typically have dark backgrounds
var ColorSchemes = []ColorScheme{
	{"Red / Blue", lipgloss.Color("9"), lipgloss.Color("12")},
	{"Gold / Blue", lipgloss.Color("11"), lipgloss.Color("12")},
	{"Cyan / Magenta", lipgloss.Color("14"), lipgloss.Color("13")},
	{"Orange / Purple", lipgloss.Color("#FF6600"), lipgloss.Color("13")},
	{"Green / Purple", lipgloss.Color("10"), lipgloss.Color("13")},
}

var (
	borderDimmed   = lipgloss.Color("238")
	borderHovered  = lipgloss.Color("15")
	borderSelected = lipgloss.Color("11")
)

var baseCellStyle = lipgloss.NewStyle().
	Padding(0, 1).
	Border(lipgloss.RoundedBorder()).
	BorderForeground(borderDimmed).
	MarginRight(1)

var fixedLine = lipgloss.NewStyle().Height(1)

const rowLabelWidth = 3

func letterMarkers() string {
	cellW := lipgloss.Width(baseCellStyle.Render(" "))
	letterStyle := lipgloss.NewStyle().Width(cellW).Align(lipgloss.Center)
	labels := make([]string, engine.BoardSize)
	for i, l := range []string{"a", "b", "c", "d"} {
		labels[i] = letterStyle.Render(l)
	}
	return fmt.Sprintf("%*s", rowLabelWidth, "") + lipgloss.JoinHorizontal(lipgloss.Top, labels...)
}

func leftLabel(n string) string {
	cellH := lipgloss.Height(baseCellStyle.Render(" "))
	return lipgloss.NewStyle().
		Height(cellH).
		AlignVertical(lipgloss.Center).
		Render(fmt.Sprintf("%*s ", rowLabelWidth-1, n))
}

func rightLabel(n string) string {
	cellH := lipgloss.Height(baseCellStyle.Render(" "))
	return lipgloss.NewStyle().
		Width(2).
		Height(cellH).
		AlignVertical(lipgloss.Center).
		Render(n)
}

func rulesView() string {
	return `
  Tic Tac Chec — Rules

  4×4 board, 2 players (White / Black)
  Each player has: Pawn, Rook, Bishop, Knight

  On your turn, either:
    • Place a piece from your hand onto any empty cell
    • Move a piece already on the board (chess rules)

  Captures return the piece to its owner's hand (shogi-style)
  Pawns reverse direction at the far edge (no promotion)

  Win: get 4 of your color in a row
       (horizontal, vertical, or diagonal)

  Controls:
    arrows/hjkl  Move cursor
    enter/space  Select piece / confirm move
    c            Change color scheme
    n            New game (local only)
    ?            Toggle this screen
    q            Quit

  Press ? to return
`
}

func turnIndicator(m Model) string {
	style := lipgloss.NewStyle()
	scheme := m.colorScheme()

	if m.gameOver() {
		if m.draw() {
			return style.Render("Draw!")
		}

		style = style.Foreground(toLipglossColor(scheme, *m.winner()))
		return style.Render(colorName(*m.winner()) + " wins!")
	}

	style = style.Foreground(toLipglossColor(scheme, m.Game.Turn))
	if m.online() {
		if m.myTurn() {
			return style.Render("Your turn")
		} else {
			return "Opponent's turn"
		}
	} else {
		turnColor := colorName(m.Game.Turn)
		return style.Render(turnColor + "'s turn")
	}
}

func (m Model) View() string {
	if m.ShowRules {
		return rulesView()
	}

	if m.Phase == PhaseWaiting {
		return "\n  Waiting for opponent...\n\n  Press ? for rules, q to quit\n"
	}

	bw := rowLabelWidth + lipgloss.Width(baseCellStyle.Render(" "))*engine.BoardSize + 2
	title := lipgloss.NewStyle().Width(bw).Align(lipgloss.Center).Render("Tic Tac Chec")

	turnLine := lipgloss.NewStyle().Width(bw).Align(lipgloss.Center).Render(turnIndicator(m))
	layout := m.layout()

	return lipgloss.JoinVertical(lipgloss.Left,
		"",
		title,
		turnLine,
		handPanel(m, layout.topColor),
		boardView(m),
		handPanel(m, layout.bottomColor),
		"",
		fixedLine.Render(m.LastErrorMessage),
		fixedLine.Render(statusLine(m)),
		fmt.Sprintf("q - quit, c - color [%s], ? - rules", m.colorScheme().Name),
	)
}

func colorName(c engine.Color) string {
	switch c {
	case engine.White:
		return "White"
	case engine.Black:
		return "Black"
	}
	return c.String()
}

func boardView(m Model) string {
	flipped := m.shouldFlip()
	parts := []string{letterMarkers()}

	for i := range engine.BoardSize {
		engineRow := i
		if flipped {
			engineRow = engine.BoardSize - 1 - i
		}
		cells := rowCells(m, engineRow)
		num := fmt.Sprintf("%d", engine.BoardSize-engineRow)
		cellsRow := lipgloss.JoinHorizontal(lipgloss.Top, cells...)
		parts = append(parts, lipgloss.JoinHorizontal(lipgloss.Top, leftLabel(num), cellsRow, rightLabel(num)))
	}

	parts = append(parts, letterMarkers())
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func rowCells(m Model, row int) []string {
	cells := make([]string, engine.BoardSize)

	for col := range engine.BoardSize {
		style := baseCellStyle.BorderForeground(cellBorderColor(m, row, col))
		cells[col] = style.Render(pieceView(m.Game.Board[row][col], m.colorScheme()))
	}

	return cells
}

func handPanel(m Model, color engine.Color) string {
	game := m.Game
	handCells := make([]string, len(Kinds))

	for col, kind := range Kinds {
		style := baseCellStyle.BorderForeground(handPieceBorderColor(m, color, col))
		piece := game.Piece(engine.Piece{Color: color, Kind: kind})
		onBoard := game.PieceOnBoard(*piece)

		pieceStr := " "
		if !onBoard {
			pieceStr = pieceView(piece, m.colorScheme())
		}

		handCells[col] = style.Render(pieceStr)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, leftLabel(color.String()), lipgloss.JoinHorizontal(lipgloss.Top, handCells...))
}

func pieceView(piece *engine.Piece, scheme ColorScheme) string {
	if piece == nil {
		return " "
	}

	var symbol string
	switch piece.Kind {
	case engine.Pawn:
		symbol = "♟"
	case engine.Rook:
		symbol = "♜"
	case engine.Bishop:
		symbol = "♝"
	case engine.Knight:
		symbol = "♞"
	default:
		symbol = "?"
	}

	color := scheme.P2
	if piece.Color == engine.White {
		color = scheme.P1
	}
	return lipgloss.NewStyle().Bold(true).Foreground(color).Render(symbol)
}

func showCursor(m Model) bool {
	if m.gameOver() {
		return false
	}

	if m.localGame() { // always show cursor in local game
		return true
	}

	return m.myTurn() // in online game, show cursor only when it's my turn
}

func cellBorderColor(m Model, row, col int) lipgloss.Color {
	if !showCursor(m) {
		return borderDimmed
	}

	if m.SelectedPiece != nil {
		selectedCell, selectedOnBoard := m.Game.Board.Find(m.SelectedPiece)

		if selectedOnBoard && selectedCell.Row == row && selectedCell.Col == col {
			return borderSelected
		}
	}

	if m.cursorOnBoard() && m.Cursor.BoardCursor.Row == row && m.Cursor.BoardCursor.Col == col {
		return borderHovered
	}

	return borderDimmed
}

func handPieceBorderColor(m Model, handColor engine.Color, pos int) lipgloss.Color {
	if !showCursor(m) {
		return borderDimmed
	}

	piece := m.Game.Pieces.Get(handColor, Kinds[pos])
	inHand := m.Game.PieceInHand(*piece)

	activeHand := m.Game.Turn == handColor
	selected := m.SelectedPiece
	cursor := m.Cursor.PanelIndex

	if !activeHand {
		return borderDimmed
	}

	if inHand && selected != nil && *piece == *selected {
		return borderSelected
	}

	if cursor != nil && pos == *cursor {
		return borderHovered
	}

	return borderDimmed
}

func statusLine(m Model) string {
	if !m.ShowStatus {
		return ""
	}

	boardCursor := "nil"
	if m.cursorOnBoard() {
		boardCursor = fmt.Sprintf("(%d, %d)", m.Cursor.BoardCursor.Row, m.Cursor.BoardCursor.Col)
	}

	panelPos := "nil"
	if m.Cursor.PanelIndex != nil {
		panelPos = fmt.Sprintf("%d", *m.Cursor.PanelIndex)
	}

	return fmt.Sprintf("cursor: board=%v pos=%v panel=%v selected=%v", m.cursorOnBoard(), boardCursor, panelPos, m.SelectedPiece)
}
