#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/reproducible-rl-repair-policy-gate.json}"; OUT="${2:-results/generated/reproducible-rl-repair-policy}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.reproducible-rl-repair-policy-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "reproducible RL policy" "make reproducible-rl-repair-policy-gate"; do grep -F "$phrase" docs/reproducible-rl-repair-policy.md README.md > /dev/null; done
bash scripts/reproducible-rl-repair-policy.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.reproducible-rl-repair-policy/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.reproducible-rl-repair-policy-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "reproducible-rl-repair-policy gate passed: every item scored with evidence on real self-data, unsupported item rejected"
