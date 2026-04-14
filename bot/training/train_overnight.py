"""Overnight training: longer run with auto-export of difficulty-tagged models.

Exports ONNX models at milestones for easy/medium/hard bots.
Usage: uv run python train_overnight.py [checkpoint_to_resume]
"""

import copy
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

MODELS_DIR = "../models"

MILESTONES = [
    (200, "easy", 0.0),
    (1000, "medium", 0.40),
    (3000, "hard", 0.70),
]

# Opponent pool settings
POOL_MAX_SIZE = 10
POOL_SAVE_EVERY = 100
POOL_SELF_PLAY_RATIO = 0.5
POOL_ADD_FIXED_AFTER_WIN_RATE = 0.60  # delay adding fixed opponent until agent is this strong vs random

# Eval settings
EVAL_VS_OPPONENT_GAMES = 400  # enough for ±3% precision
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

    # Restore exported milestones from checkpoint
    exported = set()

    if resume_from and os.path.exists(resume_from):
        checkpoint = torch.load(resume_from, map_location=device, weights_only=True)
        net.load_state_dict(checkpoint["model_state_dict"])
        optimizer.load_state_dict(checkpoint["optimizer_state_dict"])
        start_iteration = checkpoint["iteration"] + 1
        exported = set(checkpoint.get("exported", []))
        print(f"Resumed from {resume_from} at iteration {start_iteration}", flush=True)
        if exported:
            print(f"Previously exported milestones: {sorted(exported)}", flush=True)

    num_params = sum(p.numel() for p in net.parameters())
    print(f"Overnight training: {num_iterations} iterations, {games_per_iter} games/iter", flush=True)
    print(f"Model: {filters} filters, {num_res_blocks} res blocks, {num_params:,} params", flush=True)
    print(f"Milestones: {[(m[0], m[1]) for m in MILESTONES]}", flush=True)
    print(f"Opponent pool: {'enabled' if use_opponent_pool else 'disabled'}", flush=True)
    print(f"Device: {device}", flush=True)

    # Opponent pool: list of past model snapshots
    opponent_pool = []
    if use_opponent_pool:
        initial_copy = PPONet(filters=filters, num_res_blocks=num_res_blocks).to(device)
        initial_copy.load_state_dict(net.state_dict())
        initial_copy.eval()
        opponent_pool.append(initial_copy)
        print(f"Opponent pool seeded (max {POOL_MAX_SIZE}, save every {POOL_SAVE_EVERY})", flush=True)

    # FIX 1: Load eval opponent into a protected copy (never overwritten by milestones)
    eval_opponent = None
    fixed_opponent_added_to_pool = False
    eval_target_path = os.path.join(checkpoint_dir, "ppo_eval_target.pt")

    if eval_opponent_checkpoint and os.path.exists(eval_opponent_checkpoint):
        # Copy to a protected location that milestones cannot overwrite
        if not os.path.exists(eval_target_path) or eval_opponent_checkpoint != eval_target_path:
            shutil.copy2(eval_opponent_checkpoint, eval_target_path)
            print(f"Eval opponent copied to {eval_target_path} (protected)", flush=True)

        eval_opponent = PPONet(filters=eval_opponent_filters, num_res_blocks=eval_opponent_res_blocks).to(device)
        ckpt = torch.load(eval_target_path, map_location=device, weights_only=True)
        eval_opponent.load_state_dict(ckpt["model_state_dict"])
        eval_opponent.eval()
        print(f"Eval opponent loaded from {eval_target_path}", flush=True)

    # FIX 3: Load best score from existing best checkpoint
    best_path = os.path.join(checkpoint_dir, "ppo_best.pt")
    best_vs_opponent = 0.0
    if os.path.exists(best_path):
        best_ckpt = torch.load(best_path, map_location="cpu", weights_only=True)
        best_vs_opponent = best_ckpt.get("best_vs_opponent", 0.0)
        print(f"Loaded previous best: {best_vs_opponent:.1%} at iter {best_ckpt.get('iteration', '?')}", flush=True)

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
            buffer.masks.extend(buf_pool.masks)
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

            # FIX 6: Delay adding fixed opponent until agent is strong enough
            if eval_opponent is not None and not fixed_opponent_added_to_pool:
                if last_win_rate >= POOL_ADD_FIXED_AFTER_WIN_RATE:
                    opponent_pool.append(eval_opponent)
                    fixed_opponent_added_to_pool = True
                    print(f"  Fixed opponent added to pool (win_rate={last_win_rate:.1%} >= {POOL_ADD_FIXED_AFTER_WIN_RATE:.0%})", flush=True)

            if len(opponent_pool) > POOL_MAX_SIZE:
                # Keep permanent members and latest self-play snapshots
                n_permanent = 1 + (1 if fixed_opponent_added_to_pool else 0)
                permanent = opponent_pool[:n_permanent]
                recent = opponent_pool[n_permanent:][-POOL_MAX_SIZE + n_permanent:]
                opponent_pool = permanent + recent
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
            net.eval()  # FIX: switch to eval mode for evaluation
            win_rate, draw_rate, loss_rate, avg_length = evaluate_vs_random(
                net, num_games=EVAL_VS_RANDOM_GAMES, device=device
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

            # FIX 4: Evaluate against fixed eval opponent with more games and greedy play
            if eval_opponent is not None:
                opp_wr, opp_dr, opp_lr, opp_len = evaluate_vs_opponent(
                    net, eval_opponent, num_games=EVAL_VS_OPPONENT_GAMES, device=device
                )
                writer.add_scalar("eval/vs_opponent_win", opp_wr, iteration)
                writer.add_scalar("eval/vs_opponent_draw", opp_dr, iteration)
                writer.add_scalar("eval/vs_opponent_loss", opp_lr, iteration)
                print(
                    f"  VS PREV BEST: win={opp_wr:.1%} draw={opp_dr:.1%} "
                    f"loss={opp_lr:.1%} avg_len={opp_len:.0f}",
                    flush=True,
                )

                # Save best checkpoint based on vs-opponent win rate
                if opp_wr > best_vs_opponent:
                    best_vs_opponent = opp_wr
                    torch.save({
                        "iteration": iteration,
                        "model_state_dict": net.state_dict(),
                        "optimizer_state_dict": optimizer.state_dict(),
                        "best_vs_opponent": best_vs_opponent,
                        "exported": list(exported),
                    }, best_path)
                    print(
                        f"  NEW BEST: {opp_wr:.1%} vs opponent at iter {iteration} → {best_path}",
                        flush=True,
                    )

            # Evaluate against latest pool opponent
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

            net.train()  # Switch back to train mode

            # FIX 2: Check milestones with persisted exported set
            for milestone_iter, name, min_wr in MILESTONES:
                if name in exported:
                    continue
                if iteration >= milestone_iter and last_win_rate >= min_wr:
                    export_milestone(net, name, iteration, last_win_rate, checkpoint_dir, filters, num_res_blocks)
                    exported.add(name)

        # Checkpoint (FIX 2: persist exported set)
        if iteration % checkpoint_every == 0:
            path = os.path.join(checkpoint_dir, f"ppo_{iteration:05d}.pt")
            torch.save({
                "iteration": iteration,
                "model_state_dict": net.state_dict(),
                "optimizer_state_dict": optimizer.state_dict(),
                "exported": list(exported),
            }, path)
            print(f"  Saved checkpoint: {path}", flush=True)

    # Final save
    final_path = os.path.join(checkpoint_dir, "ppo_final.pt")
    torch.save({
        "iteration": num_iterations,
        "model_state_dict": net.state_dict(),
        "optimizer_state_dict": optimizer.state_dict(),
        "exported": list(exported),
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
    # FIX 1: Use milestone-specific names that cannot collide with eval target
    ckpt_path = os.path.join(checkpoint_dir, f"ppo_milestone_{name}.pt")
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
        eval_opponent_checkpoint="checkpoints/ppo_eval_target.pt",
    )
