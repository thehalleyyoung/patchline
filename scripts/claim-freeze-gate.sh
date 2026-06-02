#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/claim-freeze-gate.json}"
OUT="${2:-results/generated/claim-freeze-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.claim-freeze-gate/v1" and (.claim|length) > 200 and (.claims|length) >= 1' "$SPEC" > /dev/null

for phrase in "claim freeze" "drift" "make claim-freeze-gate"; do
  grep -F "$phrase" docs/claim-freeze.md README.md > /dev/null
done

bash scripts/claim-freeze.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in claim-freeze.json freeze.json claim-freeze.md README.md; do
  test -s "$OUT/$output"
done

# Unchanged artifacts re-verify cleanly; the tampered copy is flagged as drift.
jq -e '
  .version == "patchline.claim-freeze/v1" and
  .no_drift == true and
  .tamper_drift_detected == true and
  (.verifications | all(.[]; .match))
' "$OUT/claim-freeze.json" > /dev/null

jq -n --slurpfile r "$OUT/claim-freeze.json" '{
  version: "patchline.claim-freeze-gate-results/v1",
  no_drift: $r[0].no_drift,
  tamper_drift_detected: $r[0].tamper_drift_detected,
  verified: true
}' > "$OUT/gate-summary.json"

echo "claim-freeze gate passed: live artifacts match freeze, tampered copy flagged as drift"
