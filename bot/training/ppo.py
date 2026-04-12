"""PPO training: rollout collection and policy update for Tic Tac Chec.

Rollout collection (T017): self-play games storing transitions.
PPO update (T018): GAE advantages, clipped surrogate loss, critic loss, entropy bonus.
"""

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

    def add(self, state, action, reward, done, log_prob, value):
        self.states.append(state)
        self.actions.append(action)
        self.rewards.append(reward)
        self.dones.append(done)
        self.log_probs.append(log_prob)
        self.values.append(value)

    def clear(self):
        self.states.clear()
        self.actions.clear()
        self.rewards.clear()
        self.dones.clear()
        self.log_probs.clear()
        self.values.clear()

    def to_tensors(self, device="cpu"):
        return (
            torch.tensor(np.array(self.states), dtype=torch.float32, device=device),
            torch.tensor(self.actions, dtype=torch.long, device=device),
            torch.tensor(self.rewards, dtype=torch.float32, device=device),
            torch.tensor(self.dones, dtype=torch.float32, device=device),
            torch.tensor(self.log_probs, dtype=torch.float32, device=device),
            torch.tensor(self.values, dtype=torch.float32, device=device),
        )

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


def collect_rollouts(net: PPONet, num_games: int, device="cpu"):
    """Play num_games of self-play, collecting transitions for both sides.

    Each game alternates between White and Black. Both sides use the same
    network (self-play). Rewards are flipped for the losing side at game end.

    Returns a RolloutBuffer with all transitions.
    """
    buffer = RolloutBuffer()

    for _ in range(num_games):
        env = TicTacChecEnv()
        obs = env.encode_state()

        # Separate transition lists per player so GAE is computed independently
        white_transitions = []  # (state, action, log_prob, value)
        black_transitions = []

        done = False
        while not done:
            mask = env.legal_action_mask()
            current_player = env.turn  # 0=White, 1=Black

            action, log_prob, value = select_action(net, obs, mask, device)

            next_obs, reward, done, info = env.step(action)

            transition = (obs, action, log_prob, value)
            if current_player == 0:
                white_transitions.append(transition)
            else:
                black_transitions.append(transition)

            obs = next_obs

        # Assign terminal rewards from each player's perspective
        winner = info.get("winner")
        for transitions, color in [(white_transitions, 0), (black_transitions, 1)]:
            for i, (state, action, lp, val) in enumerate(transitions):
                is_last = (i == len(transitions) - 1)
                if is_last:
                    if winner is None:
                        r = 0.0  # draw
                    elif int(winner) == color:
                        r = 1.0  # this player won
                    else:
                        r = -1.0  # this player lost
                else:
                    r = 0.0
                buffer.add(state, action, r, float(is_last), lp, val)

    return buffer


# --- PPO update ---


def compute_gae(rewards, values, dones, gamma=0.99, lam=0.95):
    """Compute Generalized Advantage Estimation.

    Args:
        rewards: tensor of shape (T,) — reward at each timestep
        values: tensor of shape (T,) — critic value estimate at each timestep
        dones: tensor of shape (T,) — 1.0 if episode ended, 0.0 otherwise
        gamma: discount factor (how much future rewards matter)
        lam: GAE lambda (trade-off between bias and variance)

    Returns:
        (advantages, returns) — both tensors of shape (T,)
        - advantages: how much better the action was than expected
        - returns: discounted target values for the critic
    """
    T = len(rewards)
    advantages = torch.zeros(T, device=rewards.device)
    for t in reversed(range(T)):
        next_value = values[t + 1] if t < T - 1 else 0.0
        done = 1 - dones[t]  # 1.0 if episode ended, 0.0 otherwise.
        prev_advantage = advantages[t + 1] if t < T - 1 else 0.0

        delta = (
            rewards[t] + gamma * next_value * done - values[t]
        )  # done = 0 when game ended
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
    device: str = "cpu",
):
    """Run PPO policy update on collected rollout data.

    Args:
        net: the PPO actor-critic network
        optimizer: Adam or similar
        buffer: collected rollout transitions
        epochs: number of passes over the batch
        clip_eps: PPO clipping parameter (0.2 is standard)
        value_coef: weight for critic loss (0.5 is standard)
        entropy_coef: weight for entropy bonus (encourages exploration)
        device: "cpu" or "cuda"

    Returns:
        dict with training metrics: actor_loss, critic_loss, entropy, total_loss
    """

    states, actions, rewards, dones, old_log_probs, old_values = buffer.to_tensors(
        device
    )
    advantages, returns = compute_gae(rewards, old_values, dones)
    advantages = (advantages - advantages.mean()) / (advantages.std() + 1e-8)

    actor_loss, critic_loss, entropy, total_loss = 0, 0, 0, 0

    for _ in range(epochs):
        logits, values = net(states)
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
        optimizer.step()

    return {
        "actor_loss": actor_loss,
        "critic_loss": critic_loss,
        "entropy": entropy,
        "total_loss": total_loss,
    }
