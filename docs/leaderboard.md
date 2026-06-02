# Benchmark leaderboard

Patchline tracks its own progress as a **benchmark leaderboard** that compares releases
over time on the metrics that matter — number of proven gates, determinism rate, and
reproduction rate — ranking releases and computing the delta between each release and its
predecessor so that improvement and **regress**ion are both visible rather than hidden
behind a single headline number.

## Rank and diff

The worker ingests an ordered release history, builds a ranked leaderboard on the primary
metric, computes per-release deltas, and flags any release whose reproduction rate
regressed against the prior release.

## Why it matters

A leaderboard that only shows the latest number rewards cherry-picking. By keeping the
full timeline and flagging regressions, progress claims stay honest across releases.

## Reproduce

```
make leaderboard-gate
```
