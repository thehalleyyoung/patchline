# Supply-chain provenance

Patchline can emit deterministic supply-chain provenance for artifacts that are built, packaged, generated, or downloaded during validation.

Use `patchline supply-chain provenance` with repeatable `--artifact kind=path` flags. The command hashes files directly and hashes directories through a sorted file manifest. The provenance report records the Patchline toolchain, git commit, source references, reproduction commands, artifact sizes, file counts, SHA-256 digests, required artifact classes, and a deterministic report hash.

Required artifact classes for the gate are:

1. `binary` for a built Patchline executable.
2. `release_archive` for a packaged release archive.
3. `generated_experiment_artifact` for generated analysis or experiment outputs.
4. `public_corpus_download` for pinned public corpus downloads.

`scripts/fetch-public-corpus.sh` also writes `download-provenance.json` next to its fetch report so corpus downloads are tied back to their source manifest and local downloaded bytes.

Run `make supply-chain-provenance-gate` to build a binary, package a release archive, run a pinned public-repo analysis, fetch pinned public corpus files, and prove the resulting provenance covers all required artifact classes.
