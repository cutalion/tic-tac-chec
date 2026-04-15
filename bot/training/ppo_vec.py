"""PPO rollout collection using vectorized environment.

Drop-in replacement for collect_rollouts using VecTicTacChecEnv for batch
env stepping — all 128 games stepped in one call instead of a Python loop.
"""

import random as pyrandom

import numpy as np
import torch
import torch.nn.functional as F
from env_vec import VecTicTacChecEnv
from model import PPONet
from ppo import RolloutBuffer


def _batched_select(net, states_np, masks_np, device):
    """Batched forward pass + masked sampling."""
    states_t = torch.from_numpy(states_np).to(device)
    masks_t = torch.from_numpy(masks_np).to(device)

    with torch.no_grad():
        logits, values = net(states_t)

    logits[~masks_t] = float("-inf")
    probs = F.softmax(logits, dim=-1)
    dist = torch.distributions.Categorical(probs)
    actions_t = dist.sample()
    log_probs_t = dist.log_prob(actions_t)

    return actions_t.cpu().numpy(), log_probs_t.cpu().numpy(), values.squeeze(-1).cpu().numpy()


def collect_rollouts_vec(net, num_games, device="cpu", opponent_pool=None):
    """Collect rollouts using vectorized environment.

    All games run as batch numpy operations. Much faster than stepping
    individual Python env objects.
    """
    buffer = RolloutBuffer()
    env = VecTicTacChecEnv(num_games)
    observations = env.encode_states()

    # Track per-game, per-player transitions
    # Each game has two lists: [white_transitions, black_transitions]
    game_transitions = [([], []) for _ in range(num_games)]
    active_mask = np.ones(num_games, dtype=bool)

    # Opponent setup
    if opponent_pool:
        agent_colors = np.array([pyrandom.randint(0, 1) for _ in range(num_games)])
        opponents = [pyrandom.choice(opponent_pool) for _ in range(num_games)]
    else:
        agent_colors = None
        opponents = None

    while active_mask.any():
        active_ids = np.where(active_mask)[0]
        n_active = len(active_ids)

        # Get states and masks for active games
        states = observations[active_ids]
        masks = env.legal_action_masks()[active_ids]
        turns = env.turns[active_ids]

        # Agent forward pass
        actions, log_probs, vals = _batched_select(net, states, masks, device)

        # Opponent moves — batched by opponent identity
        if opponents:
            opp_groups = {}
            for idx_in_batch, game_idx in enumerate(active_ids):
                if int(turns[idx_in_batch]) != agent_colors[int(game_idx)]:
                    opp_id = id(opponents[int(game_idx)])
                    if opp_id not in opp_groups:
                        opp_groups[opp_id] = (opponents[int(game_idx)], [])
                    opp_groups[opp_id][1].append(idx_in_batch)

            for opp_net, indices in opp_groups.values():
                if indices:
                    opp_states = states[indices]
                    opp_masks = masks[indices]
                    opp_actions, _, _ = _batched_select(opp_net, opp_states, opp_masks, device)
                    for k, idx in enumerate(indices):
                        actions[idx] = opp_actions[k]

        # Store transitions before stepping
        for idx_in_batch, game_idx in enumerate(active_ids):
            game_idx = int(game_idx)
            action = int(actions[idx_in_batch])
            log_prob = float(log_probs[idx_in_batch])
            value = float(vals[idx_in_batch])
            current_player = int(turns[idx_in_batch])
            obs = states[idx_in_batch]
            mask = masks[idx_in_batch].copy()

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

        # Batch step all active games
        full_actions = np.zeros(num_games, dtype=np.int64)
        full_actions[active_ids] = actions

        next_obs, rewards, dones, infos = env.step(full_actions)
        observations = next_obs

        # Process finished games
        newly_done = active_ids[dones[active_ids].astype(bool)]
        for game_idx in newly_done:
            game_idx = int(game_idx)
            winner = infos[game_idx].get("winner")
            for transitions, color in [
                (game_transitions[game_idx][0], 0),
                (game_transitions[game_idx][1], 1),
            ]:
                for i, (state, act, lp, val, mask) in enumerate(transitions):
                    is_last = i == len(transitions) - 1
                    if is_last:
                        if winner is None:
                            r = 0.0
                        elif int(winner) == color:
                            r = 1.0
                        else:
                            r = -1.0
                    else:
                        r = 0.0
                    buffer.add(state, act, r, float(is_last), lp, val, mask)

            active_mask[game_idx] = False

    return buffer
