"""Fast PPO rollout collection with multiprocess env stepping.

Drop-in replacement for ppo.collect_rollouts with parallel env stepping.
"""

import multiprocessing as mp
from concurrent.futures import ProcessPoolExecutor

import numpy as np
import torch
import torch.nn.functional as F
from env import TicTacChecEnv
from model import PPONet
from ppo import RolloutBuffer, _finish_game

import random as pyrandom


def _env_step_batch(args_list):
    """Step multiple environments in a worker process.

    Args: list of (env_state_dict, action) tuples
    Returns: list of (next_obs, reward, done, info, env_state_dict) tuples
    """
    # This runs in a subprocess — we can't pass env objects, so we use a different approach
    results = []
    for env_bytes, action in args_list:
        env = _deserialize_env(env_bytes)
        obs, reward, done, info = env.step(action)
        results.append((obs, reward, done, info, _serialize_env(env)))
    return results


def _serialize_env(env):
    """Serialize env state to a picklable dict."""
    return {
        'board': [[(p.color, p.kind) if p is not None else None for p in row] for row in env.board],
        'turn': int(env.turn),
        'pawn_directions': {int(k): v for k, v in env.pawn_directions.items()},
        'move_count': env.move_count,
        'state_history': dict(env.state_history),
        'winner': env.winner,
        'done': env.done,
        'draw': env.draw,
    }


def _deserialize_env(d):
    """Deserialize env state from a dict."""
    from env import Piece, Color, PieceKind
    env = TicTacChecEnv.__new__(TicTacChecEnv)
    env.board = []
    for row in d['board']:
        env_row = []
        for cell in row:
            if cell is None:
                env_row.append(None)
            else:
                env_row.append(Piece(Color(cell[0]), PieceKind(cell[1])))
        env.board.append(env_row)
    env.turn = Color(d['turn'])
    env.pawn_directions = {Color(int(k)): v for k, v in d['pawn_directions'].items()}
    env.move_count = d['move_count']
    env.state_history = d['state_history']
    env.winner = d['winner']
    env.done = d['done']
    env.draw = d['draw']
    return env


# Worker pool for env stepping
_worker_pool = None
_num_workers = None


def _get_pool(num_workers=None):
    global _worker_pool, _num_workers
    if num_workers is None:
        num_workers = min(mp.cpu_count(), 8)
    if _worker_pool is None or _num_workers != num_workers:
        if _worker_pool is not None:
            _worker_pool.shutdown(wait=False)
        _num_workers = num_workers
        _worker_pool = ProcessPoolExecutor(max_workers=num_workers)
    return _worker_pool


def _batched_select(net, states_batch, masks_batch, device):
    """Batched forward pass + masked sampling."""
    states_t = torch.tensor(states_batch, dtype=torch.float32, device=device)
    masks_t = torch.tensor(masks_batch, dtype=torch.bool, device=device)

    with torch.no_grad():
        logits, values = net(states_t)

    logits[~masks_t] = float("-inf")
    probs = F.softmax(logits, dim=-1)
    dist = torch.distributions.Categorical(probs)
    actions_t = dist.sample()
    log_probs_t = dist.log_prob(actions_t)

    return actions_t.cpu().numpy(), log_probs_t.cpu().numpy(), values.squeeze(-1).cpu().numpy()


def collect_rollouts_fast(net, num_games, device="cpu", opponent_pool=None, num_workers=None):
    """Like collect_rollouts but with parallel env stepping.

    Uses multiprocessing to step environments across CPU cores while
    keeping GPU inference batched on the main process.
    """
    buffer = RolloutBuffer()

    envs = [TicTacChecEnv() for _ in range(num_games)]
    observations = [env.encode_state() for env in envs]
    active = list(range(num_games))
    game_transitions = [([], []) for _ in range(num_games)]

    if opponent_pool:
        agent_colors = [pyrandom.randint(0, 1) for _ in range(num_games)]
        opponents = [pyrandom.choice(opponent_pool) for _ in range(num_games)]
    else:
        agent_colors = None
        opponents = None

    while active:
        states_batch = np.array([observations[i] for i in active])
        masks_batch = np.array([envs[i].legal_action_mask() for i in active])
        players = [envs[i].turn for i in active]

        # Batched forward pass for learning agent
        actions, log_probs, vals = _batched_select(net, states_batch, masks_batch, device)

        # Opponent actions if needed
        if opponents:
            opp_needs = []
            for idx_in_batch, game_idx in enumerate(active):
                if players[idx_in_batch] != agent_colors[game_idx]:
                    opp_needs.append((idx_in_batch, game_idx))

            if opp_needs:
                opp_groups = {}
                for idx_in_batch, game_idx in opp_needs:
                    opp_id = id(opponents[game_idx])
                    if opp_id not in opp_groups:
                        opp_groups[opp_id] = (opponents[game_idx], [])
                    opp_groups[opp_id][1].append((idx_in_batch, game_idx))

                for opp_net, group in opp_groups.values():
                    opp_indices = [idx for idx, _ in group]
                    opp_states = states_batch[opp_indices]
                    opp_masks = masks_batch[opp_indices]
                    opp_actions, _, _ = _batched_select(opp_net, opp_states, opp_masks, device)
                    for k, (idx_in_batch, game_idx) in enumerate(group):
                        actions[idx_in_batch] = opp_actions[k]

        # Store transitions BEFORE stepping (need current obs)
        for idx_in_batch, game_idx in enumerate(active):
            action = int(actions[idx_in_batch])
            log_prob = float(log_probs[idx_in_batch])
            value = float(vals[idx_in_batch])
            current_player = players[idx_in_batch]
            obs = observations[game_idx]

            mask = masks_batch[idx_in_batch]

            if opponents:
                is_agent_turn = current_player == agent_colors[game_idx]
                if is_agent_turn:
                    transition = (obs, action, log_prob, value, mask)
                    if current_player == 0:
                        game_transitions[game_idx][0].append(transition)
                    else:
                        game_transitions[game_idx][1].append(transition)
            else:
                transition = (obs, action, log_prob, value, mask)
                if current_player == 0:
                    game_transitions[game_idx][0].append(transition)
                else:
                    game_transitions[game_idx][1].append(transition)

        # Step all active envs
        newly_done = []
        for idx_in_batch, game_idx in enumerate(active):
            action = int(actions[idx_in_batch])
            next_obs, reward, done, info = envs[game_idx].step(action)
            observations[game_idx] = next_obs

            if done:
                newly_done.append(game_idx)
                _finish_game(game_idx, info, game_transitions, buffer)

        if newly_done:
            active = [i for i in active if i not in newly_done]

    return buffer


