#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/onboarding-quest-gate.json}"
OUT="${2:-results/generated/onboarding-quest-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.onboarding-quest-gate/v1" and (.quest_steps|length) >= 5' "$SPEC" > /dev/null

for phrase in "under one hour" "scaffold" "quest" "make onboarding-quest-gate"; do
  grep -F "$phrase" docs/onboarding-quest.md README.md > /dev/null
done

bash scripts/onboarding-quest.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in onboarding-quest.json onboarding-quest.md README.md; do
  test -s "$OUT/$output"
done

jq -e '
  .version == "patchline.onboarding-quest/v1" and
  .all_files_created == true and
  .scripts_valid == true and
  .spec_valid == true and
  .quest_steps >= 5 and
  .quest_doc_in_sync == true and
  .files_created >= 4
' "$OUT/onboarding-quest.json" > /dev/null

jq -n --slurpfile r "$OUT/onboarding-quest.json" '{
  version: "patchline.onboarding-quest-gate-results/v1",
  demo_ecosystem: $r[0].demo_ecosystem,
  files_created: $r[0].files_created,
  quest_steps: $r[0].quest_steps,
  verified: true
}' > "$OUT/gate-summary.json"

echo "onboarding-quest gate passed: scaffolder emits valid runnable artifacts and the quest stays in sync"
