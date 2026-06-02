#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/expand-contract-gate.json}"
NEGATIVE_SPEC="${2:-examples/expand-contract-negative-gate.json}"
OUT="${3:-results/generated/expand-contract-templates-gate}"

rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.expand-contract/v1" and
  .invariant_spec.version == "patchline.invariants/v1" and
  (.templates | length) == 1 and
  (.orm_projects | length) >= 3
' "$SPEC" > /dev/null

for phrase in "Verified expand/contract migration templates" "expand-contract-template" "make expand-contract-templates-gate"; do
  grep -F "$phrase" docs/expand-contract-templates.md README.md > /dev/null
done

go test ./internal/expandcontract -run 'TestBuildReport|TestReadSpec'
go test ./cmd/patchline -run TestExpandContractTemplateCommandWritesReports

go run ./cmd/patchline expand-contract-template \
  --spec "$SPEC" \
  --out "$OUT" \
  --json > "$OUT/stdout.json"

test -s "$OUT/expand-contract-template.json"
test -s "$OUT/expand-contract-template.md"
test -s "$OUT/expand-contract-template.sql"

jq -e '
  .version == "patchline.expand-contract-report/v1" and
  .ok == true and
  .summary.templates == 1 and
  .summary.templates_checked == 1 and
  .summary.projects == 3 and
  .summary.projects_verified == 3 and
  .summary.stages == 4 and
  (.templates[0].invariant.kind == "unique") and
  (.templates[0].stages | map(.id) == ["expand", "backfill", "validate", "contract"]) and
  (.orm_checks | all(.evidence_ok == true and (.evidence | length) >= 6)) and
  (.orm_checks[].evidence[].path | startswith("/") | not)
' "$OUT/expand-contract-template.json" > /dev/null

go run ./cmd/patchline expand-contract-template \
  --spec "$NEGATIVE_SPEC" \
  --out "$OUT/negative" \
  --json > "$OUT/negative.stdout.json"

jq -e '
  .ok == false and
  (.orm_checks | length) == 1 and
  (.orm_checks[0].missing | index("backfill_phase")) and
  (.summary.refuted_checks >= 1)
' "$OUT/negative/expand-contract-template.json" > /dev/null

jq -n --slurpfile r "$OUT/expand-contract-template.json" --slurpfile n "$OUT/negative/expand-contract-template.json" '{
  version: "patchline.expand-contract-templates-gate-results/v1",
  templates: $r[0].summary.templates,
  orm_projects_verified: $r[0].summary.projects_verified,
  missing_backfill_negative_control: ($n[0].ok == false),
  verified: true
}' > "$OUT/gate-summary.json"

echo "expand-contract-templates gate passed: invariant-backed templates verified against ORM project evidence"
