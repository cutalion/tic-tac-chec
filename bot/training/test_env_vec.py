"""Tests for the vectorized Tic Tac Chec environment.

Verifies:
1. Basic shape/dtype contracts
2. Equivalence with the scalar env.py for random game sequences
3. Independent game handling (one finishes, others continue)
4. Win/draw detection
5. Legal action mask correctness
"""

import numpy as np
import pytest
from env import TicTacChecEnv, Color, PieceKind
from env_vec import (
    VecTicTacChecEnv,
    ACTION_SPACE_SIZE,
    NUM_CHANNELS,
    BOARD_SIZE,
    WHITE,
    BLACK,
    _piece_id,
    PAWN,
    ROOK,
    BISHOP,
    KNIGHT,
    TO_BLACK_SIDE,
    TO_WHITE_SIDE,
)


# --- Shape and dtype ---

class TestBasics:
    def test_reset_shape(self):
        env = VecTicTacChecEnv(4)
        obs = env.reset()
        assert obs.shape == (4, NUM_CHANNELS, BOARD_SIZE, BOARD_SIZE)
        assert obs.dtype == np.float32

    def test_encode_states_shape(self):
        env = VecTicTacChecEnv(3)
        states = env.encode_states()
        assert states.shape == (3, 19, 4, 4)

    def test_legal_action_masks_shape(self):
        env = VecTicTacChecEnv(5)
        masks = env.legal_action_masks()
        assert masks.shape == (5, ACTION_SPACE_SIZE)
        assert masks.dtype == bool

    def test_step_output_shapes(self):
        env = VecTicTacChecEnv(2)
        masks = env.legal_action_masks()
        actions = np.array([
            np.random.choice(np.where(masks[0])[0]),
            np.random.choice(np.where(masks[1])[0]),
        ])
        obs, rewards, dones, infos = env.step(actions)
        assert obs.shape == (2, 19, 4, 4)
        assert rewards.shape == (2,)
        assert dones.shape == (2,)
        assert len(infos) == 2


# --- Initial state ---

class TestInitialState:
    def test_initial_board_empty(self):
        env = VecTicTacChecEnv(2)
        assert np.all(env.boards == 0)

    def test_initial_turns_white(self):
        env = VecTicTacChecEnv(2)
        assert np.all(env.turns == WHITE)

    def test_initial_pawn_dirs(self):
        env = VecTicTacChecEnv(2)
        assert np.all(env.pawn_dirs[:, 0] == TO_BLACK_SIDE)
        assert np.all(env.pawn_dirs[:, 1] == TO_WHITE_SIDE)

    def test_initial_encoding_matches_scalar(self):
        """Initial encoded state should match scalar env."""
        scalar = TicTacChecEnv()
        scalar_state = scalar.encode_state()

        vec = VecTicTacChecEnv(3)
        vec_states = vec.encode_states()

        for i in range(3):
            np.testing.assert_array_equal(vec_states[i], scalar_state)

    def test_initial_mask_only_drops(self):
        """At start, only drop actions legal (64 total), no move actions."""
        env = VecTicTacChecEnv(2)
        masks = env.legal_action_masks()
        for i in range(2):
            assert np.sum(masks[i, :64]) == 64
            assert np.sum(masks[i, 64:]) == 0


# --- Equivalence with scalar env ---

