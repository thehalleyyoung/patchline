# Certification renewal

`patchline certification-renewal` keeps Patchline practitioner credentials current as database engines and hazard classes evolve. A renewal spec names dated database-engine semantic updates, newly discovered hazard classes, active credentials, and renewal attempts; the report hashes all evidence, validates engines against Patchline's supported db-semantics set, and renews an active credential only when a gate-backed attempt after the newest update covers every required update ID and topic.

```bash
go run ./cmd/patchline certification-renewal \
  --spec examples/certification-renewal.json \
  --root . \
  --out results/generated/certification-renewal \
  --json
```

The process is deterministic: `as_of`, `effective_date`, `discovered_at`, and `submitted_at` are spec-supplied dates, not wall-clock time. Reproduce the positive run, negative control, and stable hash check with:

```bash
make certification-renewal-gate
```
