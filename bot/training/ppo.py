"""PPO training: rollout collection and policy update for Tic Tac Chec.

Rollout collection: self-play and opponent-pool games storing transitions.
PPO update: GAE advantages, clipped surrogate loss, critic loss, entropy bonus.
"""

import random as pyrandom

import numpy as np
import torch
import torch.nn.functional as F
from env import ACTION_SPACE_SIZE, TicTacChecEnv
from model import PPONet

# --- Rollout storage ---


class RolloutBuffer:
    """Stores transitions from self-play games."""

    def __init__(self):
        self.states = []
        self.actions = []
        self.rewards = []
        self.dones = []
        self.log_probs = []
        self.values = []
        self.masks = []

    def add(self, state, action, reward, done, log_prob, value, mask=None):
        self.states.append(state)
        self.actions.append(action)
        self.rewards.append(reward)
        self.dones.append(done)
        self.log_probs.append(log_prob)
        self.values.append(value)
        self.masks.append(mask)

    def clear(self):
        self.states.clear()
        self.actions.clear()
        self.rewards.clear()
        self.dones.clear()
        self.log_probs.clear()
        self.values.clear()
        self.masks.clear()

    def to_tensors(self, device="cpu"):
        result = (
            torch.tensor(np.array(self.states), dtype=torch.float32, device=device),
            torch.tensor(self.actions, dtype=torch.long, device=device),
            torch.tensor(self.rewards, dtype=torch.float32, device=device),
            torch.tensor(self.dones, dtype=torch.float32, device=device),
            torch.tensor(self.log_probs, dtype=torch.float32, device=device),
            torch.tensor(self.values, dtype=torch.float32, device=device),
        )
        if self.masks[0] is not None:
            return result + (torch.tensor(np.array(self.masks), dtype=torch.bool, device=device),)
        return result + (None,)

    def __len__(self):
        return len(self.states)


# --- Rollout collection ---


def select_action(net: PPONet, state: np.ndarray, legal_mask: np.ndarray, device="cpu"):
    """Select action using the policy network with action masking.

    Returns (action_index, log_prob, value_estimate).
    """
    state_t = torch.tensor(state, dtype=torch.float32, device=device).unsqueeze(0)
    with torch.no_grad():
        logits, value = net(state_t)

    # Mask illegal actions: set to -inf before softmax
    mask_t = torch.tensor(legal_mask, dtype=torch.bool, device=device)
    logits = logits.squeeze(0)
    logits[~mask_t] = float("-inf")

    # Sample from masked distribution
    probs = F.softmax(logits, dim=0)
    dist = torch.distributions.Categorical(probs)
    action = dist.sample()

    return action.item(), dist.log_prob(action).item(), value.item()


def _batched_select(net, states_batch, masks_batch, device):
    """Batched forward pass + masked sampling. Returns (actions, log_probs, values) as numpy."""
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


def _finish_game(game_idx, info, game_transitions, buffer):
    """Assign terminal rewards and add transitions to buffer."""
    winner = info.get("winner")
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


