# False-negative discovery (seeded hazards + real cross-validation)

Recall is only credible if you measure what a detector **misses**. This gate seeds known
public-incident hazard analogues into a real migration layout and reports detection recall,
explicitly surfacing the hazards Patchline under-escalates or drops.

- **Seeded hazards.** Seven hazard classes drawn from well-known production incidents —
  destructive column/table drops, unscoped `DELETE`/`UPDATE`, `NOT NULL` without a default,
  column renames, and non-concurrent index builds — are materialized as real SQL migration
  files. Each carries its **incident analogue** and the detection kind it should produce.
- **Recall measurement.** Patchline's static baseline runs over the corpus. Each hazard is
  classified as *specific* (matched its expected kind), *generic-only* (flagged but not
  escalated), or *missed* (no risk emitted). The under-escalated and missed hazards become
  the workflow's **false-negative** output instead of being hidden.
- **Real cross-validation.** The same detectors are run against a real public repository
  (`lobsters/lobsters` migrations) to confirm the hazard kinds fire on real code, not just on
  the synthetic corpus.

```
make fn-discovery-gate
```

The gate fails unless enough hazards are detected at the specific tier, at least one genuine
false negative is surfaced, the benign control is not escalated to high severity, and the
real cross-validation finds matching-kind risks.

Outputs (`results/generated/fn-discovery/`):

- `hazard-recall.jsonl` — per-hazard detection tier.
- `fn-discovery.json` / `.md` — recall, the surfaced false-negative set, and controls.

This turns recall into a reproducible, auditable measurement and makes Patchline's blind
spots (e.g. non-concurrent index locks, `NOT NULL`-without-default rewrites) explicit.
