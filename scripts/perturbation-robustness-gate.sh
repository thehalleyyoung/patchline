#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/perturbation-robustness-gate.json}"; OUT="${2:-results/generated/perturbation-robustness}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.perturbation-robustness-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "robustness" "make perturbation-robustness-gate"; do grep -F "$phrase" docs/perturbation-robustness.md README.md > /dev/null; done
bash scripts/perturbation-robustness.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '
  .version == "patchline.perturbation-robustness/v1" and
  .fully_stable == true and .stability_rate == 1 and .semantic_flips == true
' "$OUT/robust.json" > /dev/null
jq -n --slurpfile r "$OUT/robust.json" '{version:"patchline.perturbation-robustness-gate-results/v1", stability_rate:$r[0].stability_rate, semantic_flips:$r[0].semantic_flips, verified:true}' > "$OUT/gate-summary.json"
echo "perturbation-robustness gate passed: stable under cosmetic perturbations, sensitive to semantic change"
