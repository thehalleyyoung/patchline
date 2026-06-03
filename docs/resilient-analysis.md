# Resilient distributed analysis

`patchline resilient-analysis` verifies that a distributed corpus or repo analysis run remains auditable when infrastructure fails. It is intentionally separate from the existing work-queue and work-stealing gates: those prove deterministic assignment, while this mode proves fault tolerance after assignment.

```bash
go run ./cmd/patchline resilient-analysis \
  --spec examples/resilient-analysis.json \
  --root . \
  --out results/generated/resilient-analysis \
  --json
```

The verifier replays worker leases, accepted task completions, cache artifacts, cache quarantine/rebuild events, and partial network partitions. A passing report proves:

- a task leased by a lost worker is reassigned to a surviving worker and then accepted;
- stale cache bytes are detected by hashing the real fixture, quarantined, rebuilt, and only accepted when the rebuilt hash matches the trusted result;
- tasks affected by a recovered partition still make accepted progress;
- accepted completions are unique and lease IDs are deterministic.

The reproducible gate injects all major failure classes and expects distinct counterexamples:

```bash
make resilient-analysis-gate
```
