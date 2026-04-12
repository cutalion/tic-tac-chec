"""Overnight training: longer run with auto-export of difficulty-tagged models.

Exports ONNX models at milestones for easy/medium/hard bots.
Usage: uv run python train_overnight.py
"""

import os
import sys
import time

import torch
from torch.utils.tensorboard import SummaryWriter

from model import PPONet
from ppo import collect_rollouts, ppo_update
from evaluate import evaluate_vs_random
from export import export_onnx

MODELS_DIR = "../models"

# Milestone definitions: (iteration, name, min_win_rate_to_export)
# Models are exported when the milestone iteration is reached AND win rate >= threshold.
# If win rate is below threshold, training continues and the model is exported anyway
# at the next eval that crosses the threshold (or at the milestone, whichever comes first).
MILESTONES = [
    (200, "easy", 0.0),       # early model, plays legal moves but poorly
    (1000, "medium", 0.40),   # decent, beats random ~40-60%
    (3000, "hard", 0.70),     # strong, beats random >70%
]


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
    print(f"Device: {device}", flush=True)

    exported = set()
    last_win_rate = 0.0
    train_start = time.time()

    for iteration in range(start_iteration, num_iterations + 1):
        t0 = time.time()

        buffer = collect_rollouts(net, num_games=games_per_iter, device=device)
        metrics = ppo_update(net, optimizer, buffer, device=device)
        elapsed = time.time() - t0

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
    # Save checkpoint
    ckpt_path = os.path.join(checkpoint_dir, f"ppo_{name}.pt")
    torch.save({
        "iteration": iteration,
        "model_state_dict": net.state_dict(),
    }, ckpt_path)

    # Export ONNX
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
    train_overnight(resume_from=resume, device=best_device())
