"""
Vectorized Tic Tac Chec game environment for RL training.

Runs N games simultaneously using numpy array operations instead of Python loops.
Mirrors the semantics of env.py exactly, but represents all state as numpy arrays.

Board encoding (int8):
  0 = empty
  1-4 = White Pawn, Rook, Bishop, Knight
  5-8 = Black Pawn, Rook, Bishop, Knight

Piece index mapping:
  piece_id = color * 4 + kind + 1   (1-based, 0 = empty)
"""

import numpy as np

# --- Constants (matching env.py) ---
BOARD_SIZE = 4
MAX_MOVES = 100
REPETITION_LIMIT = 5

NUM_DROP_ACTIONS = 4 * BOARD_SIZE * BOARD_SIZE  # 64
NUM_MOVE_ACTIONS = (BOARD_SIZE * BOARD_SIZE) ** 2  # 256
ACTION_SPACE_SIZE = NUM_DROP_ACTIONS + NUM_MOVE_ACTIONS  # 320
NUM_CHANNELS = 19

WHITE = 0
BLACK = 1

PAWN = 0
ROOK = 1
BISHOP = 2
KNIGHT = 3

TO_BLACK_SIDE = -1
TO_WHITE_SIDE = 1


def _piece_id(color, kind):
    """Map (color, kind) to board cell value (1-8)."""
    return color * 4 + kind + 1


def _id_to_color(pid):
    """Board cell value -> color (0 or 1)."""
    return (pid - 1) // 4


def _id_to_kind(pid):
    """Board cell value -> kind (0-3)."""
    return (pid - 1) % 4


# Precompute piece IDs
WHITE_PAWN = _piece_id(WHITE, PAWN)    # 1
WHITE_ROOK = _piece_id(WHITE, ROOK)    # 2
WHITE_BISHOP = _piece_id(WHITE, BISHOP)  # 3
WHITE_KNIGHT = _piece_id(WHITE, KNIGHT)  # 4
BLACK_PAWN = _piece_id(BLACK, PAWN)    # 5
BLACK_ROOK = _piece_id(BLACK, ROOK)    # 6
BLACK_BISHOP = _piece_id(BLACK, BISHOP)  # 7
BLACK_KNIGHT = _piece_id(BLACK, KNIGHT)  # 8

# All 8 piece IDs in the same order as ALL_PIECES in env.py
ALL_PIECE_IDS = np.array([WHITE_PAWN, WHITE_ROOK, WHITE_BISHOP, WHITE_KNIGHT,
                          BLACK_PAWN, BLACK_ROOK, BLACK_BISHOP, BLACK_KNIGHT], dtype=np.int8)

# 10 winning lines: 4 rows + 4 cols + 2 diags
# Store as flat cell indices for fast batch lookup
_WIN_LINES = np.zeros((10, 4), dtype=np.intp)
for _i in range(4):
    _WIN_LINES[_i] = [_i * 4 + c for c in range(4)]       # rows
    _WIN_LINES[4 + _i] = [r * 4 + _i for r in range(4)]   # cols
_WIN_LINES[8] = [0, 5, 10, 15]   # main diag
_WIN_LINES[9] = [3, 6, 9, 12]    # anti-diag


def _precompute_slide_reachable():
    """Precompute for rook and bishop: for each (src, dst) pair, the cells
    that must be empty for dst to be reachable (the "between" cells along the ray).

    Returns:
        rook_between: (16, 16) list-of-lists, between[src][dst] = list of cells between
        rook_reachable: (16, 16) bool, True if dst is on a rook ray from src
        bishop_between, bishop_reachable: same for bishop
    """
    def compute_rays(directions):
        rays = {}
        for src in range(16):
            sr, sc = src // 4, src % 4
            rays[src] = []
            for dr, dc in directions:
                ray = []
                r, c = sr, sc
                for _ in range(3):
                    r += dr
                    c += dc
                    if 0 <= r < 4 and 0 <= c < 4:
                        ray.append(r * 4 + c)
                    else:
                        break
                if ray:
                    rays[src].append(ray)
        return rays

    rook_dirs = [(0, 1), (0, -1), (1, 0), (-1, 0)]
    bishop_dirs = [(-1, -1), (-1, 1), (1, -1), (1, 1)]

    rook_rays = compute_rays(rook_dirs)
    bishop_rays = compute_rays(bishop_dirs)

    return rook_rays, bishop_rays


_ROOK_RAYS, _BISHOP_RAYS = _precompute_slide_reachable()


