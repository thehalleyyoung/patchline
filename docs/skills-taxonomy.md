# Reviewer skills taxonomy

`patchline skills-taxonomy` validates a public, gate-backed map from each declared hazard class to the concepts reviewers must understand: prerequisites, assessment prompts, role audiences, evidence paths, reproducing gates, negative controls, and crosswalks into tutorials or certification scenarios.

```bash
go run ./cmd/patchline skills-taxonomy \
  --spec examples/skills-taxonomy.json \
  --root . \
  --out results/generated/skills-taxonomy \
  --json

make skills-taxonomy-gate
```

The gate proves the taxonomy is executable curriculum rather than slideware: five real hazard classes cover app developers, DBAs, SREs, security reviewers, and engineering managers; each hazard has at least two reviewer concepts, hashed evidence, an exact `make <gate>` command, a failing negative control, and a tutorial/certification crosswalk. A mutated spec drops concepts, evidence, gates, prerequisites, and crosswalks to prove the report fails loudly.
