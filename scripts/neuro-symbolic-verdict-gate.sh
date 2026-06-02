#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/neuro-symbolic-verdict-gate.json}"; OUT="${2:-results/generated/neuro-symbolic-verdict}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.neuro-symbolic-verdict-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "constraint" "make neuro-symbolic-verdict-gate"; do grep -F "$phrase" docs/neuro-symbolic-verdict.md README.md > /dev/null; done
bash scripts/neuro-symbolic-verdict.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.neuro-symbolic-verdict/v1" and .all_correct==true and .constraint_overrides==true' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.neuro-symbolic-verdict-gate-results/v1",all_correct:$r[0].all_correct,constraint_overrides:$r[0].constraint_overrides,verified:true}' > "$OUT/gate-summary.json"
echo "neuro-symbolic-verdict gate passed: gate constraint overrides wrong prior, prior decides where gates silent"
