#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/quarantine-attestation-gate.json}"
OUT="${2:-results/generated/quarantine-attestation-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.quarantine-attestation-gate/v1" and (.required_attestations|length)==4' "$SPEC" > /dev/null

for phrase in "quarantine attestations" "non-execution" "manual-review" "fingerprint" "make quarantine-attestation-gate"; do
  grep -F "$phrase" docs/quarantine-attestation.md README.md > /dev/null
done

bash scripts/quarantine-attestation.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in attestations.jsonl quarantine-attestation.json quarantine-attestation.md README.md; do
  test -s "$OUT/$output"
done

minc="$(jq '.minimum_candidates' "$SPEC")"

jq -e --argjson minc "$minc" '
  .version == "patchline.quarantine-attestation/v1" and
  .candidates >= $minc and
  .none_executed == true and
  .all_quarantined == true and
  .all_require_manual_review == true and
  .none_auto_apply == true and
  .all_have_fingerprint == true and
  .no_proven_status == true and
  .fingerprint_stable == true
' "$OUT/quarantine-attestation.json" > /dev/null

# Every attestation row must explicitly state non-execution and quarantine.
jq -e 'select(.executed != false or .quarantined != true or .manual_review_required != true) | true' \
  "$OUT/attestations.jsonl" > /dev/null && { echo "found non-quarantined candidate"; exit 1; } || true

jq -n --slurpfile r "$OUT/quarantine-attestation.json" '{
  version: "patchline.quarantine-attestation-gate-results/v1",
  candidates: $r[0].candidates,
  none_executed: $r[0].none_executed,
  all_require_manual_review: $r[0].all_require_manual_review,
  fingerprint_stable: $r[0].fingerprint_stable,
  verified: true
}' > "$OUT/gate-summary.json"

echo "quarantine attestation gate passed: $(jq '.candidates' "$OUT/gate-summary.json") candidates, none executed $(jq '.none_executed' "$OUT/gate-summary.json")"
