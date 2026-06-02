#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/artifact-evaluation-kit-gate.json}"
OUT="${2:-results/generated/artifact-evaluation-kit-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '
  .version == "patchline.artifact-evaluation-kit-gate/v1" and
  (.roles | length) >= .minimum_roles and
  (.expected_outputs | length) >= .minimum_expected_outputs and
  (.pass_fail_criteria | length) >= .minimum_pass_fail_criteria
' "$SPEC" > /dev/null

for phrase in "artifact-evaluation landing kit" "reviewer roles" "time budgets" "expected outputs" "pass/fail criteria" "make artifact-evaluation-kit-gate"; do
  grep -F "$phrase" docs/artifact-evaluation-kit.md README.md > /dev/null
done

bash scripts/generate-artifact-evaluation-kit.sh "$SPEC" "$OUT" > "$OUT.run.log"

min_roles="$(jq '.minimum_roles' "$SPEC")"
min_outputs="$(jq '.minimum_expected_outputs' "$SPEC")"
min_criteria="$(jq '.minimum_pass_fail_criteria' "$SPEC")"
jq -e --argjson min_roles "$min_roles" --argjson min_outputs "$min_outputs" --argjson min_criteria "$min_criteria" '
  .version == "patchline.artifact-evaluation-kit/v1" and
  .summary.roles >= $min_roles and
  .summary.expected_outputs >= $min_outputs and
  .summary.pass_fail_criteria >= $min_criteria and
  .summary.total_time_budget_minutes >= 120 and
  .summary.public_repos >= 4 and
  .summary.ranked_risks > 100 and
  .summary.generated_files > 0 and
  .summary.rejected_examples >= 2 and
  .summary.evidence_artifact_types >= 6 and
  .summary.verified == true and
  all(.roles[]; .time_budget_minutes >= 15 and (.goal | length) > 40)
' "$OUT/artifact-evaluation-kit.json" > /dev/null

while read -r expected; do
  test -s "$OUT/$expected"
done < <(jq -r '.expected_outputs[]' "$SPEC")

for role in quickstart-reviewer artifact-reviewer security-reviewer research-reviewer; do
  grep -F "$role" "$OUT/roles.json" > /dev/null
done
grep -F "Reviewer roles and time budgets" "$OUT/landing.md" > /dev/null
grep -F "Expected outputs" "$OUT/landing.md" > /dev/null
grep -F "Pass/fail criteria" "$OUT/landing.md" > /dev/null
grep -F "Regenerated public-code evidence" "$OUT/landing.md" > /dev/null
grep -F "Patchline release-quality capstone demo" "$OUT/capstone/session.md" > /dev/null

jq -n \
  --slurpfile kit "$OUT/artifact-evaluation-kit.json" \
  '{
    version:"patchline.artifact-evaluation-kit-gate-results/v1",
    roles:$kit[0].summary.roles,
    expected_outputs:$kit[0].summary.expected_outputs,
    pass_fail_criteria:$kit[0].summary.pass_fail_criteria,
    public_repos:$kit[0].summary.public_repos,
    ranked_risks:$kit[0].summary.ranked_risks,
    verified:true
  }' > "$OUT/gate-summary.json"

echo "artifact evaluation kit gate passed: roles $(jq '.roles' "$OUT/gate-summary.json"), outputs $(jq '.expected_outputs' "$OUT/gate-summary.json"), criteria $(jq '.pass_fail_criteria' "$OUT/gate-summary.json")"
