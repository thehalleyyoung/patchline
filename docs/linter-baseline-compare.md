# Baseline linter comparison

Patchline measures its accuracy against existing migration **linters** on exactly matched
inputs, so a recall improvement reflects a better analysis rather than an easier test set,
and any input mismatch between tools is caught before numbers are compared.

## Matched inputs, per-tool recall

The worker scores each tool's predictions against shared gold labels over the identical case
set, computes per-tool recall on the hazardous cases, and verifies every tool was run on the
same inputs.

## What the gate proves

- Patchline's recall (1.0) dominates both baseline linters on matched inputs.
- A baseline evaluated on a different case set is rejected as an unmatched comparison.

## Why it matters

A head-to-head number only means something when both tools saw the same inputs. The matched
harness makes the comparison fair and the win real.

## Reproduce

```
make linter-baseline-compare-gate
```
