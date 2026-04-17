"""AlphaZero training: MCTS self-play → replay buffer → policy/value training.

Key differences from the previous attempt that failed (4.5h, worse than PPO):
1. Replay buffer — reuse positions across iterations (was throwing data away)
2. Batched GPU inference — mcts_collect_fast batches NN calls across games
3. More MCTS sims — 200 per move (was 25)
4. Deeper network — 4 res blocks (was 0)

Architecture matches the AlphaZero paper:
- Self-play with MCTS generates (state, policy_target, game_outcome)
- Policy target = normalized MCTS visit counts
- Train network to predict MCTS policy (cross-entropy) and game outcome (MSE)
- Latest network parameters used for self-play (no gating)

Usage:
    uv run python train_alphazero.py
    uv run python train_alphazero.py checkpoints_az/az_00500.pt   # resume
"""

import os
import sys
import time
import torch.multiprocessing as mp

import numpy as np
import torch
import torch.nn.functional as F
from torch.utils.tensorboard import SummaryWriter

from model import PPONet
from mcts_collect_fast import collect_alphazero_data_fast
from evaluate_fast import evaluate_vs_random, evaluate_vs_opponent
from export import export_onnx
from replay_buffer import ReplayBuffer

MODELS_DIR = "../models"

MILESTONES = [
    (50, "easy", 0.0),
    (200, "medium", 0.40),
    (500, "hard", 0.70),
]


def _selfplay_worker(state_dict, games_per_batch, num_simulations, filters,
                      num_res_blocks, use_batch_norm, time_budget, result_queue):
    """Worker process: play MCTS self-play games on CPU within a time budget."""
    torch.set_num_threads(2)
    net = PPONet(filters=filters, num_res_blocks=num_res_blocks, use_batch_norm=use_batch_norm)
    net.load_state_dict(state_dict)
    net.eval()

    all_states, all_policies, all_values = [], [], []
    games_played = 0
    deadline = time.time() + time_budget

    while time.time() < deadline:
        states, policies, values = collect_alphazero_data_fast(
            net, games_per_batch, num_simulations, device="cpu",
        )
        all_states.append(states)
        all_policies.append(policies)
        all_values.append(values)
        games_played += games_per_batch

    result_queue.put((
        np.concatenate(all_states),
        np.concatenate(all_policies),
        np.concatenate(all_values),
        games_played,
    ))


def _run_parallel_selfplay(net, num_workers, selfplay_batch, num_simulations,
                            filters, num_res_blocks, use_batch_norm, selfplay_seconds):
    """Spawn parallel CPU workers for self-play. Returns (states, policies, values, games)."""
    state_dict_cpu = {k: v.cpu() for k, v in net.state_dict().items()}
    result_queue = mp.Queue()
    workers = []

    for _ in range(num_workers):
        p = mp.Process(target=_selfplay_worker, args=(
            state_dict_cpu, selfplay_batch, num_simulations,
            filters, num_res_blocks, use_batch_norm,
            selfplay_seconds, result_queue,
        ))
        p.start()
        workers.append(p)

    return workers, result_queue


def _collect_selfplay_results(workers, result_queue, num_workers):
    """Collect results from all workers and join processes."""
    all_states, all_policies, all_values = [], [], []
    total_games = 0

    for _ in range(num_workers):
        states, policies, values, games = result_queue.get(timeout=600)
        all_states.append(states)
        all_policies.append(policies)
        all_values.append(values)
        total_games += games

    for w in workers:
        w.join(timeout=10)

    return (
        np.concatenate(all_states),
        np.concatenate(all_policies),
        np.concatenate(all_values),
        total_games,
    )


def alphazero_train_step(net, optimizer, states, policies, values):
    """One training step on a minibatch from the replay buffer.

    Loss = cross_entropy(policy, mcts_target) + MSE(value, outcome)
    """
    logits, v = net(states)
    v = v.squeeze(-1)

    log_probs = F.log_softmax(logits, dim=-1)
    policy_loss = -(policies * log_probs).sum(dim=-1).mean()
    value_loss = F.mse_loss(v, values)
    loss = policy_loss + value_loss

    optimizer.zero_grad()
    loss.backward()
    torch.nn.utils.clip_grad_norm_(net.parameters(), max_norm=1.0)
    optimizer.step()

    return policy_loss.item(), value_loss.item(), loss.item()


