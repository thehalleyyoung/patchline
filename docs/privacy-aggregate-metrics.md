# Privacy-preserving aggregate metrics

`patchline repo metrics` turns one or more `repo analyze` output directories into a shareable trend report that contains only aggregate counts, coarse buckets, and salted cohort IDs. It is designed for comparing risk trends across teams or time windows without uploading source, paths, prompts, generated content, or raw evidence.

Example:

```bash
patchline repo metrics \
  --analyses results/generated/week-1,results/generated/week-2 \
  --salt "$PATCHLINE_METRICS_SALT" \
  --out results/generated/metrics \
  --json
```

The command writes:

- `metrics.json`: machine-readable aggregate rows, deltas, privacy contract, and hash;
- `metrics.md`: a human-readable source-free summary.

The shareable artifact includes bucketed file counts, ranked-risk counts, policy/proof/compare counts, generated-file counts, high-signal totals, and trend deltas. It suppresses repository names, source paths, raw evidence, prompts, generated content, risk identifiers, and source-derived artifact hashes. Cohort IDs are salted and opaque so the same team can compare its own runs while avoiding public repository or path disclosure.

The JSON privacy contract records `source_free`, `raw_evidence_free`, `path_free`, and `salted_cohort_ids` as true for every metrics report.

Run `make privacy-metrics-gate` to prove the contract against focused CLI tests and pinned public repository analyses. The gate fails if the metrics artifact contains known repo names, migration paths, SQL snippets, prompts, generated content, or analysis output paths.
