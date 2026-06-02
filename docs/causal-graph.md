# Causal-graph root-cause attribution

Patchline represents migration-failure analysis as a **causal graph** rather than a flat
list of correlates, computing the set of nodes that are genuine causal ancestors of the
failure outcome so that a **root cause** is separated from a quantity that merely
co-occurs with failure. It verifies the graph is acyclic before drawing any causal
conclusion.

## DAG, then ancestors

The worker validates that the graph is a DAG, computes the transitive ancestors of the
failure node as the root-cause set, and confirms that a node which is only correlated —
present in the graph but with no directed path to the failure — is excluded from that set.
A cyclic graph is rejected.

## Why it matters

Correlation gets blamed for failures it didn't cause. By demanding a directed path to the
failure node and rejecting cycles, attribution stays causal rather than coincidental.

## Reproduce

```
make causal-graph-gate
```
