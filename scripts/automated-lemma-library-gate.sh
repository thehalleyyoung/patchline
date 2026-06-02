#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/automated-lemma-library-gate.json}"; OUT="${2:-results/generated/automated-lemma-library}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.automated-lemma-library-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "automated lemma library" "make automated-lemma-library-gate"; do grep -F "$phrase" docs/automated-lemma-library.md README.md > /dev/null; done
bash scripts/automated-lemma-library.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.automated-lemma-library/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.automated-lemma-library-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "automated-lemma-library gate passed: every item scored with evidence on real self-data, unsupported item rejected"
