# Historical failure counterfactuals

Patchline's historical-failure suite turns public postmortems into falsifiable counterfactual checks. Each case separates ground truth from analysis:

1. **Source assertions** are exact phrases from public incident reports, verified by `scripts/verify-historical-sources.sh`.
2. **Patchline artifacts** are semantic reconstructions of the failure class, not private operational data.
3. **Expected signals** are deterministic findings that Patchline must produce before the case is considered validated.

Run:

```bash
go run ./cmd/patchline historical-failures examples/historical-failures/suite.json --json
bash scripts/verify-historical-sources.sh examples/historical-failures/suite.json
```

Current cases:

| Case | What the public source establishes | What Patchline proves or flags |
| --- | --- | --- |
| `gitlab-2017-primary-db-removal` | Accidental primary-database data removal, production-data loss across projects/comments/users/issues/snippets, and failed backup recovery. | Destructive protected-table mutations, downstream damaged-report impact, and lack of snapshot rollback. |
| `github-2018-mysql-split-brain` | Divergent writes across database sites and stale/inconsistent user-visible state. | Conflicting writes to the same logical record from separate regions plus downstream damaged report state. |

The suite's claim is intentionally narrow: Patchline would have blocked or escalated the encoded class of unsafe operation before certifying recovery. It does not claim complete prevention of every contributing factor in the incident.
