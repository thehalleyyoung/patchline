#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/privacy-impact-gate.json}"
OUT="${2:-results/generated/privacy-impact-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.privacy-impact-gate/v1" and (.claim|length) > 200 and (.operations|length) >= 1' "$SPEC" > /dev/null

for phrase in "privacy impact" "anonymization" "make privacy-impact-gate"; do
  grep -F "$phrase" docs/privacy-impact.md README.md > /dev/null
done

bash scripts/privacy-impact.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in privacy-impact.json privacy-impact.md README.md; do
  test -s "$OUT/$output"
done

# PII export high; PII delete erasure-relevant; PII anonymize mitigating; PII retention
# relevant; non-PII op none.
jq -e '
  .version == "patchline.privacy-impact/v1" and
  ([.findings[] | select(.id=="export_pii")][0].impact == "high") and
  ([.findings[] | select(.id=="delete_pii")][0].impact == "erasure_relevant") and
  ([.findings[] | select(.id=="anonymize_pii")][0].impact == "mitigating") and
  ([.findings[] | select(.id=="retention_pii")][0].impact == "relevant") and
  ([.findings[] | select(.id=="nonpii_op")][0].impact == "none")
' "$OUT/privacy-impact.json" > /dev/null

jq -n --slurpfile r "$OUT/privacy-impact.json" '{
  version: "patchline.privacy-impact-gate-results/v1",
  findings: [$r[0].findings[] | {id, impact}],
  verified: true
}' > "$OUT/gate-summary.json"

echo "privacy-impact gate passed: export high, delete erasure, anonymize mitigating, retention relevant, non-PII none"
