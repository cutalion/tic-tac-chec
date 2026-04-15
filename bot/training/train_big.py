"""Train a larger model (128 filters, 4 res blocks, NO batch norm) for stronger play.

Usage: uv run python train_big.py [checkpoint_to_resume]
"""

import sys
from train_overnight import train_overnight, best_device

if __name__ == "__main__":
    resume = sys.argv[1] if len(sys.argv) > 1 else None
    train_overnight(
        num_iterations=100000,
        games_per_iter=128,
        eval_every=50,
        checkpoint_every=100,
        lr=1e-3,  # higher LR ok without batch norm
        checkpoint_dir="checkpoints_big",
        log_dir="runs/big",
        device=best_device(),
        resume_from=resume,
        filters=128,
        num_res_blocks=4,
        use_batch_norm=False,
        use_opponent_pool=True,
        eval_opponent_checkpoint="checkpoints_big/ppo_eval_target.pt",
        eval_opponent_filters=64,
        eval_opponent_res_blocks=0,
        eval_opponent_use_bn=True,  # eval target is old small model (no res blocks, no BN used)
    )
