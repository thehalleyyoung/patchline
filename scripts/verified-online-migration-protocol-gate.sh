#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/verified-online-migration-protocol-gate.json}"; OUT="${2:-results/generated/verified-online-migration-protocol}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.verified-online-migration-protocol-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "online-migration protocol" "make verified-online-migration-protocol-gate"; do grep -F "$phrase" docs/verified-online-migration-protocol.md README.md > /dev/null; done
bash scripts/verified-online-migration-protocol.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.verified-online-migration-protocol/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.verified-online-migration-protocol-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "verified-online-migration-protocol gate passed: every item scored with evidence on real self-data, unsupported item rejected"
