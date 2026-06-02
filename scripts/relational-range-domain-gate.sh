#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/relational-range-domain-gate.json}"; OUT="${2:-results/generated/relational-range-domain}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.relational-range-domain-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "abstract interpretation" "make relational-range-domain-gate"; do grep -F "$phrase" docs/relational-range-domain.md README.md > /dev/null; done
bash scripts/relational-range-domain.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.relational-range-domain/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.relational-range-domain-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "relational-range-domain gate passed: every item scored with evidence on real self-data, unsupported item rejected"
