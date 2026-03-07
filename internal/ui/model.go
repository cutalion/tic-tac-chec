package ui

import (
	"tic-tac-chec/engine"

	tea "github.com/charmbracelet/bubbletea"
)

type PanelCursor int // 0-3
type BoardCursor engine.Cell

var Kinds = []engine.PieceKind{engine.Pawn, engine.Rook, engine.Bishop, engine.Knight}

func (m Model) shouldFlip() bool {
	return m.Mode == ModeOnline && m.MyColor == engine.Black
}

// layout holds computed parameters for flipped/normal board orientation.
// All cursor movement and panel transitions use these instead of hardcoded directions.
type layout struct {
	topColor    engine.Color // which color's hand panel is on top
	bottomColor engine.Color // which color's hand panel is on bottom
	upDelta     int          // row delta when pressing "up" (-1 normal, +1 flipped)
	downDelta   int          // row delta when pressing "down" (+1 normal, -1 flipped)
	topRow      int          // engine row at visual top of board (0 normal, 3 flipped)
	bottomRow   int          // engine row at visual bottom of board (3 normal, 0 flipped)
}

func (m Model) layout() layout {
	if m.shouldFlip() {
		return layout{
			topColor:    engine.White,
			bottomColor: engine.Black,
			upDelta:     +1,
			downDelta:   -1,
			topRow:      engine.BoardSize - 1,
			bottomRow:   0,
		}
	}
	return layout{
		topColor:    engine.Black,
		bottomColor: engine.White,
		upDelta:     -1,
		downDelta:   +1,
		topRow:      0,
		bottomRow:   engine.BoardSize - 1,
	}
}

type Model struct {
	Game             *engine.Game
	CursorOnBoard    bool
	SelectedPiece    *engine.Piece
	BoardCursor      BoardCursor
	PanelCursor      PanelCursor
	LastErrorMessage string
	ShowStatus       bool
	ShowRules        bool
	WindowWidth      int
	SchemeIdx        int

	// Multiplayer fields
	Mode       Mode
	Phase      Phase
	MyColor    engine.Color
	Moves      chan<- MoveRequest  // send moves to Room
	Incoming   <-chan tea.Msg      // receive state updates from Room
	LobbyReady <-chan engine.Color // receives assigned color when paired
}

func InitialModel() Model {
	return Model{
		Game: engine.NewGame(),
	}
}

func (m Model) Init() tea.Cmd {
	if m.Phase == PhaseWaiting {
		lobbyReady := m.LobbyReady
		return func() tea.Msg {
			color := <-lobbyReady
			return PairedMsg{Color: color}
		}
	}
	return m.nextCmd()
}

// nextCmd returns the appropriate tea.Cmd for the current mode.
// In local mode, returns nil (no async messages to listen for).
// In online mode, returns waitForIncoming() to keep listening for Room messages.
// Must be returned from every Update path in online mode, otherwise the model
// stops receiving messages and the UI freezes.
func (m Model) nextCmd() tea.Cmd {
	if m.Mode != ModeOnline {
		return nil
	}
	return m.waitForIncoming()
}

