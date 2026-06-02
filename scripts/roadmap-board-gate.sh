#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/roadmap-board-gate.json}"
OUT="${2:-results/generated/roadmap-board-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '
  .version == "patchline.roadmap-board-gate/v1" and
  (.real_code | length) >= .minimum_public_repos and
  (.planned_features | length) >= .minimum_features
' "$SPEC" > /dev/null

for phrase in "public roadmap board" "real-repo failure mode" "proof gate" "expected artifact" "make roadmap-board-gate"; do
  grep -F "$phrase" docs/roadmap-board.md README.md > /dev/null
done

bash scripts/generate-roadmap-board.sh "$SPEC" "$OUT" > "$OUT.run.log"

min_repos="$(jq '.minimum_public_repos' "$SPEC")"
min_modes="$(jq '.minimum_failure_modes' "$SPEC")"
min_features="$(jq '.minimum_features' "$SPEC")"
jq -e --argjson min_repos "$min_repos" --argjson min_modes "$min_modes" --argjson min_features "$min_features" '
  .version == "patchline.roadmap-board/v1" and
  .taxonomy.public_repos >= $min_repos and
  .taxonomy.failure_modes >= $min_modes and
  .summary.features >= $min_features and
  (.summary.gates | length) == .summary.features and
  (.summary.expected_artifacts | length) == .summary.features and
  (.summary.linked_failure_modes | length) >= $min_modes and
  all(.cards[];
    .verified == true and
    (.gate | startswith("make ")) and
    (.expected_artifact | length) > 20 and
    (.failure_mode.id | length) > 0 and
    (.failure_mode.example.repo | contains("/")) and
    (.failure_mode.example.subpath | length) > 0 and
    (.failure_mode.example.severity | length) > 0 and
    (.failure_mode.occurrences > 0)
  )
' "$OUT/roadmap-board.json" > /dev/null

while read -r id; do
  test -s "$OUT/cards/$id.md"
  grep -F "Linked real-repo failure mode" "$OUT/cards/$id.md" > /dev/null
  grep -F "Expected artifact" "$OUT/cards/$id.md" > /dev/null
  grep -F "Gate:" "$OUT/cards/$id.md" > /dev/null
done < <(jq -r '.planned_features[].id' "$SPEC")

grep -F "Patchline public roadmap board" "$OUT/roadmap-board.md" > /dev/null
grep -F "Every planned feature links" "$OUT/roadmap-board.md" > /dev/null
grep -F "Regenerated evidence" "$OUT/roadmap-board.md" > /dev/null
for repo in $(jq -r '.real_code[].repo' "$SPEC"); do
  grep -F "$repo" "$OUT/taxonomy/failure-taxonomy.md" > /dev/null
done

jq -n \
  --slurpfile board "$OUT/roadmap-board.json" \
  '{
    version:"patchline.roadmap-board-gate-results/v1",
    features:$board[0].summary.features,
    public_repos:$board[0].taxonomy.public_repos,
    failure_modes:$board[0].taxonomy.failure_modes,
    linked_failure_modes:($board[0].summary.linked_failure_modes | length),
    occurrences:$board[0].taxonomy.occurrences,
    verified:true
  }' > "$OUT/gate-summary.json"

echo "roadmap board gate passed: features $(jq '.features' "$OUT/gate-summary.json"), modes $(jq '.failure_modes' "$OUT/gate-summary.json"), repos $(jq '.public_repos' "$OUT/gate-summary.json")"
