#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/lts-release-line-gate.json}"; OUT="${2:-results/generated/lts-release-line}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.lts-release-line-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "backport" "make lts-release-line-gate"; do grep -F "$phrase" docs/lts-release-line.md README.md > /dev/null; done
bash scripts/lts-release-line.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.lts-release-line/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.lts-release-line-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "lts-release-line gate passed: every LTS release backported with EOL, EOL-less release rejected"
