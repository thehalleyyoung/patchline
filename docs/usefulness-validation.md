# Usefulness validation

Patchline is useful now if it can produce reviewable, replayable evidence for a realistic production-data incident and if its analyzers can be checked against real public migration code rather than only hand-written toy examples.

The repository exposes that claim through one reproducible target:

```bash
z3 --version
make verify-usefulness
```

`make verify-usefulness` runs:

| Stage | What it validates |
| --- | --- |
| `go test ./...` | Core packages, CLI behavior, replay, provenance, policies, model checking, archive indexing, and Z3-backed solver integration. |
| `ci-gate` | Strict analyzer precision/recall thresholds on pinned migration fixtures. |
| `public-corpus` | Downloads pinned public SQL migrations, verifies SHA-256, and checks expected analyzer report hashes. |
| `solver-obligations` | Uses Z3 to prove the sample repair predicate is contained in the declared repair scope. |
| `semantics-audit` | Rebuilds the semantic evidence bundle and expects all artifacts to conform with zero counterexamples. |
| `archive-index` | Indexes historical-style evidence, migrations, repairs, policy, and benchmark outputs by semantic shape and decisions. |
| `archive-query` | Answers deterministic historical questions about broad-update migrations, damaged-derived reports, and repairs lacking rollback. |
| `historical-failures` | Replays public postmortem counterfactual artifacts and requires expected Patchline signals for known failure classes. |
| `verify-historical-sources.sh` | Fetches the public postmortems and verifies the exact source phrases used as ground-truth assertions. |

## Public corpus

The public corpus is described in `examples/public-corpus/sources.json`. The files are downloaded from pinned raw URLs in the public `bytebase/bytebase` repository at commit `47d2522552ce44271680424bf31a4cddd8a50ab1` and verified before analysis. The SQL files are not vendored here; only source URLs, output names, and SHA-256 hashes are committed.

After downloading, Patchline runs:

```bash
go run ./cmd/patchline benchmark-suite examples/benchmarks/public-bytebase-migration-corpus.json
```

The benchmark spec pins each expected migration-analysis report hash. A changed parser, analyzer, or upstream input changes the hash and fails validation instead of silently moving the target.

## Historical-style incident validation

The bundled billing incident is intentionally small, but it exercises the same artifact chain needed for production repair review:

1. ingest operational evidence,
2. reconstruct typed traces and causal provenance,
3. analyze a risky migration,
4. validate a scoped repair manifest,
5. prove scope containment with Z3,
6. replay the repair and check invariants,
7. model-check the approval/apply workflow,
8. emit a semantic audit,
9. sign or verify artifacts,
10. archive the incident by semantic shape and repair effect,
11. query archived facts for broad updates, damaged reports, and rollback gaps.

This makes the claim falsifiable: if any artifact cannot be rebuilt, if Z3 cannot prove the scope obligation, if a policy or benchmark fails, or if a counterexample appears, the validation command fails or downgrades the relevant claim in the JSON.

## Public historical-failure counterfactuals

The suite in `examples/historical-failures/suite.json` is deliberately conservative: it does not claim Patchline reproduces every operational detail in a postmortem. Instead, each case has two independently checkable parts:

1. public-source assertions, verified by fetching the postmortem and matching exact phrases;
2. Patchline artifacts that encode the relevant failure class and must produce expected deterministic signals.

The current cases validate:

| Case | Publicly sourced failure class | Patchline signal |
| --- | --- | --- |
| `gitlab-2017-primary-db-removal` | Accidental removal from a primary database, production-data loss, and failed backup recovery. | High-risk destructive mutations on protected primary-data tables, damaged derived reports, and missing snapshot rollback. |
| `github-2018-mysql-split-brain` | Divergent writes between database sites plus stale/inconsistent user-visible state. | Split-brain conflicting writes on one logical record and damaged derived report state. |

Run the suite directly with:

```bash
go run ./cmd/patchline historical-failures examples/historical-failures/suite.json --json
bash scripts/verify-historical-sources.sh examples/historical-failures/suite.json
```

The valid claim is counterfactual and scoped: Patchline would have blocked or flagged the encoded failure class under its proof/policy gates. The suite intentionally does not claim full incident prevention unless the postmortem evidence and artifact model support that stronger statement.
