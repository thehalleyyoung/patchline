# Refinement and signed attestations

Patchline now exposes counterexample-guided refinement and signed artifact verification as deterministic CLI surfaces. The goal is practical CEGAR discipline, not proof theater: start from a coarse repair abstraction, emit the proof holes and counterexamples it cannot discharge, add concrete historical evidence, and rerun the same semantic checks with the refined abstraction.

## CEGAR-style refinement

```bash
go run ./cmd/patchline cegar-refine examples/repairs/repair-bad-invoice-backfill.json \
  --store examples/snapshots/billing-bad-migration-before.json \
  --invariants examples/invariants/billing-core.json \
  --workflow examples/workflows/bad-migration-approved.json \
  --json
```

The report is `patchline.cegar-refinement/v1`. It records:

- iteration 0: row-level replay, solver obligations, and symbolic execution over the bounded snapshot;
- refinements: which proof holes caused extra evidence to be loaded;
- iteration 1: the rerun with invariant and workflow evidence;
- remaining holes and counterexamples, with stable hashes for replay semantics, solver obligations, symbolic execution, and workflow model checking.

The intentionally bad workflow fixture demonstrates counterexample surfacing:

```bash
go run ./cmd/patchline cegar-refine examples/repairs/repair-bad-invoice-backfill.json \
  --workflow examples/workflows/apply-before-approval.json \
  --json
```

That report exposes temporal counterexamples such as apply-before-approval instead of treating the refined model as automatically successful.

## Signed artifact attestations

Patchline signs semantic artifacts with Ed25519 over the artifact bytes' SHA-256 hash plus the subject and public key.

```bash
go run ./cmd/patchline attestation-keygen --json
```

Store the emitted seed in a CI secret or local vault; do not commit it. Then sign and verify a report:

```bash
go run ./cmd/patchline cegar-refine examples/repairs/repair-bad-invoice-backfill.json \
  --store examples/snapshots/billing-bad-migration-before.json \
  --invariants examples/invariants/billing-core.json \
  --workflow examples/workflows/bad-migration-approved.json \
  --json > /tmp/patchline-refinement.json

go run ./cmd/patchline sign-artifact /tmp/patchline-refinement.json \
  --subject cegar:billing-bad-migration \
  --seed-hex "$PATCHLINE_ATTESTATION_SEED" \
  --out /tmp/patchline-refinement.attestation.json

go run ./cmd/patchline verify-artifact /tmp/patchline-refinement.attestation.json \
  --artifact /tmp/patchline-refinement.json
```

Verification fails if the artifact bytes change or the signature/public key pair does not match. This gives CI and incident review a compact way to attach tamper-evident approvals to semantic audits, proof bundles, refinement reports, benchmark gates, or dry-run reports without adding a network dependency.
