.PHONY: build test demo intake-demo plug-and-play-demo repo-demo four-repo-demo repo-slice-matrix impact-gate parser-fact-gate generated-code-gate report-section-gate metric-impact-gate finding-signal-gate nondeterministic-gate public-command-gate industrial-research-gate development-cycle-gate doctor-gate quickstart-gate triage-gate stable-id-gate suppression-gate why-now-gate run-change-gate notify-summary-gate explain-finding-gate public-gallery-gate real-repo-catalog-gate non-github-source-gate dataset-card-gate corpus-fairness-gate stratified-benchmark-gate stale-ref-gate issue-template-gate minimizer-gate recurrence-gate corpus-release-gate research-question-gate research-experiment-driver-gate bootstrap-confidence-gate paired-statistical-tests-gate effect-size-gate sensitivity-analysis-gate ablation-dashboard-gate negative-control-gate reviewer-mode-gate artifact-consistency-gate disposable-worktree-gate language-test-placement-gate guard-mutation-gate native-sandbox-profile-gate generated-provenance-gate repair-manifest-schema-gate generated-patch-minimization-gate generated-risk-budget-gate safe-review-badge-gate intervention-replay-gate sql-dialect-normalization-gate orm-write-effect-extraction-gate transaction-boundary-inference-gate idempotency-classification-gate lock-concurrency-hazard-gate data-retention-privacy-gate invariant-mining-gate trace-code-link-gate blast-radius-gate proof-hole-minimization-gate pr-comment-workflow-gate cross-ci-code-quality-gate datadog-export-adapter-gate otel-collector-ingestion-gate issue-export-adapter-gate infrastructure-scan-gate db-dry-run-gate pre-hook-gate codeowners-routing-gate offline-validation-gate architecture-layer-gate plugin-interface-gate golden-fixture-gate fuzz-coverage-gate performance-budget-gate diagnostics-gate contributor-check-gate compatibility-gate changelog-gate secret-scan-gate prompt-context-gate redaction-stability-gate supply-chain-provenance-gate release-checksum-gate threat-model-gate archive-security-gate generated-code-quarantine-gate privacy-metrics-gate security-review-gate generated-case-studies-gate failure-taxonomy-gate qualitative-notes-gate cross-file-examples-gate rejected-generated-gate reviewability-examples-gate limitations-ledger-gate claims-evidence-gate paper-figures-gate reviewer-walkthrough-gate landing-readme-gate release-distribution-gate docs-site-gate screencast-gate awesome-patchline-gate comparison-pages-gate roadmap-board-gate reproducibility-report-gate contributor-recognition-gate capstone-demo-gate artifact-evaluation-kit-gate artifact-container-profile-gate artifact-badges-gate paper-appendix-gate reviewer-dry-run-logs-gate artifact-release-manifest-gate rebuttal-response-workspace-gate camera-ready-checklist-gate independent-replication-gate failure-injection-suite-gate longitudinal-public-reruns-gate migration-age-stratification-gate ecosystem-balanced-benchmark-gate repository-size-stratification-gate maintainer-action-simulation-gate severity-calibration-gate fp-adjudication-gate fn-discovery-gate ablation-study-gate effect-size-strata-gate otel-trace-gen-gate datadog-timeline-gate prom-grafana-gate runtime-confidence-gate incident-notebook-gate causality-limits-gate runtime-redaction-gate offline-bundle-gate incident-export-gate negative-controls-gate intervention-contracts-gate diff-minimization-gate quarantine-attestation-gate intervention-provenance-graph-gate rejection-taxonomy-gate generated-test-mutation-gate guard-effectiveness-gate intervention-budget-tuning-gate intervention-scorecard-gate intervention-regression-archive-gate mercurial-fossil-source-gate monorepo-boundary-gate multi-ecosystem-migration-gate nosql-change-gate data-pipeline-gate infra-ordering-gate schema-compat-gate fixture-minimizer-gate parser-dashboard-gate onboarding-quest-gate examples-gallery-gate issue-to-artifact-gate contributor-badges-gate starter-issues-gate governance-gate release-notes-gate office-hours-gate feedback-forms-gate conference-demos-gate adoption-case-studies-gate incremental-cache-gate parallel-corpus-gate resumable-gates-gate error-taxonomy-gate resource-budgets-gate flaky-detect-gate canonical-json-gate shell-portability-gate artifact-gc-gate release-smoke-gate gate fmt public-corpus verify-usefulness artifact-smoke artifact-demo artifact-ground-truth-check phase-check artifact-baselines artifact-ablations artifact-scale artifact-studies artifact-studies-expected artifact-studies-compare artifact-studies-refresh artifact-baselines-public artifact-ablations-public artifact-scale-public artifact-studies-public-expected artifact-studies-public-compare artifact-studies-all artifact-tables artifact-numbers artifact-subtasks artifact-corpus-audit artifact-provenance artifact-benchmark artifact-benchmark-repairs artifact-benchmark-regressions artifact-benchmark-public artifact-benchmark-public-incidents artifact-benchmark-public-repairs artifact-benchmark-public-archive artifact-benchmark-refresh artifact-benchmark-compare artifact-negative-cases artifact-full artifact-clean

