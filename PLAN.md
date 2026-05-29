# Patchline plan: formal methods that create practical historical knowledge

Patchline should be a deterministic, explicitly non-AI repair platform that turns existing historical production data into **new checkable knowledge**. The repo should ingest telemetry, migrations, row-change streams, deploy records, incident notes, repair manifests, benchmark results, and audit ledgers; reconstruct their program semantics; and answer practical questions:

- What transition most likely corrupted this state, and what evidence proves it?
- Which rows, reports, services, customers, and time windows were affected?
- Which repairs are scope-safe, reversible, invariant-preserving, and reviewable?
- Which past incidents share the same semantic shape?
- Which migrations or repair patterns repeatedly caused unsafe effects?
- Which claims are proved, checked by execution, assumed, unsupported, or refuted by counterexample?

Formal methods should not make Patchline more academic at the expense of usability. They should make every operational output stronger: better explanations, stronger CI gates, reusable incident knowledge, safer repairs, and reproducible benchmark claims.

## Core thesis

A production system is a program whose state includes code, schemas, relational data, queues, caches, reports, deploys, traces, policies, repair records, and audit ledgers. Historical telemetry is a partial execution trace of that program. A repair is a typed state transformer over that program state. Patchline's job is to reconstruct enough semantics from historical evidence to make repair claims checkable.

The immediate value-add is not "prove everything." It is:

1. Turn scattered historical data into typed traces.
2. Derive causal slices and blast-radius facts from those traces.
3. Express repairs as contracts with preconditions, effects, frames, postconditions, and rollback.
4. Replay repairs and emit concrete diffs plus abstract effect summaries.
5. Check invariants, policies, and bounded solver obligations against both historical evidence and proposed repairs.
6. Link migrations to observed traces, row/report damage, repairs, policies, benchmark hashes, and source-code data effects.
7. Preserve all claims as canonical, benchmarkable artifacts.

## SOTA formal-methods alignment

| Literature idea | Practical Patchline use |
| --- | --- |
| Structural operational semantics | Explain each replay step and incident workflow transition as a state change with normal/error/stuck outcomes. |
| Denotational semantics | Treat a repair manifest as a partial function from historical incident states to repaired states. |
| Hoare logic and weakest preconditions | Convert repair manifests into `{pre} repair {post}` obligations and generate reviewer-visible missing preconditions. |
| Separation logic and frame conditions | Prove which rows, columns, tables, reports, and services a repair does not touch. |
| Abstract interpretation | Summarize concrete historical diffs into monotone effect facts: bounded, destructive, reversible, idempotent, downstream-impacting. |
| Refinement | Check that generated SQL and transaction plans refine the abstract repair intent. |
| SMT solving | Decide predicate implication, scope containment, row-count bounds, and bounded invariant preservation. |
| Datalog and provenance semirings | Query historical evidence for minimal causes, affected observations, recurring incident shapes, and confidence-aware traces. |
| Model checking and temporal logic | Enforce workflow properties: no apply before approval, evidence before approval, eventual verification, rollback availability, immutable audit. |
| Proof-carrying code/data | Bundle repair manifests, trace slices, dry-run hashes, policy results, proof obligations, counterexamples, ledger checkpoints, and signed artifact attestations. |
| CEGAR | When a claim fails, emit a counterexample/proof hole, load missing historical evidence such as invariants or workflow models, and rerun the analysis with stable hashes. |

## Semantic artifacts to implement

Patchline should standardize these artifacts as JSON-first, hashable objects:

1. **Trace projection**: typed events reconstructed from telemetry and operational exports.
2. **Causal certificate**: minimal evidence slice supporting a cause/effect claim.
3. **Semantic migration report**: statement ASTs, relational-signature effects, observed historical outcomes, and risk obligations.
4. **Repair contract**: manifest plus Hoare-style preconditions, postconditions, frame, effects, and rollback obligations.
5. **Replay step trace**: small-step execution of the repair over a concrete or historical snapshot, including compensating-action semantics for append-only external effects.
6. **Snapshot drift report**: strict comparison of repair behavior across imported historical row snapshots.
7. **Abstract effect summary**: monotone abstraction of replay diffs.
8. **Invariant report**: declared and candidate invariants, support counts, violations, and counterexamples.
9. **Proof-obligation report**: statuses for proved, checked, assumed, unsupported/not-supported, and counterexample-producing claims, including Z3-backed scope implication plus bounded frame, row-count, and invariant checks.
10. **Symbolic execution report**: bounded row paths, guard constraints, symbolic assignments, and stuck-step counterexamples for small repair programs.
11. **Workflow model-check report**: bounded incident-response state exploration, temporal properties, counterexample traces, proof obligations, and proof holes.
12. **CEGAR refinement report**: iteration-by-iteration abstraction refinements, remaining proof holes, and counterexamples across replay, solver, symbolic, and workflow checks.
13. **Signed attestation**: Ed25519 signature over canonical artifact hashes for CI and incident-review handoff.
14. **Incident archive index**: deterministic buckets over evidence shape, migration tables/risks, repair effects, policy decisions, benchmark results, and proof-bundle readiness.
15. **Historical knowledge report**: newly linked facts across deploys, migrations, traces, row mutations, reports, repairs, and recurrence.
16. **Proof-carrying incident bundle**: content-addressed archive of all evidence and claims needed for review or replay.

