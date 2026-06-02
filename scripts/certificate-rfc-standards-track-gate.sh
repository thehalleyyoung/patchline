#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/certificate-rfc-standards-track-gate.json}"; OUT="${2:-results/generated/certificate-rfc-standards-track}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.certificate-rfc-standards-track-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "interoperable" "make certificate-rfc-standards-track-gate"; do grep -F "$phrase" docs/certificate-rfc-standards-track.md README.md > /dev/null; done
bash scripts/certificate-rfc-standards-track.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.certificate-rfc-standards-track/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.certificate-rfc-standards-track-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "certificate-rfc-standards-track gate passed: every item scored with evidence on real self-data, unsupported item rejected"
