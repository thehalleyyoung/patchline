# Robustness against an automated adversary

Patchline runs a robustness evaluation against an automated **adversary** searching for evasions.

## How it works

The worker checks every adversarially-mutated migration is still flagged by the gate.

## What the gate proves

- Every adversarial evasion attempt is caught.
- A successful evasion is rejected as a failure.

## Why it matters

An adversary that actively searches for holes is a far harder test than a static benchmark.

## Reproduce

```
make adversarial-robustness-gate
```
