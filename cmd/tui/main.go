package main

import (
	"fmt"
	"os"

	"tic-tac-chec/engine"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ColorScheme struct {
	Name string
	P1   lipgloss.AdaptiveColor // "White" player
	P2   lipgloss.AdaptiveColor // "Black" player
}

var colorSchemes = []ColorScheme{
	{"Red / Blue",      lipgloss.AdaptiveColor{Light: "1", Dark: "9"},       lipgloss.AdaptiveColor{Light: "4", Dark: "12"}},
	{"Gold / Blue",     lipgloss.AdaptiveColor{Light: "3", Dark: "11"},      lipgloss.AdaptiveColor{Light: "4", Dark: "12"}},
	{"Cyan / Magenta",  lipgloss.AdaptiveColor{Light: "6", Dark: "14"},      lipgloss.AdaptiveColor{Light: "5", Dark: "13"}},
	{"Orange / Purple", lipgloss.AdaptiveColor{Light: "#994400", Dark: "#FF6600"}, lipgloss.AdaptiveColor{Light: "5", Dark: "13"}},
	{"Green / Purple",  lipgloss.AdaptiveColor{Light: "2", Dark: "10"},      lipgloss.AdaptiveColor{Light: "5", Dark: "13"}},
}

type model struct {
	game             *engine.Game
	cursorOnBoard    bool
	selectedPiece    *engine.Piece
	boardCursor      boardCursor
	panelCursor      panelCursor
	lastErrorMessage string
	showStatus       bool
	windowWidth      int
	schemeIdx        int
}

type panelCursor int // 0-3
type boardCursor engine.Cell

var kinds = []engine.PieceKind{engine.Pawn, engine.Rook, engine.Bishop, engine.Knight}

func initialModel() model {
	return model{
		game: engine.NewGame(),
	}
}

func (m model) Init() tea.Cmd {
	// Just return `nil`, which means "no I/O right now, please."
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.lastErrorMessage = ""

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.windowWidth = msg.Width
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {

		// These keys should exit the program.
		case "ctrl+c", "q":
			return m, tea.Quit

		case "s":
			m.showStatus = !m.showStatus
			return m, nil

		case "n":
			if m.game.Status == engine.GameOver {
				schemeIdx := m.schemeIdx
				m = initialModel()
				m.schemeIdx = schemeIdx
			}
			return m, nil

		case "c":
			m.schemeIdx = (m.schemeIdx + 1) % len(colorSchemes)
			return m, nil
		}

		// block all other input when game is over
		if m.game.Status == engine.GameOver {
			return m, nil
		}

		switch msg.String() {
		case "up", "k":
			if m.cursorOnBoard {
				if m.boardCursor.Row > 0 {
					m.boardCursor = boardCursor(engine.Cell{Row: m.boardCursor.Row - 1, Col: m.boardCursor.Col})
				} else if m.game.Turn == engine.Black { // black hand panel on top
					cursor, ok := m.pickUnusedPanelPiece()
					if ok {
						m.panelCursor = cursor
						m.cursorOnBoard = false
					}
				}
			} else {
				if m.game.Turn == engine.White { // white hand panel at the bottom
					m.cursorOnBoard = true
					m.boardCursor = boardCursor(engine.Cell{Row: engine.BoardSize - 1, Col: int(m.panelCursor)})
				}
			}

		// The "down" and "j" keys move the cursor down
		case "down", "j":
			if m.cursorOnBoard {
				if m.boardCursor.Row < engine.BoardSize-1 {
					m.boardCursor = boardCursor(engine.Cell{Row: m.boardCursor.Row + 1, Col: m.boardCursor.Col})
				} else if m.game.Turn == engine.White { // white hand panel at the bottom
					m.cursorOnBoard = false
					m.panelCursor = panelCursor(m.boardCursor.Col)
				}
			} else {
				if m.game.Turn == engine.Black { // black hand panel on top
					m.cursorOnBoard = true
					m.boardCursor = boardCursor(engine.Cell{Row: 0, Col: int(m.panelCursor)})
				}
			}

		case "right", "h":
			if m.cursorOnBoard {
				if m.boardCursor.Col < engine.BoardSize-1 {
					m.boardCursor = boardCursor(engine.Cell{Row: m.boardCursor.Row, Col: m.boardCursor.Col + 1})
				}
			} else {
				if m.panelCursor < engine.BoardSize-1 {
					m.panelCursor += 1
				}
			}

		case "left", "l":
			{
				if m.cursorOnBoard {
					if m.boardCursor.Col > 0 {
						m.boardCursor = boardCursor(engine.Cell{Row: m.boardCursor.Row, Col: m.boardCursor.Col - 1})
					}
				} else {
					if m.panelCursor > 0 {
						m.panelCursor -= 1
					}
				}
			}

		// The "enter" key and the spacebar (a literal space) toggle
		// the selected state for the item that the cursor is pointing at.
		case "enter", " ":
			if m.cursorOnBoard {
				if m.selectedPiece == nil {
					piece := m.game.Board.At(engine.Cell(m.boardCursor))
					if piece != nil && piece.Color == m.game.Turn {
						m.selectedPiece = piece
					}
				} else {
					piece := m.game.Board.At(engine.Cell(m.boardCursor))
					if piece != nil && piece.Color == m.game.Turn {
						m.selectedPiece = piece
					} else {
						err := m.game.Move(*m.selectedPiece, engine.Cell(m.boardCursor))
						if err != nil {
							m.lastErrorMessage = err.Error()
						} else {
							m.selectedPiece = nil
							m.cursorOnBoard = false
							m.resetCursor()
						}
					}
				}
			} else {
				piece := m.game.Pieces.Get(m.game.Turn, kinds[m.panelCursor])
				if piece != nil && piece.Color == m.game.Turn {
					m.selectedPiece = piece
				}
			}
		}
	}

	// Return the updated model to the Bubble Tea runtime for processing.
	// Note that we're not returning a command.
	return m, nil
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

// letterMarkers returns the "   a  b  c  d" row aligned to board columns.
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

func (m model) View() string {
	blackActive := m.game.Turn == engine.Black
	whiteActive := m.game.Turn == engine.White

	bw := rowLabelWidth + lipgloss.Width(baseCellStyle.Render(" "))*engine.BoardSize + 2
	title := lipgloss.NewStyle().Width(bw).Align(lipgloss.Center).Render("Tic Tac Chec")

	statusLine := ""
	if m.showStatus {
		statusLine = fmt.Sprintf("cursor: board=%v pos=%v panel=%v selected=%v", m.cursorOnBoard, engine.Cell(m.boardCursor), m.panelCursor, m.selectedPiece)
	}

	scheme := colorSchemes[m.schemeIdx]

	bh := 2 + engine.BoardSize*lipgloss.Height(baseCellStyle.Render(" "))
	var mainBoard string
	if m.game.Status == engine.GameOver {
		mainBoard = gameOverView(*m.game.Winner, bw, bh)
	} else {
		mainBoard = boardView(m.game.Board, engine.Cell(m.boardCursor), m.cursorOnBoard, m.selectedPiece, scheme)
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		title,
		"",
		handPanel(m.game, engine.Black, !m.cursorOnBoard && blackActive, int(m.panelCursor), m.selectedPiece, scheme),
		mainBoard,
		handPanel(m.game, engine.White, !m.cursorOnBoard && whiteActive, int(m.panelCursor), m.selectedPiece, scheme),
		"",
		fixedLine.Render(m.lastErrorMessage),
		fixedLine.Render(statusLine),
		fmt.Sprintf("q quit  s status  n new game  c color [%s]", scheme.Name),
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

func boardView(board engine.Board, cursor engine.Cell, active bool, selected *engine.Piece, scheme ColorScheme) string {
	var selectedCell engine.Cell
	selectedOnBoard := false
	if selected != nil {
		selectedCell, selectedOnBoard = board.Find(selected)
	}

	parts := []string{letterMarkers()}

	for i := range engine.BoardSize {
		cells := make([]string, engine.BoardSize)
		for j := range engine.BoardSize {
			style := baseCellStyle
			switch {
			case active && cursor.Row == i && cursor.Col == j:
				style = style.BorderForeground(borderHovered)
			case selectedOnBoard && selectedCell.Row == i && selectedCell.Col == j:
				style = style.BorderForeground(borderSelected)
			}
			cells[j] = style.Render(pieceView(board[i][j], scheme))
		}
		num := fmt.Sprintf("%d", engine.BoardSize-i) // row 0 → 4, row 3 → 1
		cellsRow := lipgloss.JoinHorizontal(lipgloss.Top, cells...)
		parts = append(parts, lipgloss.JoinHorizontal(lipgloss.Top, leftLabel(num), cellsRow, rightLabel(num)))
	}

	parts = append(parts, letterMarkers())
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
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
	cells := make([]string, len(kinds))
	for i, kind := range kinds {
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
		cells[i] = style.Render(pieceView(piece, scheme))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, leftLabel(color.String()), lipgloss.JoinHorizontal(lipgloss.Top, cells...))
}

func (m *model) resetCursor() {
	for i, kind := range kinds {
		piece := m.game.Pieces.Get(m.game.Turn, kind)
		_, onBoard := m.game.Board.Find(piece)

		if !onBoard {
			m.panelCursor = panelCursor(i)
			m.cursorOnBoard = false
			return
		}
	}

	m.cursorOnBoard = true
}

func (m *model) pickUnusedPanelPiece() (panelCursor, bool) {
	sameCol := kinds[m.boardCursor.Col]
	piece := m.game.Pieces.Get(m.game.Turn, sameCol)
	_, onBoard := m.game.Board.Find(piece)

	if !onBoard {
		return panelCursor(sameCol), true
	}

	for i, kind := range kinds {
		piece := m.game.Pieces.Get(m.game.Turn, kind)
		_, onBoard := m.game.Board.Find(piece)

		if !onBoard {
			return panelCursor(i), true
		}
	}

	return 0, false
}

func main() {
	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
