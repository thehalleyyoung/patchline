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

manifest_for() {
  local manifest_path="$1"
  local jsonl_path
  jsonl_path="$(mktemp)"
  find "$OUT" -type f \
    ! -name 'summary.json' \
    ! -name 'summary.md' \
    ! -name 'bundle-manifest.json' \
    -print0 |
    sort -z |
    while IFS= read -r -d '' file; do
      local rel sha bytes
      rel="${file#$OUT/}"
      sha="$(shasum -a 256 "$file" | awk '{print $1}')"
      bytes="$(wc -c < "$file" | tr -d ' ')"
      jq -n --arg path "$rel" --arg sha256 "$sha" --argjson bytes "$bytes" \
        '{path: $path, sha256: $sha256, bytes: $bytes}' >> "$jsonl_path"
    done
  jq -s '{files: ., file_count: length}' "$jsonl_path" > "$manifest_path"
  rm -f "$jsonl_path"
}

echo "Patchline artifact demo"
echo "expected_runtime=<5m"
echo "network=not-required"

go run ./cmd/patchline trace-reconstruct examples/incidents/bad-migration.jsonl --json > "$OUT/trace.json"
go run ./cmd/patchline analyze-migration demos/billing/migrations/002_bad_backfill.sql --json > "$OUT/migration.json"
go run ./cmd/patchline solver-obligations examples/repairs/repair-bad-invoice-backfill.json --invariants examples/invariants/billing-core.json --json > "$OUT/solver.json"
go run ./cmd/patchline repair-semantics examples/repairs/repair-bad-invoice-backfill.json --store examples/snapshots/billing-bad-migration-before.json --json > "$OUT/repair.json"
go run ./cmd/patchline archive-query examples/archive/bad-migration-corpus.json --json > "$OUT/archive.json"
go run ./cmd/patchline semantic-regressions examples/archive/bad-migration-corpus.json --json > "$OUT/regressions.json"

go run ./cmd/patchline phase-check benchmarks/manifests/public_archive.json --json > "$OUT/public-archive-phase-check.json"
go run ./cmd/patchline archive-index examples/archive/public-postmortem-derived-paired-archive.json --json > "$OUT/public-archive-index.json"
go run ./cmd/patchline archive-query examples/archive/public-postmortem-derived-paired-archive.json semantic-regressions --json > "$OUT/public-archive-query.json"
go run ./cmd/patchline semantic-regressions examples/archive/public-postmortem-derived-paired-archive.json --json > "$OUT/public-regressions.json"
go run ./cmd/patchline artifact-benchmark validate benchmarks/manifests/public_archive.json --json > "$OUT/public-archive-benchmark-validate.json"
go run ./cmd/patchline artifact-benchmark run benchmarks/manifests/public_archive.json --out "$OUT/public-archive-benchmark-report.json" --json > "$OUT/public-archive-benchmark-run.json"
go run ./cmd/patchline artifact-benchmark compare "$OUT/public-archive-benchmark-report.json" benchmarks/expected/public-archive-report.json --json > "$OUT/public-archive-benchmark-compare.json"

manifest_for "$OUT/bundle-manifest.json"
bundle_hash="$(shasum -a 256 "$OUT/bundle-manifest.json" | awk '{print $1}')"

jq -n \
  --arg claim "historical repair semantics detects recurrence and proves repair obligations across local and public-derived archives" \
  --arg bundle_hash "$bundle_hash" \
  --arg output_dir "$OUT" \
  --slurpfile trace "$OUT/trace.json" \
  --slurpfile migration "$OUT/migration.json" \
  --slurpfile solver "$OUT/solver.json" \
  --slurpfile repair "$OUT/repair.json" \
  --slurpfile regressions "$OUT/regressions.json" \
  --slurpfile public_archive "$OUT/public-archive-query.json" \
  --slurpfile public_benchmark "$OUT/public-archive-benchmark-report.json" \
  --slurpfile manifest "$OUT/bundle-manifest.json" \
  '{
    claim: $claim,
    artifact_bundle_hash: $bundle_hash,
    output_dir: $output_dir,
    bundle_file_count: $manifest[0].file_count,
    trace_projection_hash: $trace[0].projection_hash,
    migration_report_hash: $migration[0].summary.report_hash,
    solver_engine: $solver[0].solver_engine,
    solver_report_hash: $solver[0].hash,
    repair_trace_hash: $repair[0].hash,
    semantic_regression_count: ($regressions[0] | length),
    public_archive_relation_count: ($public_archive[0] | length),
    public_archive_benchmark_hash: $public_benchmark[0].hash,
    public_archive_benchmark_cases: ($public_benchmark[0].cases | length),
    public_archive_expected_result: $public_benchmark[0].cases[0].expected_result,
    public_archive_actual_result: $public_benchmark[0].cases[0].actual_result
  }' > "$OUT/summary.json"

{
  echo "# Patchline artifact demo summary"
  echo
  echo "- claim: historical repair semantics detects recurrence and proves repair obligations across local and public-derived archives"
  echo "- output_dir: \`$OUT\`"
  echo "- artifact_bundle_hash: \`$bundle_hash\`"
  echo "- bundle_file_count: \`$(jq -r '.bundle_file_count' "$OUT/summary.json")\`"
  echo "- trace_projection_hash: \`$(jq -r '.trace_projection_hash' "$OUT/summary.json")\`"
  echo "- migration_report_hash: \`$(jq -r '.migration_report_hash' "$OUT/summary.json")\`"
  echo "- solver_engine: \`$(jq -r '.solver_engine' "$OUT/summary.json")\`"
  echo "- solver_report_hash: \`$(jq -r '.solver_report_hash' "$OUT/summary.json")\`"
  echo "- repair_trace_hash: \`$(jq -r '.repair_trace_hash' "$OUT/summary.json")\`"
  echo "- semantic_regression_count: \`$(jq -r '.semantic_regression_count' "$OUT/summary.json")\`"
  echo "- public_archive_relation_count: \`$(jq -r '.public_archive_relation_count' "$OUT/summary.json")\`"
  echo "- public_archive_benchmark_hash: \`$(jq -r '.public_archive_benchmark_hash' "$OUT/summary.json")\`"
  echo "- public_archive_benchmark_cases: \`$(jq -r '.public_archive_benchmark_cases' "$OUT/summary.json")\`"
  echo "- public_archive_result: \`$(jq -r '.public_archive_actual_result' "$OUT/summary.json")\`"
} > "$OUT/summary.md"

cat "$OUT/summary.md"
