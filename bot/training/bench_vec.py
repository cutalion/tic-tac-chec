"""Benchmark: vectorized env vs scalar env for 128 games of random play."""

import time
import numpy as np
from env import TicTacChecEnv
from env_vec import VecTicTacChecEnv


def bench_scalar(num_games, seed=42):
    """Run num_games sequentially with the scalar env."""
    rng = np.random.RandomState(seed)
    total_steps = 0
    t0 = time.perf_counter()

    for _ in range(num_games):
        env = TicTacChecEnv()
        while not env.done:
            mask = env.legal_action_mask()
            legal = np.where(mask)[0]
            action = rng.choice(legal)
            env.step(action)
            total_steps += 1

    elapsed = time.perf_counter() - t0
    return elapsed, total_steps


def bench_vec(num_games, seed=42):
    """Run num_games in parallel with the vectorized env."""
    rng = np.random.RandomState(seed)
    env = VecTicTacChecEnv(num_games)
    total_steps = 0
    t0 = time.perf_counter()

    while not np.all(env.dones):
        masks = env.legal_action_masks()
        actions = np.zeros(num_games, dtype=np.int32)
        for i in range(num_games):
            if not env.dones[i]:
                legal = np.where(masks[i])[0]
                actions[i] = rng.choice(legal)
                total_steps += 1
        env.step(actions)

    elapsed = time.perf_counter() - t0
    return elapsed, total_steps


if __name__ == "__main__":
    N = 128
    print(f"Benchmarking {N} random games...\n")

    # Warmup
    bench_scalar(4, seed=0)
    bench_vec(4, seed=0)

    scalar_time, scalar_steps = bench_scalar(N)
    vec_time, vec_steps = bench_vec(N)

    print(f"Scalar env:     {scalar_time:.3f}s  ({scalar_steps} total steps, "
          f"{scalar_steps / scalar_time:.0f} steps/s)")
    print(f"Vectorized env: {vec_time:.3f}s  ({vec_steps} total steps, "
          f"{vec_steps / vec_time:.0f} steps/s)")
    print(f"\nSpeedup: {scalar_time / vec_time:.2f}x")
