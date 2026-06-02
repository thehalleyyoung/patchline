#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/reproducibility-vault-gate.json}"; OUT="${2:-results/generated/reproducibility-vault}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.reproducibility-vault-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "snapshot" "make reproducibility-vault-gate"; do grep -F "$phrase" docs/reproducibility-vault.md README.md > /dev/null; done
bash scripts/reproducibility-vault.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.reproducibility-vault/v1" and .all_complete==true and .incomplete_complete==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.reproducibility-vault-gate-results/v1",all_complete:$r[0].all_complete,incomplete_rejected:($r[0].incomplete_complete|not),verified:true}' > "$OUT/gate-summary.json"
echo "reproducibility-vault gate passed: every release snapshot complete and verifiable, incomplete snapshot rejected"
