# Paper appendix generator

Patchline can render a paper appendix from current generated artifacts: claims, limitations, figures, tables, and reproduction commands are extracted from regenerated public-code evidence instead of copied into a hand-maintained appendix.

```bash
make paper-appendix-gate
```

The generator writes:

- `appendix.md`: reviewer-readable appendix with claims, limitations, figures, tables, and reproduction commands.
- `appendix.json`: machine-readable appendix with source artifact paths and summary counts.
- `tables/*.md` and `tables/*.json`: artifact summary, risk taxonomy, claim status, limitation categories, and reproduction-command tables.
- `evidence/artifact-container-rebuild/*`: the current public-code evidence used to populate the appendix.

The gate proves the appendix against pinned public repositories and checks that every paper-facing section remains tied to generated evidence.
