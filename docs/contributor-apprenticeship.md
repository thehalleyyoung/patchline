# Contributor apprenticeship pathway

Patchline treats contributor growth as a reproducible artifact. A new
contributor graduates by shipping a real detector, its `make` gate, a short
documentation page, a minimized passing fixture, a minimized failing fixture,
mentor signoff, and independent reviewer evidence.

## What the gate proves

- Every apprenticeship track points at detector code in this checkout and the
  expected emitted signal.
- Every track has a Make-backed gate, an exact reproducing command, expected
  artifacts, and a described negative control.
- Documentation contains the gate phrase reviewers need to rerun the work.
- Fixtures stay under the configured byte budget and include a readable
  negative-control fixture.
- Mentor signoff and at least two reviewers are required before graduation.

## Reproduce

```bash
make contributor-apprenticeship-gate
```
