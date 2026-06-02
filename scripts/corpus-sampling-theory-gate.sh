#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/corpus-sampling-theory-gate.json}"; OUT="${2:-results/generated/corpus-sampling-theory}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.corpus-sampling-theory-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "provable sampling bound" "make corpus-sampling-theory-gate"; do grep -F "$phrase" docs/corpus-sampling-theory.md README.md > /dev/null; done
bash scripts/corpus-sampling-theory.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.corpus-sampling-theory/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.corpus-sampling-theory-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "corpus-sampling-theory gate passed: every item scored with evidence on real self-data, unsupported item rejected"
