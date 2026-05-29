# Invariant declarations and checking

Patchline invariants are strict JSON declarations checked against replay snapshots before and after a repair. They are deterministic review artifacts: failures produce counterexamples, and discovered candidates are hypotheses rather than accepted rules.

```bash
go run ./cmd/patchline check-invariants examples/repairs/repair-bad-invoice-backfill.json examples/invariants/billing-core.json
go run ./cmd/patchline discover-invariants examples/repairs/repair-bad-invoice-backfill.json --json
```

Supported declarations:

| Kind | Required fields | Meaning |
| --- | --- | --- |
| `unique` | `table`, `column` | Non-empty values in the column are unique |
| `foreign_key` | `table`, `column`, `ref_table`, `ref_column` | Every non-empty source value exists in the referenced table/column |
| `enum` | `table`, `column`, `values` | Non-empty column values are in the declared set |
| `nonnegative` | `table`, `column` | Column values parse as integers and are `>= 0` |
| `sum` | `table`, `column`, `expect` | Integer column sum equals the expected value |
| `count` | `table`, `expect` | Table row count equals the expected value |
| `materialized_report` | `table`, `column`, `ref_table`, `ref_column` | Source column sum matches a materialized report value |
| `ledger_balance` | `table`, `column`, `ref_column` | Two integer ledger columns have equal sums |
| `customer_total` | `table`, `column`, `group_column` | Grouped customer-visible totals are nonnegative |

Candidate discovery currently emits explicit support-only hypotheses for uniqueness, small enums, nonnegative integer columns, and table counts. The report never silently promotes candidates to enforced invariants; a human or policy must copy them into an invariant spec.
