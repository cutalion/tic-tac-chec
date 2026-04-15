"""Measure how much MCTS at inference improves a trained model.

Plays raw network (greedy argmax) vs network+MCTS (N simulations).
"""

import sys
import time
import torch
import numpy as np
from model import PPONet
from env import TicTacChecEnv, Color
from mcts_collect import run_mcts, visit_counts_to_policy

def play_raw_vs_mcts(net, num_games=100, num_simulations=50, device="cpu"):
    """Play raw network vs network+MCTS."""
    wins_raw, wins_mcts, draws = 0, 0, 0
    total_length = 0

    for game_idx in range(num_games):
        env = TicTacChecEnv()
        # Alternate who uses MCTS
        mcts_color = Color.WHITE if game_idx % 2 == 0 else Color.BLACK

        done = False
        while not done:
            if env.turn == mcts_color:
                # MCTS move
                visit_counts, actions = run_mcts(env, net, num_simulations, device)
                policy = visit_counts_to_policy(visit_counts, actions, temperature=0.1)
                action = int(np.argmax(policy))
            else:
                # Raw network move (greedy)
                state = env.encode_state()
                state_t = torch.tensor(state, dtype=torch.float32, device=device).unsqueeze(0)
                mask = env.legal_action_mask()
                with torch.no_grad():
                    logits, _ = net(state_t)
                logits = logits.squeeze(0).cpu().numpy()
                logits[~mask] = -1e9
                action = int(np.argmax(logits))

            _, _, done, info = env.step(action)

        total_length += env.move_count
        winner = info.get("winner")
        if winner is None:
            draws += 1
        elif winner == mcts_color:
            wins_mcts += 1
        else:
            wins_raw += 1

    n = num_games
    return wins_mcts / n, draws / n, wins_raw / n, total_length / n


if __name__ == "__main__":
    device = "mps" if torch.backends.mps.is_available() else "cpu"

    checkpoint = sys.argv[1] if len(sys.argv) > 1 else "checkpoints/ppo_05000.pt"
    filters = int(sys.argv[2]) if len(sys.argv) > 2 else 64
    res_blocks = int(sys.argv[3]) if len(sys.argv) > 3 else 0

    net = PPONet(filters=filters, num_res_blocks=res_blocks, use_batch_norm=False).to(device)
    ckpt = torch.load(checkpoint, map_location=device, weights_only=True)
    net.load_state_dict(ckpt["model_state_dict"])
    net.eval()
    print(f"Loaded {checkpoint} ({filters}f, {res_blocks}r)")

    for sims in [25, 50, 100, 200]:
        t0 = time.time()
        mcts_wr, draw_r, raw_wr, avg_len = play_raw_vs_mcts(
            net, num_games=100, num_simulations=sims, device=device
        )
        elapsed = time.time() - t0
        print(
            f"  MCTS({sims:3d} sims) vs Raw: "
            f"mcts={mcts_wr:.1%} draw={draw_r:.1%} raw={raw_wr:.1%} "
            f"len={avg_len:.0f} time={elapsed:.0f}s"
        )
