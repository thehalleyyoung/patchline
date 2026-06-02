# Cross-repository dependency hazard analysis

Patchline runs **cross-repository** dependency analysis detecting hazards that span multiple services.

## How it works

The worker checks every cross-service dependency edge flagged as a hazard carries concrete supporting evidence.

## What the gate proves

- Every detected cross-repo hazard is evidence-backed.
- An evidence-free detection is rejected.

## Why it matters

Most real outages cross a service boundary; single-repo analysis structurally cannot see them.

## Reproduce

```
make cross-repo-dependency-analysis-gate
```
