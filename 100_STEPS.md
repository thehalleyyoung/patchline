# 100 steps for high-impact analysis of pre-existing repos

Patchline's strongest promise should be industrially practical and research-interestingly novel at the same time: **download a real repository that was not built for Patchline, infer the data-repair surface it already contains, run deterministic analysis, optionally add bounded LLM-generated tests/guards/repair code in an isolated workspace, and then re-analyze the changed repo more deeply than before.**

The industrial promise is plug-and-play usefulness for teams with messy existing systems: migrations, source SQL, incident notes, telemetry exports, tests, runbooks, CI, and repair scripts. The research promise is a new feedback loop for software/data repair: repo-native fact extraction -> risk/cause/repair hypotheses -> generated interventions -> semantic before/after verification -> ablations on real projects.

Every addition to the repo should be judged by whether it makes that loop more accurate, safer, more reproducible, or more actionable on real repositories.

## Stage 0: Keep the promise sharp

1. [x] Define Patchline as deterministic data/code repair analysis for existing projects, not as a labeling framework.
2. [x] Treat arbitrary GitHub repos, local checkouts, archives, and project exports as first-class inputs.
3. [x] Keep LLMs optional and bounded: generated code is a proposed artifact to analyze, not a trusted oracle.
4. [x] Make every high-level finding traceable to file paths, line ranges, identifiers, timestamps, and hashes.
5. [x] Prefer outputs maintainers can act on today: commands, patches, SARIF, Markdown summaries, JSON facts, and risk rankings.
6. [x] Preserve the research novelty: combine project-native mining, repair synthesis, provenance, semantic checks, and ablation-driven evaluation.
7. [x] Add only features that improve real-repo analysis quality, generated-change safety, or before/after explanatory power.
8. [x] Avoid demo-only features that work only on curated fixtures.
9. [x] Keep outputs stable enough for CI, artifact bundles, and comparative experiments.
10. [x] Maintain a one-command path from public repo URL to actionable report.

## Stage 1: Download real repositories reproducibly

11. [x] Add `patchline repo fetch owner/repo --ref <ref> --out <dir>` with normalized source metadata.
12. [x] Support GitHub URLs, `owner/repo`, local paths, tarballs, zip archives, and pre-existing worktrees.
13. [x] Record repo owner, name, ref, resolved commit SHA, subpath, archive hash, fetch timestamp, and tool version.
14. [x] Cache downloads by content hash so repeated experiments reuse the same source.
15. [x] Add safe subpath extraction for scanning only migrations, services, docs, or exports inside large repos.
16. [x] Emit `source.json` for every run so reports can be reproduced or cited.
17. [x] Detect ignored/generated/vendor directories and explain what was skipped.
18. [x] Provide a `--full` mode for exhaustive scans and a default mode optimized for maintainer signal.
19. [x] Add public-repo smoke cases across Rails, Django, Alembic, Flyway, Prisma, TypeORM, Go services, and mixed monorepos.
20. [x] Make the fetch stage independent from later analysis so failures are easy to isolate.

## Stage 2: Build a project-native fact layer

21. [x] Add `patchline repo inventory <path>` for languages, frameworks, migration systems, DB engines, CI, test commands, and deploy config.
22. [x] Classify files as migrations, source SQL, ORM/schema declarations, tests, fixtures, incidents, runbooks, repair scripts, logs, traces, configs, or unknown.
23. [x] Extract tables, columns, models, endpoints, queues, jobs, reports, incidents, commits, PRs, deploy IDs, timestamps, and error names.
24. [x] Infer schema evolution from migrations and ORM declarations without requiring a Patchline schema.
25. [x] Extract embedded SQL and persistence calls from common languages.
26. [x] Detect migration frameworks and native commands such as `rails db:migrate`, `manage.py migrate`, `prisma migrate`, `alembic`, `flyway`, and `go test`.
27. [x] Preserve unknown JSON/YAML/TOML/log fields as searchable evidence rather than discarding them.
28. [x] Emit `facts.jsonl` as the stable low-level interface for later stages.
29. [x] Emit `project-map.md` showing where data-change evidence lives in the repo.
30. [x] Hash every fact with provenance so generated prompts and reports can cite exact context.

## Stage 3: Do real baseline analysis before generation

31. [x] Rank risky migrations and SQL snippets by destructive operation, breadth, reversibility, guard presence, transaction use, and affected table importance.
32. [x] Rank risky code paths by persistent write breadth, missing idempotency, missing transaction boundaries, retry hazards, and weak rollback behavior.
33. [x] Link risk candidates to nearby tests, docs, incidents, commits, deploy notes, and repair scripts using identifiers and time signals.
34. [x] Detect cause clusters around shared tables, dates, incidents, deploys, commits, error names, and changed files.
35. [x] Detect repair clusters around rollback scripts, fix migrations, reconciliation jobs, backfills, runbooks, and generated/manual data patches.
36. [x] Emit top findings as problem/cause/repair hypotheses with evidence, confidence, and specific next commands.
37. [x] Emit SARIF for high-risk data changes so existing code-scanning systems can display results.
38. [x] Emit "native checks to run" based on detected project tooling.
39. [x] Compare findings to simple baselines: grep-only SQL risk, SQL-only scanning, identifier-only linking, and date-only linking.
40. [x] Store a baseline report that later generated-code analysis must improve or explain.

## Stage 4: Create bounded LLM-generated interventions

