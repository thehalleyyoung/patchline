# Policy freezing during incidents

Patchline's **policy-freezing mechanism** pins high-stakes organizations to
audited detector releases while an incident is active.

## Contract

```bash
go run ./cmd/patchline feedback policy-freeze \
  --spec examples/policy-freeze-gate.json \
  --out results/generated/policy-freeze-gate --json
```

High-stakes incident decisions fail closed: if the current audited release is
missing, or a proposed high-stakes release lacks approval, the report blocks the
policy change and records a missing-audit reason. Non-incident updates are
allowed only when the proposed release is audited.

## Reproduce

```bash
make policy-freeze-gate
```
