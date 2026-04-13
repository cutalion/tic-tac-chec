"""AlphaZero-style training: MCTS self-play + policy/value targets.

Replaces PPO's rollout collection with MCTS-guided self-play.
Each position produces (state, policy_target, game_outcome) — the network
learns to predict what MCTS would do (policy) and who wins (value).

Opponent pool is kept for evaluation only (not training).

Usage: uv run python train_alphazero.py [checkpoint_to_resume]
"""

import copy
import os
import sys
import time

import numpy as np
import torch
import torch.nn.functional as F
from torch.utils.tensorboard import SummaryWriter

from model import PPONet
from mcts_collect_fast import collect_alphazero_data_fast as collect_alphazero_data
from evaluate_fast import evaluate_vs_random, evaluate_vs_opponent
from export import export_onnx

MODELS_DIR = "../models"

MILESTONES = [
    (50, "easy", 0.0),
    (200, "medium", 0.40),
    (500, "hard", 0.70),
]

# Opponent pool settings (evaluation only)
POOL_MAX_SIZE = 10
POOL_SAVE_EVERY = 100


def alphazero_update(net, optimizer, states, policy_targets, value_targets, device, num_epochs=10, batch_size=256):
    """Train the network on MCTS-generated data.

    Loss = cross_entropy(predicted_policy, mcts_policy) + MSE(predicted_value, game_outcome)

    Returns metrics dict.
    """
    net.train()

    states_t = torch.tensor(states, dtype=torch.float32, device=device)
    policy_t = torch.tensor(policy_targets, dtype=torch.float32, device=device)
    value_t = torch.tensor(value_targets, dtype=torch.float32, device=device)

    n = len(states)
    total_policy_loss = 0.0
    total_value_loss = 0.0
    total_loss = 0.0
    num_batches = 0

    for _ in range(num_epochs):
        indices = torch.randperm(n, device=device)

        for start in range(0, n, batch_size):
            end = min(start + batch_size, n)
            idx = indices[start:end]

            batch_states = states_t[idx]
            batch_policies = policy_t[idx]
            batch_values = value_t[idx]

            logits, values = net(batch_states)
            values = values.squeeze(-1)

            # Policy loss: cross-entropy with MCTS policy targets
            # = -sum(pi * log(p)) where pi is MCTS target, p is network output
            log_probs = F.log_softmax(logits, dim=-1)
            policy_loss = -(batch_policies * log_probs).sum(dim=-1).mean()

            # Value loss: MSE between predicted value and game outcome
            value_loss = F.mse_loss(values, batch_values)

            loss = policy_loss + value_loss

            optimizer.zero_grad()
            loss.backward()
            # Gradient clipping (same as opponent pool branch)
            torch.nn.utils.clip_grad_norm_(net.parameters(), max_norm=0.5)
            optimizer.step()

            total_policy_loss += policy_loss.item()
            total_value_loss += value_loss.item()
            total_loss += loss.item()
            num_batches += 1

    return {
        "policy_loss": total_policy_loss / max(num_batches, 1),
        "value_loss": total_value_loss / max(num_batches, 1),
        "total_loss": total_loss / max(num_batches, 1),
    }


