"""Overnight training: longer run with auto-export of difficulty-tagged models.

Exports ONNX models at milestones for easy/medium/hard bots.
Usage: uv run python train_overnight.py [checkpoint_to_resume]
"""

import os
import shutil
import sys
import time

import torch
from torch.utils.tensorboard import SummaryWriter

from model import PPONet
from ppo import ppo_update
from ppo_vec import collect_rollouts_vec as collect_rollouts
from evaluate_fast import evaluate_vs_random, evaluate_vs_opponent
from export import export_onnx
from opponent_pool import SmartOpponentPool

MODELS_DIR = "../models"

MILESTONES = [
    (200, "easy", 0.0),
    (1000, "medium", 0.40),
    (3000, "hard", 0.70),
]

# Pool settings
POOL_MAX_SIZE = 10
POOL_CHECK_EVERY = 100  # check if model should join pool every N iters
POOL_UPDATE_WIN_RATES_EVERY = 200  # re-evaluate all win rates every N iters
POOL_SELF_PLAY_RATIO = 0.5

# Eval settings
EVAL_VS_OPPONENT_GAMES = 400
EVAL_VS_RANDOM_GAMES = 100


def train_overnight(
    num_iterations: int = 100000,
    games_per_iter: int = 128,
    eval_every: int = 50,
    checkpoint_every: int = 100,
    lr: float = 3e-4,
    checkpoint_dir: str = "checkpoints",
    log_dir: str = "runs/overnight",
    device: str = "cpu",
    resume_from: str = None,
    filters: int = 64,
    num_res_blocks: int = 0,
    use_batch_norm: bool = True,
    use_opponent_pool: bool = False,
    eval_opponent_checkpoint: str = None,
    eval_opponent_filters: int = 64,
    eval_opponent_res_blocks: int = 0,
    eval_opponent_use_bn: bool = True,
):
    os.makedirs(checkpoint_dir, exist_ok=True)
    os.makedirs(MODELS_DIR, exist_ok=True)
    writer = SummaryWriter(log_dir)

    net = PPONet(filters=filters, num_res_blocks=num_res_blocks, use_batch_norm=use_batch_norm).to(device)
    optimizer = torch.optim.Adam(net.parameters(), lr=lr)
    start_iteration = 1
    exported = set()

    if resume_from and os.path.exists(resume_from):
        checkpoint = torch.load(resume_from, map_location=device, weights_only=True)
        net.load_state_dict(checkpoint["model_state_dict"])
        optimizer.load_state_dict(checkpoint["optimizer_state_dict"])
        start_iteration = checkpoint["iteration"] + 1
        exported = set(checkpoint.get("exported", []))
        print(f"Resumed from {resume_from} at iteration {start_iteration}", flush=True)

    num_params = sum(p.numel() for p in net.parameters())
    print(f"Training: {num_iterations} iters, {games_per_iter} games/iter", flush=True)
    print(f"Model: {filters}f, {num_res_blocks}r, {num_params:,} params", flush=True)
    print(f"Device: {device}", flush=True)

    # Smart opponent pool
    pool = None
    if use_opponent_pool:
        pool = SmartOpponentPool(max_size=POOL_MAX_SIZE, device=device)
        # Seed with initial model
        seed = PPONet(filters=filters, num_res_blocks=num_res_blocks, use_batch_norm=use_batch_norm).to(device)
        seed.load_state_dict(net.state_dict())
        seed.eval()
        pool.add("v0_init", seed)
        print(f"Smart opponent pool initialized (max {POOL_MAX_SIZE})", flush=True)

    # Protected eval opponent
    eval_opponent = None
    eval_target_path = os.path.join(checkpoint_dir, "ppo_eval_target.pt")

    if eval_opponent_checkpoint and os.path.exists(eval_opponent_checkpoint):
        if not os.path.exists(eval_target_path) or eval_opponent_checkpoint != eval_target_path:
            shutil.copy2(eval_opponent_checkpoint, eval_target_path)
        eval_opponent = PPONet(filters=eval_opponent_filters, num_res_blocks=eval_opponent_res_blocks, use_batch_norm=eval_opponent_use_bn).to(device)
        ckpt = torch.load(eval_target_path, map_location=device, weights_only=True)
        eval_opponent.load_state_dict(ckpt["model_state_dict"])
        eval_opponent.eval()
        print(f"Eval opponent: {eval_target_path}", flush=True)

        # Add fixed opponent to pool
        if pool is not None:
            pool.add("fixed_eval_target", eval_opponent)
            print(f"Fixed opponent added to pool", flush=True)

    # Best checkpoint tracking
    best_path = os.path.join(checkpoint_dir, "ppo_best.pt")
    best_vs_opponent = 0.0
    if os.path.exists(best_path):
        best_ckpt = torch.load(best_path, map_location="cpu", weights_only=True)
        best_vs_opponent = best_ckpt.get("best_vs_opponent", 0.0)
        print(f"Previous best: {best_vs_opponent:.1%} at iter {best_ckpt.get('iteration', '?')}", flush=True)

    last_win_rate = 0.0
    train_start = time.time()
    pool_version = 0

    for iteration in range(start_iteration, num_iterations + 1):
        t0 = time.time()

        # Collect rollouts
        if pool is not None and len(pool.opponents) > 0:
            n_self = int(games_per_iter * POOL_SELF_PLAY_RATIO)
            n_pool = games_per_iter - n_self

            # For pool games, sample opponents using smart weights
            pool_opponents = []
            for _ in range(n_pool):
                opp = pool.sample()
                if opp is not None:
                    pool_opponents.append(opp)

            buf_self = collect_rollouts(net, num_games=n_self, device=device)
            if pool_opponents:
                buf_pool = collect_rollouts(net, num_games=n_pool, device=device,
                                           opponent_pool=pool_opponents)
                # Merge
                buf_self.states.extend(buf_pool.states)
                buf_self.actions.extend(buf_pool.actions)
                buf_self.rewards.extend(buf_pool.rewards)
                buf_self.dones.extend(buf_pool.dones)
                buf_self.log_probs.extend(buf_pool.log_probs)
                buf_self.values.extend(buf_pool.values)
                buf_self.masks.extend(buf_pool.masks)

            buffer = buf_self
        else:
            buffer = collect_rollouts(net, num_games=games_per_iter, device=device)

        metrics = ppo_update(net, optimizer, buffer, device=device)
        elapsed = time.time() - t0

        # Pool management
        if pool is not None and iteration % POOL_CHECK_EVERY == 0:
            if pool.should_add_to_pool(net, threshold=0.55):
                pool_version += 1
                snapshot = PPONet(filters=filters, num_res_blocks=num_res_blocks, use_batch_norm=use_batch_norm).to(device)
                snapshot.load_state_dict(net.state_dict())
                snapshot.eval()
                pool.add(f"v{pool_version}_iter{iteration}", snapshot)
                print(f"  Pool: added v{pool_version} | {pool.summary()}", flush=True)
            else:
                print(f"  Pool: not strong enough to join | {pool.summary()}", flush=True)

        # Update pool win rates periodically
        if pool is not None and iteration % POOL_UPDATE_WIN_RATES_EVERY == 0:
            pool.update_win_rates(net)
            print(f"  Pool win rates updated: {pool.summary()}", flush=True)

        # TensorBoard
        writer.add_scalar("loss/total", metrics["total_loss"], iteration)
        writer.add_scalar("policy/entropy", metrics["entropy"], iteration)
        writer.add_scalar("perf/seconds", elapsed, iteration)

        if iteration % 10 == 0:
            wall = time.time() - train_start
            avg = wall / (iteration - start_iteration + 1)
            remaining = avg * (num_iterations - iteration)
            eta_m, eta_s = divmod(int(remaining), 60)
            eta_h, eta_m = divmod(eta_m, 60)
            print(
                f"[{iteration}] loss={metrics['total_loss']:.4f} "
                f"ent={metrics['entropy']:.3f} "
                f"time={elapsed:.1f}s "
                f"ETA={eta_h}h{eta_m:02d}m",
                flush=True,
            )

        # Evaluate
        if iteration % eval_every == 0:
            net.eval()

            wr, dr, lr_val, avg_len = evaluate_vs_random(
                net, num_games=EVAL_VS_RANDOM_GAMES, device=device
            )
            last_win_rate = wr
            writer.add_scalar("eval/win_rate", wr, iteration)
            print(f"  EVAL random: win={wr:.1%} loss={lr_val:.1%} avg_len={avg_len:.0f}", flush=True)

            if eval_opponent is not None:
                opp_wr, opp_dr, opp_lr, opp_len = evaluate_vs_opponent(
                    net, eval_opponent, num_games=EVAL_VS_OPPONENT_GAMES, device=device
                )
                writer.add_scalar("eval/vs_opponent", opp_wr, iteration)
                print(f"  EVAL target: win={opp_wr:.1%} draw={opp_dr:.1%} loss={opp_lr:.1%} len={opp_len:.0f}", flush=True)

                if opp_wr > best_vs_opponent:
                    best_vs_opponent = opp_wr
                    torch.save({
                        "iteration": iteration,
                        "model_state_dict": net.state_dict(),
                        "optimizer_state_dict": optimizer.state_dict(),
                        "best_vs_opponent": best_vs_opponent,
                        "exported": list(exported),
                    }, best_path)
                    print(f"  NEW BEST: {opp_wr:.1%} at iter {iteration}", flush=True)

            # Milestones
            for mi, name, min_wr in MILESTONES:
                if name not in exported and iteration >= mi and last_win_rate >= min_wr:
                    _export_milestone(net, name, iteration, last_win_rate, checkpoint_dir, filters, num_res_blocks)
                    exported.add(name)

            net.train()

        # Checkpoint
        if iteration % checkpoint_every == 0:
            path = os.path.join(checkpoint_dir, f"ppo_{iteration:05d}.pt")
            torch.save({
                "iteration": iteration,
                "model_state_dict": net.state_dict(),
                "optimizer_state_dict": optimizer.state_dict(),
                "exported": list(exported),
            }, path)
            print(f"  Checkpoint: {path}", flush=True)

    # Final
    final_path = os.path.join(checkpoint_dir, "ppo_final.pt")
    torch.save({
        "iteration": num_iterations,
        "model_state_dict": net.state_dict(),
        "optimizer_state_dict": optimizer.state_dict(),
        "exported": list(exported),
    }, final_path)

    for _, name, _ in MILESTONES:
        if name not in exported:
            _export_milestone(net, name, num_iterations, last_win_rate, checkpoint_dir, filters, num_res_blocks)
            exported.add(name)

    best_onnx = os.path.join(MODELS_DIR, "bot.onnx")
    export_onnx(final_path, best_onnx, filters=filters, num_res_blocks=num_res_blocks)

    total = time.time() - train_start
    h, r = divmod(int(total), 3600)
    m, s = divmod(r, 60)
    print(f"Done in {h}h{m:02d}m. Models: {sorted(exported)}", flush=True)
    writer.close()


def _export_milestone(net, name, iteration, win_rate, checkpoint_dir, filters, num_res_blocks):
    ckpt_path = os.path.join(checkpoint_dir, f"ppo_milestone_{name}.pt")
    torch.save({"iteration": iteration, "model_state_dict": net.state_dict()}, ckpt_path)
    onnx_path = os.path.join(MODELS_DIR, f"bot_{name}.onnx")
    export_onnx(ckpt_path, onnx_path, filters=filters, num_res_blocks=num_res_blocks)
    print(f"  MILESTONE '{name}' at iter {iteration} ({win_rate:.1%}): {onnx_path}", flush=True)


def best_device():
    if torch.backends.mps.is_available():
        return "mps"
    if torch.cuda.is_available():
        return "cuda"
    return "cpu"


if __name__ == "__main__":
    resume = sys.argv[1] if len(sys.argv) > 1 else None
    train_overnight(
        resume_from=resume,
        device=best_device(),
        use_opponent_pool=True,
        eval_opponent_checkpoint="checkpoints/ppo_eval_target.pt",
    )
