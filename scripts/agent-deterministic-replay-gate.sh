#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/agent-deterministic-replay-gate.json}"; OUT="${2:-results/generated/agent-deterministic-replay}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.agent-deterministic-replay-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "deterministic replay" "make agent-deterministic-replay-gate"; do grep -F "$phrase" docs/agent-deterministic-replay.md README.md > /dev/null; done
bash scripts/agent-deterministic-replay.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.agent-deterministic-replay/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.agent-deterministic-replay-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "agent-deterministic-replay gate passed: every item scored with evidence on real self-data, unsupported item rejected"
