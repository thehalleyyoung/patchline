# Proof-hole dependency graph

Patchline models the **proof hole**s behind an uncertain risk as a dependency graph and
computes the smallest, dependency-respecting set of evidence whose acquisition would
lower the risk's uncertainty below a target, so that a reviewer is told the cheapest
concrete way to make a finding trustworthy rather than a vague request for more
information.

## Selection

The worker enumerates evidence subsets, keeps only those whose dependencies are
satisfied and whose combined uncertainty reduction meets the target, and selects the
**minimum-cost** feasible set (tie-broken by fewer items, then lexicographic ids).

## Why it stays honest

The gate proves the selected set reaches the target at minimum cost — here `{A, B}` at
cost 2, cheaper than the single high-cost `{C}` — respects evidence dependencies (`B`
requires `A`), and never includes the irrelevant zero-reduction item `D`.

## Reproduce

```
make proof-hole-graph-gate
```
