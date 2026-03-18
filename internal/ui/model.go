package ui

import (
	"tic-tac-chec/engine"

	tea "github.com/charmbracelet/bubbletea"
)

// Mode determines whether the game is local or online.
type Mode int

const (
	ModeLocal Mode = iota
	ModeOnline
)

// Phase tracks whether the player is waiting in the lobby or playing.
type Phase int

const (
	PhasePlaying Phase = iota
	PhaseWaiting
)

var Kinds = []engine.PieceKind{engine.Pawn, engine.Rook, engine.Bishop, engine.Knight}

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

type Model struct {
	Game             *engine.Game
	SelectedPiece    *engine.Piece
	Cursor           *Cursor
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
	Updates   <-chan any          // receive state updates from Room
	LobbyReady <-chan engine.Color // receives assigned color when paired
}

func InitialModel() Model {
	return Model{
		Game:   engine.NewGame(),
		Cursor: NewCursor(),
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
// In online mode, returns waitForUpdates() to keep listening for Room messages.
// Must be returned from every Update path in online mode, otherwise the model
// stops receiving messages and the UI freezes.
func (m Model) nextCmd() tea.Cmd {
	if !m.online() {
		return nil
	}
	return m.waitForUpdates()
}

// waitForUpdates returns a tea.Cmd that blocks until a message arrives on
// the Updates channel, then delivers it to the Bubble Tea runtime.
//
// Bubble Tea runs each tea.Cmd in a goroutine. Every call to nextCmd() spawns
// a new goroutine blocked on <-updates. When a message arrives, only one
// goroutine receives it — the rest remain blocked until the channel closes
// (on SSH session end). This means we accumulate ~1 stale goroutine per
// Update call in online mode. This is a Bubble Tea architectural limitation:
// there's no way to cancel a previous Cmd. The leak is bounded per session
// (~2-4KB per goroutine) and cleaned up when the session ends.
func (m Model) waitForUpdates() tea.Cmd {
	updates := m.Updates
	return func() tea.Msg {
		msg, ok := <-updates
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
			if m.gameOver() && !m.online() {
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
		if m.gameOver() {
			return m, m.nextCmd()
		}

		// block all input when not your turn
		if m.online() && !m.myTurn() {
			return m, m.nextCmd()
		}

		switch msg.String() {
		case "up", "k":
			lay := m.layout()
			if m.cursorOnBoard() {
				if m.Cursor.BoardCursor.Row != lay.topRow {
					m.Cursor.moveVertically(lay.upDelta)
				} else if m.Game.Turn == lay.topColor {
					cursor, ok := m.pickUnusedPanelPiece()
					if ok {
						m.Cursor.enterPanel(cursor)
					}
				}
			} else {
				if m.Game.Turn == lay.bottomColor {
					m.Cursor.enterBoard(lay.bottomRow, *m.Cursor.PanelIndex)
				}
			}

		case "down", "j":
			lay := m.layout()
			if m.cursorOnBoard() {
				if m.Cursor.BoardCursor.Row != lay.bottomRow {
					m.Cursor.moveVertically(lay.downDelta)
				} else if m.Game.Turn == lay.bottomColor {
					cursor, ok := m.pickUnusedPanelPiece()
					if ok {
						m.Cursor.enterPanel(cursor)
					}
				}
			} else {
				if m.Game.Turn == lay.topColor {
					m.Cursor.enterBoard(lay.topRow, *m.Cursor.PanelIndex)
				}
			}

		case "right", "l":
			if m.Cursor.col() < engine.BoardSize-1 {
				m.Cursor.moveHorizontally(+1)
			}

		case "left", "h":
			if m.Cursor.col() > 0 {
				m.Cursor.moveHorizontally(-1)
			}

		case "enter", " ":
			if m.cursorOnBoard() {
				if m.SelectedPiece == nil {
					piece := m.Game.Board.At(*m.Cursor.BoardCursor)
					if piece != nil && piece.Color == m.Game.Turn {
						m.SelectedPiece = piece
					}
				} else {
					piece := m.Game.Board.At(*m.Cursor.BoardCursor)
					if piece != nil && piece.Color == m.Game.Turn {
						m.SelectedPiece = piece
					} else {
						return m.executeMove(*m.SelectedPiece, *m.Cursor.BoardCursor)
					}
				}
			} else { // cursor on hand panel
				piece := m.Game.Pieces.Get(m.Game.Turn, Kinds[*m.Cursor.PanelIndex])

				if piece != nil && piece.Color == m.Game.Turn && !m.Game.PieceOnBoard(*piece) {
					m.SelectedPiece = piece
				}
			}
		}
	}

	return m, m.nextCmd()
}

func (m Model) executeMove(piece engine.Piece, cell engine.Cell) (tea.Model, tea.Cmd) {
	if m.online() {
		m.SelectedPiece = nil
		updates := m.Updates

		// This cmd sends the move AND waits for the Room's response in one
		// closure. This avoids relying on a stale waitForUpdates goroutine
		// to pick up the response (which would work by accident but wastes
		// a goroutine slot). After the Room processes the move, it sends
		// either GameStateMsg or ErrorMsg back on updates.
		return m, func() tea.Msg {
			m.Moves <- MoveRequest{Piece: piece, Cell: cell}
			msg, ok := <-updates
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
	m.resetCursor()
	return m, nil
}

func (m *Model) resetCursor() {
	for i, kind := range Kinds {
		piece := m.Game.Pieces.Get(m.Game.Turn, kind)
		_, onBoard := m.Game.Board.Find(piece)

		if !onBoard {
			m.Cursor.enterPanel(i)
			return
		}
	}

	// All pieces on board — place cursor on the first own piece,
	// scanning from the player's bottom row upward.
	lay := m.layout()
	row := lay.bottomRow
	for range engine.BoardSize {
		for col := range engine.BoardSize {
			p := m.Game.Board.At(engine.Cell{Row: row, Col: col})
			if p != nil && p.Color == m.Game.Turn {
				m.Cursor.enterBoard(row, col)
				return
			}
		}
		row += lay.upDelta
	}
}

func (m *Model) pickUnusedPanelPiece() (int, bool) {
	if m.cursorOnBoard() {
		kind := Kinds[m.Cursor.col()]
		piece := m.Game.Pieces.Get(m.Game.Turn, kind)

		if m.Game.PieceInHand(*piece) {
			return m.Cursor.col(), true
		}
	}

	for col, kind := range Kinds {
		piece := m.Game.Pieces.Get(m.Game.Turn, kind)

		if m.Game.PieceInHand(*piece) {
			return col, true
		}
	}

	return 0, false
}

func (m *Model) gameOver() bool {
	return m.Game != nil && m.Game.Status == engine.GameOver
}

func (m *Model) draw() bool {
	return m.gameOver() && m.winner() == nil
}

func (m *Model) winner() *engine.Color {
	return m.Game.Winner
}

func (m *Model) online() bool {
	return m.Mode == ModeOnline
}

func (m *Model) localGame() bool {
	return m.Mode == ModeLocal
}

func (m *Model) myTurn() bool {
	return m.Game != nil && m.Game.Turn == m.MyColor
}

func (m *Model) colorScheme() ColorScheme {
	return ColorSchemes[m.SchemeIdx]
}

func (m *Model) shouldFlip() bool {
	return m.online() && m.MyColor == engine.Black
}

func (m *Model) cursorOnBoard() bool {
	return m.Cursor.onBoard()
}

func (m *Model) layout() layout {
	if m.shouldFlip() { // if my color is black, flip the board
		// white on top, black on bottom
		return layout{
			topColor:    engine.White,
			bottomColor: engine.Black,
			upDelta:     +1,
			downDelta:   -1,
			topRow:      engine.BoardSize - 1,
			bottomRow:   0,
		}
	}

	// black on top, white on bottom
	return layout{
		topColor:    engine.Black,
		bottomColor: engine.White,
		upDelta:     -1,
		downDelta:   +1,
		topRow:      0,
		bottomRow:   engine.BoardSize - 1,
	}
}
