#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/information-bottleneck-features-gate.json}"; OUT="${2:-results/generated/information-bottleneck-features}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.information-bottleneck-features-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "information bottleneck" "make information-bottleneck-features-gate"; do grep -F "$phrase" docs/information-bottleneck-features.md README.md > /dev/null; done
bash scripts/information-bottleneck-features.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.information-bottleneck-features/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.information-bottleneck-features-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "information-bottleneck-features gate passed: every item scored with evidence on real self-data, unsupported item rejected"