def collect_rollouts(net: PPONet, num_games: int, device="cpu", opponent_pool=None):
    """Play num_games in parallel, collecting transitions for the learning agent.

    If opponent_pool is provided, each game randomly picks an opponent from the pool.
    The learning agent (net) plays one color, the opponent plays the other.
    Only transitions from the learning agent's side are stored for training.

    If opponent_pool is None, all games are self-play (both sides use net).

    Returns a RolloutBuffer with transitions.
    """
    buffer = RolloutBuffer()

    # Initialize all environments
    envs = [TicTacChecEnv() for _ in range(num_games)]
    observations = [env.encode_state() for env in envs]
    active = list(range(num_games))

    # Per-game transition lists: [white_transitions, black_transitions]
    game_transitions = [([], []) for _ in range(num_games)]

    # Assign opponents and sides for each game
    if opponent_pool:
        # Learning agent plays a random color per game
        agent_colors = [pyrandom.randint(0, 1) for _ in range(num_games)]
        opponents = [pyrandom.choice(opponent_pool) for _ in range(num_games)]
    else:
        agent_colors = None
        opponents = None

    while active:
        states_batch = np.array([observations[i] for i in active])
        masks_batch = np.array([envs[i].legal_action_mask() for i in active])
        players = [envs[i].turn for i in active]

        # Batched forward pass for the learning agent
        actions, log_probs, vals = _batched_select(net, states_batch, masks_batch, device)

        # If using opponent pool, also get opponent actions for games where it's the opponent's turn
        if opponents:
            # Group active games by opponent for batched inference
            opp_needs = []  # (idx_in_batch, game_idx) where opponent should move
            for idx_in_batch, game_idx in enumerate(active):
                if players[idx_in_batch] != agent_colors[game_idx]:
                    opp_needs.append((idx_in_batch, game_idx))

            if opp_needs:
                # Group by opponent identity for batched inference
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

        # Step each active environment
        newly_done = []
        for idx_in_batch, game_idx in enumerate(active):
            action = int(actions[idx_in_batch])
            log_prob = float(log_probs[idx_in_batch])
            value = float(vals[idx_in_batch])
            current_player = players[idx_in_batch]

            obs = observations[game_idx]

            mask = masks_batch[idx_in_batch]

            if opponents:
                # Only store transitions for the learning agent's side
                is_agent_turn = current_player == agent_colors[game_idx]
                if is_agent_turn:
                    transition = (obs, action, log_prob, value, mask)
                    if current_player == 0:
                        game_transitions[game_idx][0].append(transition)
                    else:
                        game_transitions[game_idx][1].append(transition)
            else:
                # Self-play: store both sides
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


# --- PPO update ---


def compute_gae(rewards, values, dones, gamma=0.99, lam=0.95):
    """Compute Generalized Advantage Estimation."""
    T = len(rewards)
    advantages = torch.zeros(T, device=rewards.device)
    for t in reversed(range(T)):
        next_value = values[t + 1] if t < T - 1 else 0.0
        done = 1 - dones[t]
        prev_advantage = advantages[t + 1] if t < T - 1 else 0.0

        delta = rewards[t] + gamma * next_value * done - values[t]
        advantages[t] = delta + gamma * lam * done * prev_advantage

    returns = advantages + values
    return advantages, returns


def ppo_update(
    net: PPONet,
    optimizer: torch.optim.Optimizer,
    buffer: RolloutBuffer,
    epochs: int = 4,
    clip_eps: float = 0.2,
    value_coef: float = 0.5,
    entropy_coef: float = 0.05,
    max_grad_norm: float = 0.5,
    device: str = "cpu",
):
    """Run PPO policy update on collected rollout data."""
    states, actions, rewards, dones, old_log_probs, old_values, legal_masks = buffer.to_tensors(device)
    advantages, returns = compute_gae(rewards, old_values, dones)
    advantages = (advantages - advantages.mean()) / (advantages.std() + 1e-8)

    actor_loss, critic_loss, entropy, total_loss = 0, 0, 0, 0

    for _ in range(epochs):
        logits, values = net(states)
        if legal_masks is not None:
            logits[~legal_masks] = float("-inf")
        dist = torch.distributions.Categorical(logits=logits)
        new_log_probs = dist.log_prob(actions)
        entropy = dist.entropy().mean()
        ratio = torch.exp(new_log_probs - old_log_probs)
        surr1 = ratio * advantages
        surr2 = torch.clamp(ratio, 1 - clip_eps, 1 + clip_eps) * advantages
        actor_loss = -torch.min(surr1, surr2).mean()
        critic_loss = F.mse_loss(values.squeeze(), returns)
        total_loss = actor_loss + value_coef * critic_loss - entropy_coef * entropy
        optimizer.zero_grad()
        total_loss.backward()
        torch.nn.utils.clip_grad_norm_(net.parameters(), max_grad_norm)
        optimizer.step()

    return {
        "actor_loss": actor_loss,
        "critic_loss": critic_loss,
        "entropy": entropy,
        "total_loss": total_loss,
    }
