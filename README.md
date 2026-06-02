# Patchline

Patchline is a deterministic checker for the data-change material teams already have: GitHub repos, migration directories, service source trees, telemetry exports, JSON logs, incident notes, and repair scripts.

It is not an AI tool, and it does not require you to label data or adopt a Patchline-specific format first. Point it at existing files; it inventories what is there, finds risky changes across SQL migrations, NoSQL stores, data pipelines, protobuf/Avro schemas, and infrastructure ordering, and prints the next commands that can run immediately.

[![deterministic](https://img.shields.io/badge/deterministic-data--repair-2563eb)](#60-second-demo)
[![not-ai](https://img.shields.io/badge/not--ai-static--analysis-16a34a)](#why-this-is-useful)
[![public-repos](https://img.shields.io/badge/proven_on-public_repos-7c3aed)](#real-public-repo-output)
[![artifact-review](https://img.shields.io/badge/artifact-reviewer_walkthrough-f97316)](docs/reviewer-walkthrough.md)
[![available](https://img.shields.io/badge/artifact-available-2563eb)](docs/artifact-badges.md)
[![functional](https://img.shields.io/badge/artifact-functional-16a34a)](docs/artifact-badges.md)
[![reusable](https://img.shields.io/badge/artifact-reusable-7c3aed)](docs/artifact-badges.md)
[![reproducible](https://img.shields.io/badge/artifact-reproducible-f97316)](docs/artifact-badges.md)

## 60-second demo

Install:

```bash
go install github.com/thehalleyyoung/patchline/cmd/patchline@latest
# or from a checkout:
go build -o bin/patchline ./cmd/patchline
```

Run Patchline on a real public migration directory, report ranked data-change risks, and keep generated code quarantined:

```bash
go run ./cmd/patchline repo analyze \
  --github lobsters/lobsters \
  --ref 3b80b47aa5aaba37ec44413e7d1dc96fcf1585b6 \
  --subpath db/migrate \
  --stages inventory,baseline,propose,compare \
  --proposal-kind all \
  --budget files=8,lines=100,tokens=12000,changes=2 \
  --no-llm \
  --out results/generated/landing-demo \
  --json
```

![Patchline landing demo screenshot](docs/assets/landing-demo.svg)

### Real public-repo output

`make landing-readme-gate` regenerates this from pinned `lobsters/lobsters` code.

| Public slice | Files scanned | Ranked data-change risks | Provenance slices | Generated review artifacts | Deterministic checks failed |
| --- | ---: | ---: | ---: | ---: | ---: |
| `lobsters/lobsters` `db/migrate` | 158 | 328 | 100 | 8 | 0 |

```bash
go run ./cmd/patchline intake . --out results/generated/intake
go run ./cmd/patchline intake --github owner/repo --subpath path/to/migrations --out results/generated/intake
```

If you want a reproducible local workspace first, fetch and inventory the repo before intake:

```bash
go run ./cmd/patchline repo fetch django/django \
  --subpath django/contrib/auth/migrations \
  --out results/generated/repos/django-auth
go run ./cmd/patchline doctor --github django/django \
  --subpath django/contrib/auth/migrations \
  --out results/generated/repos/django-auth-doctor
go run ./cmd/patchline quickstart --github django/django \
  --subpath django/contrib/auth/migrations \
  --out results/generated/repos/django-auth-quickstart
go run ./cmd/patchline repo analyze --github django/django \
  --subpath django/contrib/auth/migrations \
  --stages inventory,baseline,propose,compare,deep \
  --budget files=15,lines=120,tokens=50000,changes=3 \
  --redact \
  --no-llm \
  --out results/generated/repos/django-auth-analysis
go run ./cmd/patchline repo inventory \
  results/generated/repos/django-auth/django-django-*/django/contrib/auth/migrations \
  --out results/generated/repos/django-auth-inventory

go run ./cmd/patchline intake \
  results/generated/repos/django-auth/django-django-*/django/contrib/auth/migrations \
  --out results/generated/repos/django-auth-intake

go run ./cmd/patchline repo baseline \
  --inventory results/generated/repos/django-auth-inventory \
  --intake results/generated/repos/django-auth-intake \
  --out results/generated/repos/django-auth-baseline

go run ./cmd/patchline repo propose \
  --from-report results/generated/repos/django-auth-baseline \
  --proposal-kind all \
  --budget files=15,lines=120,tokens=50000,changes=3 \
  --out results/generated/repos/django-auth-proposal

go run ./cmd/patchline repo compare \
  --before results/generated/repos/django-auth-baseline \
  --after results/generated/repos/django-auth-proposal \
  --out results/generated/repos/django-auth-compare

go run ./cmd/patchline repo hook pre-commit --root . --json
go run ./cmd/patchline repo hook pre-push --root . --base origin/main --json
go run ./cmd/patchline repo offline --analysis results/generated/repos/django-auth-analysis --json
```

Quickstart writes `quickstart.json`, `quickstart.md`, and `commands.sh` with exactly three copy/paste commands plus expected artifacts. Doctor writes `doctor.json` and `doctor.md` with tool availability, cache state, scanned files, facts, and safe native checks before analysis. Fetch writes `source.json` with source provenance, resolved GitHub commit, archive hash, timestamp, tool version, and cache metadata. Inventory writes `inventory.json`, `inventory.md`, `facts.jsonl`, and `project-map.md`. Baseline writes `baseline.json`, `baseline.md`, and `baseline.sarif`, including ranked risks, invariant candidates, privacy/lock hazards, blast-radius estimates, proof-hole minimizations, and trace-to-code links from OpenTelemetry, Datadog-style exports, structured logs, deploy markers, and incident timelines. Propose writes `prompt-context.json`, `prompt.txt`, `proposal.patch`, `proposal.json`, and generated untrusted artifacts under `patchline-proposals/`. Compare writes `compare.json` and `compare.md`. Triage writes `triage/triage.json` and `triage/triage.md`, also copied into `analysis-bundle/`, grouping findings by maintainer owner surface. See [docs/architecture.md](docs/architecture.md) for the fetch, inventory, intake, baseline, proposal, compare, deep-analysis, and gate layer contract.

`repo analyze` also writes `commands.md`, a copy/paste maintainer report with the equivalent one-command and staged command sequences plus the shareable bundle paths.

To plug in a local or hosted generator, pass `--llm-command '<cmd>'`; Patchline sends the prompt on stdin and stores the output as an untrusted artifact for deterministic compare. Use `--no-llm` when you want template-only analysis and rejection of any generator command.

Use `--budget files=N,lines=N,tokens=N,changes=N` to bound generated scope before a patch is written. `changes` limits targeted risks, `files` limits generated artifacts, `lines` limits each artifact, and `tokens` limits approximate generated output tokens.

Rerun `repo analyze --resume --out <same-dir>` to reuse existing fetch, inventory, intake, baseline, proposal, and compare artifacts while changing later experiment settings.

Run `repo offline --analysis <analysis-dir>` in restricted environments to validate cached repo inputs, adapter outputs, and generated analysis reports without performing network fetches.

Run `plugins list` or `plugins probe <path>` to inspect the deterministic parser, fact extractor, linker, ranker, proposal generator, compare check, and report renderer interfaces described in [docs/plugin-interfaces.md](docs/plugin-interfaces.md).

Run `golden-fixture generate --github owner/repo --ref <sha> --subpath <path> --out <dir>` to turn a real-repo slice into a tiny deterministic Go test without vendoring the full repository; see [docs/golden-fixtures.md](docs/golden-fixtures.md).

<details>
<summary><strong>Full verification catalog</strong> — 430+ reproducible <code>make &lt;name&gt;-gate</code> checks, each with a positive proof and a negative control on real public-repo data (click to expand)</summary>

Run `make fuzz-coverage-gate` to execute the parser, fact normalization, redaction, SQL analysis, archive extraction, and report-loading fuzz seed suite plus a short stress pass and real-repo slice proof; see [docs/fuzzing.md](docs/fuzzing.md).

Run `make performance-budget-gate` to benchmark large-repo, monorepo, generated-bundle, and four-repo matrix analyses against explicit wall-clock and artifact-size budgets; see [docs/performance-budgets.md](docs/performance-budgets.md).

Add `--trace` to `repo analyze` to write structured JSONL spans and logs for long runs under `diagnostics/`; run `make diagnostics-gate` to prove the trace contract on four public repo slices. See [docs/diagnostics.md](docs/diagnostics.md).

Run `patchline contributor check` before a PR to execute formatting checks, focused Go tests, fast gates, ignored-roadmap hygiene, and forbidden-reference scans in one report; see [docs/contributor-check.md](docs/contributor-check.md).

Run `make compatibility-gate` to cross-build macOS/Linux binaries, validate the container recipe, and prove the current host can analyze a pinned public repo slice with minimal tools; see [docs/compatibility.md](docs/compatibility.md).

Run `make changelog-gate` before release notes change so each user-visible feature links to a public proof repo and a reproducing gate; see [CHANGELOG.md](CHANGELOG.md) and [docs/changelog-discipline.md](docs/changelog-discipline.md).

Run `make secret-scan-gate` to prove redacted reports, prompts, bundles, generated files, diagnostics logs, and CI artifacts do not leak deterministic canary values; see [docs/secret-scanning.md](docs/secret-scanning.md).

Run `make prompt-context-gate` to prove proposal prompts include only selected-risk evidence while reporting excluded context counts; see [docs/prompt-context-minimization.md](docs/prompt-context-minimization.md).

Run `make redaction-stability-gate` to prove redacted bundles, SARIF, prompts, and compare reports stay byte-stable across repeated and resumed runs; see [docs/redaction-stability.md](docs/redaction-stability.md).

Run `make supply-chain-provenance-gate` to prove binaries, release archives, generated experiment artifacts, and public corpus downloads carry deterministic provenance; see [docs/supply-chain-provenance.md](docs/supply-chain-provenance.md).

Run `make release-checksum-gate` to prove release archives get sorted SHA-256 checksums, signed attestations, and reproducible build instructions; see [docs/release-checksums.md](docs/release-checksums.md).

Run `make threat-model-gate` to verify the documented threat model for untrusted repos, archives, generated code, native tests, and adapter inputs against real artifacts; see [docs/threat-model.md](docs/threat-model.md).

Run `make archive-security-gate` to prove archive traversal, symlink escape, malformed archive, and archive-bomb regressions fail while pinned public GitHub archives still extract through the content-addressed cache; see [docs/archive-security.md](docs/archive-security.md).

Run `make generated-code-quarantine-gate` to prove generated proposal files stay non-executable, unapplied, and skipped from native execution unless `--run-native-tests` explicitly enables safe allowlisted checks; see [docs/generated-code-quarantine.md](docs/generated-code-quarantine.md).

Run `make privacy-metrics-gate` to emit source-free aggregate risk trend metrics with bucketed counts and salted cohort IDs for sharing without raw evidence; see [docs/privacy-aggregate-metrics.md](docs/privacy-aggregate-metrics.md).

Run `make security-review-gate` to prove adapter, generator, archive-handler, and execution-feature changes are blocked unless their required proof gates have passed; see [docs/security-review-gates.md](docs/security-review-gates.md).

Run `make generated-case-studies-gate` to generate case studies for eight pinned public repositories with problem, evidence, generated intervention, deterministic outcome, and maintainer action narratives; see [docs/generated-case-studies.md](docs/generated-case-studies.md).

Run `make failure-taxonomy-gate` to derive a public-corpus taxonomy of real data-change repair failure modes with examples, repair risks, and maintainer decisions; see [docs/failure-mode-taxonomy.md](docs/failure-mode-taxonomy.md).

Run `make qualitative-notes-gate` to emit qualitative coding notes for false-positive candidates, false-negative candidates, proof holes, and maintainer decisions across pinned public repos; see [docs/qualitative-coding-notes.md](docs/qualitative-coding-notes.md).

Run `make cross-file-examples-gate` to generate side-by-side examples where Patchline links cross-file repair clues that grep-only and SQL-only baselines miss; see [docs/cross-file-examples.md](docs/cross-file-examples.md).

Run `make rejected-generated-gate` to show plausible generated-code diffs being rejected by deterministic re-analysis before they can be trusted; see [docs/rejected-generated-examples.md](docs/rejected-generated-examples.md).

Run `make reviewability-examples-gate` to show generated tests and guards improving reviewability while preserving proof holes and avoiding full-repair claims; see [docs/reviewability-examples.md](docs/reviewability-examples.md).

Run `make limitations-ledger-gate` to publish a public-corpus limitations ledger for unsupported ecosystems, uncertain causality, missing runtime evidence, and intentionally conservative checks; see [docs/limitations-ledger.md](docs/limitations-ledger.md).

Run `make claims-evidence-gate` to map future abstract, introduction, and evaluation claims to concrete public-corpus artifacts, limitations, missing evidence, and reviewer checks; see [docs/claims-evidence.md](docs/claims-evidence.md).

Run `make paper-figures-gate` to regenerate SVG/JSON figures for the repair-analysis loop, architecture, corpus composition, ablations, and before/after intervention outcomes; see [docs/paper-figures.md](docs/paper-figures.md).

Run `make reviewer-walkthrough-gate` to simulate a fresh-machine artifact review that regenerates public-repo analyses, evaluation tables, figures, reports, and a case-study bundle; see [docs/reviewer-walkthrough.md](docs/reviewer-walkthrough.md).

Run `make landing-readme-gate` to prove the top-level README badges, 60-second demo, screenshot, install commands, and real public-repo output stay reproducible; see [scripts/generate-landing-demo.sh](scripts/generate-landing-demo.sh).

Run `make release-distribution-gate` to build release archives, signed checksums, Homebrew and Docker packaging paths, GitHub Release metadata, and a packaged-binary public-code proof; see [docs/release-distribution.md](docs/release-distribution.md).

Run `make docs-site-gate` to build the GitHub Pages documentation site with maintainer, researcher, security reviewer, and contributor tutorials backed by real public-repo output; see [docs/hosted-docs-site.md](docs/hosted-docs-site.md).

Run `make screencast-gate` to regenerate short terminal screencasts for first-run analysis, generated intervention review, CI integration, and artifact reproduction from pinned public-repo output; see [docs/screencasts.md](docs/screencasts.md).

Run `make awesome-patchline-gate` to regenerate the Awesome Patchline catalog of community-submitted examples across ecosystems and source hosts from pinned public code; see [docs/awesome-patchline.md](docs/awesome-patchline.md).

Run `make comparison-pages-gate` to regenerate comparison pages against code scanning, SQL linters, migration tools, observability dashboards, and AI coding assistants from pinned public-code evidence; see [docs/comparison-pages.md](docs/comparison-pages.md).

Run `make roadmap-board-gate` to regenerate the public roadmap board where every planned feature links to a real-repo failure mode, proof gate, and expected artifact; see [docs/roadmap-board.md](docs/roadmap-board.md).

Run `make reproducibility-report-gate` to regenerate monthly reproducibility reports that rerun public gates and publish cache status, failures, fixes, and benchmark trends; see [docs/reproducibility-reports.md](docs/reproducibility-reports.md).

Run `make contributor-recognition-gate` to regenerate contributor recognition for new real-repo slices, ecosystem parsers, false-positive reductions, and artifact improvements from public proof gates; see [docs/contributor-recognition.md](docs/contributor-recognition.md).

Run `make capstone-demo-gate` to regenerate the release-quality capstone demo where a fresh user analyzes four unfamiliar repos, generates bounded interventions, rejects bad output, and rebuilds experiment-ready evidence; see [docs/capstone-demo.md](docs/capstone-demo.md).

Run `make artifact-evaluation-kit-gate` to regenerate the artifact-evaluation landing kit with reviewer roles, time budgets, expected outputs, and pass/fail criteria; see [docs/artifact-evaluation-kit.md](docs/artifact-evaluation-kit.md).

Run `make artifact-container-profile-gate` to verify the one-command artifact VM/container profile that rebuilds public results without host-specific assumptions; see [docs/artifact-container-profile.md](docs/artifact-container-profile.md).

Run `make artifact-badges-gate` to regenerate artifact badges for reusable, available, functional, and reproducible evidence with gate-backed justifications; see [docs/artifact-badges.md](docs/artifact-badges.md).

Run `make paper-appendix-gate` to render claims, limitations, figures, tables, and reproduction commands from current generated artifacts; see [docs/paper-appendix.md](docs/paper-appendix.md).

Run `make reviewer-dry-run-logs-gate` to generate anonymized reviewer dry-run logs with setup failures, fixes, and final regenerated public-code results; see [docs/reviewer-dry-run-logs.md](docs/reviewer-dry-run-logs.md).

Run `make artifact-release-manifest-gate` to generate the deterministic artifact DOI/release manifest with exact refs, archives, checksums, and command versions; see [docs/artifact-release-manifest.md](docs/artifact-release-manifest.md).

Run `make rebuttal-response-workspace-gate` to generate a public rebuttal-response workspace linking likely reviewer concerns to evidence and limitations; see [docs/rebuttal-response-workspace.md](docs/rebuttal-response-workspace.md).

Run `make camera-ready-checklist-gate` to block camera-ready release when claims, figures, tables, or docs drift from generated evidence; see [docs/camera-ready-checklist.md](docs/camera-ready-checklist.md).

Run `make independent-replication-gate` to generate no-credential replication instructions using anonymous public archives and ordinary tools; see [docs/independent-replication.md](docs/independent-replication.md).

Run `make failure-injection-suite-gate` to prove artifact checks fail loudly when refs, caches, or generated evidence drift; see [docs/failure-injection-suite.md](docs/failure-injection-suite.md).

Run `make longitudinal-public-reruns-gate` to rerun public-corpus slices over multiple historical commits per repository and summarize risk/evidence deltas; see [docs/longitudinal-public-reruns.md](docs/longitudinal-public-reruns.md).

Run `make migration-age-stratification-gate` to stratify real public migrations by age band (recent vs old) and change type (schema-only vs backfill-heavy) and compare ranked-risk density across strata; see [docs/migration-age-stratification.md](docs/migration-age-stratification.md).

Run `make ecosystem-balanced-benchmark-gate` to build an ecosystem-balanced benchmark manifest with equal representation across Rails, Django, Alembic, Prisma, TypeORM, Liquibase, Flyway, EF Core, and Go migrators, plus a real-code proof sample; see [docs/ecosystem-balanced-benchmark.md](docs/ecosystem-balanced-benchmark.md).

Run `make repository-size-stratification-gate` to stratify the catalog into small apps, medium services, monorepos, and infrastructure-heavy repos and prove a representative real download from each stratum; see [docs/repository-size-stratification.md](docs/repository-size-stratification.md).

Run `make maintainer-action-simulation-gate` to label every ranked finding with a simulated maintainer decision (accept, revise, reject, defer, needs-runtime-evidence) from deterministic signals across real repos; see [docs/maintainer-action-simulation.md](docs/maintainer-action-simulation.md).

Run `make severity-calibration-gate` to validate severity against independent danger evidence (incident/cause clusters, rollback/fix migrations, recurrences) and report the calibration lift across real repos; see [docs/severity-calibration.md](docs/severity-calibration.md).

Run `make fp-adjudication-gate` to run a blinded, three-rater false-positive adjudication of findings and report Cohen's kappa, three-way agreement, and the majority-adjudicated false-positive rate; see [docs/fp-adjudication.md](docs/fp-adjudication.md).

Run `make fn-discovery-gate` to seed known public-incident hazard analogues into a real migration layout, measure detection recall, and surface the false negatives Patchline under-escalates or misses (cross-validated on real repo migrations); see [docs/fn-discovery.md](docs/fn-discovery.md).

Run `make ablation-study-gate` to ablate provenance links, cross-file context, generated guards, runtime traces, and risk budgets on a real downloaded baseline and report each signal's measured contribution to the ranking; see [docs/ablation-study.md](docs/ablation-study.md).

Run `make effect-size-strata-gate` to compare risk density and hazard-class severity across ecosystem, repository size, and migration framework with standardized Cohen's d effect sizes on real downloaded baselines; see [docs/effect-size-strata.md](docs/effect-size-strata.md).

Run `make otel-trace-gen-gate` to generate valid OpenTelemetry (OTLP) traces from a real baseline, linking each data-change span back to a real finding by id and table for offline observability replay; see [docs/otel-trace-gen.md](docs/otel-trace-gen.md).

Run `make datadog-timeline-gate` to reconstruct a Datadog-style incident timeline (deploy marker, APM spans, logs, monitor alert) around real findings with verified deploy<span<log<alert ordering and correlation coverage; see [docs/datadog-timeline.md](docs/datadog-timeline.md).

Run `make prom-grafana-gate` to generate and re-ingest a Prometheus range export and Grafana dashboard (SLO burn, error-rate, latency panels), correlating observed SLO breaches back to real high-severity findings; see [docs/prom-grafana.md](docs/prom-grafana.md).

Run `make runtime-confidence-gate` to score every real finding on independent static-risk and observed-runtime axes, separating confirmed incidents from unconfirmed static risk via confidence quadrants and a divergence metric; see [docs/runtime-confidence.md](docs/runtime-confidence.md).

Run `make incident-notebook-gate` to reconstruct a data-change failure hypothesis as a replayable, byte-identical notebook (load, select incident, gather evidence, temporal check, hypothesis, conclusion) from a real baseline; see [docs/incident-notebook.md](docs/incident-notebook.md).

Run `make causality-limits-gate` to enforce the causality limitations of trace-to-migration links — constructing clean, confounded, temporally-inconsistent, and cross-table scenarios so correlation is never overclaimed as proven causation (ceiling verdict: consistent-with); see [docs/causality-limits.md](docs/causality-limits.md).

Run `make runtime-redaction-gate` to prove the deterministic `[redacted:<kind>:<hash>]` token policy is stable on runtime evidence — trace attributes, log lines, metric labels, and incident text — with byte-identical reruns, deterministic tokens, and zero raw-value leaks; see [docs/runtime-redaction.md](docs/runtime-redaction.md).

Run `make offline-bundle-gate` to package real findings and runtime evidence into a self-contained, checksummed offline bundle a reviewer can verify air-gapped (`shasum -c MANIFEST.checks`), with no network endpoints and deterministic rebuilds; see [docs/offline-bundle.md](docs/offline-bundle.md).

Run `make incident-export-gate` to export real findings to PagerDuty, Opsgenie, Slack, and Statuspage in each provider's native schema, with severity mapped to provider vocabularies and stable cross-adapter finding linkage; see [docs/incident-export.md](docs/incident-export.md).

Run `make negative-controls-gate` to run paired positive/negative-control telemetry tests proving the runtime layer is no rubber stamp — silent telemetry never confirms a warning (even high-severity), giving specificity 1.0 while the positive arm retains power; see [docs/negative-controls.md](docs/negative-controls.md).

Run `make intervention-contracts-gate` to attach an explicit contract to every generated intervention — preconditions, postconditions, rollback assumptions, and remaining proof holes — built from real repair-proof summaries, where no contract claims a proven status; see [docs/intervention-contracts.md](docs/intervention-contracts.md).

Run `make diff-minimization-gate` to minimize a redundant intervention bundle (tests, guards, instrumentation, repair candidates) by set-cover over a finding's required evidence, proving the result still covers everything and is 1-minimal so no generated line is redundant; see [docs/diff-minimization.md](docs/diff-minimization.md).

Run `make quarantine-attestation-gate` to issue a quarantine attestation for every real repair candidate — explicit non-execution, manual-review requirement (escalating with open proof holes), and a stable fingerprint — so no generated repair is ever auto-applied; see [docs/quarantine-attestation.md](docs/quarantine-attestation.md).

Run `make intervention-provenance-graph-gate` to build a provenance graph over every generated intervention line — proving zero orphan (untraceable) lines because each line is bound to both the real risk it addresses and at least one source-evidence path, with a negative control that flags an injected untraceable line; see [docs/intervention-provenance-graph.md](docs/intervention-provenance-graph.md).

Run `make rejection-taxonomy-gate` to classify candidate data changes through a closed, deterministic rejection taxonomy — unsafe SQL, broad writes, missing rollback, and unbounded runtime — proving every category fires on real evidence while a synthetic safe candidate yields zero rejection codes; see [docs/rejection-taxonomy.md](docs/rejection-taxonomy.md).

Run `make generated-test-mutation-gate` to mutation-test the generated reviewability tests themselves — proving each one fails when its stated assumption (target table, declared scope) is violated, for a mutation score of 1.0, while a tautological control test kills zero mutants; see [docs/generated-test-mutation.md](docs/generated-test-mutation.md).

Run `make guard-effectiveness-gate` to simulate generated migration guards against synthetic before/after datasets derived from the real public schema — proving the guard allows only bounded-safe changes, blocks every broad change, and fails closed on missing or unknown table metadata, while a no-op control guard scores strictly lower; see [docs/guard-effectiveness.md](docs/guard-effectiveness.md).

Run `make intervention-budget-tuning-gate` to sweep the generated-intervention budget over files, lines, tokens, and changes — proving risk coverage rises monotonically with budget, zero budget covers nothing, full budget covers everything, and each dimension has a diminishing-returns knee that recommends a setting; see [docs/intervention-budget-tuning.md](docs/intervention-budget-tuning.md).

Run `make intervention-scorecard-gate` to emit a reviewer scorecard for every intervention that keeps usefulness, safety, completeness, and uncertainty as four separate axes — proving the axes are separable, all scores fall in [0,1], and no card overclaims completeness while proof holes remain open; see [docs/intervention-scorecard.md](docs/intervention-scorecard.md).

Run `make intervention-regression-archive-gate` to archive every generated intervention per release with its safety, completeness, and uncertainty scores — proving no release silently regresses an intervention (lower safety/completeness or higher uncertainty), with a negative control that detects an injected regression; see [docs/intervention-regression-archive.md](docs/intervention-regression-archive.md).

Run `make mercurial-fossil-source-gate` to ingest **Mercurial and Fossil** working trees as first-class sources — detecting the VCS, recording its revision as provenance, content-addressing the tree independently of VCS metadata, and proving a destructive migration survives ingestion; see [docs/mercurial-fossil-source.md](docs/mercurial-fossil-source.md).

Run `make monorepo-boundary-gate` to detect **package boundaries** in monorepos (Bazel, Pants, Nx, Turborepo, Maven, Gradle, Go workspaces) so risks attribute to the owning package — proven on a real Turborepo monorepo with a no-false-positive rule for incidental build files; see [docs/monorepo-boundary.md](docs/monorepo-boundary.md).

Run `make multi-ecosystem-migration-gate` to detect **Laravel, Ecto, Diesel, Sequelize, Knex, Doctrine, and Rails multi-db** migrations and their native commands — proven on the real laravel/laravel repo plus a seven-framework unit matrix; see [docs/multi-ecosystem-migration.md](docs/multi-ecosystem-migration.md).

Run `make nosql-change-gate` to detect destructive **NoSQL** changes across MongoDB, Cassandra, Elasticsearch, Redis, and DynamoDB — proven on a real Cassandra repo (DROP KEYSPACE/TABLE) plus a five-engine unit matrix with a no-false-positive rule; see [docs/nosql-change.md](docs/nosql-change.md).

Run `make data-pipeline-gate` to surface destructive **data-pipeline** changes in Airflow DAGs, dbt models, Spark jobs, and Kafka consumers — proven on a real lakehouse repo (Spark overwrites + dbt full-refresh + Airflow backfills) plus a four-framework unit matrix with a no-false-positive rule; see [docs/data-pipeline.md](docs/data-pipeline.md).

Run `make infra-ordering-gate` to run **infrastructure/data ordering** analysis across Helm hooks, Argo sync-waves, initContainers, and Terraform depends_on — classifying each migration/database job as sequenced or unordered; proven on the real helm/charts repo (Kong/Anchore sequenced migrations) plus a unit test; see [docs/infra-ordering.md](docs/infra-ordering.md).

Run `make schema-compat-gate` to flag **protobuf/Avro schema-compatibility** hazards (proto2 required fields, missing reserved tags, Avro fields without defaults) tied to data-change risk — proven on the real apache/avro repo plus a unit matrix with a no-false-positive rule; see [docs/schema-compat.md](docs/schema-compat.md).

Run `make fixture-minimizer-gate` to **delta-debug** any failing input down to a 1-minimal reproducing fixture across every ecosystem — using the analyzer as an oracle and proven by reducing a real Cassandra migration to its minimal destructive core; see [docs/fixture-minimizer.md](docs/fixture-minimizer.md).

Run `make parser-dashboard-gate` to publish an **ecosystem parser quality dashboard** unifying coverage, fuzz robustness, known gaps, and real-repo proofs — every ecosystem carries a real-repository proof and the analyzer survives a malformed-input fuzz corpus with zero crashes; see [docs/parser-dashboard.md](docs/parser-dashboard.md).

Run `make onboarding-quest-gate` to scaffold a new ecosystem gate in minutes — a generator emits valid, runnable starter files and a six-step **onboarding quest** takes a contributor from zero to a passing real-repo gate in under one hour; see [docs/onboarding-quest.md](docs/onboarding-quest.md).

Run `make examples-gallery-gate` to render a public **examples gallery** straight from `examples/*.json` and their **reproducibility** backing — every advertised capability is cross-checked against its gate and doc, so the gallery has zero orphan entries; see [docs/examples-gallery.md](docs/examples-gallery.md).

Run `make issue-to-artifact-gate` to turn accepted user submissions into **pinned public proof** entries — only pinned refs that map to a real gate are admitted, and unpinned or unknown-capability submissions are **rejected** by deterministic negative controls; see [docs/issue-to-artifact.md](docs/issue-to-artifact.md).

Run `make contributor-badges-gate` to render contributor **recognition** badges from **gate-backed** data alone — tiers are monotonic in verified contribution count and unbacked claims are dropped, so a badge never credits unproven work; see [docs/contributor-badges.md](docs/contributor-badges.md).

Run `make starter-issues-gate` to generate **good first issue** templates from structured **roadmap card** data — each issue names its failure mode, expected gate, artifact path, and acceptance criteria, so contributors never get a vague task; see [docs/starter-issues.md](docs/starter-issues.md).

Run `make governance-gate` to render role-specific **governance** charters for maintainers, security reviewers, research reviewers, and ecosystem owners — each with scope, responsibilities, an **escalation** path, and accountable gates; see [docs/governance.md](docs/governance.md).

Run `make release-notes-gate` to generate release notes that embed a **public proof** delta computed against the live gate set, contributor recognition, and a **known-limitations** ledger — so notes never advertise a capability whose gate is absent; see [docs/release-notes.md](docs/release-notes.md).

Run `make office-hours-gate` to assemble a public **office hours** agenda from recent **reproducibility** failures and roadmap cards — every failure item reproduces with `make <gate>`, so the room always works on real, reproducible issues; see [docs/office-hours.md](docs/office-hours.md).

Run `make feedback-forms-gate` to render **analytics-free** docs feedback forms that produce structured **local issue** templates — the gate enforces zero external URLs or trackers, so reader feedback never phones home; see [docs/feedback-forms.md](docs/feedback-forms.md).

Run `make conference-demos-gate` to generate audience-tailored **conference demo** run sheets for Datadog, Microsoft RISE, database, and PL audiences — every step is a real **gate-backed** `make` target, so what's green on main is green on stage; see [docs/conference-demos.md](docs/conference-demos.md).

Run `make adoption-case-studies-gate` to render **case studies** of teams using Patchline alongside CI, **observability**, and migration tooling — every cited capability maps to a real gate, so adoption stories are anchored to proof, not marketing; see [docs/adoption-case-studies.md](docs/adoption-case-studies.md).

Run `make incremental-cache-gate` to verify **incremental analysis caching** keyed by archive hash, subpath, **parser version**, and config — the gate proves a cold miss warms to a hit, the key is stable, and all four key components are load-bearing; see [docs/incremental-cache.md](docs/incremental-cache.md).

Run `make parallel-corpus-gate` to run a public corpus concurrently with **deterministic output ordering** and per-repo **failure isolation** — results collate by repo identity despite out-of-order completion, and one failing repo never aborts the rest; see [docs/parallel-corpus.md](docs/parallel-corpus.md).

Run `make resumable-gates-gate` to verify **resumable** gates — an **interrupt**ed corpus sweep preserves completed analyses, and a resume run recomputes none of them while finishing the remainder, so every repo is processed exactly once; see [docs/resumable-gates.md](docs/resumable-gates.md).

Run `make error-taxonomy-gate` to verify the structured **error taxonomy** — every failure across the six pipeline stages carries a unique code, a **retryable** flag, and a concrete remediation; see [docs/error-taxonomy.md](docs/error-taxonomy.md).

Run `make resource-budgets-gate` to enforce per-stage **resource budget**s — an analysis is admitted only when every stage is within its time, memory, and file budget, and an **over-budget** run is rejected at the offending stage and resource; see [docs/resource-budgets.md](docs/resource-budgets.md).

Run `make flaky-detect-gate` to catch **flaky** gates — each candidate is run repeatedly and any **nondeterministic** output hash is flagged, so only byte-reproducible gates are trusted as proof; see [docs/flaky-detect.md](docs/flaky-detect.md).

Run `make canonical-json-gate` to verify order-independent proof checksums — a **canonical** JSON form (sorted keys, compact) makes a reordered-but-equal document collide under **checksum** while a content change diverges; see [docs/canonical-json.md](docs/canonical-json.md).

Run `make shell-portability-gate` to lint gate scripts for **portability** hazards (mapfile, `/tmp` writes, GNU `sed -i`) — shipped scripts stay clean while a **negative-control** fixture is flagged for every hazard; see [docs/shell-portability.md](docs/shell-portability.md).

Run `make artifact-gc-gate` to prune the artifact cache by **LRU** under a fixed budget while never evicting a **pinned** entry — disk stays bounded but open-proof artifacts are never collected; see [docs/artifact-gc.md](docs/artifact-gc.md).

Run `make release-smoke-gate` to gate releases behind a minimal **smoke** suite — a **release-blocking** critical failure stops the release and is named, while advisory failures never block; see [docs/release-smoke.md](docs/release-smoke.md).

Run `make dataflow-summary-gate` to build a **dataflow summary** joining application writes to migration-touched columns — a write to a dropped/renamed column becomes a high-severity **impact edge** while added-column and unrelated writes are excluded; see [docs/dataflow-summary.md](docs/dataflow-summary.md).

Run `make query-shape-gate` to extract a normalized **query shape** from ORM, raw SQL, prepared-statement, and **query-builder** code — all four reduce to the same `(operation, table)` while a non-query comment yields no shape; see [docs/query-shape.md](docs/query-shape.md).

Run `make rollback-check-gate` to run **rollback** semantic checks — each migration step is classified reversible, data_lossy, **irreversible**, or partial from its up/down pair, so dangerous rollbacks are named before deploy; see [docs/rollback-check.md](docs/rollback-check.md).

Run `make lock-duration-gate` to estimate **lock-duration** from table size hints, op kind, dialect rules, and **concurrent**-index config — a blocking large-table index is predicted while a concurrent or metadata-only op collapses to short/instant; see [docs/lock-duration.md](docs/lock-duration.md).

Run `make tenant-risk-gate` to infer **tenant-boundary** and **sharding** risks in multi-tenant schemas — an unscoped backfill on a tenant table or a shard-key rewrite is flagged high while scoped and global-table ops stay low; see [docs/tenant-risk.md](docs/tenant-risk.md).

Run `make privacy-impact-gate` to infer **privacy impact** for deletes, exports, **anonymization**, and retention changes — PII exports flag high and PII deletes erasure-relevant while non-PII ops carry none; see [docs/privacy-impact.md](docs/privacy-impact.md).

Run `make uncertainty-calibration-gate` to check **calibration** of ranked-risk confidences via **expected calibration error** — a calibrated predictor passes while an overconfident one (right half the time at 0.9 confidence) is rejected; see [docs/uncertainty-calibration.md](docs/uncertainty-calibration.md).

Run `make proof-hole-graph-gate` to compute the **minimum-cost** evidence set that closes a **proof hole** — the smallest dependency-respecting set that lowers a risk's uncertainty below target, never selecting an irrelevant zero-reduction item; see [docs/proof-hole-graph.md](docs/proof-hole-graph.md).

Run `make pattern-mining-gate` to mine recurring migration **failure mode**s across repositories — a mode seen in many repos becomes a **recurring pattern** ranked by prevalence while a one-off incident is excluded; see [docs/pattern-mining.md](docs/pattern-mining.md).

Run `make explainable-ranking-gate` to produce an **explainable ranking** that decomposes each score into per-signal contributions — removing the dominant signal flips the ranking, proving the cited factor was **load-bearing**; see [docs/explainable-ranking.md](docs/explainable-ranking.md).

Run `make paper-build-gate` to run the **paper build** pipeline — a LaTeX capabilities table, figure, and appendix are generated from the live gate catalog and compiled with **pdflatex** into a PDF, so the paper can never drift from the implementation; see [docs/paper-build.md](docs/paper-build.md).

Run `make claim-freeze-gate` to freeze paper claims into a checksum manifest — a **claim freeze** re-verifies cited artifacts and flags any post-submission **drift**, so reviewers read exactly the evidence that was submitted; see [docs/claim-freeze.md](docs/claim-freeze.md).

Run `make reviewer-search-gate` to run a **reviewer-question search** over claims, limitations, artifacts, and roadmap cards — in-scope questions retrieve the backing typed entry while an **out-of-scope** query returns nothing; see [docs/reviewer-search.md](docs/reviewer-search.md).

Run `make artifact-mirror-gate` to publish a public **artifact mirror** of non-sensitive evidence — clean artifacts are mirrored with checksums while anything matching a secret marker is automatically held in **quarantine**; see [docs/artifact-mirror.md](docs/artifact-mirror.md).

Run `make rc-rehearsal-gate` to run a **release-candidate rehearsal** across capstone, reviewer-walkthrough, reproducibility, and docs stages — the candidate is **blessed** only when every stage passes and a single failure blocks it at the named stage; see [docs/rc-rehearsal.md](docs/rc-rehearsal.md).

Run `make leaderboard-gate` to build a **benchmark leaderboard** comparing releases over time on gates, determinism, and reproduction rate — releases are ranked and any release that **regress**ed against its predecessor is flagged; see [docs/leaderboard.md](docs/leaderboard.md).

Run `make case-bundle-gate` to assemble an archival **case-study bundle** of deep narrative studies plus dozens of **lightweight** worked examples — deep studies must carry a sufficiently detailed narrative and a shallow one is rejected; see [docs/case-bundle.md](docs/case-bundle.md).

Run `make eval-matrix-gate` to build a best-paper **evaluation matrix** mapping novelty, rigor, impact, reproducibility, and limitations to concrete backing artifacts — every criterion must resolve to a real file and an empty one is reported **unsupported**; see [docs/eval-matrix.md](docs/eval-matrix.md).

Run `make launch-kit-gate` to validate a star-growth **launch kit** of README hook, long-form post, social thread, demo script, and FAQ — every channel must be present and each social post within the platform **character limit**; see [docs/launch-kit.md](docs/launch-kit.md).

Run `make burndown-gate` to compute a **roadmap burndown** where every deliverable is **gate-backed** — a deliverable counts as done only when its gate script exists on disk and one pointing at a missing gate stays outstanding; see [docs/burndown.md](docs/burndown.md).

Run `make counterfactual-gate` to run a **counterfactual** repair eval — the safety verdict flips only when a **causally** relevant factor changes (removing a backfill) and stays put under an irrelevant edit (a comment change); see [docs/counterfactual.md](docs/counterfactual.md).

Run `make invariant-extract-gate` to perform formal **invariant extraction** from a schema — NOT NULL, unique, and foreign-key invariants are extracted and a migration that drops a `NOT NULL` guarantee is flagged as a violation; see [docs/invariant-extract.md](docs/invariant-extract.md).

Run `make symexec-gate` to run bounded **symbolic execution** of a migration guard — it explores every symbolic input, returns a **witness** for any reachable unsafe path, and confirms a hardened guard exposes none; see [docs/symexec.md](docs/symexec.md).

Run `make modelcheck-gate` to **model check** a migration rollout state machine — a safe model satisfies the *never reach data_loss* invariant while a buggy model yields the shortest **counterexample** trace; see [docs/modelcheck.md](docs/modelcheck.md).

Run `make causal-graph-gate` to build a **causal graph** of failure factors — it computes the true **root cause** ancestors of the failure, excludes a mere correlate with no path to failure, and rejects a cyclic graph; see [docs/causal-graph.md](docs/causal-graph.md).

Run `make active-learning-gate` to build an **active-learning queue** that prioritizes the most informative examples nearest the decision **boundary** for human labeling while excluding already-labeled ones; see [docs/active-learning.md](docs/active-learning.md).

Run `make risk-economics-gate` to make the ship-or-block call as **repair economics** — it weighs **expected** failure loss against blocking cost, blocking only when expected loss is greater; see [docs/risk-economics.md](docs/risk-economics.md).

Run `make reviewer-sim-gate` to simulate a multi-agent **reviewer panel** — strict/balanced/lenient agents vote and the same panel yields different outcomes under majority versus **veto** aggregation; see [docs/reviewer-sim.md](docs/reviewer-sim.md).

Run `make interop-gate` to prove **cross-tool interop** — Patchline findings export to a **SARIF**-style document and round-trip back field-for-field, while a malformed interchange document is rejected; see [docs/interop.md](docs/interop.md).

Run `make dossier-gate` to certify the 1.0 **release-readiness dossier** — it audits the six-artifact **evidence chain** (example, worker, gate, doc, Makefile target, README mention) for every sampled capability and rejects any incomplete one; see [docs/dossier.md](docs/dossier.md).

Run `make taint-tracker-gate` for **inter-procedural taint tracking** — it propagates untrusted request input across call boundaries to migration-affected columns, proves a multi-hop tainted path reaches a constrained column, and shows a sanitizer node cuts the flow; see [docs/taint-tracker.md](docs/taint-tracker.md).

Run `make orm-normalizer-gate` to map **Django, Rails, SQLAlchemy, and Prisma** migrations to one **canonical IR** — equivalent operations across dialects normalize identically while an unknown dialect is rejected; see [docs/orm-normalizer.md](docs/orm-normalizer.md).

Run `make schema-diff-gate` for a **typed, minimal, reversible** schema-diff engine — it emits an edit script taking snapshot A to B whose inverse restores A exactly, and rejects a non-minimal script with redundant ops; see [docs/schema-diff.md](docs/schema-diff.md).

Run `make constraint-solver-gate` to discharge generated **NOT NULL / FK** obligations against sample rows — a satisfiable obligation is proven and an unsatisfiable one returns a concrete **counterexample** row; see [docs/constraint-solver.md](docs/constraint-solver.md).

Run `make column-lineage-gate` to build a **column-lineage graph** tracing each column to the reads and writes that depend on it — a live column lists its consumers while an unreferenced column has none; see [docs/column-lineage.md](docs/column-lineage.md).

Run `make backfill-completeness-gate` to prove a **backfill covers every pre-existing row** before a NOT NULL flip — a complete backfill is certified and a backfill missing one row is rejected with the uncovered ids; see [docs/backfill-completeness.md](docs/backfill-completeness.md).

Run `make index-coverage-gate` to flag a migration that **drops an index a hot query still needs** — dropping a covering index used by a hot query is blocked while dropping an unused index is allowed; see [docs/index-coverage.md](docs/index-coverage.md).

Run `make transaction-boundary-gate` to prove each DDL/DML step is **atomic or explicitly compensated** — a fully wrapped plan passes and a step left outside any transaction with no compensation is flagged; see [docs/transaction-boundary.md](docs/transaction-boundary.md).

Run `make type-narrowing-gate` for a **type-narrowing safety checker** — widening conversions are allowed automatically while a narrowing change without a backing proof is rejected; see [docs/type-narrowing.md](docs/type-narrowing.md).

Run `make dead-column-gate` to prove **no live code reads a column** before it may be dropped — a column with zero live reads is safe to drop while a still-read column is retained; see [docs/dead-column.md](docs/dead-column.md).

Run `make corpus-harness-gate` for a **1000-repository** corpus harness with **deterministic sharding** and **resumable** sweeps — the same repository always lands in the same shard across runs and resuming from a checkpoint skips already-analyzed repositories; see [docs/corpus-harness.md](docs/corpus-harness.md).

Run `make language-extractors-gate` to prove **per-language extractors** (Python, Ruby, Go, TypeScript, Java) emit findings against **one shared schema** — every language's findings validate while a finding missing a required field is rejected; see [docs/language-extractors.md](docs/language-extractors.md).

Run `make streaming-analyzer-gate` for a **streaming analyzer** that bounds memory on repositories larger than RAM — its windowed pass never retains more than the bound yet returns the same aggregate as a buffer-everything pass; see [docs/streaming-analyzer.md](docs/streaming-analyzer.md).

Run `make work-queue-gate` for a **distributed work queue** that scales the sweep across workers reproducibly — task assignment is deterministic, complete, and non-overlapping while a duplicated assignment is detected; see [docs/work-queue.md](docs/work-queue.md).

Run `make incremental-reanalysis-gate` to reprocess **only repositories whose inputs changed** — the incremental set matches exactly the changed repositories and is strictly smaller than a full re-run; see [docs/incremental-reanalysis.md](docs/incremental-reanalysis.md).

Run `make gold-subset-gate` to build a **gold-label subset** with adjudicated ground truth — items where two adjudicators agree form the gold set while a disagreed item is excluded; see [docs/gold-subset.md](docs/gold-subset.md).

Run `make confusion-matrix-gate` for a **confusion-matrix** report with per-category **precision, recall, and F1** over the gold subset, validated against a hand-computed expectation; see [docs/confusion-matrix.md](docs/confusion-matrix.md).

Run `make stratified-sampler-gate` for a **stratified sampler** guaranteeing every stratum — including rare migration patterns — is represented, where a naive uniform sample would miss the rare one; see [docs/stratified-sampler.md](docs/stratified-sampler.md).

Run `make drift-monitor-gate` for a **corpus drift monitor** computing total-variation distance between input distributions — an identical distribution shows no drift while a shifted one trips the threshold; see [docs/drift-monitor.md](docs/drift-monitor.md).

Run `make benchmark-release-gate` to publish a **versioned benchmark** with a **frozen split** and a leaderboard submission format — the split checksum is stable and train/test are disjoint, a conforming submission is accepted, and a malformed one is rejected; see [docs/benchmark-release.md](docs/benchmark-release.md).

Run `make linter-baseline-compare-gate` to compare Patchline against **existing migration linters** on matched inputs — recall is computed per tool on the same cases, Patchline's recall dominates the baselines, and a baseline run on mismatched inputs is rejected; see [docs/linter-baseline-compare.md](docs/linter-baseline-compare.md).

Run `make mcnemar-significance-gate` for **statistical significance** testing — a **McNemar** statistic plus a percentile **bootstrap CI** declare a real improvement significant while two identical systems are not; see [docs/mcnemar-significance.md](docs/mcnemar-significance.md).

Run `make stage-ablation-gate` for an **ablation** suite measuring each analysis stage's **marginal contribution** — removing a load-bearing stage drops accuracy while a redundant stage contributes nothing; see [docs/stage-ablation.md](docs/stage-ablation.md).

Run `make human-timing-study-gate` for a **human-study protocol** comparing reviewer time with and without Patchline findings — the counterbalanced design is validated and the with-findings condition is faster, while an unbalanced protocol is rejected; see [docs/human-timing-study.md](docs/human-timing-study.md).

Run `make error-cost-model-gate` for a **cost-of-error** model weighting false negatives by historical incident severity — the configuration that misses a critical hazard ranks worse by expected cost even with fewer raw misses; see [docs/error-cost-model.md](docs/error-cost-model.md).

Run `make reliability-calibration-gate` for a **calibration** study with reliability bins — a well-calibrated model's expected calibration error stays under threshold while an overconfident model exceeds it; see [docs/reliability-calibration.md](docs/reliability-calibration.md).

Run `make framework-holdout-gate` for a **generalization** test holding out an entire framework — the threshold is selected only on training frameworks, evaluated on the unseen one, and any leakage of the held-out framework into training is detected; see [docs/framework-holdout.md](docs/framework-holdout.md).

Run `make perturbation-robustness-gate` for a **robustness** suite — semantic-preserving perturbations (renames, reformatting, comments) leave the verdict stable while a semantic change (removing the backfill) correctly flips it; see [docs/perturbation-robustness.md](docs/perturbation-robustness.md).

Run `make historical-replay-study-gate` for a **longitudinal** replay of historical migrations against known post-merge incidents — Patchline flags every incident-causing migration and clears the ones that shipped safely; see [docs/historical-replay-study.md](docs/historical-replay-study.md).

Run `make external-replication-kit-gate` for an **external-replication** kit — every paper number maps to a one-command recomputation whose value matches the manifest, and a tampered expected value is caught; see [docs/external-replication-kit.md](docs/external-replication-kit.md).
Run `make signed-provenance-chain-gate` for an end-to-end signed **provenance** chain from input commit to printed verdict, where a broken digest link is rejected; see [docs/signed-provenance-chain.md](docs/signed-provenance-chain.md).
Run `make reproducible-build-attestation-gate` for a deterministic build **attestation** proving two pinned builds are byte-identical while a nondeterministic build is flagged; see [docs/reproducible-build-attestation.md](docs/reproducible-build-attestation.md).
Run `make merkle-audit-log-gate` for a **tamper-evident** Merkle-chained audit log over every gate run, where an edited past entry is detected; see [docs/merkle-audit-log.md](docs/merkle-audit-log.md).
Run `make secret-leak-scanner-gate` for a **zero-tolerance** secret-leak scan over all generated artifacts, where a seeded API key is caught; see [docs/secret-leak-scanner.md](docs/secret-leak-scanner.md).
Run `make sbom-pinned-deps-gate` for a **supply-chain** SBOM with pinned, hash-verified dependencies where a compromised installed hash is flagged; see [docs/sbom-pinned-deps.md](docs/sbom-pinned-deps.md).
Run `make differential-privacy-stats-gate` to share aggregate corpus stats under **differential privacy**, where the Laplace noise scale matches sensitivity/epsilon and a zero-epsilon request is rejected; see [docs/differential-privacy-stats.md](docs/differential-privacy-stats.md).
Run `make red-team-adversarial-gate` for a red-team suite of **adversarial** migrations crafted to evade analysis, asserting zero successful evasions and a clean benign control; see [docs/red-team-adversarial.md](docs/red-team-adversarial.md).
Run `make fuzzing-harness-gate` for a **fuzzing** harness that mutates migrations and asserts no crash and no unsound pass, with a planted unsound pass detected; see [docs/fuzzing-harness.md](docs/fuzzing-harness.md).
Run `make soundness-boundary-gate` for an explicit **soundness boundary** where every guaranteed hazard class is backed by a gate and an unbacked guarantee is rejected; see [docs/soundness-boundary.md](docs/soundness-boundary.md).
Run `make security-threat-model-gate` for a security **threat model** where every threat has a present mitigation and an unmitigated threat is flagged; see [docs/security-threat-model.md](docs/security-threat-model.md).
Run `make quickstart-sixty-seconds-gate` for a single-command quickstart that analyzes a real repo in under **sixty seconds**, where an over-budget run is flagged; see [docs/quickstart-sixty-seconds.md](docs/quickstart-sixty-seconds.md).
Run `make inline-review-surface-gate` to render findings **inline** on the review surface with one-click reproduction, rejecting any finding missing its line anchor; see [docs/inline-review-surface.md](docs/inline-review-surface.md).
Run `make minimal-repro-generator-gate` to reduce any finding to a **minimal reproduction** that preserves the verdict, rejecting an over-reduction that drops the hazard; see [docs/minimal-repro-generator.md](docs/minimal-repro-generator.md).
Run `make fix-suggestion-engine-gate` to propose a **safe migration variant** for each hazard that clears it on re-analysis, rejecting a bogus fix that does not; see [docs/fix-suggestion-engine.md](docs/fix-suggestion-engine.md).
Run `make evidence-trace-view-gate` for an interactive view tracing every verdict to its **supporting evidence** down to source spans, rejecting a dangling ungrounded node; see [docs/evidence-trace-view.md](docs/evidence-trace-view.md).
Run `make ci-pr-bot-gate` for a CI bot that posts gate-backed PR verdicts with stable, **idempotent** output, updating in place on a changed diff; see [docs/ci-pr-bot.md](docs/ci-pr-bot.md).
Run `make triage-prioritizer-gate` to deduplicate and **prioritize** findings by severity-times-confidence so the highest-impact item is first; see [docs/triage-prioritizer.md](docs/triage-prioritizer.md).
Run `make config-profiles-gate` for **strict/balanced/lenient** profiles with a documented, monotonic recall-precision trade-off, rejecting a misconfigured profile; see [docs/config-profiles.md](docs/config-profiles.md).
Run `make regression-snapshot-gate` for a snapshot mode that fails CI only on **newly introduced** hazards, never on pre-existing debt; see [docs/regression-snapshot.md](docs/regression-snapshot.md).
Run `make a11y-i18n-output-gate` for an **accessibility** and i18n pass ensuring every message is text-marked and localizable, rejecting color-only output; see [docs/a11y-i18n-output.md](docs/a11y-i18n-output.md).
Run `make contributor-onboarding-gate` for a **one script** onboarding that builds, tests, and runs a first analysis, rejecting a plan missing a stage; see [docs/contributor-onboarding.md](docs/contributor-onboarding.md).
Run `make good-first-issue-gen-gate` to seed **good first issue**s from real gate-catalog gaps, rejecting a fabricated issue with no backing gap; see [docs/good-first-issue-gen.md](docs/good-first-issue-gen.md).
Run `make office-hours-rotation-gate` for a documented office-hours triage **rotation** with full coverage and no conflicts, rejecting an unstaffed schedule; see [docs/office-hours-rotation.md](docs/office-hours-rotation.md).
Run `make plugin-conformance-gate` for a plugin API **conformance** suite where a compliant plugin passes and one missing a contract method fails; see [docs/plugin-conformance.md](docs/plugin-conformance.md).
Run `make showcase-gallery-gate` for a gallery of protected repos, each with **reproducible evidence**, rejecting an entry with no reproduce command; see [docs/showcase-gallery.md](docs/showcase-gallery.md).
Run `make quarterly-benchmark-report-gate` for a quarterly report auto-generated from the **leaderboard**, proving the series is non-regressing and flagging a regression quarter; see [docs/quarterly-benchmark-report.md](docs/quarterly-benchmark-report.md).
Run `make governance-policy-gate` for a governance policy with semver and a **deprecation** window, rejecting a breaking change shipped under the minimum window; see [docs/governance-policy.md](docs/governance-policy.md).
Run `make citation-doi-gate` for a citation file and archival **DOI** with complete bibliographic fields, rejecting a malformed DOI; see [docs/citation-doi.md](docs/citation-doi.md).
Run `make sustainability-plan-gate` for a sustainability plan checking CI cost, maintainer load, and **bus-factor**, flagging a single-maintainer project; see [docs/sustainability-plan.md](docs/sustainability-plan.md).
Run `make roadmap-burndown-gate` for a 1.0-to-2.0 **milestone** burndown where every open milestone is gate-backed and an evidence-free completion is rejected; see [docs/roadmap-burndown.md](docs/roadmap-burndown.md).
Run `make learned-risk-model-gate` for a learned risk model evaluated on a **held-out** split that beats the majority baseline; see [docs/learned-risk-model.md](docs/learned-risk-model.md).
Run `make neuro-symbolic-verdict-gate` for neuro-symbolic verdicts where deterministic gates act as hard **constraint**s that override a confidently-wrong learned prior; see [docs/neuro-symbolic-verdict.md](docs/neuro-symbolic-verdict.md).
Run `make backfill-synthesis-gate` to synthesize a safe backfill from a declarative **invariant** spec and verify it establishes the invariant while a no-op fails; see [docs/backfill-synthesis.md](docs/backfill-synthesis.md).
Run `make llm-judge-harness-gate` for an LLM-judge harness with a deterministic rubric and **inter-rater** agreement, flagging a chance-level judge pair; see [docs/llm-judge-harness.md](docs/llm-judge-harness.md).
Run `make invariant-inference-gate` to infer invariants over fixtures and emit a **proof obligation** per survivor, discarding any with a counterexample; see [docs/invariant-inference.md](docs/invariant-inference.md).
Run `make differential-semantics-gate` to differentially test the analyzer against a **reference semantics** for a migration DSL, detecting a seeded divergence; see [docs/differential-semantics.md](docs/differential-semantics.md).
Run `make incident-forecaster-gate` for an incident-risk forecaster evaluated with a proper **scoring rule** that beats an uninformative baseline; see [docs/incident-forecaster.md](docs/incident-forecaster.md).
Run `make counterfactual-explanation-gate` for minimal **counterfactual** explanations that flip a hazard to safe, rejecting a non-flipping change; see [docs/counterfactual-explanation.md](docs/counterfactual-explanation.md).
Run `make transfer-learning-study-gate` for a **zero-shot** cross-ecosystem transfer study with disjoint train/test, rejecting an overlapping split; see [docs/transfer-learning-study.md](docs/transfer-learning-study.md).
Run `make causal-effect-estimate-gate` to estimate Patchline's effect on incident rates with **confounder** control, flagging a naive unadjusted estimate; see [docs/causal-effect-estimate.md](docs/causal-effect-estimate.md).
Run `make theorem-prover-backend-gate` for a theorem-proving backend that emits a machine-checkable **proof** per obligation and reports an unprovable one as unproved; see [docs/theorem-prover-backend.md](docs/theorem-prover-backend.md).
Run `make rl-reviewer-gate` for a learned triage order that lowers **reviewer cost** below a random ordering, flagging a degenerate policy; see [docs/rl-reviewer.md](docs/rl-reviewer.md).
Run `make multimodal-finding-gate` for **multimodal** findings (diagram + text + code) checked for cross-modal consistency, flagging disagreement; see [docs/multimodal-finding.md](docs/multimodal-finding.md).
Run `make abstention-policy-gate` for an uncertainty-aware **abstention** policy with a guaranteed selective-accuracy floor, shown to fail under forced full coverage; see [docs/abstention-policy.md](docs/abstention-policy.md).
Run `make self-improving-loop-gate` for a loop that mines candidate gates from **unexplained** corpus failures, rejecting a proposal with no backing failure; see [docs/self-improving-loop.md](docs/self-improving-loop.md).
Run `make repro-appendix-gate` for a reproducibility appendix mapping every paper claim to a **one-command** gate, rejecting a claim with no command; see [docs/repro-appendix.md](docs/repro-appendix.md).
Run `make hermetic-artifact-container-gate` for a hermetic container passing the ACM **Artifacts-Available**/Reusable checklist, rejecting one needing network access; see [docs/hermetic-artifact-container.md](docs/hermetic-artifact-container.md).
Run `make results-regeneration-gate` to regenerate every figure and table **deterministically** from raw data, flagging a nondeterministic artifact; see [docs/results-regeneration.md](docs/results-regeneration.md).
Run `make anonymized-build-gate` for an **anonymized**-for-review build that strips identifying metadata, with the un-anonymized control detected as leaking; see [docs/anonymized-build.md](docs/anonymized-build.md).
Run `make threats-to-validity-gate` for a **threats to validity** section where every threat is backed by a robustness or ablation experiment; see [docs/threats-to-validity.md](docs/threats-to-validity.md).
Run `make related-work-table-gate` for a related-work table generated from the **baseline harness** numbers where Patchline leads, rejecting an unmeasured row; see [docs/related-work-table.md](docs/related-work-table.md).
Run `make limitations-gate-gate` to ensure every claimed **limitation** has a backing experiment or example, rejecting a speculative one; see [docs/limitations-gate.md](docs/limitations-gate.md).
Run `make reviewer-reproduction-guide-gate` for a one-page reviewer guide regenerating the headline result **in minutes**, rejecting an over-length guide; see [docs/reviewer-reproduction-guide.md](docs/reviewer-reproduction-guide.md).
Run `make dataset-datasheet-gate` for a dataset **datasheet** documenting collection, consent, licensing, and biases, rejecting an incomplete one; see [docs/dataset-datasheet.md](docs/dataset-datasheet.md).
Run `make model-card-gate` for a model card documenting intended use and **failure mode**s for the learned component, rejecting an incomplete card; see [docs/model-card.md](docs/model-card.md).
Run `make camera-ready-build-gate` for a camera-ready PDF pipeline with **pinned tooling**, rejecting a build on a floating tool version; see [docs/camera-ready-build.md](docs/camera-ready-build.md).
Run `make demo-video-script-gate` for a video script of the **end-to-end workflow** where every beat maps to a runnable command, rejecting an uncovered beat; see [docs/demo-video-script.md](docs/demo-video-script.md).
Run `make artifact-badge-audit-gate` to self-audit each artifact **badge** criterion against evidence, rejecting an unearned badge; see [docs/artifact-badge-audit.md](docs/artifact-badge-audit.md).
Run `make evaluation-preregistration-gate` for a public **pre-registration** of the evaluation protocol, detecting a post-hoc altered protocol; see [docs/evaluation-preregistration.md](docs/evaluation-preregistration.md).
Run `make rebuttal-evidence-pack-gate` for a rebuttal pack pairing each anticipated reviewer question with a **reproducible answer** command, rejecting an unanswered one; see [docs/rebuttal-evidence-pack.md](docs/rebuttal-evidence-pack.md).
Run `make ci-integrations-marketplace-gate` for a marketplace listing with a **verified**, reproducible setup per CI system, rejecting an unverified listing; see [docs/ci-integrations-marketplace.md](docs/ci-integrations-marketplace.md).
Run `make five-minute-landing-gate` for a five-minute landing flow whose **completion rate** clears a threshold within budget, flagging a high-friction flow; see [docs/five-minute-landing.md](docs/five-minute-landing.md).
Run `make localized-quickstarts-gate` for **localized** quickstarts parity-checked against the canonical steps, flagging an incomplete locale; see [docs/localized-quickstarts.md](docs/localized-quickstarts.md).
Run `make incident-prevention-scoreboard-gate` for a public scoreboard of **anonymized** adopter outcomes with consistent totals, flagging an identity-leaking entry; see [docs/incident-prevention-scoreboard.md](docs/incident-prevention-scoreboard.md).
Run `make conference-talk-kit-gate` for a talk kit whose every **live demo** is gate-backed, rejecting an unbacked segment; see [docs/conference-talk-kit.md](docs/conference-talk-kit.md).
Run `make partner-case-study-gate` for a partner case-study program with **signed**, reproducible result bundles, rejecting an unsigned bundle; see [docs/partner-case-study.md](docs/partner-case-study.md).
Run `make ecosystem-certification-gate` for an extension certification mark backed by an automated **conformance** gate, denying a non-conforming extension; see [docs/ecosystem-certification.md](docs/ecosystem-certification.md).
Run `make reproducibility-vault-gate` for a vault that **snapshot**s toolchain, corpus, and results per release, rejecting an incomplete snapshot; see [docs/reproducibility-vault.md](docs/reproducibility-vault.md).
Run `make community-impact-report-gate` for an impact report tying stars, adopters, and prevented incidents to **gate-backed evidence**, rejecting an unbacked metric; see [docs/community-impact-report.md](docs/community-impact-report.md).
Run `make vision-dossier-gate` for a 2.0 vision dossier proving sustained **novelty**, rigor, adoption, and reproducibility, each gate-backed, rejecting an incomplete dossier; see [docs/vision-dossier.md](docs/vision-dossier.md).
Run `make mechanized-operational-semantics-gate` for a mechanized **operational semantics** of the migration DSL where every reduction rule carries a checked-in proof, rejecting an unproven rule; see [docs/mechanized-operational-semantics.md](docs/mechanized-operational-semantics.md).
Run `make soundness-theorem-gate` for a **soundness** theorem proving every safe verdict implies the absence of the modeled hazard, rejecting an unproven class; see [docs/soundness-theorem.md](docs/soundness-theorem.md).
Run `make completeness-characterization-gate` for a **completeness** characterization stating exactly which hazards are provably caught versus best-effort, rejecting an uncharacterized one; see [docs/completeness-characterization.md](docs/completeness-characterization.md).
Run `make decision-procedure-complexity-gate` for a decision-procedure **complexity** analysis with empirical runtime confirming the predicted bound, rejecting a super-bound regression; see [docs/decision-procedure-complexity.md](docs/decision-procedure-complexity.md).
Run `make refinement-types-columns-gate` for a **refinement type** encoding of column invariants checked against extracted fixtures, rejecting an unchecked refinement; see [docs/refinement-types-columns.md](docs/refinement-types-columns.md).
Run `make bisimulation-equivalence-gate` for a **bisimulation** argument that the analyzer and reference semantics agree on all DSL programs, detecting a seeded divergence; see [docs/bisimulation-equivalence.md](docs/bisimulation-equivalence.md).
Run `make certificate-composition-gate` to prove gate certificates **compose** without contradiction, rejecting a contradictory pair; see [docs/certificate-composition.md](docs/certificate-composition.md).
Run `make cutover-protocol-model-gate` for a formal backfill/cutover model with a proven **safety invariant**, rejecting an invariant-violating state; see [docs/cutover-protocol-model.md](docs/cutover-protocol-model.md).
Run `make cegar-termination-gate` for a counterexample-guided abstraction-refinement loop with a **termination** proof, rejecting a non-terminating run; see [docs/cegar-termination.md](docs/cegar-termination.md).
Run `make shell-go-equivalence-gate` for a machine-checked **equivalence** between the shell gate logic and its Go reimplementation, detecting a seeded mismatch; see [docs/shell-go-equivalence.md](docs/shell-go-equivalence.md).
Run `make million-migration-harness-gate` for a million-migration harness with **sharded**, resumable analysis under a cost budget, rejecting an over-budget shard; see [docs/million-migration-harness.md](docs/million-migration-harness.md).
Run `make cross-repo-dependency-analysis-gate` for **cross-repository** dependency analysis detecting hazards spanning services, rejecting an evidence-free detection; see [docs/cross-repo-dependency-analysis.md](docs/cross-repo-dependency-analysis.md).
Run `make work-stealing-scheduler-gate` for a distributed **work-stealing** scheduler with proven no-lost-task and no-duplicate guarantees, rejecting a lost task; see [docs/work-stealing-scheduler.md](docs/work-stealing-scheduler.md).
Run `make incremental-hazard-index-gate` for an incremental index enabling **sub-second** hazard queries at scale, rejecting an over-budget query; see [docs/incremental-hazard-index.md](docs/incremental-hazard-index.md).
Run `make git-history-streaming-gate` for a streaming-from-**git history** mode that analyzes every historical migration, rejecting a skipped commit; see [docs/git-history-streaming.md](docs/git-history-streaming.md).
Run `make multi-engine-matrix-gate` for a multi-engine matrix with **per-engine** semantics across Postgres/MySQL/SQLite/SQL Server, rejecting an undefined engine; see [docs/multi-engine-matrix.md](docs/multi-engine-matrix.md).
Run `make polyglot-orm-frontend-gate` for a polyglot ORM front-end covering eight-plus **framework**s on a shared core, rejecting a framework with no extractor; see [docs/polyglot-orm-frontend.md](docs/polyglot-orm-frontend.md).
Run `make prevalence-estimator-gate` for a sampling-theory estimator of hazard prevalence with **confidence** bounds, rejecting an out-of-interval estimate; see [docs/prevalence-estimator.md](docs/prevalence-estimator.md).
Run `make corpus-refresh-pipeline-gate` for a weekly corpus-refresh pipeline with **drift** alerts, rejecting a cycle that skips drift detection; see [docs/corpus-refresh-pipeline.md](docs/corpus-refresh-pipeline.md).
Run `make hardware-cost-model-gate` for a hardware-cost model reporting analysis throughput **per dollar** across machine classes, rejecting a zero-throughput class; see [docs/hardware-cost-model.md](docs/hardware-cost-model.md).
Run `make field-study-preregistration-gate` for a **pre-registered**, powered field study with a control group, rejecting an unregistered arm; see [docs/field-study-preregistration.md](docs/field-study-preregistration.md).
Run `make multi-rater-ground-truth-gate` for a multi-rater labeling protocol reporting **Krippendorff**'s alpha, rejecting a low-agreement batch; see [docs/multi-rater-ground-truth.md](docs/multi-rater-ground-truth.md).
Run `make tool-comparison-frozen-gate` for a comparison against published tools on a shared, **frozen** benchmark, rejecting an unmeasured competitor; see [docs/tool-comparison-frozen.md](docs/tool-comparison-frozen.md).
Run `make longitudinal-ab-deployment-gate` for a longitudinal A/B measuring **incident-rate** deltas with sequential analysis, rejecting an underperforming period; see [docs/longitudinal-ab-deployment.md](docs/longitudinal-ab-deployment.md).
Run `make cost-benefit-analysis-gate` for a **cost-benefit** analysis monetizing prevented incidents against reviewer time, rejecting a net-negative scenario; see [docs/cost-benefit-analysis.md](docs/cost-benefit-analysis.md).
Run `make survival-analysis-gate` for a **survival** analysis of time-to-incident with and without gating, rejecting a cohort where gating shortens it; see [docs/survival-analysis.md](docs/survival-analysis.md).
Run `make external-auditor-repro-gate` for an external-auditor reproduction with a signed **attestation**, rejecting an unsigned reproduction; see [docs/external-auditor-repro.md](docs/external-auditor-repro.md).
Run `make negative-results-section-gate` for a **negative-results** section where each non-benefit is backed by an experiment, rejecting an unsupported claim; see [docs/negative-results-section.md](docs/negative-results-section.md).
Run `make generalization-study-gate` for a generalization study across five disjoint ecosystems with **held-out** evaluation, rejecting an overlapping split; see [docs/generalization-study.md](docs/generalization-study.md).
Run `make adversarial-robustness-gate` for a robustness evaluation against an automated **adversary** searching for evasions, rejecting a successful evasion; see [docs/adversarial-robustness.md](docs/adversarial-robustness.md).
Run `make calibration-over-time-gate` for a **calibration**-over-time study under distribution shift, rejecting a miscalibrated window; see [docs/calibration-over-time.md](docs/calibration-over-time.md).
Run `make human-factors-study-gate` for a human-factors study of reviewer trust and **over-reliance** with mitigations, rejecting an unmitigated risk; see [docs/human-factors-study.md](docs/human-factors-study.md).
Run `make replication-package-ci-gate` for a replication package re-run on three different **CI provider**s, rejecting a divergent provider; see [docs/replication-package-ci.md](docs/replication-package-ci.md).
Run `make meta-analysis-pooled-gate` for a meta-analysis combining studies into one **pooled** effect with heterogeneity reporting, rejecting a null-effect study; see [docs/meta-analysis-pooled.md](docs/meta-analysis-pooled.md).
Run `make data-availability-statement-gate` for a data-availability statement with **DOI**-pinned raw data per figure, rejecting a figure with no DOI; see [docs/data-availability-statement.md](docs/data-availability-statement.md).
Run `make saas-reference-deployment-gate` for a hosted reference deployment with published **SLO**s and a status page, rejecting a breached SLO; see [docs/saas-reference-deployment.md](docs/saas-reference-deployment.md).
Run `make github-app-install-gate` for a one-click GitHub App install with **least-privilege** scopes and an audit trail, rejecting an over-broad scope; see [docs/github-app-install.md](docs/github-app-install.md).
Run `make policy-as-code-layer-gate` for a **policy-as-code** layer mapping org rules to gate configurations, rejecting an unmapped rule; see [docs/policy-as-code-layer.md](docs/policy-as-code-layer.md).
Run `make findings-to-ticket-bridge-gate` for a findings-to-ticket bridge with **idempotent** sync, rejecting a duplicate-creating integration; see [docs/findings-to-ticket-bridge.md](docs/findings-to-ticket-bridge.md).
Run `make realtime-pr-checks-gate` for real-time PR checks with sub-ten-second **incremental** verdicts, rejecting an over-budget check; see [docs/realtime-pr-checks.md](docs/realtime-pr-checks.md).
Run `make self-serve-onboarding-gate` for self-serve onboarding measured by activation and week-four **retention**, flagging a churned cohort; see [docs/self-serve-onboarding.md](docs/self-serve-onboarding.md).
Run `make upgrade-safety-advisor-gate` for an upgrade-safety advisor pairing risky changes with a **guided fix**, rejecting a risky change with no fix; see [docs/upgrade-safety-advisor.md](docs/upgrade-safety-advisor.md).
Run `make multi-tenant-isolation-gate` for a multi-tenant **isolation** model with a no-cross-tenant-leak property, rejecting a leaking tenant; see [docs/multi-tenant-isolation.md](docs/multi-tenant-isolation.md).
Run `make usage-metering-gate` for usage metering with reproducible **invoice**s from event logs, rejecting a non-reproducible invoice; see [docs/usage-metering.md](docs/usage-metering.md).
Run `make airgapped-distribution-gate` for an **air-gapped** distribution preserving every gate guarantee offline, rejecting a network-requiring gate; see [docs/airgapped-distribution.md](docs/airgapped-distribution.md).
Run `make customer-managed-keys-gate` for customer-managed keys on all artifacts with **rotation**, rejecting an unrotated artifact; see [docs/customer-managed-keys.md](docs/customer-managed-keys.md).
Run `make admin-analytics-dashboard-gate` for an admin dashboard tying findings to **prevented-incident** estimates, rejecting an unbacked metric; see [docs/admin-analytics-dashboard.md](docs/admin-analytics-dashboard.md).
Run `make soc2-controls-map-gate` for a SOC2-style controls map where each **control** is automated, rejecting a manually-only control; see [docs/soc2-controls-map.md](docs/soc2-controls-map.md).
Run `make break-glass-override-gate` for a **break-glass** migration-freeze workflow with full override provenance, rejecting an unlogged override; see [docs/break-glass-override.md](docs/break-glass-override.md).
Run `make reproducibility-portal-gate` for a customer-facing portal exposing every verdict's **evidence chain**, rejecting a chain-less verdict; see [docs/reproducibility-portal.md](docs/reproducibility-portal.md).
Run `make learned-program-repair-gate` for a learned program-repair model that proposes and **verifies** safe migrations, rejecting an unverified proposal; see [docs/learned-program-repair.md](docs/learned-program-repair.md).
Run `make active-learning-loop-gate` for an active-learning loop querying reviewers only on maximally-**informative** cases, rejecting a low-information query; see [docs/active-learning-loop.md](docs/active-learning-loop.md).
Run `make fm-assisted-extractor-gate` for a foundation-model extractor with **deterministic** verification of every extraction, rejecting an unverified one; see [docs/fm-assisted-extractor.md](docs/fm-assisted-extractor.md).
Run `make causal-discovery-module-gate` for a **causal**-discovery module inferring incident-causing patterns, rejecting a spurious correlation; see [docs/causal-discovery-module.md](docs/causal-discovery-module.md).
Run `make backfill-formal-synthesis-gate` for a formal-synthesis engine generating **provably-correct** backfills from invariants, rejecting a no-op backfill; see [docs/backfill-formal-synthesis.md](docs/backfill-formal-synthesis.md).
Run `make multi-agent-debate-gate` for a multi-agent debate harness with a proven **tie-break** rule, rejecting an unresolved case; see [docs/multi-agent-debate.md](docs/multi-agent-debate.md).
Run `make conformal-uncertainty-gate` for an uncertainty layer with conformal-prediction **coverage** guarantees, rejecting an undercovering set; see [docs/conformal-uncertainty.md](docs/conformal-uncertainty.md).
Run `make cross-domain-transfer-gate` for a transfer study to other **state-transition** domains, rejecting a failed transfer; see [docs/cross-domain-transfer.md](docs/cross-domain-transfer.md).
Run `make rl-rollout-sequencing-gate` for an RL rollout-sequencing policy with a **safety-constrained** objective, rejecting an unsafe rollout; see [docs/rl-rollout-sequencing.md](docs/rl-rollout-sequencing.md).
Run `make neurosymbolic-explanations-gate` for a neurosymbolic explanation generator producing readable, checked **proof**s, rejecting an unproven explanation; see [docs/neurosymbolic-explanations.md](docs/neurosymbolic-explanations.md).
Run `make hazard-benchmark-generator-gate` for an automated generator synthesizing **novel** valid hazards, rejecting an invalid or duplicate one; see [docs/hazard-benchmark-generator.md](docs/hazard-benchmark-generator.md).
Run `make reviewer-action-model-gate` for a reviewer model that **predict**s which findings a reviewer acts on, rejecting a mispredicted case; see [docs/reviewer-action-model.md](docs/reviewer-action-model.md).
Run `make continual-learning-eval-gate` for a continual-learning evaluation guarding against catastrophic **forgetting**, rejecting a forgetting release; see [docs/continual-learning-eval.md](docs/continual-learning-eval.md).
Run `make data-valuation-analysis-gate` for a data-**valuation** analysis keeping only positive-value examples, rejecting a negative-value one; see [docs/data-valuation-analysis.md](docs/data-valuation-analysis.md).
Run `make adversarial-training-loop-gate` for an adversarial-training loop **hardening** learned components, rejecting a robustness regression; see [docs/adversarial-training-loop.md](docs/adversarial-training-loop.md).
Run `make interpretability-probe-gate` for a mechanistic-**interpretability** probe explaining internal features, rejecting an opaque feature; see [docs/interpretability-probe.md](docs/interpretability-probe.md).
Run `make cross-lingual-comments-gate` for cross-lingual comment analysis on **non-English** codebases, rejecting a failed-language extraction; see [docs/cross-lingual-comments.md](docs/cross-lingual-comments.md).
Run `make incident-simulation-env-gate` for a simulation environment generating **counterfactual** incident timelines, rejecting a malformed timeline; see [docs/incident-simulation-env.md](docs/incident-simulation-env.md).
Run `make meta-gate-predictor-gate` for a meta-gate that **predict**s which gate fires from cheap features, rejecting a mispredicted case; see [docs/meta-gate-predictor.md](docs/meta-gate-predictor.md).
Run `make replication-leaderboard-gate` for a research-replication **leaderboard** of external reproductions, rejecting an unreproduced entry; see [docs/replication-leaderboard.md](docs/replication-leaderboard.md).
Run `make acm-reproduced-badge-gate` for an ACM Results-**Reproduced** badge package with independent sign-off, rejecting an unmet criterion; see [docs/acm-reproduced-badge.md](docs/acm-reproduced-badge.md).
Run `make tutorial-autograder-gate` for a tutorial series with a gate-backed **autograder**, rejecting an ungraded exercise; see [docs/tutorial-autograder.md](docs/tutorial-autograder.md).
Run `make textbook-chapter-gate` for a textbook-quality chapter derived from repo **evidence**, rejecting an unbacked section; see [docs/textbook-chapter.md](docs/textbook-chapter.md).
Run `make certificate-standard-gate` for a standards proposal codifying the certificate format for **interoperability**, rejecting an underspecified field; see [docs/certificate-standard.md](docs/certificate-standard.md).
Run `make dataset-release-package-gate` for a public dataset release with a **datasheet**, license, and DOI, rejecting a missing requirement; see [docs/dataset-release-package.md](docs/dataset-release-package.md).
Run `make maintainer-council-gate` for a maintainer-council model with documented **decision record**s and term limits, rejecting an undocumented element; see [docs/maintainer-council.md](docs/maintainer-council.md).
Run `make bug-bounty-program-gate` for a funded bug-**bounty** program with payout history and triage SLAs, rejecting an SLA-breaching report; see [docs/bug-bounty-program.md](docs/bug-bounty-program.md).
Run `make contributor-ladder-gate` for a contributor-**ladder** with measurable progression and mentorship, rejecting an undefined rung; see [docs/contributor-ladder.md](docs/contributor-ladder.md).
Run `make community-survey-gate` for an annual community **survey** whose published results drive the roadmap, rejecting an unpublished cycle; see [docs/community-survey.md](docs/community-survey.md).
Run `make workshop-proposal-gate` for a conference workshop with **reproducible demo**s per talk, rejecting a non-reproducible demo; see [docs/workshop-proposal.md](docs/workshop-proposal.md).
Run `make integration-partner-program-gate` for an integration-partner program with **certified**, reproducible deployments, rejecting an uncertified partner; see [docs/integration-partner-program.md](docs/integration-partner-program.md).
Run `make localization-parity-gate` for a localization program with **parity** gates across ten languages, rejecting a lagging locale; see [docs/localization-parity.md](docs/localization-parity.md).
Run `make lts-release-line-gate` for an LTS line with security **backport**s and a clear EOL policy, rejecting a release with no EOL; see [docs/lts-release-line.md](docs/lts-release-line.md).
Run `make accessibility-conformance-gate` for a **WCAG** conformance audit of all human-facing surfaces, rejecting a failing surface; see [docs/accessibility-conformance.md](docs/accessibility-conformance.md).
Run `make public-roadmap-burndown-gate` for a public roadmap with gate-backed quarterly **burndown**s and retrospectives, rejecting an unbacked quarter; see [docs/public-roadmap-burndown.md](docs/public-roadmap-burndown.md).
Run `make third-party-security-audit-gate` for an independent security audit with all findings **remediated** and re-verified, rejecting an open finding; see [docs/third-party-security-audit.md](docs/third-party-security-audit.md).
Run `make adopter-incident-reduction-gate` for a measured reduction in adopter migration-**incident rate**, rejecting an adopter whose rate rose; see [docs/adopter-incident-reduction.md](docs/adopter-incident-reduction.md).
Run `make citation-tracking-dashboard-gate` for a **citation**-tracking dashboard linked to the artifact DOI, rejecting an unlinked citation; see [docs/citation-tracking-dashboard.md](docs/citation-tracking-dashboard.md).
Run `make results-never-regress-gate` for a results-**never regress** guarantee enforced by the full historical benchmark, rejecting a regressing release; see [docs/results-never-regress.md](docs/results-never-regress.md).
Run `make end-to-end-provenance-gate` for an end-to-end **provenance** proof from raw corpus to every paper number, rejecting an untraceable number; see [docs/end-to-end-provenance.md](docs/end-to-end-provenance.md).
Run `make bit-identical-rebuild-gate` for a **bit-identical** rebuild guarantee from frozen snapshots, rejecting a non-deterministic build; see [docs/bit-identical-rebuild.md](docs/bit-identical-rebuild.md).
Run `make formal-methods-appendix-gate` for a formal-methods appendix with all proofs **machine-checked** in CI, rejecting an unchecked proof; see [docs/formal-methods-appendix.md](docs/formal-methods-appendix.md).
Run `make adoption-case-studies-signed-gate` for an adoption case-study series with **signed** bundles from ten organizations, rejecting an unsigned bundle; see [docs/adoption-case-studies-signed.md](docs/adoption-case-studies-signed.md).
Run `make best-paper-readiness-gate` for a best-paper-readiness self-assessment against award **rubric**s with evidence, rejecting an unsupported criterion; see [docs/best-paper-readiness.md](docs/best-paper-readiness.md).
Run `make thousand-star-growth-gate` for a thousand-star growth experiment with measured **funnel** metrics and reproducible interventions, rejecting an unmeasured one; see [docs/thousand-star-growth.md](docs/thousand-star-growth.md).
Run `make one-command-paper-gate` for a **one command** that reproduces the entire paper in a clean container, rejecting a missing artifact; see [docs/one-command-paper.md](docs/one-command-paper.md).
Run `make sustainability-endowment-gate` for a sustainability **endowment** plan with a multi-year funded budget, rejecting an unfunded year; see [docs/sustainability-endowment.md](docs/sustainability-endowment.md).
Run `make extensibility-proof-gate` for an **extensibility** proof that new hazard classes are added cheaply, rejecting an over-budget addition; see [docs/extensibility-proof.md](docs/extensibility-proof.md).
Run `make impact-retrospective-gate` for an impact retrospective tying every design decision to a measured **outcome**, rejecting an unmeasured decision; see [docs/impact-retrospective.md](docs/impact-retrospective.md).
Run `make grand-unified-evidence-index-gate` for a grand-unified evidence index proving **novelty**, rigor, adoption, and reproducibility, rejecting an unbacked pillar; see [docs/grand-unified-evidence-index.md](docs/grand-unified-evidence-index.md).
Run `make type-effect-system-gate` for a mechanized **type-and-effect** system for the migration DSL with progress and preservation proofs, rejecting an unsupported item; see [docs/type-effect-system.md](docs/type-effect-system.md).
Run `make separation-logic-migrations-gate` for a **separation-logic** model of concurrent migrations proving freedom from lost-update anomalies, rejecting an unsupported item; see [docs/separation-logic-migrations.md](docs/separation-logic-migrations.md).
Run `make proof-carrying-verdict-gate` for a proof-carrying-verdict format so every verdict ships a checkable **proof witness**, rejecting an unsupported item; see [docs/proof-carrying-verdict.md](docs/proof-carrying-verdict.md).
Run `make verified-sql-parser-gate` for a verified SQL parser front-end with a proof that the AST **round-trip**s the source, rejecting an unsupported item; see [docs/verified-sql-parser.md](docs/verified-sql-parser.md).
Run `make redaction-structure-proof-gate` for a mechanized proof that **redaction preserves** all join and hazard structure, rejecting an unsupported item; see [docs/redaction-structure-proof.md](docs/redaction-structure-proof.md).
Run `make relational-range-domain-gate` for a relational **abstract interpretation** domain for column value ranges with a soundness proof, rejecting an unsupported item; see [docs/relational-range-domain.md](docs/relational-range-domain.md).
Run `make verified-incremental-analysis-gate` for a verified incremental-analysis algorithm proven **equivalent to a full re-analysis**, rejecting an unsupported item; see [docs/verified-incremental-analysis.md](docs/verified-incremental-analysis.md).
Run `make cutover-temporal-logic-gate` for a **temporal-logic** specification of cutover safety model-checked over all interleavings, rejecting an unsupported item; see [docs/cutover-temporal-logic.md](docs/cutover-temporal-logic.md).
Run `make certificate-lattice-proof-gate` for a mechanized proof that gate-certificate composition forms a **lattice** with a top safe element, rejecting an unsupported item; see [docs/certificate-lattice-proof.md](docs/certificate-lattice-proof.md).
Run `make verified-dsl-compiler-gate` for a verified compiler from the migration DSL to each engine dialect with **semantics preservation**, rejecting an unsupported item; see [docs/verified-dsl-compiler.md](docs/verified-dsl-compiler.md).
Run `make ten-million-migration-mining-gate` for a ten-million-migration mining run with a public, queryable, **content-addressed** index, rejecting an unsupported item; see [docs/ten-million-migration-mining.md](docs/ten-million-migration-mining.md).
Run `make global-hazard-prevalence-map-gate` for a real-time global hazard-**prevalence map** refreshed from public commits hourly, rejecting an unsupported item; see [docs/global-hazard-prevalence-map.md](docs/global-hazard-prevalence-map.md).
Run `make federated-cross-org-analysis-gate` for a **federated** cross-organization analysis that shares hazards without sharing source, rejecting an unsupported item; see [docs/federated-cross-org-analysis.md](docs/federated-cross-org-analysis.md).
Run `make streaming-differential-analyzer-gate` for a streaming differential analyzer reporting **hazard delta**s between any two revisions, rejecting an unsupported item; see [docs/streaming-differential-analyzer.md](docs/streaming-differential-analyzer.md).
Run `make provenance-dedup-gate` for a provenance-preserving **deduplication** proving identical migrations are counted once, rejecting an unsupported item; see [docs/provenance-dedup.md](docs/provenance-dedup.md).
Run `make corpus-bias-correction-gate` for a corpus-bias correction estimator with **post-stratification** weights and diagnostics, rejecting an unsupported item; see [docs/corpus-bias-correction.md](docs/corpus-bias-correction.md).
Run `make multi-cloud-scale-benchmark-gate` for a **multi-cloud** reproducible scale benchmark with identical results across three providers, rejecting an unsupported item; see [docs/multi-cloud-scale-benchmark.md](docs/multi-cloud-scale-benchmark.md).
Run `make energy-carbon-accounting-gate` for an energy-and-**carbon** accounting report for every large analysis run, rejecting an unsupported item; see [docs/energy-carbon-accounting.md](docs/energy-carbon-accounting.md).
Run `make cost-optimal-autoscaling-gate` for a cost-optimal autoscaling policy with a proven **throughput-per-dollar** lower bound, rejecting an unsupported item; see [docs/cost-optimal-autoscaling.md](docs/cost-optimal-autoscaling.md).
Run `make corpus-stats-public-api-gate` for a public, versioned API serving corpus-scale hazard statistics with **rate-limited** reproducibility, rejecting an unsupported item; see [docs/corpus-stats-public-api.md](docs/corpus-stats-public-api.md).
Run `make multi-site-rct-gate` for a multi-site **randomized controlled trial** across organizations with pooled analysis, rejecting an unsupported item; see [docs/multi-site-rct.md](docs/multi-site-rct.md).
Run `make instrumental-variable-estimate-gate` for an **instrumental-variable** estimate of gating's incident effect robust to confounding, rejecting an unsupported item; see [docs/instrumental-variable-estimate.md](docs/instrumental-variable-estimate.md).
Run `make regression-discontinuity-study-gate` for a **regression-discontinuity** study around adoption thresholds with a placebo test, rejecting an unsupported item; see [docs/regression-discontinuity-study.md](docs/regression-discontinuity-study.md).
Run `make difference-in-differences-gate` for a difference-in-differences analysis with **parallel-trends** diagnostics and event-study plots, rejecting an unsupported item; see [docs/difference-in-differences.md](docs/difference-in-differences.md).
Run `make bayesian-hierarchical-model-gate` for a Bayesian hierarchical model of per-team effects with **posterior predictive** checks, rejecting an unsupported item; see [docs/bayesian-hierarchical-model.md](docs/bayesian-hierarchical-model.md).
Run `make preregistered-replication-gate` for a pre-registered replication of the headline trial with a **frozen protocol**, rejecting an unsupported item; see [docs/preregistered-replication.md](docs/preregistered-replication.md).
Run `make evalue-confounding-bound-gate` for a sensitivity-to-confounding bound (**E-value**) for every causal claim, rejecting an unsupported item; see [docs/evalue-confounding-bound.md](docs/evalue-confounding-bound.md).
Run `make long-horizon-cohort-study-gate` for a long-horizon cohort study tracking adopters for twelve months with **attrition** analysis, rejecting an unsupported item; see [docs/long-horizon-cohort-study.md](docs/long-horizon-cohort-study.md).
Run `make mediation-analysis-gate` for a **mediation** analysis decomposing how much effect flows through each gate family, rejecting an unsupported item; see [docs/mediation-analysis.md](docs/mediation-analysis.md).
Run `make economic-field-study-gate` for an economic field study monetizing incident reductions with **confidence interval**s, rejecting an unsupported item; see [docs/economic-field-study.md](docs/economic-field-study.md).
Run `make non-inferiority-analysis-gate` for a **non-inferiority** analysis proving low reviewer-time overhead within a margin, rejecting an unsupported item; see [docs/non-inferiority-analysis.md](docs/non-inferiority-analysis.md).
Run `make hte-analysis-gate` for a **heterogeneity**-of-treatment-effects analysis identifying who benefits most, rejecting an unsupported item; see [docs/hte-analysis.md](docs/hte-analysis.md).
Run `make fraud-resistant-outcome-verification-gate` for a **fraud-resistant** outcome-verification protocol for self-reported incidents, rejecting an unsupported item; see [docs/fraud-resistant-outcome-verification.md](docs/fraud-resistant-outcome-verification.md).
Run `make registered-report-gate` for a registered-report submission with **in-principle acceptance** before results, rejecting an unsupported item; see [docs/registered-report.md](docs/registered-report.md).
Run `make adversarial-collaboration-gate` for an **adversarial-collaboration** study with a skeptic co-author and agreed analysis plan, rejecting an unsupported item; see [docs/adversarial-collaboration.md](docs/adversarial-collaboration.md).
Run `make autonomous-repair-agent-gate` for an **autonomous** repair agent that opens, defends, and merges verified migration PRs, rejecting an unsupported item; see [docs/autonomous-repair-agent.md](docs/autonomous-repair-agent.md).
Run `make verifier-in-the-loop-gate` for a verifier-in-the-loop guarantee that no action merges without a **passing certificate**, rejecting an unsupported item; see [docs/verifier-in-the-loop.md](docs/verifier-in-the-loop.md).
Run `make capability-scoped-sandbox-gate` for a capability-scoped sandbox with a proven **no-side-effect-outside-scope** property, rejecting an unsupported item; see [docs/capability-scoped-sandbox.md](docs/capability-scoped-sandbox.md).
Run `make human-override-audit-trail-gate` for a human-override audit trail proving every autonomous action is **reversible** and logged, rejecting an unsupported item; see [docs/human-override-audit-trail.md](docs/human-override-audit-trail.md).
Run `make learned-policy-safety-case-gate` for a learned-policy safety case documenting hazards, mitigations, and **residual risk**, rejecting an unsupported item; see [docs/learned-policy-safety-case.md](docs/learned-policy-safety-case.md).
Run `make agent-prompt-injection-redteam-gate` for a red-team evaluation of the agent against **prompt-injection** in descriptions, rejecting an unsupported item; see [docs/agent-prompt-injection-redteam.md](docs/agent-prompt-injection-redteam.md).
Run `make abstention-escalation-policy-gate` for an escalation policy that **abstains** and requests review under bounded uncertainty, rejecting an unsupported item; see [docs/abstention-escalation-policy.md](docs/abstention-escalation-policy.md).
Run `make agent-regression-guard-gate` for a regression guard proving the agent never reintroduces a **previously fixed hazard**, rejecting an unsupported item; see [docs/agent-regression-guard.md](docs/agent-regression-guard.md).
Run `make multi-repo-coordination-gate` for a multi-repository coordination protocol with **two-phase** safety, rejecting an unsupported item; see [docs/multi-repo-coordination.md](docs/multi-repo-coordination.md).
Run `make agent-deterministic-replay-gate` for a **deterministic replay** of any agent session reproducing every decision from logs, rejecting an unsupported item; see [docs/agent-deterministic-replay.md](docs/agent-deterministic-replay.md).
Run `make agent-budget-enforcer-gate` for a cost-and-latency budget enforcer with a **hard ceiling** and graceful degradation, rejecting an unsupported item; see [docs/agent-budget-enforcer.md](docs/agent-budget-enforcer.md).
Run `make agent-shadow-mode-gate` for a counterfactual **shadow mode** for safe evaluation of the agent, rejecting an unsupported item; see [docs/agent-shadow-mode.md](docs/agent-shadow-mode.md).
Run `make reviewer-preference-fairness-gate` for a learned reviewer-preference model gated by a **fairness-across-teams** constraint, rejecting an unsupported item; see [docs/reviewer-preference-fairness.md](docs/reviewer-preference-fairness.md).
Run `make provable-kill-switch-gate` for a provable **kill-switch** that halts all autonomy and leaves the repo safe, rejecting an unsupported item; see [docs/provable-kill-switch.md](docs/provable-kill-switch.md).
Run `make autonomy-maturity-model-gate` for an autonomy **maturity model** with measurable levels and gate-backed promotion, rejecting an unsupported item; see [docs/autonomy-maturity-model.md](docs/autonomy-maturity-model.md).
Run `make enterprise-reference-deployment-gate` for a reference enterprise deployment with **published SLOs** for a quarter, rejecting an unsupported item; see [docs/enterprise-reference-deployment.md](docs/enterprise-reference-deployment.md).
Run `make orm-upstream-contribution-gate` for an **upstream** contribution wiring Patchline gates into a major ORM's tooling, rejecting an unsupported item; see [docs/orm-upstream-contribution.md](docs/orm-upstream-contribution.md).
Run `make certificate-rfc-standards-track-gate` for a standards-track RFC with two **interoperable** certificate implementations, rejecting an unsupported item; see [docs/certificate-rfc-standards-track.md](docs/certificate-rfc-standards-track.md).
Run `make hosted-public-good-service-gate` for a hosted public-good service analyzing PRs for free with **transparent cost** reporting, rejecting an unsupported item; see [docs/hosted-public-good-service.md](docs/hosted-public-good-service.md).
Run `make certified-integration-badges-gate` for a certified-integration badge program with ten tools passing **conformance**, rejecting an unsupported item; see [docs/certified-integration-badges.md](docs/certified-integration-badges.md).
Run `make external-curriculum-module-gate` for a curriculum module with **graded**, gate-backed assignments, rejecting an unsupported item; see [docs/external-curriculum-module.md](docs/external-curriculum-module.md).
Run `make industry-working-group-gate` for an industry **working-group** charter with named members and minutes, rejecting an unsupported item; see [docs/industry-working-group.md](docs/industry-working-group.md).
Run `make ten-thousand-repo-funnel-gate` for a ten-thousand-repository activation funnel with **retention** and expansion metrics, rejecting an unsupported item; see [docs/ten-thousand-repo-funnel.md](docs/ten-thousand-repo-funnel.md).
Run `make localization-accessibility-parity-gate` for localization and accessibility conformance covering **twenty languages** with parity gates, rejecting an unsupported item; see [docs/localization-accessibility-parity.md](docs/localization-accessibility-parity.md).
Run `make community-gate-marketplace-gate` for a marketplace of community gates with **signing**, review, and reproducibility, rejecting an unsupported item; see [docs/community-gate-marketplace.md](docs/community-gate-marketplace.md).
Run `make audited-incident-dashboard-gate` for an adopters' incident-rate dashboard, **independently audited**, updated quarterly, rejecting an unsupported item; see [docs/audited-incident-dashboard.md](docs/audited-incident-dashboard.md).
Run `make partner-hazard-sdk-gate` for a partner SDK enabling third parties to ship hazard classes with **conformance tests**, rejecting an unsupported item; see [docs/partner-hazard-sdk.md](docs/partner-hazard-sdk.md).
Run `make office-hours-triage-sla-gate` for a public office-hours and triage rotation with met **response-time SLA**s, rejecting an unsupported item; see [docs/office-hours-triage-sla.md](docs/office-hours-triage-sla.md).
Run `make enterprise-procurement-kit-gate` for an enterprise **procurement** kit backed by automated evidence, rejecting an unsupported item; see [docs/enterprise-procurement-kit.md](docs/enterprise-procurement-kit.md).
Run `make developer-productivity-study-gate` for a developer-productivity study showing reduced migration **review time** at scale, rejecting an unsupported item; see [docs/developer-productivity-study.md](docs/developer-productivity-study.md).
Run `make suite-program-synthesis-gate` for a program-synthesis engine repairing a migration suite to satisfy a **global invariant** set, rejecting an unsupported item; see [docs/suite-program-synthesis.md](docs/suite-program-synthesis.md).
Run `make abstraction-selection-policy-gate` for a learned abstraction-selection policy proven to **never weaken soundness**, rejecting an unsupported item; see [docs/abstraction-selection-policy.md](docs/abstraction-selection-policy.md).
Run `make extractable-neuro-symbolic-gate` for a neuro-symbolic model whose symbolic core is **extractable** and checkable, rejecting an unsupported item; see [docs/extractable-neuro-symbolic.md](docs/extractable-neuro-symbolic.md).
Run `make schema-self-supervised-pretrain-gate` for a **self-supervised** pretraining objective over schemas with a downstream-accuracy gate, rejecting an unsupported item; see [docs/schema-self-supervised-pretrain.md](docs/schema-self-supervised-pretrain.md).
Run `make hazard-equivalence-classes-gate` for a theory of hazard equivalence classes with a proven **canonical-form** algorithm, rejecting an unsupported item; see [docs/hazard-equivalence-classes.md](docs/hazard-equivalence-classes.md).
Run `make information-theoretic-bound-gate` for a formal **information-theoretic** bound on detectable hazards from a feature set, rejecting an unsupported item; see [docs/information-theoretic-bound.md](docs/information-theoretic-bound.md).
Run `make theorem-discovery-loop-gate` for an automated **theorem-discovery** loop proposing and proving safety lemmas, rejecting an unsupported item; see [docs/theorem-discovery-loop.md](docs/theorem-discovery-loop.md).
Run `make event-sourcing-crdt-transfer-gate` for a cross-paradigm transfer to event-sourcing and **CRDT** transitions with held-out tests, rejecting an unsupported item; see [docs/event-sourcing-crdt-transfer.md](docs/event-sourcing-crdt-transfer.md).
Run `make macro-hygiene-dsl-extension-gate` for a verified-by-construction DSL extension mechanism with **macro-hygiene** proofs, rejecting an unsupported item; see [docs/macro-hygiene-dsl-extension.md](docs/macro-hygiene-dsl-extension.md).
Run `make continual-evaluation-harness-gate` for a continual-evaluation harness on a growing benchmark with **anti-overfitting** audits, rejecting an unsupported item; see [docs/continual-evaluation-harness.md](docs/continual-evaluation-harness.md).
Run `make mechanistic-feature-study-gate` for a **mechanistic** study explaining which corpus features drive each decision, rejecting an unsupported item; see [docs/mechanistic-feature-study.md](docs/mechanistic-feature-study.md).
Run `make uncertainty-decomposition-gate` for an uncertainty-decomposition separating **aleatoric** from epistemic risk, rejecting an unsupported item; see [docs/uncertainty-decomposition.md](docs/uncertainty-decomposition.md).
Run `make robustness-certificate-gate` for a formal robustness certificate against **bounded perturbations** of the input, rejecting an unsupported item; see [docs/robustness-certificate.md](docs/robustness-certificate.md).
Run `make foundation-model-finetune-gate` for a reproducible foundation-model finetune with deterministic **post-hoc verification**, rejecting an unsupported item; see [docs/foundation-model-finetune.md](docs/foundation-model-finetune.md).
Run `make research-problems-registry-gate` for an open research-problems registry with **bounties** and gate-checkable criteria, rejecting an unsupported item; see [docs/research-problems-registry.md](docs/research-problems-registry.md).
Run `make one-command-paper-repro-gate` for a single command reproducing the entire paper in a **clean container**, rejecting an unsupported item; see [docs/one-command-paper-repro.md](docs/one-command-paper-repro.md).
Run `make provenance-linked-camera-ready-gate` for a continuously-rebuilt camera-ready PDF with every number **provenance-linked**, rejecting an unsupported item; see [docs/provenance-linked-camera-ready.md](docs/provenance-linked-camera-ready.md).
Run `make machine-checked-appendix-badge-gate` for a machine-checked appendix with a public **proof-status badge**, rejecting an unsupported item; see [docs/machine-checked-appendix-badge.md](docs/machine-checked-appendix-badge.md).
Run `make interactive-web-companion-gate` for an interactive web companion where readers re-run claims against **live gates**, rejecting an unsupported item; see [docs/interactive-web-companion.md](docs/interactive-web-companion.md).
Run `make doi-pinned-artifact-snapshot-gate` for a frozen, **DOI-pinned** artifact snapshot with a bit-identical rebuild attestation, rejecting an unsupported item; see [docs/doi-pinned-artifact-snapshot.md](docs/doi-pinned-artifact-snapshot.md).
Run `make threats-to-validity-map-gate` for a **threats-to-validity** section mapping each threat to an experiment, rejecting an unsupported item; see [docs/threats-to-validity-map.md](docs/threats-to-validity-map.md).
Run `make frozen-related-work-comparison-gate` for a related-work comparison generated from a shared, **frozen benchmark** harness, rejecting an unsupported item; see [docs/frozen-related-work-comparison.md](docs/frozen-related-work-comparison.md).
Run `make negative-results-chapter-gate` for a **negative-results** and limitations chapter with experiments per boundary, rejecting an unsupported item; see [docs/negative-results-chapter.md](docs/negative-results-chapter.md).
Run `make independent-artifact-evaluation-gate` for an independent artifact-evaluation dry run with an external **reproduction log**, rejecting an unsupported item; see [docs/independent-artifact-evaluation.md](docs/independent-artifact-evaluation.md).
Run `make historical-results-never-regress-gate` for a results-**never regress** guarantee enforced by the full historical benchmark, rejecting an unsupported item; see [docs/historical-results-never-regress.md](docs/historical-results-never-regress.md).
Run `make audited-incident-reduction-field-gate` for an **independently-audited** reduction in real-world migration incidents, rejecting an unsupported item; see [docs/audited-incident-reduction-field.md](docs/audited-incident-reduction-field.md).
Run `make citation-adoption-tracking-gate` for a citation- and adoption-tracking dashboard tied to the **DOI-pinned** artifact, rejecting an unsupported item; see [docs/citation-adoption-tracking.md](docs/citation-adoption-tracking.md).
Run `make successor-architecture-proof-gate` for a successor-architecture proof that the next frontier is reachable **without a rewrite**, rejecting an unsupported item; see [docs/successor-architecture-proof.md](docs/successor-architecture-proof.md).
Run `make sustainability-endowment-budget-gate` for a multi-year sustainability endowment with a published, **funded maintenance** budget, rejecting an unsupported item; see [docs/sustainability-endowment-budget.md](docs/sustainability-endowment-budget.md).
Run `make governance-succession-plan-gate` for a governance succession plan proving **bus-factor** resilience with term limits, rejecting an unsupported item; see [docs/governance-succession-plan.md](docs/governance-succession-plan.md).
Run `make ratified-certificate-format-gate` for a standards-body-ratified certificate format with **conformance tests** and reference implementations, rejecting an unsupported item; see [docs/ratified-certificate-format.md](docs/ratified-certificate-format.md).
Run `make best-paper-best-artifact-dossier-gate` for a best-paper-and-best-artifact dossier scored against published **award rubrics** with evidence, rejecting an unsupported item; see [docs/best-paper-best-artifact-dossier.md](docs/best-paper-best-artifact-dossier.md).
Run `make thousand-plus-star-funnel-gate` for a thousand-plus-star growth result with a reproducible **acquisition funnel**, rejecting an unsupported item; see [docs/thousand-plus-star-funnel.md](docs/thousand-plus-star-funnel.md).
Run `make longitudinal-impact-retrospective-gate` for a longitudinal impact-retrospective tying every decision to a **measured** outcome, rejecting an unsupported item; see [docs/longitudinal-impact-retrospective.md](docs/longitudinal-impact-retrospective.md).
Run `make grand-unified-one-command-index-gate` for a grand-unified, **one-command** evidence index proving novelty, rigor, autonomy, adoption, and reproducibility, rejecting an unsupported item; see [docs/grand-unified-one-command-index.md](docs/grand-unified-one-command-index.md).

</details>

Add `--redact` to write `analysis-bundle/` copies with stable redaction tokens for identifiers, literals, customer-like strings, and secret-like values while preserving joins and existing artifact hashes.

Add `--ci` to write `ci/summary.md` plus upload snippets and code-quality artifacts for GitHub Actions, GitLab CI, and Bitbucket Pipelines: SARIF under `analysis-bundle/summary.sarif`, GitLab `ci/gl-code-quality-report.json`, and Bitbucket `ci/bitbucket-code-insights.json`.

For pull requests, `repo pr-comment --base <baseline> --head <baseline>` writes a Markdown body that includes only new or changed data-risk findings; the composite GitHub Action can post that body when `comment-on-pr: "true"` and a pull-request token are provided. Baseline, proposal, and maintainer triage reports read CODEOWNERS when present so risky findings and generated interventions include likely reviewers.

For local developer hooks, `repo hook pre-commit` scans staged files from the git index and `repo hook pre-push` scans branch deltas from a base ref. Both modes mirror only changed local files into a scratch scan tree and report finding deltas without downloading external repositories.

For example, with an OpenAI key already exported as `OPENAI_API_KEY` or `openai_api_key`, you can pass a command that reads Patchline's prompt from stdin and writes generated text to stdout:

```bash
go run ./cmd/patchline repo propose \
  --from-report results/generated/repos/django-auth-baseline \
  --proposal-kind tests \
  --llm-command 'python3 -c "import json, os, sys, urllib.request; prompt=sys.stdin.read(); key=os.environ.get(\"OPENAI_API_KEY\") or os.environ[\"openai_api_key\"]; body=json.dumps({\"model\":\"gpt-4o-mini\",\"messages\":[{\"role\":\"user\",\"content\":prompt}]}).encode(); req=urllib.request.Request(\"https://api.openai.com/v1/chat/completions\", data=body, headers={\"Authorization\":\"Bearer \"+key,\"Content-Type\":\"application/json\"}); print(json.load(urllib.request.urlopen(req))[\"choices\"][0][\"message\"][\"content\"])"' \
  --out results/generated/repos/django-auth-openai-proposal
```

## Why this is useful

Real production data problems often start as ordinary files: a broad migration, a risky backfill, a rollback note, a deploy export, a trace dump, or an ad hoc repair script. Those files are usually scattered across repos and tools. Patchline gives you a first pass over them without asking you to prepare a special dataset.

On the public Bytebase repository, this command downloads the repo and checks its real migration directory:

```bash
go run ./cmd/patchline intake \
  --github bytebase/bytebase \
  --subpath backend/migrator/migration \
  --out results/generated/intake
```

Recent output from that command:

```text
files=251 sql_files=251 high_risk=378 medium_risk=725
problems=339 causes=339 repair_candidates=16 links=10243
```

One concrete finding was a high-risk update in Bytebase's real migration history:

```text
3.1/0000##sheet_blob.sql
table=sheet
update sheet set sha256 = sha256(convert_to(sheet.statement, ?)) where statement is not null
```

Patchline also emits runnable follow-up commands such as:

```bash
patchline analyze-migration .../backend/migrator/migration/3.1/0000##sheet_blob.sql --json
```

Migration analysis can normalize PostgreSQL, MySQL, SQLite, SQL Server, Oracle, and BigQuery syntax with `--dialect`; source scanning also extracts ORM write effects, transaction-boundary evidence, idempotency signals, lock/concurrency hazards, data-retention/privacy hazards, and invariant candidates from common application frameworks.

That is the core value: before you write custom labels, manifests, or benchmark cases, Patchline can already tell you where risky data transitions, possible causes, and possible repair/rollback evidence are hiding in a real project.

## Plug-and-play demo on existing projects

Run one command to check several public projects and get a compact report:

```bash
make plug-and-play-demo
```

To run the staged fetch -> inventory -> intake -> baseline path on real GitHub repos:

```bash
make repo-demo
make four-repo-demo
make repo-slice-matrix
make impact-gate
make parser-fact-gate
make generated-code-gate
make report-section-gate
make metric-impact-gate
make finding-signal-gate
make nondeterministic-gate
make public-command-gate
make industrial-research-gate
make development-cycle-gate
make doctor-gate
make quickstart-gate
make triage-gate
make stable-id-gate
make suppression-gate
make why-now-gate
make run-change-gate
make notify-summary-gate
make explain-finding-gate
make public-gallery-gate
make real-repo-catalog-gate
make non-github-source-gate
make dataset-card-gate
make corpus-fairness-gate
make stratified-benchmark-gate
make stale-ref-gate
make issue-template-gate
make compatibility-gate
make changelog-gate
make secret-scan-gate
make prompt-context-gate
make redaction-stability-gate
make supply-chain-provenance-gate
make release-checksum-gate
make threat-model-gate
make minimizer-gate
make recurrence-gate
make corpus-release-gate
make research-question-gate
make research-experiment-driver-gate
make bootstrap-confidence-gate
make paired-statistical-tests-gate
make effect-size-gate
make sensitivity-analysis-gate
make ablation-dashboard-gate
make negative-control-gate
make reviewer-mode-gate
make artifact-consistency-gate
make disposable-worktree-gate
make language-test-placement-gate
make guard-mutation-gate
make native-sandbox-profile-gate
make generated-provenance-gate
make repair-manifest-schema-gate
make generated-patch-minimization-gate
make generated-risk-budget-gate
make safe-review-badge-gate
make intervention-replay-gate
```

The demo downloads real GitHub project subpaths and writes:

```text
results/generated/plug-and-play-demo/summary.md
results/generated/plug-and-play-demo/summary.json
results/generated/plug-and-play-demo/cases/*/summary.sarif
results/generated/real-repo-slice-matrix/slice-matrix.md
results/generated/real-repo-slice-matrix/slice-matrix.json
```

The real-repo slice matrix is backed by `examples/real-repo-slices.json` and `examples/real-repo-adjudications.json`. It reports each public slice by ecosystem, migration framework, repo size class, available evidence types, fetched commit, runtime, memory, download size, cache hit rate, maintainer review burden, inventory coverage, grep-only, SQL-only, identifier-only, temporal-only, fact-grounded-generation, deterministic re-analysis, sampled false-positive/false-negative adjudications, risks, linked candidates, time signals, generated artifacts, before/after deltas, and cache proof.

`make impact-gate` checks `examples/feature-impact-gates.json` so each feature entry names the public repo slice and real-repo failure mode it is meant to fix.

`make parser-fact-gate` checks `examples/parser-fact-gates.json` by fetching a public repo slice, running inventory, and proving the expected fact kind appears in `facts.jsonl`.

`make generated-code-gate` checks `examples/generated-code-gates.json` by proving deliberately bad generated artifacts are rejected by deterministic compare-stage checks on a public repo slice.

`make report-section-gate` checks `examples/report-section-gates.json` by generating reports for a public repo slice and proving each gated section exists and names the maintainer decision it improves.

`make metric-impact-gate` checks `examples/metric-impact-gates.json` by generating public-repo analysis and proving each metric affects ranking, repair safety, or baseline comparison.

`make finding-signal-gate` checks `examples/finding-signal-gates.json` by proving human-facing reports cap findings to the strongest ranked risks across four public repo slices while full JSON keeps the complete ranked set.

`make nondeterministic-gate` checks `examples/nondeterministic-gates.json` by proving generator-backed proposals are optional, budgeted, hash-audited, and followed by deterministic compare analysis across four public repo slices.

`make public-command-gate` checks `examples/public-command-gates.json` by fetching four public repo slices and proving the analysis workflow runs from downloaded local paths in short shell sequences.

`make industrial-research-gate` checks `examples/industrial-research-gates.json` by regenerating the four-repo matrix and proving each practical maintainer report field is paired with experiment-grade comparison, ablation, adjudication, and verification fields.

`make development-cycle-gate` checks `examples/development-cycle-gates.json` by regenerating the four-repo capstone demo and proving generated interventions add covered risks before deterministic deep re-analysis.

`make doctor-gate` checks `examples/doctor-gates.json` by running `patchline doctor` on four public repo slices and proving preflight diagnostics are emitted before analysis.

`make quickstart-gate` checks `examples/quickstart-gates.json` by running emitted three-command quickstarts against four public repo slices and verifying the expected artifacts.

`make triage-gate` checks `examples/triage-gates.json` by running analysis on four public repo slices and proving maintainer triage dashboards group findings by migrations, app write paths, jobs, tests, incidents, runbooks, and generated interventions.

`make stable-id-gate` checks `examples/stable-id-gates.json` by running analysis on four public repo slices and proving ranked findings include path- and line-drift-resistant stable risk IDs.

`make suppression-gate` checks `examples/suppression-gates.json` by validating suppression ledgers with owners, rationales, expiry dates, stable IDs, and evidence hashes across four public repo slices.

`make why-now-gate` checks `examples/why-now-gates.json` by comparing stored previous baselines to current baselines across four public repo slices and highlighting newly introduced stable risks.

`make run-change-gate` checks `examples/run-change-gates.json` by comparing stored analysis runs across four public repo slices and proving changed facts, ranked risks, evidence links, generated artifacts, and deterministic check outcomes are reported.

`make notify-summary-gate` checks `examples/notify-summary-gates.json` by emitting Slack/GitHub-friendly summaries with only the top maintainer action, top risk, reproduction command, and bundle link across four public repo slices.

`make explain-finding-gate` checks `examples/explain-finding-gates.json` by explaining stable finding IDs from four public repo slices with evidence, ranking factors, alternatives considered, proof holes, and verification commands.

`make public-gallery-gate` checks `examples/public-gallery-gates.json` by generating a public gallery with redacted analysis bundles, pinned commits, expected bundle/screenshot hashes, and SVG maintainer-facing screenshots for four public repo slices.

`make real-repo-catalog-gate` checks `examples/real-repo-catalog.json` by verifying at least 25 pinned public slices across Rails, Django, Alembic, Flyway, Liquibase, Prisma, TypeORM, EF Core, Go, Java, Node, and monorepos.

`make non-github-source-gate` checks `examples/non-github-source-gates.json` by fetching GitLab, Bitbucket, SourceHut, and release/archive tarball sources through the same provenance and content-addressed cache path.

`make dataset-card-gate` generates dataset cards for every public catalog slice with license, commit, ecosystem, migration framework, evidence types, limitations, and reproducibility commands.

`make corpus-fairness-gate` audits the public catalog for ecosystem, framework, and source-host coverage and reports over-reliance flags with recommendations.

`make stratified-benchmark-gate` materializes benchmark manifests by ecosystem and migration framework so experiments can report stratified results instead of aggregate-only metrics.

`make stale-ref-gate` checks pinned public refs still resolve and downloaded archive hashes match expected values.

`make issue-template-gate` validates issue labels and triage forms for real-repo nominations, ecosystem support, parser requests, false positives, false negatives, and artifact regressions, then generates sample payloads from a pinned public Bytebase analysis.

`make compatibility-gate` cross-builds the CLI for macOS and Linux, validates Dockerfile/devcontainer assumptions, and analyzes a pinned public Lobsters migration slice using only the minimal local tool profile.

`make changelog-gate` validates `CHANGELOG.md` against `examples/changelog-gate.json`, checks every user-visible entry names a public proof and gate, and runs a pinned public Lobsters smoke analysis.

`make secret-scan-gate` injects deterministic canaries into a local data-change fixture, runs redacted analysis with diagnostics and CI outputs, scans reports/prompts/bundles/generated artifacts/logs for leaks, and also verifies a pinned public Lobsters slice.

`make minimizer-gate` runs `repo minimize` on four public slices and proves minimized source copies preserve findings, evidence links, and generated intervention metadata.

`make recurrence-gate` runs cross-repo recurrence analysis on four unrelated public slices and verifies repeated failure-mode signatures are reported without source paths, SQL text, or table identifiers.

`make corpus-release-gate` builds public corpus release artifacts with generated reports, SHA-256 checksums, a signed checksum attestation, and one-command reproduction instructions.

`make research-question-gate` validates the primary research questions and operationalizes fact extraction, risk ranking, evidence linking, generated-intervention safety, and before/after re-analysis on the four pinned public slices.

`make research-experiment-driver-gate` runs the research-question experiment driver from a detached clean checkout and writes immutable hashed result ledgers.

`make bootstrap-confidence-gate` computes deterministic bootstrap confidence intervals for ranking, linking, generated-check, runtime, and review-burden metrics across the four pinned public slices.

`make paired-statistical-tests-gate` regenerates the four-slice matrix and runs exact paired sign tests for Patchline versus grep-only, SQL-only, identifier-only, temporal-only, and no-facts-generation baselines.

`make effect-size-gate` adds magnitude reporting to the paired tests: mean/median deltas, relative lift, win/tie rates, and standardized paired deltas.

`make sensitivity-analysis-gate` runs budget variants and deterministic post-hoc sweeps for finding caps, link-confidence thresholds, temporal windows, and risk-weight settings on the four pinned public slices.

`make ablation-dashboard-gate` builds JSON and Markdown ablation dashboards showing which feature families matter by ecosystem and observed failure-mode kind across the four pinned public slices.

`make negative-control-gate` runs documentation-only, vendor-only, and test-only public slices and verifies Patchline does not emit high-confidence repair claims for them.

`make reviewer-mode-gate` rebuilds reviewer tables, an SVG figure, and claim ledgers from raw generated JSON outputs without manual copying.

`make artifact-consistency-gate` verifies README command coverage, regenerated reviewer tables, claim ledgers, checksums, and raw experiment JSON stay consistent.

`make disposable-worktree-gate` applies generated proposal files into throwaway Git worktrees for the four pinned public slices and verifies the real diffs match deterministic compare outputs.

`make language-test-placement-gate` verifies generated test proposals land in ecosystem-native test locations for Rails, Django/Python, Go, Java, Node, and Python public slices.

`make guard-mutation-gate` deletes required generated-guard checks across four public slices and proves deterministic compare rejects each weakened artifact.

`make native-sandbox-profile-gate` checks Go, Node, Python, and Ruby public repositories and proves discovered native tests carry network-off sandbox profiles with isolated HOME/cache/temp write scopes.

`make generated-provenance-gate` checks four public repo slices and proves generated patches cite risk IDs, fact hashes, and sanitized evidence paths.

`make repair-manifest-schema-gate` generates repair manifests for four public repo slices and checks machine-readable scope, preconditions, postconditions, rollback steps, validation commands, and owner review status.

`make generated-patch-minimization-gate` injects redundant generated proposal hunks into four public repo analyses and proves `repo proposal-minimize` removes them while preserving compare coverage and deterministic checks.

`make generated-risk-budget-gate` mutates generated explain proposals on four public repo slices and proves compare rejects interventions whose new SQL risk budget exceeds covered risks.

`make safe-review-badge-gate` checks four public repo slices and proves compare emits a safe-to-review badge only when deterministic checks pass, native checks are passed or explicitly unavailable, and proof holes are listed.

`make intervention-replay-gate` replays generated interventions from four public repo analyses, hashing prompt context, generation output, applied diff, compare results, and replay metadata.

Current default projects:

| Project slice | Files | SQL files | Loose SQL | High-risk SQL | Problems | Causes | Repairs | Links | Time signals |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| Bytebase migrations (`bytebase/bytebase:backend/migrator/migration`) | 251 | 251 | 0 | 378 | 339 | 339 | 16 | 10243 | 2 |
| Django auth migrations (`django/django:django/contrib/auth/migrations`) | 13 | 0 | 3 | 3 | 2 | 2 | 0 | 2 | 0 |
| Mastodon migrations (`mastodon/mastodon:db/migrate`) | 514 | 0 | 97 | 58 | 44 | 44 | 6 | 76 | 525 |

Each case directory contains the full intake report, SARIF for code-scanning systems, and command log. To scan your own project list:

```bash
PATCHLINE_PLUGPLAY_CASES=$'My app|owner/repo|path/to/migrations\nOther app|owner2/repo2|db/migrate' \
  bash scripts/plug-and-play-demo.sh results/generated/my-plugplay
```

## What intake finds

`patchline intake` produces `summary.json` and `summary.md` with:

| Finds | From |
| --- | --- |
| Source provenance | `source.json` from repo fetch: owner/repo, ref, resolved commit, subpath, archive hash, timestamp, tool version, and cache hit/path |
| One-command analysis | `analyze.json`, `analyze.md`, and `analysis-bundle/` from repo analyze: fetch, inventory, baseline, proposal, compare, and deep-analysis summaries in one staged run |
| Risky data-changing SQL | `.sql`, `.psql`, `.ddl`, and SQL snippets in text/source files |
| Project facts | `facts.jsonl` from repo inventory: files, languages, migrations, schema evolution, native commands, unknown JSON/YAML/TOML/log fields, docs, evidence exports, tables, columns, models, endpoints, queues, jobs, deploys, timestamps, errors, and next commands |
| Baseline ranking | `baseline.json`, `baseline.md`, and `baseline.sarif` from repo baseline: ranked SQL and code-path risks, ranking explanations with feature ablations, provenance slices, Datalog-style query rows, abstract effects, symbolic checks, temporal windows, recurrence patterns, policy checks, repair proof summaries, evidence links, native checks, and ablation counts |
| Bounded proposals | `proposal.patch` plus isolated untrusted tests, guards, instrumentation notes, repair manifests, explain SQL, and intervention metadata from repo propose |
| Proposal checks | `compare.json` and `compare.md` from repo compare: generated-artifact checks, coverage, intervention-loop status, risky generated SQL, optional safe native-test execution, and review status |
| Embedded/source SQL | Go, Python, Ruby, JavaScript/TypeScript, Java, C#, shell, and SQL files |
| Cause candidates | risky migrations, deploy/trace/commit/migration signals, incident-like notes, export fields |
| Repair candidates | repair manifests, rollback/revert/restore/backfill/reconcile/fix clues in SQL/docs/scripts |
| Time signals | dates in filenames and text that can slice a repo/export around likely migration, incident, deploy, or repair windows |
| Existing evidence | JSONL evidence, Datadog/OTLP/GitHub/Postgres/migration-runner exports when recognizable |
| Unknown JSON signals | generic SQL/trace/deploy/commit/record fields without requiring a known schema |
| SARIF output | `summary.sarif` for CI/code-scanning integration |
| Next commands | exact commands that should run on discovered files |

Candidate links are conservative: Patchline links problems, causes, and repairs only through shared identifiers such as table names, incident IDs, or commits. A link is a lead to inspect, not proof of causality.

## Common commands

```bash
# Build
go build -o bin/patchline ./cmd/patchline

# Scan current repo/export data
go run ./cmd/patchline intake . --out results/generated/intake

# Scan part of a public GitHub repo
go run ./cmd/patchline intake --github bytebase/bytebase --subpath backend/migrator/migration --out results/generated/intake

# Compare several existing public projects
make plug-and-play-demo

# Fetch, inventory, analyze, and rank one real GitHub repo slice
make repo-demo

# Prove the staged workflow across four real external repo slices
make four-repo-demo

# Use as a GitHub Action in another repo
# - uses: thehalleyyoung/patchline@main
#   with:
#     path: .
#     out: results/patchline-intake

# Analyze one migration directly
go run ./cmd/patchline analyze-migration path/to/migration.sql --json

# Extract embedded SQL from a source tree
go run ./cmd/patchline extract-sql path/to/source --json

# Adapt existing observability/deploy exports when they match known formats
go run ./cmd/patchline adapt-evidence datadog export.json --json
go run ./cmd/patchline adapt-evidence otlp export.json --json
go run ./cmd/patchline adapt-evidence postgres logical-decoding.json --json
go run ./cmd/patchline adapt-evidence github deployments.json --json
go run ./cmd/patchline adapt-evidence migration-runner migrations.json --json
go run ./cmd/patchline adapt-evidence jira issues.json --json
go run ./cmd/patchline adapt-evidence linear issues.json --json

# Lint or replay richer repair artifacts if you already have them
go run ./cmd/patchline lint-repair repair.json --json
go run ./cmd/patchline dry-run repair.json --store store.json --json
go run ./cmd/patchline db-dry-run repair.json --dialect postgres --json
```

The Datadog-style adapter recognizes deploy events, incidents, traces, logs, monitors, SLOs, and notebooks from exported JSON or exported IaC-shaped JSON without requiring Datadog API access. The OTLP adapter ingests OpenTelemetry collector `resourceSpans` and `resourceLogs` exports so traces and logs can join the same deterministic evidence graph.
The Jira and Linear adapters normalize issue exports into incident evidence while preserving issue IDs, created/updated/resolved timestamps, owners, labels, URLs, and repair links when present.
Repo inventory and baseline reports also scan Kubernetes and Terraform files for database jobs, migration jobs, cron repairs, secret references, and deploy-ordering gates that can change the safety envelope of a data repair.
`db-dry-run` emits schema-only Postgres/MySQL scripts and local container commands for repair manifests; it refuses non-local DSNs so production credentials are never required for this hook.

## What deeper checks add

The intake command is the front door. If your tree also contains richer inputs, Patchline can go further:

| If you have | Patchline can check |
| --- | --- |
| repair manifests | operation scope, dependency cycles, rollback requirements, generated SQL |
| bounded before/after stores | replayed row diffs and repair effects |
| invariants | row-count/frame/invariant obligations, with Z3 when available |
| event JSONL or adapted exports | provenance graphs, trace reconstruction, blast radius |
| prior incident archives | recurring risky tables, missing rollback patterns, repeated repair shapes |

Patchline always records when it cannot prove something. Missing evidence becomes an explicit downgrade, not a made-up success.

## Validation

Run the normal test suite:

```bash
go test ./...
```

Run the small intake demo bundled with this repo:

```bash
make intake-demo
```

Run the broader deterministic validation suite when you want to exercise the benchmark, provenance, baseline, ablation, and public-corpus checks:

```bash
PATCHLINE_PUBLIC_CORPUS_OFFLINE=1 make artifact-full
```

`artifact-full` is intentionally separate from live source checks and maintainer refresh targets. Expected-output refresh commands require explicit opt-in:

```bash
PATCHLINE_ACCEPT_EXPECTED_REFRESH=1 make artifact-benchmark-refresh
PATCHLINE_ACCEPT_EXPECTED_REFRESH=1 make artifact-studies-refresh
```

## Repository map

```text
cmd/patchline/          CLI entry point
internal/intake/       current-data scanner and GitHub project intake
internal/migration/    SQL and embedded-source-SQL analysis
internal/evidence/     JSONL ingest plus Datadog/OTLP/GitHub/Postgres adapters
internal/repair/       repair manifests, linting, SQL generation, dry-run support
internal/artifact/     deterministic validation reports and benchmark checks
examples/              small runnable examples and public-source-derived fixtures
benchmarks/            frozen validation inputs and expected outputs
docs/                  deeper design notes
```

## Current limits

Patchline is useful as a deterministic first pass and audit trail, not as an automatic incident resolver. It does not infer private production data, prove arbitrary SQL, synthesize repairs from prose, or replace operator review. It gives you grounded leads and runnable checks from the data already in front of you.
