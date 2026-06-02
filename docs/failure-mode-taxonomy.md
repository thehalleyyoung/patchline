# Public-corpus failure-mode taxonomy

`patchline repo taxonomy` reads one or more `repo analyze` output directories and derives a deterministic taxonomy of real data-change repair failure modes. It is not a hand-written list: each `failure_modes[]` entry is backed by baseline risks, summary signals, generated-intervention compare output, and public-repo examples.

```bash
go run ./cmd/patchline repo taxonomy \
  --analyses results/generated/failure-taxonomy-gate/analyses/lobsters-rails-migrations,results/generated/failure-taxonomy-gate/analyses/bytebase-go-migrator \
  --out results/generated/failure-taxonomy-gate/taxonomy \
  --json
```

The JSON report writes `failure-taxonomy.json` with:

- `failure_modes[]`: recurring failure modes such as broad/destructive mutation, missing rollback evidence, unsafe backfill or repair script, schema evolution risk, missing transaction boundary, non-idempotent or unknown repair, lock/concurrency hazard, retention/privacy hazard, and open proof hole.
- `definition`: what the mode means in data-change repair review.
- `repair_risk`: how generated or manual repair can fail if the mode is ignored.
- `maintainer_decision`: the concrete review decision maintainers should make.
- `examples[]`: repo, ref, subpath, risk id, severity, score, generated file count, deterministic outcome, and evidence text.
- `evidence_kinds`: the analyzer surfaces supporting the mode.
- `summary`: aggregate counts for analyses, public repos, failure modes, occurrences, high-severity occurrences, and generated-intervention links.

The paired Markdown report writes a compact public-corpus narrative for maintainers and research reviewers.

`make failure-taxonomy-gate` validates the command against eight pinned public repository slices, checks that the docs name the machine fields above, and requires multiple recurring modes with real public examples.
