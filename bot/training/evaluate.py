"""Evaluation: play games against a random-move opponent."""

import random

import numpy as np
import torch

from env import TicTacChecEnv, Color
from model import PPONet
from ppo import select_action


def evaluate_vs_random(net: PPONet, num_games: int = 100, device: str = "cpu"):
    """Play num_games against a random opponent. Bot plays White in half, Black in half.

    Returns (win_rate, draw_rate, loss_rate, avg_game_length).
    """
    wins = 0
    draws = 0
    losses = 0
    total_length = 0

    for game_idx in range(num_games):
        env = TicTacChecEnv()
        obs = env.encode_state()

        # Alternate sides: bot plays White in even games, Black in odd
        bot_color = Color.WHITE if game_idx % 2 == 0 else Color.BLACK

        done = False
        while not done:
            if env.turn == bot_color:
                # Bot move: use network
                mask = env.legal_action_mask()
                action, _, _ = select_action(net, obs, mask, device)
            else:
                # Random opponent: pick uniformly from legal actions
                legal = env.legal_actions()
                action = random.choice(legal)

            obs, reward, done, info = env.step(action)

        total_length += env.move_count

        winner = info.get("winner")
        if winner is None:
            draws += 1
        elif winner == bot_color:
            wins += 1
        else:
            losses += 1

    n = num_games
    return wins / n, draws / n, losses / n, total_length / n


if __name__ == "__main__":
    net = PPONet()

    # Optionally load a checkpoint
    import sys
    if len(sys.argv) > 1:
        checkpoint = torch.load(sys.argv[1], map_location="cpu")
        net.load_state_dict(checkpoint["model_state_dict"])
        print(f"Loaded checkpoint: {sys.argv[1]}")

    win_rate, draw_rate, loss_rate, avg_len = evaluate_vs_random(net, num_games=100)
    print(f"Win:  {win_rate:.1%}")
    print(f"Draw: {draw_rate:.1%}")
    print(f"Loss: {loss_rate:.1%}")
    print(f"Avg game length: {avg_len:.0f}")
