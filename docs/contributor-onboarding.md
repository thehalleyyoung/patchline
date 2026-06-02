# Contributor onboarding

Patchline ships a contributor-onboarding path that, in **one script**, builds the tool, runs the tests, and produces a first analysis verdict.

## How it works

The worker verifies the onboarding plan contains the build, test, and first-analysis stages in order and that each has a runnable command.

## What the gate proves

- All three onboarding stages are present and runnable.
- A plan missing the test stage is rejected.

## Why it matters

A one-script path to a green build and a first verdict is what converts a curious developer into a contributor.

## Reproduce

```
make contributor-onboarding-gate
```
