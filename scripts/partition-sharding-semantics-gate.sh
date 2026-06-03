#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/partition-sharding-semantics-gate.json}"
OUT="${2:-results/generated/partition-sharding-semantics}"
rm -rf "$OUT"
mkdir -p "$OUT/cases" "$OUT/reports"

jq -e '.version == "patchline.partition-sharding-semantics-gate/v1" and (.claim|length) > 200 and (.cases|length) >= .minimum_findings' "$SPEC" > /dev/null

for phrase in "partitioning and sharding semantics" "tenant routing" "partition swap" "rebalancing operations" "make partition-sharding-semantics-gate"; do
  grep -F "$phrase" docs/partition-sharding-semantics.md README.md > /dev/null
done

go test ./internal/dbsemantics ./cmd/patchline -run 'Test(PartitionSharding|DBSemanticsCommandWritesPartitionSharding)' -count=1 > "$OUT/go-test.log"

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

  expect_finding="$(jq -r '.expect.finding' <<<"$case_json")"
  if [[ "$expect_finding" == "true" ]]; then
    jq -e \
      --arg operation "$(jq -r '.expect.operation' <<<"$case_json")" \
      --arg class "$(jq -r '.expect.class' <<<"$case_json")" \
      --arg table "$(jq -r '.expect.table' <<<"$case_json")" \
      --arg scope "$(jq -r '.expect.scope' <<<"$case_json")" \
      --arg partition "$(jq -r '.expect.partition // ""' <<<"$case_json")" \
      --arg tenant_key "$(jq -r '.expect.tenant_key // ""' <<<"$case_json")" \
      --arg target "$(jq -r '.expect.target // ""' <<<"$case_json")" \
      --argjson requires_rebalance "$(jq -r '.expect.requires_rebalance_backfill // false' <<<"$case_json")" \
      --argjson mitigation "$(jq -r '.expect.mitigation // false' <<<"$case_json")" \
      '.summary.partition_sharding_findings == 1 and
       .statements[0].partition_sharding.operation == $operation and
       .statements[0].partition_sharding.class == $class and
       .statements[0].partition_sharding.table == $table and
       .statements[0].partition_sharding.affected_scope == $scope and
       (if $partition != "" then .statements[0].partition_sharding.partition == $partition else true end) and
       (if $tenant_key != "" then .statements[0].partition_sharding.tenant_key == $tenant_key else true end) and
       (if $target != "" then .statements[0].partition_sharding.target_object == $target else true end) and
       (if $requires_rebalance then .statements[0].partition_sharding.requires_rebalance_backfill == true else true end) and
       (if $mitigation then (.statements[0].partition_sharding.mitigations|length) >= 1 else true end) and
       (.statements[0].partition_sharding.hazards|length) >= 3 and
       (.statements[0].partition_sharding.evidence|length) >= 2 and
       (.statements[0].partition_sharding.obligations|length) >= 4 and
       any(.statements[0].rules[]?; .id == ("partition_sharding." + $operation) and .verdict == "conditional")' \
      "$report" > /dev/null
  else
    jq -e '(.summary.partition_sharding_findings // 0) == 0 and (.statements[0].partition_sharding? == null)' "$report" > /dev/null
  fi

  jq -n \
    --arg id "$id" \
    --arg engine "$engine" \
    --slurpfile report "$report" \
    '{
      id:$id,
      engine:$engine,
      finding:(($report[0].summary.partition_sharding_findings // 0) == 1),
      operation:($report[0].statements[0].partition_sharding.operation // "none"),
      scope:($report[0].statements[0].partition_sharding.affected_scope // "none"),
      table:($report[0].statements[0].partition_sharding.table // $report[0].statements[0].table // "none"),
      obligations:($report[0].statements[0].partition_sharding.obligations // [] | length),
      verified:true
    }' > "$OUT/reports/$id.row.json"
  rows+=("$OUT/reports/$id.row.json")
done < <(jq -r '.cases[] | @base64' "$SPEC")

jq -n \
  --slurpfile spec "$SPEC" \
  --slurpfile rows <(jq -s '.' "${rows[@]}") \
  '{
    version:"patchline.partition-sharding-semantics-results/v1",
    claim:$spec[0].claim,
    cases:$rows[0],
    summary:{
      cases:($rows[0]|length),
      findings:($rows[0] | map(select(.finding == true)) | length),
      controls:($rows[0] | map(select(.finding == false)) | length),
      verified:($rows[0] | map(select(.verified == true)) | length),
      operations:($rows[0] | map(select(.finding == true) | .operation) | unique)
    }
  }' > "$OUT/partition-sharding-semantics.json"

jq -e --slurpfile spec "$SPEC" '
  .summary.findings >= $spec[0].minimum_findings and
  .summary.controls >= 1 and
  .summary.verified == .summary.cases and
  (.summary.operations | index("tenant_routing")) and
  (.summary.operations | index("partition_exchange")) and
  (.summary.operations | index("partition_switch")) and
  (.summary.operations | index("partition_replace")) and
  (.summary.operations | index("rebalance"))
' "$OUT/partition-sharding-semantics.json" > /dev/null

{
  echo "# Partitioning and sharding semantics"
  echo
  echo "| Case | Engine | Operation | Scope | Obligations |"
  echo "|---|---|---|---|---:|"
  jq -r '.cases[] | "| \(.id) | \(.engine) | \(.operation) | \(.scope) | \(.obligations) |"' "$OUT/partition-sharding-semantics.json"
} > "$OUT/partition-sharding-semantics.md"
cp "$OUT/partition-sharding-semantics.md" "$OUT/README.md"

echo "partition-sharding semantics gate passed: $(jq -r '.summary.findings' "$OUT/partition-sharding-semantics.json") findings, $(jq -r '.summary.controls' "$OUT/partition-sharding-semantics.json") control"
