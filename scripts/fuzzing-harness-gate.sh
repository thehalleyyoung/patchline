#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/fuzzing-harness-gate.json}"; OUT="${2:-results/generated/fuzzing-harness}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.fuzzing-harness-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "fuzzing" "make fuzzing-harness-gate"; do grep -F "$phrase" docs/fuzzing-harness.md README.md > /dev/null; done
bash scripts/fuzzing-harness.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.fuzzing-harness/v1" and .no_crash==true and .sound==true and .planted_detected==true' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.fuzzing-harness-gate-results/v1",no_crash:$r[0].no_crash,sound:$r[0].sound,planted_detected:$r[0].planted_detected,verified:true}' > "$OUT/gate-summary.json"
echo "fuzzing-harness gate passed: no crashes, no unsound passes, planted unsound pass detected"