def train_alphazero(
    num_iterations: int = 1000,
    selfplay_seconds: int = 60,
    selfplay_batch: int = 4,
    num_simulations: int = 400,
    num_workers: int = 4,
    replay_buffer_size: int = 100_000,
    min_buffer_size: int = 500,
    train_steps_per_iter: int = 100,
    batch_size: int = 256,
    lr: float = 1e-3,
    lr_milestones: tuple = (300, 600),
    lr_gamma: float = 0.1,
    weight_decay: float = 1e-4,
    filters: int = 64,
    num_res_blocks: int = 4,
    use_batch_norm: bool = False,
    eval_every: int = 25,
    checkpoint_every: int = 50,
    checkpoint_dir: str = "checkpoints_az",
    log_dir: str = "runs/alphazero_v2",
    device: str = "cpu",
    resume_from: str = None,
    eval_opponent_path: str = None,
    eval_opponent_filters: int = 64,
    eval_opponent_res_blocks: int = 0,
):
    os.makedirs(checkpoint_dir, exist_ok=True)
    os.makedirs(MODELS_DIR, exist_ok=True)

    # --- Network + optimizer ---
    net = PPONet(filters=filters, num_res_blocks=num_res_blocks, use_batch_norm=use_batch_norm).to(device)
    optimizer = torch.optim.Adam(net.parameters(), lr=lr, weight_decay=weight_decay)
    # --- Replay buffer ---
    replay_buffer = ReplayBuffer(max_size=replay_buffer_size)
    buffer_path = os.path.join(checkpoint_dir, "replay_buffer.npz")

    # --- State ---
    start_iteration = 1
    exported = set()
    best_vs_opponent = 0.0
    total_games_played = 0

    # --- Resume ---
    if resume_from and os.path.exists(resume_from):
        ckpt = torch.load(resume_from, map_location=device, weights_only=True)
        net.load_state_dict(ckpt["model_state_dict"])
        if "optimizer_state_dict" in ckpt:
            optimizer.load_state_dict(ckpt["optimizer_state_dict"])
        # Override LR from checkpoint with the requested LR (enables warm restarts)
        for pg in optimizer.param_groups:
            pg["lr"] = lr
        start_iteration = ckpt.get("iteration", 0) + 1
        exported = set(ckpt.get("exported", []))
        best_vs_opponent = ckpt.get("best_vs_opponent", 0.0)
        total_games_played = ckpt.get("total_games_played", 0)

        # Load replay buffer
        if os.path.exists(buffer_path):
            replay_buffer.load(buffer_path)
            print(f"Replay buffer loaded: {len(replay_buffer)} positions", flush=True)

        print(f"Resumed from {resume_from} at iteration {start_iteration}", flush=True)

    # Create scheduler after determining start_iteration (avoids step-before-optimizer warning)
    scheduler = torch.optim.lr_scheduler.MultiStepLR(
        optimizer, milestones=list(lr_milestones), gamma=lr_gamma,
        last_epoch=start_iteration - 2 if start_iteration > 1 else -1,
    )

    # --- Eval opponent ---
    eval_opponent = None
    if eval_opponent_path and os.path.exists(eval_opponent_path):
        eval_opponent = PPONet(
            filters=eval_opponent_filters,
            num_res_blocks=eval_opponent_res_blocks,
        ).to(device)
        ckpt = torch.load(eval_opponent_path, map_location=device, weights_only=True)
        eval_opponent.load_state_dict(ckpt["model_state_dict"])
        eval_opponent.eval()
        print(f"Eval opponent: {eval_opponent_path}", flush=True)

    # --- Print config ---
    num_params = sum(p.numel() for p in net.parameters())
    print(f"AlphaZero training", flush=True)
    print(f"  Model: {filters}f, {num_res_blocks} res blocks, {'BN' if use_batch_norm else 'no BN'}, {num_params:,} params", flush=True)
    print(f"  Self-play: {selfplay_seconds}s budget, {num_workers} workers × {selfplay_batch} games/batch, {num_simulations} MCTS sims/move", flush=True)
    print(f"  Training: {train_steps_per_iter} steps/iter, batch={batch_size}, lr={lr}", flush=True)
    print(f"  Buffer: {replay_buffer_size:,} max, {min_buffer_size:,} min before training", flush=True)
    print(f"  LR decay: ×{lr_gamma} at iterations {lr_milestones}", flush=True)
    print(f"  Device: {device}", flush=True)
    print(flush=True)

    writer = SummaryWriter(log_dir)
    train_start = time.time()
    last_win_rate = 0.0

    for iteration in range(start_iteration, num_iterations + 1):
        iter_start = time.time()

        # === 1. START SELF-PLAY WORKERS (parallel, CPU) ===
        net.eval()
        workers, result_queue = _run_parallel_selfplay(
            net, num_workers, selfplay_batch, num_simulations,
            filters, num_res_blocks, use_batch_norm, selfplay_seconds,
        )

        # === 2. TRAIN on buffer while workers run (MPS/GPU) ===
        train_time = 0.0
        avg_p_loss = avg_v_loss = avg_loss = 0.0
        if len(replay_buffer) >= min_buffer_size:
            net.train()
            train_start_t = time.time()
            total_p_loss = 0.0
            total_v_loss = 0.0
            total_loss = 0.0

            for _ in range(train_steps_per_iter):
                s, p, v = replay_buffer.sample(batch_size)
                s_t = torch.from_numpy(s).to(device)
                p_t = torch.from_numpy(p).to(device)
                v_t = torch.from_numpy(v).to(device)

                p_loss, v_loss, loss = alphazero_train_step(net, optimizer, s_t, p_t, v_t)
                total_p_loss += p_loss
                total_v_loss += v_loss
                total_loss += loss

            train_time = time.time() - train_start_t
            avg_p_loss = total_p_loss / train_steps_per_iter
            avg_v_loss = total_v_loss / train_steps_per_iter
            avg_loss = total_loss / train_steps_per_iter
            net.eval()

        # === 3. COLLECT self-play results ===
        states, policies, values, games_this_iter = _collect_selfplay_results(
            workers, result_queue, num_workers,
        )
        replay_buffer.add(states, policies, values)
        total_games_played += games_this_iter
        selfplay_time = time.time() - iter_start

        num_positions = len(states)
        avg_game_len = num_positions / games_this_iter if games_this_iter else 0
        writer.add_scalar("selfplay/positions", num_positions, iteration)
        writer.add_scalar("selfplay/games", games_this_iter, iteration)
        writer.add_scalar("selfplay/avg_game_length", avg_game_len, iteration)
        writer.add_scalar("selfplay/seconds", selfplay_time, iteration)
        writer.add_scalar("selfplay/total_games", total_games_played, iteration)
        writer.add_scalar("buffer/size", len(replay_buffer), iteration)

        if len(replay_buffer) < min_buffer_size:
            print(
                f"[{iteration}/{num_iterations}] "
                f"Buffer filling: {len(replay_buffer)}/{min_buffer_size} "
                f"(+{num_positions}pos/{games_this_iter}games, {selfplay_time:.1f}s)",
                flush=True,
            )
            continue

        writer.add_scalar("loss/policy", avg_p_loss, iteration)
        writer.add_scalar("loss/value", avg_v_loss, iteration)
        writer.add_scalar("loss/total", avg_loss, iteration)
        writer.add_scalar("train/seconds", train_time, iteration)
        writer.add_scalar("train/lr", optimizer.param_groups[0]["lr"], iteration)

        scheduler.step()

        # === 4. LOG ===
        if iteration % 10 == 0:
            wall = time.time() - train_start
            avg_per_iter = wall / (iteration - start_iteration + 1)
            remaining = avg_per_iter * (num_iterations - iteration)
            eta_h, rem = divmod(int(remaining), 3600)
            eta_m, eta_s = divmod(rem, 60)
            print(
                f"[{iteration}/{num_iterations}] "
                f"loss={avg_loss:.4f} (p={avg_p_loss:.4f} v={avg_v_loss:.4f}) "
                f"buf={len(replay_buffer):,} +{num_positions}pos/{games_this_iter}games "
                f"selfplay={selfplay_time:.1f}s train={train_time:.1f}s "
                f"lr={optimizer.param_groups[0]['lr']:.1e} "
                f"ETA={eta_h}h{eta_m:02d}m{eta_s:02d}s",
                flush=True,
            )

        # === 5. EVALUATE ===
        if iteration % eval_every == 0:
            net.eval()

            # vs random
            wr, dr, lr_, avg_len = evaluate_vs_random(net, num_games=200, device=device)
            last_win_rate = wr
            writer.add_scalar("eval/vs_random_win", wr, iteration)
            writer.add_scalar("eval/vs_random_draw", dr, iteration)
            writer.add_scalar("eval/vs_random_loss", lr_, iteration)
            writer.add_scalar("eval/avg_game_length", avg_len, iteration)
            print(
                f"  EVAL vs random: win={wr:.1%} draw={dr:.1%} loss={lr_:.1%} len={avg_len:.0f}",
                flush=True,
            )

            # vs eval opponent
            if eval_opponent is not None:
                owr, odr, olr, olen = evaluate_vs_opponent(
                    net, eval_opponent, num_games=200, device=device,
                )
                writer.add_scalar("eval/vs_opponent_win", owr, iteration)
                writer.add_scalar("eval/vs_opponent_draw", odr, iteration)
                writer.add_scalar("eval/vs_opponent_loss", olr, iteration)
                print(
                    f"  EVAL vs champion: win={owr:.1%} draw={odr:.1%} loss={olr:.1%} len={olen:.0f}",
                    flush=True,
                )

                # Track best
                if owr > best_vs_opponent:
                    best_vs_opponent = owr
                    best_path = os.path.join(checkpoint_dir, "az_best.pt")
                    torch.save({
                        "iteration": iteration,
                        "model_state_dict": net.state_dict(),
                        "optimizer_state_dict": optimizer.state_dict(),
                        "best_vs_opponent": best_vs_opponent,
                        "win_rate_vs_random": wr,
                    }, best_path)
                    print(f"  NEW BEST vs champion: {owr:.1%} (saved az_best.pt)", flush=True)

            # Milestones
            for milestone_iter, name, min_wr in MILESTONES:
                if name in exported:
                    continue
                if iteration >= milestone_iter and last_win_rate >= min_wr:
                    _export_milestone(net, name, iteration, last_win_rate,
                                      checkpoint_dir, filters, num_res_blocks, use_batch_norm)
                    exported.add(name)

        # === 6. CHECKPOINT ===
        if iteration % checkpoint_every == 0:
            ckpt_path = os.path.join(checkpoint_dir, f"az_{iteration:05d}.pt")
            torch.save({
                "iteration": iteration,
                "model_state_dict": net.state_dict(),
                "optimizer_state_dict": optimizer.state_dict(),
                "exported": list(exported),
                "best_vs_opponent": best_vs_opponent,
                "total_games_played": total_games_played,
                "filters": filters,
                "num_res_blocks": num_res_blocks,
                "use_batch_norm": use_batch_norm,
            }, ckpt_path)
            replay_buffer.save(buffer_path)
            print(f"  Checkpoint: {ckpt_path} + replay buffer ({len(replay_buffer):,} positions)", flush=True)

    # === Final ===
    final_path = os.path.join(checkpoint_dir, "az_final.pt")
    torch.save({
        "iteration": num_iterations,
        "model_state_dict": net.state_dict(),
        "optimizer_state_dict": optimizer.state_dict(),
        "exported": list(exported),
        "best_vs_opponent": best_vs_opponent,
        "total_games_played": total_games_played,
        "filters": filters,
        "num_res_blocks": num_res_blocks,
        "use_batch_norm": use_batch_norm,
    }, final_path)
    replay_buffer.save(buffer_path)

    # Export remaining milestones
    for _, name, _ in MILESTONES:
        if name not in exported:
            _export_milestone(net, name, num_iterations, last_win_rate,
                              checkpoint_dir, filters, num_res_blocks, use_batch_norm)

    # Export best model
    best_onnx = os.path.join(MODELS_DIR, "bot.onnx")
    best_ckpt = os.path.join(checkpoint_dir, "az_best.pt")
    if os.path.exists(best_ckpt):
        export_onnx(best_ckpt, best_onnx, filters=filters, num_res_blocks=num_res_blocks, use_batch_norm=use_batch_norm)
    else:
        export_onnx(final_path, best_onnx, filters=filters, num_res_blocks=num_res_blocks, use_batch_norm=use_batch_norm)
    print(f"Exported: {best_onnx}", flush=True)

    total_time = time.time() - train_start
    h, remainder = divmod(int(total_time), 3600)
    m, s = divmod(remainder, 60)
    print(f"Done in {h}h{m:02d}m{s:02d}s — {total_games_played} games played", flush=True)
    writer.close()


def _export_milestone(net, name, iteration, win_rate, checkpoint_dir, filters, num_res_blocks, use_batch_norm):
    ckpt_path = os.path.join(checkpoint_dir, f"az_milestone_{name}.pt")
    torch.save({
        "iteration": iteration,
        "model_state_dict": net.state_dict(),
    }, ckpt_path)
    onnx_path = os.path.join(MODELS_DIR, f"bot_{name}.onnx")
    export_onnx(ckpt_path, onnx_path, filters=filters, num_res_blocks=num_res_blocks, use_batch_norm=use_batch_norm)
    print(
        f"  MILESTONE '{name}' at iter {iteration} (win={win_rate:.1%}): {onnx_path}",
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

    # Use PPO champion as eval opponent (our current best)
    eval_opp = "checkpoints/ppo_eval_target.pt"
    if not os.path.exists(eval_opp):
        eval_opp = "checkpoints/ppo_hard.pt"
    if not os.path.exists(eval_opp):
        eval_opp = None

    train_alphazero(
        resume_from=resume,
        device=best_device(),
        eval_opponent_path=eval_opp,
        eval_opponent_filters=64,
        eval_opponent_res_blocks=0,
    )
