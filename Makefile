.PHONY: build test demo gate fmt

build:
	go build -o bin/patchline ./cmd/patchline

test:
	go test ./...

demo:
	go run ./cmd/patchline dry-run examples/repairs/repair-bad-invoice-backfill.json --json

gate:
	go run ./cmd/patchline ci-gate examples/benchmarks/strict-migration-corpus.json --min-precision 0.95 --min-recall 0.95

fmt:
	gofmt -w cmd internal