build:
	go build -o bin/patchline ./cmd/patchline

test:
	go test ./...

demo:
	go run ./cmd/patchline dry-run examples/repairs/repair-bad-invoice-backfill.json --json

intake-demo:
	go run ./cmd/patchline intake examples --out results/generated/intake

plug-and-play-demo:
	bash scripts/plug-and-play-demo.sh

repo-demo:
	bash scripts/repo-analysis-demo.sh

four-repo-demo:
	bash scripts/four-repo-analysis-demo.sh

repo-slice-matrix:
	bash scripts/four-repo-analysis-demo.sh results/generated/real-repo-slice-matrix

impact-gate:
	bash scripts/impact-gate.sh

parser-fact-gate:
	bash scripts/parser-fact-gate.sh

generated-code-gate:
	bash scripts/generated-code-gate.sh

report-section-gate:
	bash scripts/report-section-gate.sh

metric-impact-gate:
	bash scripts/metric-impact-gate.sh

finding-signal-gate:
	bash scripts/finding-signal-gate.sh

nondeterministic-gate:
	bash scripts/nondeterministic-gate.sh

public-command-gate:
	bash scripts/public-command-gate.sh

industrial-research-gate:
	bash scripts/industrial-research-gate.sh

development-cycle-gate:
	bash scripts/development-cycle-gate.sh

doctor-gate:
	bash scripts/doctor-gate.sh

quickstart-gate:
	bash scripts/quickstart-gate.sh

triage-gate:
	bash scripts/triage-gate.sh

stable-id-gate:
	bash scripts/stable-id-gate.sh

suppression-gate:
	bash scripts/suppression-gate.sh

why-now-gate:
	bash scripts/why-now-gate.sh

run-change-gate:
	bash scripts/run-change-gate.sh

notify-summary-gate:
	bash scripts/notify-summary-gate.sh

explain-finding-gate:
	bash scripts/explain-finding-gate.sh

public-gallery-gate:
	bash scripts/public-gallery-gate.sh

real-repo-catalog-gate:
	bash scripts/real-repo-catalog-gate.sh

non-github-source-gate:
	bash scripts/non-github-source-gate.sh

dataset-card-gate:
	bash scripts/dataset-card-gate.sh

corpus-fairness-gate:
	bash scripts/corpus-fairness-gate.sh

stratified-benchmark-gate:
	bash scripts/stratified-benchmark-gate.sh

stale-ref-gate:
	bash scripts/stale-ref-gate.sh

issue-template-gate:
	bash scripts/issue-template-gate.sh

minimizer-gate:
	bash scripts/minimizer-gate.sh

recurrence-gate:
	bash scripts/recurrence-gate.sh

corpus-release-gate:
	bash scripts/corpus-release-gate.sh

research-question-gate:
	bash scripts/research-question-gate.sh

research-experiment-driver-gate:
	bash scripts/research-experiment-driver-gate.sh

bootstrap-confidence-gate:
	bash scripts/bootstrap-confidence-gate.sh

paired-statistical-tests-gate:
	bash scripts/paired-statistical-tests-gate.sh

effect-size-gate:
	bash scripts/effect-size-gate.sh

sensitivity-analysis-gate:
	bash scripts/sensitivity-analysis-gate.sh

ablation-dashboard-gate:
	bash scripts/ablation-dashboard-gate.sh

