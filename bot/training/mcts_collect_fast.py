"""Fast MCTS self-play with batched leaf evaluation.

Instead of evaluating one leaf at a time, collects leaves across multiple
concurrent MCTS searches and evaluates them in a single batched forward pass.
"""

import math

import numpy as np
import torch
import torch.nn.functional as F
from env import ACTION_SPACE_SIZE, TicTacChecEnv
from model import PPONet
from mcts_collect import (
    Node,
    select_leaf,
    backpropagate,
    add_dirichlet_noise,
    visit_counts_to_policy,
    get_temperature,
    DEFAULT_CPUCT,
)


def expand_batch(leaves, net, device):
    """Expand multiple leaf nodes with a single batched forward pass.

    Returns list of values (one per leaf, from current player's perspective).
    """
    if not leaves:
        return []

    # Encode all leaf states
    states = np.array([leaf.env.encode_state() for leaf in leaves], dtype=np.float32)
    states_t = torch.from_numpy(states).to(device)

    with torch.no_grad():
        logits_batch, values_batch = net(states_t)

    values = values_batch.squeeze(-1).cpu().numpy()
    logits_batch = logits_batch.cpu()

    results = []
    for i, leaf in enumerate(leaves):
        logits = logits_batch[i]
        value = float(values[i])

        legal = leaf.env.legal_actions()
        if not legal:
            leaf.is_terminal = True
            leaf.terminal_value = 0.0
            results.append(0.0)
            continue

        legal_logits = logits[legal]
        priors = F.softmax(legal_logits, dim=0).numpy()

        for j, action in enumerate(legal):
            child_env = leaf.env.clone()
            _, _, done, info = child_env.step(action)

            child = Node(child_env, parent=leaf, action=action, prior=priors[j])

            if done:
                child.is_terminal = True
                if info.get("winner") is not None:
                    child.terminal_value = 1.0
                else:
                    child.terminal_value = 0.0

            leaf.children.append(child)

        results.append(value)

    return results


def run_mcts_batch(game_roots, net, num_simulations, device):
    """Run MCTS for multiple game positions simultaneously with batched evaluation.

    Args:
        game_roots: list of (env, root_node_or_None) — positions to search
        net: neural network for evaluation
        num_simulations: number of MCTS simulations per position
        device: torch device

    Returns: list of (visit_counts, actions) per game
    """
    n_games = len(game_roots)

    # Initialize roots
    roots = []
    for env in game_roots:
        root = Node(env.clone())
        roots.append(root)

    # Expand all roots in one batch
    values = expand_batch(roots, net, device)
    for root, value in zip(roots, values):
        backpropagate(root, value)
        add_dirichlet_noise(root)

    # Run remaining simulations with batched leaf evaluation
    for _ in range(num_simulations - 1):
        # Select leaves from all trees
        leaves_to_expand = []
        leaf_indices = []  # which game each leaf belongs to

        for game_idx, root in enumerate(roots):
            leaf = select_leaf(root)

            if leaf.is_terminal:
                backpropagate(leaf, -leaf.terminal_value)
            else:
                leaves_to_expand.append(leaf)
                leaf_indices.append(game_idx)

        # Batch evaluate all non-terminal leaves
        if leaves_to_expand:
            values = expand_batch(leaves_to_expand, net, device)
            for leaf, value in zip(leaves_to_expand, values):
                backpropagate(leaf, value)

    # Extract results
    results = []
    for root in roots:
        actions = [child.action for child in root.children]
        visit_counts = [child.visit_count for child in root.children]
        if sum(visit_counts) == 0:
            visit_counts = [max(1, int(child.prior * 100)) for child in root.children]
        results.append((visit_counts, actions))

    return results


def play_games_batched(net, num_games, num_simulations, device):
    """Play multiple self-play games with batched MCTS.

    All games run in parallel: at each move, MCTS searches for all active
    games share batched neural net evaluations.

    Returns list of training examples: (state, policy_target, value_target).
    """
    envs = [TicTacChecEnv() for _ in range(num_games)]
    active = list(range(num_games))
    move_numbers = [0] * num_games

    # Per-game example storage: [(state, policy_target, player), ...]
    game_examples = [[] for _ in range(num_games)]

    while active:
        # Run MCTS for all active games in one batched call
        active_envs = [envs[i] for i in active]
        results = run_mcts_batch(active_envs, net, num_simulations, device)

        # Process results and step environments
        newly_done = []
        for idx_in_batch, game_idx in enumerate(active):
            move_numbers[game_idx] += 1
            state = envs[game_idx].encode_state()
            current_player = envs[game_idx].turn

            visit_counts, actions = results[idx_in_batch]
            temperature = get_temperature(move_numbers[game_idx])
            policy_target = visit_counts_to_policy(visit_counts, actions, temperature)

            game_examples[game_idx].append((state, policy_target, current_player))

            # Sample action from policy
            action = np.random.choice(ACTION_SPACE_SIZE, p=policy_target)
            envs[game_idx].step(action)

            if envs[game_idx].done:
                newly_done.append(game_idx)

        if newly_done:
            active = [i for i in active if i not in newly_done]

    # Assign game outcomes
    all_examples = []
    for game_idx in range(num_games):
        env = envs[game_idx]
        for state, policy_target, player in game_examples[game_idx]:
            if env.winner is not None:
                z = 1.0 if int(env.winner) == int(player) else -1.0
            else:
                z = 0.0
            all_examples.append((state, policy_target, z))

    return all_examples


def collect_alphazero_data_fast(net, num_games, num_simulations, device="cpu"):
    """Batched MCTS self-play data collection.

    Returns:
        states: np.ndarray of shape (N, 19, 4, 4)
        policy_targets: np.ndarray of shape (N, 320)
        value_targets: np.ndarray of shape (N,)
    """
    examples = play_games_batched(net, num_games, num_simulations, device)

    states = np.array([e[0] for e in examples], dtype=np.float32)
    policies = np.array([e[1] for e in examples], dtype=np.float32)
    values = np.array([e[2] for e in examples], dtype=np.float32)

    return states, policies, values
