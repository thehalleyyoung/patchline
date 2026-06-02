#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/corpus-stats-public-api-gate.json}"; OUT="${2:-results/generated/corpus-stats-public-api}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.corpus-stats-public-api-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "rate-limited" "make corpus-stats-public-api-gate"; do grep -F "$phrase" docs/corpus-stats-public-api.md README.md > /dev/null; done
bash scripts/corpus-stats-public-api.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.corpus-stats-public-api/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.corpus-stats-public-api-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "corpus-stats-public-api gate passed: every item scored with evidence on real self-data, unsupported item rejected"
