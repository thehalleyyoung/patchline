#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/certificate-standard-gate.json}"; OUT="${2:-results/generated/certificate-standard}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.certificate-standard-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "interoperability" "make certificate-standard-gate"; do grep -F "$phrase" docs/certificate-standard.md README.md > /dev/null; done
bash scripts/certificate-standard.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.certificate-standard/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.certificate-standard-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "certificate-standard gate passed: every format field specified and tested, underspecified field rejected"
