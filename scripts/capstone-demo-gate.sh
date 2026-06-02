#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/capstone-demo-gate.json}"
OUT="${2:-results/generated/capstone-demo-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '
  .version == "patchline.capstone-demo-gate/v1" and
  (.real_code | length) >= .minimum_public_repos and
  (.required_outputs | length) >= .minimum_evidence_artifacts
' "$SPEC" > /dev/null

for phrase in "release-quality capstone demo" "fresh user" "four unfamiliar" "high-signal" "rejects bad output" "experiment-ready evidence" "make capstone-demo-gate"; do
  grep -F "$phrase" docs/capstone-demo.md README.md > /dev/null
done

bash scripts/capstone-demo.sh "$SPEC" "$OUT" > "$OUT.run.log"

min_repos="$(jq '.minimum_public_repos' "$SPEC")"
min_risks="$(jq '.minimum_ranked_risks' "$SPEC")"
min_generated="$(jq '.minimum_generated_files' "$SPEC")"
min_rejected="$(jq '.minimum_rejected_examples' "$SPEC")"
jq -e --argjson min_repos "$min_repos" --argjson min_risks "$min_risks" --argjson min_generated "$min_generated" --argjson min_rejected "$min_rejected" '
  .version == "patchline.capstone-demo/v1" and
  .summary.public_repos >= $min_repos and
  .summary.ranked_risks >= $min_risks and
  .summary.generated_files >= $min_generated and
  .summary.bounded_interventions >= $min_repos and
  .summary.rejected_examples >= $min_rejected and
  .summary.rejected_interventions >= $min_rejected and
  .summary.deterministic_only == true and
  .summary.verified == true and
  .summary.evidence_artifacts.failure_modes > 0 and
  .summary.evidence_artifacts.figures >= 5 and
  .summary.evidence_artifacts.case_studies >= $min_repos and
  (.summary.hash | length) == 64 and
  all(.analyses[]; .verified == true and .files_scanned > 0 and .ranked_risks > 0 and .generated_files > 0)
' "$OUT/summary.json" > /dev/null

while read -r required; do
  test -s "$OUT/$required"
done < <(jq -r '.required_outputs[]' "$SPEC")

for repo in $(jq -r '.real_code[].repo' "$SPEC"); do
  grep -F "$repo" "$OUT/session.md" > /dev/null
done
grep -F "Bad output rejection" "$OUT/session.md" > /dev/null
grep -F "Experiment-ready evidence" "$OUT/session.md" > /dev/null
grep -F "Reproduce" "$OUT/session.md" > /dev/null
grep -F "rejected generated-code examples" "$OUT/rejections/rejected-generated.md" > /dev/null

jq -n \
  --slurpfile summary "$OUT/summary.json" \
  '{
    version:"patchline.capstone-demo-gate-results/v1",
    public_repos:$summary[0].summary.public_repos,
    ranked_risks:$summary[0].summary.ranked_risks,
    generated_files:$summary[0].summary.generated_files,
    rejected_examples:$summary[0].summary.rejected_examples,
    figures:$summary[0].summary.evidence_artifacts.figures,
    case_studies:$summary[0].summary.evidence_artifacts.case_studies,
    hash:$summary[0].summary.hash,
    verified:true
  }' > "$OUT/gate-summary.json"

echo "capstone demo gate passed: repos $(jq '.public_repos' "$OUT/gate-summary.json"), risks $(jq '.ranked_risks' "$OUT/gate-summary.json"), rejected $(jq '.rejected_examples' "$OUT/gate-summary.json")"
