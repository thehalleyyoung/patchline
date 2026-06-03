#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/remediation-cost-optimizer.json}"
OUT="${2:-results/generated/remediation-cost-optimizer-gate}"

rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.remediation-cost/v1" and
  .thresholds.max_residual_loss == 500 and
  .thresholds.max_uncertainty == 0.5 and
  (.cases | length) == 4 and
  (.cases | map(.id) == ["runtime-guard","verified-backfill","expand-contract","uncertain-remedy"])
' "$SPEC" > /dev/null

for phrase in "Remediation-cost optimizer" "remediation-cost" "make remediation-cost-optimizer-gate"; do
  grep -F "$phrase" docs/remediation-cost-optimizer.md README.md > /dev/null
done

go test ./internal/remediationcost -run 'TestBuildReport|TestReadSpec'
go test ./cmd/patchline -run TestRemediationCostCommandWritesReports

go run ./cmd/patchline remediation-cost \
  --spec "$SPEC" \
  --out "$OUT/safe" \
  --json > "$OUT/safe.stdout.json"

test -s "$OUT/safe/remediation-cost.json"
test -s "$OUT/safe/remediation-cost.md"

jq -e '
  .version == "patchline.remediation-cost-report/v1" and
  .ok == true and
  .summary.guard == 1 and
  .summary.backfill == 1 and
  .summary.expand_contract == 1 and
  .summary.manual_review == 1 and
  (.cases[] | select(.id == "runtime-guard") | .selected.kind == "guard" and .selection_reason == "lowest_total_expected_loss") and
  (.cases[] | select(.id == "verified-backfill") | .selected.kind == "backfill") and
  (.cases[] | select(.id == "expand-contract") | .selected.kind == "expand_contract") and
  (.cases[] | select(.id == "uncertain-remedy") | .selected.kind == "manual_review" and .selection_reason == "uncertainty_exceeds_threshold") and
  (.cases[] | select(.id == "uncertain-remedy") | .rankings[] | select(.kind == "guard") | .viable == false and (.missing_requirements | index("runtime_guard")))
' "$OUT/safe/remediation-cost.json" > /dev/null

jq -n '{
  version: "patchline.remediation-cost/v1",
  name: "unsafe remediation-cost optimizer control",
  thresholds: {max_residual_loss: 100, max_uncertainty: 0.5},
  cases: [{
    id: "unsafe-manual",
    hazard_class: "destructive-contract",
    affected_rows: 100,
    probability: 1,
    impact_per_row: 100,
    uncertainty: 0.8,
    evidence: {},
    options: [{
      id: "manual",
      kind: "manual_review",
      direct_cost: 250,
      risk_reduction: 0.25
    }]
  }]
}' > "$OUT/unsafe-spec.json"

go run ./cmd/patchline remediation-cost \
  --spec "$OUT/unsafe-spec.json" \
  --out "$OUT/unsafe" \
  --json > "$OUT/unsafe.stdout.json"

jq -e '
  .ok == false and
  .summary.manual_review == 1 and
  .summary.counterexamples == 1 and
  (.counterexamples | any(.id == "unsafe-manual.selected.residual_bound")) and
  (.cases[0].obligations | any(.id == "selected.residual_bound" and .status == "refuted"))
' "$OUT/unsafe/remediation-cost.json" > /dev/null

first_hash="$(jq -r '.hash' "$OUT/safe/remediation-cost.json")"
go run ./cmd/patchline remediation-cost \
  --spec "$SPEC" \
  --out "$OUT/repeat" \
  --json > "$OUT/repeat.stdout.json"
second_hash="$(jq -r '.hash' "$OUT/repeat/remediation-cost.json")"
if [[ "$first_hash" != "$second_hash" ]]; then
  echo "FAIL: remediation-cost optimizer hash is not deterministic" >&2
  exit 1
fi

jq -n --slurpfile safe "$OUT/safe/remediation-cost.json" --slurpfile unsafe "$OUT/unsafe/remediation-cost.json" '{
  version: "patchline.remediation-cost-optimizer-gate-results/v1",
  selected_kinds: ($safe[0].cases | map({(.id): .selected.kind}) | add),
  unsafe_counterexamples: $unsafe[0].counterexamples,
  deterministic_hash: $safe[0].hash,
  verified: true
}' > "$OUT/gate-summary.json"

echo "remediation-cost optimizer gate passed: expected-loss ranking selects guard, backfill, expand/contract, and manual review"
