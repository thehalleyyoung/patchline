#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/contributor-recognition-gate.json}"
OUT="${2:-results/generated/contributor-recognition-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '
  .version == "patchline.contributor-recognition-gate/v1" and
  (.contributors | length) >= .minimum_contributors and
  (.required_categories | length) >= .minimum_categories
' "$SPEC" > /dev/null

for phrase in "contributor recognition" "real-repo slices" "ecosystem parsers" "false-positive reductions" "artifact improvements" "make contributor-recognition-gate"; do
  grep -F "$phrase" docs/contributor-recognition.md README.md > /dev/null
done

bash scripts/generate-contributor-recognition.sh "$SPEC" "$OUT" > "$OUT.run.log"

min_contributors="$(jq '.minimum_contributors' "$SPEC")"
min_categories="$(jq '.minimum_categories' "$SPEC")"
jq -e --argjson min_contributors "$min_contributors" --argjson min_categories "$min_categories" '
  .version == "patchline.contributor-recognition/v1" and
  .summary.contributors >= $min_contributors and
  (.summary.categories | length) >= $min_categories and
  (.summary.proof_gates | length) >= 3 and
  .summary.real_repo_slices > 0 and
  .summary.ecosystem_count >= 4 and
  .summary.false_positive_reductions > 0 and
  .summary.artifact_improvements > 0 and
  .summary.verified == true and
  all(.contributors[]; .verified == true and .score > 0 and (.badge | length) > 0 and (.proof.verified == true))
' "$OUT/contributor-recognition.json" > /dev/null

while read -r category; do
  jq -e --arg category "$category" '.summary.categories | index($category) != null' "$OUT/contributor-recognition.json" > /dev/null
  grep -F "$category" "$OUT/leaderboard.md" > /dev/null
done < <(jq -r '.required_categories[]' "$SPEC")

while read -r contributor; do
  test -s "$OUT/cards/$contributor.md"
  grep -F "Proof gate" "$OUT/cards/$contributor.md" > /dev/null
done < <(jq -r '.contributors[].id' "$SPEC")

for proof in awesome-patchline rejected-generated reviewability-examples; do
  test -s "$OUT/proofs/$proof.run.log"
  test -s "$OUT/proofs/$proof/proof-row.json"
done

jq -n \
  --slurpfile recognition "$OUT/contributor-recognition.json" \
  '{
    version:"patchline.contributor-recognition-gate-results/v1",
    contributors:$recognition[0].summary.contributors,
    categories:($recognition[0].summary.categories | length),
    real_repo_slices:$recognition[0].summary.real_repo_slices,
    ecosystem_count:$recognition[0].summary.ecosystem_count,
    false_positive_reductions:$recognition[0].summary.false_positive_reductions,
    artifact_improvements:$recognition[0].summary.artifact_improvements,
    verified:true
  }' > "$OUT/gate-summary.json"

echo "contributor recognition gate passed: contributors $(jq '.contributors' "$OUT/gate-summary.json"), categories $(jq '.categories' "$OUT/gate-summary.json")"
