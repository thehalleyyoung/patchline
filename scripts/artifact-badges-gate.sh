#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/artifact-badges-gate.json}"
OUT="${2:-results/generated/artifact-badges-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '
  . as $root |
  .version == "patchline.artifact-badges-gate/v1" and
  (.required_badges | sort) == ["available","functional","reproducible","reusable"] and
  all(.badges[]; (.criteria | length) >= $root.minimum_justifications_per_badge)
' "$SPEC" > /dev/null

for phrase in "artifact badges" "gate-backed justifications" "reusable" "available" "functional" "reproducible" "make artifact-badges-gate"; do
  grep -F "$phrase" docs/artifact-badges.md README.md > /dev/null
done

bash scripts/generate-artifact-badges.sh "$SPEC" "$OUT" > "$OUT.run.log"

min_repos="$(jq '.minimum_public_repos' "$SPEC")"
min_risks="$(jq '.minimum_ranked_risks' "$SPEC")"
min_generated="$(jq '.minimum_generated_files' "$SPEC")"
min_justifications="$(jq '.minimum_justifications_per_badge' "$SPEC")"
jq -e --argjson min_repos "$min_repos" --argjson min_risks "$min_risks" --argjson min_generated "$min_generated" --argjson min_justifications "$min_justifications" '
  .version == "patchline.artifact-badges/v1" and
  .summary.badges == 4 and
  .summary.awarded == 4 and
  .summary.public_repos >= $min_repos and
  .summary.ranked_risks >= $min_risks and
  .summary.generated_files >= $min_generated and
  .summary.rejected_examples >= 2 and
  .summary.verified == true and
  all(.badges[]; .awarded == true and (.criteria | length) >= $min_justifications and (.svg | endswith(".svg")))
' "$OUT/artifact-badges.json" > /dev/null

for badge in available functional reusable reproducible; do
  test -s "$OUT/badges/$badge.svg"
  grep -F "<svg" "$OUT/badges/$badge.svg" > /dev/null
  grep -F "### $badge" "$OUT/badges.md" > /dev/null
done

grep -F "Patchline release-quality capstone demo" "$OUT/evidence/artifact-container-rebuild/public-results/capstone/session.md" > /dev/null
grep -F "Host-independence guarantees" "$OUT/evidence/artifact-container-rebuild/README.md" > /dev/null
grep -F "Gate-backed justifications" "$OUT/badges.md" > /dev/null

jq -n \
  --slurpfile badges "$OUT/artifact-badges.json" \
  '{
    version:"patchline.artifact-badges-gate-results/v1",
    badges:$badges[0].summary.badges,
    awarded:$badges[0].summary.awarded,
    public_repos:$badges[0].summary.public_repos,
    ranked_risks:$badges[0].summary.ranked_risks,
    verified:true
  }' > "$OUT/gate-summary.json"

echo "artifact badges gate passed: badges $(jq '.badges' "$OUT/gate-summary.json"), repos $(jq '.public_repos' "$OUT/gate-summary.json"), risks $(jq '.ranked_risks' "$OUT/gate-summary.json")"
