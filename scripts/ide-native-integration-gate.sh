#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/ide-native-integration-gate.json}"; OUT="${2:-results/generated/ide-native-integration}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.ide-native-integration-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "IDE-native integration" "make ide-native-integration-gate"; do grep -F "$phrase" docs/ide-native-integration.md README.md > /dev/null; done
bash scripts/ide-native-integration.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.ide-native-integration/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.ide-native-integration-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "ide-native-integration gate passed: every item scored with evidence on real self-data, unsupported item rejected"
