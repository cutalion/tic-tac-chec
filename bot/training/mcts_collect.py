"""MCTS self-play data collection for AlphaZero-style training.

Runs MCTS at each move to produce training examples:
  (encoded_state, policy_target, game_outcome)

The policy target is the normalized visit count distribution from MCTS root.
The game outcome z is +1 (win), -1 (loss), or 0 (draw) from each position's
player perspective.

Optimization: lazy node realization — child nodes are created without
environments during expand(). The env is cloned only when the child is
actually selected for visitation (realize_node). This reduces env.clone()
calls from ~40 per MCTS simulation to exactly 1.
"""

import math

import numpy as np
import torch
import torch.nn.functional as F
from env import ACTION_SPACE_SIZE, TicTacChecEnv
from model import PPONet

# --- MCTS constants ---

DEFAULT_CPUCT = 1.4
DIRICHLET_ALPHA = 0.3
DIRICHLET_WEIGHT = 0.25  # prior = 0.75 * net_prior + 0.25 * Dir(alpha)
TEMP_THRESHOLD = 8  # moves 1-8 use temp=1.0, after that temp=0.1
TEMP_HIGH = 1.0
TEMP_LOW = 0.1


# --- MCTS node ---


class Node:
    """MCTS tree node.

    Nodes start without an environment (env=None) when created as children
    during expand(). The env is created lazily via realize_node() only when
    the node is actually selected for visitation.
    """

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

    def __init__(self, env=None, parent=None, action=-1, prior=0.0):
        self.env = env
        self.parent = parent
        self.children = []
        self.action = action
        self.prior = prior
        self.visit_count = 0
        self.total_value = 0.0
        self.is_terminal = (env is not None and env.done)
        self.terminal_value = 0.0

    def ucb_score(self, cpuct):
        if self.visit_count == 0:
            return float("inf")
        q = -self.total_value / self.visit_count
        exploration = (
            cpuct
            * self.prior
            * math.sqrt(self.parent.visit_count)
            / (1 + self.visit_count)
        )
        return q + exploration


# --- Lazy realization ---


def realize_node(node):
    """Create the environment for a lazy node by cloning parent and stepping.

    Called only when this node is selected for visitation. Sets is_terminal
    and terminal_value if the game ends after this move.
    """
    if node.env is not None:
        return
    node.env = node.parent.env.clone()
    _, _, done, info = node.env.step(node.action)
    if done:
        node.is_terminal = True
        if info.get("winner") is not None:
            node.terminal_value = 1.0
        else:
            node.terminal_value = 0.0


# --- MCTS core ---


def select_leaf(node):
    """Walk from node to a leaf using UCB scores.

    When a child without an env is selected, it's realized (env cloned
    from parent) and returned as the leaf.
    """
    while node.children:
        best = None
        best_score = -1e9
        for child in node.children:
            score = child.ucb_score(DEFAULT_CPUCT)
            if score > best_score:
                best_score = score
                best = child
        if best.env is None:
            realize_node(best)
            return best  # newly realized: either terminal or needs expand
        if not best.children:
            return best  # realized but unexpanded (shouldn't normally happen)
        node = best
    return node


def expand(node, net, device):
    """Expand a leaf node: run neural net, create children with priors.

    Children are created WITHOUT environments (lazy). Envs are created
    later via realize_node() only when a child is selected.

    Returns the value estimate from the network (from current player's perspective).
    """
    state = node.env.encode_state()
    state_t = torch.tensor(state, dtype=torch.float32, device=device).unsqueeze(0)

    with torch.no_grad():
        logits, value = net(state_t)

    value = value.item()
    logits = logits.squeeze(0)

    legal = node.env.legal_actions()
    if not legal:
        node.is_terminal = True
        node.terminal_value = 0.0
        return 0.0

    legal_logits = logits[legal]
    priors = F.softmax(legal_logits, dim=0).cpu().numpy()

    for i, action in enumerate(legal):
        child = Node(env=None, parent=node, action=action, prior=float(priors[i]))
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
            backpropagate(leaf, -leaf.terminal_value)
        else:
            value = expand(leaf, net, device)
            backpropagate(leaf, value)

    # Extract visit counts
    actions = [child.action for child in root.children]
    visit_counts = [child.visit_count for child in root.children]

    if sum(visit_counts) == 0:
        visit_counts = [max(1, int(child.prior * 100)) for child in root.children]

    return visit_counts, actions


def visit_counts_to_policy(visit_counts, actions, temperature):
    """Convert visit counts to a policy distribution over the full action space.

    Returns numpy array of shape (ACTION_SPACE_SIZE,).
    """
    policy = np.zeros(ACTION_SPACE_SIZE, dtype=np.float64)

    if temperature < 1e-3:
        best_idx = np.argmax(visit_counts)
        policy[actions[best_idx]] = 1.0
    else:
        counts = np.array(visit_counts, dtype=np.float64)
        counts = counts ** (1.0 / temperature)
        total = counts.sum()
        if total > 0:
            counts /= total
        for i, action in enumerate(actions):
            policy[action] = counts[i]

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

        visit_counts, actions = run_mcts(env, net, num_simulations, device)

        temperature = get_temperature(move_number)
        policy_target = visit_counts_to_policy(visit_counts, actions, temperature)

        examples.append((state, policy_target, current_player))

        action = np.random.choice(ACTION_SPACE_SIZE, p=policy_target)
        env.step(action)

    training_examples = []
    for state, policy_target, player in examples:
        if env.winner is not None:
            z = 1.0 if int(env.winner) == int(player) else -1.0
        else:
            z = 0.0
        training_examples.append((state, policy_target, z))

    return training_examples


def collect_alphazero_data(net, num_games, num_simulations, device="cpu"):
    """Play num_games of MCTS self-play, collecting training data.

    Returns:
        states: np.ndarray of shape (N, 19, 4, 4)
        policy_targets: np.ndarray of shape (N, 320)
        value_targets: np.ndarray of shape (N,)
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
