# Open textbook companion

`patchline open-textbook-companion` validates the education material as executable notebooks. Each chapter names source evidence, an `.ipynb` file, the exact command that regenerates a teaching example, expected JSON/Markdown outputs, learning objectives, and a negative control.

```bash
go run ./cmd/patchline open-textbook-companion \
  --spec examples/open-textbook-companion.json \
  --root . \
  --out results/generated/open-textbook-companion \
  --json

make open-textbook-companion-gate
```

The gate first runs the notebook regeneration commands for classroom lab kits, reviewer skills, and localized teaching examples. It then hashes the notebooks, source specs, docs, and regenerated reports, and mutates the spec so missing chapters, missing notebook commands, stale generated artifacts, missing evidence, and absent negative controls fail.
