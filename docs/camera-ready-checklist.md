# Camera-ready checklist

Patchline's camera-ready checklist blocks release if claims, figures, tables, docs drift, rebuttal responses, or release metadata no longer match generated evidence.

```bash
make camera-ready-checklist-gate
```

The generated checklist includes:

- `camera-ready-checklist.json`: machine-readable release-blocking checks.
- `camera-ready-checklist.md`: reviewer-facing checklist.
- `drift-policy.json`: explicit drift cases that block release.
- `evidence/rebuttal-workspace/*`: regenerated appendix, release manifest, and rebuttal evidence.

The gate verifies the current generated evidence and also runs local drift probes for claims, figures, and tables to prove the checklist blocks release when paper-facing artifacts drift.
