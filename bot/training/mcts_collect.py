"""MCTS self-play data collection for AlphaZero-style training.

Runs MCTS at each move to produce training examples:
  (encoded_state, policy_target, game_outcome)

The policy target is the normalized visit count distribution from MCTS root.
The game outcome z is +1 (win), -1 (loss), or 0 (draw) from each position's
player perspective.
"""

import math

import numpy as np
import torch
import torch.nn.functional as F
from env import ACTION_SPACE_SIZE, TicTacChecEnv
from model import PPONet

# --- MCTS constants ---

DEFAULT_CPUCT = 1.4
DIRICHLET_ALPHA = 0.5
DIRICHLET_WEIGHT = 0.25  # prior = 0.75 * net_prior + 0.25 * Dir(alpha)
TEMP_THRESHOLD = 8  # moves 1-8 use temp=1.0, after that temp=0.1
TEMP_HIGH = 1.0
TEMP_LOW = 0.1


# --- MCTS node ---


class Node:
    """MCTS tree node."""

    __slots__ = (
        "env",
        "parent",
        "children",
        "action",
        "prior",
        "visit_count",
        "total_value",
        "is_terminal",
        "terminal_value",
    )

    def __init__(self, env, parent=None, action=-1, prior=0.0):
        self.env = env
        self.parent = parent
        self.children = []
        self.action = action
        self.prior = prior
        self.visit_count = 0
        self.total_value = 0.0
        self.is_terminal = env.done
        self.terminal_value = 0.0

    def ucb_score(self, cpuct):
        if self.visit_count == 0:
            return float("inf")
        q = self.total_value / self.visit_count
        exploration = (
            cpuct
            * self.prior
            * math.sqrt(self.parent.visit_count)
            / (1 + self.visit_count)
        )
        return q + exploration


# --- MCTS core ---


def select_leaf(node):
    """Walk from node to an unexpanded leaf using UCB scores."""
    while node.children:
        best = None
        best_score = -1e9
        for child in node.children:
            score = child.ucb_score(DEFAULT_CPUCT)
            if score > best_score:
                best_score = score
                best = child
        node = best
    return node


def expand(node, net, device):
    """Expand a leaf node: run neural net, create children for legal actions.

    Returns the value estimate from the network (from current player's perspective).
    """
    state = node.env.encode_state()
    state_t = torch.tensor(state, dtype=torch.float32, device=device).unsqueeze(0)

    with torch.no_grad():
        logits, value = net(state_t)

    value = value.item()
    logits = logits.squeeze(0)

    # Get legal actions and compute priors via masked softmax
    legal = node.env.legal_actions()
    if not legal:
        node.is_terminal = True
        node.terminal_value = 0.0
        return 0.0

    legal_logits = logits[legal]
    priors = F.softmax(legal_logits, dim=0).cpu().numpy()

    for i, action in enumerate(legal):
        child_env = node.env.clone()
        _, _, done, info = child_env.step(action)

        child = Node(child_env, parent=node, action=action, prior=priors[i])

        if done:
            child.is_terminal = True
            if info.get("winner") is not None:
                child.terminal_value = 1.0  # parent's player won (just moved)
            else:
                child.terminal_value = 0.0  # draw

        node.children.append(child)

    return value


def backpropagate(node, value):
    """Update visit counts and values from node to root, flipping sign each level."""
    current = node
    while current is not None:
        current.visit_count += 1
        current.total_value += value
        value = -value
        current = current.parent


def add_dirichlet_noise(node):
    """Add Dirichlet noise to root node priors for exploration."""
    if not node.children:
        return
    noise = np.random.dirichlet([DIRICHLET_ALPHA] * len(node.children))
    for i, child in enumerate(node.children):
        child.prior = (1 - DIRICHLET_WEIGHT) * child.prior + DIRICHLET_WEIGHT * noise[i]


