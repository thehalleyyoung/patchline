#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/online-schema-change-adapters-gate.json}"
OUT="${2:-results/generated/online-schema-change-adapters}"
rm -rf "$OUT"
mkdir -p "$OUT/cases" "$OUT/reports"

jq -e '.version == "patchline.online-schema-change-adapters-gate/v1" and (.claim|length) > 200 and (.cases|length) >= .minimum_cases' "$SPEC" > /dev/null

for phrase in "online-schema-change adapters" "pt-online-schema-change" "gh-ost" "make online-schema-change-adapters-gate"; do
  grep -F "$phrase" docs/online-schema-change-adapters.md README.md > /dev/null
done

go test ./internal/dbsemantics -run TestOnlineSchemaChangeAdaptersCoverToolsNativeAndFrameworks -count=1 > "$OUT/go-test.log"

rows=()
while IFS= read -r encoded; do
  case_json="$(printf '%s' "$encoded" | base64 --decode)"
  id="$(jq -r '.id' <<<"$case_json")"
  engine="$(jq -r '.engine' <<<"$case_json")"
  version="$(jq -r '.version' <<<"$case_json")"
  case_sql="$OUT/cases/$id.sql"
  report="$OUT/reports/$id.json"
  jq -r '.sql' <<<"$case_json" > "$case_sql"

  go run ./cmd/patchline db-semantics --engine "$engine" --version "$version" --sql "$case_sql" --out "$report" --json > "$OUT/reports/$id.stdout.json"

  jq -e \
    --arg adapter "$(jq -r '.expect.adapter' <<<"$case_json")" \
    --arg mode "$(jq -r '.expect.mode' <<<"$case_json")" \
    --arg duration "$(jq -r '.expect.duration_class' <<<"$case_json")" \
    --argjson shadow "$(jq -r '.expect.uses_shadow_table' <<<"$case_json")" \
    --argjson triggers "$(jq -r '.expect.uses_triggers' <<<"$case_json")" \
    --argjson binlog "$(jq -r '.expect.uses_binlog' <<<"$case_json")" \
    '.summary.online_schema_change_adapters == 1 and
     .statements[0].online_schema_change.adapter == $adapter and
     .statements[0].online_schema_change.uses_shadow_table == $shadow and
     .statements[0].online_schema_change.uses_triggers == $triggers and
     .statements[0].online_schema_change.uses_binlog == $binlog and
     (.statements[0].online_schema_change.evidence|length) >= 1 and
     (.statements[0].online_schema_change.obligations|length) >= 1 and
     .statements[0].lock_simulation.mode == $mode and
     .statements[0].lock_simulation.duration_class == $duration and
     .statements[0].lock_simulation.online == true and
     .statements[0].lock_simulation.blocks_writers == false' \
    "$report" > /dev/null

  jq -n \
    --arg id "$id" \
    --arg engine "$engine" \
    --slurpfile report "$report" \
    '{id:$id, engine:$engine, adapter:$report[0].statements[0].online_schema_change.adapter, mode:$report[0].statements[0].lock_simulation.mode, obligations:($report[0].statements[0].online_schema_change.obligations|length), evidence:($report[0].statements[0].online_schema_change.evidence|length), verified:true}' \
    > "$OUT/reports/$id.row.json"
  rows+=("$OUT/reports/$id.row.json")
done < <(jq -r '.cases[] | @base64' "$SPEC")

jq -n \
  --slurpfile spec "$SPEC" \
  --slurpfile rows <(jq -s '.' "${rows[@]}") \
  '{
    version:"patchline.online-schema-change-adapters-results/v1",
    claim:$spec[0].claim,
    cases:$rows[0],
    summary:{
      cases:($rows[0]|length),
      adapters:($rows[0] | map(.adapter) | unique | length),
      verified:($rows[0] | map(select(.verified == true)) | length),
      total_obligations:($rows[0] | map(.obligations) | add),
      total_evidence_refs:($rows[0] | map(.evidence) | add)
    }
  }' > "$OUT/online-schema-change-adapters.json"

jq -e --slurpfile spec "$SPEC" '
  .summary.cases >= $spec[0].minimum_cases and
  .summary.adapters == .summary.cases and
  .summary.verified == .summary.cases and
  .summary.total_obligations >= .summary.cases and
  .summary.total_evidence_refs >= .summary.cases
' "$OUT/online-schema-change-adapters.json" > /dev/null

{
  echo "# Online-schema-change adapters"
  echo
  echo "| Case | Engine | Adapter | Lock mode |"
  echo "|---|---|---|---|"
  jq -r '.cases[] | "| \(.id) | \(.engine) | \(.adapter) | \(.mode) |"' "$OUT/online-schema-change-adapters.json"
} > "$OUT/online-schema-change-adapters.md"
cp "$OUT/online-schema-change-adapters.md" "$OUT/README.md"

echo "online-schema-change adapters gate passed: $(jq -r '.summary.adapters' "$OUT/online-schema-change-adapters.json") adapters verified"
