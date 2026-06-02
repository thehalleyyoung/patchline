# Admin analytics dashboard of prevented incidents

Patchline provides an admin dashboard tying findings to **prevented-incident** estimates.

## How it works

The worker checks each dashboard metric references a backing prevented-incident estimate.

## What the gate proves

- Every dashboard metric is estimate-backed.
- An unbacked metric is rejected.

## Why it matters

Showing prevented incidents, not raw finding counts, is what justifies the tool to leadership.

## Reproduce

```
make admin-analytics-dashboard-gate
```
