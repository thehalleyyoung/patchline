# Replayable incident notebook (real findings)

This gate produces a **Replayable incident notebook** that reconstructs a data-change failure
**hypothesis** from public artifacts — a real Patchline baseline plus deterministic runtime
evidence — so reviewers can replay the reasoning offline.

- **Cells with expected outputs.** The notebook is an ordered set of cells: *load-baseline*,
  *select-incident*, *gather-evidence*, *temporal-check*, *hypothesis*, and *conclusion*. Each
  cell carries the output it should produce.
- **Hypothesis reconstruction.** The notebook selects the highest-severity finding on a table
  with observed impact, gathers its runtime evidence, verifies deploy-precedes-error ordering,
  composes a failure hypothesis, and concludes whether it is supported.
- **Deterministic replay.** The reconstruction is regenerated independently and required to be
  **byte-for-byte identical**, proving the notebook replays the same conclusion every time.

```
make incident-notebook-gate
```

The gate fails unless the notebook contains exactly the required cells in order, the selected
incident is a real finding, the conclusion is a boolean, and the replay is byte-identical.

Outputs (`results/generated/incident-notebook/`):

- `incident-notebook.json` / `.replay.json` — the notebook and its byte-identical replay.
- `runtime-evidence.jsonl` — the deterministic runtime evidence used.
- `incident-notebook-result.json` / `.md` — reconstruction summary.
