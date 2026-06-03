# Maintainer acceptance study

`patchline maintainer-acceptance-study` evaluates paired maintainer reviews of baseline remediation material versus generated remediation plans. It measures review-time reduction while enforcing that generated plans keep uncertainty visible.

```bash
go run ./cmd/patchline maintainer-acceptance-study \
  --spec examples/maintainer-acceptance-study.json \
  --root . \
  --out results/generated/maintainer-acceptance-study \
  --json
```

The study spec names real repository artifacts to hash, ground-truth uncertainty items for each review task, and paired observations for each participant under `baseline` and `generated_plan` conditions. The report fails if generated plans save too little time, lose uncertainty recall, over-inflate confidence, produce incorrect decisions, or omit any required uncertainty item.

Reproduce the positive, hidden-uncertainty, and determinism controls with:

```bash
make maintainer-acceptance-study-gate
```
