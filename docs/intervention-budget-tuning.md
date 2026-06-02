# Intervention budget tuning study

Generated interventions are deliberately bounded. The `--budget files=N,lines=N,tokens=N,changes=N`
flag caps generation scope, but what should those numbers be? This study answers that empirically
instead of by guesswork.

Each real ranked risk is assigned a deterministic intervention cost vector:

- `files` = distinct evidence paths, `lines` = `2 + repair_paths`, `tokens` = `6 × lines`,
  `changes` = `1`.

The study then sweeps each of the four **budget tuning** dimensions — files, lines, tokens, changes
— across budget levels (0/20/40/60/80/100% of the total cost of covering every risk) and measures
how many risks a highest-score-first generator covers within each budget.

Guarantees enforced by the gate:

1. **Monotonic** — coverage never decreases as the budget grows, for every dimension.
2. Zero budget covers nothing; full budget covers every risk.
3. Every dimension has a diminishing-returns **knee** (first level reaching ≥90% coverage), the
   recommended budget setting.
4. Determinism across reruns.

```
make intervention-budget-tuning-gate
```

Outputs land in `results/generated/intervention-budget-tuning/`.
