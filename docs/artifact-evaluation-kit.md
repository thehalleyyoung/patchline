# Artifact-evaluation landing kit

Patchline's artifact-evaluation landing kit gives reviewers explicit reviewer roles, time budgets, expected outputs, and pass/fail criteria. The kit is generated from a capstone run over pinned public repositories so reviewer instructions stay connected to real regenerated evidence.

```bash
make artifact-evaluation-kit-gate
```

The generated kit includes:

- `landing.md`: reviewer-facing landing page with role table, expected outputs, pass/fail criteria, and public-code evidence.
- `reviewer-guide.md`: step-by-step artifact evaluation checklist.
- `roles.json`: role definitions and time budgets for quickstart, artifact, security, and research reviewers.
- `expected-outputs.json`: required files reviewers should find.
- `pass-fail-criteria.json`: objective checks for accepting or rejecting the artifact.
- `capstone/*`: regenerated release-quality capstone evidence used to anchor the kit.

The gate verifies every role, output, criterion, and public-code evidence threshold before the kit can be trusted.
