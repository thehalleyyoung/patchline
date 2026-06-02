#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/neuro-symbolic-proof-search-gate.json}"; OUT="${2:-results/generated/neuro-symbolic-proof-search}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.neuro-symbolic-proof-search-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "neuro-symbolic proof search" "make neuro-symbolic-proof-search-gate"; do grep -F "$phrase" docs/neuro-symbolic-proof-search.md README.md > /dev/null; done
bash scripts/neuro-symbolic-proof-search.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.neuro-symbolic-proof-search/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.neuro-symbolic-proof-search-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "neuro-symbolic-proof-search gate passed: every item scored with evidence on real self-data, unsupported item rejected"