def run_mcts(env, net, num_simulations, device):
    """Run MCTS from the given position.

    Returns (visit_counts, actions) where visit_counts[i] is the visit count
    for actions[i] among root's children.

    NOTE: Future optimization — batch leaf evaluations across simulations
    instead of one forward pass per simulation.
    """
    root = Node(env.clone())

    # Expand root
    value = expand(root, net, device)
    backpropagate(root, value)

    # Add exploration noise at root
    add_dirichlet_noise(root)

    # Run remaining simulations
    for _ in range(num_simulations - 1):
        leaf = select_leaf(root)

        if leaf.is_terminal:
            # Terminal value is from parent's perspective (player who moved into this node won).
            # We still need to count the visit on the leaf itself, so start backprop from leaf
            # but use -terminal_value (flipped to leaf's perspective) since backprop adds to
            # current node then flips.
            backpropagate(leaf, -leaf.terminal_value)
        else:
            value = expand(leaf, net, device)
            backpropagate(leaf, value)

    # Extract visit counts
    actions = [child.action for child in root.children]
    visit_counts = [child.visit_count for child in root.children]

    # Safety: if all visits are zero (shouldn't happen), fall back to priors
    if sum(visit_counts) == 0:
        visit_counts = [max(1, int(child.prior * 100)) for child in root.children]

    return visit_counts, actions


def visit_counts_to_policy(visit_counts, actions, temperature):
    """Convert visit counts to a policy distribution over the full action space.

    Returns numpy array of shape (ACTION_SPACE_SIZE,).
    """
    policy = np.zeros(ACTION_SPACE_SIZE, dtype=np.float64)

    if temperature < 1e-3:
        # Near-zero temp: argmax
        best_idx = np.argmax(visit_counts)
        policy[actions[best_idx]] = 1.0
    else:
        # Apply temperature: pi(a) = visits(a)^(1/temp) / sum
        counts = np.array(visit_counts, dtype=np.float64)
        counts = counts ** (1.0 / temperature)
        total = counts.sum()
        if total > 0:
            counts /= total
        for i, action in enumerate(actions):
            policy[action] = counts[i]

    # Ensure exact sum=1 for np.random.choice
    nonzero = policy > 0
    if nonzero.any():
        policy[nonzero] /= policy[nonzero].sum()

    return policy.astype(np.float32)


def get_temperature(move_number):
    """Temperature schedule: high early for exploration, low later for exploitation."""
    if move_number <= TEMP_THRESHOLD:
        return TEMP_HIGH
    return TEMP_LOW


# --- Self-play data collection ---


def play_one_game(net, num_simulations, device):
    """Play one self-play game with MCTS, collecting training examples.

    Returns list of (state, policy_target, player) tuples.
    Game outcome z is assigned after the game ends.
    """
    env = TicTacChecEnv()
    examples = []
    move_number = 0

    while not env.done:
        move_number += 1
        state = env.encode_state()
        current_player = env.turn

        # Run MCTS
        visit_counts, actions = run_mcts(env, net, num_simulations, device)

        # Convert to policy target with temperature
        temperature = get_temperature(move_number)
        policy_target = visit_counts_to_policy(visit_counts, actions, temperature)

        examples.append((state, policy_target, current_player))

        # Sample action from policy (not argmax — diverse training games)
        action = np.random.choice(ACTION_SPACE_SIZE, p=policy_target)
        env.step(action)

    # Assign game outcomes
    training_examples = []
    for state, policy_target, player in examples:
        if env.winner is not None:
            z = 1.0 if int(env.winner) == int(player) else -1.0
        else:
            z = 0.0  # draw
        training_examples.append((state, policy_target, z))

    return training_examples


def collect_alphazero_data(net, num_games, num_simulations, device="cpu"):
    """Play num_games of MCTS self-play, collecting training data.

    Returns:
        states: np.ndarray of shape (N, 19, 4, 4)
        policy_targets: np.ndarray of shape (N, 320)
        value_targets: np.ndarray of shape (N,)

    NOTE: Future optimization — run multiple games in parallel with batched
    leaf evaluation across games.
    """
    all_states = []
    all_policies = []
    all_values = []

    for _ in range(num_games):
        examples = play_one_game(net, num_simulations, device)
        for state, policy, z in examples:
            all_states.append(state)
            all_policies.append(policy)
            all_values.append(z)

    return (
        np.array(all_states, dtype=np.float32),
        np.array(all_policies, dtype=np.float32),
        np.array(all_values, dtype=np.float32),
    )
