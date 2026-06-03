#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/replication-lag-risk-gate.json}"
OUT="${2:-results/generated/replication-lag-risk}"
rm -rf "$OUT"
mkdir -p "$OUT/cases" "$OUT/reports"

jq -e '.version == "patchline.replication-lag-risk-gate/v1" and (.claim|length) > 200 and (.cases|length) >= .minimum_risks' "$SPEC" > /dev/null

for phrase in "replication-lag risk" "read replica" "CDC" "event-stream" "make replication-lag-risk-gate"; do
  grep -F "$phrase" docs/replication-lag-risk.md README.md > /dev/null
done

go test ./internal/dbsemantics ./cmd/patchline -run 'Test(ReplicationLagRisk|DBSemanticsCommandWritesVersionedReport)' -count=1 > "$OUT/go-test.log"

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

  expect_risk="$(jq -r '.expect.risk' <<<"$case_json")"
  if [[ "$expect_risk" == "true" ]]; then
    jq -e \
      --arg shape "$(jq -r '.expect.shape' <<<"$case_json")" \
      --arg class "$(jq -r '.expect.class' <<<"$case_json")" \
      --arg pipeline "$(jq -r '.expect.pipeline' <<<"$case_json")" \
      --argjson mitigation "$(jq -r '.expect.mitigation' <<<"$case_json")" \
      '.summary.replication_lag_risks == 1 and
       .statements[0].replication_lag_risk.migration_shape == $shape and
       .statements[0].replication_lag_risk.class == $class and
       (.statements[0].replication_lag_risk.conditional_pipelines | index($pipeline)) and
       (.statements[0].replication_lag_risk.hazards|length) >= 2 and
       (.statements[0].replication_lag_risk.evidence|length) >= 1 and
       (.statements[0].replication_lag_risk.obligations|length) >= 3 and
       (if $mitigation then (.statements[0].replication_lag_risk.mitigations|length) >= 1 else true end) and
       any(.statements[0].rules[]?; .id == ("replication_lag." + $shape) and .verdict == "conditional")' \
      "$report" > /dev/null
  else
    jq -e '(.summary.replication_lag_risks // 0) == 0 and (.statements[0].replication_lag_risk? == null)' "$report" > /dev/null
  fi

  jq -n \
    --arg id "$id" \
    --arg engine "$engine" \
    --slurpfile report "$report" \
    '{
      id:$id,
      engine:$engine,
      risk:(($report[0].summary.replication_lag_risks // 0) == 1),
      shape:($report[0].statements[0].replication_lag_risk.migration_shape // "none"),
      pipelines:($report[0].statements[0].replication_lag_risk.conditional_pipelines // []),
      obligations:($report[0].statements[0].replication_lag_risk.obligations // [] | length),
      verified:true
    }' > "$OUT/reports/$id.row.json"
  rows+=("$OUT/reports/$id.row.json")
done < <(jq -r '.cases[] | @base64' "$SPEC")

jq -n \
  --slurpfile spec "$SPEC" \
  --slurpfile rows <(jq -s '.' "${rows[@]}") \
  '{
    version:"patchline.replication-lag-risk-results/v1",
    claim:$spec[0].claim,
    cases:$rows[0],
    summary:{
      cases:($rows[0]|length),
      risks:($rows[0] | map(select(.risk == true)) | length),
      controls:($rows[0] | map(select(.risk == false)) | length),
      verified:($rows[0] | map(select(.verified == true)) | length),
      pipelines:($rows[0] | map(.pipelines[]) | unique)
    }
  }' > "$OUT/replication-lag-risk.json"

jq -e --slurpfile spec "$SPEC" '
  .summary.risks >= $spec[0].minimum_risks and
  .summary.controls >= 1 and
  .summary.verified == .summary.cases and
  (.summary.pipelines | index("read_replica")) and
  (.summary.pipelines | index("cdc")) and
  (.summary.pipelines | index("event_stream"))
' "$OUT/replication-lag-risk.json" > /dev/null

{
  echo "# Replication-lag risk"
  echo
  echo "| Case | Engine | Shape | Conditional pipelines |"
  echo "|---|---|---|---|"
  jq -r '.cases[] | "| \(.id) | \(.engine) | \(.shape) | \(.pipelines | join(", ")) |"' "$OUT/replication-lag-risk.json"
} > "$OUT/replication-lag-risk.md"
cp "$OUT/replication-lag-risk.md" "$OUT/README.md"

echo "replication-lag risk gate passed: $(jq -r '.summary.risks' "$OUT/replication-lag-risk.json") risks, $(jq -r '.summary.controls' "$OUT/replication-lag-risk.json") control"
