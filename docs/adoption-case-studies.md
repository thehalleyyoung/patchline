# Adoption case studies

Patchline ships adoption **case studies** showing teams using it alongside their
existing CI, **observability**, and migration tooling, with each narrative anchored
to the specific gate-backed capabilities the team relied on rather than to marketing
claims.

## Anatomy of a case study

Every case study declares:

- the **integration context** (`ci`, `observability`, or `migration`),
- the **capabilities used** (each a `make <gate>` target), and
- a measurable **outcome**.

The worker renders one narrative per study plus an integration matrix.

## Why it stays honest

The gate asserts that the CI, observability, and migration integration contexts are
all represented and that **every** capability cited by a case study maps to a real
gate. A case study therefore cannot credit a capability the repository cannot prove.

## Reproduce

```
make adoption-case-studies-gate
```