class TestEquivalence:
    def _play_random_game_scalar(self, seed):
        """Play a game with the scalar env, return action sequence and outcomes."""
        rng = np.random.RandomState(seed)
        env = TicTacChecEnv()
        actions = []
        states = [env.encode_state().copy()]
        masks_list = [env.legal_action_mask().copy()]

        while not env.done:
            mask = env.legal_action_mask()
            legal = np.where(mask)[0]
            action = rng.choice(legal)
            actions.append(action)
            obs, reward, done, info = env.step(action)
            states.append(obs.copy())
            if not done:
                masks_list.append(env.legal_action_mask().copy())

        return actions, states, masks_list, env.done, env.draw, env.winner

    def test_single_game_matches_scalar(self):
        """A single vectorized game with same actions should match scalar env."""
        for seed in range(5):
            actions, states, masks_list, done, draw, winner = \
                self._play_random_game_scalar(seed)

            vec = VecTicTacChecEnv(1)
            vec_states = [vec.encode_states()[0].copy()]
            vec_masks = [vec.legal_action_masks()[0].copy()]

            for step_idx, action in enumerate(actions):
                obs, rewards, dones, infos = vec.step(np.array([action]))
                vec_states.append(obs[0].copy())
                if not dones[0]:
                    vec_masks.append(vec.legal_action_masks()[0].copy())

            # Compare all states
            for step_idx in range(len(states)):
                np.testing.assert_array_equal(
                    vec_states[step_idx], states[step_idx],
                    err_msg=f"State mismatch at step {step_idx}, seed {seed}"
                )

            # Compare all masks
            for step_idx in range(len(masks_list)):
                np.testing.assert_array_equal(
                    vec_masks[step_idx], masks_list[step_idx],
                    err_msg=f"Mask mismatch at step {step_idx}, seed {seed}"
                )

            # Compare final outcome
            assert vec.dones[0] == done
            assert vec.draws[0] == draw
            if winner is not None:
                assert vec.winners[0] == int(winner)

    def test_multiple_parallel_games_match_scalar(self):
        """Run N games in parallel, verify each matches independent scalar run."""
        N = 8
        seeds = list(range(100, 100 + N))

        # Run scalar games to get action sequences
        all_actions = []
        all_scalar_states = []
        all_scalar_masks = []
        max_len = 0
        for seed in seeds:
            actions, states, masks, _, _, _ = self._play_random_game_scalar(seed)
            all_actions.append(actions)
            all_scalar_states.append(states)
            all_scalar_masks.append(masks)
            max_len = max(max_len, len(actions))

        # Now replay in vectorized env
        vec = VecTicTacChecEnv(N)

        # Check initial states
        vec_init = vec.encode_states()
        for i in range(N):
            np.testing.assert_array_equal(vec_init[i], all_scalar_states[i][0])

        # Check initial masks
        vec_init_masks = vec.legal_action_masks()
        for i in range(N):
            np.testing.assert_array_equal(vec_init_masks[i], all_scalar_masks[i][0])

        # Step through
        mask_step = [0] * N  # track which mask step each env is at
        for step in range(max_len):
            batch_actions = np.zeros(N, dtype=np.int32)
            active = np.zeros(N, dtype=bool)
            for i in range(N):
                if step < len(all_actions[i]):
                    batch_actions[i] = all_actions[i][step]
                    active[i] = True

            # For done envs, just pass a dummy action (it will be skipped)
            obs, rewards, dones, infos = vec.step(batch_actions)

            for i in range(N):
                if active[i]:
                    expected_state = all_scalar_states[i][step + 1]
                    np.testing.assert_array_equal(
                        obs[i], expected_state,
                        err_msg=f"Env {i} state mismatch at step {step}"
                    )


# --- Win detection ---

class TestVecWin:
    def test_four_in_a_row(self):
        env = VecTicTacChecEnv(1)
        # Place 3 white pieces manually
        env.boards[0, 0, 0] = _piece_id(WHITE, PAWN)
        env.boards[0, 0, 1] = _piece_id(WHITE, ROOK)
        env.boards[0, 0, 2] = _piece_id(WHITE, BISHOP)
        # Drop knight at (0,3)
        action = KNIGHT * 16 + 0 * 4 + 3  # drop knight at (0,3)
        obs, rewards, dones, infos = env.step(np.array([action]))
        assert dones[0]
        assert rewards[0] == 1.0
        assert infos[0]["winner"] == WHITE

    def test_column_win(self):
        env = VecTicTacChecEnv(1)
        env.boards[0, 0, 0] = _piece_id(WHITE, PAWN)
        env.boards[0, 1, 0] = _piece_id(WHITE, ROOK)
        env.boards[0, 2, 0] = _piece_id(WHITE, BISHOP)
        action = KNIGHT * 16 + 3 * 4 + 0
        obs, rewards, dones, infos = env.step(np.array([action]))
        assert dones[0]
        assert rewards[0] == 1.0

    def test_diagonal_win(self):
        env = VecTicTacChecEnv(1)
        env.boards[0, 0, 0] = _piece_id(WHITE, PAWN)
        env.boards[0, 1, 1] = _piece_id(WHITE, ROOK)
        env.boards[0, 2, 2] = _piece_id(WHITE, BISHOP)
        action = KNIGHT * 16 + 3 * 4 + 3
        obs, rewards, dones, infos = env.step(np.array([action]))
        assert dones[0]

    def test_mixed_colors_no_win(self):
        env = VecTicTacChecEnv(1)
        env.boards[0, 0, 0] = _piece_id(WHITE, PAWN)
        env.boards[0, 0, 1] = _piece_id(BLACK, ROOK)
        env.boards[0, 0, 2] = _piece_id(WHITE, BISHOP)
        action = KNIGHT * 16 + 0 * 4 + 3
        obs, rewards, dones, infos = env.step(np.array([action]))
        assert not dones[0]


# --- Draw detection ---

class TestVecDraw:
    def test_draw_at_move_limit(self):
        env = VecTicTacChecEnv(1)
        env.move_counts[0] = 99
        env.boards[0, 0, 0] = _piece_id(WHITE, ROOK)
        # Move rook from (0,0) to (0,1)
        src = 0 * 4 + 0
        dst = 0 * 4 + 1
        action = 64 + src * 16 + dst
        obs, rewards, dones, infos = env.step(np.array([action]))
        assert dones[0]
        assert rewards[0] == 0.0
        assert infos[0].get("draw") == "move_limit"


