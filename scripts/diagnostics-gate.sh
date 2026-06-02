#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

OUT="${1:-results/generated/diagnostics-gate}"
rm -rf "$OUT"
mkdir -p "$OUT/cases" "$OUT/cache"

for term in \
  "repo analyze --trace" \
  "diagnostics/events.jsonl" \
  "diagnostics/summary.json" \
  "make diagnostics-gate"; do
  grep -F "$term" docs/diagnostics.md > /dev/null
done

run_case() {
  local id="$1"
  local repo="$2"
  local ref="$3"
  local subpath="$4"
  local case_out="$OUT/cases/$id"
  mkdir -p "$case_out"

  go run ./cmd/patchline repo analyze \
    --github "$repo" \
    --ref "$ref" \
    --subpath "$subpath" \
    --download-dir "$OUT/cache" \
    --stages inventory,baseline,propose,compare,deep \
    --proposal-kind all \
    --budget files=4,lines=80,tokens=12000,changes=2 \
    --no-llm \
    --trace \
    --out "$case_out/analyze" \
    --json > "$case_out/stdout.json"

  test -s "$case_out/analyze/diagnostics/events.jsonl"
  test -s "$case_out/analyze/diagnostics/summary.json"
  test -s "$case_out/analyze/analyze.json"
  test -s "$case_out/analyze/analyze.md"

  jq -e '
    .version == "patchline.diagnostics/v1" and
    .events > 0 and
    .spans >= 8 and
    .logs >= 2 and
    .failed_spans == 0 and
    .duration_ms >= 0 and
    (.events_path | endswith("diagnostics/events.jsonl")) and
    (.summary_path | endswith("diagnostics/summary.json")) and
    (.hash | length) > 0
  ' "$case_out/analyze/diagnostics/summary.json" > /dev/null

  for span in repo.analyze inventory intake baseline deep-summary proposal compare triage analysis-bundle; do
    jq -e --arg span "$span" 'select(.type == "span" and .name == $span and .status == "ok")' "$case_out/analyze/diagnostics/events.jsonl" > /dev/null
  done

  jq -e '
    .version == "patchline.repo-analyze/v1" and
    (.outputs.diagnostics | endswith("/diagnostics")) and
    .diagnostics.failed_spans == 0 and
    .diagnostics.spans >= 8 and
    .summary.files_scanned > 0 and
    .summary.ranked_risks > 0 and
    .summary.generated_files > 0 and
    (.hash | length) > 0
  ' "$case_out/analyze/analyze.json" > /dev/null

  grep -F "diagnostics" "$case_out/analyze/analyze.md" > /dev/null

  jq -n \
    --arg id "$id" \
    --arg repo "$repo" \
    --arg subpath "$subpath" \
    --slurpfile analyze "$case_out/analyze/analyze.json" \
    --slurpfile diagnostics "$case_out/analyze/diagnostics/summary.json" \
    '{
      id:$id,
      repo:$repo,
      subpath:$subpath,
      files_scanned:$analyze[0].summary.files_scanned,
      ranked_risks:$analyze[0].summary.ranked_risks,
      generated_files:$analyze[0].summary.generated_files,
      diagnostic_spans:$diagnostics[0].spans,
      diagnostic_logs:$diagnostics[0].logs,
      failed_spans:$diagnostics[0].failed_spans,
      hash:$diagnostics[0].hash
    }' > "$case_out/row.json"
}

run_case "forem" "forem/forem" "9c5509c3aeecd4a86a8950206fa937ebcbc2f8d1" "db/migrate"
run_case "bytebase" "bytebase/bytebase" "0765652ea2dbdf8e93ae44bff5acafc1b97a92cc" "backend/migrator/migration"
run_case "mastodon" "mastodon/mastodon" "facb552c9cdbe8a2ebff0b94ebf2c9e9ec385347" "db/migrate"
run_case "lobsters" "lobsters/lobsters" "3b80b47aa5aaba37ec44413e7d1dc96fcf1585b6" "db/migrate"

jq -s '{
  version:"patchline.diagnostics-gate-results/v1",
  runs: .,
  summary: {
    public_repos: (map(.repo) | unique | length),
    files_scanned: (map(.files_scanned) | add),
    ranked_risks: (map(.ranked_risks) | add),
    generated_files: (map(.generated_files) | add),
    diagnostic_spans: (map(.diagnostic_spans) | add),
    diagnostic_logs: (map(.diagnostic_logs) | add),
    failed_spans: (map(.failed_spans) | add)
  }
}' "$OUT"/cases/*/row.json > "$OUT/summary.json"

jq -e '
  .version == "patchline.diagnostics-gate-results/v1" and
  .summary.public_repos >= 4 and
  .summary.files_scanned > 0 and
  .summary.ranked_risks > 0 and
  .summary.generated_files > 0 and
  .summary.diagnostic_spans >= 32 and
  .summary.diagnostic_logs >= 8 and
  .summary.failed_spans == 0
' "$OUT/summary.json" > /dev/null

echo "diagnostics gate passed: $(jq '.summary.public_repos' "$OUT/summary.json") public repos, $(jq '.summary.diagnostic_spans' "$OUT/summary.json") spans"
