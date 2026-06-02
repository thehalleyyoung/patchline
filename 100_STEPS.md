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
79. [x] Add CI mode that uploads SARIF and stores the analysis bundle as an artifact.
80. [x] Add a clean "copy these commands" report for maintainers evaluating a repo for the first time.

## Stage 8: Evaluate novelty on real projects

81. [x] Maintain a public matrix of real repo slices by ecosystem, migration framework, repo size, and available evidence types.
82. [x] For each slice, report inventory coverage, risks, linked candidates, time signals, generated artifacts, and before/after deltas.
83. [x] Compare against grep-only risk detection.
84. [x] Compare against SQL-only analysis without code/docs/evidence links.
85. [x] Compare against identifier-only linking without temporal signals.
86. [x] Compare against temporal-only linking without identifiers.
87. [x] Compare fact-grounded generated code against prompt-without-facts generated code.
88. [x] Compare deterministic re-analysis against trusting generated code without verification.
89. [x] Track false positives and false negatives with sampled public findings and explicit adjudication notes.
90. [x] Report runtime, memory, download size, cache hit rate, and maintainers' review burden.

## Stage 9: Keep every future addition high-impact

91. [x] Before adding a feature, name the real-repo failure mode it fixes.
92. [x] Before adding a parser, show the new facts it extracts from at least one real project.
93. [x] Before adding a generated-code feature, show the deterministic checks that can catch bad generated output.
94. [x] Before adding a report section, show what maintainer decision it improves.
95. [x] Before adding a metric, show how it affects ranking, repair safety, or comparison against baselines.
96. [x] Prefer fewer, stronger findings over exhaustive low-signal output.
97. [x] Keep non-deterministic steps optional, bounded, auditable, and followed by deterministic analysis.
98. [x] Make every new command runnable on a downloaded public repo in a few shell commands.
99. [x] Keep industrial value and research novelty aligned: practical reports should also support rigorous experiments.
100. [x] End each development cycle with a real-repo demonstration where baseline analysis, generated intervention, and deeper re-analysis produce a better result than the previous version.

## Stage 10: Make Patchline irresistible to real maintainers

101. [x] Add a `patchline doctor` command that diagnoses missing tools, cache state, network reachability, and safe native-test availability before a repo analysis run.
102. [x] Add an interactive-free `patchline quickstart --github owner/repo --subpath path` command that emits exactly the next three copy/paste commands and expected artifacts.
103. [x] Add a maintainer triage dashboard that groups findings by owner-relevant surfaces: migrations, app write paths, jobs, tests, incidents, runbooks, and generated interventions.
104. [x] Add stable finding IDs that survive file moves and line drift by hashing normalized evidence, table identifiers, operation families, and provenance slices.
105. [x] Add suppression files with expiry dates, rationale, owner, evidence hash, and automatic stale-suppression detection.
106. [x] Add a "why now" report that highlights newly introduced risks relative to a previous commit, release tag, or stored baseline bundle.
107. [x] Add a "what changed since last run" report that compares facts, ranked risks, links, generated artifacts, and deterministic check outcomes.
108. [x] Add Slack/GitHub-friendly summaries with only the top maintainer action, top risk, reproduction command, and bundle link.
109. [x] Add `patchline explain <finding-id>` to print evidence, ranking factors, alternatives considered, proof holes, and the exact commands that verify the finding.
110. [x] Add a public gallery of real-repo analysis bundles with redacted identifiers, pinned commits, expected hashes, and maintainer-facing screenshots.

## Stage 11: Become the best public corpus for data-change repair

111. [x] Expand the real-repo catalog from 4 slices to at least 25 pinned slices across Rails, Django, Alembic, Flyway, Liquibase, Prisma, TypeORM, EF Core, Go, Java, Node, and monorepos.
112. [x] Add non-GitHub sources from GitLab, Bitbucket, SourceHut, and release tarballs with the same provenance and cache semantics.
113. [x] Add dataset cards for every public slice covering license, commit, ecosystem, migration framework, evidence types, known limitations, and reproducibility commands.
114. [x] Add a corpus fairness audit that reports language/framework/source-host coverage and flags over-reliance on any one ecosystem.
115. [x] Add stratified benchmark manifests so experiments can report per-ecosystem results instead of only aggregate numbers.
116. [x] Add automatic stale-ref detection that checks whether pinned public repos still download and whether expected hashes still match.
117. [x] Add a public issue template for users to nominate real repos and failure modes that Patchline should support.
118. [x] Add a corpus minimizer that extracts the smallest public subpath preserving each finding, link, and generated-intervention behavior.
119. [x] Add cross-repo recurrence reports that identify repeated data-change failure modes across unrelated projects without leaking private code.
120. [x] Add corpus release artifacts with signed checksums, generated reports, and one-command reproduction instructions.

## Stage 12: Add paper-grade evaluation methodology