def _precompute_knight_table():
    knight_offsets = [(-2, -1), (-2, 1), (2, -1), (2, 1),
                      (-1, -2), (-1, 2), (1, -2), (1, 2)]
    table = np.zeros((16, 16), dtype=bool)
    for src in range(16):
        sr, sc = src // 4, src % 4
        for dr, dc in knight_offsets:
            nr, nc = sr + dr, sc + dc
            if 0 <= nr < 4 and 0 <= nc < 4:
                table[src, nr * 4 + nc] = True
    return table


_KNIGHT_TABLE = _precompute_knight_table()


def _precompute_pawn_tables():
    """Precompute pawn move tables for both directions.

    Returns:
        pawn_forward: dict[direction] -> (16,) int, target cell for forward move (-1 if off board)
        pawn_captures: dict[direction] -> (16, 2) int, target cells for diagonal captures (-1 if off board)
    """
    pawn_forward = {}
    pawn_captures = {}
    for direction in [TO_BLACK_SIDE, TO_WHITE_SIDE]:
        fwd = np.full(16, -1, dtype=np.int8)
        cap = np.full((16, 2), -1, dtype=np.int8)
        for src in range(16):
            sr, sc = src // 4, src % 4
            nr = sr + direction
            if 0 <= nr < 4:
                fwd[src] = nr * 4 + sc
                for j, dc in enumerate([-1, 1]):
                    nc = sc + dc
                    if 0 <= nc < 4:
                        cap[src, j] = nr * 4 + nc
        pawn_forward[direction] = fwd
        pawn_captures[direction] = cap
    return pawn_forward, pawn_captures


_PAWN_FORWARD, _PAWN_CAPTURES = _precompute_pawn_tables()