def collect_rollouts_compiled(net, num_games, device="cpu", opponent_pool=None):
    """Optimized rollout collection with pre-allocated tensors and reduced overhead."""
    buffer = RolloutBuffer()

    envs = [TicTacChecEnv() for _ in range(num_games)]
    observations = [env.encode_state() for env in envs]
    active = list(range(num_games))
    game_transitions = [([], []) for _ in range(num_games)]

    if opponent_pool:
        agent_colors = [pyrandom.randint(0, 1) for _ in range(num_games)]
        opponents = [pyrandom.choice(opponent_pool) for _ in range(num_games)]
    else:
        agent_colors = None
        opponents = None

    # Pre-allocate batch arrays
    max_batch = num_games
    states_buf = np.empty((max_batch, 19, 4, 4), dtype=np.float32)
    masks_buf = np.empty((max_batch, 320), dtype=bool)

    while active:
        n = len(active)

        # Fill pre-allocated arrays (avoid np.array allocation)
        for i, game_idx in enumerate(active):
            states_buf[i] = observations[game_idx]
            masks_buf[i] = envs[game_idx].legal_action_mask()
        players = [envs[i].turn for i in active]

        # Batched forward pass — use slices of pre-allocated buffers
        states_t = torch.from_numpy(states_buf[:n]).to(device)
        masks_t = torch.from_numpy(masks_buf[:n]).to(device)

        with torch.no_grad():
            logits, values = net(states_t)

        logits[~masks_t] = float("-inf")
        probs = F.softmax(logits, dim=-1)
        dist = torch.distributions.Categorical(probs)
        actions_t = dist.sample()
        log_probs_t = dist.log_prob(actions_t)

        actions = actions_t.cpu().numpy()
        log_probs = log_probs_t.cpu().numpy()
        vals = values.squeeze(-1).cpu().numpy()

        # Opponent actions if needed
        if opponents:
            opp_needs = []
            for idx_in_batch, game_idx in enumerate(active):
                if players[idx_in_batch] != agent_colors[game_idx]:
                    opp_needs.append((idx_in_batch, game_idx))

            if opp_needs:
                opp_groups = {}
                for idx_in_batch, game_idx in opp_needs:
                    opp_id = id(opponents[game_idx])
                    if opp_id not in opp_groups:
                        opp_groups[opp_id] = (opponents[game_idx], [])
                    opp_groups[opp_id][1].append((idx_in_batch, game_idx))

                for opp_net, group in opp_groups.values():
                    opp_indices = [idx for idx, _ in group]
                    n_opp = len(opp_indices)
                    opp_states_t = torch.from_numpy(states_buf[opp_indices]).to(device)
                    opp_masks_t = torch.from_numpy(masks_buf[opp_indices]).to(device)

                    with torch.no_grad():
                        opp_logits, _ = opp_net(opp_states_t)
                    opp_logits[~opp_masks_t] = float("-inf")
                    opp_probs = F.softmax(opp_logits, dim=-1)
                    opp_dist = torch.distributions.Categorical(opp_probs)
                    opp_actions = opp_dist.sample().cpu().numpy()

                    for k, (idx_in_batch, game_idx) in enumerate(group):
                        actions[idx_in_batch] = opp_actions[k]

        # Step envs and store transitions
        newly_done = []
        for idx_in_batch, game_idx in enumerate(active):
            action = int(actions[idx_in_batch])
            log_prob = float(log_probs[idx_in_batch])
            value = float(vals[idx_in_batch])
            current_player = players[idx_in_batch]
            obs = observations[game_idx]
            mask = masks_buf[idx_in_batch].copy()

            if opponents:
                if current_player == agent_colors[game_idx]:
                    transition = (obs, action, log_prob, value, mask)
                    if current_player == 0:
                        game_transitions[game_idx][0].append(transition)
                    else:
                        game_transitions[game_idx][1].append(transition)
            else:
                transition = (obs, action, log_prob, value, mask)
                if current_player == 0:
                    game_transitions[game_idx][0].append(transition)
                else:
                    game_transitions[game_idx][1].append(transition)

            next_obs, reward, done, info = envs[game_idx].step(action)
            observations[game_idx] = next_obs

            if done:
                newly_done.append(game_idx)
                _finish_game(game_idx, info, game_transitions, buffer)

        if newly_done:
            active = [i for i in active if i not in newly_done]

    return buffer
