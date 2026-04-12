"""PPO actor-critic network for Tic Tac Chec.

Architecture (from research.md):
- Input: (batch, 19, 4, 4) state tensor
- Body: Conv2d(19→F, 3x3, pad=1) → ReLU → [optional residual blocks] → Flatten
- Actor head: Linear(F*16→320) → action logits
- Critic head: Linear(F*16→1) → state value

Default (small): F=64, 0 residual blocks (~380K params)
Large: F=128, 4 residual blocks (~2M params)
"""

import torch
import torch.nn as nn
from env import ACTION_SPACE_SIZE, BOARD_SIZE, NUM_CHANNELS


class ResBlock(nn.Module):
    """Residual block: conv → BN → ReLU → conv → BN → skip add → ReLU."""

    def __init__(self, filters: int):
        super().__init__()
        self.conv1 = nn.Conv2d(filters, filters, kernel_size=3, padding=1)
        self.bn1 = nn.BatchNorm2d(filters)
        self.conv2 = nn.Conv2d(filters, filters, kernel_size=3, padding=1)
        self.bn2 = nn.BatchNorm2d(filters)

    def forward(self, x: torch.Tensor) -> torch.Tensor:
        residual = x
        out = torch.relu(self.bn1(self.conv1(x)))
        out = self.bn2(self.conv2(out))
        return torch.relu(out + residual)


class PPONet(nn.Module):
    """Actor-critic network for PPO self-play training."""

    def __init__(self, filters: int = 64, num_res_blocks: int = 0):
        super().__init__()
        output_size = filters * BOARD_SIZE * BOARD_SIZE

        layers = [
            nn.Conv2d(NUM_CHANNELS, filters, kernel_size=3, padding=1),
            nn.ReLU(),
        ]

        if num_res_blocks > 0:
            for _ in range(num_res_blocks):
                layers.append(ResBlock(filters))
        else:
            # Original architecture: second conv + ReLU
            layers.append(nn.Conv2d(filters, filters, kernel_size=3, padding=1))
            layers.append(nn.ReLU())

        layers.append(nn.Flatten())
        self.body = nn.Sequential(*layers)

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
