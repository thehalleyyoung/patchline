#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/git-history-streaming-gate.json}"; OUT="${2:-results/generated/git-history-streaming}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.git-history-streaming-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "git history" "make git-history-streaming-gate"; do grep -F "$phrase" docs/git-history-streaming.md README.md > /dev/null; done
bash scripts/git-history-streaming.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.git-history-streaming/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.git-history-streaming-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "git-history-streaming gate passed: every historical migration analyzed, skipped commit rejected"
