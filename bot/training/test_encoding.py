"""Tests for state encoding, action masking, and action decoding.

Verifies: encode_state() channel layout, legal_action_mask() correctness,
decode_action() round-trips with encode, mask blocks illegal moves.
"""

import numpy as np
import pytest
from env import (
    TicTacChecEnv, Piece, Color, PieceKind,
    BOARD_SIZE, NUM_CHANNELS, ACTION_SPACE_SIZE,
    TO_BLACK_SIDE, TO_WHITE_SIDE,
)


@pytest.fixture
def env():
    return TicTacChecEnv()


# --- encode_state: shape and dtype ---

class TestEncodeStateBasics:
    def test_shape(self, env):
        state = env.encode_state()
        assert state.shape == (NUM_CHANNELS, BOARD_SIZE, BOARD_SIZE)

    def test_dtype(self, env):
        state = env.encode_state()
        assert state.dtype == np.float32

    def test_initial_state_all_in_hand(self, env):
        """At start, no pieces on board (ch 0-7 zero), all in hand (ch 8-15 ones)."""
        state = env.encode_state()
        # Board channels should be all zeros
        assert np.all(state[:8] == 0.0)
        # Hand channels should be all ones
        assert np.all(state[8:16] == 1.0)

    def test_initial_turn_is_white(self, env):
        state = env.encode_state()
        assert np.all(state[16] == 1.0)

    def test_initial_pawn_directions(self, env):
        state = env.encode_state()
        # White starts moving toward black side (row 0) → channel 17 = 1.0
        assert np.all(state[17] == 1.0)
        # Black starts moving toward white side (row 3) → channel 18 = 0.0
        assert np.all(state[18] == 0.0)


# --- encode_state: piece placement ---

class TestEncodeStatePieces:
    def test_white_pawn_on_board(self, env):
        """White pawn at (1,2) → channel 0 has 1.0 at (1,2), channel 8 is zero."""
        pawn = Piece(Color.WHITE, PieceKind.PAWN)
        env.board[1][2] = pawn
        state = env.encode_state()
        assert state[0, 1, 2] == 1.0
        # Rest of channel 0 should be zero
        assert np.sum(state[0]) == 1.0
        # Hand channel for white pawn should be zero (piece on board)
        assert np.all(state[8] == 0.0)

    def test_black_knight_on_board(self, env):
        """Black knight at (3,0) → channel 7 has 1.0 at (3,0)."""
        knight = Piece(Color.BLACK, PieceKind.KNIGHT)
        env.board[3][0] = knight
        state = env.encode_state()
        assert state[7, 3, 0] == 1.0
        assert np.sum(state[7]) == 1.0
        assert np.all(state[15] == 0.0)  # Not in hand

    def test_multiple_pieces(self, env):
        """Place several pieces, verify each channel."""
        env.board[0][0] = Piece(Color.WHITE, PieceKind.ROOK)
        env.board[2][3] = Piece(Color.BLACK, PieceKind.BISHOP)
        state = env.encode_state()
        # White rook: channel 1
        assert state[1, 0, 0] == 1.0
        assert np.all(state[9] == 0.0)  # Rook not in hand
        # Black bishop: channel 6
        assert state[6, 2, 3] == 1.0
        assert np.all(state[14] == 0.0)  # Bishop not in hand
        # Other pieces still in hand
        assert np.all(state[8] == 1.0)   # White pawn in hand
        assert np.all(state[12] == 1.0)  # Black pawn in hand


# --- encode_state: turn and pawn direction ---

class TestEncodeStateContext:
    def test_black_turn(self, env):
        """After white moves, turn channel should be all zeros."""
        env.step(env.encode_action_drop(PieceKind.PAWN, 0, 0))
        state = env.encode_state()
        assert np.all(state[16] == 0.0)

    def test_pawn_direction_reversal(self, env):
        """After white pawn reaches row 0, channel 17 should flip to 0."""
        pawn = Piece(Color.WHITE, PieceKind.PAWN)
        env.board[1][0] = pawn
        env.step(env.encode_action_move(1, 0, 0, 0))
        # White's pawn direction reversed to TO_WHITE_SIDE
        state = env.encode_state()
        assert np.all(state[17] == 0.0)


# --- legal_action_mask ---

