# Generated-code quarantine

Patchline writes generator output as untrusted proposal artifacts. Generated files are never applied to the scanned repository by `repo propose`, `repo compare`, or `repo analyze`, and generated artifact files are forced to non-executable `0644` mode when written.

`repo compare` re-analyzes generated content deterministically, but project-native commands are skipped by default. Native checks run only when a user explicitly passes `--run-native-tests`; even then, Patchline executes only safe allowlisted commands without a shell and with the offline sandbox profile recorded in `native_results`.

The machine-readable quarantine record appears in both `proposal.json` and `compare.json`:

- `status: enforced`
- `trust: untrusted-generated-proposal`
- `generated_artifacts_executable: false`
- `generated_artifacts_applied: false`
- `native_checks_require_opt_in: true`
- `required_flag: --run-native-tests`
- `quarantined_paths` listing generated artifact paths

Run `make generated-code-quarantine-gate` to prove the policy with focused regression tests and a pinned public repository analysis. The gate verifies default native-check skipping, explicit safe-native opt-in state, non-executable generated artifact modes, and real generated proposal artifacts from a public GitHub archive.
