# Reproducibility chaos

`make reproducibility-chaos-gate` runs seeded chaos tests against Patchline's real reproducibility paths rather than a mock harness. The tests remove cache metadata, archive cache bytes, archive mirrors, rendered marketplace files, and optional VCS tools, then require the system either to regenerate from trusted source bytes or to fail safe on source drift.

The gate proves three recovery boundaries:

- archive URL fetches reuse an intact content-addressed cache without network access and redownload only when metadata or bytes disappear;
- evidence marketplace archive mirrors are regenerated after mirror-file deletion, while source-artifact hash drift is rejected instead of silently remirrored;
- local Mercurial and Fossil checkouts remain provenance-addressed without native binaries, and local Git checkouts remain analyzable without inventing a revision when `git` is unavailable.

The chaos order is deterministic (`seed=780`) so failures reproduce exactly while still exercising randomized removal ordering.