negative-control-gate:
	bash scripts/negative-control-gate.sh

reviewer-mode-gate:
	bash scripts/reviewer-mode-gate.sh

artifact-consistency-gate:
	bash scripts/artifact-consistency-gate.sh

disposable-worktree-gate:
	bash scripts/disposable-worktree-gate.sh

language-test-placement-gate:
	bash scripts/language-test-placement-gate.sh

guard-mutation-gate:
	bash scripts/guard-mutation-gate.sh

native-sandbox-profile-gate:
	bash scripts/native-sandbox-profile-gate.sh

generated-provenance-gate:
	bash scripts/generated-provenance-gate.sh

repair-manifest-schema-gate:
	bash scripts/repair-manifest-schema-gate.sh

generated-patch-minimization-gate:
	bash scripts/generated-patch-minimization-gate.sh

generated-risk-budget-gate:
	bash scripts/generated-risk-budget-gate.sh

safe-review-badge-gate:
	bash scripts/safe-review-badge-gate.sh

intervention-replay-gate:
	bash scripts/intervention-replay-gate.sh

sql-dialect-normalization-gate:
	bash scripts/sql-dialect-normalization-gate.sh

orm-write-effect-extraction-gate:
	bash scripts/orm-write-effect-extraction-gate.sh

transaction-boundary-inference-gate:
	bash scripts/transaction-boundary-inference-gate.sh

idempotency-classification-gate:
	bash scripts/idempotency-classification-gate.sh

lock-concurrency-hazard-gate:
	bash scripts/lock-concurrency-hazard-gate.sh

data-retention-privacy-gate:
	bash scripts/data-retention-privacy-gate.sh

invariant-mining-gate:
	bash scripts/invariant-mining-gate.sh

trace-code-link-gate:
	bash scripts/trace-code-link-gate.sh

blast-radius-gate:
	bash scripts/blast-radius-gate.sh

proof-hole-minimization-gate:
	bash scripts/proof-hole-minimization-gate.sh

pr-comment-workflow-gate:
	bash scripts/pr-comment-workflow-gate.sh

cross-ci-code-quality-gate:
	bash scripts/cross-ci-code-quality-gate.sh

datadog-export-adapter-gate:
	bash scripts/datadog-export-adapter-gate.sh

otel-collector-ingestion-gate:
	bash scripts/otel-collector-ingestion-gate.sh

issue-export-adapter-gate:
	bash scripts/issue-export-adapter-gate.sh

infrastructure-scan-gate:
	bash scripts/infrastructure-scan-gate.sh

db-dry-run-gate:
	bash scripts/db-dry-run-gate.sh

pre-hook-gate:
	bash scripts/pre-hook-gate.sh

codeowners-routing-gate:
	bash scripts/codeowners-routing-gate.sh

offline-validation-gate:
	bash scripts/offline-validation-gate.sh

architecture-layer-gate:
	bash scripts/architecture-layer-gate.sh

plugin-interface-gate:
	bash scripts/plugin-interface-gate.sh

golden-fixture-gate:
	bash scripts/golden-fixture-gate.sh

fuzz-coverage-gate:
	bash scripts/fuzz-coverage-gate.sh

performance-budget-gate:
	bash scripts/performance-budget-gate.sh

diagnostics-gate:
	bash scripts/diagnostics-gate.sh

contributor-check-gate:
	bash scripts/contributor-check-gate.sh

compatibility-gate:
	bash scripts/compatibility-gate.sh

changelog-gate:
	bash scripts/changelog-gate.sh

secret-scan-gate:
	bash scripts/secret-scan-gate.sh

prompt-context-gate:
	bash scripts/prompt-context-gate.sh

redaction-stability-gate:
	bash scripts/redaction-stability-gate.sh

supply-chain-provenance-gate:
	bash scripts/supply-chain-provenance-gate.sh

release-checksum-gate:
	bash scripts/release-checksum-gate.sh

threat-model-gate:
	bash scripts/threat-model-gate.sh

archive-security-gate:
	bash scripts/archive-security-gate.sh

generated-code-quarantine-gate:
	bash scripts/generated-code-quarantine-gate.sh

privacy-metrics-gate:
	bash scripts/privacy-metrics-gate.sh