# --- Independent games ---

class TestIndependentGames:
    def test_one_game_finishes_others_continue(self):
        """When one game ends, others should keep going."""
        env = VecTicTacChecEnv(2)

        # Game 0: set up instant win
        env.boards[0, 0, 0] = _piece_id(WHITE, PAWN)
        env.boards[0, 0, 1] = _piece_id(WHITE, ROOK)
        env.boards[0, 0, 2] = _piece_id(WHITE, BISHOP)

        # Game 1: just start normally
        # Both drop knight
        action0 = KNIGHT * 16 + 0 * 4 + 3  # game0: complete row
        action1 = PAWN * 16 + 2 * 4 + 2    # game1: drop pawn at (2,2)

        obs, rewards, dones, infos = env.step(np.array([action0, action1]))
        assert dones[0]  # game 0 finished
        assert not dones[1]  # game 1 continues

        # Subsequent step: game 0 should be skipped
        masks = env.legal_action_masks()
        assert np.all(masks[0] == False)  # no legal actions for done game
        assert np.any(masks[1])  # game 1 has legal actions


# --- Legal action mask specifics ---

class TestVecLegalMasks:
    def test_mask_blocks_occupied_drops(self):
        env = VecTicTacChecEnv(1)
        env.boards[0, 1, 1] = _piece_id(WHITE, ROOK)
        masks = env.legal_action_masks()
        # Cell (1,1) = cell index 5, should block drops there
        for kind_idx in range(4):
            # Rook is on board, so its drop is blocked entirely (it's not in hand)
            if kind_idx == ROOK:
                # Rook not in hand, ALL its drops should be blocked
                for cell in range(16):
                    assert not masks[0, kind_idx * 16 + cell]
            else:
                # Other pieces in hand, but can't drop on (1,1)
                action = kind_idx * 16 + 1 * 4 + 1
                assert not masks[0, action]

    def test_rook_moves_in_mask(self):
        env = VecTicTacChecEnv(1)
        env.boards[0, 2, 2] = _piece_id(WHITE, ROOK)
        masks = env.legal_action_masks()
        src = 2 * 4 + 2
        # Rook should reach these cells
        for dr, dc in [(2, 0), (2, 1), (2, 3), (0, 2), (1, 2), (3, 2)]:
            dst = dr * 4 + dc
            action = 64 + src * 16 + dst
            assert masks[0, action], f"Rook should reach ({dr},{dc})"
        # Should NOT reach diagonals
        for dr, dc in [(3, 3), (1, 1), (0, 0), (3, 1)]:
            dst = dr * 4 + dc
            action = 64 + src * 16 + dst
            assert not masks[0, action], f"Rook should NOT reach ({dr},{dc})"

    def test_knight_moves_in_mask(self):
        env = VecTicTacChecEnv(1)
        env.boards[0, 2, 2] = _piece_id(WHITE, KNIGHT)
        masks = env.legal_action_masks()
        src = 2 * 4 + 2
        expected = [(0, 1), (0, 3), (1, 0), (3, 0)]
        for dr, dc in expected:
            dst = dr * 4 + dc
            action = 64 + src * 16 + dst
            assert masks[0, action], f"Knight should reach ({dr},{dc})"

    def test_pawn_moves_in_mask(self):
        env = VecTicTacChecEnv(1)
        env.boards[0, 2, 1] = _piece_id(WHITE, PAWN)
        masks = env.legal_action_masks()
        src = 2 * 4 + 1
        # White pawn moves toward row 0 (direction -1)
        # Forward to (1,1)
        dst = 1 * 4 + 1
        assert masks[0, 64 + src * 16 + dst]
        # Should NOT go backward to (3,1)
        dst = 3 * 4 + 1
        assert not masks[0, 64 + src * 16 + dst]

    def test_pawn_diagonal_capture(self):
        env = VecTicTacChecEnv(1)
        env.boards[0, 2, 1] = _piece_id(WHITE, PAWN)
        env.boards[0, 1, 0] = _piece_id(BLACK, ROOK)
        env.boards[0, 1, 2] = _piece_id(BLACK, BISHOP)
        masks = env.legal_action_masks()
        src = 2 * 4 + 1
        # Capture left
        assert masks[0, 64 + src * 16 + (1 * 4 + 0)]
        # Capture right
        assert masks[0, 64 + src * 16 + (1 * 4 + 2)]

    def test_mask_agrees_with_scalar_env(self):
        """Verify mask equivalence with scalar env on various board states."""
        rng = np.random.RandomState(42)
        for trial in range(20):
            scalar = TicTacChecEnv()
            vec = VecTicTacChecEnv(1)

            # Play a few random moves
            num_moves = rng.randint(0, 15)
            for _ in range(num_moves):
                if scalar.done:
                    break
                mask = scalar.legal_action_mask()
                legal = np.where(mask)[0]
                action = rng.choice(legal)
                scalar.step(action)

                # Sync vec env to match scalar
            # After playing, sync the vec env state
            for r in range(4):
                for c in range(4):
                    p = scalar.board[r][c]
                    if p is None:
                        vec.boards[0, r, c] = 0
                    else:
                        vec.boards[0, r, c] = _piece_id(int(p.color), int(p.kind))
            vec.turns[0] = int(scalar.turn)
            vec.pawn_dirs[0, 0] = scalar.pawn_directions[Color.WHITE]
            vec.pawn_dirs[0, 1] = scalar.pawn_directions[Color.BLACK]
            vec.move_counts[0] = scalar.move_count
            vec.dones[0] = scalar.done

            if not scalar.done:
                scalar_mask = scalar.legal_action_mask()
                vec_mask = vec.legal_action_masks()[0]
                np.testing.assert_array_equal(
                    vec_mask, scalar_mask,
                    err_msg=f"Mask mismatch on trial {trial}"
                )


