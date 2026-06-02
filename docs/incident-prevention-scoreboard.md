# Incident-prevention scoreboard

Patchline maintains a public incident-prevention scoreboard aggregating **anonymized** adopter outcomes, with privacy-safe entries and consistent totals.

## How it works

The worker sums the per-adopter prevented-incident counts, checks the published total matches, and confirms no entry carries identifying information.

## What the gate proves

- The aggregate total is consistent and privacy-safe.
- An entry leaking an adopter identity is flagged.

## Why it matters

A privacy-safe, internally consistent scoreboard makes impact visible without exposing any adopter.

## Reproduce

```
make incident-prevention-scoreboard-gate
```