security-review-gate:
	bash scripts/security-review-gate.sh

generated-case-studies-gate:
	bash scripts/generated-case-studies-gate.sh

failure-taxonomy-gate:
	bash scripts/failure-taxonomy-gate.sh

qualitative-notes-gate:
	bash scripts/qualitative-notes-gate.sh

cross-file-examples-gate:
	bash scripts/cross-file-examples-gate.sh

rejected-generated-gate:
	bash scripts/rejected-generated-gate.sh

reviewability-examples-gate:
	bash scripts/reviewability-examples-gate.sh

limitations-ledger-gate:
	bash scripts/limitations-ledger-gate.sh

claims-evidence-gate:
	bash scripts/claims-evidence-gate.sh

paper-figures-gate:
	bash scripts/paper-figures-gate.sh

reviewer-walkthrough-gate:
	bash scripts/reviewer-walkthrough-gate.sh

landing-readme-gate:
	bash scripts/landing-readme-gate.sh

release-distribution-gate:
	bash scripts/release-distribution-gate.sh

docs-site-gate:
	bash scripts/docs-site-gate.sh

screencast-gate:
	bash scripts/screencast-gate.sh

awesome-patchline-gate:
	bash scripts/awesome-patchline-gate.sh

comparison-pages-gate:
	bash scripts/comparison-pages-gate.sh

roadmap-board-gate:
	bash scripts/roadmap-board-gate.sh

reproducibility-report-gate:
	bash scripts/reproducibility-report-gate.sh

contributor-recognition-gate:
	bash scripts/contributor-recognition-gate.sh

capstone-demo-gate:
	bash scripts/capstone-demo-gate.sh

artifact-evaluation-kit-gate:
	bash scripts/artifact-evaluation-kit-gate.sh

artifact-container-profile-gate:
	bash scripts/artifact-container-profile-gate.sh

artifact-badges-gate:
	bash scripts/artifact-badges-gate.sh

paper-appendix-gate:
	bash scripts/paper-appendix-gate.sh

reviewer-dry-run-logs-gate:
	bash scripts/reviewer-dry-run-logs-gate.sh

artifact-release-manifest-gate:
	bash scripts/artifact-release-manifest-gate.sh

rebuttal-response-workspace-gate:
	bash scripts/rebuttal-response-workspace-gate.sh

camera-ready-checklist-gate:
	bash scripts/camera-ready-checklist-gate.sh

independent-replication-gate:
	bash scripts/independent-replication-gate.sh

failure-injection-suite-gate:
	bash scripts/failure-injection-suite-gate.sh

longitudinal-public-reruns-gate:
	bash scripts/longitudinal-public-reruns-gate.sh

migration-age-stratification-gate:
	bash scripts/migration-age-stratification-gate.sh

ecosystem-balanced-benchmark-gate:
	bash scripts/ecosystem-balanced-benchmark-gate.sh

repository-size-stratification-gate:
	bash scripts/repository-size-stratification-gate.sh

maintainer-action-simulation-gate:
	bash scripts/maintainer-action-simulation-gate.sh

severity-calibration-gate:
	bash scripts/severity-calibration-gate.sh

fp-adjudication-gate:
	bash scripts/fp-adjudication-gate.sh

fn-discovery-gate:
	bash scripts/fn-discovery-gate.sh

ablation-study-gate:
	bash scripts/ablation-study-gate.sh

effect-size-strata-gate:
	bash scripts/effect-size-strata-gate.sh

otel-trace-gen-gate:
	bash scripts/otel-trace-gen-gate.sh

datadog-timeline-gate:
	bash scripts/datadog-timeline-gate.sh

prom-grafana-gate:
	bash scripts/prom-grafana-gate.sh

runtime-confidence-gate:
	bash scripts/runtime-confidence-gate.sh

incident-notebook-gate:
	bash scripts/incident-notebook-gate.sh

causality-limits-gate:
	bash scripts/causality-limits-gate.sh

runtime-redaction-gate:
	bash scripts/runtime-redaction-gate.sh

offline-bundle-gate:
	bash scripts/offline-bundle-gate.sh

incident-export-gate:
	bash scripts/incident-export-gate.sh

negative-controls-gate:
	bash scripts/negative-controls-gate.sh

intervention-contracts-gate:
	bash scripts/intervention-contracts-gate.sh

