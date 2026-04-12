"""
Tic Tac Chec game environment for reinforcement learning.

Faithfully replicates the Go engine rules:
- 4x4 board, 2 players (White=0, Black=1)
- 4 piece types per player: Pawn, Rook, Bishop, Knight
- Place from hand or move on board (chess-style movement)
- Capture returns piece to owner's hand (shogi-style)
- Pawn reverses direction at far edge (no promotion)
- Win: 4 of your color in a row (horizontal, vertical, diagonal)
- Draw: 100 moves or 5-fold state repetition
"""

from enum import IntEnum
from typing import Optional

import numpy as np

# --- Constants ---

BOARD_SIZE = 4
MAX_MOVES = 100
REPETITION_LIMIT = 5

# Action space: 320 total
# Drops: 4 piece_types * 16 cells = 64 (indices 0-63)
# Moves: 16 source_cells * 16 target_cells = 256 (indices 64-319)
NUM_DROP_ACTIONS = 4 * BOARD_SIZE * BOARD_SIZE  # 64
NUM_MOVE_ACTIONS = (BOARD_SIZE * BOARD_SIZE) * (BOARD_SIZE * BOARD_SIZE)  # 256
ACTION_SPACE_SIZE = NUM_DROP_ACTIONS + NUM_MOVE_ACTIONS  # 320

# State tensor: 19 channels of 4x4
NUM_CHANNELS = 19


class Color(IntEnum):
    WHITE = 0
    BLACK = 1


class PieceKind(IntEnum):
    PAWN = 0
    ROOK = 1
    BISHOP = 2
    KNIGHT = 3


PIECE_KIND_COUNT = 4

# Pawn directions: -1 = toward row 0 (black side), +1 = toward row 3 (white side)
TO_BLACK_SIDE = -1
TO_WHITE_SIDE = 1


class Piece:
    __slots__ = ("color", "kind")

    def __init__(self, color: Color, kind: PieceKind):
        self.color = color
        self.kind = kind

    def __eq__(self, other):
        return (
            isinstance(other, Piece)
            and self.color == other.color
            and self.kind == other.kind
        )

    def __hash__(self):
        return hash((self.color, self.kind))

    def __repr__(self):
        color_name = "W" if self.color == Color.WHITE else "B"
        kind_name = ["P", "R", "B", "N"][self.kind]
        return f"{color_name}{kind_name}"


# order is important. Used to build the channels and masks
ALL_PIECES = [
    Piece(Color.WHITE, PieceKind.PAWN),
    Piece(Color.WHITE, PieceKind.ROOK),
    Piece(Color.WHITE, PieceKind.BISHOP),
    Piece(Color.WHITE, PieceKind.KNIGHT),
    Piece(Color.BLACK, PieceKind.PAWN),
    Piece(Color.BLACK, PieceKind.ROOK),
    Piece(Color.BLACK, PieceKind.BISHOP),
    Piece(Color.BLACK, PieceKind.KNIGHT),
]


