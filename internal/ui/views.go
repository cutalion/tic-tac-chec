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

func turnIndicator(m Model, scheme ColorScheme) string {
	style := lipgloss.NewStyle()

	if m.Game.Status == engine.GameOver {
		style = style.Foreground(toLipglossColor(scheme, *m.Game.Winner))
		return style.Render(colorName(*m.Game.Winner) + " wins!")
	}

	turnColor := colorName(m.Game.Turn)
	style = style.Foreground(toLipglossColor(scheme, m.Game.Turn))
	if m.Mode == ModeOnline {
		if m.Game.Turn == m.MyColor {
			return style.Render("Your turn")
		} else {
			return "Opponent's turn"
		}
	} else {
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

	statusLine := ""
	if m.ShowStatus {
		statusLine = fmt.Sprintf("cursor: board=%v pos=%v panel=%v selected=%v", m.CursorOnBoard, engine.Cell(m.BoardCursor), m.PanelCursor, m.SelectedPiece)
	}

	scheme := ColorSchemes[m.SchemeIdx]
	lay := m.layout()

	mainBoard := boardView(m, scheme)

	turnLine := lipgloss.NewStyle().Width(bw).Align(lipgloss.Center).Render(turnIndicator(m, scheme))

	topActive := m.Game.Turn == lay.topColor
	bottomActive := m.Game.Turn == lay.bottomColor

	return lipgloss.JoinVertical(lipgloss.Left,
		"",
		title,
		turnLine,
		handPanel(m.Game, lay.topColor, !m.CursorOnBoard && topActive, int(m.PanelCursor), m.SelectedPiece, scheme),
		mainBoard,
		handPanel(m.Game, lay.bottomColor, !m.CursorOnBoard && bottomActive, int(m.PanelCursor), m.SelectedPiece, scheme),
		"",
		fixedLine.Render(m.LastErrorMessage),
		fixedLine.Render(statusLine),
		fmt.Sprintf("q - quit, c - color [%s], ? - rules", scheme.Name),
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

func boardView(m Model, scheme ColorScheme) string {
	flipped := m.shouldFlip()
	parts := []string{letterMarkers()}

	for i := range engine.BoardSize {
		engineRow := i
		if flipped {
			engineRow = engine.BoardSize - 1 - i
		}
		cells := rowCells(m, engineRow, scheme)
		num := fmt.Sprintf("%d", engine.BoardSize-engineRow)
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