def train_alphazero(
    num_iterations: int = 1000,
    games_per_iter: int = 64,
    num_simulations: int = 25,
    eval_every: int = 25,
    checkpoint_every: int = 50,
    lr: float = 1e-3,
    num_epochs: int = 10,
    batch_size: int = 256,
    checkpoint_dir: str = "checkpoints",
    log_dir: str = "runs/alphazero",
    device: str = "cpu",
    resume_from: str = None,
    filters: int = 64,
    num_res_blocks: int = 0,
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
        if "optimizer_state_dict" in checkpoint:
            optimizer.load_state_dict(checkpoint["optimizer_state_dict"])
        start_iteration = checkpoint.get("iteration", 0) + 1
        print(f"Resumed from {resume_from} at iteration {start_iteration}", flush=True)

    num_params = sum(p.numel() for p in net.parameters())
    print(f"AlphaZero training: {num_iterations} iters, {games_per_iter} games/iter, {num_simulations} MCTS sims/move", flush=True)
    print(f"Model: {filters} filters, {num_res_blocks} res blocks, {num_params:,} params", flush=True)
    print(f"SGD: {num_epochs} epochs, batch_size={batch_size}, lr={lr}", flush=True)
    print(f"Milestones: {[(m[0], m[1]) for m in MILESTONES]}", flush=True)
    print(f"Device: {device}", flush=True)

    # Opponent pool (evaluation only)
    opponent_pool = []
    initial_copy = PPONet(filters=filters, num_res_blocks=num_res_blocks).to(device)
    initial_copy.load_state_dict(net.state_dict())
    initial_copy.eval()
    opponent_pool.append(initial_copy)

    # Load fixed eval opponent
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

        # --- Collect MCTS self-play data ---
        net.eval()
        states, policy_targets, value_targets = collect_alphazero_data(
            net, games_per_iter, num_simulations, device=device
        )
        collect_time = time.time() - t0

        # --- Train on collected data ---
        t1 = time.time()
        metrics = alphazero_update(
            net, optimizer, states, policy_targets, value_targets,
            device=device, num_epochs=num_epochs, batch_size=batch_size,
        )
        train_time = time.time() - t1
        elapsed = time.time() - t0

        # Log to TensorBoard
        writer.add_scalar("loss/policy", metrics["policy_loss"], iteration)
        writer.add_scalar("loss/value", metrics["value_loss"], iteration)
        writer.add_scalar("loss/total", metrics["total_loss"], iteration)
        writer.add_scalar("perf/examples", len(states), iteration)
        writer.add_scalar("perf/collect_seconds", collect_time, iteration)
        writer.add_scalar("perf/train_seconds", train_time, iteration)

        # Add to opponent pool periodically
        if iteration % POOL_SAVE_EVERY == 0:
            snapshot = PPONet(filters=filters, num_res_blocks=num_res_blocks).to(device)
            snapshot.load_state_dict(net.state_dict())
            snapshot.eval()
            opponent_pool.append(snapshot)
            if len(opponent_pool) > POOL_MAX_SIZE:
                opponent_pool = [opponent_pool[0]] + opponent_pool[-POOL_MAX_SIZE + 1:]
            print(f"  Pool updated: {len(opponent_pool)} opponents", flush=True)

        if iteration % 10 == 0:
            wall = time.time() - train_start
            avg_per_iter = wall / (iteration - start_iteration + 1)
            remaining = avg_per_iter * (num_iterations - iteration)
            eta_min, eta_sec = divmod(int(remaining), 60)
            eta_hr, eta_min = divmod(eta_min, 60)
            print(
                f"[{iteration}/{num_iterations}] "
                f"loss={metrics['total_loss']:.4f} (p={metrics['policy_loss']:.4f} v={metrics['value_loss']:.4f}) "
                f"examples={len(states)} "
                f"collect={collect_time:.1f}s train={train_time:.1f}s "
                f"ETA={eta_hr}h{eta_min:02d}m{eta_sec:02d}s",
                flush=True,
            )

        # Evaluate
        if iteration % eval_every == 0:
            net.eval()
            win_rate, draw_rate, loss_rate, avg_length = evaluate_vs_random(
                net, num_games=100, device=device
            )
            writer.add_scalar("eval/win_rate", win_rate, iteration)
            writer.add_scalar("eval/draw_rate", draw_rate, iteration)
            writer.add_scalar("eval/loss_rate", loss_rate, iteration)
            writer.add_scalar("eval/avg_game_length", avg_length, iteration)
            last_win_rate = win_rate
            print(
                f"  EVAL vs random: win={win_rate:.1%} draw={draw_rate:.1%} "
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
                    f"  EVAL vs fixed: win={opp_wr:.1%} draw={opp_dr:.1%} "
                    f"loss={opp_lr:.1%} avg_len={opp_len:.0f}",
                    flush=True,
                )

            # Evaluate against latest pool opponent
            if len(opponent_pool) > 1:
                pool_wr, pool_dr, pool_lr, pool_len = evaluate_vs_opponent(
                    net, opponent_pool[-2], num_games=50, device=device
                )
                writer.add_scalar("eval/vs_pool_win", pool_wr, iteration)
                print(
                    f"  EVAL vs pool[-2]: win={pool_wr:.1%} draw={pool_dr:.1%} "
                    f"loss={pool_lr:.1%}",
                    flush=True,
                )

            # Check milestones
            for milestone_iter, name, min_wr in MILESTONES:
                if name in exported:
                    continue
                if iteration >= milestone_iter and last_win_rate >= min_wr:
                    export_milestone(net, name, iteration, last_win_rate, checkpoint_dir, filters, num_res_blocks)
                    exported.add(name)

        # Checkpoint
        if iteration % checkpoint_every == 0:
            path = os.path.join(checkpoint_dir, f"az_{iteration:05d}.pt")
            torch.save({
                "iteration": iteration,
                "model_state_dict": net.state_dict(),
                "optimizer_state_dict": optimizer.state_dict(),
            }, path)
            print(f"  Saved checkpoint: {path}", flush=True)

    # Final save
    final_path = os.path.join(checkpoint_dir, "az_final.pt")
    torch.save({
        "iteration": num_iterations,
        "model_state_dict": net.state_dict(),
        "optimizer_state_dict": optimizer.state_dict(),
    }, final_path)

    for _, name, _ in MILESTONES:
        if name not in exported:
            export_milestone(net, name, num_iterations, last_win_rate, checkpoint_dir, filters, num_res_blocks)
            exported.add(name)

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
    ckpt_path = os.path.join(checkpoint_dir, f"az_{name}.pt")
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
    train_alphazero(
        resume_from=resume,
        device=best_device(),
        eval_opponent_checkpoint="checkpoints/ppo_hard.pt",
    )
