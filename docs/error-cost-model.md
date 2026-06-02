# Cost-of-error model

Patchline scores analyzer configurations by an **expected cost** that weights each missed
hazard by its historical incident **severity**, so a configuration that misses one
catastrophic data-loss hazard is correctly ranked worse than one that misses several
cosmetic ones — even though it has fewer raw misses.

## Severity-weighted misses

The worker multiplies each false negative by its severity weight, sums the expected cost per
configuration, and compares the cost ranking against the naive raw-miss-count ranking.

## What the gate proves

- `config_a` (one critical miss, cost 100) is the worst by expected cost.
- `config_b` (four cheap misses, cost 23) is the worst by raw count.
- The severity-weighted ranking **flips** the naive one.

## Why it matters

Not all misses are equal. A cost model aligns the tool's optimization target with the real
business risk of an incident.

## Reproduce

```
make error-cost-model-gate
```
