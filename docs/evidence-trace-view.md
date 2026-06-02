# Evidence-trace explanation view

Patchline traces every verdict to its **supporting evidence** as an interactive view, where each conclusion links to the facts and rules that justify it down to source spans.

## How it works

The worker verifies the evidence graph resolves every dependency, that leaves ground in a source span, and reruns the check on a dangling graph.

## What the gate proves

- The trace is complete and grounded.
- A verdict with a dangling, ungrounded evidence node is rejected.

## Why it matters

An auditable evidence trace lets a reviewer verify the reasoning instead of taking the verdict on faith.

## Reproduce

```
make evidence-trace-view-gate
```
