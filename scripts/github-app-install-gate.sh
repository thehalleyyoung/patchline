#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/github-app-install-gate.json}"; OUT="${2:-results/generated/github-app-install}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.github-app-install-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "least-privilege" "make github-app-install-gate"; do grep -F "$phrase" docs/github-app-install.md README.md > /dev/null; done
bash scripts/github-app-install.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.github-app-install/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.github-app-install-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "github-app-install gate passed: every scope least-privilege and audited, over-broad scope rejected"
