# Contributor onboarding quest

Adding a new ecosystem to Patchline should take an afternoon, not a week. This quest takes a
contributor from zero to a passing, real-repo-backed gate in **under one hour**, and a scaffolder
generates all the boilerplate so you only write the detector and its proof.

## Scaffold

```
scripts/new-ecosystem.sh "My Ecosystem"
```

This creates an example spec, a worker script, a gate script, and a doc stub following the
established gate pattern.

## The quest

1. Scaffold the gate files with scripts/new-ecosystem.sh
2. Implement the detector in internal/project and emit a fact kind
3. Add a unit test with a positive case and a no-false-positive case
4. Prove the detector on a real public repository
5. Wire a Makefile target and a README mention
6. Run the gate and confirm it passes

Each step maps to an existing example you can copy: the NoSQL, data-pipeline, and schema-compatibility
detectors are all small, self-contained, and proven on real repositories.

Guarantees enforced by the gate:

- The scaffolder emits all expected files.
- The generated worker and gate scripts are valid bash and the spec is valid JSON.
- This quest narrative stays in sync with the required steps.

```
make onboarding-quest-gate
```

Outputs land in `results/generated/onboarding-quest/`.
