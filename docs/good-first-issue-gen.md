# Good-first-issue generator

Patchline seeds **good first issue** suggestions from real gaps in the gate catalog, proposing each as a scoped task tied to a concrete missing gate.

## How it works

The worker filters catalog entries to those with an identified gap, checks each issue has a scope and a backing gap reference, and counts actionable issues.

## What the gate proves

- Every generated issue references a real gap and is scoped.
- A fabricated issue with no backing gap is rejected.

## Why it matters

Newcomers contribute when issues are real, scoped, and tied to a concrete gap rather than vague wishes.

## Reproduce

```
make good-first-issue-gen-gate
```
