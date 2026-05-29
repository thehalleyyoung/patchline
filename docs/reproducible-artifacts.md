# Reproducible repair artifacts

Patchline benchmark artifacts make repair demos reproducible instead of merely illustrative.

```bash
go run ./cmd/patchline reproduce examples/reproduce/bad-migration-billing.json
go run ./cmd/patchline benchmark examples/reproduce/bad-migration-billing.json --json
go run ./cmd/patchline export-bundle examples/reproduce/bad-migration-billing.json examples/policies/review-required.json demos/billing/migrations/002_bad_backfill.sql
go run ./cmd/patchline benchmark-suite examples/benchmarks/strict-migration-corpus.json
```

The artifact pins a repair manifest, expected dry-run hash, ledger checkpoint, and executable checks:

```json
{
  "version": "patchline.reproduce/v1",
  "name": "bad-migration-billing",
  "repair_manifest": "../repairs/repair-bad-invoice-backfill.json",
  "expected_report_hash": "98353fadab4e19af5a1d51aefd08dd6425b2f7aa8d1898c183fdbf1cac0a75ab",
  "expected_ledger_checkpoint": {
    "count": 2,
    "tip_hash": "c6e87ee83146c7fd43d1e652d1d19a367e683380b1f5d3ed077a87c299d8872c"
  },
  "checks": [
    {"kind": "max_changed_rows", "expect": "1"},
    {"kind": "operation_effect_equals", "ref": "restore-invoice-total", "expect": "reversible_update"},
    {"kind": "changed_row_equals", "ref": "invoices/inv_1002.total_cents", "expect": "4200"},
    {"kind": "downstream_contains", "ref": "report:monthly_revenue", "expect": "true"},
    {"kind": "no_unscoped_updates", "expect": "true"}
  ]
}
```

## Check kinds

| Check | Meaning |
| --- | --- |
| `report_hash_equals` | Dry-run report must match the pinned canonical hash |
| `max_changed_rows` | Total row diffs must stay under a threshold |
| `operation_effect_equals` | Effect inference must classify an operation as expected |
| `changed_row_equals` | A specific changed row column must have the expected after-value |
| `downstream_contains` | The provenance graph must include an expected downstream entity |
| `no_unscoped_updates` | Dry-run diffs must stay inside the declared repair scope |

## Updating expected hashes

When an intentional change updates the dry-run semantics, regenerate the pinned expected values:

```bash
go run ./cmd/patchline reproduce examples/reproduce/bad-migration-billing.json --update
```

This workflow is meant to feel like golden tests for production repair research.

## Proof-carrying bundles

`export-bundle` emits `patchline.bundle/v2`, which includes the repair manifest, dry-run report, provenance slice, migration analysis, policy results, ledger checkpoint, and proof artifacts:

| Entry | Meaning |
| --- | --- |
| `solver-obligations.json` | Bounded SMT-style scope/frame/row-count/invariant obligations. |
| `symbolic-execution.json` | Bounded row paths and symbolic assignments. |
| `workflow-model-check.json` | Incident workflow temporal properties, proof obligations, proof holes, and counterexamples. |

This makes the handoff artifact reviewable without rerunning every command, while still preserving hashes for rerun verification.

## Strict corpus suites

Reproducibility artifacts validate one incident end to end. Benchmark suites validate analyzer usefulness across a frozen corpus:

```json
{
  "version": "patchline.benchmark-suite/v1",
  "name": "strict-migration-corpus",
  "cases": [
    {
      "id": "billing-bad-backfill",
      "path": "../../demos/billing/migrations/002_bad_backfill.sql",
      "label": "unsafe",
      "expected_report_hash": "0be915737d677c0d06eae943c9293f175c19ca855acb225d93c06de803f39fbe"
    }
  ]
}
```

Every case must pin `expected_report_hash`. The runner fails if the human label no longer matches the analyzer prediction or if the canonical report hash changes. See [`strict-benchmarking.md`](strict-benchmarking.md) for the real-world corpus protocol.
