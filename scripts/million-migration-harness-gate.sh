#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/million-migration-harness-gate.json}"; OUT="${2:-results/generated/million-migration-harness}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.million-migration-harness-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "sharded" "make million-migration-harness-gate"; do grep -F "$phrase" docs/million-migration-harness.md README.md > /dev/null; done
bash scripts/million-migration-harness.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.million-migration-harness/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.million-migration-harness-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "million-migration-harness gate passed: every shard resumable and on-budget, over-budget shard rejected"
