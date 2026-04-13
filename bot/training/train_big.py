"""Train a larger model (128 filters, 4 residual blocks) for stronger play.

Usage: uv run python train_big.py
"""

import sys
from train_overnight import train_overnight, best_device

if __name__ == "__main__":
    resume = sys.argv[1] if len(sys.argv) > 1 else None
    train_overnight(
        num_iterations=5000,
        games_per_iter=128,
        eval_every=25,
        checkpoint_every=200,
        lr=1e-4,
        checkpoint_dir="checkpoints_big",
        log_dir="runs/big",
        device=best_device(),
        resume_from=resume,
        filters=128,
        num_res_blocks=4,
        opponent_checkpoint="checkpoints/ppo_hard.pt",
        opponent_filters=64,
        opponent_res_blocks=0,
    )
