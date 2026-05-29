# Patchline semantic pipeline

Patchline's artifact story is a pipeline from public or local evidence to reusable semantic memory.

```mermaid
flowchart LR
  A[Evidence<br/>events, SQL, manifests, public sources] --> B[Trace<br/>trace-reconstruct]
  B --> C[Transition<br/>analyze-migration / migration-semantics]
  C --> D[Repair<br/>repair-semantics / dry-run]
  D --> E[Proof<br/>solver-obligations with Z3]
  E --> F[Replay<br/>row diffs and verification hashes]
  F --> G[Archive<br/>archive-index / archive-query]
  G --> H[Regression<br/>semantic-regressions]

  P1[pre-deploy] -. migration, schema, policy, prior archive .-> C
  P2[during repair] -. bad state, repair plan, rollback facts .-> D
  P3[postmortem/archive] -. source-grounded incident facts .-> G
```

## Stage-to-command map

| Stage | Current command | Artifact emitted | Phase |
| --- | --- | --- | --- |
| Evidence | `historical-failures`, `ingest-evidence`, `extract-sql` | source-grounded observations and event graphs | postmortem, during migration |
| Trace | `trace-reconstruct` | projection hash and event ordering summary | during migration |
| Transition | `analyze-migration`, `migration-semantics` | risk report, statement effects, schema semantics | pre-deploy |
| Repair | `repair-semantics`, `dry-run` | step trace, row diffs, repair hash | during repair |
| Proof | `solver-obligations` | Z3-backed obligation report or explicit downgrade | during repair |
| Replay | `repair-outcomes` | dry-run/applied SQL/verification/rollback history | during repair, archive |
| Archive | `archive-index`, `archive-query` | deterministic incident index and query hashes | archive-only |
| Regression | `semantic-regressions` | recurrence candidates and learned invariants | archive-only |

## Reviewer contract

The artifact path should never require a reviewer to infer which layer a command belongs to. Every runnable demonstration should identify:

1. input artifacts,
2. phase,
3. command,
4. emitted hash or result file,
5. claim supported by that result.
