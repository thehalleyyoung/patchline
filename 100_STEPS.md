# 100 steps to make Patchline a semantics-first historical repair system

Patchline should be practical infrastructure software first: ingest real historical telemetry, migrations, incidents, ledgers, and repair records; turn them into typed semantic artifacts; and produce new, checkable knowledge about what broke, why it broke, what was affected, what repairs are safe, and which risks are recurring. The formal-methods angle should strictly improve this usability: operational semantics, Hoare triples, abstract interpretation, refinement, model checking, SMT, Datalog, provenance semirings, and proof-carrying artifacts become concrete CLI outputs, benchmark fields, review evidence, and CI gates.

Checked items are already represented in the repo. Unchecked items are the next implementation increments.

## A. Practical semantic thesis

1. [x] Define Patchline as deterministic production data/code repair software, not AI software.
2. [x] Instantiate a Go CLI with canonical JSON, stable hashes, and reproducible command output.
3. [x] Document the Datadog-style incident-response value proposition and Microsoft RiSE research angle.
4. [x] Provide a reproducible billing incident demo that acts as the first executable semantic witness.
5. [x] Position repair as typed provenance plus replay plus policy plus audit, rather than ad hoc cleanup scripts.
6. [x] Define the Patchline semantic contract: every command must emit facts, obligations, counterexamples, or hashes that can be checked later.
7. [x] Define the core state model for historical production systems: code, schema, rows, jobs, queues, reports, deploys, migrations, traces, policies, and ledgers.
8. [x] Define the observation model: alerts, report totals, row predicates, traces, logs, dashboards, customer-visible outputs, and approval records.
9. [x] Define repair as a partial state transformer with explicit domain, codomain, effects, and failure states.
10. [x] Add `docs/semantics.md` with the operational/denotational semantics used by the implementation.

## B. Historical data ingestion as trace reconstruction

11. [x] Implement typed provenance entities for services, commits, deploys, migrations, traces, SQL mutations, records, reports, jobs, queues, and repairs.
12. [x] Implement typed provenance edges for deploy, execute, cause, mutate, derive, observe, and repair relationships.
13. [x] Add JSONL evidence ingestion for deploy, migration, trace, SQL mutation, row mutation, and derived-output events.
14. [x] Add adapters for current OTLP, Datadog, Postgres logical decoding, GitHub deployment/release, and migration-runner exports.
15. [x] Preserve unknown operational fields while validating required Patchline evidence fields.
16. [x] Make ingested graph projections usable by `explain` and `slice`.
17. [x] Recast evidence ingestion as trace reconstruction from partial, heterogeneous historical observations.
18. [x] Add source-confidence and clock-confidence fields to distinguish exact, causal, temporal, inferred, and conflicting trace facts.
19. [x] Add deterministic event-time normalization with explicit uncertainty intervals.
20. [x] Add import equivalence checks proving that two source exports reconstruct the same typed trace projection.

## C. Provenance semantics and immediate incident value

21. [x] Implement deterministic backtracing from corrupted records to causal deploys and migrations.
22. [x] Implement deterministic provenance slicing with evidence thresholds and slice hashes.
23. [x] Propagate damage through derived-record and derived-report edges.
24. [x] Add Datalog-style queries for historical cause discovery: minimal causes, common ancestors, affected observations, and repair lineage.
25. [x] Add provenance semiring annotations for exact, strong, weak, absent, conflicting, and redacted evidence.
26. [x] Add minimal-explanation output: the smallest evidence slice sufficient to justify a causal claim.
27. [x] Add differential provenance: compare two historical incidents and emit shared causal structure.
28. [x] Add recurring-cause mining over incident archives without ML: canonical trace-shape clustering by graph isomorphism and hashes.
29. [x] Add blast-radius summaries over historical traces: tables, reports, services, customers, and time windows repeatedly affected.
30. [x] Add reviewer-facing causal certificates that bundle trace slice, confidence, hash, and missing-evidence holes.

