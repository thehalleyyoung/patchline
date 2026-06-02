# Contributor badges

Patchline renders contributor recognition **badges** exclusively from
**gate-backed** data, so a badge can never claim credit for a capability that is
not proven by a real gate.

## How recognition is computed

Each contributor lists the capabilities they shipped. The badge worker:

- keeps only the contributions whose gate script (`scripts/<capability>-gate.sh`)
  actually exists — unbacked claims are silently dropped;
- computes a deterministic **recognition tier** (bronze/silver/gold) as the highest
  tier whose threshold is met by the count of gate-backed contributions;
- emits a shields-style badge endpoint per contributor plus a Markdown wall of fame.

## Why it stays honest

The gate asserts that every badged contribution maps to a real gate script, that
tiers are **monotonic** in the verified count (more gate-backed work never yields a
lower tier), and that a contributor listing a capability with no gate is not
credited for it. Recognition therefore tracks proven impact, not self-reported
claims.

## Reproduce

```
make contributor-badges-gate
```