class TicTacChecEnv:
    """Tic Tac Chec environment with standard RL interface."""

    def __init__(self):
        self.board: list[list[Optional[Piece]]] = []
        self.turn: Color = Color.WHITE
        self.pawn_directions: dict[Color, int] = {}
        self.move_count: int = 0
        self.state_history: dict[tuple, int] = {}
        self.winner: Optional[Color] = None
        self.done: bool = False
        self.draw: bool = False
        self.reset()

    def reset(self):
        """Reset to initial state. Returns initial observation."""
        self.board = [[None] * BOARD_SIZE for _ in range(BOARD_SIZE)]
        self.turn = Color.WHITE
        self.pawn_directions = {
            Color.WHITE: TO_BLACK_SIDE,  # white moves up (toward row 0)
            Color.BLACK: TO_WHITE_SIDE,  # black moves down (toward row 3)
        }
        self.move_count = 0
        self.state_history = {}
        self.winner = None
        self.done = False
        self.draw = False
        self._record_state()
        return self.encode_state()

    # --- Piece tracking ---

    def _pieces_on_board(self, color: Color) -> list[tuple[Piece, int, int]]:
        """Return [(piece, row, col)] for all pieces of given color on board."""
        result = []
        for r in range(BOARD_SIZE):
            for c in range(BOARD_SIZE):
                p = self.board[r][c]
                if p is not None and p.color == color:
                    result.append((p, r, c))
        return result

    def _pieces_in_hand(self, color: Color) -> list[Piece]:
        """Return pieces of given color that are NOT on the board."""
        on_board_kinds = set()
        for r in range(BOARD_SIZE):
            for c in range(BOARD_SIZE):
                p = self.board[r][c]
                if p is not None and p.color == color:
                    on_board_kinds.add(p.kind)
        hand = []
        for kind in PieceKind:
            if kind not in on_board_kinds:
                hand.append(Piece(color, kind))
        return hand

    def _find_piece(self, piece: Piece) -> Optional[tuple[int, int]]:
        """Find a piece on the board. Returns (row, col) or None."""
        for r in range(BOARD_SIZE):
            for c in range(BOARD_SIZE):
                p = self.board[r][c]
                if p is not None and p == piece:
                    return (r, c)
        return None

    def _is_in_hand(self, piece: Piece) -> bool:
        return self._find_piece(piece) is None

    # --- Movement rules ---

    def _piece_moves(self, piece: Piece) -> list[tuple[int, int]]:
        """Legal moves for a piece currently on the board."""
        pos = self._find_piece(piece)
        if pos is None:
            return []
        row, col = pos

        if piece.kind == PieceKind.PAWN:
            return self._pawn_moves(piece, row, col)
        elif piece.kind == PieceKind.ROOK:
            return self._slide_moves(
                piece, row, col, [(0, 1), (0, -1), (1, 0), (-1, 0)]
            )
        elif piece.kind == PieceKind.BISHOP:
            return self._slide_moves(
                piece, row, col, [(-1, -1), (-1, 1), (1, -1), (1, 1)]
            )
        elif piece.kind == PieceKind.KNIGHT:
            return self._knight_moves(piece, row, col)
        return []

    def _pawn_moves(self, pawn: Piece, row: int, col: int) -> list[tuple[int, int]]:
        direction = self.pawn_directions[pawn.color]
        moves = []
        # Forward move (no capture)
        nr = row + direction
        if 0 <= nr < BOARD_SIZE and self.board[nr][col] is None:
            moves.append((nr, col))
        # Diagonal captures
        for dc in [-1, 1]:
            nc = col + dc
            if 0 <= nr < BOARD_SIZE and 0 <= nc < BOARD_SIZE:
                target = self.board[nr][nc]
                if target is not None and target.color != pawn.color:
                    moves.append((nr, nc))
        return moves

    def _slide_moves(
        self, piece: Piece, row: int, col: int, directions: list[tuple[int, int]]
    ) -> list[tuple[int, int]]:
        moves = []
        for dr, dc in directions:
            r, c = row, col
            for _ in range(BOARD_SIZE - 1):
                r += dr
                c += dc
                if not (0 <= r < BOARD_SIZE and 0 <= c < BOARD_SIZE):
                    break
                target = self.board[r][c]
                if target is None:
                    moves.append((r, c))
                elif target.color != piece.color:
                    moves.append((r, c))  # capture
                    break
                else:
                    break  # blocked by own piece
        return moves

    def _knight_moves(self, piece: Piece, row: int, col: int) -> list[tuple[int, int]]:
        offsets = [
            (-2, -1),
            (-2, 1),
            (2, -1),
            (2, 1),
            (-1, -2),
            (-1, 2),
            (1, -2),
            (1, 2),
        ]
        moves = []
        for dr, dc in offsets:
            r, c = row + dr, col + dc
            if 0 <= r < BOARD_SIZE and 0 <= c < BOARD_SIZE:
                target = self.board[r][c]
                if target is None or target.color != piece.color:
                    moves.append((r, c))
        return moves

    # --- Core game logic ---

    def _place_piece(self, piece: Piece, row: int, col: int) -> bool:
        """Place a hand piece on an empty cell."""
        if self.board[row][col] is not None:
            return False
        self.board[row][col] = piece
        return True

    def _move_piece(self, piece: Piece, to_row: int, to_col: int) -> bool:
        """Move a board piece to a target cell (may capture)."""
        legal = self._piece_moves(piece)
        if (to_row, to_col) not in legal:
            return False
        from_pos = self._find_piece(piece)
        if from_pos is None:
            return False
        fr, fc = from_pos
        # Move (capture is implicit: overwriting removes captured piece from board,
        # making it "in hand" for its owner since we track by presence)
        self.board[fr][fc] = None
        self.board[to_row][to_col] = piece
        return True

    def _check_win(self) -> bool:
        """Check if current player (self.turn) has 4 in a row."""
        lines = []
        # Rows
        for r in range(BOARD_SIZE):
            lines.append([(r, c) for c in range(BOARD_SIZE)])
        # Columns
        for c in range(BOARD_SIZE):
            lines.append([(r, c) for r in range(BOARD_SIZE)])
        # Diagonals
        lines.append([(i, i) for i in range(BOARD_SIZE)])
        lines.append([(i, BOARD_SIZE - 1 - i) for i in range(BOARD_SIZE)])

        for line in lines:
            if all(
                self.board[r][c] is not None and self.board[r][c].color == self.turn
                for r, c in line
            ):
                return True
        return False

    def _maybe_reverse_pawn_direction(self, pawn: Piece):
        """Reverse pawn direction if it reached the far edge."""
        pos = self._find_piece(pawn)
        if pos is not None:
            row, _ = pos
            if self.pawn_directions[pawn.color] == TO_BLACK_SIDE and row == 0:
                self.pawn_directions[pawn.color] = TO_WHITE_SIDE
            elif (
                self.pawn_directions[pawn.color] == TO_WHITE_SIDE
                and row == BOARD_SIZE - 1
            ):
                self.pawn_directions[pawn.color] = TO_BLACK_SIDE
        else:
            # Pawn not on board: reset to initial direction
            if pawn.color == Color.WHITE:
                self.pawn_directions[Color.WHITE] = TO_BLACK_SIDE
            else:
                self.pawn_directions[Color.BLACK] = TO_WHITE_SIDE

    def _state_hash(self) -> tuple:
        """Deterministic hash of current game state for repetition detection."""
        board_tuple = tuple(
            tuple((p.color, p.kind) if p is not None else None for p in row)
            for row in self.board
        )
        # Hand pieces for each color (sorted by kind for determinism)
        white_hand = tuple(sorted(p.kind for p in self._pieces_in_hand(Color.WHITE)))
        black_hand = tuple(sorted(p.kind for p in self._pieces_in_hand(Color.BLACK)))
        return (board_tuple, white_hand, black_hand, int(self.turn))

    def _record_state(self):
        """Record current state for repetition detection."""
        h = self._state_hash()
        self.state_history[h] = self.state_history.get(h, 0) + 1

    def _check_repetition(self) -> bool:
        """Check if current state has occurred REPETITION_LIMIT times."""
        h = self._state_hash()
        return self.state_history.get(h, 0) >= REPETITION_LIMIT

    # --- Action encoding/decoding ---

    def decode_action(self, action: int) -> tuple:
        """Decode action index to (action_type, piece, row, col) or (action_type, src, dst).

        Returns:
            For drops: ("drop", Piece, target_row, target_col)
            For moves: ("move", (src_row, src_col), (dst_row, dst_col))
        """
        if action < NUM_DROP_ACTIONS:
            # Drop action: piece_type * 16 + row * 4 + col
            piece_type = action // (BOARD_SIZE * BOARD_SIZE)
            remainder = action % (BOARD_SIZE * BOARD_SIZE)
            row = remainder // BOARD_SIZE
            col = remainder % BOARD_SIZE
            piece = Piece(self.turn, PieceKind(piece_type))
            return ("drop", piece, row, col)
        else:
            # Move action: 64 + src * 16 + dst
            idx = action - NUM_DROP_ACTIONS
            src = idx // (BOARD_SIZE * BOARD_SIZE)
            dst = idx % (BOARD_SIZE * BOARD_SIZE)
            src_row, src_col = src // BOARD_SIZE, src % BOARD_SIZE
            dst_row, dst_col = dst // BOARD_SIZE, dst % BOARD_SIZE
            return ("move", (src_row, src_col), (dst_row, dst_col))

    def encode_action_drop(self, piece_kind: PieceKind, row: int, col: int) -> int:
        """Encode a drop action to action index."""
        return int(piece_kind) * (BOARD_SIZE * BOARD_SIZE) + row * BOARD_SIZE + col

    def encode_action_move(
        self, src_row: int, src_col: int, dst_row: int, dst_col: int
    ) -> int:
        """Encode a move action to action index."""
        src = src_row * BOARD_SIZE + src_col
        dst = dst_row * BOARD_SIZE + dst_col
        return NUM_DROP_ACTIONS + src * (BOARD_SIZE * BOARD_SIZE) + dst

    # --- RL interface ---

    def legal_actions(self) -> list[int]:
        """Return list of legal action indices for current player."""
        actions = []
        hand = self._pieces_in_hand(self.turn)

        # Drop actions
        for piece in hand:
            for r in range(BOARD_SIZE):
                for c in range(BOARD_SIZE):
                    if self.board[r][c] is None:
                        actions.append(self.encode_action_drop(piece.kind, r, c))

        # Move actions
        for piece, row, col in self._pieces_on_board(self.turn):
            src = row * BOARD_SIZE + col
            for mr, mc in self._piece_moves(piece):
                dst = mr * BOARD_SIZE + mc
                actions.append(NUM_DROP_ACTIONS + src * (BOARD_SIZE * BOARD_SIZE) + dst)

        return actions

    def legal_action_mask(self) -> np.ndarray:
        """Return boolean mask of shape (320,). True = legal action."""
        mask = np.zeros(ACTION_SPACE_SIZE, dtype=bool)
        for a in self.legal_actions():
            mask[a] = True
        return mask

    def encode_state(self) -> np.ndarray:
        """Encode current game state as tensor of shape (19, 4, 4)."""

        state = np.zeros((NUM_CHANNELS, BOARD_SIZE, BOARD_SIZE), dtype=np.float32)

        # channel 0: white pawns
        # channel 1: white rooks
        # channel 2: white bishops
        # channel 3: white knights
        # channel 4: black pawns
        # channel 5: black rooks
        # channel 6: black bishops
        # channel 7: black knights
        #
        # fill channels 0-7 (pieces on board)
        for i, piece in enumerate(ALL_PIECES):
            res = self._find_piece(piece)
            if res is not None:
                state[i, res[0], res[1]] = 1.0

        # fill channels 8-15 (pieces in hand)
        for i, piece in enumerate(ALL_PIECES):
            if self._is_in_hand(piece):
                state[i + 8] = 1.0

        # turn indicator (channel 16)
        state[16] = 1.0 if self.turn == Color.WHITE else 0.0

        # white pawn dir (channel 17)
        state[17] = 1.0 if self.pawn_directions[Color.WHITE] == TO_BLACK_SIDE else 0.0

        # black pawn dir (channel 18)
        state[18] = 1.0 if self.pawn_directions[Color.BLACK] == TO_BLACK_SIDE else 0.0

        return state

    def step(self, action: int) -> tuple[np.ndarray, float, bool, dict]:
        """Execute action. Returns (observation, reward, done, info).

        reward: +1 win, -1 loss, 0 draw or ongoing
        """
        if self.done:
            raise RuntimeError("Game is already over")

        decoded = self.decode_action(action)

        if decoded[0] == "drop":
            _, piece, row, col = decoded
            if not self._is_in_hand(piece):
                raise ValueError(f"Piece {piece} is not in hand")
            if not self._place_piece(piece, row, col):
                raise ValueError(f"Cannot place {piece} at ({row}, {col})")
        else:
            _, (src_row, src_col), (dst_row, dst_col) = decoded
            piece = self.board[src_row][src_col]
            if piece is None or piece.color != self.turn:
                raise ValueError(f"No own piece at ({src_row}, {src_col})")
            if not self._move_piece(piece, dst_row, dst_col):
                raise ValueError(f"Illegal move for {piece} to ({dst_row}, {dst_col})")

        self.move_count += 1

        # Check win (before turn switch — current player just moved)
        if self._check_win():
            self.winner = self.turn
            self.done = True
            reward = 1.0  # current player wins
            return (
                self.encode_state(),
                reward,
                True,
                {"winner": self.turn, "move_count": self.move_count},
            )

        # Switch turn
        self.turn = Color.BLACK if self.turn == Color.WHITE else Color.WHITE

        # Update pawn directions after turn switch (matches Go engine behavior)
        self._maybe_reverse_pawn_direction(Piece(Color.WHITE, PieceKind.PAWN))
        self._maybe_reverse_pawn_direction(Piece(Color.BLACK, PieceKind.PAWN))

        # Record state for repetition (after turn switch)
        self._record_state()

        # Check draw conditions
        if self.move_count >= MAX_MOVES:
            self.done = True
            self.draw = True
            return (
                self.encode_state(),
                0.0,
                True,
                {"draw": "move_limit", "move_count": self.move_count},
            )

        if self._check_repetition():
            self.done = True
            self.draw = True
            return (
                self.encode_state(),
                0.0,
                True,
                {"draw": "repetition", "move_count": self.move_count},
            )

        return self.encode_state(), 0.0, False, {"move_count": self.move_count}
