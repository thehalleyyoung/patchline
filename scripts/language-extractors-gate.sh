#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/language-extractors-gate.json}"; OUT="${2:-results/generated/language-extractors}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.language-extractors-gate/v1" and (.claim|length) > 200 and (.findings|length) >= 5' "$SPEC" > /dev/null
for phrase in "schema" "make language-extractors-gate"; do grep -F "$phrase" docs/language-extractors.md README.md > /dev/null; done
bash scripts/language-extractors.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '
  .version == "patchline.language-extractors/v1" and
  (.languages | sort) == ["go","java","python","ruby","typescript"] and
  .all_valid == true and
  .malformed_valid == false and
  (.malformed_missing | index("hazard"))
' "$OUT/extractors.json" > /dev/null
jq -n --slurpfile r "$OUT/extractors.json" '{version:"patchline.language-extractors-gate-results/v1", languages:$r[0].languages, all_valid:$r[0].all_valid, malformed_rejected:($r[0].malformed_valid|not), verified:true}' > "$OUT/gate-summary.json"
echo "language-extractors gate passed: five languages share one schema, malformed finding rejected"
