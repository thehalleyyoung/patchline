# Ecosystem-balanced benchmark slices

Aggregate benchmark numbers hide whether a tool works only on one popular ecosystem.
This gate builds an **ecosystem-balanced** benchmark manifest that gives **equal
representation** to nine migration frameworks — Rails Active Record, Django migrations,
Alembic, Prisma, TypeORM, Liquibase, Flyway, EF Core, and Go migrators — so experiments
can report per-framework results instead of leaning on one ecosystem.

```
make ecosystem-balanced-benchmark-gate
```

What it does:

- **Balance audit.** It reads the real-repo catalog, groups slices by migration
  framework, and selects an equal number of slices per framework (`balanced_count`),
  yielding a perfectly balanced manifest with runnable `repo analyze` commands and pinned
  refs.
- **Real-code proof sample.** It downloads and analyzes a diverse proof sample spanning
  Rails, Django, Alembic, Flyway, Go, TypeORM, and Liquibase, confirming each produces
  files, facts, and ranked risks against real downloaded code.

Outputs (`results/generated/ecosystem-balanced-benchmark/`):

- `balanced-manifest.json` — per-framework groups and the balanced selection.
- `benchmark-manifest.jsonl` — one runnable analyze command per balanced slice.
- `ecosystem-balanced-benchmark.json` / `.md` — balance audit plus the real-code proof table.

This makes per-ecosystem reporting first-class and guards against over-reliance on any
single migrator when comparing Patchline releases or baselines.
