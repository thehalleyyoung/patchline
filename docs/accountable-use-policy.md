# Accountable-use policy

Patchline ships an accountable-use policy that maps autonomous or blocking capabilities to required **human oversight** before they can affect a maintainer decision.

## How it works

The policy treats advisory-only capabilities differently from autonomy or blocking gates. Advisory capabilities may run in shadow mode, but any capability that can release, block, merge, page, or change high-stakes policy must name:

- the accountable human role,
- the decision point where review is required,
- the review artifact that must preserve rationale, and
- the reproducible gate that backs the capability.

## What the gate proves

- Every autonomous or blocking capability in the frozen spec has a role, decision point, review artifact, and gate-backed evidence.
- Advisory-only capabilities are documented without forcing fake approval steps.
- A blocking rule with no human oversight mapping is rejected.

## Why it matters

Patchline can analyze risky data changes, propose interventions, and enforce gates, but it must not hide accountability behind automation. The accountable-use gate keeps autonomy, blocking policy, and human review coupled in the artifact itself.

## Reproduce

```
make accountable-use-policy-gate
```
