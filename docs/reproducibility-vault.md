# Long-term reproducibility vault

Patchline keeps a long-term reproducibility vault that **snapshot**s the toolchain, corpus, and results per release.

## How it works

The worker checks each release snapshot bundles all three components with content digests and that the digests verify.

## What the gate proves

- Every release snapshot is complete and verifiable.
- A snapshot missing the corpus component is rejected.

## Why it matters

Snapshotting everything per release means a result stays reproducible long after the dependencies move on.

## Reproduce

```
make reproducibility-vault-gate
```
