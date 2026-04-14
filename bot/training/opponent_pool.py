"""Smart Opponent Pool with Prioritized Fictitious Play.

Instead of sampling opponents randomly, tracks win rates against each
pool member and preferentially plays against opponents the agent loses to.
This prevents the "rock-paper-scissors" forgetting cycle.
"""

import random
from collections import defaultdict

import torch
from model import PPONet
from evaluate_fast import evaluate_vs_opponent


class SmartOpponentPool:
    """Opponent pool with win-rate-based sampling."""

    def __init__(self, max_size=10, device="cpu"):
        self.opponents = []  # list of (name, net) tuples
        self.win_rates = {}  # name -> win_rate of current model vs this opponent
        self.max_size = max_size
        self.device = device
        self._eval_games = 50  # games to play for win rate estimation

    def add(self, name, net):
        """Add an opponent to the pool."""
        self.opponents.append((name, net))
        self.win_rates[name] = 0.5  # assume even until measured
        self._evict_if_needed()

    def _evict_if_needed(self):
        """Remove opponents the agent dominates (>90% win rate)."""
        if len(self.opponents) <= self.max_size:
            return

        # Sort by win rate — remove easiest opponents first
        # But never remove "fixed" opponents (name starts with "fixed_")
        removable = [(name, wr) for name, wr in self.win_rates.items()
                     if not name.startswith("fixed_")]
        removable.sort(key=lambda x: -x[1])  # highest win rate first (easiest)

        while len(self.opponents) > self.max_size and removable:
            name_to_remove, wr = removable.pop(0)
            if wr < 0.85:  # don't remove challenging opponents
                break
            self.opponents = [(n, net) for n, net in self.opponents if n != name_to_remove]
            del self.win_rates[name_to_remove]

    def sample(self):
        """Sample an opponent weighted by difficulty (inverse win rate).

        Opponents the agent loses to are sampled more frequently.
        """
        if not self.opponents:
            return None

        names = [name for name, _ in self.opponents]
        # Weight = (1 - win_rate)^2 — strongly prefer hard opponents
        weights = [(1.0 - self.win_rates.get(name, 0.5)) ** 2 + 0.05 for name in names]
        total = sum(weights)
        weights = [w / total for w in weights]

        chosen_name = random.choices(names, weights=weights, k=1)[0]
        for name, net in self.opponents:
            if name == chosen_name:
                return net
        return self.opponents[0][1]  # fallback

    def get_all_nets(self):
        """Return all opponent networks (for collect_rollouts compatibility)."""
        return [net for _, net in self.opponents]

    def update_win_rates(self, current_net, num_games=None):
        """Re-evaluate win rates against all pool opponents."""
        if num_games is None:
            num_games = self._eval_games

        current_net.eval()
        for name, opp_net in self.opponents:
            wr, _, _, _ = evaluate_vs_opponent(
                current_net, opp_net, num_games=num_games, device=self.device
            )
            self.win_rates[name] = wr

    def should_add_to_pool(self, current_net, threshold=0.55):
        """Check if current model is strong enough to join the pool.

        Must beat the latest pool member by threshold.
        """
        if not self.opponents:
            return True

        latest_name, latest_net = self.opponents[-1]
        if latest_name.startswith("fixed_"):
            # Don't compare against fixed — compare against latest self-play
            self_play_opponents = [(n, net) for n, net in self.opponents
                                   if not n.startswith("fixed_")]
            if not self_play_opponents:
                return True
            latest_name, latest_net = self_play_opponents[-1]

        current_net.eval()
        wr, _, _, _ = evaluate_vs_opponent(
            current_net, latest_net, num_games=100, device=self.device
        )
        return wr >= threshold

    def summary(self):
        """Return a summary string."""
        parts = []
        for name, _ in self.opponents:
            wr = self.win_rates.get(name, 0.5)
            parts.append(f"{name}:{wr:.0%}")
        return f"Pool({len(self.opponents)}): {', '.join(parts)}"
