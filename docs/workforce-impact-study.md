# Workforce-impact study

`patchline workforce-impact-study` measures whether migration-safety automation changes review ownership, escalation load, or learning outcomes without rewarding vanity metrics. It uses a treated/control before/after design, hashes every evidence artifact, verifies automation references are real `make` gates, and reports difference-in-differences effects instead of raw before/after deltas.

```bash
go run ./cmd/patchline workforce-impact-study \
  --spec examples/workforce-impact-study.json \
  --root . \
  --out results/generated/workforce-impact-study \
  --json
```

The study fails if the control cohort shifts with treatment, if escalation drops while downstream misses rise, if learning scores improve without held-out detection lift, if reviewer IDs look personally identifying, or if post-automation observations omit the reproducing gate commands. Reproduce the positive, negative, and determinism controls with:

```bash
make workforce-impact-study-gate
```
