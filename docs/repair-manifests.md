# Repair manifest operations

Patchline repair manifests are strict JSON documents. Unknown fields are rejected for the current `patchline.repair/v1` schema so review tools can trust that every meaningful field is typed and validated.

## Version migration

Older manifests can be upgraded explicitly:

```bash
go run ./cmd/patchline migrate-repair examples/repairs/legacy-v0-repair.json > /tmp/repair-v1.json
go run ./cmd/patchline validate-repair /tmp/repair-v1.json
```

The migration command supports `patchline.repair/v0` and current `patchline.repair/v1`. It preserves legacy incident IDs, affected entities, table predicates, checks, repair steps, dependencies, and snapshot rollback intent while emitting canonical v1 JSON.

## Templates

Templates provide safe starting points for common incidents:

```bash
go run ./cmd/patchline template-repair row-restore
go run ./cmd/patchline template-repair scoped-backfill-reversal
go run ./cmd/patchline template-repair report-recompute
```

Each template includes a scope, at least one operation, preconditions, postconditions, and rollback settings. Placeholders such as `replace-me`, `table`, and `record:table/id` must be edited before use.

## Linting

The linter is stricter than schema validation:

```bash
go run ./cmd/patchline lint-repair examples/repairs/repair-bad-invoice-backfill.json --json
go run ./cmd/patchline lint-repair examples/repairs/repair-bad-invoice-backfill.json --proof --json
```

Findings include:

| Field | Meaning |
| --- | --- |
| `level` | `error` or `warning` |
| `code` | Stable machine-readable rule identifier |
| `message` | Human-readable finding |
| `ref` | Operation ID, entity ID, table, or other finding target |
| `remediation` | Concrete repair author guidance |

This is meant for PR review and incident-approval workflows where a human still owns the repair, but tooling must make risky omissions obvious.

With `--proof`, linting emits a hashable repair proof report:

| Field | Meaning |
| --- | --- |
| `hoare_triple` | Reviewer-facing `{historical evidence + preconditions} repair {postconditions + obligations}` view |
| `weakest_preconditions` | Operation-specific obligations such as row existence, row absence for inserts, scoped predicates, and assignments |
| `frame_conditions` | Syntactic tables, predicates, and columns the manifest may write, plus columns/tables it must not touch |
| `refinement_checks` | Checks that generated SQL preserves operation id, kind, table, predicates, and assignments from the abstract manifest |
| `counterexamples` | Refuted obligations with witnesses when a repair cannot be proven safe enough for review |

Database facts such as row existence are intentionally marked `assumed` until a live query checker or replay snapshot discharges them; syntactic manifest and SQL-shape properties are marked `checked`.

## SQL and rollback generation

Approved manifests can produce deterministic SQL plans:

```bash
go run ./cmd/patchline generate-sql examples/repairs/repair-bad-invoice-backfill.json --json
```

SQL generation supports scoped `insert`, `update`, and `delete` operations. Inserts require explicit row values including an `id`, updates require both `set` and `where`, and deletes require a non-empty `where` predicate. The manifest must pass lint first; broad or incomplete operations fail before SQL is emitted.

Replay semantics also accepts external operation kinds `replay`, `rebuild-index`, `append-log`, `emit-event`, and `enqueue`. These do not generate row-level SQL; they surface proof holes and compensating-action obligations so append-only logs, event-sourced stores, queues, and derived rebuilds can be reviewed honestly instead of being forced into a fake rollback model.

Rollback plans are generated from dry-run diffs:

```bash
go run ./cmd/patchline rollback-plan examples/repairs/repair-bad-invoice-backfill.json --json
```

The rollback plan is derived from the same dry-run diffs used for approval. Updates restore changed columns to their pre-repair values, inserted rows become deterministic delete rollback statements, and deleted rows become deterministic insert rollback statements. Every plan emits a stable hash for review and attestation.

Multi-table repairs can be wrapped in a deterministic transaction plan:

```bash
go run ./cmd/patchline transaction-plan examples/repairs/repair-bad-invoice-backfill.json --json
```

The transaction plan emits `BEGIN`, a sorted `LOCK TABLE ... IN ROW EXCLUSIVE MODE` statement, dependency-ordered repair statements, and `COMMIT`. It exposes both `lock_order` and `operation_order` so reviewers can verify deadlock-avoidance and dependency sequencing before execution.
