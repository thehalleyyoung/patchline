.PHONY: build test demo gate fmt public-corpus verify-usefulness artifact-smoke artifact-demo artifact-ground-truth-check artifact-baselines artifact-ablations artifact-scale artifact-studies artifact-benchmark artifact-benchmark-public artifact-benchmark-public-incidents artifact-benchmark-refresh artifact-benchmark-compare artifact-negative-cases artifact-full artifact-clean

build:
	go build -o bin/patchline ./cmd/patchline

test:
	go test ./...

demo:
	go run ./cmd/patchline dry-run examples/repairs/repair-bad-invoice-backfill.json --json

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

artifact-baselines:
	go run ./cmd/patchline artifact-baselines examples/benchmarks/strict-migration-corpus.json --out results/generated/artifact-studies

artifact-ablations:
	go run ./cmd/patchline artifact-ablations examples/benchmarks/strict-migration-corpus.json --out results/generated/artifact-studies

artifact-scale:
	go run ./cmd/patchline artifact-scale examples/benchmarks/strict-migration-corpus.json --out results/generated/artifact-studies

artifact-studies: artifact-baselines artifact-ablations artifact-scale

artifact-benchmark:
	go run ./cmd/patchline artifact-benchmark validate benchmarks/manifests/smoke.json
	go run ./cmd/patchline artifact-benchmark run benchmarks/manifests/smoke.json --out results/generated/artifact-benchmark/smoke-report.json
	go run ./cmd/patchline artifact-benchmark validate benchmarks/manifests/negative.json
	go run ./cmd/patchline artifact-benchmark run benchmarks/manifests/negative.json --out results/generated/artifact-benchmark/negative-report.json

artifact-benchmark-public: public-corpus
	go run ./cmd/patchline artifact-benchmark validate benchmarks/manifests/public_migrations.json
	go run ./cmd/patchline artifact-benchmark run benchmarks/manifests/public_migrations.json --out results/generated/artifact-benchmark/public-migrations-report.json
	go run ./cmd/patchline artifact-benchmark compare results/generated/artifact-benchmark/public-migrations-report.json benchmarks/expected/public-migrations-report.json

artifact-benchmark-public-incidents:
	go run ./cmd/patchline artifact-benchmark validate benchmarks/manifests/public_incidents.json
	go run ./cmd/patchline artifact-benchmark run benchmarks/manifests/public_incidents.json --out results/generated/artifact-benchmark/public-incidents-report.json
	go run ./cmd/patchline artifact-benchmark compare results/generated/artifact-benchmark/public-incidents-report.json benchmarks/expected/public-incidents-report.json

artifact-benchmark-refresh:
	bash scripts/refresh-artifact-benchmarks.sh
	$(MAKE) artifact-benchmark-compare
	$(MAKE) artifact-benchmark-public
	$(MAKE) artifact-benchmark-public-incidents

artifact-benchmark-compare: artifact-benchmark
	go run ./cmd/patchline artifact-benchmark compare results/generated/artifact-benchmark/smoke-report.json benchmarks/expected/smoke-report.json
	go run ./cmd/patchline artifact-benchmark compare results/generated/artifact-benchmark/negative-report.json benchmarks/expected/negative-report.json

artifact-negative-cases:
	bash scripts/artifact_negative_cases.sh

artifact-full: artifact-smoke artifact-demo artifact-studies artifact-benchmark-compare artifact-benchmark-public-incidents artifact-negative-cases verify-usefulness

artifact-clean:
	rm -rf results/generated

fmt:
	gofmt -w cmd internal
