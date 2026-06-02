# Redaction stability

Patchline redaction uses deterministic tokens so a value maps to the same token across reports, prompts, bundles, SARIF, diagnostics, and compare outputs. This lets teams diff redacted artifacts across repeated runs and resume mode without revealing source paths, table names, customer-like values, literals, or secret-like strings.

`repo analyze --redact` applies the same stable-token policy to `analysis-bundle/` and to the broader `redacted-artifacts/` export. The stability contract is:

1. Repeated redaction of the same value produces the same `[redacted:<kind>:<hash>]` token.
2. JSON, JSONL, SARIF, Markdown, prompt text, generated proposal files, and comparison reports all use the same token format.
3. Re-running the same analysis with `--redact` rewrites shareable artifacts deterministically.
4. Re-running with `--resume --redact` preserves the same redacted bundle, SARIF, generated prompt, and compare outputs.

Run `make redaction-stability-gate` to execute focused redactor/resume tests and prove the artifact stability contract on a pinned public repository slice.
