# Rebuttal-response workspace

Patchline generates a public rebuttal-response workspace that links likely reviewer concerns to generated evidence and explicit limitations. It is meant to make artifact and paper review more concrete: every response draft points back to current evidence paths, limitations, reviewer checks, and reproduction commands.

```bash
make rebuttal-response-workspace-gate
```

The generated workspace includes:

- `rebuttal-workspace.md`: index of likely reviewer concerns and response status.
- `rebuttal-workspace.json`: machine-readable concern, evidence, limitation, and response mapping.
- `responses/*.md`: one response draft per concern.
- `evidence/paper-appendix/*`: generated claims, limitations, figures, tables, and reproduction commands.
- `evidence/release-manifest/*`: exact refs, archive checksums, generated artifact checksums, command versions, and DOI candidate.

The gate proves the workspace against regenerated public-code evidence and requires each concern to have evidence links, linked limitations, and a concrete reviewer check.
