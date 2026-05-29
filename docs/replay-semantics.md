# Replay semantics

`repair-semantics` exposes the operational semantics of a repair manifest instead of only showing the final dry-run diff:

```bash
go run ./cmd/patchline repair-semantics examples/repairs/repair-bad-invoice-backfill.json
go run ./cmd/patchline repair-semantics examples/repairs/repair-bad-invoice-backfill.json --json
go run ./cmd/patchline repair-semantics examples/repairs/repair-bad-invoice-backfill.json --store examples/snapshots/billing-bad-migration-before.json
go run ./cmd/patchline snapshot-drift examples/repairs/repair-bad-invoice-backfill.json examples/snapshots/billing-bad-migration-before.json examples/snapshots/billing-bad-migration-before.json
```

The report is deterministic and hashable. It includes:

- a small-step trace with `op_id`, rule name, pre/post store hashes, touched rows, and terminal state;
- read/write footprints for each operation;
- pairwise syntactic independence and fixture-observed commutativity checks;
- bounded confluence checking over dependency-valid operation orders;
- read committed, repeatable read, snapshot, and serializable hazard reports for modeled write/write and predicate read/write conflicts;
- compensating-action semantics for append-only logs, event streams, queues, logical replays, and derived rebuilds;
- replayable counterexample JSON containing the store fragment, operation subset, before hash, and observed after hashes.

Small-step states are deliberately explicit. `normal` means a concrete transition rewrote the replay store. `error` means a transition rule fired and replay returned an explicit error. `stuck` means no concrete row-level rule applied, such as an update/delete predicate matching zero rows. `rollback` means a declared snapshot rollback restored the pre-error store hash. `unknown` means the manifest operation is accepted but the current replay store has no row-level transition for it, as with logical `replay`, `rebuild-index`, `append-log`, `emit-event`, and `enqueue`.

Commutativity is reported in two layers to avoid overclaiming. `syntactic_verdict` is a sufficient-condition analysis over tables, row keys, predicate columns, write columns, and dependencies. `observation_status` is a concrete replay observation over the fixture store. A fixture observation can corroborate a result but is not labeled as a proof.

Confluence is checked by enumerating dependency-valid operation orders up to a fixed bound. Above the bound, the report returns `unknown` rather than silently treating pairwise checks as global confluence.

`--store` lets the same repair run over imported historical row snapshots instead of the built-in demo store. `snapshot-drift` runs the repair over two snapshots and fails if matched rows or concrete row diffs change, which turns historical drift into a strict benchmark signal rather than a narrative claim.
