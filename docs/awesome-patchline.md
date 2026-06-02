# Awesome Patchline

`scripts/generate-awesome-examples.sh` builds an Awesome Patchline catalog from community-submitted public-code examples. Each example has contributor metadata, ecosystem and framework tags, source host provenance, a pinned ref or archive, and regenerated evidence.

```bash
make awesome-patchline-gate
```

The gate covers two example lanes:

- Full analysis examples across ecosystems such as Ruby/Rails, Python/Django, Python/Alembic, Go SQL migrations, and Node/Prisma.
- Source host examples across GitHub, GitLab, Bitbucket, SourceHut, and release/archive URLs, including Java/Flyway archive provenance and content-addressed cache reuse.

Generated artifacts include `awesome-examples.json`, `awesome-patchline.md`, and a README-ready catalog. The catalog is intentionally regenerated from real public code so new examples cannot be added as unverified badges or screenshots.
