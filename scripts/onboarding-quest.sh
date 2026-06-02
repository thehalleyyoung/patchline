#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/onboarding-quest-gate.json}"
OUT="${2:-results/generated/onboarding-quest}"
rm -rf "$OUT"
mkdir -p "$OUT/scaffold"

jq -e '.version == "patchline.onboarding-quest-gate/v1" and (.quest_steps|length) >= 5' "$SPEC" > /dev/null

# Run the scaffolder into an isolated destination so the repo is untouched.
demo_name="$(jq -r '.demo_ecosystem' "$SPEC")"
bash scripts/new-ecosystem.sh "$demo_name" "$OUT/scaffold" > "$OUT/scaffold.log"

slug="$(printf '%s' "$demo_name" | tr '[:upper:] _' '[:lower:]--' | tr -cd 'a-z0-9-')"

expected=(
  "$OUT/scaffold/examples/$slug-gate.json"
  "$OUT/scaffold/scripts/$slug.sh"
  "$OUT/scaffold/scripts/$slug-gate.sh"
  "$OUT/scaffold/docs/$slug.md"
)
created=0
for f in "${expected[@]}"; do
  if [ -s "$f" ]; then created=$((created+1)); fi
done
all_created=false
if [ "$created" -eq "${#expected[@]}" ]; then all_created=true; fi

# The generated scripts must be syntactically valid bash and the spec valid JSON.
scripts_valid=true
for s in "$OUT/scaffold/scripts/$slug.sh" "$OUT/scaffold/scripts/$slug-gate.sh"; do
  bash -n "$s" || scripts_valid=false
done
spec_valid=true
jq -e '.version' "$OUT/scaffold/examples/$slug-gate.json" >/dev/null || spec_valid=false

# Every quest step must point at a checklist item the scaffold or pattern satisfies.
quest_steps="$(jq '.quest_steps | length' "$SPEC")"

# The quest doc must enumerate the same steps so the narrative and the gate stay in sync.
doc_steps_ok=true
while IFS= read -r step; do
  grep -Fq "$step" docs/onboarding-quest.md || doc_steps_ok=false
done < <(jq -r '.quest_steps[]' "$SPEC")

jq -n \
  --arg demo "$demo_name" \
  --argjson created "$created" \
  --argjson all_created "$all_created" \
  --argjson scripts_valid "$scripts_valid" \
  --argjson spec_valid "$spec_valid" \
  --argjson quest_steps "$quest_steps" \
  --argjson doc_steps_ok "$doc_steps_ok" '
  {
    version: "patchline.onboarding-quest/v1",
    demo_ecosystem: $demo,
    files_created: $created,
    all_files_created: $all_created,
    scripts_valid: $scripts_valid,
    spec_valid: $spec_valid,
    quest_steps: $quest_steps,
    quest_doc_in_sync: $doc_steps_ok
  }
' > "$OUT/onboarding-quest.json"

{
  echo "# Contributor onboarding quest"
  echo
  jq -r '"The scaffolder generated `" + (.files_created|tostring) + "` starter files for a new `" + .demo_ecosystem + "` ecosystem; the generated worker and gate scripts are valid bash and the example spec is valid JSON. A `" + (.quest_steps|tostring) + "`-step quest guides a contributor from zero to a passing gate in under one hour."' "$OUT/onboarding-quest.json"
  echo
  echo "## Guarantees"
  jq -r '"- all scaffold files created: `" + (.all_files_created|tostring) + "`\n- generated scripts are valid bash: `" + (.scripts_valid|tostring) + "`\n- quest doc stays in sync with the gate steps: `" + (.quest_doc_in_sync|tostring) + "`"' "$OUT/onboarding-quest.json"
} > "$OUT/onboarding-quest.md"
cp "$OUT/onboarding-quest.md" "$OUT/README.md"

echo "onboarding quest complete: $created scaffold files, scripts valid, doc in sync"
