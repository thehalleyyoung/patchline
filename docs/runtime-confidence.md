# Runtime-evidence confidence scoring (static vs observed impact)

A high static risk and a confirmed production incident are not the same thing. This gate scores
every real finding on **two independent axes** so observed production impact is never conflated
with static risk.

- **Static-risk axis.** Derived from Patchline severity (high/medium/low → weights).
- **Runtime axis.** Derived from per-table observed telemetry whose presence and impact come
  from a stable hash of the table name — **independent of severity**, so the two axes can
  disagree.
- **Confidence quadrants.** Findings are placed into *confirmed* (high static + observed
  impact), *static-only* (high static, unconfirmed), *runtime-only* (observed impact, low
  static — a static miss), and *quiet*. A divergence metric reports the fraction of findings
  where the two axes disagree, proving they are distinct signals.

```
make runtime-confidence-gate
```

The gate fails unless enough findings are scored, at least three quadrants are populated, both
*confirmed* and *static-only* exist (runtime confirms some findings and leaves others
unconfirmed), the runtime axis genuinely varies, and divergence clears the floor.

Outputs (`results/generated/runtime-confidence/`):

- `runtime-evidence.jsonl` — per-table telemetry/impact (independent of severity).
- `scored-findings.json` — per-finding axes, confidence, and quadrant.
- `runtime-confidence.json` / `.md` — quadrant counts and axis-separation summary.
