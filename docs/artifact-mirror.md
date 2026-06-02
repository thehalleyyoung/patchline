# Public artifact mirror

Patchline publishes public mirrors of its generated evidence but only after scanning each
artifact for sensitive markers, so that reproducible non-sensitive results are shared
while anything resembling a secret, private key, or access token is held back
automatically rather than by manual review.

## Scan and mirror

The worker materializes candidate artifacts, scans each for a catalog of sensitivity
markers (`PRIVATE KEY`, `SECRET`, `ACCESS_TOKEN`), mirrors only the clean artifacts with
their SHA-256 checksums, and **quarantine**s the rest with the marker that triggered
exclusion.

## Why it stays honest

A public **artifact mirror** must never leak a secret just because an artifact looked
routine. The gate proves the clean artifacts are mirrored with checksums while an
artifact containing a secret marker is quarantined and never appears in the public mirror
directory.

## Reproduce

```
make artifact-mirror-gate
```
