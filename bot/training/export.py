"""Export trained PPO model to ONNX for Go inference.

Exports both heads (action logits + state value). Go side only uses action_logits.
"""

import sys
import os

import torch

from model import PPONet


def export_onnx(checkpoint_path: str, output_path: str):
    net = PPONet()
    checkpoint = torch.load(checkpoint_path, map_location="cpu", weights_only=True)
    net.load_state_dict(checkpoint["model_state_dict"])
    net.eval()

    dummy_input = torch.randn(1, 19, 4, 4)

    torch.onnx.export(
        net,
        dummy_input,
        output_path,
        input_names=["state"],
        output_names=["action_logits", "state_value"],
        dynamic_axes={
            "state": {0: "batch"},
            "action_logits": {0: "batch"},
            "state_value": {0: "batch"},
        },
        opset_version=17,
    )
    size_kb = os.path.getsize(output_path) / 1024
    print(f"Exported to {output_path} ({size_kb:.0f} KB)")

    # Verify with onnxruntime
    import onnxruntime as ort
    import numpy as np
    sess = ort.InferenceSession(output_path)
    inputs = {sess.get_inputs()[0].name: np.random.randn(1, 19, 4, 4).astype(np.float32)}
    outputs = sess.run(None, inputs)
    print(f"Verification: input {sess.get_inputs()[0].name} {sess.get_inputs()[0].shape}")
    for i, out in enumerate(sess.get_outputs()):
        print(f"  output[{i}]: {out.name} shape={outputs[i].shape}")


if __name__ == "__main__":
    checkpoint = sys.argv[1] if len(sys.argv) > 1 else "checkpoints/ppo_final.pt"
    output = sys.argv[2] if len(sys.argv) > 2 else "../models/bot.onnx"
    os.makedirs(os.path.dirname(output), exist_ok=True)
    export_onnx(checkpoint, output)
