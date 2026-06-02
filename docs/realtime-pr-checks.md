# Real-time PR checks with sub-ten-second verdicts

Patchline delivers real-time PR checks with sub-ten-second **incremental** verdicts.

## How it works

The worker checks each PR check produced an incremental verdict within the ten-second latency budget.

## What the gate proves

- Every PR verdict returns within ten seconds.
- An over-budget check is rejected.

## Why it matters

Sub-ten-second feedback is what makes the gate part of the review flow instead of a delayed batch job.

## Reproduce

```
make realtime-pr-checks-gate
```