class TestLegalActionMask:
    def test_mask_shape_and_dtype(self, env):
        mask = env.legal_action_mask()
        assert mask.shape == (ACTION_SPACE_SIZE,)
        assert mask.dtype == bool

    def test_initial_mask_only_drops(self, env):
        """At start, only drop actions are legal (no pieces on board to move)."""
        mask = env.legal_action_mask()
        # All 64 drop indices should be True (4 pieces × 16 cells)
        assert np.sum(mask[:64]) == 64
        # No move actions should be legal
        assert np.sum(mask[64:]) == 0

    def test_mask_blocks_occupied_drops(self, env):
        """Cannot drop on occupied cells."""
        env.board[1][1] = Piece(Color.WHITE, PieceKind.ROOK)
        mask = env.legal_action_mask()
        # Cell (1,1) is occupied: drop actions targeting it should be blocked
        for kind_idx in range(4):
            action = kind_idx * 16 + 1 * 4 + 1
            assert not mask[action], f"Drop kind={kind_idx} to (1,1) should be blocked"

    def test_mask_includes_moves_for_placed_piece(self, env):
        """After placing a rook, mask should include move actions from its position."""
        env.board[2][2] = Piece(Color.WHITE, PieceKind.ROOK)
        mask = env.legal_action_mask()
        # Rook at (2,2) can move to (2,0), (2,1), (2,3), (0,2), (1,2), (3,2)
        src = 2 * 4 + 2  # cell index 10
        for dst_r, dst_c in [(2, 0), (2, 1), (2, 3), (0, 2), (1, 2), (3, 2)]:
            dst = dst_r * 4 + dst_c
            action = 64 + src * 16 + dst
            assert mask[action], f"Rook move to ({dst_r},{dst_c}) should be legal"

    def test_mask_agrees_with_legal_actions(self, env):
        """Mask and legal_actions() should produce identical sets."""
        env.board[0][0] = Piece(Color.WHITE, PieceKind.ROOK)
        env.board[3][3] = Piece(Color.BLACK, PieceKind.BISHOP)
        mask = env.legal_action_mask()
        legal = set(env.legal_actions())
        mask_set = set(np.where(mask)[0])
        assert mask_set == legal


# --- decode_action round-trips ---

class TestDecodeRoundTrip:
    def test_drop_round_trip_all_kinds(self, env):
        """Encode then decode every piece kind at a sample cell."""
        for kind in PieceKind:
            action = env.encode_action_drop(kind, 1, 3)
            typ, piece, row, col = env.decode_action(action)
            assert typ == "drop"
            assert piece.kind == kind
            assert piece.color == Color.WHITE  # White's turn
            assert (row, col) == (1, 3)

    def test_move_round_trip(self, env):
        for sr, sc, dr, dc in [(0, 0, 3, 3), (2, 1, 0, 3), (3, 3, 0, 0)]:
            action = env.encode_action_move(sr, sc, dr, dc)
            typ, src, dst = env.decode_action(action)
            assert typ == "move"
            assert src == (sr, sc)
            assert dst == (dr, dc)

    def test_drop_boundary_indices(self, env):
        """First drop (pawn at 0,0) = 0, last drop (knight at 3,3) = 63."""
        assert env.encode_action_drop(PieceKind.PAWN, 0, 0) == 0
        assert env.encode_action_drop(PieceKind.KNIGHT, 3, 3) == 63

    def test_move_boundary_indices(self, env):
        """First move = 64, last move = 319."""
        assert env.encode_action_move(0, 0, 0, 0) == 64
        assert env.encode_action_move(3, 3, 3, 3) == 319

    def test_no_overlap_between_drops_and_moves(self, env):
        """Drop indices [0,63] and move indices [64,319] don't overlap."""
        drops = set()
        for kind in PieceKind:
            for r in range(BOARD_SIZE):
                for c in range(BOARD_SIZE):
                    drops.add(env.encode_action_drop(kind, r, c))
        assert min(drops) == 0
        assert max(drops) == 63

        moves = set()
        for sr in range(BOARD_SIZE):
            for sc in range(BOARD_SIZE):
                for dr in range(BOARD_SIZE):
                    for dc in range(BOARD_SIZE):
                        moves.add(env.encode_action_move(sr, sc, dr, dc))
        assert min(moves) == 64
        assert max(moves) == 319
        assert len(drops & moves) == 0
