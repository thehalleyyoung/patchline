# Release-channel isolation

Patchline keeps experimental detectors useful without letting them affect **stable certificate results**. The `release-channel-isolation` worker recomputes a stable PLCI-style certificate twice: once from the stable release channel alone and once after experimental detector findings have been injected into the same result pool.

The stable hashes must be byte-identical. The gate also computes a deliberately naive certificate that ignores release channels; that hash must change, proving the experimental findings were non-trivial and would have contaminated the certificate if isolation were broken.

## What the gate proves

- Every detector and result has an explicit `stable` or `experimental` channel tag; missing or unknown channels fail closed.
- Stable and experimental detector identities are disjoint.
- Stable certificates reference only stable-channel detectors and stable-channel result hashes.
- Experimental results remain advisory-only and are excluded from the stable certificate.
- A bad certificate that references an experimental detector is rejected by the stable-only reference check.
- A mutated stable result is rejected by recomputing the certificate hash.

## Reproduce

```bash
make release-channel-isolation-gate
```
