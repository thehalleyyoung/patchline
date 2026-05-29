# Workflow model checking

`model-check-workflow` checks incident-response workflows against a bounded state machine:

```bash
go run ./cmd/patchline model-check-workflow examples/workflows/bad-migration-approved.json
```

The model includes the operational stages `ingest`, `explain`, `approve`, `dry_run`, `apply`, `verify`, `rollback`, `audit`, and `archive`. Transitions are guarded by evidence hashes, policy approval, dry-run hashes, rollback availability, and immutable ledger checkpoints.

The checker enumerates bounded reachable traces and validates temporal properties:

| Property | Formula |
| --- | --- |
| No apply before approval | `always(apply -> once(approve))` |
| No approval without evidence | `always(approve -> evidence_hash != empty)` |
| Eventual verification or rollback | `always(apply -> eventually(verify or rollback) within bound)` |
| Rollback availability | `always(apply -> rollback_available or verify)` |
| Immutable audit | `always(audit -> ledger_checkpoint.tip_hash != empty)` |

Reports emit standardized proof obligations with status `proved`, `checked`, `counterexample`, `assumed`, or `not_supported`. Assumed and unsupported obligations become proof holes with suggested discharge actions.

Fixtures demonstrate both useful success and failure modes:

```bash
go run ./cmd/patchline model-check-workflow examples/workflows/bad-migration-approved.json
go run ./cmd/patchline model-check-workflow examples/workflows/apply-before-approval.json
go run ./cmd/patchline model-check-workflow examples/workflows/missing-rollback.json --json
```

The first fixture checks the bad-migration repair workflow. The second intentionally produces an apply-before-approval counterexample. The third produces a rollback proof hole, showing where an operator must provide stronger rollback or verification evidence.
