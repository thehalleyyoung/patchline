#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/secret-scan-gate.json}"
OUT="${2:-results/generated/secret-scan-gate}"
rm -rf "$OUT"
mkdir -p "$OUT/fixture/db/migrate" "$OUT/fixture/docs" "$OUT/cache" "$OUT/public"

jq -e '
  .version == "patchline.secret-scan-gate/v1" and
  (.claim | length) > 60 and
  (.canaries | length) >= 3 and
  (.required_artifacts | length) >= 8 and
  (.real_code.repo | length) > 0 and
  (.real_code.ref | test("^[0-9a-f]{40}$")) and
  (.real_code.subpath | length) > 0
' "$SPEC" > /dev/null

for term in \
  "make secret-scan-gate" \
  "redacted-artifacts" \
  "diagnostics logs" \
  "prompts" \
  "Lobsters"; do
  grep -F "$term" docs/secret-scanning.md > /dev/null
done

cat > "$OUT/fixture/db/migrate/001_canary_backfill.sql" <<'SQL'
-- public test canary: PATCHLINE_LEAK_CANARY_ALPHA
UPDATE accounts
SET api_token = 'PATCHLINE_LEAK_CANARY_BETA',
    owner_email = 'patchline_canary@example.invalid'
WHERE id = 42;
SQL

cat > "$OUT/fixture/docs/incident.md" <<'MD'
# Canary incident note

The rollback note mentions PATCHLINE_LEAK_CANARY_ALPHA and patchline_canary@example.invalid so reports, prompts, logs, and generated artifacts must be redacted before sharing.
MD

go test ./cmd/patchline -run 'TestBundleRedactorRemovesCanaryValues|TestRepoAnalyzeTraceWritesDiagnostics' > "$OUT/go-test.log"

go run ./cmd/patchline repo analyze "$OUT/fixture" \
  --stages inventory,baseline,propose,compare \
  --proposal-kind all \
  --budget files=4,lines=80,tokens=12000,changes=2 \
  --no-llm \
  --redact \
  --trace \
  --ci \
  --out "$OUT/analyze" \
  --json > "$OUT/stdout.json"

jq -e '
  .version == "patchline.repo-analyze/v1" and
  .redact == true and
  (.outputs.redacted_artifacts | endswith("/redacted-artifacts")) and
  .summary.files_scanned > 0 and
  .summary.ranked_risks > 0 and
  .summary.generated_files > 0
' "$OUT/analyze/analyze.json" > /dev/null

while IFS= read -r artifact; do
  test -s "$OUT/analyze/$artifact"
done < <(jq -r '.required_artifacts[]' "$SPEC")

scan_dir_for_canaries() {
  local dir="$1"
  local label="$2"
  if rg -n -F -f <(jq -r '.canaries[]' "$SPEC") "$dir" > "$OUT/$label-leaks.txt"; then
    echo "canary leak detected in $label" >&2
    cat "$OUT/$label-leaks.txt" >&2
    exit 1
  fi
}

scan_dir_for_canaries "$OUT/analyze/redacted-artifacts" "redacted-artifacts"
scan_dir_for_canaries "$OUT/analyze/analysis-bundle" "analysis-bundle"

jq -e '
  .version == "patchline.diagnostics/v1" and
  .failed_spans == 0 and
  .spans > 0 and
  .logs > 0
' "$OUT/analyze/diagnostics/summary.json" > /dev/null

read -r repo ref subpath < <(jq -r '[.real_code.repo, .real_code.ref, .real_code.subpath] | @tsv' "$SPEC")
go run ./cmd/patchline repo analyze \
  --github "$repo" \
  --ref "$ref" \
  --subpath "$subpath" \
  --download-dir "$OUT/cache" \
  --stages inventory,baseline,propose,compare \
  --proposal-kind all \
  --budget files=4,lines=80,tokens=12000,changes=2 \
  --no-llm \
  --redact \
  --out "$OUT/public/analyze" \
  --json > "$OUT/public/stdout.json"

jq -e '.summary.files_scanned > 0 and .summary.ranked_risks > 0 and .summary.generated_files > 0 and (.outputs.redacted_artifacts | length) > 0' "$OUT/public/analyze/analyze.json" > /dev/null
scan_dir_for_canaries "$OUT/public/analyze/redacted-artifacts" "public-redacted-artifacts"

jq -n \
  --slurpfile local "$OUT/analyze/analyze.json" \
  --slurpfile public "$OUT/public/analyze/analyze.json" \
  --slurpfile spec "$SPEC" \
  '{
    version:"patchline.secret-scan-gate-results/v1",
    canaries:$spec[0].canaries,
    local:{
      files_scanned:$local[0].summary.files_scanned,
      ranked_risks:$local[0].summary.ranked_risks,
      generated_files:$local[0].summary.generated_files,
      redacted_artifacts:$local[0].outputs.redacted_artifacts
    },
    public:{
      repo:$spec[0].real_code.repo,
      files_scanned:$public[0].summary.files_scanned,
      ranked_risks:$public[0].summary.ranked_risks,
      generated_files:$public[0].summary.generated_files
    },
    surfaces:["reports","prompts","bundles","generated-code","diagnostics-logs","ci","redacted-artifacts"],
    leaks:0,
    verified:true
  }' > "$OUT/summary.json"

jq -e '.verified == true and .leaks == 0 and .local.ranked_risks > 0 and .public.ranked_risks > 0 and (.surfaces | length) >= 7' "$OUT/summary.json" > /dev/null

echo "secret scan gate passed: 0 leaks across $(jq '.surfaces | length' "$OUT/summary.json") surfaces, public risks $(jq '.public.ranked_risks' "$OUT/summary.json")"
