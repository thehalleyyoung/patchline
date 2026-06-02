# Office-hours triage rotation

Patchline documents a public office-hours and triage **rotation**, gate-verified to have full calendar coverage with no unstaffed slot.

## How it works

The worker checks every scheduled slot has an assigned maintainer and that no maintainer covers two slots simultaneously.

## What the gate proves

- Coverage is complete with no conflicts.
- A schedule with an unstaffed slot is rejected.

## Why it matters

Dependable, gate-verified support coverage is how a project earns the trust of adopters who need help.

## Reproduce

```
make office-hours-rotation-gate
```
