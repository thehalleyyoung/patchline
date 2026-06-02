# PLCI/1 standards-body conformance corpus

This directory is the frozen standards-body conformance corpus for PLCI/1 certificates.

Each case contains:

1. `positive.plci`: a real certificate proof over Patchline source, spec, or documentation files.
2. `negative.plci`: a near-miss negative control with valid syntax and digests but invalid verdict semantics.
3. `reference-output.json`: the expected checker result, signed with the corpus Ed25519 public key in `corpus.json`.

`make standards-body-conformance-corpus-gate` verifies every positive proof, rejects every negative control, recomputes the signed reference payloads from real code, and fails if any signature or payload drifts.
