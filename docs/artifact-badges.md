# Artifact badges

Patchline artifact badges are awarded from gate-backed justifications rather than static claims. The badge gate regenerates public-code evidence, checks the artifact container rebuild profile, and then emits reusable, available, functional, and reproducible badges.

```bash
make artifact-badges-gate
```

The generated outputs include:

- `artifact-badges.json`: machine-readable badge award decisions, criteria, and public-code evidence counts.
- `badges.md`: reviewer-facing badge strip and justifications.
- `badges/*.svg`: local badge images for available, functional, reusable, and reproducible evidence.
- `evidence/artifact-container-rebuild/*`: regenerated public-code evidence used to justify the awards.

Each badge requires at least three criteria and must be backed by a public-code rebuild with pinned repositories, ranked risks, bounded generated files, rejected bad output, and checksum-producing evidence.
