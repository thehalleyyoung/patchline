# Verified expand/contract migration templates

Patchline's **verified expand/contract migration templates** turn a
`patchline.invariants/v1` declaration into a staged remediation skeleton:
expand a nullable compatibility surface, backfill it, validate the invariant,
then contract only after the invariant and backfill obligations have evidence.

The command checks each template against ORM project evidence without embedding
source snippets in the report. Evidence is summarized as stable relative paths,
line numbers, phase names, and missing obligations.

```bash
go run ./cmd/patchline expand-contract-template \
  --spec examples/expand-contract-gate.json \
  --out results/generated/expand-contract-templates-gate \
  --json
```

The gate covers Rails ActiveRecord, Django migrations/models, and Prisma
schema/migration projects, then reruns a negative control that omits the
backfill migration and must be refuted by the ORM scanner.

```bash
make expand-contract-templates-gate
```
