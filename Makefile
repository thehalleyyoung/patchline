.PHONY: build test demo gate fmt public-corpus verify-usefulness

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

fmt:
	gofmt -w cmd internal
