#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/db-rollback-feasibility-gate.json}"
OUT="${2:-results/generated/db-rollback-feasibility}"
rm -rf "$OUT"
mkdir -p "$OUT/cases" "$OUT/reports"

jq -e '.version == "patchline.db-rollback-feasibility-gate/v1" and (.claim|length) > 200 and (.cases|length) >= .minimum_findings' "$SPEC" > /dev/null

for phrase in "rollback feasibility" "implicit commits" "irreversible metadata" "make db-rollback-feasibility-gate"; do
  grep -F "$phrase" docs/db-version-semantics.md README.md > /dev/null
done

go test ./internal/dbsemantics ./cmd/patchline -run 'Test(RollbackFeasibility|DBSemanticsCommandWritesRollbackFeasibility)' -count=1 > "$OUT/go-test.log"

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
      --arg class "$(jq -r '.expect.class' <<<"$case_json")" \
      --arg status "$(jq -r '.expect.status' <<<"$case_json")" \
      --argjson feasible "$(jq -r '.expect.feasible' <<<"$case_json")" \
      --argjson transactional "$(jq -r '.expect.transactional' <<<"$case_json")" \
      --argjson implicit_commit "$(jq -r '.expect.implicit_commit' <<<"$case_json")" \
      --argjson irreversible_metadata "$(jq -r '.expect.irreversible_metadata' <<<"$case_json")" \
      --argjson before_image "$(jq -r '.expect.requires_before_image' <<<"$case_json")" \
      --argjson snapshot "$(jq -r '.expect.requires_snapshot' <<<"$case_json")" \
      --argjson time_travel "$(jq -r '.expect.time_travel' <<<"$case_json")" \
      '.summary.rollback_feasibility_checks == 1 and
       .statements[0].rollback_feasibility.class == $class and
       .statements[0].rollback_feasibility.status == $status and
       .statements[0].rollback_feasibility.feasible == $feasible and
       .statements[0].rollback_feasibility.transactional_rollback == $transactional and
       .statements[0].rollback_feasibility.implicit_commit == $implicit_commit and
       .statements[0].rollback_feasibility.irreversible_metadata == $irreversible_metadata and
       .statements[0].rollback_feasibility.requires_before_image == $before_image and
       .statements[0].rollback_feasibility.requires_snapshot == $snapshot and
       .statements[0].rollback_feasibility.time_travel_eligible == $time_travel and
       (.statements[0].rollback_feasibility.recovery_mechanism|length) > 0 and
       (.statements[0].rollback_feasibility.evidence|length) >= 1 and
       (.statements[0].rollback_feasibility.obligations|length) >= 2 and
       (.statements[0].rollback_feasibility.failure_modes|length) >= 1 and
       any(.statements[0].rules[]?; .id == ("rollback." + $class))' \
      "$report" > /dev/null
  else
    jq -e '(.summary.rollback_feasibility_checks // 0) == 0 and (.statements[0].rollback_feasibility? == null)' "$report" > /dev/null
  fi

  jq -n \
    --arg id "$id" \
    --arg engine "$engine" \
    --slurpfile report "$report" \
    '{
      id:$id,
      engine:$engine,
      finding:(($report[0].summary.rollback_feasibility_checks // 0) == 1),
      class:($report[0].statements[0].rollback_feasibility.class // "none"),
      status:($report[0].statements[0].rollback_feasibility.status // "none"),
      feasible:($report[0].statements[0].rollback_feasibility.feasible // false),
      implicit_commit:($report[0].statements[0].rollback_feasibility.implicit_commit // false),
      irreversible_metadata:($report[0].statements[0].rollback_feasibility.irreversible_metadata // false),
      verified:true
    }' > "$OUT/reports/$id.row.json"
  rows+=("$OUT/reports/$id.row.json")
done < <(jq -r '.cases[] | @base64' "$SPEC")

jq -n \
  --slurpfile spec "$SPEC" \
  --slurpfile rows <(jq -s '.' "${rows[@]}") \
  '{
    version:"patchline.db-rollback-feasibility-results/v1",
    claim:$spec[0].claim,
    cases:$rows[0],
    summary:{
      cases:($rows[0]|length),
      findings:($rows[0] | map(select(.finding == true)) | length),
      controls:($rows[0] | map(select(.finding == false)) | length),
      verified:($rows[0] | map(select(.verified == true)) | length),
      classes:($rows[0] | map(select(.finding == true) | .class) | unique),
      implicit_commit:($rows[0] | map(select(.implicit_commit == true)) | length),
      irreversible_metadata:($rows[0] | map(select(.irreversible_metadata == true)) | length)
    }
  }' > "$OUT/db-rollback-feasibility.json"

jq -e --slurpfile spec "$SPEC" '
  .summary.findings >= $spec[0].minimum_findings and
  .summary.controls >= 1 and
  .summary.verified == .summary.cases and
  (.summary.classes | index("transactional_ddl")) and
  (.summary.classes | index("implicit_commit_compensation")) and
  (.summary.classes | index("irreversible_metadata")) and
  (.summary.classes | index("async_mutation_recovery")) and
  (.summary.classes | index("compensating_dml")) and
  (.summary.classes | index("snapshot_required")) and
  .summary.implicit_commit >= 2 and
  .summary.irreversible_metadata >= 2
' "$OUT/db-rollback-feasibility.json" > /dev/null

{
  echo "# Database rollback feasibility"
  echo
  echo "| Case | Engine | Class | Status | Feasible |"
  echo "|---|---|---|---|---:|"
  jq -r '.cases[] | "| \(.id) | \(.engine) | \(.class) | \(.status) | \(.feasible) |"' "$OUT/db-rollback-feasibility.json"
} > "$OUT/db-rollback-feasibility.md"
cp "$OUT/db-rollback-feasibility.md" "$OUT/README.md"

echo "db rollback feasibility gate passed: $(jq -r '.summary.findings' "$OUT/db-rollback-feasibility.json") findings, $(jq -r '.summary.controls' "$OUT/db-rollback-feasibility.json") control"
