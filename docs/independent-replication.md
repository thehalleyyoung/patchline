# Independent replication instructions

Patchline includes independent replication instructions for reviewers without GitHub credentials or proprietary tooling. The path uses anonymous public archive URLs, SHA-256 verification, and the generated artifact release manifest.

```bash
make independent-replication-gate
```

The generated bundle includes:

- `independent-replication.md`: reviewer-facing no-credential replication path.
- `independent-replication.json`: machine-readable instructions, archive verification, and evidence summary.
- `replication-commands.sh`: curl-based anonymous archive fetch and checksum commands plus artifact rebuild commands.
- `archive-verification.json`: proof that cached public archives match release-manifest SHA-256 hashes.
- `no-credential-environment.json`: required ordinary tools and forbidden credential/tooling assumptions.
- `evidence/release-manifest/*`: exact refs, archive URLs, checksums, command versions, and regenerated public-code evidence.

The gate unsets GitHub/vendor credential variables, verifies public codeload archives, and rejects forbidden CLI or credential usage in the generated replication path.
