# Guard effectiveness simulation

A generated guard claims to make a risky data change safe. This gate **simulates** that claim against
synthetic before/after datasets derived from the **real public schema** — the tables that appear in
ranked risks — instead of trusting the guard's text.

For every table the gate builds a deterministic synthetic dataset and runs the guard on four
scenarios:

- **safe** — a bounded change touching one row → the guard must **allow**.
- **unsafe-broad** — a change touching every row → the guard must **block**.
- **unsafe-missing** — the target table is absent → the guard must **fail closed** (block).
- **unknown-meta** — row count is unavailable → the guard must **fail closed** (block).

The guard predicate is deliberately fail-closed:

> ALLOW iff `table_exists AND row_count_known AND affected_rows <= scope_bound`, else BLOCK.

Guarantees enforced by the gate (measured guard effectiveness):

1. **Effectiveness 1.0** — every scenario decision is correct, with zero unsafe changes allowed.
2. **Always fails closed** on missing-table and unknown-row-count cases.
3. Determinism across reruns.
4. **Negative control** — a no-op control guard that always allows scores strictly lower, proving
   the simulation has discriminating power.

```
make guard-effectiveness-gate
```

Outputs land in `results/generated/guard-effectiveness/`.
