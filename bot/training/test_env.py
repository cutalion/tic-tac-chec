"""Tests for Tic Tac Chec game environment.

Verifies: placement, piece movement (all 4 types), captures,
pawn direction reversal, win detection, draw detection.
"""

import pytest
from env import (
    TicTacChecEnv, Piece, Color, PieceKind,
    BOARD_SIZE, TO_BLACK_SIDE, TO_WHITE_SIDE,
)


@pytest.fixture
def env():
    return TicTacChecEnv()


# --- Placement ---

class TestPlacement:
    def test_place_piece_on_empty_cell(self, env):
        piece = Piece(Color.WHITE, PieceKind.PAWN)
        assert env._is_in_hand(piece)
        action = env.encode_action_drop(PieceKind.PAWN, 2, 2)
        env.step(action)
        assert env.board[2][2] == piece
        assert not env._is_in_hand(piece)

    def test_cannot_place_on_occupied_cell(self, env):
        env.board[1][1] = Piece(Color.WHITE, PieceKind.ROOK)
        # Try to drop pawn on occupied cell
        action = env.encode_action_drop(PieceKind.PAWN, 1, 1)
        with pytest.raises(ValueError, match="Cannot place"):
            env.step(action)

    def test_all_pieces_start_in_hand(self, env):
        for color in Color:
            hand = env._pieces_in_hand(color)
            assert len(hand) == 4
            kinds = {p.kind for p in hand}
            assert kinds == {PieceKind.PAWN, PieceKind.ROOK, PieceKind.BISHOP, PieceKind.KNIGHT}


# --- Piece Movement ---

class TestRookMoves:
    def test_rook_slides_horizontally_and_vertically(self, env):
        rook = Piece(Color.WHITE, PieceKind.ROOK)
        env.board[2][2] = rook
        moves = env._piece_moves(rook)
        # Should be able to move along row and column
        assert (2, 0) in moves
        assert (2, 1) in moves
        assert (2, 3) in moves
        assert (0, 2) in moves
        assert (1, 2) in moves
        assert (3, 2) in moves
        # Should NOT move diagonally
        assert (3, 3) not in moves
        assert (1, 1) not in moves

    def test_rook_blocked_by_own_piece(self, env):
        rook = Piece(Color.WHITE, PieceKind.ROOK)
        env.board[2][2] = rook
        env.board[2][0] = Piece(Color.WHITE, PieceKind.PAWN)
        moves = env._piece_moves(rook)
        # Blocked: can reach (2,1) but not (2,0)
        assert (2, 1) in moves
        assert (2, 0) not in moves

    def test_rook_captures_opponent(self, env):
        rook = Piece(Color.WHITE, PieceKind.ROOK)
        env.board[2][2] = rook
        env.board[2][0] = Piece(Color.BLACK, PieceKind.PAWN)
        moves = env._piece_moves(rook)
        # Can capture at (2,0) but not pass through
        assert (2, 1) in moves
        assert (2, 0) in moves


class TestBishopMoves:
    def test_bishop_slides_diagonally(self, env):
        bishop = Piece(Color.WHITE, PieceKind.BISHOP)
        env.board[2][2] = bishop
        moves = env._piece_moves(bishop)
        assert (1, 1) in moves
        assert (0, 0) in moves
        assert (3, 3) in moves
        assert (1, 3) in moves
        assert (3, 1) in moves
        # Should NOT move in straight lines
        assert (2, 0) not in moves
        assert (0, 2) not in moves


class TestKnightMoves:
    def test_knight_L_shaped_moves(self, env):
        knight = Piece(Color.WHITE, PieceKind.KNIGHT)
        env.board[2][2] = knight
        moves = env._piece_moves(knight)
        expected = [(0, 1), (0, 3), (1, 0), (3, 0)]
        for pos in expected:
            assert pos in moves, f"Expected {pos} in knight moves"

    def test_knight_jumps_over_pieces(self, env):
        knight = Piece(Color.WHITE, PieceKind.KNIGHT)
        env.board[2][2] = knight
        # Surround with pieces
        env.board[2][1] = Piece(Color.WHITE, PieceKind.PAWN)
        env.board[1][2] = Piece(Color.WHITE, PieceKind.ROOK)
        moves = env._piece_moves(knight)
        # Knight should still be able to jump
        assert len(moves) > 0


