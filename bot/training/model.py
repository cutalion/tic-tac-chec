"""PPO actor-critic network for Tic Tac Chec.

Architecture (from research.md):
- Input: (batch, 19, 4, 4) state tensor
- Body: Conv2d(19→64, 3x3, pad=1) → ReLU → Conv2d(64→64, 3x3, pad=1) → ReLU → Flatten
- Actor head: Linear(1024→320) → action logits
- Critic head: Linear(1024→1) → state value
"""

import torch
import torch.nn as nn
from env import ACTION_SPACE_SIZE, BOARD_SIZE, NUM_CHANNELS


class PPONet(nn.Module):
    """Actor-critic network for PPO self-play training."""

    def __init__(self):
        super().__init__()
        filters_num = 64
        output_size = filters_num * BOARD_SIZE * BOARD_SIZE

        self.body = nn.Sequential(
            nn.Conv2d(
                NUM_CHANNELS, filters_num, kernel_size=3, padding=1
            ),  # input layer
            nn.ReLU(),
            nn.Conv2d(filters_num, filters_num, kernel_size=3, padding=1),
            nn.ReLU(),
            nn.Flatten(),  # gives (batch, filters_num * BOARD_SIZE * BOARD_SIZE) = 64 * 4 * 4 = 1024
        )

        self.actor_head = nn.Linear(output_size, ACTION_SPACE_SIZE)
        self.critic_head = nn.Linear(output_size, 1)

    def forward(self, x: torch.Tensor) -> tuple[torch.Tensor, torch.Tensor]:
        """Forward pass.

        Args:
            x: state tensor of shape (batch, 19, 4, 4)

        Returns:
            (action_logits, state_value) where:
            - action_logits: shape (batch, 320) — raw scores before softmax
            - state_value: shape (batch, 1) — estimated return from this state
        """
        body_output = self.body(x)
        action_logits = self.actor_head(body_output)
        state_value = self.critic_head(body_output)
        return action_logits, state_value
