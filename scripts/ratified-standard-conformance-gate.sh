#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/ratified-standard-conformance-gate.json}"; OUT="${2:-results/generated/ratified-standard-conformance}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.ratified-standard-conformance-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "ratified standard" "make ratified-standard-conformance-gate"; do grep -F "$phrase" docs/ratified-standard-conformance.md README.md > /dev/null; done
bash scripts/ratified-standard-conformance.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.ratified-standard-conformance/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.ratified-standard-conformance-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "ratified-standard-conformance gate passed: every item scored with evidence on real self-data, unsupported item rejected"
