# Database-semantics reproducibility report

`patchline db-semantics-reproducibility` aggregates real `db-semantics` JSON reports into one deterministic audit artifact. It is meant for reviewers who need to know exactly which engine/version profiles, runtime images, and observed behavior rows support a database-semantics claim.

```bash
go run ./cmd/patchline db-semantics-reproducibility \
  --report results/generated/db-version-semantics/postgres15.json \
  --report results/generated/db-version-semantics/mysql80.json \
  --out results/generated/db-version-semantics/reproducibility-report.json \
  --markdown results/generated/db-version-semantics/reproducibility-report.md \
  --json
```

The builder sorts input reports by engine, resolved version, source, and report hash before hashing, rejects duplicate engine/version inputs, and requires at least one report for every supported engine. The output records:

- source report hashes, input SQL hashes, runtime hint hashes, and statement counts;
- engine/version pins plus runnable container image tags where a local engine image exists;
- embedded or managed-service profiles for SQLite, BigQuery, and Snowflake without pretending those are ordinary production containers;
- profile evidence, statement rules, lock simulations, smoke fixtures, unsafe counter-profile controls, rollback feasibility, query-plan, runtime, replication-lag, and partition/sharding observations.

Reproduce it with:

```bash
make db-semantics-reproducibility-gate
```

The gate generates fresh per-engine reports from `examples/db-version-semantics/semantics.sql`, builds the reproducibility report, and checks that the report covers all eight engines, ten engine/version pins, concrete image references, observed behavior rows, negative controls, rollback evidence, and query-plan evidence. 
