# Longitudinal education study

`patchline longitudinal-education-study` checks whether Patchline-trained reviewers still catch real migration hazards months later. The study is deliberately artifact-backed: each hidden hazard cites real repo evidence, names a reproducing `make <gate>` command, compares a trained cohort with a control cohort, and counts a detection only when the reviewer records the expected safety decision, cites evidence, and includes the gate command.

```bash
go run ./cmd/patchline longitudinal-education-study \
  --spec examples/longitudinal-education-study.json \
  --root . \
  --out results/generated/longitudinal-education-study \
  --json
```

The report hashes every evidence file, verifies the hidden hazards are gate-backed, checks blinded six-month follow-up coverage, computes trained-versus-control detection lift at the delayed timepoint, and rejects prose-only detections that omit evidence citations or reproducing commands. Reproduce the positive, negative, and determinism controls with:

```bash
make longitudinal-education-study-gate
```
