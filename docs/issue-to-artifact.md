# Issue-to-artifact automation

Patchline turns accepted user-submitted examples into **pinned public proof
entries** through a deterministic, auditable pipeline that refuses to admit
anything that is not reproducible.

## The admission contract

Each submission names a capability, a public repository, a pinned ref, and a
claim. A submission is admitted **only if**:

- the **ref is pinned** — a 40-character commit SHA or an explicit `refs/tags/...`
  ref (a bare branch like `main` is rejected as `unpinned-ref`);
- the **capability maps to a real gate** — `scripts/<capability>-gate.sh` exists
  (otherwise rejected as `unknown-capability`);
- the submission carries a non-empty **claim** (otherwise `missing-claim`).

Admitted submissions become proof entries with a deterministic content-derived id,
written under `proofs/` and collected into a sorted `proofs-index.json` plus a
human-readable `proofs.md`. Every other submission is **rejected** with a
structured reason.

## Negative controls

The example deliberately includes an unpinned submission and an unknown-capability
submission; the gate asserts that both are always rejected, so the automation can
never silently admit an unreproducible proof.

## Reproduce

```
make issue-to-artifact-gate
```
