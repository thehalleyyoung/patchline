#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/capability-scoped-sandbox-gate.json}"; OUT="${2:-results/generated/capability-scoped-sandbox}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.capability-scoped-sandbox-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "no-side-effect-outside-scope" "make capability-scoped-sandbox-gate"; do grep -F "$phrase" docs/capability-scoped-sandbox.md README.md > /dev/null; done
bash scripts/capability-scoped-sandbox.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.capability-scoped-sandbox/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.capability-scoped-sandbox-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "capability-scoped-sandbox gate passed: every item scored with evidence on real self-data, unsupported item rejected"
