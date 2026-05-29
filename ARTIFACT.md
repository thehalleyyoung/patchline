# Patchline Artifact Guide

Patchline's artifact claim is:

> Patchline converts historical production data-repair incidents into executable semantic artifacts that can replay repairs, prove scope and frame obligations, and detect recurrence across future migrations.

This guide is for artifact reviewers. It separates the implemented evaluator path from the larger roadmap in `NEWEST_PLAN.md`.

## Implemented in this artifact path

The current reviewer path is intentionally narrow and executable from a fresh checkout:

1. Validate the benchmark and ground-truth manifests.
2. Run the Go unit suite.
3. Execute the semantic pipeline over committed smoke fixtures.
4. Emit deterministic JSON outputs under `results/generated/`.
5. Run negative/limitation cases that show unsupported or insufficient-evidence outcomes are represented explicitly.

The smoke path uses only committed fixtures and existing CLI commands. It does not require network access.

## Setup

Prerequisites:

- Go 1.22+
- `make`
- `jq`
- Z3 on `PATH` for solver-backed claims

Z3 is expected for the strongest artifact claims. If Z3 is absent, Patchline records solver downgrade information rather than replacing it with hand-written SMT logic.

## Container path

The repository includes a Dockerfile and devcontainer definition:

```bash
docker build -t patchline-artifact .
docker run --rm patchline-artifact make artifact-smoke
```

The container installs Go, Z3, `jq`, `curl`, `git`, and `make`.

## Quick reviewer path

```bash
make artifact-smoke
```

Expected runtime: under 5 minutes on a laptop-class machine.

The target writes outputs to:

```text
results/generated/artifact-smoke/
```

It runs:

- `go test ./...`
- `artifact-ground-truth`
- `scripts/validate-ground-truth.sh`
- `scripts/check-artifact-targets.sh`
- `semantics-audit`
- `trace-reconstruct`
- `analyze-migration`
- `solver-obligations`
- `repair-semantics`
- `archive-query`
- `semantic-regressions`
- `benchmark-suite`

## Canonical demo

```bash
make artifact-demo
```

Expected runtime: under 5 minutes.

The demo emits:

```text
results/generated/artifact-demo/
  trace.json
  migration.json
  solver.json
  repair.json
  archive.json
  regressions.json
  summary.json
  summary.md
```

The final summary includes a deterministic `artifact_bundle_hash` computed from the stage JSON files.

## Ground-truth validation

```bash
make artifact-ground-truth-check
```

This validates every file under `benchmarks/ground_truth/` and every manifest under `benchmarks/manifests/`.

The Make target runs both the first-class Go validator:

```bash
go run ./cmd/patchline artifact-ground-truth benchmarks --json
```

and the shell validator used as an independent packaging check.

The validator fails if:

- a case has no `case_id`, `case_type`, `phase`, expected result, or evidence;
- a pre-deploy case cites postmortem-only evidence;
- evidence lacks locator/rationale metadata;
- a manifest references a missing ground-truth file;
- a manifest case ID disagrees with its ground-truth case ID.

## Negative cases

```bash
make artifact-negative-cases
```

The current negative-case path validates explicit smoke labels for:

- unsupported SQL fragments;
- insufficient public evidence;
- phase-leakage protection;
- non-replayable repairs;
- safe non-recurrence.

These cases are part of the artifact, not merely prose limitations.

## Relationship to `make verify-usefulness`

`make verify-usefulness` remains the broader project validation target. It includes network-backed public corpus fetching and source checks. `make artifact-smoke` is the artifact-review entry point: it is smaller, committed-fixture-only, and intended to be stable without network access.

## Data provenance

Smoke fixtures are committed under `examples/` and benchmark labels are committed under `benchmarks/ground_truth/`. Public historical references are represented as source URLs and hashes/notes where appropriate; the smoke path does not download them.

Network-backed corpus validation remains available through:

```bash
make verify-usefulness
make public-corpus
```

## Roadmap not yet claimed by the smoke path

The following are planned in `NEWEST_PLAN.md` but are not yet claimed as fully implemented artifact results:

- large public migration benchmark with hundreds or thousands of statements;
- multi-incident public postmortem benchmark with precision/recall;
- full baseline comparison table;
- full ablation study;
- scale and memory measurements over large corpora.

The smoke artifact path is designed so these can be added without changing the central claim.
