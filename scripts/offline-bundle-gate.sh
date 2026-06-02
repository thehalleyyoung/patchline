#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/offline-bundle-gate.json}"
OUT="${2:-results/generated/offline-bundle-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.offline-bundle-gate/v1" and (.required_members|length)==4' "$SPEC" > /dev/null

for phrase in "Offline runtime-evidence bundle" "self-contained" "MANIFEST" "make offline-bundle-gate"; do
  grep -F "$phrase" docs/offline-bundle.md README.md > /dev/null
done

bash scripts/offline-bundle.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in offline-bundle.json offline-bundle.md README.md; do
  test -s "$OUT/$output"
done
for member in findings.json runtime-evidence.jsonl MANIFEST.json INDEX.md MANIFEST.checks; do
  test -s "$OUT/bundle/$member"
done

minf="$(jq '.minimum_findings' "$SPEC")"

jq -e --argjson minf "$minf" '
  .version == "patchline.offline-bundle/v1" and
  .findings >= $minf and
  .checksums_valid == true and
  .self_contained == true and
  .manifest_complete == true and
  .rebuild_deterministic == true and
  .linkage_preserved == true and
  .required_members_present == true
' "$OUT/offline-bundle.json" > /dev/null

# Independent offline re-verification of the produced bundle.
( cd "$OUT/bundle" && shasum -a 256 -c MANIFEST.checks > /dev/null )

jq -n --slurpfile r "$OUT/offline-bundle.json" '{
  version: "patchline.offline-bundle-gate-results/v1",
  findings: $r[0].findings,
  manifest_files: $r[0].manifest_files,
  checksums_valid: $r[0].checksums_valid,
  self_contained: $r[0].self_contained,
  rebuild_deterministic: $r[0].rebuild_deterministic,
  verified: true
}' > "$OUT/gate-summary.json"

echo "offline bundle gate passed: checksums $(jq '.checksums_valid' "$OUT/gate-summary.json"), self-contained $(jq '.self_contained' "$OUT/gate-summary.json")"
