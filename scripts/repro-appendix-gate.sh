#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/repro-appendix-gate.json}"; OUT="${2:-results/generated/repro-appendix}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.repro-appendix-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "one-command" "make repro-appendix-gate"; do grep -F "$phrase" docs/repro-appendix.md README.md > /dev/null; done
bash scripts/repro-appendix.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.repro-appendix/v1" and .all_covered==true and .uncovered_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.repro-appendix-gate-results/v1",covered:$r[0].covered,all_covered:$r[0].all_covered,uncovered_rejected:($r[0].uncovered_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "repro-appendix gate passed: every claim maps to a one-command gate, uncovered claim rejected"
