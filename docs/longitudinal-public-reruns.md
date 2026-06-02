# Longitudinal public-corpus reruns

These longitudinal public-corpus reruns are the public-history gate for Stage 21.

Patchline can rerun the same public repository slices over multiple historical commits per repository. This turns a one-shot public-corpus result into a time-aware artifact that exposes risk delta, file-count delta, and provenance changes across pinned public history.

```bash
make longitudinal-public-reruns-gate
```

The generated outputs include:

- `longitudinal-reruns.json`: all run rows, per-repository trends, and summary counts.
- `longitudinal-reruns.md`: reviewer-facing trend table.
- `repository-trends.json`: compact per-repository deltas.
- `runs.jsonl`: one JSON row per repository commit.

The gate analyzes real public code from Lobsters, Django, and Apache Airflow at two pinned commits per repository and verifies that every run has a stable hash, scanned files, and longitudinal trend data.
