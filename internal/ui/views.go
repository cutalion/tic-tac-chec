package ui

import (
	"fmt"

	"tic-tac-chec/engine"

	"github.com/charmbracelet/lipgloss"
)

type ColorScheme struct {
	Name string
	P1   lipgloss.AdaptiveColor // "White" player
	P2   lipgloss.AdaptiveColor // "Black" player
}

var ColorSchemes = []ColorScheme{
	{"Red / Blue", lipgloss.AdaptiveColor{Light: "1", Dark: "9"}, lipgloss.AdaptiveColor{Light: "4", Dark: "12"}},
	{"Gold / Blue", lipgloss.AdaptiveColor{Light: "3", Dark: "11"}, lipgloss.AdaptiveColor{Light: "4", Dark: "12"}},
	{"Cyan / Magenta", lipgloss.AdaptiveColor{Light: "6", Dark: "14"}, lipgloss.AdaptiveColor{Light: "5", Dark: "13"}},
	{"Orange / Purple", lipgloss.AdaptiveColor{Light: "#994400", Dark: "#FF6600"}, lipgloss.AdaptiveColor{Light: "5", Dark: "13"}},
	{"Green / Purple", lipgloss.AdaptiveColor{Light: "2", Dark: "10"}, lipgloss.AdaptiveColor{Light: "5", Dark: "13"}},
}

var (
	borderDimmed   = lipgloss.AdaptiveColor{Light: "250", Dark: "238"}
	borderHovered  = lipgloss.AdaptiveColor{Light: "0", Dark: "15"}
	borderSelected = lipgloss.AdaptiveColor{Light: "3", Dark: "11"}
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

func (m Model) View() string {
	blackActive := m.Game.Turn == engine.Black
	whiteActive := m.Game.Turn == engine.White

	bw := rowLabelWidth + lipgloss.Width(baseCellStyle.Render(" "))*engine.BoardSize + 2
	title := lipgloss.NewStyle().Width(bw).Align(lipgloss.Center).Render("Tic Tac Chec")

	statusLine := ""
	if m.ShowStatus {
		statusLine = fmt.Sprintf("cursor: board=%v pos=%v panel=%v selected=%v", m.CursorOnBoard, engine.Cell(m.BoardCursor), m.PanelCursor, m.SelectedPiece)
	}

	scheme := ColorSchemes[m.SchemeIdx]

	bh := engine.BoardSize * lipgloss.Height(baseCellStyle.Render(" "))
	var mainBoard string
	var gameOver string

	if m.Game.Status == engine.GameOver {
		gameOver = gameOverView(*m.Game.Winner, bw, bh)
	}

	mainBoard = boardView(m, scheme)

	return lipgloss.JoinVertical(lipgloss.Left,
		"",
		title,
		"",
		handPanel(m.Game, engine.Black, !m.CursorOnBoard && blackActive, int(m.PanelCursor), m.SelectedPiece, scheme),
		mainBoard,
		handPanel(m.Game, engine.White, !m.CursorOnBoard && whiteActive, int(m.PanelCursor), m.SelectedPiece, scheme),
		"",
		gameOver,
		"",
		fixedLine.Render(m.LastErrorMessage),
		fixedLine.Render(statusLine),
		fmt.Sprintf("q - quit, n - new game, c - color [%s]", scheme.Name),
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

func gameOverView(winner engine.Color, bw, bh int) string {
	popup := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderSelected).
		Padding(1, 3).
		Align(lipgloss.Center).
		Render(fmt.Sprintf("%v wins!\n\nPress N for new game", colorName(winner)))
	return lipgloss.Place(bw, bh, lipgloss.Center, lipgloss.Center, popup)
}

// mainBoard = boardView(m.Game.Board, engine.Cell(m.BoardCursor), m.CursorOnBoard, m.SelectedPiece, scheme)
// func boardView(board engine.Board, cursor engine.Cell, active bool, selected *engine.Piece, scheme ColorScheme) string {
func boardView(m Model, scheme ColorScheme) string {
	parts := []string{letterMarkers()}

	for i := range engine.BoardSize {
		cells := rowCells(m, i, scheme)
		num := fmt.Sprintf("%d", engine.BoardSize-i)
		cellsRow := lipgloss.JoinHorizontal(lipgloss.Top, cells...)
		parts = append(parts, lipgloss.JoinHorizontal(lipgloss.Top, leftLabel(num), cellsRow, rightLabel(num)))
	}

	parts = append(parts, letterMarkers())
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func rowCells(m Model, i int, scheme ColorScheme) []string {
	var selectedCell engine.Cell
	selectedOnBoard := false
	if m.SelectedPiece != nil {
		selectedCell, selectedOnBoard = m.Game.Board.Find(m.SelectedPiece)
	}

	cells := make([]string, engine.BoardSize)
	for j := range engine.BoardSize {
		style := baseCellStyle
		switch {
		case m.CursorOnBoard && m.BoardCursor.Row == i && m.BoardCursor.Col == j:
			style = style.BorderForeground(borderHovered)
		case selectedOnBoard && selectedCell.Row == i && selectedCell.Col == j:
			style = style.BorderForeground(borderSelected)
		}
		cells[j] = style.Render(pieceView(m.Game.Board[i][j], scheme))
	}

	return cells
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

func handPanel(game *engine.Game, color engine.Color, active bool, cursor int, selected *engine.Piece, scheme ColorScheme) string {
	cells := make([]string, len(Kinds))
	for i, kind := range Kinds {
		piece := game.Pieces.Get(color, kind)
		_, onBoard := game.Board.Find(piece)
		style := baseCellStyle
		switch {
		case onBoard:
			style = style.Faint(true)
		case piece == selected:
			style = style.BorderForeground(borderSelected)
		case active && i == cursor:
			style = style.BorderForeground(borderHovered)
		}

		if onBoard {
			cells[i] = style.Render(pieceView(nil, scheme))
		} else {
			cells[i] = style.Render(pieceView(piece, scheme))
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, leftLabel(color.String()), lipgloss.JoinHorizontal(lipgloss.Top, cells...))
}
