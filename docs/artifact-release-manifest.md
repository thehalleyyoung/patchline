# Artifact DOI/release manifest

Patchline generates a deterministic artifact DOI/release manifest with exact refs, archives, checksums, and command versions for reviewer reproduction.

```bash
make artifact-release-manifest-gate
```

The manifest generator rebuilds the public-code artifact evidence, then writes:

- `artifact-release-manifest.json`: deterministic release metadata, DOI candidate, exact refs, archive URLs, checksums, command versions, and reproduction commands.
- `artifact-release-manifest.md`: reviewer-readable summary.
- `archives.json`: codeload archive URLs, resolved commits, and SHA-256 archive hashes.
- `artifact-checksums.sha256`: sorted checksums for generated release evidence.
- `command-versions.json`: `go`, `git`, `bash`, `jq`, and `make` versions used to generate the manifest.

The DOI is a deterministic candidate derived from the sorted manifest payload hash. It is marked unregistered until an external archive service assigns a real DOI, but the manifest content hash remains stable for release review.
