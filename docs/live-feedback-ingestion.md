# Live feedback ingestion

Patchline's **source-free live feedback** ingester records reviewer outcomes
from adopters without collecting source code, diffs, file paths, evidence
snippets, finding identifiers, evidence hashes, adopter IDs, or the local salt.

## Contract

The input is a local JSON export with a fixed schema:

```bash
go run ./cmd/patchline feedback ingest examples/live-feedback-ingestion-gate.json \
  --out results/generated/live-feedback-ingestion-gate --json
```

Only typed fields are accepted: detector name, release, confidence, reviewer
verdict, reviewer action, burden minutes, reviewer role, and opaque identifiers
used solely for in-memory deduplication. Unknown fields, nested raw-evidence
keys, source-like values, invalid enums, invalid confidence scores, and missing
required fields are rejected by reason code only; their field names and values
are not stored.

## Privacy model

The report is shareable because it emits only aggregate outcomes:

- a secret-salted adopter cohort, with the salt never emitted,
- k-anonymous detector/release/confidence/verdict/action groups,
- a dimension-free residual bucket for low-count groups, and
- rejected-record reason codes without offending values.

Patchline enforces a tool-side k-anonymity floor even if an adopter requests a
smaller group size. Suppressed groups are folded into the residual bucket so a
published total cannot be subtracted to recover a singleton detector, release,
or finding.

## Reproduce

```bash
make live-feedback-ingestion-gate
```
