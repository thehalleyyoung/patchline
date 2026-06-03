# Verified multi-service rollback planner

`patchline multi-service-rollback-plan` turns a declared set of service migrations into a rollback plan whose order is the reverse of a deterministic topological sort over migration `depends_on` edges. Service upstream/downstream metadata is checked as an audit trail for cross-service handoffs, but it is not allowed to invent ordering edges.

```bash
go run ./cmd/patchline multi-service-rollback-plan \
  --spec examples/multi-service-rollback-plan.json \
  --out results/generated/verified-rollback-planner \
  --json
```

The report proves three things: the dependency graph is acyclic and within explicit depth, fanout, and wave bounds; every reversible migration has a verified rollback action; and any irreversible or unverified step is summed against explicit row, critical-row, and affected-service data-loss bounds. Unsafe plans still write a report with `ok: false` so CI gates can inspect the counterexamples.

Reproduce the positive and negative controls with:

```bash
make verified-rollback-planner-gate
```
