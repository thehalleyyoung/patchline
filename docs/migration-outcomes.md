# Migration outcome histories

`migration-outcomes` turns historical evidence into a deterministic outcome report for a migration. It links observed migration entities to traces, SQL mutation fingerprints, mutated rows, derived records/reports, repair manifests, policy failures, benchmark hashes, and source-SQL inventory hashes.

```bash
go run ./cmd/patchline migration-outcomes \
  examples/incidents/bad-migration.jsonl \
  demos/billing/migrations/002_bad_backfill.sql \
  --repair examples/repairs/repair-bad-invoice-backfill.json \
  --policy examples/policies/review-required.json \
  --benchmark examples/benchmarks/strict-migration-corpus.json \
  --source-sql examples/source-sql \
  --json
```

The report includes:

| Field | Meaning |
| --- | --- |
| `outcomes` | Per-migration observed traces, SQL mutations, records, reports, repair metadata, and policy failures |
| `changelog.changed_tables` | Tables and statement fingerprints changed by the migration analyzer |
| `changelog.broad_effects` | High-risk or broad effects such as destructive statements or risky updates |
| `changelog.observed_outcomes` | Counts of linked migrations, traces, SQL mutations, records, reports, and repairs |
| `changelog.benchmark_hash` | Strict benchmark suite hash used to validate analyzer behavior |
| `changelog.source_sql_hash` | Source SQL/ORM inventory hash for code-level data effects |

The command intentionally reports **observed historical outcomes**, not omniscient causality. If telemetry does not link a migration to a trace, row mutation, repair, or policy result, the output omits that link instead of inventing one. That keeps the artifact useful for current incident review while preserving formal-methods honesty about missing evidence.

To validate policy-failure capture, run the same command with `examples/policies/reject-high-risk.json`; the changelog will carry the failed high-risk-migration policy rule into `policy_failures`.
