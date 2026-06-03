# Classroom lab kits

Patchline classroom lab kits turn the verifier surface into course material for database, software engineering, programming languages, and DevOps classes.

Each lab is checked as a real artifact: it must cite repo evidence, include a student prompt, name an instructor solution gate, provide `make <gate>` as the reproducing command, list expected outputs, and include a negative control instructors can use to demonstrate failure.

## Reproduce

```bash
go run ./cmd/patchline classroom-lab-kits \
  --spec examples/classroom-lab-kits.json \
  --root . \
  --out results/generated/classroom-lab-kits \
  --json

make classroom-lab-kits-gate
```

The gate runs focused Go tests, renders JSON and Markdown reports, checks all four course audiences, hashes the real evidence files, verifies the instructor solution gates, then mutates the spec so missing coverage, missing evidence, and missing instructor commands are rejected.
