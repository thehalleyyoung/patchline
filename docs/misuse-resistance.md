# Misuse-resistance analysis

Patchline now gate-checks the places where success claims can be gamed: **certificates**, **scoreboards**, and **adoption metrics**.

## What it checks

`patchline misuse-resistance` loads a versioned adversarial-scenario file, hashes every cited evidence file, and fails closed when:

- a required surface is missing;
- an adversarial scenario lacks independent reviewers, control diversity, accountable owners, public-safe failure modes, or fresh review;
- a control has no readable evidence under the repository root;
- a reproduced attack simulation is absent or does not show the expected fail-closed outcome.

The report uses an explicit `as_of_date`, so stale-review checks and report hashes are reproducible instead of depending on wall-clock time.

## Reproduce

```bash
go run ./cmd/patchline misuse-resistance \
  --spec examples/misuse-resistance.json \
  --root . \
  --out results/generated/misuse-resistance \
  --json

make misuse-resistance-gate
```