## D. Repair manifests as executable contracts

31. [x] Define a strict repair manifest DSL with incident, scope, operations, preconditions, postconditions, rollback, and dependencies.
32. [x] Reject unknown manifest fields and unsupported manifest versions.
33. [x] Validate repair manifests against provenance graph scope.
34. [x] Reject repair operations whose predicates do not respect declared scope.
35. [x] Add manifest migration, templates, linting, deterministic SQL generation, rollback planning, and transaction planning.
36. [x] Support dry-run replay for update, insert, delete, replay, and rebuild-index operations.
37. [x] Give each manifest a Hoare-triple view: `{historical evidence + preconditions} repair {postconditions + obligations}`.
38. [x] Generate weakest preconditions for supported repair operations and expose them in `lint-repair --proof`.
39. [x] Add frame-condition checks so a repair proves which rows, columns, tables, reports, and services it leaves untouched.
40. [x] Add refinement checks showing generated SQL refines the abstract repair manifest and does not introduce extra effects.

## E. Effects, abstract interpretation, and invariants

41. [x] Implement deterministic repair-effect inference.
42. [x] Add executable attestation checks for changed row values, max changed rows, operation effects, downstream impact, scoped updates, report hashes, and ledger checkpoints.
43. [x] Replace informal effect labels with a documented effect lattice and monotone transfer functions.
44. [x] Add an abstract interpreter that summarizes bounded row effects, destructive effects, idempotence, reversibility, downstream impact, and proof holes.
45. [x] Emit both concrete dry-run diffs and abstract summaries, with a documented abstraction/concretization relation.
46. [x] Add invariant declarations for uniqueness, foreign keys, enums, sums, counts, materialized reports, ledger balance, and customer-visible totals.
47. [x] Add invariant checking over historical snapshots before and after replay.
48. [x] Add invariant candidate discovery from historical data with explicit support counts and counterexamples, never silent auto-acceptance.
49. [x] Add commutativity, independence, and confluence checks for multi-operation repairs.
50. [x] Add counterexample JSON objects whenever an effect, invariant, frame, or refinement check fails.

## F. Replay and transaction semantics

51. [x] Implement an in-memory replay sandbox with deterministic row diffs and report hashes.
52. [x] Generate deterministic rollback plans from reversible dry-run effects.
53. [x] Generate deterministic transaction plans with sorted lock ordering and dependency ordering.
54. [x] Model replay as small-step operational semantics with normal, error, stuck, rollback, and unknown states.
55. [x] Add a step trace output so reviewers can inspect every semantic transition, not just the final diff.
56. [x] Add transaction-isolation models for read committed, repeatable read, snapshot isolation, and serializable execution.
57. [x] Check serializability and write-conflict hazards for repair plans touching multiple tables or derived artifacts.
58. [x] Add compensating-action semantics for append-only logs, event-sourced stores, queues, and irreversible external effects.
59. [x] Add replay over imported historical row snapshots, not only built-in demo stores.
60. [x] Add replay comparison across two historical snapshots to quantify drift and repair stability.

## G. Migration and code semantics

61. [x] Implement lexical SQL migration analysis with statement fingerprints and risk classification.
62. [x] Add dialect-specific analyzer modes for Postgres, MySQL, SQLite, and SQL Server.
63. [x] Add a lightweight AST-backed parsing layer behind migration analysis.
64. [x] Complete schema-state diffing against expected relational signatures.
65. [x] Define schema migrations as typed transformations over relational signatures.
66. [x] Add relational-algebra semantics for supported `select`, `insert`, `update`, `delete`, and DDL fragments.
67. [x] Extract embedded SQL from Go, Python, TypeScript, Ruby, Java, shell scripts, and migration frameworks.
68. [x] Add ORM-aware extraction for Rails, Django, Prisma, TypeORM, Sequelize, Entity Framework, and common query builders.
69. [x] Link historical migrations to later traces, row mutations, policy failures, and repairs to create migration outcome histories.
70. [x] Emit migration "semantic changelogs": changed tables, changed invariants, possible broad effects, observed historical outcomes, and benchmark hashes.

