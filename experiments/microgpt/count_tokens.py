"""Token count of microgpt.py's training run, without touching the reference.

Replays its data prep (seed 42, then shuffle, before any other draw) and sums
the per-step positions, so Python wall time can be reported per token.
    python3 -I count_tokens.py [num_steps]
"""
import random, sys
random.seed(42)
docs = [line.strip() for line in open('input.txt') if line.strip()]
random.shuffle(docs)
steps = int(sys.argv[1]) if len(sys.argv) > 1 else 1000
print(sum(min(16, len(docs[s % len(docs)]) + 1) for s in range(steps)))
