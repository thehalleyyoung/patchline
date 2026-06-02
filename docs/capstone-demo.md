# Release-quality capstone demo

Patchline's release-quality capstone demo shows a fresh user downloading four unfamiliar public repositories, finding high-signal repair risks, generating bounded interventions, rejecting bad output, and regenerating experiment-ready evidence in one documented session.

```bash
make capstone-demo-gate
```

The capstone produces:

- `session.md`: narrated end-to-end session with commands, public repositories, risk totals, rejection proof, and evidence links.
- `analyses/*`: deterministic analysis bundles for four pinned public repositories.
- `bad-output-analyses/*` and `rejections/rejected-generated.json`: plausible bad generated SQL rejected by deterministic checks.
- `evidence/*`: metrics, failure taxonomy, claims, limitations, figures, and case studies for paper/artifact review.
- `checksums.txt`: checksums for regenerated session artifacts.

The gate proves the session against real public code and checks that every repo produced high-signal risks, bounded generated artifacts, rejected bad output, and experiment-ready evidence.
