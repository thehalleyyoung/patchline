# Standards-body conformance corpus

Patchline ships a standards-body conformance corpus for PLCI/1 certificates. Every example has a real positive proof, a near-miss negative control, and an Ed25519-signed reference output.

The corpus lives under `specs/certificate-conformance/v1/`. Its cases cover the four PLCI verdict classes (`safe`, `guarded`, `blocked`, and `unsupported`) against real Patchline files: the CLI entrypoint, repair proof frame, PLCI ABNF grammar, and certificate language documentation.

## What the gate proves

- Every `positive.plci` parses and verifies its `file:` evidence digest against the current repository.
- Every `negative.plci` is syntactically valid but rejected for the expected verdict/obligation semantic rule.
- Every `reference-output.json` payload exactly matches the checker output and verifies against the corpus Ed25519 public key.
- A tampered signed reference output is rejected before the gate passes.

## Reproduce

```bash
make standards-body-conformance-corpus-gate
```
