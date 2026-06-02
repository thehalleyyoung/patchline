#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/agent-prompt-injection-redteam-gate.json}"; OUT="${2:-results/generated/agent-prompt-injection-redteam}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.agent-prompt-injection-redteam-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "prompt-injection" "make agent-prompt-injection-redteam-gate"; do grep -F "$phrase" docs/agent-prompt-injection-redteam.md README.md > /dev/null; done
bash scripts/agent-prompt-injection-redteam.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.agent-prompt-injection-redteam/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.agent-prompt-injection-redteam-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "agent-prompt-injection-redteam gate passed: every item scored with evidence on real self-data, unsupported item rejected"
