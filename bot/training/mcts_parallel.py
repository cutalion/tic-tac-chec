"""Parallel MCTS self-play using multiprocessing.

Each worker process gets a CPU copy of the model and runs games independently.
The model is tiny (377K params), so CPU inference is fast enough — the bottleneck
is env stepping/cloning, which parallelizes perfectly across cores.
"""

import multiprocessing as mp
from functools import partial

import numpy as np
import torch

from mcts_collect import play_one_game
from model import PPONet


def _worker_play_games(model_state_dict, num_games, num_simulations, filters, num_res_blocks):
    """Worker function: load model on CPU, play games, return examples."""
    net = PPONet(filters=filters, num_res_blocks=num_res_blocks)
    net.load_state_dict(model_state_dict)
    net.eval()

    all_examples = []
    for _ in range(num_games):
        examples = play_one_game(net, num_simulations, device="cpu")
        all_examples.extend(examples)

    return [(s.tolist(), p.tolist(), z) for s, p, z in all_examples]


def collect_alphazero_data_parallel(
    net, num_games, num_simulations, device="cpu", num_workers=None, filters=64, num_res_blocks=0
):
    """Parallel MCTS self-play data collection.

    Distributes games across worker processes, each with a CPU copy of the model.

    Returns:
        states: np.ndarray of shape (N, 19, 4, 4)
        policy_targets: np.ndarray of shape (N, 320)
        value_targets: np.ndarray of shape (N,)
    """
    if num_workers is None:
        num_workers = min(mp.cpu_count() - 1, 8)

    # Split games across workers
    games_per_worker = [num_games // num_workers] * num_workers
    for i in range(num_games % num_workers):
        games_per_worker[i] += 1

    # Get model weights (CPU)
    model_state_dict = {k: v.cpu() for k, v in net.state_dict().items()}

    # Run workers
    with mp.Pool(num_workers) as pool:
        results = pool.starmap(
            _worker_play_games,
            [(model_state_dict, g, num_simulations, filters, num_res_blocks) for g in games_per_worker if g > 0],
        )

    # Merge results
    all_states = []
    all_policies = []
    all_values = []

    for worker_examples in results:
        for state, policy, z in worker_examples:
            all_states.append(state)
            all_policies.append(policy)
            all_values.append(z)

    return (
        np.array(all_states, dtype=np.float32),
        np.array(all_policies, dtype=np.float32),
        np.array(all_values, dtype=np.float32),
    )
