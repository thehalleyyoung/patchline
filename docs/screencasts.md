# Screencasts

Patchline's short screencasts are generated terminal artifacts, not hand-edited marketing videos. `scripts/generate-screencasts.sh` runs Patchline against pinned public `lobsters/lobsters` migration code, then emits asciinema-compatible `.cast` files, transcripts, storyboards, and a machine-readable summary.

```bash
make screencast-gate
```

The generated set covers four reviewer-facing moments:

- `first-run-analysis`: first-run analysis over a public repository slice, including files scanned, ranked data-change risks, provenance slices, generated artifacts, and deterministic check results.
- `generated-intervention-review`: generated intervention review showing the bounded `proposal.patch`, compare outcome, and the rule that generated code remains review evidence rather than trusted executable output.
- `ci-integration`: CI integration artifacts for SARIF, GitLab code-quality findings, Bitbucket annotations, and the Patchline analysis bundle.
- `artifact-reproduction`: artifact reproduction with a checksum manifest and bundle hash for the generated analysis bundle.

The gate verifies every cast, transcript, and storyboard; parses each cast header; checks that the metrics exceed public-code thresholds; and confirms the screencasts were derived from real Patchline artifacts.
