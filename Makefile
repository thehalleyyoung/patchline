.PHONY: build test demo intake-demo plug-and-play-demo repo-demo four-repo-demo repo-slice-matrix gate fmt public-corpus verify-usefulness artifact-smoke artifact-demo artifact-ground-truth-check phase-check artifact-baselines artifact-ablations artifact-scale artifact-studies artifact-studies-expected artifact-studies-compare artifact-studies-refresh artifact-baselines-public artifact-ablations-public artifact-scale-public artifact-studies-public artifact-studies-public-expected artifact-studies-public-compare artifact-studies-all artifact-tables artifact-numbers artifact-subtasks artifact-corpus-audit artifact-provenance artifact-benchmark artifact-benchmark-repairs artifact-benchmark-regressions artifact-benchmark-public artifact-benchmark-public-incidents artifact-benchmark-public-repairs artifact-benchmark-public-archive artifact-benchmark-refresh artifact-benchmark-compare artifact-negative-cases artifact-full artifact-clean

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
