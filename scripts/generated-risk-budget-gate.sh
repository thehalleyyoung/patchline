#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/generated-risk-budget.json}"
OUT="${2:-results/generated/generated-risk-budget-gate}"
rm -rf "$OUT"
mkdir -p "$OUT/cases"

jq -e '.version == "patchline.generated-risk-budget/v1" and .minimum_public_slices >= 4 and (.claim | contains("newly introduced SQL risk budget"))' "$SPEC" > /dev/null

rows=()
while IFS=$'\t' read -r id repo ref subpath; do
  case_out="$OUT/cases/$id"
  mkdir -p "$case_out"
  go run ./cmd/patchline repo analyze --github "$repo" --ref "$ref" --subpath "$subpath" --stages inventory,baseline,propose,compare --proposal-kind explain --budget files=1,lines=120,tokens=4000,changes=1 --no-llm --out "$case_out/analyze" --json > "$case_out/analyze.json"
  jq -e '.summary.risk_budget_rejected == false and .summary.patchline_checks_failed == 0' "$case_out/analyze/compare/compare.json" > /dev/null
  cp -R "$case_out/analyze/proposal" "$case_out/risky-proposal"
  explain_path="$(jq -r '.generated_files[0].path' "$case_out/risky-proposal/proposal.json")"
  cat >> "$case_out/risky-proposal/$explain_path" <<'SQL'

-- Budget mutation: generated writes that must be rejected before review.
UPDATE patchline_budget_probe SET unsafe = true;
DELETE FROM patchline_budget_probe;
SQL
  go run ./cmd/patchline repo compare --before "$case_out/analyze/baseline" --after "$case_out/risky-proposal" --out "$case_out/risky-compare" --json > "$case_out/risky-compare.json"
  jq -e '
    .summary.patchline_checks_failed == 0 and
    .summary.risk_budget_rejected == true and
    .summary.risk_budget_added > .summary.risk_budget_covered and
    .intervention_loop.status == "rejected-by-deterministic-checks"
  ' "$case_out/risky-compare.json" > /dev/null
  jq -n \
    --arg id "$id" \
    --arg repo "$repo" \
    --arg explain_path "$explain_path" \
    --slurpfile compare "$case_out/risky-compare.json" \
    '{id:$id, repo:$repo, explain_path:$explain_path, risk_budget_covered:$compare[0].summary.risk_budget_covered, risk_budget_added:$compare[0].summary.risk_budget_added, risk_budget_rejected:$compare[0].summary.risk_budget_rejected, intervention_status:$compare[0].intervention_loop.status, verified:true}' > "$case_out/row.json"
  rows+=("$case_out/row.json")
done < <(jq -r '.slices[] | [.id, .repo, .ref, .subpath] | @tsv' examples/real-repo-slices.json)

jq -n \
  --slurpfile spec "$SPEC" \
  --slurpfile rows <(jq -s '.' "${rows[@]}") \
  '{
    version:"patchline.generated-risk-budget-results/v1",
    claim:$spec[0].claim,
    slices:$rows[0],
    summary:{
      public_slices:($rows[0] | length),
      rejected:($rows[0] | map(select(.risk_budget_rejected == true)) | length)
    }
  }' > "$OUT/generated-risk-budget.json"

jq -e --slurpfile spec "$SPEC" '
  (.slices | length) >= $spec[0].minimum_public_slices and
  .summary.rejected == (.slices | length) and
  all(.slices[]; .verified == true and .risk_budget_added > .risk_budget_covered and .intervention_status == "rejected-by-deterministic-checks")
' "$OUT/generated-risk-budget.json" > /dev/null

echo "generated risk budget gate passed: $(jq '.summary.rejected' "$OUT/generated-risk-budget.json") risky interventions rejected"
