# Paper claim freeze

Patchline freezes the evidence behind every paper claim into a checksum manifest at
submission time, so that any post-submission **drift** in a cited artifact is detected by
re-verification instead of silently changing what the paper asserts. This **claim
freeze** is itself a gate.

## Freeze and verify

The worker canonicalizes and checksums each claim's backing artifact into a freeze file,
then re-verifies the live artifacts against the freeze — reporting no drift when they are
unchanged and detecting drift when an artifact is tampered with.

## Why it stays honest

A frozen claim must be detectably broken if its evidence changes. The gate proves the
unchanged artifacts re-verify cleanly against the freeze while a tampered copy produces a
checksum mismatch that is flagged as **drift** — so reviewers can trust that the evidence
they read is the evidence that was submitted.

## Reproduce

```
make claim-freeze-gate
```
