# Governance and versioning policy

Patchline documents a governance policy with semantic versioning, a minimum **deprecation** window, and architecture decision records.

## How it works

The worker checks the version follows semver, the deprecation window meets the minimum, and each breaking change links to a decision record.

## What the gate proves

- The policy is satisfied with adequate deprecation and recorded decisions.
- A breaking change under the minimum deprecation window is rejected.

## Why it matters

Predictable versioning and deprecation are what let downstream users depend on a tool long-term.

## Reproduce

```
make governance-policy-gate
```
