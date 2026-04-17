"""FIFO replay buffer for AlphaZero training.

Stores (state, policy_target, value_target) positions from MCTS self-play.
Positions are reused across multiple training iterations — much more data-efficient
than throwing away data after each iteration.

Usage:
    buf = ReplayBuffer(max_size=100_000)
    buf.add(states, policies, values)       # add batch of positions
    s, p, v = buf.sample(256)               # random minibatch
    buf.save("buffer.npz")                  # persist for resume
    buf.load("buffer.npz")                  # restore
"""

import numpy as np

STATE_SHAPE = (19, 4, 4)
POLICY_SIZE = 320


class ReplayBuffer:
    """Fixed-size FIFO circular buffer for AlphaZero training data."""

    def __init__(self, max_size: int = 100_000):
        self.max_size = max_size
        self.states = np.zeros((max_size, *STATE_SHAPE), dtype=np.float32)
        self.policies = np.zeros((max_size, POLICY_SIZE), dtype=np.float32)
        self.values = np.zeros(max_size, dtype=np.float32)
        self.size = 0       # current number of valid entries
        self.index = 0      # next write position

    def __len__(self):
        return self.size

    def add(self, states: np.ndarray, policies: np.ndarray, values: np.ndarray):
        """Add a batch of positions to the buffer.

        Args:
            states: (N, 19, 4, 4) encoded board states
            policies: (N, 320) MCTS visit count distributions
            values: (N,) game outcomes from current player's perspective
        """
        n = len(states)
        if n == 0:
            return

        if n >= self.max_size:
            # More data than buffer — keep only the last max_size entries
            states = states[-self.max_size:]
            policies = policies[-self.max_size:]
            values = values[-self.max_size:]
            n = self.max_size
            self.states[:] = states
            self.policies[:] = policies
            self.values[:] = values
            self.size = self.max_size
            self.index = 0
            return

        # Wrap-around write
        end = self.index + n
        if end <= self.max_size:
            self.states[self.index:end] = states
            self.policies[self.index:end] = policies
            self.values[self.index:end] = values
        else:
            first = self.max_size - self.index
            self.states[self.index:] = states[:first]
            self.policies[self.index:] = policies[:first]
            self.values[self.index:] = values[:first]
            remainder = n - first
            self.states[:remainder] = states[first:]
            self.policies[:remainder] = policies[first:]
            self.values[:remainder] = values[first:]

        self.index = end % self.max_size
        self.size = min(self.size + n, self.max_size)

    def sample(self, batch_size: int):
        """Sample a random minibatch.

        Returns:
            states: (batch_size, 19, 4, 4)
            policies: (batch_size, 320)
            values: (batch_size,)
        """
        indices = np.random.randint(0, self.size, size=batch_size)
        return self.states[indices], self.policies[indices], self.values[indices]

    def save(self, path: str):
        """Save buffer contents to disk."""
        np.savez_compressed(
            path,
            states=self.states[:self.size] if self.size < self.max_size else self.states,
            policies=self.policies[:self.size] if self.size < self.max_size else self.policies,
            values=self.values[:self.size] if self.size < self.max_size else self.values,
            index=np.array([self.index]),
            size=np.array([self.size]),
        )

    def load(self, path: str):
        """Load buffer contents from disk."""
        data = np.load(path)
        n = int(data["size"][0])
        self.states[:n] = data["states"][:n]
        self.policies[:n] = data["policies"][:n]
        self.values[:n] = data["values"][:n]
        self.size = n
        self.index = int(data["index"][0])
