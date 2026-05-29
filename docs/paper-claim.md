# Patchline artifact claim

Patchline's paper-facing claim is:

> Patchline converts historical production data-repair incidents into executable semantic artifacts that can replay repairs, prove scope and frame obligations, and detect recurrence across future migrations.

The claim is deliberately narrower than “incident automation.” Patchline does not guess root causes or generate repairs. It packages available operational evidence, migrations, repair plans, policies, bounded stores, solver obligations, replay traces, and archive entries into deterministic artifacts that can be inspected and rerun.

## Central object

The central object is a **repair semantic artifact**:

```text
Evidence -> Trace -> Transition -> Repair -> Proof -> Replay -> Archive -> Regression
```

| Term | Meaning in Patchline | Reviewer-visible command |
| --- | --- | --- |
| Evidence | Source-grounded events, observations, public incident facts, SQL files, and repair manifests. | `ingest-evidence`, `historical-failures`, `extract-sql` |
| Trace | A normalized event projection with stable ordering, confidence, and hash. | `trace-reconstruct` |
| Transition | A migration or state-changing operation classified by relational effect and risk. | `analyze-migration`, `migration-semantics` |
| Repair | An explicit repair manifest interpreted as bounded state transformation. | `validate-repair`, `repair-semantics`, `dry-run` |
| Proof obligation | A scope, frame, row-count, invariant, or replay claim checked by Z3 or marked as downgraded. | `solver-obligations` |
| Replay | A deterministic execution over a bounded store that emits row diffs and verification hashes. | `dry-run`, `repair-outcomes` |
| Archive | An ordered incident corpus with queryable evidence, migration, policy, benchmark, repair, and rollback facts. | `archive-index`, `archive-query` |
| Regression | A later incident that reuses a previous damaged shape, high-risk table, or damaged derived report relation. | `semantic-regressions` |
| Artifact study | A generated comparison that shows and drift-checks which layers add detection, actionability, proof links, archive links, ground-truth links, and scale evidence. | `artifact-baselines`, `artifact-ablations`, `artifact-scale`, `artifact-study compare` |
| Artifact benchmark | A phase-aware manifest run that predicts from fixtures, refuses excluded inputs, and compares against frozen expected reports. | `artifact-benchmark validate/run/compare` |

## Phase-aware claims

Patchline uses phase boundaries to avoid hindsight leakage.

| Phase | Allowed evidence | Claim shape |
| --- | --- | --- |
| Pre-deploy | migration text, schema, policy, prior archive entries, known invariants | “Patchline flags this transition before it runs.” |
| During migration | migration runner output, traces, partial state observations | “Patchline reconstructs the transition and its affected records.” |
| During repair | bad state, repair manifest, rollback/backup facts, bounded store | “Patchline checks whether the repair can be replayed and bounded.” |
| Postmortem | public incident narrative, root-cause report, final repair outcome | “Patchline records source-grounded incident memory.” |
| Archive-only | prior incident artifacts | “Patchline detects recurrence or refuses unsupported recurrence claims.” |

## What is not claimed

Patchline does not claim to:

- infer private production data that is not present in the artifact;
- prove arbitrary SQL or arbitrary distributed-system behavior;
- synthesize a repair plan from natural language;
- replace operator review;
- treat postmortem-only facts as pre-deploy evidence;
- use handwritten SMT as a substitute for Z3-backed obligations.

When evidence is missing or an input fragment is unsupported, the artifact should say so explicitly.

## Why this is novel

The novelty is not that provenance, migration linting, replay, or SMT solving exist independently. The novelty is the executable object that composes them: a historical data-repair incident becomes a stable semantic artifact whose claims are replayable, hashable, queryable, solver-backed where possible, and reusable as future recurrence memory.

The current artifact also exposes this composition through ablations and executable manifests: reviewers can run `make artifact-studies-compare` to observe how a migration-only detector becomes a richer repair artifact as policy, Z3 obligations, archive memory, and phase-aware ground truth are added, run `make artifact-studies-public-compare` to repeat and hash-check baseline/ablation/scale reports on pinned public Bytebase migrations, run `make artifact-benchmark-compare` to verify committed smoke, negative, repair/replay, and semantic-regression cases against frozen no-leakage expected reports, run `make artifact-benchmark-public` to check the same manifest protocol against real OSS migration SQL, run `make artifact-benchmark-public-incidents` to validate GitLab/GitHub public source observations plus an insufficient-evidence boundary without network access, run `make artifact-benchmark-public-repairs` to check a GitLab-2017-derived repair boundary where missing snapshot evidence must remain `cannot_prove`, and run `make artifact-benchmark-public-archive` to exercise a paired GitLab/GitHub postmortem-derived archive where public evidence plus explicit local reconstructions produce a recurrence flag without claiming access to private production rows.
