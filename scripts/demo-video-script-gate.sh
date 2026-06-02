#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/demo-video-script-gate.json}"; OUT="${2:-results/generated/demo-video-script}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.demo-video-script-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "end-to-end workflow" "make demo-video-script-gate"; do grep -F "$phrase" docs/demo-video-script.md README.md > /dev/null; done
bash scripts/demo-video-script.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.demo-video-script/v1" and .all_runnable==true and .uncovered_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.demo-video-script-gate-results/v1",runnable:$r[0].runnable,all_runnable:$r[0].all_runnable,uncovered_rejected:($r[0].uncovered_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "demo-video-script gate passed: every script beat backed by a runnable command, uncovered beat rejected"