121. [x] Define primary research questions for repo-native fact extraction, risk ranking, evidence linking, generated-intervention safety, and before/after re-analysis.
122. [ ] Add experiment drivers that run every research question from a clean checkout and write immutable result ledgers.
123. [ ] Add bootstrap confidence intervals for ranking, linking, generated-check, runtime, and review-burden metrics.
124. [ ] Add paired statistical tests for Patchline versus grep-only, SQL-only, identifier-only, temporal-only, and no-facts-generation baselines.
125. [ ] Add effect-size reporting so improvements are not presented only as p-values.
126. [ ] Add sensitivity analysis over budgets, finding caps, link thresholds, temporal windows, and risk-weight settings.
127. [ ] Add ablation dashboards that show which feature families matter for each ecosystem and failure mode.
128. [ ] Add negative controls that run on documentation-only, vendor-only, and test-only slices where Patchline should avoid high-confidence repair claims.
129. [ ] Add reviewer-mode scripts that rebuild all tables, figures, and claims from raw generated JSON without manual copying.
130. [ ] Add an artifact consistency checker that fails when README claims, generated tables, expected hashes, and experiment outputs diverge.

## Stage 13: Make generated interventions genuinely safer

131. [ ] Add patch application in disposable worktrees so generated changes can be analyzed as real diffs rather than detached artifact files.
132. [ ] Add language-aware generated-test placement for Rails, Django, Go, Java, Node, and Python projects.
133. [ ] Add mutation testing for generated guards by deleting required checks and proving deterministic compare rejects the weakened artifact.
134. [ ] Add sandboxed native-test execution profiles for common ecosystems with timeouts, file-system write constraints, and network-off defaults.
135. [ ] Add generated-patch provenance comments that cite risk IDs, fact hashes, and evidence paths without exposing redacted secrets.
136. [ ] Add repair-manifest schemas with machine-checkable preconditions, postconditions, rollback steps, validation commands, and owner review status.
137. [ ] Add patch minimization that removes generated files or hunks that do not improve coverage, ranking, or deterministic check outcomes.
138. [ ] Add generated-change risk budgeting that rejects interventions adding more new risk than they cover.
139. [ ] Add a "safe to review" badge only when deterministic checks pass, native checks are either passed or explicitly unavailable, and proof holes are listed.
140. [ ] Add generated-intervention replay so reviewers can reproduce prompt context, generation output, applied diff, compare results, and hashes.

## Stage 14: Push semantic analysis beyond static heuristics

141. [ ] Add dialect-aware SQL normalization for PostgreSQL, MySQL, SQLite, SQL Server, Oracle, and BigQuery migrations.
142. [ ] Add ORM-aware write-effect extraction for Active Record, Django ORM, SQLAlchemy, Prisma, TypeORM, Hibernate, and GORM.
143. [ ] Add transaction-boundary inference across migration DSLs, raw SQL, app code, jobs, and generated repairs.
144. [ ] Add idempotency classification for migrations, backfills, repair jobs, generated scripts, and runbook commands.
145. [ ] Add lock and concurrency hazard detection for data changes likely to block deploys or background jobs.
146. [ ] Add data-retention and privacy hazard detection for broad deletes, anonymization changes, export scripts, and rollback gaps.
147. [ ] Add invariant mining from tests, schema constraints, validations, fixtures, and production-like example data.
148. [ ] Add trace-to-code linking for OpenTelemetry, Datadog-style span exports, structured logs, deploy markers, and incident timelines.
149. [ ] Add approximate blast-radius estimation from table centrality, foreign-key reachability, code-path fanout, and query usage evidence.
150. [ ] Add proof-hole minimization that ranks the smallest missing evidence needed to upgrade a warning into a checked claim.

## Stage 15: Integrate where serious teams work

151. [ ] Add a GitHub App or action workflow that comments on pull requests with only new or changed data-risk findings.
152. [ ] Add GitLab CI and Bitbucket Pipelines examples with SARIF or equivalent code-quality artifacts.
153. [ ] Add Datadog-style export adapters for deploy events, incidents, traces, logs, monitors, SLOs, and notebooks without depending on proprietary APIs.
154. [ ] Add OpenTelemetry collector export ingestion for traces and logs linked to repo-native findings.
155. [ ] Add Jira/Linear incident export adapters that preserve issue IDs, timestamps, owners, labels, and repair links.
156. [ ] Add Kubernetes and Terraform scanners for database jobs, migration jobs, cron repairs, secrets references, and deploy ordering.
157. [ ] Add database dry-run hooks for local Postgres/MySQL containers with schema-only execution and no production credentials.
158. [ ] Add pre-commit and pre-push modes that run fast finding deltas without downloading external repos.
159. [ ] Add CODEOWNERS-aware routing so findings and generated interventions identify likely reviewers.
160. [ ] Add enterprise-safe offline mode that validates all cached repo inputs, adapters, and reports without network access.

