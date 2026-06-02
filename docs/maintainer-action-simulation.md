# Maintainer-action simulation

Empirical evaluation needs a maintainer-decision label per finding so accept/revise/reject
rates can be compared across ecosystems and hazard classes. This gate assigns each ranked
finding one of five **simulated maintainer decisions** using only deterministic signals
already produced by `repo analyze`:

| Decision | Deterministic trigger |
| --- | --- |
| `needs-runtime-evidence` | the finding's proof holes require missing runtime data (row counts, traces, transfer functions) |
| `accept` | high severity **and** linked project evidence — act now |
| `revise` | partial policy controls present, or a conditional repair proof — revise to add the missing guard/rollback/approval/test |
| `reject` | low-signal finding — dismiss |
| `defer` | medium severity with no urgent or runtime trigger |

```
make maintainer-action-simulation-gate
```

The gate downloads real repositories (Rails, Alembic, SQL-infra), runs deterministic
analysis, labels every finding, and proves all five decision classes are produced across
the corpus.

Outputs (`results/generated/maintainer-action-simulation/`):

- `findings.jsonl` — one row per finding with its decision and a maintainer-readable reason.
- `maintainer-action-simulation.json` / `.md` — the label distribution and per-repository breakdown.

These labels let later experiments report calibrated accept/revise/reject/defer rates and
quantify how often Patchline correctly routes a finding to "needs runtime evidence" instead
of overclaiming.
