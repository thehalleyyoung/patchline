# Migration-age stratification

Patchline stratifies real public migrations by **age band** and **change type** so
empirical results can separate recent, old, backfill-heavy, and schema-only changes
instead of reporting only aggregate numbers.

- **Age band** ranks each migration by its own ordinal/timestamp prefix within its
  repository (Rails 14-digit timestamps, Django/Alembic numeric ordinals). The newer
  half of a repository's migrations are labelled `recent`; the older half `old`.
- **Change type** classifies a migration as `backfill-heavy` when Patchline's ranked
  risks include data writes (`update`/`delete`/`insert` code paths) and `schema-only`
  when the migration only performs DDL (create/alter/drop/column/table).

The gate downloads three real public slices (Rails, Django, Alembic), runs deterministic
`repo analyze` inventory + baseline stages on each, and reports per-stratum ranked-risk
density:

```
make migration-age-stratification-gate
```

Outputs (`results/generated/migration-age-stratification/`):

- `migration-age-stratification.json` — strata, age x change-type cross-tab, per-repository counts.
- `migration-age-stratification.md` — maintainer-readable tables including **risk density** per stratum.
- `migrations.jsonl` — one row per real migration with its age key, age band, change type, and ranked-risk counts.

This lets experiments report effect sizes per stratum and exposes whether backfill-heavy
changes carry higher risk density than schema-only changes on real repositories.