## Stage 16: Make the project delightful for contributors

161. [ ] Add architecture docs that explain the fetch, inventory, intake, baseline, proposal, compare, deep-analysis, and gate layers.
162. [ ] Add plugin interfaces for parsers, fact extractors, linkers, rankers, proposal generators, compare checks, and report renderers.
163. [ ] Add golden-fixture generators that turn real-repo slices into minimal deterministic tests without vendoring entire repos.
164. [ ] Add fuzz tests for parsers, fact normalization, redaction, SQL analysis, archive extraction, and report loading.
165. [ ] Add performance benchmarks with budgets for large repos, monorepos, generated bundles, and four-repo matrix runs.
166. [ ] Add structured logging and trace spans for Patchline itself so long analyses can be diagnosed.
167. [ ] Add a contributor command that runs formatting, focused tests, fast gates, and forbidden-reference scans in one invocation.
168. [ ] Add issue labels and triage templates for new ecosystem support, parser requests, false positives, false negatives, and artifact regressions.
169. [ ] Add compatibility tests for macOS, Linux, and containerized execution with minimal tool assumptions.
170. [ ] Add a changelog discipline that links each user-visible feature to a real-repo proof and a gate.

## Stage 17: Strengthen trust, security, and privacy

171. [ ] Add secret-scanning tests that prove reports, prompts, bundles, generated code, logs, and redacted artifacts do not leak secret-like values.
172. [ ] Add prompt-context minimization that includes only evidence needed for selected risks and reports excluded context counts.
173. [ ] Add redaction stability tests across repeated runs, resume mode, bundles, SARIF, generated prompts, and comparison reports.
174. [ ] Add supply-chain provenance for binaries, release archives, generated experiment artifacts, and public corpus downloads.
175. [ ] Add signed release checksums and reproducible build instructions.
176. [ ] Add threat-model documentation for untrusted repos, archives, generated code, native tests, and adapter inputs.
177. [ ] Add archive bomb, path traversal, symlink escape, and malformed tar/zip regression tests.
178. [ ] Add generated-code quarantine rules that prevent execution unless a user explicitly enables safe native checks.
179. [ ] Add privacy-preserving aggregate metrics so teams can compare risk trends without uploading source or raw evidence.
180. [ ] Add security review gates that must pass before new adapters, generators, archive handlers, or execution features merge.

## Stage 18: Produce Best Paper quality narratives

181. [ ] Add generated case studies for at least eight public repos showing problem, evidence, generated intervention, deterministic rejection or acceptance, and maintainer action.
182. [ ] Add a taxonomy of real data-change repair failure modes discovered across the public corpus.
183. [ ] Add qualitative coding notes for false positives, false negatives, proof holes, and maintainer decisions.
184. [ ] Add side-by-side examples where Patchline finds a cross-file repair clue that grep-only and SQL-only baselines miss.
185. [ ] Add examples where Patchline rejects plausible generated code that would otherwise look useful in a normal code-review diff.
186. [ ] Add examples where generated tests or guards improve reviewability without claiming to fully repair the underlying issue.
187. [ ] Add a limitations ledger that distinguishes unsupported ecosystems, uncertain causality, missing runtime evidence, and intentionally conservative checks.
188. [ ] Add claims-to-evidence mapping for every abstract, introduction, and evaluation claim expected in a future paper.
189. [ ] Add figure-generation scripts for the repair-analysis loop, architecture, corpus composition, ablations, and before/after intervention outcomes.
190. [ ] Add a reviewer walkthrough that starts from a fresh machine and ends with regenerated tables, figures, reports, and case-study bundles.

## Stage 19: Build toward a 1000-star open-source project

191. [ ] Add a polished landing README section with a 60-second demo, screenshots, badges, install commands, and real public-repo output.
192. [ ] Add release binaries and package-manager installation paths for Homebrew, Docker, GitHub Releases, and Go install.
193. [ ] Add a hosted documentation site with tutorials for maintainers, researchers, security reviewers, and contributors.
194. [ ] Add short screencasts showing first-run analysis, generated intervention review, CI integration, and artifact reproduction.
195. [ ] Add "awesome Patchline" examples contributed by users across ecosystems and source hosts.
196. [ ] Add comparison pages against code scanning, SQL linters, migration tools, observability dashboards, and AI coding assistants.
197. [ ] Add a public roadmap board where every planned feature links to a real-repo failure mode, gate, and expected artifact.
198. [ ] Add monthly reproducibility reports that rerun public gates and publish cache status, failures, fixes, and benchmark trends.
199. [ ] Add contributor recognition for new real-repo slices, ecosystem parsers, false-positive reductions, and artifact improvements.
200. [ ] Add a release-quality capstone demo where a fresh user downloads four unfamiliar repos, finds high-signal repair risks, generates bounded interventions, rejects bad output, and regenerates experiment-ready evidence in one documented session.
