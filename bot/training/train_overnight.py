"""Overnight training: longer run with auto-export of difficulty-tagged models.

Exports ONNX models at milestones for easy/medium/hard bots.
Usage: uv run python train_overnight.py
"""

import copy
import os
import sys
import time

import torch
from torch.utils.tensorboard import SummaryWriter

from model import PPONet
from ppo import collect_rollouts, ppo_update
from evaluate import evaluate_vs_random, evaluate_vs_opponent
from export import export_onnx

MODELS_DIR = "../models"

MILESTONES = [
    (200, "easy", 0.0),
    (1000, "medium", 0.40),
    (3000, "hard", 0.70),
]

# Opponent pool settings
POOL_MAX_SIZE = 10
POOL_SAVE_EVERY = 100  # add current model to pool every N iterations
POOL_SELF_PLAY_RATIO = 0.5  # fraction of games that are self-play (rest use pool)


def train_overnight(
    num_iterations: int = 5000,
    games_per_iter: int = 128,
    eval_every: int = 25,
    checkpoint_every: int = 100,
    lr: float = 3e-4,
    checkpoint_dir: str = "checkpoints",
    log_dir: str = "runs/overnight",
    device: str = "cpu",
    resume_from: str = None,
    filters: int = 64,
    num_res_blocks: int = 0,
    use_opponent_pool: bool = False,
    eval_opponent_checkpoint: str = None,
    eval_opponent_filters: int = 64,
    eval_opponent_res_blocks: int = 0,
):
    os.makedirs(checkpoint_dir, exist_ok=True)
    os.makedirs(MODELS_DIR, exist_ok=True)
    writer = SummaryWriter(log_dir)

    net = PPONet(filters=filters, num_res_blocks=num_res_blocks).to(device)
    optimizer = torch.optim.Adam(net.parameters(), lr=lr)
    start_iteration = 1

    if resume_from and os.path.exists(resume_from):
        checkpoint = torch.load(resume_from, map_location=device, weights_only=True)
        net.load_state_dict(checkpoint["model_state_dict"])
        optimizer.load_state_dict(checkpoint["optimizer_state_dict"])
        start_iteration = checkpoint["iteration"] + 1
        print(f"Resumed from {resume_from} at iteration {start_iteration}", flush=True)

    num_params = sum(p.numel() for p in net.parameters())
    print(f"Overnight training: {num_iterations} iterations, {games_per_iter} games/iter", flush=True)
    print(f"Model: {filters} filters, {num_res_blocks} res blocks, {num_params:,} params", flush=True)
    print(f"Milestones: {[(m[0], m[1]) for m in MILESTONES]}", flush=True)
    print(f"Opponent pool: {'enabled' if use_opponent_pool else 'disabled'}", flush=True)
    print(f"Device: {device}", flush=True)

    # Opponent pool: list of past model snapshots
    opponent_pool = []
    if use_opponent_pool:
        # Seed the pool with the initial model
        initial_copy = PPONet(filters=filters, num_res_blocks=num_res_blocks).to(device)
        initial_copy.load_state_dict(net.state_dict())
        initial_copy.eval()
        opponent_pool.append(initial_copy)
        print(f"Opponent pool seeded (max {POOL_MAX_SIZE}, save every {POOL_SAVE_EVERY})", flush=True)

    # Load fixed eval opponent (e.g., previous training run's best model)
    eval_opponent = None
    if eval_opponent_checkpoint and os.path.exists(eval_opponent_checkpoint):
        eval_opponent = PPONet(filters=eval_opponent_filters, num_res_blocks=eval_opponent_res_blocks).to(device)
        ckpt = torch.load(eval_opponent_checkpoint, map_location=device, weights_only=True)
        eval_opponent.load_state_dict(ckpt["model_state_dict"])
        eval_opponent.eval()
        print(f"Eval opponent loaded: {eval_opponent_checkpoint}", flush=True)

    exported = set()
    last_win_rate = 0.0
    train_start = time.time()

    for iteration in range(start_iteration, num_iterations + 1):
        t0 = time.time()

        # Split games between self-play and opponent-pool play
        if opponent_pool and len(opponent_pool) > 0:
            n_self = int(games_per_iter * POOL_SELF_PLAY_RATIO)
            n_pool = games_per_iter - n_self

            buf_self = collect_rollouts(net, num_games=n_self, device=device)
            buf_pool = collect_rollouts(net, num_games=n_pool, device=device, opponent_pool=opponent_pool)

            # Merge buffers
            buffer = buf_self
            buffer.states.extend(buf_pool.states)
            buffer.actions.extend(buf_pool.actions)
            buffer.rewards.extend(buf_pool.rewards)
            buffer.dones.extend(buf_pool.dones)
            buffer.log_probs.extend(buf_pool.log_probs)
            buffer.values.extend(buf_pool.values)
        else:
            buffer = collect_rollouts(net, num_games=games_per_iter, device=device)

        metrics = ppo_update(net, optimizer, buffer, device=device)
        elapsed = time.time() - t0

        # Add current model to opponent pool periodically
        if use_opponent_pool and iteration % POOL_SAVE_EVERY == 0:
            snapshot = PPONet(filters=filters, num_res_blocks=num_res_blocks).to(device)
            snapshot.load_state_dict(net.state_dict())
            snapshot.eval()
            opponent_pool.append(snapshot)
            if len(opponent_pool) > POOL_MAX_SIZE:
                # Keep first (weakest) and evenly sample the rest
                opponent_pool = [opponent_pool[0]] + opponent_pool[-POOL_MAX_SIZE + 1:]
            print(f"  Pool updated: {len(opponent_pool)} opponents", flush=True)

        # Log to TensorBoard
        writer.add_scalar("loss/actor", metrics["actor_loss"], iteration)
        writer.add_scalar("loss/critic", metrics["critic_loss"], iteration)
        writer.add_scalar("loss/total", metrics["total_loss"], iteration)
        writer.add_scalar("policy/entropy", metrics["entropy"], iteration)
        writer.add_scalar("perf/transitions", len(buffer), iteration)
        writer.add_scalar("perf/seconds", elapsed, iteration)

        if iteration % 10 == 0:
            wall = time.time() - train_start
            avg_per_iter = wall / (iteration - start_iteration + 1)
            remaining = avg_per_iter * (num_iterations - iteration)
            eta_min, eta_sec = divmod(int(remaining), 60)
            eta_hr, eta_min = divmod(eta_min, 60)
            print(
                f"[{iteration}/{num_iterations}] "
                f"loss={metrics['total_loss']:.4f} "
                f"entropy={metrics['entropy']:.3f} "
                f"time={elapsed:.1f}s "
                f"ETA={eta_hr}h{eta_min:02d}m{eta_sec:02d}s",
                flush=True,
            )

        # Evaluate
        if iteration % eval_every == 0:
            win_rate, draw_rate, loss_rate, avg_length = evaluate_vs_random(
                net, num_games=100, device=device
            )
            writer.add_scalar("eval/win_rate", win_rate, iteration)
            writer.add_scalar("eval/draw_rate", draw_rate, iteration)
            writer.add_scalar("eval/loss_rate", loss_rate, iteration)
            writer.add_scalar("eval/avg_game_length", avg_length, iteration)
            last_win_rate = win_rate
            print(
                f"  EVAL: win={win_rate:.1%} draw={draw_rate:.1%} "
                f"loss={loss_rate:.1%} avg_len={avg_length:.0f}",
                flush=True,
            )

            # Evaluate against fixed eval opponent
            if eval_opponent is not None:
                opp_wr, opp_dr, opp_lr, opp_len = evaluate_vs_opponent(
                    net, eval_opponent, num_games=100, device=device
                )
                writer.add_scalar("eval/vs_opponent_win", opp_wr, iteration)
                writer.add_scalar("eval/vs_opponent_draw", opp_dr, iteration)
                writer.add_scalar("eval/vs_opponent_loss", opp_lr, iteration)
                print(
                    f"  VS PREV BEST: win={opp_wr:.1%} draw={opp_dr:.1%} "
                    f"loss={opp_lr:.1%} avg_len={opp_len:.0f}",
                    flush=True,
                )

            # Evaluate against latest pool opponent (shows self-improvement)
            if opponent_pool and len(opponent_pool) > 1:
                pool_wr, pool_dr, pool_lr, pool_len = evaluate_vs_opponent(
                    net, opponent_pool[-2], num_games=50, device=device
                )
                writer.add_scalar("eval/vs_pool_win", pool_wr, iteration)
                print(
                    f"  VS POOL[-2]: win={pool_wr:.1%} draw={pool_dr:.1%} "
                    f"loss={pool_lr:.1%}",
                    flush=True,
                )

            # Check milestones after each eval
            for milestone_iter, name, min_wr in MILESTONES:
                if name in exported:
                    continue
                if iteration >= milestone_iter and last_win_rate >= min_wr:
                    export_milestone(net, name, iteration, last_win_rate, checkpoint_dir, filters, num_res_blocks)
                    exported.add(name)

        # Checkpoint
        if iteration % checkpoint_every == 0:
            path = os.path.join(checkpoint_dir, f"ppo_{iteration:05d}.pt")
            torch.save({
                "iteration": iteration,
                "model_state_dict": net.state_dict(),
                "optimizer_state_dict": optimizer.state_dict(),
            }, path)
            print(f"  Saved checkpoint: {path}", flush=True)

    # Final save + export any remaining milestones
    final_path = os.path.join(checkpoint_dir, "ppo_final.pt")
    torch.save({
        "iteration": num_iterations,
        "model_state_dict": net.state_dict(),
        "optimizer_state_dict": optimizer.state_dict(),
    }, final_path)

    for _, name, _ in MILESTONES:
        if name not in exported:
            export_milestone(net, name, num_iterations, last_win_rate, checkpoint_dir, filters, num_res_blocks)
            exported.add(name)

    # Always export "bot.onnx" as the strongest model
    best_onnx = os.path.join(MODELS_DIR, "bot.onnx")
    export_onnx(final_path, best_onnx, filters=filters, num_res_blocks=num_res_blocks)
    print(f"Exported best model: {best_onnx}", flush=True)

    total_time = time.time() - train_start
    h, remainder = divmod(int(total_time), 3600)
    m, s = divmod(remainder, 60)
    print(f"Training complete in {h}h{m:02d}m{s:02d}s", flush=True)
    print(f"Models exported: {sorted(exported)}", flush=True)

    writer.close()


def export_milestone(net, name, iteration, win_rate, checkpoint_dir, filters=64, num_res_blocks=0):
    ckpt_path = os.path.join(checkpoint_dir, f"ppo_{name}.pt")
    torch.save({
        "iteration": iteration,
        "model_state_dict": net.state_dict(),
    }, ckpt_path)

    onnx_path = os.path.join(MODELS_DIR, f"bot_{name}.onnx")
    export_onnx(ckpt_path, onnx_path, filters=filters, num_res_blocks=num_res_blocks)
    print(
        f"  MILESTONE '{name}' exported at iter {iteration} "
        f"(win_rate={win_rate:.1%}): {onnx_path}",
        flush=True,
    )


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
        eval_opponent_checkpoint="checkpoints/ppo_hard.pt",
    )
