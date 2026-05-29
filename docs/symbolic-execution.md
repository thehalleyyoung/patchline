# Symbolic execution

`symbolic-exec` explores a repair program over a bounded replay store and emits deterministic path constraints plus symbolic assignments:

```bash
go run ./cmd/patchline symbolic-exec examples/repairs/repair-bad-invoice-backfill.json --json
```

For each operation, the report includes pre/post store hashes, bounded row paths, guard constraints, whether each guard is satisfied by the bounded store, and the symbolic writes that would occur on satisfying paths. This complements `dry-run`: dry-run shows concrete diffs, while symbolic execution shows why a row path was or was not touched.

The default bad-migration fixture explores two invoice rows, finds one satisfying path, and emits two assignments for the repaired row. The output is hashable and is included in `semantics-audit` as the `symbolic_execution` artifact.
