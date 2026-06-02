# Intervention provenance graph

A generated intervention is only reviewable if a maintainer can answer two questions for **every
line**: *which risk does this line address?* and *what repository evidence justifies it?* This gate
builds an explicit **provenance graph** that makes both answers machine-checkable.

The graph has three node types:

- **line nodes** — each generated intervention line (guard preconditions, bounded-scope
  assertions, rollback obligations).
- **risk nodes** — the real ranked risks the lines guard.
- **source-evidence nodes** — the repository paths (repair migrations, incident files) each line
  was derived from.

Two edge types connect them: `line -> risk` and `line -> evidence`. A line with no risk edge or no
evidence edge is an **orphan** — an untraceable generated line — and is rejected.

Guarantees enforced by the gate:

1. **No orphan lines** — every generated line traces to both a risk ID and at least one
   source-evidence path.
2. **Determinism** — the graph is identical across reruns, so it is diffable in review.
3. **Negative control** — injecting an untraceable line is detected by the graph builder, proving
   the check is not vacuous.

```
make intervention-provenance-graph-gate
```

Outputs land in `results/generated/intervention-provenance-graph/`.
