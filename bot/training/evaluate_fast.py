"""Fast batched evaluation: play games with batched inference."""

import random

import numpy as np
import torch
import torch.nn.functional as F

from env import TicTacChecEnv, Color
from model import PPONet


def _batched_select_greedy(net, states_batch, masks_batch, device):
    """Batched forward pass with greedy action selection (no sampling)."""
    states_t = torch.from_numpy(states_batch).to(device)
    masks_t = torch.from_numpy(masks_batch).to(device)

    with torch.no_grad():
        logits, _ = net(states_t)

    logits[~masks_t] = float("-inf")
    actions = logits.argmax(dim=-1).cpu().numpy()
    return actions


def _batched_select_sample(net, states_batch, masks_batch, device):
    """Batched forward pass with stochastic action selection."""
    states_t = torch.from_numpy(states_batch).to(device)
    masks_t = torch.from_numpy(masks_batch).to(device)

    with torch.no_grad():
        logits, _ = net(states_t)

    logits[~masks_t] = float("-inf")
    probs = F.softmax(logits, dim=-1)
    dist = torch.distributions.Categorical(probs)
    actions = dist.sample().cpu().numpy()
    return actions


def evaluate_vs_random(net, num_games=100, device="cpu"):
    """Batched evaluation against random opponent."""
    wins, draws, losses, total_length = 0, 0, 0, 0

    envs = [TicTacChecEnv() for _ in range(num_games)]
    observations = [env.encode_state() for env in envs]
    bot_colors = [Color.WHITE if i % 2 == 0 else Color.BLACK for i in range(num_games)]
    active = list(range(num_games))

    states_buf = np.empty((num_games, 19, 4, 4), dtype=np.float32)
    masks_buf = np.empty((num_games, 320), dtype=bool)

    while active:
        # Split active games into bot-turn and random-turn
        bot_indices = []
        random_indices = []
        for game_idx in active:
            if envs[game_idx].turn == bot_colors[game_idx]:
                bot_indices.append(game_idx)
            else:
                random_indices.append(game_idx)

        # Batched bot moves
        actions = {}
        if bot_indices:
            n = len(bot_indices)
            for i, game_idx in enumerate(bot_indices):
                states_buf[i] = observations[game_idx]
                masks_buf[i] = envs[game_idx].legal_action_mask()
            bot_actions = _batched_select_greedy(net, states_buf[:n], masks_buf[:n], device)
            for i, game_idx in enumerate(bot_indices):
                actions[game_idx] = int(bot_actions[i])

        # Random moves
        for game_idx in random_indices:
            legal = envs[game_idx].legal_actions()
            actions[game_idx] = random.choice(legal)

        # Step all active envs
        newly_done = []
        for game_idx in active:
            obs, _, done, info = envs[game_idx].step(actions[game_idx])
            observations[game_idx] = obs
            if done:
                newly_done.append(game_idx)
                total_length += envs[game_idx].move_count
                winner = info.get("winner")
                if winner is None:
                    draws += 1
                elif winner == bot_colors[game_idx]:
                    wins += 1
                else:
                    losses += 1

        if newly_done:
            active = [i for i in active if i not in newly_done]

    n = num_games
    return wins / n, draws / n, losses / n, total_length / n


def evaluate_vs_opponent(net, opponent, num_games=100, device="cpu"):
    """Batched evaluation against another model."""
    wins, draws, losses, total_length = 0, 0, 0, 0

    envs = [TicTacChecEnv() for _ in range(num_games)]
    observations = [env.encode_state() for env in envs]
    bot_colors = [Color.WHITE if i % 2 == 0 else Color.BLACK for i in range(num_games)]
    active = list(range(num_games))

    states_buf = np.empty((num_games, 19, 4, 4), dtype=np.float32)
    masks_buf = np.empty((num_games, 320), dtype=bool)

    while active:
        # Split by whose turn it is
        bot_indices = []
        opp_indices = []
        for game_idx in active:
            if envs[game_idx].turn == bot_colors[game_idx]:
                bot_indices.append(game_idx)
            else:
                opp_indices.append(game_idx)

        actions = {}

        # Batched bot moves (sampling for game diversity)
        if bot_indices:
            n = len(bot_indices)
            for i, game_idx in enumerate(bot_indices):
                states_buf[i] = observations[game_idx]
                masks_buf[i] = envs[game_idx].legal_action_mask()
            bot_actions = _batched_select_sample(net, states_buf[:n], masks_buf[:n], device)
            for i, game_idx in enumerate(bot_indices):
                actions[game_idx] = int(bot_actions[i])

        # Batched opponent moves (sampling for game diversity)
        if opp_indices:
            n = len(opp_indices)
            for i, game_idx in enumerate(opp_indices):
                states_buf[i] = observations[game_idx]
                masks_buf[i] = envs[game_idx].legal_action_mask()
            opp_actions = _batched_select_sample(opponent, states_buf[:n], masks_buf[:n], device)
            for i, game_idx in enumerate(opp_indices):
                actions[game_idx] = int(opp_actions[i])

        # Step all active envs
        newly_done = []
        for game_idx in active:
            obs, _, done, info = envs[game_idx].step(actions[game_idx])
            observations[game_idx] = obs
            if done:
                newly_done.append(game_idx)
                total_length += envs[game_idx].move_count
                winner = info.get("winner")
                if winner is None:
                    draws += 1
                elif winner == bot_colors[game_idx]:
                    wins += 1
                else:
                    losses += 1

        if newly_done:
            active = [i for i in active if i not in newly_done]

    n = num_games
    return wins / n, draws / n, losses / n, total_length / n
