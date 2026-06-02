#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/cross-lingual-comments-gate.json}"; OUT="${2:-results/generated/cross-lingual-comments}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.cross-lingual-comments-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "non-English" "make cross-lingual-comments-gate"; do grep -F "$phrase" docs/cross-lingual-comments.md README.md > /dev/null; done
bash scripts/cross-lingual-comments.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.cross-lingual-comments/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.cross-lingual-comments-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "cross-lingual-comments gate passed: extraction works across languages, failed-language extraction rejected"
