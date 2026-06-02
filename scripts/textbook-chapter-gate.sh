#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/textbook-chapter-gate.json}"; OUT="${2:-results/generated/textbook-chapter}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.textbook-chapter-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "evidence" "make textbook-chapter-gate"; do grep -F "$phrase" docs/textbook-chapter.md README.md > /dev/null; done
bash scripts/textbook-chapter.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.textbook-chapter/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.textbook-chapter-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "textbook-chapter gate passed: every section evidence-backed, unbacked section rejected"
