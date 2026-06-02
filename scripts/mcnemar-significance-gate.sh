#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/mcnemar-significance-gate.json}"; OUT="${2:-results/generated/mcnemar-significance}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.mcnemar-significance-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "McNemar" "make mcnemar-significance-gate"; do grep -F "$phrase" docs/mcnemar-significance.md README.md > /dev/null; done
bash scripts/mcnemar-significance.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '
  .version == "patchline.mcnemar-significance/v1" and
  .improved.significant == true and .improved.ci_excludes_zero == true and
  .identical.statistic == 0 and .identical.significant == false and .identical.ci_excludes_zero == false
' "$OUT/mcnemar.json" > /dev/null
jq -n --slurpfile r "$OUT/mcnemar.json" '{version:"patchline.mcnemar-significance-gate-results/v1", improved:$r[0].improved, identical:$r[0].identical, verified:true}' > "$OUT/gate-summary.json"
echo "mcnemar-significance gate passed: real improvement significant (CI excludes 0), identical systems not significant"