// waitForIncoming returns a tea.Cmd that blocks until a message arrives on
// the Incoming channel, then delivers it to the Bubble Tea runtime.
//
// Bubble Tea runs each tea.Cmd in a goroutine. Every call to nextCmd() spawns
// a new goroutine blocked on <-incoming. When a message arrives, only one
// goroutine receives it — the rest remain blocked until the channel closes
// (on SSH session end). This means we accumulate ~1 stale goroutine per
// Update call in online mode. This is a Bubble Tea architectural limitation:
// there's no way to cancel a previous Cmd. The leak is bounded per session
// (~2-4KB per goroutine) and cleaned up when the session ends.
func (m Model) waitForIncoming() tea.Cmd {
	incoming := m.Incoming
	return func() tea.Msg {
		msg, ok := <-incoming
		if !ok {
			return OpponentDisconnectedMsg{}
		}
		return msg
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.LastErrorMessage = ""

	switch msg := msg.(type) {

	case PairedMsg:
		m.Phase = PhasePlaying
		m.MyColor = msg.Color
		return m, m.nextCmd()

	case GameStateMsg:
		game := msg.Game
		m.Game = &game
		m.SelectedPiece = nil

		m.CursorOnBoard = false
		m.resetCursor()
		return m, m.nextCmd()

	case ErrorMsg:
		m.LastErrorMessage = msg.Err.Error()
		return m, m.nextCmd()

	case OpponentDisconnectedMsg:
		m.LastErrorMessage = "Opponent disconnected"
		return m, nil

	case tea.WindowSizeMsg:
		m.WindowWidth = msg.Width
		return m, m.nextCmd()

	case tea.KeyMsg:
		switch msg.String() {

		case "ctrl+c", "q":
			return m, tea.Quit

		case "s":
			m.ShowStatus = !m.ShowStatus
			return m, m.nextCmd()

		case "n":
			if m.Game.Status == engine.GameOver && m.Mode != ModeOnline {
				schemeIdx := m.SchemeIdx
				m = InitialModel()
				m.SchemeIdx = schemeIdx
			}
			return m, m.nextCmd()

		case "c":
			m.SchemeIdx = (m.SchemeIdx + 1) % len(ColorSchemes)
			return m, m.nextCmd()

		case "?":
			m.ShowRules = !m.ShowRules
			return m, m.nextCmd()
		}

		// block all other input when game is over
		if m.Game.Status == engine.GameOver {
			return m, m.nextCmd()
		}

		switch msg.String() {
		case "up", "k":
			lay := m.layout()
			if m.CursorOnBoard {
				if m.BoardCursor.Row != lay.topRow {
					m.BoardCursor = BoardCursor(engine.Cell{Row: m.BoardCursor.Row + lay.upDelta, Col: m.BoardCursor.Col})
				} else if m.Game.Turn == lay.topColor {
					cursor, ok := m.pickUnusedPanelPiece()
					if ok {
						m.PanelCursor = cursor
						m.CursorOnBoard = false
					}
				}
			} else {
				if m.Game.Turn == lay.bottomColor {
					m.CursorOnBoard = true
					m.BoardCursor = BoardCursor(engine.Cell{Row: lay.bottomRow, Col: int(m.PanelCursor)})
				}
			}

		case "down", "j":
			lay := m.layout()
			if m.CursorOnBoard {
				if m.BoardCursor.Row != lay.bottomRow {
					m.BoardCursor = BoardCursor(engine.Cell{Row: m.BoardCursor.Row + lay.downDelta, Col: m.BoardCursor.Col})
				} else if m.Game.Turn == lay.bottomColor {
					m.CursorOnBoard = false
					m.PanelCursor = PanelCursor(m.BoardCursor.Col)
				}
			} else {
				if m.Game.Turn == lay.topColor {
					m.CursorOnBoard = true
					m.BoardCursor = BoardCursor(engine.Cell{Row: lay.topRow, Col: int(m.PanelCursor)})
				}
			}

		case "right", "l":
			if m.CursorOnBoard {
				if m.BoardCursor.Col < engine.BoardSize-1 {
					m.BoardCursor = BoardCursor(engine.Cell{Row: m.BoardCursor.Row, Col: m.BoardCursor.Col + 1})
				}
			} else {
				if m.PanelCursor < engine.BoardSize-1 {
					m.PanelCursor++
				}
			}

		case "left", "h":
			if m.CursorOnBoard {
				if m.BoardCursor.Col > 0 {
					m.BoardCursor = BoardCursor(engine.Cell{Row: m.BoardCursor.Row, Col: m.BoardCursor.Col - 1})
				}
			} else {
				if m.PanelCursor > 0 {
					m.PanelCursor--
				}
			}

		case "enter", " ":
			// In online mode, only allow input on your turn
			if m.Mode == ModeOnline && m.Game.Turn != m.MyColor {
				return m, m.nextCmd()
			}

			if m.CursorOnBoard {
				if m.SelectedPiece == nil {
					piece := m.Game.Board.At(engine.Cell(m.BoardCursor))
					if piece != nil && piece.Color == m.Game.Turn {
						m.SelectedPiece = piece
					}
				} else {
					piece := m.Game.Board.At(engine.Cell(m.BoardCursor))
					if piece != nil && piece.Color == m.Game.Turn {
						m.SelectedPiece = piece
					} else {
						return m.executeMove(*m.SelectedPiece, engine.Cell(m.BoardCursor))
					}
				}
			} else {
				piece := m.Game.Pieces.Get(m.Game.Turn, Kinds[m.PanelCursor])
				if piece != nil && piece.Color == m.Game.Turn {
					m.SelectedPiece = piece
				}
			}
		}
	}

	return m, m.nextCmd()
}

func (m Model) executeMove(piece engine.Piece, cell engine.Cell) (tea.Model, tea.Cmd) {
	if m.Mode == ModeOnline {
		m.SelectedPiece = nil
		incoming := m.Incoming

		// This cmd sends the move AND waits for the Room's response in one
		// closure. This avoids relying on a stale waitForIncoming goroutine
		// to pick up the response (which would work by accident but wastes
		// a goroutine slot). After the Room processes the move, it sends
		// either GameStateMsg or ErrorMsg back on incoming.
		return m, func() tea.Msg {
			m.Moves <- MoveRequest{Piece: piece, Cell: cell}
			msg, ok := <-incoming
			if !ok {
				return OpponentDisconnectedMsg{}
			}
			return msg
		}
	}

	err := m.Game.Move(piece, cell)
	if err != nil {
		m.LastErrorMessage = err.Error()
		return m, nil
	}
	m.SelectedPiece = nil
	m.CursorOnBoard = false
	m.resetCursor()
	return m, nil
}

func (m *Model) resetCursor() {
	for i, kind := range Kinds {
		piece := m.Game.Pieces.Get(m.Game.Turn, kind)
		_, onBoard := m.Game.Board.Find(piece)

		if !onBoard {
			m.PanelCursor = PanelCursor(i)
			m.CursorOnBoard = false
			return
		}
	}

	m.CursorOnBoard = true
}

func (m *Model) pickUnusedPanelPiece() (PanelCursor, bool) {
	sameCol := Kinds[m.BoardCursor.Col]
	piece := m.Game.Pieces.Get(m.Game.Turn, sameCol)
	_, onBoard := m.Game.Board.Find(piece)

	if !onBoard {
		return PanelCursor(m.BoardCursor.Col), true
	}

	for i, kind := range Kinds {
		piece := m.Game.Pieces.Get(m.Game.Turn, kind)
		_, onBoard := m.Game.Board.Find(piece)

		if !onBoard {
			return PanelCursor(i), true
		}
	}

	return 0, false
}