class VecTicTacChecEnv:
    """Vectorized Tic Tac Chec environment running N games in parallel."""

    def __init__(self, num_envs: int):
        self.num_envs = num_envs
        self.boards = np.zeros((num_envs, 4, 4), dtype=np.int8)
        self.turns = np.zeros(num_envs, dtype=np.int8)
        self.pawn_dirs = np.zeros((num_envs, 2), dtype=np.int8)
        self.move_counts = np.zeros(num_envs, dtype=np.int32)
        self.dones = np.zeros(num_envs, dtype=bool)
        self.winners = np.full(num_envs, -1, dtype=np.int8)
        self.draws = np.zeros(num_envs, dtype=bool)
        self._state_histories = [dict() for _ in range(num_envs)]
        self.reset()

    def reset(self, env_ids=None):
        """Reset specified environments (or all). Returns observations (N, 19, 4, 4)."""
        if env_ids is None:
            env_ids = np.arange(self.num_envs)
        elif isinstance(env_ids, (int, np.integer)):
            env_ids = np.array([env_ids])

        self.boards[env_ids] = 0
        self.turns[env_ids] = WHITE
        self.pawn_dirs[env_ids, 0] = TO_BLACK_SIDE
        self.pawn_dirs[env_ids, 1] = TO_WHITE_SIDE
        self.move_counts[env_ids] = 0
        self.dones[env_ids] = False
        self.winners[env_ids] = -1
        self.draws[env_ids] = False
        for i in env_ids:
            self._state_histories[int(i)] = {}
            self._record_state(int(i))

        return self.encode_states()

    # ------------------------------------------------------------------ #
    # encode_states  (batch)
    # ------------------------------------------------------------------ #
    def encode_states(self):
        """Encode all game states as tensor (N, 19, 4, 4) float32."""
        N = self.num_envs
        boards_flat = self.boards.reshape(N, 16)
        state = np.zeros((N, NUM_CHANNELS, 4, 4), dtype=np.float32)

        # Channels 0-7: one-hot piece positions
        for i, pid in enumerate(ALL_PIECE_IDS):
            state[:, i] = (self.boards == pid).astype(np.float32)

        # Channels 8-15: pieces in hand (broadcast scalar per env)
        for i, pid in enumerate(ALL_PIECE_IDS):
            on_board = np.any(boards_flat == pid, axis=1)  # (N,)
            state[:, 8 + i] = (~on_board).astype(np.float32)[:, None, None]

        # Channels 16-18: turn and pawn directions
        state[:, 16] = (self.turns == WHITE).astype(np.float32)[:, None, None]
        state[:, 17] = (self.pawn_dirs[:, 0] == TO_BLACK_SIDE).astype(np.float32)[:, None, None]
        state[:, 18] = (self.pawn_dirs[:, 1] == TO_BLACK_SIDE).astype(np.float32)[:, None, None]

        return state

    # ------------------------------------------------------------------ #
    # legal_action_masks  (per-env loop, optimized inner loops)
    # ------------------------------------------------------------------ #
    def legal_action_masks(self):
        """Compute legal action masks for all envs. Returns (N, 320) bool."""
        N = self.num_envs
        masks = np.zeros((N, ACTION_SPACE_SIZE), dtype=bool)
        boards_flat = self.boards.reshape(N, 16)
        # Preload arrays to avoid repeated attribute lookups
        dones = self.dones
        turns = self.turns
        pawn_dirs = self.pawn_dirs
        pawn_fwd_neg = _PAWN_FORWARD[TO_BLACK_SIDE]
        pawn_fwd_pos = _PAWN_FORWARD[TO_WHITE_SIDE]
        pawn_cap_neg = _PAWN_CAPTURES[TO_BLACK_SIDE]
        pawn_cap_pos = _PAWN_CAPTURES[TO_WHITE_SIDE]

        for ei in range(N):
            if dones[ei]:
                continue
            turn = int(turns[ei])
            board = boards_flat[ei]  # (16,) int8 view
            mask = masks[ei]  # (320,) bool view

            base_pid = turn * 4 + 1
            enemy_lo = (1 - turn) * 4 + 1
            enemy_hi = enemy_lo + 4

            # Find empty cells and own piece locations using plain iteration
            # (avoids numpy overhead for 16 elements)
            empty_cells = []
            own_cells = []
            in_hand = [True, True, True, True]  # kind 0-3
            for cell in range(16):
                v = int(board[cell])
                if v == 0:
                    empty_cells.append(cell)
                elif base_pid <= v < base_pid + 4:
                    own_cells.append(cell)
                    in_hand[v - base_pid] = False

            # --- Drop actions ---
            for kind in range(4):
                if in_hand[kind]:
                    base = kind * 16
                    for cell in empty_cells:
                        mask[base + cell] = True

            # --- Move actions ---
            if not own_cells:
                continue

            pawn_dir = int(pawn_dirs[ei, turn])
            pawn_fwd = pawn_fwd_neg if pawn_dir == TO_BLACK_SIDE else pawn_fwd_pos
            pawn_cap = pawn_cap_neg if pawn_dir == TO_BLACK_SIDE else pawn_cap_pos

            for cell in own_cells:
                v = int(board[cell])
                kind = (v - 1) % 4
                action_base = 64 + cell * 16  # NUM_DROP_ACTIONS=64

                if kind == 3:  # KNIGHT
                    kt_row = _KNIGHT_TABLE[cell]
                    for dst in range(16):
                        if kt_row[dst]:
                            t = int(board[dst])
                            if t == 0 or (enemy_lo <= t < enemy_hi):
                                mask[action_base + dst] = True

                elif kind == 1:  # ROOK
                    for ray in _ROOK_RAYS[cell]:
                        for dst in ray:
                            t = int(board[dst])
                            if t == 0:
                                mask[action_base + dst] = True
                            elif enemy_lo <= t < enemy_hi:
                                mask[action_base + dst] = True
                                break
                            else:
                                break  # own piece

                elif kind == 2:  # BISHOP
                    for ray in _BISHOP_RAYS[cell]:
                        for dst in ray:
                            t = int(board[dst])
                            if t == 0:
                                mask[action_base + dst] = True
                            elif enemy_lo <= t < enemy_hi:
                                mask[action_base + dst] = True
                                break
                            else:
                                break

                elif kind == 0:  # PAWN
                    fwd = int(pawn_fwd[cell])
                    if fwd >= 0 and board[fwd] == 0:
                        mask[action_base + fwd] = True
                    c0 = int(pawn_cap[cell, 0])
                    if c0 >= 0:
                        t = int(board[c0])
                        if enemy_lo <= t < enemy_hi:
                            mask[action_base + c0] = True
                    c1 = int(pawn_cap[cell, 1])
                    if c1 >= 0:
                        t = int(board[c1])
                        if enemy_lo <= t < enemy_hi:
                            mask[action_base + c1] = True

        return masks

    # ------------------------------------------------------------------ #
    # step  (batch with vectorized parts)
    # ------------------------------------------------------------------ #
    def step(self, actions):
        """Execute actions for all envs. Returns (obs, rewards, dones, infos)."""
        N = self.num_envs
        actions = np.asarray(actions, dtype=np.int32)
        rewards = np.zeros(N, dtype=np.float32)
        infos = [None] * N

        active = ~self.dones  # (N,) bool

        if not np.any(active):
            for i in range(N):
                infos[i] = {"move_count": int(self.move_counts[i])}
            return self.encode_states(), rewards, self.dones.copy(), infos

        # Split into drops and moves
        is_drop = actions < NUM_DROP_ACTIONS
        is_move = ~is_drop

        boards_flat = self.boards.reshape(N, 16)

        # --- Apply drop actions (vectorized) ---
        drop_active = active & is_drop
        if np.any(drop_active):
            da = np.flatnonzero(drop_active)
            drop_acts = actions[da]
            kinds = drop_acts // 16
            cells = drop_acts % 16
            pids = self.turns[da] * 4 + kinds + 1
            boards_flat[da, cells] = pids.astype(np.int8)

        # --- Apply move actions (vectorized) ---
        move_active = active & is_move
        if np.any(move_active):
            ma = np.flatnonzero(move_active)
            move_acts = actions[ma] - NUM_DROP_ACTIONS
            srcs = move_acts // 16
            dsts = move_acts % 16
            # Copy pieces
            pieces = boards_flat[ma, srcs]
            boards_flat[ma, dsts] = pieces
            boards_flat[ma, srcs] = 0

        # Increment move counts for active envs
        self.move_counts[active] += 1

        # --- Check wins (vectorized) ---
        # For each active env, check if current player has 4 in a line
        active_idx = np.flatnonzero(active)
        turns_active = self.turns[active_idx]

        # Get cells along all win lines for active envs: (n_active, 10, 4)
        line_cells = boards_flat[active_idx][:, _WIN_LINES]  # (n_active, 10, 4)

        # A cell belongs to current player if pid > 0 and color matches
        # color = (pid - 1) // 4, so for WHITE (0): pid in 1-4, for BLACK (1): pid in 5-8
        # Quick check: (pid - 1) // 4 == turn  <=>  pid in [turn*4+1, turn*4+4]
        low = turns_active * 4 + 1  # (n_active,)
        high = low + 4
        # Broadcast: line_cells (n_active, 10, 4) vs low/high (n_active, 1, 1)
        is_own = (line_cells >= low[:, None, None]) & (line_cells < high[:, None, None])
        line_wins = np.all(is_own, axis=2)  # (n_active, 10)
        has_win = np.any(line_wins, axis=1)  # (n_active,)

        win_env_idx = active_idx[has_win]
        if len(win_env_idx) > 0:
            self.winners[win_env_idx] = self.turns[win_env_idx]
            self.dones[win_env_idx] = True
            rewards[win_env_idx] = 1.0

        # --- Post-win: process non-winning active envs ---
        still_active = active & ~self.dones  # envs that didn't just win
        still_idx = np.flatnonzero(still_active)

        if len(still_idx) > 0:
            # Switch turns (vectorized)
            self.turns[still_idx] = 1 - self.turns[still_idx]

            # Update pawn directions (batch)
            self._batch_reverse_pawn_dirs(still_idx)

            # Record state for repetition
            for i in still_idx:
                self._record_state(i)

            # Check draw: move limit (vectorized)
            move_limit = still_active & (self.move_counts >= MAX_MOVES)
            ml_idx = np.flatnonzero(move_limit)
            if len(ml_idx) > 0:
                self.dones[ml_idx] = True
                self.draws[ml_idx] = True

            # Check draw: repetition (per-env for remaining)
            still2 = still_active & ~self.dones
            for i in np.flatnonzero(still2):
                if self._check_repetition(i):
                    self.dones[i] = True
                    self.draws[i] = True

        # Build infos
        for i in range(N):
            if not active[i]:
                infos[i] = {"move_count": int(self.move_counts[i])}
            elif self.dones[i] and self.winners[i] >= 0:
                infos[i] = {"winner": int(self.winners[i]),
                            "move_count": int(self.move_counts[i])}
            elif self.dones[i] and self.draws[i]:
                mc = int(self.move_counts[i])
                if self.move_counts[i] >= MAX_MOVES:
                    infos[i] = {"draw": "move_limit", "move_count": mc}
                else:
                    infos[i] = {"draw": "repetition", "move_count": mc}
            else:
                infos[i] = {"move_count": int(self.move_counts[i])}

        observations = self.encode_states()
        return observations, rewards, self.dones.copy(), infos

    # ------------------------------------------------------------------ #
    # Internal helpers
    # ------------------------------------------------------------------ #
    def _batch_reverse_pawn_dirs(self, env_ids):
        """Batch update pawn directions for given envs, both colors."""
        boards = self.boards[env_ids]  # (M, 4, 4)
        pawn_dirs = self.pawn_dirs  # (N, 2)

        for color in (WHITE, BLACK):
            pid = color * 4 + 1  # pawn piece id
            initial_dir = TO_BLACK_SIDE if color == WHITE else TO_WHITE_SIDE

            # Check each row for pawn presence: (M, 4) bool per row
            # boards shape (M, 4, 4), check if any cell in each row equals pid
            has_pawn_in_row = np.any(boards == pid, axis=2)  # (M, 4)

            # Find which envs have the pawn on board at all
            on_board = np.any(has_pawn_in_row, axis=1)  # (M,)

            # For envs where pawn is NOT on board: reset direction
            off_board_idx = env_ids[~on_board]
            if len(off_board_idx) > 0:
                pawn_dirs[off_board_idx, color] = initial_dir

            # For envs where pawn IS on board: find the row
            on_idx = np.flatnonzero(on_board)
            if len(on_idx) == 0:
                continue

            # argmax on has_pawn_in_row gives first True row
            pawn_rows = np.argmax(has_pawn_in_row[on_idx], axis=1)  # (k,)
            abs_idx = env_ids[on_idx]
            cur_dirs = pawn_dirs[abs_idx, color]

            # Reverse if at far edge
            rev_to_pos = (cur_dirs == TO_BLACK_SIDE) & (pawn_rows == 0)
            rev_to_neg = (cur_dirs == TO_WHITE_SIDE) & (pawn_rows == 3)

            if np.any(rev_to_pos):
                pawn_dirs[abs_idx[rev_to_pos], color] = TO_WHITE_SIDE
            if np.any(rev_to_neg):
                pawn_dirs[abs_idx[rev_to_neg], color] = TO_BLACK_SIDE

    def _maybe_reverse_pawn_dir(self, env_idx, color):
        """Reverse pawn direction if at far edge, reset if not on board.

        Uses direct element access (board is a small 4x4 numpy array viewed
        as int8, indexed with Python ints -- faster than any/argwhere for
        such a tiny array).
        """
        pid = color * 4 + 1  # _piece_id(color, PAWN) inlined
        # Unrolled scan of 4x4 board (16 element checks)
        b = self.boards[env_idx]
        # Check each row for the pawn (rows 0..3)
        # We only need the row, not exact column.
        r0 = b[0]
        if r0[0] == pid or r0[1] == pid or r0[2] == pid or r0[3] == pid:
            row = 0
        else:
            r1 = b[1]
            if r1[0] == pid or r1[1] == pid or r1[2] == pid or r1[3] == pid:
                row = 1
            else:
                r2 = b[2]
                if r2[0] == pid or r2[1] == pid or r2[2] == pid or r2[3] == pid:
                    row = 2
                else:
                    r3 = b[3]
                    if r3[0] == pid or r3[1] == pid or r3[2] == pid or r3[3] == pid:
                        row = 3
                    else:
                        # Not on board: reset
                        self.pawn_dirs[env_idx, color] = TO_BLACK_SIDE if color == WHITE else TO_WHITE_SIDE
                        return

        d = self.pawn_dirs[env_idx, color]
        if d == TO_BLACK_SIDE and row == 0:
            self.pawn_dirs[env_idx, color] = TO_WHITE_SIDE
        elif d == TO_WHITE_SIDE and row == 3:
            self.pawn_dirs[env_idx, color] = TO_BLACK_SIDE

    def _state_hash(self, env_idx):
        """Compute state hash for repetition detection.

        Uses board bytes + turn as hash. Hand pieces are fully determined
        by what's on the board, so board_bytes encodes everything needed
        except the turn.
        """
        # board bytes uniquely determine which pieces are on board (and thus in hand)
        # We just need to add the turn.
        return (self.boards[env_idx].tobytes(), int(self.turns[env_idx]))

    def _record_state(self, env_idx):
        h = self._state_hash(env_idx)
        hist = self._state_histories[env_idx]
        hist[h] = hist.get(h, 0) + 1

    def _check_repetition(self, env_idx):
        h = self._state_hash(env_idx)
        return self._state_histories[env_idx].get(h, 0) >= REPETITION_LIMIT

    def _check_win(self, env_idx, color):
        """Single-env win check (used only in rare fallback paths)."""
        board_flat = self.boards[env_idx].ravel()
        low = color * 4 + 1
        high = low + 4
        for line in _WIN_LINES:
            if all(low <= board_flat[c] < high for c in line):
                return True
        return False

    def get_single_env_mask(self, env_idx):
        """Get legal action mask for a single env."""
        return self.legal_action_masks()[env_idx]
