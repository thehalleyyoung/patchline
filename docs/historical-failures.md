# Historical failure counterfactuals

Patchline's historical-failure suite turns public incident records into falsifiable counterfactual checks. Each case separates ground truth from analysis:

1. **Source documents** are public postmortems, public issues, or public API records that define where each ground-truth assertion comes from.
2. **Source assertions** are exact phrases from those documents, verified by `scripts/verify-historical-sources.sh`.
3. **Source observations** are JSONL facts derived from verified assertions, such as primary-data loss, recovery gaps, or follow-up remediation work.
4. **Patchline artifacts** are semantic reconstructions of the failure class, not private operational data.
5. **Expected signals** are deterministic findings that Patchline must produce before the case is considered validated.

Run:

```bash
go run ./cmd/patchline historical-failures examples/historical-failures/suite.json --json
bash scripts/verify-historical-sources.sh examples/historical-failures/suite.json
```

Current cases:

| Case | What the public source establishes | What Patchline proves or flags |
| --- | --- | --- |
| `gitlab-2017-primary-db-removal` | Accidental primary-database data removal, production-data loss across projects/comments/users/issues/snippets, failed backup recovery, plus linked public work on production/staging differentiation, backup monitoring, PITR, hourly snapshots, restore testing, staging migration rollback, and hard-delete policy. | Destructive protected-table mutations, downstream damaged-report impact, lack of snapshot rollback, and source-grounded observations that map the public follow-up issues to Patchline recovery/rollback obligations. |
| `github-2018-mysql-split-brain` | Divergent writes across database sites and stale/inconsistent user-visible state. | Conflicting writes to the same logical record from separate regions plus downstream damaged report state. |

The suite's claim is intentionally narrow: Patchline would have blocked or escalated the encoded class of unsafe operation before certifying recovery. It does not claim complete prevention of every contributing factor in the incident. The GitLab case is now source-derived from a cluster of public documents rather than only a single prose postmortem, so the report can distinguish source-established historical facts from Patchline's counterfactual semantic checks.
