# Independent third-party security audit

Patchline carries an independent third-party security audit with all findings **remediated** and re-verified.

## How it works

The worker checks each audit finding was remediated and independently re-verified as closed.

## What the gate proves

- Every finding is remediated and re-verified.
- An open finding is rejected.

## Why it matters

An independent audit with every finding closed and re-verified is the credible security bar for adoption.

## Reproduce

```
make third-party-security-audit-gate
```
