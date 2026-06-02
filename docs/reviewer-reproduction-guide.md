# One-page reviewer reproduction guide

Patchline includes a one-page reviewer reproduction guide that regenerates the headline result **in minutes** with a single command path.

## How it works

The worker checks the guide fits a one-page step budget, has a measured runtime within the minutes bound, and ends at the headline result.

## What the gate proves

- The guide is within the page and time budget and reaches the headline result.
- An over-length guide exceeding the step budget is rejected.

## Why it matters

A reviewer who can confirm your headline result in three steps and three minutes is a reviewer on your side.

## Reproduce

```
make reviewer-reproduction-guide-gate
```
