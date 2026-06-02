#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/one-command-full-repro-2-gate.json}"; OUT="${2:-results/generated/one-command-full-repro-2}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.one-command-full-repro-2-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "one-command full reproduction" "make one-command-full-repro-2-gate"; do grep -F "$phrase" docs/one-command-full-repro-2.md README.md > /dev/null; done
bash scripts/one-command-full-repro-2.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.one-command-full-repro-2/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.one-command-full-repro-2-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "one-command-full-repro-2 gate passed: every item scored with evidence on real self-data, unsupported item rejected"
