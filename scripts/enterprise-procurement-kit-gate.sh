#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/enterprise-procurement-kit-gate.json}"; OUT="${2:-results/generated/enterprise-procurement-kit}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.enterprise-procurement-kit-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "procurement" "make enterprise-procurement-kit-gate"; do grep -F "$phrase" docs/enterprise-procurement-kit.md README.md > /dev/null; done
bash scripts/enterprise-procurement-kit.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.enterprise-procurement-kit/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.enterprise-procurement-kit-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "enterprise-procurement-kit gate passed: every item scored with evidence on real self-data, unsupported item rejected"
