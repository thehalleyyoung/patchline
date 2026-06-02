#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/dataset-release-package-gate.json}"; OUT="${2:-results/generated/dataset-release-package}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.dataset-release-package-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "datasheet" "make dataset-release-package-gate"; do grep -F "$phrase" docs/dataset-release-package.md README.md > /dev/null; done
bash scripts/dataset-release-package.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.dataset-release-package/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.dataset-release-package-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "dataset-release-package gate passed: every release requirement present, missing requirement rejected"