diff-minimization-gate:
	bash scripts/diff-minimization-gate.sh

quarantine-attestation-gate:
	bash scripts/quarantine-attestation-gate.sh

intervention-provenance-graph-gate:
	bash scripts/intervention-provenance-graph-gate.sh

rejection-taxonomy-gate:
	bash scripts/rejection-taxonomy-gate.sh

generated-test-mutation-gate:
	bash scripts/generated-test-mutation-gate.sh

guard-effectiveness-gate:
	bash scripts/guard-effectiveness-gate.sh

intervention-budget-tuning-gate:
	bash scripts/intervention-budget-tuning-gate.sh

intervention-scorecard-gate:
	bash scripts/intervention-scorecard-gate.sh

intervention-regression-archive-gate:
	bash scripts/intervention-regression-archive-gate.sh

mercurial-fossil-source-gate:
	bash scripts/mercurial-fossil-source-gate.sh

monorepo-boundary-gate:
	bash scripts/monorepo-boundary-gate.sh

multi-ecosystem-migration-gate:
	bash scripts/multi-ecosystem-migration-gate.sh

nosql-change-gate:
	bash scripts/nosql-change-gate.sh

data-pipeline-gate:
	bash scripts/data-pipeline-gate.sh

infra-ordering-gate:
	bash scripts/infra-ordering-gate.sh

schema-compat-gate:
	bash scripts/schema-compat-gate.sh

fixture-minimizer-gate:
	bash scripts/fixture-minimizer-gate.sh

parser-dashboard-gate:
	bash scripts/parser-dashboard-gate.sh

onboarding-quest-gate:
	bash scripts/onboarding-quest-gate.sh

examples-gallery-gate:
	bash scripts/examples-gallery-gate.sh

issue-to-artifact-gate:
	bash scripts/issue-to-artifact-gate.sh

contributor-badges-gate:
	bash scripts/contributor-badges-gate.sh

starter-issues-gate:
	bash scripts/starter-issues-gate.sh

governance-gate:
	bash scripts/governance-gate.sh

release-notes-gate:
	bash scripts/release-notes-gate.sh

office-hours-gate:
	bash scripts/office-hours-gate.sh

feedback-forms-gate:
	bash scripts/feedback-forms-gate.sh

conference-demos-gate:
	bash scripts/conference-demos-gate.sh

adoption-case-studies-gate:
	bash scripts/adoption-case-studies-gate.sh

incremental-cache-gate:
	bash scripts/incremental-cache-gate.sh

parallel-corpus-gate:
	bash scripts/parallel-corpus-gate.sh

resumable-gates-gate:
	bash scripts/resumable-gates-gate.sh

error-taxonomy-gate:
	bash scripts/error-taxonomy-gate.sh

resource-budgets-gate:
	bash scripts/resource-budgets-gate.sh

flaky-detect-gate:
	bash scripts/flaky-detect-gate.sh

canonical-json-gate:
	bash scripts/canonical-json-gate.sh

shell-portability-gate:
	bash scripts/shell-portability-gate.sh

artifact-gc-gate:
	bash scripts/artifact-gc-gate.sh

release-smoke-gate:
	bash scripts/release-smoke-gate.sh

gate:
	go run ./cmd/patchline ci-gate examples/benchmarks/strict-migration-corpus.json --min-precision 0.95 --min-recall 0.95

public-corpus:
	bash scripts/fetch-public-corpus.sh
	go run ./cmd/patchline benchmark-suite examples/benchmarks/public-bytebase-migration-corpus.json

verify-usefulness: test gate public-corpus
	go run ./cmd/patchline solver-obligations examples/repairs/repair-bad-invoice-backfill.json --invariants examples/invariants/billing-core.json --json
	go run ./cmd/patchline semantics-audit --json
	go run ./cmd/patchline archive-index examples/archive/bad-migration-corpus.json --json
	go run ./cmd/patchline archive-query examples/archive/bad-migration-corpus.json --json
	go run ./cmd/patchline repair-outcomes examples/archive/bad-migration-corpus.json --json
	go run ./cmd/patchline semantic-regressions examples/archive/bad-migration-corpus.json --json
	go run ./cmd/patchline historical-failures examples/historical-failures/suite.json --json
	bash scripts/verify-historical-sources.sh examples/historical-failures/suite.json

