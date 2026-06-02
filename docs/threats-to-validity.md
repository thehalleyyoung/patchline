# Threats-to-validity section

Patchline's **threats to validity** section ties every stated threat to a backing experiment from the robustness or ablation suites.

## How it works

The worker checks every threat references an existing backing suite and that the references resolve.

## What the gate proves

- Every threat is backed by a real experiment.
- A threat with no backing experiment is rejected.

## Why it matters

Threats backed by experiments are credible; an unevidenced threats section is just boilerplate.

## Reproduce

```
make threats-to-validity-gate
```
