# Public roadmap board

Patchline's public roadmap board is generated from pinned public-code failure evidence. Every planned feature card must link to a real-repo failure mode, a proof gate, and an expected artifact.

```bash
make roadmap-board-gate
```

The generated board includes:

- `roadmap-board.json`: machine-readable roadmap cards, linked failure modes, gates, expected artifacts, owners, and stages.
- `roadmap-board.md`: README-ready public board.
- `cards/*.md`: one card per planned feature with a linked public-repo failure example.
- `taxonomy/failure-taxonomy.json`: regenerated failure-mode evidence used by the board.

The gate rejects roadmap items that do not have a `make ...` gate, a concrete expected artifact path, and a linked public-code failure mode from the regenerated taxonomy.
