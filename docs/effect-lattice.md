# Effect lattice and abstract interpretation

Patchline's repair-effect analyzer is a deterministic abstract interpreter over replay observations. It is not a risk score and not a model: it maps concrete row diffs into a small lattice that reviewers can compare, hash, and gate.

```bash
go run ./cmd/patchline effect-summary examples/repairs/repair-bad-invoice-backfill.json
go run ./cmd/patchline effect-summary examples/repairs/repair-bad-invoice-backfill.json --json
```

## Lattice

The current lattice is a conservative total order by review risk:

| Rank | Effect | Meaning |
| ---: | --- | --- |
| 0 | `noop` | No concrete row change |
| 1 | `idempotent_update` | Bounded deterministic write whose repeated application is stable |
| 2 | `reversible_update` | Bounded write with declared snapshot rollback |
| 3 | `replay` | External replay operation with system-specific semantics |
| 4 | `derived_rebuild` | Derived-state rebuild from source records |
| 5 | `append_only_external` | Append-only external effect requiring an explicit compensating action |
| 6 | `destructive` | Delete or unbounded write that removes or may rewrite state |
| 7 | `unknown` | Operation outside known transfer functions |

The join operator returns the higher-ranked element, so a multi-operation repair summarizes to the most conservative effect class present.

## Transfer functions

`internal/effects` exposes transfer functions through `Infer` and `Summarize`:

| Transfer | Source | Result |
| --- | --- | --- |
| `T_noop` | No assignments or concrete changes | `noop` |
| `T_const_write_idempotent` | Predicate-bounded constant assignment without snapshot rollback | `idempotent_update` |
| `T_snapshot_reversible_write` | Predicate-bounded write with snapshot rollback | `reversible_update` |
| `T_external_replay` | Replay operation | `replay` plus a proof hole |
| `T_derived_rebuild` | Derived rebuild operation | `derived_rebuild` plus a proof hole |
| `T_compensating_external_append` | Append-only log, emitted event, or queue message | `append_only_external` plus a compensating-action proof hole |
| `T_destructive` | Delete or unbounded update | `destructive` |
| `T_unknown` | Unsupported operation | `unknown` plus a proof hole |

## Abstraction and concretization

The abstraction function is:

```text
alpha(row diffs) =
  row-count bounds
  + changed-column set
  + downstream-entity count
  + operation effect
```

The concretization relation is:

```text
gamma(summary) =
  all concrete executions whose changed rows, columns,
  downstream impact, and effect rank are within the summary bounds
```

The soundness obligation emitted in JSON is intentionally explicit: every concrete replay diff must be represented by exactly one abstract operation summary. Future invariant and SMT work can discharge stronger row-count and predicate-implication claims; until then, the command gives immediate value by turning concrete dry-run diffs into a monotone, hashable reviewer artifact.
