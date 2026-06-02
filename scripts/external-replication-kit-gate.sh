#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/external-replication-kit-gate.json}"; OUT="${2:-results/generated/external-replication-kit}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.external-replication-kit-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "frozen" "make external-replication-kit-gate"; do grep -F "$phrase" docs/external-replication-kit.md README.md > /dev/null; done
bash scripts/external-replication-kit.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '
  .version == "patchline.external-replication-kit/v1" and
  .all_reproduced == true and .reproduced_fraction == 1 and .tamper_detected == true
' "$OUT/kit.json" > /dev/null
jq -n --slurpfile r "$OUT/kit.json" '{version:"patchline.external-replication-kit-gate-results/v1", reproduced_fraction:$r[0].reproduced_fraction, tamper_detected:$r[0].tamper_detected, verified:true}' > "$OUT/gate-summary.json"
echo "external-replication-kit gate passed: every claim reproduced, tampered value detected"
