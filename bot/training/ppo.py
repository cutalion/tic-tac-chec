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
    """Play num_games of self-play in parallel, collecting transitions for both sides.

    All games run simultaneously: states are batched into a single tensor for
    one forward pass per step, then each environment is stepped individually.
    Games that finish early are replaced or skipped until all are done.

    Returns a RolloutBuffer with all transitions.
    """
    buffer = RolloutBuffer()

    # Initialize all environments
    envs = [TicTacChecEnv() for _ in range(num_games)]
    observations = [env.encode_state() for env in envs]
    active = list(range(num_games))  # indices of games still in progress

    # Per-game transition lists: [white_transitions, black_transitions]
    game_transitions = [
        ([], []) for _ in range(num_games)
    ]  # (white_list, black_list)

    while active:
        # Batch states and masks for all active games
        states_batch = np.array([observations[i] for i in active])
        masks_batch = np.array([envs[i].legal_action_mask() for i in active])
        players = [envs[i].turn for i in active]

        # Single batched forward pass
        states_t = torch.tensor(states_batch, dtype=torch.float32, device=device)
        masks_t = torch.tensor(masks_batch, dtype=torch.bool, device=device)

        with torch.no_grad():
            logits, values = net(states_t)

        # Mask illegal actions and sample
        logits[~masks_t] = float("-inf")
        probs = F.softmax(logits, dim=-1)
        dist = torch.distributions.Categorical(probs)
        actions_t = dist.sample()
        log_probs_t = dist.log_prob(actions_t)

        actions = actions_t.cpu().numpy()
        log_probs = log_probs_t.cpu().numpy()
        vals = values.squeeze(-1).cpu().numpy()

        # Step each active environment
        newly_done = []
        for idx_in_batch, game_idx in enumerate(active):
            action = int(actions[idx_in_batch])
            log_prob = float(log_probs[idx_in_batch])
            value = float(vals[idx_in_batch])
            current_player = players[idx_in_batch]

            obs = observations[game_idx]
            transition = (obs, action, log_prob, value)
            if current_player == 0:
                game_transitions[game_idx][0].append(transition)
            else:
                game_transitions[game_idx][1].append(transition)

            next_obs, reward, done, info = envs[game_idx].step(action)
            observations[game_idx] = next_obs

            if done:
                newly_done.append(game_idx)
                # Assign terminal rewards
                winner = info.get("winner")
                for transitions, color in [
                    (game_transitions[game_idx][0], 0),
                    (game_transitions[game_idx][1], 1),
                ]:
                    for i, (state, act, lp, val) in enumerate(transitions):
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
                        buffer.add(state, act, r, float(is_last), lp, val)

        # Remove finished games from active list
        if newly_done:
            active = [i for i in active if i not in newly_done]

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
