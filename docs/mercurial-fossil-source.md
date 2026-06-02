# Mercurial and Fossil source adapters

Patchline's promise is to analyze *real repositories that were not built for Patchline*. Many of
those repositories do not live in Git. This adapter makes **Mercurial** and **Fossil** working
trees first-class sources alongside Git, GitHub, GitLab, Bitbucket, SourceHut, archives, and plain
local paths.

When `patchline repo fetch <path>` ingests a local working tree, it now:

- detects the **version-control system** from its on-disk metadata (`.hg/` for Mercurial,
  `_FOSSIL_`/`.fslckout` for Fossil, `.git/` for Git);
- records the VCS name and a **resolved revision** as `vcs` and `resolved_commit` **provenance** —
  using the native `hg`/`fossil` binary when present and falling back to the revision recorded in
  the VCS metadata otherwise;
- computes a **content-addressed** tree hash that *ignores* VCS metadata, so the same working tree
  caches identically regardless of which VCS produced it, and a second fetch of unchanged content
  is a cache hit.

Guarantees enforced by the gate:

1. A Mercurial working tree is ingested with `vcs=mercurial`, a resolved revision, and a
   `sha256:` tree hash.
2. A Fossil working tree is ingested with `vcs=fossil`, a resolved revision, and a `sha256:` tree
   hash.
3. **Cache semantics** — the first fetch misses, the second fetch of identical content hits, and
   mutating only VCS metadata does not change the content-addressed hash.
4. A destructive SQL migration inside the ingested tree is still surfaced by deterministic baseline
   analysis, proving ingestion preserves the data-repair surface.

```
make mercurial-fossil-source-gate
```

Outputs land in `results/generated/mercurial-fossil-source/`.
