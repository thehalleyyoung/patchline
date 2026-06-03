# Ethics review template

`patchline ethics-review-template` turns governance promises for new evidence into a reproducible gate. A versioned review file must cover every high-stakes intake area before Patchline treats the data as benchmark, learning, or outcome-study evidence:

- `new_data_source` for public corpora, marketplace submissions, incident notes, and benchmark additions;
- `live_feedback_loop` for adopter-local feedback that can influence thresholds, calibration, or release decisions;
- `adopter_outcome_study` for claims about reviewer time, burden, false positives, incident reduction, or adoption outcomes.

Each review preserves the purpose, data sources, consent basis, privacy review, retention policy, minimization plan, withdrawal path, security owner, independent reviewer roles, risk score, and hashed evidence paths. Live feedback loops must name the human oversight path before any learned signal can affect blocking policy. Outcome studies must name a preregistration or protocol before results can become claims evidence.

## Reproduce

```bash
go run ./cmd/patchline ethics-review-template \
  --spec examples/ethics-review-template.json \
  --root . \
  --out results/generated/ethics-review-template \
  --json

make ethics-review-template-gate
```

The gate also mutates the fixture into negative controls for missing human oversight, missing preregistration, stale review evidence, escaped evidence paths, high risk without mitigations, and insufficient independent reviewers.
