#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

OUT="results/generated/artifact-smoke"
rm -rf "$OUT"
mkdir -p "$OUT"

echo "Patchline artifact smoke"
echo "expected_runtime=<5m"
echo "network=not-required"

go test ./...
go run ./cmd/patchline artifact-ground-truth benchmarks --json > "$OUT/artifact-ground-truth.json"
bash scripts/validate-ground-truth.sh | tee "$OUT/ground-truth.txt"
bash scripts/check-artifact-targets.sh | tee "$OUT/commands.txt"

go run ./cmd/patchline semantics-audit --json > "$OUT/semantics-audit.json"
go run ./cmd/patchline trace-reconstruct examples/incidents/bad-migration.jsonl --json > "$OUT/trace.json"
go run ./cmd/patchline analyze-migration demos/billing/migrations/002_bad_backfill.sql --json > "$OUT/migration.json"
go run ./cmd/patchline solver-obligations examples/repairs/repair-bad-invoice-backfill.json --invariants examples/invariants/billing-core.json --json > "$OUT/solver.json"
go run ./cmd/patchline repair-semantics examples/repairs/repair-bad-invoice-backfill.json --store examples/snapshots/billing-bad-migration-before.json --json > "$OUT/repair.json"
go run ./cmd/patchline archive-query examples/archive/bad-migration-corpus.json semantic-regressions --json > "$OUT/archive-regressions.json"
go run ./cmd/patchline semantic-regressions examples/archive/bad-migration-corpus.json --json > "$OUT/semantic-regressions.json"
go run ./cmd/patchline benchmark-suite examples/benchmarks/strict-migration-corpus.json --json > "$OUT/benchmark-suite.json"

echo "artifact_smoke_outputs=$OUT"
