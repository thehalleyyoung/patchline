#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/verifier-in-the-loop-gate.json}"; OUT="${2:-results/generated/verifier-in-the-loop}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.verifier-in-the-loop-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "passing certificate" "make verifier-in-the-loop-gate"; do grep -F "$phrase" docs/verifier-in-the-loop.md README.md > /dev/null; done
bash scripts/verifier-in-the-loop.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.verifier-in-the-loop/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.verifier-in-the-loop-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "verifier-in-the-loop gate passed: every item scored with evidence on real self-data, unsupported item rejected"
