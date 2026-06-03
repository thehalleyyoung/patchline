# Practitioner certification exam

`patchline practitioner-certification` grades a hands-on migration-safety exam against real Patchline evidence. Each scenario names a role, hazard class, evidence files, an expected safety decision, a reproducible gate command, and an explicit rubric.

```bash
go run ./cmd/patchline practitioner-certification \
  --spec examples/practitioner-certification-exam.json \
  --root . \
  --out results/generated/practitioner-certification-exam \
  --json
```

The report hashes every evidence file, verifies that each scenario is backed by an actual `make <gate>` target and script, grades candidate attempts by required concepts and safety decision, and rejects missing gate commands or rubric misses. Reproduce the positive, negative, and determinism controls with:

```bash
make practitioner-certification-exam-gate
```
