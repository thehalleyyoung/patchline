# LLM-judge harness

Patchline runs an LLM-judge harness with a deterministic scoring rubric and reports **inter-rater** agreement.

## How it works

The worker computes the agreement rate between two judges over the rubric-scored items and checks it meets a minimum reliability threshold.

## What the gate proves

- The judges agree above threshold under the rubric.
- A pair of judges scoring at chance is flagged as unreliable.

## Why it matters

LLM judgments are only usable as evidence when anchored to a rubric and shown to be reproducibly consistent.

## Reproduce

```
make llm-judge-harness-gate
```
