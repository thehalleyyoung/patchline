#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

GATES="${1:-examples/recurrence-gates.json}"
OUT="${2:-results/generated/recurrence-gate}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.recurrence-gates/v1" and
  (.gates | length) >= 1 and
  all(.gates[];
    (.real_repos | length) >= 4 and
    (.recurrence_claim | contains("without emitting source paths"))
  )
' "$GATES" > /dev/null

analyses=()
while IFS=$'\t' read -r id repo ref subpath; do
  case_out="$OUT/$id"
  mkdir -p "$case_out"
  go run ./cmd/patchline repo analyze --github "$repo" --ref "$ref" --subpath "$subpath" --stages inventory,baseline --no-llm --out "$case_out/analysis" --json > "$case_out/analysis.json"
  analyses+=("$case_out/analysis")
done < <(jq -r '.slices[] | [.id, .repo, .ref, .subpath] | @tsv' examples/real-repo-slices.json)

analysis_csv="$(IFS=,; echo "${analyses[*]}")"
go run ./cmd/patchline repo recurrence --analyses "$analysis_csv" --out "$OUT/recurrence" --json > "$OUT/recurrence.json"
test -s "$OUT/recurrence/recurrence.md"

jq -e '
  .version == "patchline.repo-recurrence/v1" and
  .summary.analyses >= 4 and
  .summary.unrelated_projects >= 4 and
  .summary.risks > 0 and
  .summary.repeated > 0 and
  .summary.redacted_fields >= 3 and
  (.recurrences | length) > 0 and
  all(.recurrences[]; .project_count >= 2 and (.signature | startswith("recurrence:"))) and
  (.. | objects | has("path") | not) and
  (.. | objects | has("table") | not)
' "$OUT/recurrence.json" > /dev/null

jq -n \
  --slurpfile report "$OUT/recurrence.json" \
  --slurpfile gates "$GATES" \
  '{
    version:"patchline.recurrence-gate-results/v1",
    claim:$gates[0].gates[0].recurrence_claim,
    analyses:$report[0].summary.analyses,
    repeated:$report[0].summary.repeated,
    unrelated_projects:$report[0].summary.unrelated_projects,
    redacted_fields:$report[0].summary.redacted_fields,
    verified:true
  }' > "$OUT/summary.json"

echo "recurrence gate passed: $(jq '.repeated' "$OUT/summary.json") repeated redacted signatures across $(jq '.unrelated_projects' "$OUT/summary.json") projects"