artifact-smoke:
	bash scripts/artifact_smoke.sh

artifact-demo:
	bash scripts/artifact_demo.sh

artifact-ground-truth-check:
	go run ./cmd/patchline artifact-ground-truth benchmarks --json
	bash scripts/validate-ground-truth.sh

phase-check:
	go run ./cmd/patchline phase-check benchmarks/manifests/smoke.json
	go run ./cmd/patchline phase-check benchmarks/manifests/negative.json
	go run ./cmd/patchline phase-check benchmarks/manifests/repair_cases.json
	go run ./cmd/patchline phase-check benchmarks/manifests/semantic_regressions.json
	go run ./cmd/patchline phase-check benchmarks/manifests/public_migrations.json
	go run ./cmd/patchline phase-check benchmarks/manifests/public_incidents.json
	go run ./cmd/patchline phase-check benchmarks/manifests/public_repairs.json
	go run ./cmd/patchline phase-check benchmarks/manifests/public_archive.json

artifact-baselines:
	go run ./cmd/patchline artifact-baselines examples/benchmarks/strict-migration-corpus.json --out results/generated/artifact-studies

artifact-ablations:
	go run ./cmd/patchline artifact-ablations examples/benchmarks/strict-migration-corpus.json --out results/generated/artifact-studies

artifact-scale:
	go run ./cmd/patchline artifact-scale examples/benchmarks/strict-migration-corpus.json --out results/generated/artifact-studies

artifact-studies: artifact-baselines artifact-ablations artifact-scale

artifact-studies-expected: artifact-studies
	go run ./cmd/patchline artifact-study summarize results/generated/artifact-studies --out benchmarks/expected/studies-strict.json

artifact-studies-compare: artifact-studies
	go run ./cmd/patchline artifact-study compare results/generated/artifact-studies benchmarks/expected/studies-strict.json

artifact-studies-refresh:
	bash scripts/refresh-artifact-studies.sh

artifact-baselines-public: public-corpus
	go run ./cmd/patchline artifact-baselines examples/benchmarks/public-bytebase-migration-corpus.json --out results/generated/artifact-studies/public-migrations

artifact-ablations-public: public-corpus
	go run ./cmd/patchline artifact-ablations examples/benchmarks/public-bytebase-migration-corpus.json --out results/generated/artifact-studies/public-migrations

artifact-scale-public: public-corpus
	go run ./cmd/patchline artifact-scale examples/benchmarks/public-bytebase-migration-corpus.json --out results/generated/artifact-studies/public-migrations

artifact-studies-public: artifact-baselines-public artifact-ablations-public artifact-scale-public

artifact-studies-public-expected: artifact-studies-public
	go run ./cmd/patchline artifact-study summarize results/generated/artifact-studies/public-migrations --out benchmarks/expected/studies-public-migrations.json

artifact-studies-public-compare: artifact-studies-public
	go run ./cmd/patchline artifact-study compare results/generated/artifact-studies/public-migrations benchmarks/expected/studies-public-migrations.json

artifact-studies-all: artifact-studies artifact-studies-public

artifact-tables: artifact-studies-all artifact-benchmark-compare artifact-benchmark-public artifact-benchmark-public-incidents artifact-benchmark-public-repairs artifact-benchmark-public-archive
	go run ./cmd/patchline artifact-tables --out results/generated/artifact-tables

artifact-numbers: artifact-studies-all artifact-benchmark-compare artifact-benchmark-public artifact-benchmark-public-incidents artifact-benchmark-public-repairs artifact-benchmark-public-archive
	go run ./cmd/patchline artifact-numbers --out results/generated/artifact-numbers

artifact-subtasks: artifact-numbers
	go run ./cmd/patchline artifact-subtasks --out results/generated/artifact-subtasks

artifact-corpus-audit:
	go run ./cmd/patchline artifact-corpus-audit --out results/generated/artifact-corpus-audit

artifact-provenance: public-corpus artifact-demo artifact-tables artifact-numbers artifact-subtasks artifact-corpus-audit
	go run ./cmd/patchline artifact-provenance --out results/generated/artifact-provenance

