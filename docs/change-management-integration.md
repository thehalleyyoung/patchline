# Change-management integration

Patchline can verify that its safety gates feed an organization's existing change-management workflow instead of bypassing it.

`patchline change-management-verify` reads a versioned workflow spec, checks that each approved change step cites a passed blocking Patchline gate, hashes the reviewed gate reports and approval artifacts, and keeps rollback and emergency-expiry evidence in the generated report.

## What the verifier rejects

- approval steps that reference unknown gates or only non-blocking gates;
- blocking gates that failed, were not run, or have report-hash drift;
- changes with no ticket, rollback plan, real approval evidence, or distinct approvers;
- emergency workflows without a deterministic RFC3339 expiry.

## Reproduce

```bash
go run ./cmd/patchline change-management-verify \
  --spec examples/change-management-integration.json \
  --root . \
  --out results/generated/change-management-integration \
  --json

make change-management-integration-gate
```