## Near-term implementation priorities

The next work should prioritize features that are both formally meaningful and immediately useful over historical data:

1. **Semantic documentation and types**: define state, observation, transition, trace, repair, invariant, proof obligation, and counterexample.
2. **Trace reconstruction upgrades**: add confidence, time uncertainty, source identity, and import-equivalence checks for historical evidence.
3. **Historical causal queries**: add Datalog-style/minimal-slice commands that expose what caused what and what evidence is missing.
4. **Replay step traces**: show the operational semantics of repair execution, not only final row diffs.
5. **Effect lattice and abstract interpreter**: turn effect inference into a documented abstraction with transfer functions and proof holes.
6. **Invariant declarations and candidate discovery**: mine historical data for candidate invariants with support counts and counterexamples; require explicit promotion before enforcement.
7. **Schema-state diffing**: finish migration effects over relational signatures and compare expected vs actual schema states.
8. **Migration outcome histories**: emit semantic changelogs that combine changed tables, broad effects, observed downstream damage, repairs, policy outcomes, and benchmark/source hashes.
9. **Solver-backed proof obligations**: deepen the current bounded equality/finite-store solver into a reusable obligation engine for repair review and gates.
10. **Symbolic execution**: extend current bounded row-path execution toward branch coverage and richer path merging for small repair programs.
11. **Workflow model checking**: extend current bounded workflow checks with richer workflow descriptors and organization-specific review policies.
12. **Historical archive index**: make prior incidents queryable by trace shape, migration effect, repair outcome, policy decision, benchmark decision, and proof-bundle readiness.
13. **Proof-carrying bundles and attestations**: extend current v2 proof bundles with signed attestation workflows and verification commands.

## Design constraints

- **No AI claims**: Patchline may discover, cluster, and summarize by deterministic algorithms, but it must not infer uncheckable conclusions.
- **Historical-first**: every feature should work on existing exports and records before requiring new instrumentation.
- **Counterexamples over confidence theater**: if a claim cannot be proved or checked, emit a proof hole or counterexample.
- **Canonical everything**: every semantic artifact must have stable JSON and hashes.
- **Operationally useful defaults**: formal artifacts must surface through CLI commands, CI gates, docs, examples, and reviewer-facing explanations.
- **Research honesty**: distinguish proved facts, executable checks, heuristic candidates, and unsupported claims.

## What this lets Patchline claim

Patchline can credibly become a platform for extracting new, reproducible knowledge from historical operational data:

- It reconstructs typed execution traces from messy telemetry and change streams.
- It computes causal slices and blast-radius facts with explicit evidence.
- It turns repairs into executable contracts and proof obligations.
- It links migrations to observed runtime/data outcomes and reviewer-facing semantic changelogs.
- It discharges practical bounded proof obligations for scope, frame, row-count, and invariant preservation claims.
- It symbolically executes bounded repair programs to expose path conditions and touched-row explanations.
- It model-checks bounded incident workflows and exposes counterexamples/proof holes before repair application.
- It performs CEGAR-style reruns that refine coarse repair abstractions with invariant specs and workflow evidence, while preserving explicit remaining holes.
- It signs and verifies semantic artifacts with Ed25519 so CI gates and incident reviews can detect tampering.
- It builds deterministic incident archives over historical evidence, migrations, repairs, policies, and benchmark outputs so recurrence and repair-effect queries have stable inputs.
- It validates usefulness with a `make verify-usefulness` path that runs the core suite, strict gate, Z3-backed solver obligations, semantic audit, archive index, and SHA-pinned public migration corpus.
- It replays repairs over imported historical row snapshots and quantifies drift/stability across snapshots.
- It gives append-only logs, event streams, queues, and external replays explicit compensating-action semantics instead of pretending snapshot rollback can undo them.
- It compares new incidents and migrations against historical semantic shapes.
- It provides deterministic benchmarks for data/code repair research.
- It improves production safety while serving as a serious applied formal-methods testbed.
