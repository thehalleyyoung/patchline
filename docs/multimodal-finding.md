# Multimodal finding representation

Patchline represents each finding **multimodal**ly — a schema diagram, a textual explanation, and the code span — and asserts the three are mutually consistent.

## How it works

The worker checks every finding carries all three modalities and that they agree on the referenced entity.

## What the gate proves

- All findings are complete and cross-modally consistent.
- A finding whose diagram and code disagree is flagged.

## Why it matters

A diagram, prose, and code that provably refer to the same entity make a finding far easier to trust and act on.

## Reproduce

```
make multimodal-finding-gate
```
