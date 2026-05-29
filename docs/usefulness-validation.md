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
