#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/foundation-model-finetune-gate.json}"; OUT="${2:-results/generated/foundation-model-finetune}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.foundation-model-finetune-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "post-hoc verification" "make foundation-model-finetune-gate"; do grep -F "$phrase" docs/foundation-model-finetune.md README.md > /dev/null; done
bash scripts/foundation-model-finetune.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.foundation-model-finetune/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.foundation-model-finetune-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "foundation-model-finetune gate passed: every item scored with evidence on real self-data, unsupported item rejected"
