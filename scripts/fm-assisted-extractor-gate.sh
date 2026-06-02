#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/fm-assisted-extractor-gate.json}"; OUT="${2:-results/generated/fm-assisted-extractor}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.fm-assisted-extractor-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "deterministic" "make fm-assisted-extractor-gate"; do grep -F "$phrase" docs/fm-assisted-extractor.md README.md > /dev/null; done
bash scripts/fm-assisted-extractor.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.fm-assisted-extractor/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.fm-assisted-extractor-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "fm-assisted-extractor gate passed: every extraction deterministically verified, unverified extraction rejected"
