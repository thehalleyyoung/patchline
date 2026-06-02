#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/ci-pr-bot-gate.json}"; OUT="${2:-results/generated/ci-pr-bot}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.ci-pr-bot-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "idempotent" "make ci-pr-bot-gate"; do grep -F "$phrase" docs/ci-pr-bot.md README.md > /dev/null; done
bash scripts/ci-pr-bot.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.ci-pr-bot/v1" and .idempotent==true and .anchored==true and .body_match==true and .changed_updates==true' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.ci-pr-bot-gate-results/v1",idempotent:$r[0].idempotent,changed_updates:$r[0].changed_updates,verified:true}' > "$OUT/gate-summary.json"
echo "ci-pr-bot gate passed: idempotent across identical runs, changed diff updates comment"
