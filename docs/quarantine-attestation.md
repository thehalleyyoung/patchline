# Repair-candidate quarantine attestations

Generated repairs must never run on their own. This gate issues a **quarantine attestation** for
every real repair candidate, making non-execution and manual review explicit and auditable.

Each attestation records:

- `executed: false` and a **non-execution** statement — the candidate was generated for review
  only and has not touched any database.
- `quarantined: true` and `manual_review_required: true` — it is held for human review.
- `required_reviewers` — a count that **escalates** (to 2) when the candidate still has open
  proof holes.
- `fingerprint` — a stable content fingerprint over the candidate identity and repair paths.

Guarantees enforced by the gate:

1. **No candidate is executed** and none is auto-applied (no status is treated as executable).
2. **Every candidate requires manual review** and carries a fingerprint.
3. **Fingerprints are stable** across reruns, so attestations are diffable.

```
make quarantine-attestation-gate
```

Outputs land in `results/generated/quarantine-attestation/`.
