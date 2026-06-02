# External-auditor reproduction with signed attestation

Patchline carries an external-auditor reproduction of the headline result with a signed **attestation**.

## How it works

The worker checks each reproduced result was confirmed by the auditor and carries a non-empty signature.

## What the gate proves

- Every headline result is independently reproduced and signed.
- An unsigned reproduction is rejected.

## Why it matters

An independent signed attestation is the strongest credibility signal a results claim can carry.

## Reproduce

```
make external-auditor-repro-gate
```
