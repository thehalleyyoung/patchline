#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/one-command-paper-repro-gate.json}"; OUT="${2:-results/generated/one-command-paper-repro}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.one-command-paper-repro-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "clean container" "make one-command-paper-repro-gate"; do grep -F "$phrase" docs/one-command-paper-repro.md README.md > /dev/null; done
bash scripts/one-command-paper-repro.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.one-command-paper-repro/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.one-command-paper-repro-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "one-command-paper-repro gate passed: every item scored with evidence on real self-data, unsupported item rejected"
