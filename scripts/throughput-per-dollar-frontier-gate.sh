#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/throughput-per-dollar-frontier-gate.json}"; OUT="${2:-results/generated/throughput-per-dollar-frontier}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.throughput-per-dollar-frontier-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "throughput-per-dollar frontier" "make throughput-per-dollar-frontier-gate"; do grep -F "$phrase" docs/throughput-per-dollar-frontier.md README.md > /dev/null; done
bash scripts/throughput-per-dollar-frontier.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.throughput-per-dollar-frontier/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.throughput-per-dollar-frontier-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "throughput-per-dollar-frontier gate passed: every item scored with evidence on real self-data, unsupported item rejected"