# --- Capture / pawn direction ---

class TestVecCapture:
    def test_capture_removes_piece(self):
        env = VecTicTacChecEnv(1)
        env.boards[0, 2, 2] = _piece_id(WHITE, ROOK)
        env.boards[0, 2, 0] = _piece_id(BLACK, PAWN)
        # Move rook from (2,2) to (2,0) to capture
        src = 2 * 4 + 2
        dst = 2 * 4 + 0
        action = 64 + src * 16 + dst
        env.step(np.array([action]))
        assert env.boards[0, 2, 0] == _piece_id(WHITE, ROOK)
        assert env.boards[0, 2, 2] == 0
        # Black pawn should now be in hand (not on board)
        assert not np.any(env.boards[0] == _piece_id(BLACK, PAWN))


class TestVecPawnDirection:
    def test_white_pawn_reverses_at_row_0(self):
        env = VecTicTacChecEnv(1)
        env.boards[0, 1, 0] = _piece_id(WHITE, PAWN)
        assert env.pawn_dirs[0, 0] == TO_BLACK_SIDE
        # Move pawn to row 0
        src = 1 * 4 + 0
        dst = 0 * 4 + 0
        action = 64 + src * 16 + dst
        env.step(np.array([action]))
        assert env.pawn_dirs[0, 0] == TO_WHITE_SIDE

    def test_pawn_resets_when_captured(self):
        env = VecTicTacChecEnv(1)
        env.boards[0, 0, 0] = _piece_id(WHITE, PAWN)
        env.pawn_dirs[0, 0] = TO_WHITE_SIDE  # reversed
        # Black rook captures white pawn
        env.boards[0, 0, 3] = _piece_id(BLACK, ROOK)
        env.boards[0, 3, 3] = _piece_id(WHITE, KNIGHT)  # so white can move
        # White drops bishop
        action = BISHOP * 16 + 1 * 4 + 1  # drop at (1,1)
        env.step(np.array([action]))
        # Now black captures: rook (0,3) -> (0,0)
        src = 0 * 4 + 3
        dst = 0 * 4 + 0
        action = 64 + src * 16 + dst
        env.step(np.array([action]))
        # White pawn captured -> direction resets
        assert env.pawn_dirs[0, 0] == TO_BLACK_SIDE


# --- Auto-reset ---

class TestAutoReset:
    def test_reset_specific_envs(self):
        env = VecTicTacChecEnv(3)
        # Play a step in all
        masks = env.legal_action_masks()
        actions = np.array([np.random.choice(np.where(masks[i])[0]) for i in range(3)])
        env.step(actions)

        # Reset only env 1
        env.reset(env_ids=np.array([1]))
        assert env.move_counts[1] == 0
        assert env.turns[1] == WHITE
        assert np.all(env.boards[1] == 0)
        # Env 0 and 2 should be untouched
        assert env.move_counts[0] == 1
        assert env.move_counts[2] == 1


# --- Full random game completion ---

class TestFullGame:
    def test_random_games_complete(self):
        """Run several random games to completion, verify they all end."""
        N = 16
        env = VecTicTacChecEnv(N)
        rng = np.random.RandomState(123)

        for _ in range(200):  # generous upper bound
            if np.all(env.dones):
                break
            masks = env.legal_action_masks()
            actions = np.zeros(N, dtype=np.int32)
            for i in range(N):
                if not env.dones[i]:
                    legal = np.where(masks[i])[0]
                    assert len(legal) > 0, f"No legal actions for env {i} but not done"
                    actions[i] = rng.choice(legal)
            env.step(actions)

        assert np.all(env.dones), "Not all games completed within 200 steps"
