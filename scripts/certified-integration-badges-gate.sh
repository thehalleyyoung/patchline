#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/certified-integration-badges-gate.json}"; OUT="${2:-results/generated/certified-integration-badges}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.certified-integration-badges-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "conformance" "make certified-integration-badges-gate"; do grep -F "$phrase" docs/certified-integration-badges.md README.md > /dev/null; done
bash scripts/certified-integration-badges.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.certified-integration-badges/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.certified-integration-badges-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "certified-integration-badges gate passed: every item scored with evidence on real self-data, unsupported item rejected"