class TestPawnMoves:
    def test_white_pawn_moves_toward_black_side(self, env):
        pawn = Piece(Color.WHITE, PieceKind.PAWN)
        env.board[2][1] = pawn
        moves = env._piece_moves(pawn)
        # White pawn moves toward row 0 (TO_BLACK_SIDE = -1)
        assert (1, 1) in moves
        assert (3, 1) not in moves

    def test_pawn_captures_diagonally(self, env):
        pawn = Piece(Color.WHITE, PieceKind.PAWN)
        env.board[2][1] = pawn
        env.board[1][0] = Piece(Color.BLACK, PieceKind.ROOK)
        env.board[1][2] = Piece(Color.BLACK, PieceKind.BISHOP)
        moves = env._piece_moves(pawn)
        assert (1, 0) in moves  # capture left
        assert (1, 2) in moves  # capture right

    def test_pawn_cannot_capture_own_piece(self, env):
        pawn = Piece(Color.WHITE, PieceKind.PAWN)
        env.board[2][1] = pawn
        env.board[1][0] = Piece(Color.WHITE, PieceKind.ROOK)
        moves = env._piece_moves(pawn)
        assert (1, 0) not in moves

    def test_pawn_blocked_by_piece_ahead(self, env):
        pawn = Piece(Color.WHITE, PieceKind.PAWN)
        env.board[2][1] = pawn
        env.board[1][1] = Piece(Color.BLACK, PieceKind.ROOK)
        moves = env._piece_moves(pawn)
        assert (1, 1) not in moves  # blocked (pawn doesn't capture forward)


# --- Capture to hand ---

class TestCapture:
    def test_capture_returns_piece_to_hand(self, env):
        white_rook = Piece(Color.WHITE, PieceKind.ROOK)
        black_pawn = Piece(Color.BLACK, PieceKind.PAWN)
        env.board[2][2] = white_rook
        env.board[2][0] = black_pawn

        # White rook captures black pawn
        action = env.encode_action_move(2, 2, 2, 0)
        env.step(action)

        assert env.board[2][0] == white_rook
        assert env.board[2][2] is None
        # Black pawn should be back in hand
        black_hand = env._pieces_in_hand(Color.BLACK)
        assert any(p.kind == PieceKind.PAWN for p in black_hand)


# --- Pawn direction reversal ---

class TestPawnReversal:
    def test_white_pawn_reverses_at_row_0(self, env):
        pawn = Piece(Color.WHITE, PieceKind.PAWN)
        env.board[1][0] = pawn
        assert env.pawn_directions[Color.WHITE] == TO_BLACK_SIDE

        # Move pawn to row 0
        action = env.encode_action_move(1, 0, 0, 0)
        env.step(action)

        # After turn switch, direction should reverse
        assert env.pawn_directions[Color.WHITE] == TO_WHITE_SIDE

    def test_black_pawn_reverses_at_row_3(self, env):
        # Place white piece first (it's white's turn)
        env.board[0][0] = Piece(Color.WHITE, PieceKind.ROOK)
        env.step(env.encode_action_drop(PieceKind.PAWN, 3, 3))  # white drops pawn

        # Now black's turn
        black_pawn = Piece(Color.BLACK, PieceKind.PAWN)
        env.board[2][1] = black_pawn
        assert env.pawn_directions[Color.BLACK] == TO_WHITE_SIDE

        action = env.encode_action_move(2, 1, 3, 1)
        env.step(action)
        assert env.pawn_directions[Color.BLACK] == TO_BLACK_SIDE

    def test_pawn_resets_direction_when_captured(self, env):
        white_pawn = Piece(Color.WHITE, PieceKind.PAWN)
        env.board[0][0] = white_pawn
        # Pawn at row 0 → direction reversed to TO_WHITE_SIDE
        env.pawn_directions[Color.WHITE] = TO_WHITE_SIDE

        # Black rook captures white pawn
        black_rook = Piece(Color.BLACK, PieceKind.ROOK)
        env.board[0][3] = black_rook
        # Skip white turn by placing something
        env.board[3][3] = Piece(Color.WHITE, PieceKind.KNIGHT)
        env.step(env.encode_action_drop(PieceKind.BISHOP, 1, 1))  # white drops bishop
        # Now black captures
        action = env.encode_action_move(0, 3, 0, 0)
        env.step(action)

        # White pawn is now in hand, direction should reset
        assert env.pawn_directions[Color.WHITE] == TO_BLACK_SIDE


# --- Win detection ---

class TestWinDetection:
    def test_four_in_a_row_horizontal(self, env):
        # Set up 3 white pieces in row 0, then place 4th
        env.board[0][0] = Piece(Color.WHITE, PieceKind.PAWN)
        env.board[0][1] = Piece(Color.WHITE, PieceKind.ROOK)
        env.board[0][2] = Piece(Color.WHITE, PieceKind.BISHOP)
        # Drop knight to complete the row
        action = env.encode_action_drop(PieceKind.KNIGHT, 0, 3)
        _, reward, done, info = env.step(action)
        assert done
        assert reward == 1.0
        assert info["winner"] == Color.WHITE

    def test_four_in_a_column(self, env):
        env.board[0][0] = Piece(Color.WHITE, PieceKind.PAWN)
        env.board[1][0] = Piece(Color.WHITE, PieceKind.ROOK)
        env.board[2][0] = Piece(Color.WHITE, PieceKind.BISHOP)
        action = env.encode_action_drop(PieceKind.KNIGHT, 3, 0)
        _, reward, done, info = env.step(action)
        assert done
        assert reward == 1.0

    def test_four_in_diagonal(self, env):
        env.board[0][0] = Piece(Color.WHITE, PieceKind.PAWN)
        env.board[1][1] = Piece(Color.WHITE, PieceKind.ROOK)
        env.board[2][2] = Piece(Color.WHITE, PieceKind.BISHOP)
        action = env.encode_action_drop(PieceKind.KNIGHT, 3, 3)
        _, reward, done, info = env.step(action)
        assert done

    def test_mixed_colors_no_win(self, env):
        env.board[0][0] = Piece(Color.WHITE, PieceKind.PAWN)
        env.board[0][1] = Piece(Color.BLACK, PieceKind.ROOK)
        env.board[0][2] = Piece(Color.WHITE, PieceKind.BISHOP)
        action = env.encode_action_drop(PieceKind.KNIGHT, 0, 3)
        _, reward, done, _ = env.step(action)
        assert not done


