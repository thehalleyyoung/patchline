# Repository-size stratification

Repository scale and type change the cost and shape of data-change risk: a small app, a
medium service, a large monorepo, and an infrastructure-heavy repo each surface evidence
differently. This gate stratifies the real-repo catalog into four repository-size/type
strata so experiments can report results per size class instead of mixing them:

- **small apps** — repos classed small in the catalog,
- **medium services** — repos classed medium (and large non-monorepo apps),
- **monorepos** — multi-package monorepos,
- **infrastructure-heavy repos** — SQL/DB-infra-centric migrators (Liquibase, Flyway,
  Bytebase, Go SQL migrators, project-native SQL).

```
make repository-size-stratification-gate
```

What it does:

- Classifies every catalog slice into exactly one of the four strata and reports counts.
- Downloads and analyzes a representative real slice from each stratum, reporting the
  measured files, facts, and ranked risks so each stratum is proven end-to-end on real code.

Outputs (`results/generated/repository-size-stratification/`):

- `size-strata.json` — the full catalog classified into the four strata.
- `repository-size-stratification.json` / `.md` — per-stratum counts and the real-code proof table.

This makes per-repository-size reporting first-class and prevents aggregate metrics from
being dominated by one scale of repository.
