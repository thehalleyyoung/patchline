#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/merkle-audit-log-gate.json}"; OUT="${2:-results/generated/merkle-audit-log}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.merkle-audit-log-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "tamper-evident" "make merkle-audit-log-gate"; do grep -F "$phrase" docs/merkle-audit-log.md README.md > /dev/null; done
bash scripts/merkle-audit-log.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.merkle-audit-log/v1" and .chained==true and .genesis==true and .tamper_present==true' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.merkle-audit-log-gate-results/v1",chained:$r[0].chained,tamper_detected:$r[0].tamper_present,verified:true}' > "$OUT/gate-summary.json"
echo "merkle-audit-log gate passed: honest log verifies, tampered entry detected"