# --- Draw detection ---

class TestDrawDetection:
    def test_draw_at_move_limit(self, env):
        """Simulate reaching MAX_MOVES without a winner."""
        env.move_count = 99  # One move away from limit
        # Place pieces so we can make a legal non-winning move
        env.board[0][0] = Piece(Color.WHITE, PieceKind.ROOK)
        action = env.encode_action_move(0, 0, 0, 1)
        _, reward, done, info = env.step(action)
        assert done
        assert reward == 0.0
        assert info.get("draw") == "move_limit"
        assert env.draw

    def test_draw_by_repetition(self, env):
        """Same state occurring REPETITION_LIMIT times triggers draw."""
        # Place pieces for back-and-forth moves
        env.board[0][0] = Piece(Color.WHITE, PieceKind.ROOK)
        env.board[3][3] = Piece(Color.BLACK, PieceKind.ROOK)

        # Move rook back and forth to repeat states
        moves = [
            (0, 0, 0, 1),  # W: rook 0,0 → 0,1
            (3, 3, 3, 2),  # B: rook 3,3 → 3,2
            (0, 1, 0, 0),  # W: rook 0,1 → 0,0
            (3, 2, 3, 3),  # B: rook 3,2 → 3,3
        ]

        # Need to repeat enough times for 5-fold repetition
        # Initial state is recorded once. Each full cycle returns to initial state.
        done = False
        for cycle in range(10):  # More than enough cycles
            for src_r, src_c, dst_r, dst_c in moves:
                if done:
                    break
                action = env.encode_action_move(src_r, src_c, dst_r, dst_c)
                _, reward, done, info = env.step(action)

            if done:
                break

        assert done
        assert env.draw
        assert info.get("draw") == "repetition"


# --- Turn management ---

class TestTurns:
    def test_alternating_turns(self, env):
        assert env.turn == Color.WHITE
        env.step(env.encode_action_drop(PieceKind.PAWN, 0, 0))
        assert env.turn == Color.BLACK
        env.step(env.encode_action_drop(PieceKind.PAWN, 3, 3))
        assert env.turn == Color.WHITE

    def test_cannot_move_opponents_piece(self, env):
        env.board[0][0] = Piece(Color.BLACK, PieceKind.ROOK)
        # White's turn, trying to move black's rook
        action = env.encode_action_move(0, 0, 0, 1)
        with pytest.raises(ValueError, match="No own piece"):
            env.step(action)


# --- Action encoding round-trip ---

class TestActionEncoding:
    def test_drop_encode_decode_roundtrip(self, env):
        action = env.encode_action_drop(PieceKind.BISHOP, 2, 3)
        decoded = env.decode_action(action)
        assert decoded[0] == "drop"
        assert decoded[1] == Piece(Color.WHITE, PieceKind.BISHOP)
        assert decoded[2] == 2
        assert decoded[3] == 3

    def test_move_encode_decode_roundtrip(self, env):
        action = env.encode_action_move(1, 2, 3, 0)
        decoded = env.decode_action(action)
        assert decoded[0] == "move"
        assert decoded[1] == (1, 2)
        assert decoded[2] == (3, 0)

    def test_action_space_boundaries(self, env):
        # First drop action
        assert env.encode_action_drop(PieceKind.PAWN, 0, 0) == 0
        # Last drop action
        assert env.encode_action_drop(PieceKind.KNIGHT, 3, 3) == 63
        # First move action
        assert env.encode_action_move(0, 0, 0, 0) == 64
        # Last move action
        assert env.encode_action_move(3, 3, 3, 3) == 319


# --- Reset ---

class TestReset:
    def test_reset_clears_board(self, env):
        env.board[0][0] = Piece(Color.WHITE, PieceKind.PAWN)
        env.move_count = 50
        env.done = True
        env.reset()
        assert all(env.board[r][c] is None for r in range(BOARD_SIZE) for c in range(BOARD_SIZE))
        assert env.move_count == 0
        assert not env.done
        assert env.turn == Color.WHITE
