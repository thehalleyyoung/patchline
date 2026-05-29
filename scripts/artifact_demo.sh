#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

if ! command -v jq >/dev/null 2>&1; then
  echo "artifact-demo requires jq" >&2
  exit 1
fi

OUT="results/generated/artifact-demo"
rm -rf "$OUT"
mkdir -p "$OUT"

echo "Patchline artifact demo"
echo "expected_runtime=<5m"
echo "network=not-required"

go run ./cmd/patchline trace-reconstruct examples/incidents/bad-migration.jsonl --json > "$OUT/trace.json"
go run ./cmd/patchline analyze-migration demos/billing/migrations/002_bad_backfill.sql --json > "$OUT/migration.json"
go run ./cmd/patchline solver-obligations examples/repairs/repair-bad-invoice-backfill.json --invariants examples/invariants/billing-core.json --json > "$OUT/solver.json"
go run ./cmd/patchline repair-semantics examples/repairs/repair-bad-invoice-backfill.json --store examples/snapshots/billing-bad-migration-before.json --json > "$OUT/repair.json"
go run ./cmd/patchline archive-query examples/archive/bad-migration-corpus.json --json > "$OUT/archive.json"
go run ./cmd/patchline semantic-regressions examples/archive/bad-migration-corpus.json --json > "$OUT/regressions.json"

bundle_hash="$(find "$OUT" -type f -name '*.json' -print0 | sort -z | xargs -0 shasum -a 256 | shasum -a 256 | awk '{print $1}')"

jq -n \
  --arg claim "historical repair semantics detects recurrence and proves repair obligations" \
  --arg bundle_hash "$bundle_hash" \
  --arg output_dir "$OUT" \
  --slurpfile trace "$OUT/trace.json" \
  --slurpfile migration "$OUT/migration.json" \
  --slurpfile solver "$OUT/solver.json" \
  --slurpfile repair "$OUT/repair.json" \
  --slurpfile regressions "$OUT/regressions.json" \
  '{
    claim: $claim,
    artifact_bundle_hash: $bundle_hash,
    output_dir: $output_dir,
    trace_projection_hash: $trace[0].projection_hash,
    migration_report_hash: $migration[0].summary.report_hash,
    solver_engine: $solver[0].solver_engine,
    solver_report_hash: $solver[0].hash,
    repair_trace_hash: $repair[0].hash,
    semantic_regression_count: ($regressions[0] | length)
  }' > "$OUT/summary.json"

{
  echo "# Patchline artifact demo summary"
  echo
  echo "- claim: historical repair semantics detects recurrence and proves repair obligations"
  echo "- output_dir: \`$OUT\`"
  echo "- artifact_bundle_hash: \`$bundle_hash\`"
  echo "- trace_projection_hash: \`$(jq -r '.trace_projection_hash' "$OUT/summary.json")\`"
  echo "- migration_report_hash: \`$(jq -r '.migration_report_hash' "$OUT/summary.json")\`"
  echo "- solver_engine: \`$(jq -r '.solver_engine' "$OUT/summary.json")\`"
  echo "- solver_report_hash: \`$(jq -r '.solver_report_hash' "$OUT/summary.json")\`"
  echo "- repair_trace_hash: \`$(jq -r '.repair_trace_hash' "$OUT/summary.json")\`"
  echo "- semantic_regression_count: \`$(jq -r '.semantic_regression_count' "$OUT/summary.json")\`"
} > "$OUT/summary.md"

cat "$OUT/summary.md"
