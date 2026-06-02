# Reviewer dry-run logs

Patchline publishes anonymized reviewer dry-run logs that show fresh-machine setup failures, fixes, and final regenerated results. The logs are generated from public-code artifact evidence and are checked for reviewer anonymity before publication.

```bash
make reviewer-dry-run-logs-gate
```

The generated bundle includes:

- `index.md`: aggregate reviewer-session summary.
- `reviewer-dry-run-logs.json`: machine-readable anonymized logs, redaction rules, and public-code evidence links.
- `logs/reviewer-*.md` and `logs/reviewer-*.json`: per-reviewer dry-run notes using stable anonymous reviewer IDs.
- `evidence/artifact-container-rebuild/*`: final regenerated public-code results used to prove the dry run reached the artifact thresholds.

The gate verifies that logs include setup failures, fixes, final regenerated results, pinned public-code evidence, and no obvious reviewer emails, host paths, access tokens, or private-key material.
