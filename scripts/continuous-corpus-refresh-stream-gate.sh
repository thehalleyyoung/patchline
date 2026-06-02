#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/continuous-corpus-refresh-stream-gate.json}"; OUT="${2:-results/generated/continuous-corpus-refresh-stream}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.continuous-corpus-refresh-stream-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "continuous corpus refresh" "make continuous-corpus-refresh-stream-gate"; do grep -F "$phrase" docs/continuous-corpus-refresh-stream.md README.md > /dev/null; done
bash scripts/continuous-corpus-refresh-stream.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.continuous-corpus-refresh-stream/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.continuous-corpus-refresh-stream-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "continuous-corpus-refresh-stream gate passed: every item scored with evidence on real self-data, unsupported item rejected"
