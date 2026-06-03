#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/federated-benchmark-split-gate.json}"
OUT="${2:-results/generated/federated-benchmark-split}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.federated-benchmark-split-gate/v1" and (.claim|length) > 200 and (.private_cases|length) >= .min_private_cases' "$SPEC" > /dev/null
for phrase in "signed aggregate metrics" "make federated-benchmark-split-gate"; do
  grep -F "$phrase" docs/federated-benchmark-split.md README.md > /dev/null
done

bash scripts/federated-benchmark-split.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.federated-benchmark-split/v1" and .ok==true and .all_ok==true and .bad_ok==false and .signed==true and .aggregate_only==true' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.federated-benchmark-split-gate-results/v1",ok:$r[0].ok,signed:$r[0].signed,aggregate_only:$r[0].aggregate_only,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "federated-benchmark-split gate passed: private benchmark ran locally, signed aggregate verified, tampering/leakage rejected"