41. [x] Add `patchline repo propose --from-report <baseline> --kind tests|guards|instrumentation|repair --out <worktree>`.
42. [x] Build prompt context only from deterministic facts, top-ranked evidence, relevant file excerpts, and explicit constraints.
43. [x] Record model name, prompt hash, context hash, output hash, generated files, and target risk IDs.
44. [x] Generate tests that exercise risky migrations, ORM paths, repair scripts, or data invariants.
45. [x] Generate migration guards for row counts, table existence, transaction safety, reversibility, dry-run behavior, and bounded scope.
46. [x] Generate instrumentation patches that expose metrics, structured logs, or trace attributes for risky data changes.
47. [x] Generate repair candidates as patches plus manifests containing scope, preconditions, postconditions, rollback, and validation commands.
48. [x] Generate SQL explain/dry-run scripts for maintainers who cannot run full production-like tests.
49. [x] Apply generated code only in an isolated worktree or patch file, never silently in the user's active tree.
50. [x] Label generated code as untrusted until deterministic re-analysis and project-native checks complete.

## Stage 5: Re-analyze generated code more deeply

51. [x] Add `patchline repo compare --before <repo> --after <generated-worktree>` for deterministic before/after analysis.
52. [x] Detect new risky SQL, broader writes, destructive DDL, missing guards, and non-idempotent generated changes.
53. [x] Check whether generated tests cover the exact tables, code paths, and risk IDs they claim to cover.
54. [x] Check whether generated guards fail closed when assumptions or database metadata are unavailable.
55. [x] Check whether generated repairs preserve scope and include rollback/validation obligations.
56. [x] Run safe native tests when discovered, recording commands, exit status, logs, and hashes.
57. [x] Run Patchline semantic checks even when native tests are unavailable.
58. [x] Emit a before/after delta: risks reduced, risks added, coverage added, evidence strengthened, and unresolved risks.
59. [x] Reject or flag generated changes that improve documentation while making data safety worse.
60. [x] Produce a maintainer-ready patch review: what changed, why it was proposed, how it was checked, and what remains uncertain.

## Stage 6: Add semantic depth that is research-worthy

61. [x] Build provenance slices connecting migration -> table -> source path -> test -> incident -> repair.
62. [x] Add Datalog-style queries for minimal cause sets, shared ancestors, repair lineage, and affected outputs.
63. [x] Add abstract interpretation for SQL/data-change effects when concrete data is unavailable.
64. [x] Add symbolic checks for idempotency, reversibility, frame conditions, and scope preservation.
65. [x] Add temporal windowing around incidents, releases, migrations, and generated repairs.
66. [x] Add recurrence detection for patterns similar to prior risky migrations or repairs in the same repo.
67. [x] Add policy checks requiring guard, rollback, approval, dry run, or tests for selected risk classes.
68. [x] Add proof-carrying summaries when a repair's scope and frame conditions can be checked.
69. [x] Explain rankings with feature contributions so results are inspectable and ablation-friendly.
70. [x] Treat generated code as an intervention in a repair-analysis loop, not merely as text completion.

## Stage 7: Make it industrially easy to run

71. [x] Add `patchline repo analyze --github owner/repo --stages inventory,baseline,propose,compare,deep`.
72. [x] Support `--no-llm` for deterministic-only analysis.
73. [x] Support `--llm-command <cmd>` so users can plug in local or hosted code generation without coupling Patchline to one provider.
74. [x] Support `--proposal-kind tests|guards|instrumentation|repair|all`.
75. [x] Support `--budget files=N,lines=N,tokens=N,changes=N` to bound generated-code scope.
76. [x] Emit `analysis-bundle/` containing `source.json`, `facts.jsonl`, `baseline.json`, `proposal.patch`, `compare.json`, `summary.md`, and `summary.sarif`.
77. [x] Add resume mode so fetch, inventory, and baseline stages are reused across experiments.
78. [x] Add redaction mode for identifiers, literals, customer names, and secrets while preserving joins and hashes.
79. [ ] Add CI mode that uploads SARIF and stores the analysis bundle as an artifact.
80. [ ] Add a clean "copy these commands" report for maintainers evaluating a repo for the first time.

## Stage 8: Evaluate novelty on real projects

81. [ ] Maintain a public matrix of real repo slices by ecosystem, migration framework, repo size, and available evidence types.
82. [ ] For each slice, report inventory coverage, risks, linked candidates, time signals, generated artifacts, and before/after deltas.
83. [ ] Compare against grep-only risk detection.
84. [ ] Compare against SQL-only analysis without code/docs/evidence links.
85. [ ] Compare against identifier-only linking without temporal signals.
86. [ ] Compare against temporal-only linking without identifiers.
87. [ ] Compare fact-grounded generated code against prompt-without-facts generated code.
88. [ ] Compare deterministic re-analysis against trusting generated code without verification.
89. [ ] Track false positives and false negatives with sampled public findings and explicit adjudication notes.
90. [ ] Report runtime, memory, download size, cache hit rate, and maintainers' review burden.

## Stage 9: Keep every future addition high-impact

91. [ ] Before adding a feature, name the real-repo failure mode it fixes.
92. [ ] Before adding a parser, show the new facts it extracts from at least one real project.
93. [ ] Before adding a generated-code feature, show the deterministic checks that can catch bad generated output.
94. [ ] Before adding a report section, show what maintainer decision it improves.
95. [ ] Before adding a metric, show how it affects ranking, repair safety, or comparison against baselines.
96. [ ] Prefer fewer, stronger findings over exhaustive low-signal output.
97. [ ] Keep non-deterministic steps optional, bounded, auditable, and followed by deterministic analysis.
98. [ ] Make every new command runnable on a downloaded public repo in a few shell commands.
99. [ ] Keep industrial value and research novelty aligned: practical reports should also support rigorous experiments.
100. [ ] End each development cycle with a real-repo demonstration where baseline analysis, generated intervention, and deeper re-analysis produce a better result than the previous version.