## H. Solvers, model checking, and proof-carrying artifacts

71. [x] Add Z3-backed predicate implication for scope containment.
72. [x] Add finite-store frame, row-count, and invariant preservation checks for bounded relational fragments.
73. [x] Add symbolic execution for small repair programs over bounded stores.
74. [x] Add model checking for incident workflows: ingest, explain, approve, dry-run, apply, verify, rollback, audit, archive.
75. [x] Add temporal properties: no apply before approval, no approval without evidence, eventual verification, rollback availability, immutable audit.
76. [x] Add proof-obligation objects with status `proved`, `checked`, `counterexample`, `assumed`, or `not_supported`.
77. [x] Add proof-hole reporting so practical checks are honest about what remains unproved.
78. [x] Add proof-carrying repair bundles containing manifest, evidence slice, dry-run hash, policies, proof obligations, counterexamples, and ledger checkpoint.
79. [x] Add CEGAR-style refinement: emit counterexample, refine abstraction, rerun semantic analysis.
80. [x] Add signed proof/gate attestations for CI and incident review artifacts.

## I. Historical knowledge generation

81. [x] Add an incident archive index over past evidence, migrations, repair manifests, policies, and benchmark results.
82. [x] Add deterministic historical queries: "which migrations caused broad updates?", "which reports repeatedly derived from damaged rows?", "which repairs lacked rollback?"
82a. [x] Add public historical-failure counterfactuals that verify postmortem source assertions and show current Patchline gates would flag destructive primary-data mutations and split-brain write divergence.
82b. [x] Add source-derived public incident observations from linked GitLab 2017 postmortem/issues/API records, so historical counterfactuals are grounded in a multi-document public corpus rather than a single synthetic reconstruction.
83. [ ] Add repair outcome history: dry-run hash, applied SQL hash, verification result, rollback availability, and later recurrence.
84. [ ] Add semantic regression detection over history: a new migration resembles prior high-risk trace shapes or violates learned invariant candidates.
85. [ ] Add historical invariant dashboards as JSON/Markdown generated from actual records and evidence, with counterexamples included.
86. [ ] Add "new knowledge" reports that summarize previously unconnected causal links across deploys, traces, migrations, row mutations, and repairs.
87. [ ] Add time-windowed incident slicing so teams can ask what changed before a corruption window.
88. [ ] Add organization-local benchmark generation from historical incidents with redaction-safe hashes.
89. [ ] Add redaction that preserves proof-relevant structure while hiding sensitive values.
90. [ ] Add content-addressed bundle export and verification for archived incident knowledge.

## J. Benchmarks, gates, and adoption

91. [x] Add strict benchmark suites with labels, pinned analyzer hashes, precision, and recall.
92. [x] Add CI gates with precision/recall floors, GitHub Actions summaries, annotations, and distinct exit codes.
93. [x] Add deterministic policy gates over manifests, dry-runs, migrations, and ledger checkpoints.
94. [x] Add hash-chained audit ledger entries and checkpoint verification.
95. [ ] Add public OSS migration fixtures with dialects, labels, expected schema states, semantic obligations, and pinned hashes.
96. [ ] Add benchmark cases for provenance slicing, repair replay, invariant checking, solver obligations, policy gates, and bundle verification.
97. [ ] Add baseline comparison for benchmark regressions against `main`.
98. [ ] Add SARIF output and reusable GitHub Action packaging for semantic migration and repair gates.
99. [ ] Add fuzz tests and performance benchmarks for parsers, evidence ingest, graph slicing, replay, abstract interpretation, and historical archive queries.
100. [ ] Publish an end-to-end demo: ingest historical telemetry, reconstruct trace semantics, discover causal knowledge, analyze migration semantics, replay repair, check obligations, export proof-carrying bundle, and enforce CI.