artifact-benchmark:
	go run ./cmd/patchline artifact-benchmark validate benchmarks/manifests/smoke.json
	go run ./cmd/patchline artifact-benchmark run benchmarks/manifests/smoke.json --out results/generated/artifact-benchmark/smoke-report.json
	go run ./cmd/patchline artifact-benchmark validate benchmarks/manifests/negative.json
	go run ./cmd/patchline artifact-benchmark run benchmarks/manifests/negative.json --out results/generated/artifact-benchmark/negative-report.json

artifact-benchmark-repairs:
	go run ./cmd/patchline artifact-benchmark validate benchmarks/manifests/repair_cases.json
	go run ./cmd/patchline artifact-benchmark run benchmarks/manifests/repair_cases.json --out results/generated/artifact-benchmark/repair-cases-report.json

artifact-benchmark-regressions:
	go run ./cmd/patchline artifact-benchmark validate benchmarks/manifests/semantic_regressions.json
	go run ./cmd/patchline artifact-benchmark run benchmarks/manifests/semantic_regressions.json --out results/generated/artifact-benchmark/semantic-regressions-report.json

artifact-benchmark-public: public-corpus
	go run ./cmd/patchline artifact-benchmark validate benchmarks/manifests/public_migrations.json
	go run ./cmd/patchline artifact-benchmark run benchmarks/manifests/public_migrations.json --out results/generated/artifact-benchmark/public-migrations-report.json
	go run ./cmd/patchline artifact-benchmark compare results/generated/artifact-benchmark/public-migrations-report.json benchmarks/expected/public-migrations-report.json

artifact-benchmark-public-incidents:
	go run ./cmd/patchline artifact-benchmark validate benchmarks/manifests/public_incidents.json
	go run ./cmd/patchline artifact-benchmark run benchmarks/manifests/public_incidents.json --out results/generated/artifact-benchmark/public-incidents-report.json
	go run ./cmd/patchline artifact-benchmark compare results/generated/artifact-benchmark/public-incidents-report.json benchmarks/expected/public-incidents-report.json

artifact-benchmark-public-repairs:
	go run ./cmd/patchline artifact-benchmark validate benchmarks/manifests/public_repairs.json
	go run ./cmd/patchline artifact-benchmark run benchmarks/manifests/public_repairs.json --out results/generated/artifact-benchmark/public-repairs-report.json
	go run ./cmd/patchline artifact-benchmark compare results/generated/artifact-benchmark/public-repairs-report.json benchmarks/expected/public-repairs-report.json

artifact-benchmark-public-archive:
	go run ./cmd/patchline artifact-benchmark validate benchmarks/manifests/public_archive.json
	go run ./cmd/patchline artifact-benchmark run benchmarks/manifests/public_archive.json --out results/generated/artifact-benchmark/public-archive-report.json
	go run ./cmd/patchline artifact-benchmark compare results/generated/artifact-benchmark/public-archive-report.json benchmarks/expected/public-archive-report.json

artifact-benchmark-refresh:
	bash scripts/refresh-artifact-benchmarks.sh

artifact-benchmark-compare: artifact-benchmark artifact-benchmark-repairs artifact-benchmark-regressions
	go run ./cmd/patchline artifact-benchmark compare results/generated/artifact-benchmark/smoke-report.json benchmarks/expected/smoke-report.json
	go run ./cmd/patchline artifact-benchmark compare results/generated/artifact-benchmark/negative-report.json benchmarks/expected/negative-report.json
	go run ./cmd/patchline artifact-benchmark compare results/generated/artifact-benchmark/repair-cases-report.json benchmarks/expected/repair-cases-report.json
	go run ./cmd/patchline artifact-benchmark compare results/generated/artifact-benchmark/semantic-regressions-report.json benchmarks/expected/semantic-regressions-report.json

artifact-negative-cases:
	bash scripts/artifact_negative_cases.sh

artifact-full: artifact-smoke artifact-demo artifact-ground-truth-check phase-check artifact-studies-compare artifact-tables artifact-numbers artifact-subtasks artifact-corpus-audit artifact-provenance artifact-benchmark-compare artifact-benchmark-public-incidents artifact-benchmark-public-repairs artifact-benchmark-public-archive artifact-negative-cases

artifact-clean:
	rm -rf results/generated

fmt:
	gofmt -w cmd internal
