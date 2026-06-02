# Threat model

Patchline assumes every repository, archive, generated artifact, native command, and adapter export is untrusted until it has been hashed, parsed conservatively, and re-analyzed by deterministic checks. This document describes the boundaries that matter for users and contributors.

## Assets

- Local developer machines and CI runners.
- Cached repository archives and extracted source trees.
- Analysis reports, SARIF, bundles, redacted artifacts, and release artifacts.
- Secrets, customer identifiers, source paths, production DSNs, and private incident data.
- Trust decisions made by reviewers from Patchline output.

## Boundaries and threats

| Boundary | Untrusted input | Primary threats | Required controls |
| --- | --- | --- | --- |
| Repository source | GitHub archives and local repo trees | malicious files, huge projects, misleading metadata, sensitive literals | pinned refs, `source.json` archive hash/cache path, content-addressed cache, inventory before interpretation, redaction for shared outputs |
| Archive handling | zip/tar/archive downloads | path traversal, symlink escape, archive bombs, cache poisoning | extraction through archive helpers, scanned subpath roots, recorded archive hash, offline cache validation |
| Adapter inputs | Datadog/OpenTelemetry/Jira/Linear/Postgres/GitHub exports | malformed JSON, spoofed IDs, secret leakage, event-count mismatch | adapter result version, input hash, event count consistency, offline adapter validation, redaction before sharing |
| Generated code | proposal files, repair manifests, SQL, CI hints, generator output | plausible but unsafe fixes, prompt injection, generated secrets, overbroad changes | generated artifacts are labeled `untrusted-generated-proposal`, budgets, prompt-context minimization, deterministic compare re-analysis |
| Native tests | project-discovered test commands | arbitrary command execution, shell injection, network/credential use | skipped by default, explicit `--run-native-tests`, allowlisted command execution without a shell, logs and hashes recorded |
| Database hooks | dry-run scripts and DSNs | accidental production access or destructive mutations | local-only DSN checks, schema-only scripts, explicit user execution, no production credentials |
| Sharing/release | bundles, SARIF, release archives, checksums | leaking sensitive context, tampered artifacts | stable redaction, signed checksums, supply-chain provenance, reproducible build instructions |

## Non-goals

Patchline does not sandbox arbitrary third-party build systems, prove generated code correct, authenticate public GitHub content beyond recorded hashes, or execute native project tests unless a user opts in. It reports proof holes rather than silently upgrading static evidence into runtime guarantees.

## Safe operating rules

1. Prefer pinned refs for remote analysis and keep `source.json`.
2. Treat every generated file as a proposal until `repo compare` has re-analyzed it.
3. Do not pass `--run-native-tests` in CI unless the commands are reviewed for the runner environment.
4. Use `--redact` before sharing bundles outside the team that owns the source.
5. Validate cached analyses with `repo offline` in restricted environments.
6. Publish release artifacts with `release checksums` and supply-chain provenance.

Run `make threat-model-gate` to verify this document against real Patchline artifacts from a pinned public repository slice plus an adapter-export validation path.
