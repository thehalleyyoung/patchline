# Resource budgets

Patchline runs under explicit per-stage **resource budget**s so that an analysis can
never silently consume unbounded time, memory, or file handles on an adversarial
public repository.

## Budgets

Each stage declares a budget for wall-clock seconds, peak memory in megabytes, and
files opened. The worker measures each stage's actual usage against its budget and
admits a run only when every stage is within budget.

## Why it stays honest

The gate proves a within-budget run is admitted and that an **over-budget** negative
control is rejected at the offending stage — here the `analyze` stage overruns its
memory budget. The rejection names the specific stage and resource that overran, so
a budget violation is always actionable rather than a generic timeout.

## Reproduce

```
make resource-budgets-gate
```
