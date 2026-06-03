# Patch-series verifier

`patchline patch-series-verify` checks a declared sequence of migration PRs, applies every modeled SQL statement in order, and verifies schema invariants after the initial state and after each statement boundary.

```bash
go run ./cmd/patchline patch-series-verify \
  --spec examples/patch-series-verifier.json \
  --out results/generated/patch-series-verifier \
  --json
```

The verifier proves PR dependencies are ordered, records before/after schema hashes for every migration statement, and emits counterexamples when a required table or column disappears or a forbidden column appears. The current proof covers the modeled DDL fragment used by Patchline's schema engine: `CREATE TABLE`, `DROP TABLE`, `ALTER TABLE ... ADD COLUMN`, and `ALTER TABLE ... DROP COLUMN`.

Reproduce the positive, negative, and determinism controls with:

```bash
make patch-series-verifier-gate
```
