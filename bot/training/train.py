"""Training entry point: self-play loop with logging and checkpointing."""

import os
import time

import torch
from torch.utils.tensorboard import SummaryWriter

from model import PPONet
from ppo import collect_rollouts, ppo_update
from evaluate import evaluate_vs_random


def train(
    num_iterations: int = 500,
    games_per_iter: int = 20,
    eval_every: int = 25,
    checkpoint_every: int = 50,
    lr: float = 3e-4,
    checkpoint_dir: str = "checkpoints",
    log_dir: str = "runs",
    device: str = "cpu",
):
    os.makedirs(checkpoint_dir, exist_ok=True)
    writer = SummaryWriter(log_dir)

    net = PPONet().to(device)
    optimizer = torch.optim.Adam(net.parameters(), lr=lr)

    print(f"Starting training: {num_iterations} iterations, {games_per_iter} games/iter", flush=True)
    print(f"Eval every {eval_every} iters, checkpoint every {checkpoint_every} iters", flush=True)
    print(f"Device: {device}", flush=True)

    train_start = time.time()

    for iteration in range(1, num_iterations + 1):
        t0 = time.time()

        # Collect self-play rollouts
        buffer = collect_rollouts(net, num_games=games_per_iter, device=device)

        # PPO update
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
            avg_per_iter = wall / iteration
            remaining = avg_per_iter * (num_iterations - iteration)
            eta_min, eta_sec = divmod(int(remaining), 60)
            print(
                f"[{iteration}/{num_iterations}] "
                f"loss={metrics['total_loss']:.4f} "
                f"entropy={metrics['entropy']:.3f} "
                f"transitions={len(buffer)} "
                f"time={elapsed:.1f}s "
                f"ETA={eta_min}m{eta_sec:02d}s",
                flush=True,
            )

        # Evaluate vs random
        if iteration % eval_every == 0:
            win_rate, draw_rate, loss_rate, avg_length = evaluate_vs_random(
                net, num_games=100, device=device
            )
            writer.add_scalar("eval/win_rate", win_rate, iteration)
            writer.add_scalar("eval/draw_rate", draw_rate, iteration)
            writer.add_scalar("eval/loss_rate", loss_rate, iteration)
            writer.add_scalar("eval/avg_game_length", avg_length, iteration)
            print(
                f"  EVAL: win={win_rate:.1%} draw={draw_rate:.1%} "
                f"loss={loss_rate:.1%} avg_len={avg_length:.0f}",
                flush=True,
            )

        # Checkpoint
        if iteration % checkpoint_every == 0:
            path = os.path.join(checkpoint_dir, f"ppo_{iteration:05d}.pt")
            torch.save({
                "iteration": iteration,
                "model_state_dict": net.state_dict(),
                "optimizer_state_dict": optimizer.state_dict(),
            }, path)
            print(f"  Saved checkpoint: {path}", flush=True)

    # Final save
    final_path = os.path.join(checkpoint_dir, "ppo_final.pt")
    torch.save({
        "iteration": num_iterations,
        "model_state_dict": net.state_dict(),
        "optimizer_state_dict": optimizer.state_dict(),
    }, final_path)
    total_time = time.time() - train_start
    m, s = divmod(int(total_time), 60)
    print(f"Training complete in {m}m{s:02d}s. Final model: {final_path}", flush=True)

    writer.close()


if __name__ == "__main__":
    train()
