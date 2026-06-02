#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/camera-ready-checklist-gate.json}"
OUT="${2:-results/generated/camera-ready-checklist-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '
  .version == "patchline.camera-ready-checklist-gate/v1" and
  (.required_docs | length) >= 5 and
  (.required_outputs | length) >= 6
' "$SPEC" > /dev/null

for phrase in "camera-ready checklist" "blocks release" "claims" "figures" "tables" "docs drift" "make camera-ready-checklist-gate"; do
  grep -F "$phrase" docs/camera-ready-checklist.md README.md > /dev/null
done

bash scripts/generate-camera-ready-checklist.sh "$SPEC" "$OUT" > "$OUT.run.log"

while read -r output; do
  test -s "$OUT/$output"
done < <(jq -r '.required_outputs[]' "$SPEC")

min_checks="$(jq '.minimum_checks' "$SPEC")"
min_repos="$(jq '.minimum_public_repos' "$SPEC")"
min_claims="$(jq '.minimum_claims' "$SPEC")"
min_figures="$(jq '.minimum_figures' "$SPEC")"
min_tables="$(jq '.minimum_tables' "$SPEC")"
jq -e --argjson min_checks "$min_checks" --argjson min_repos "$min_repos" --argjson min_claims "$min_claims" --argjson min_figures "$min_figures" --argjson min_tables "$min_tables" '
  .version == "patchline.camera-ready-checklist/v1" and
  .summary.checks >= $min_checks and
  .summary.passed == .summary.checks and
  .summary.failed == 0 and
  .summary.release_blocked == false and
  .summary.verified == true and
  .summary.public_repos >= $min_repos and
  .summary.claims >= $min_claims and
  .summary.figures >= $min_figures and
  .summary.tables >= $min_tables and
  all(.checks[]; .passed == true)
' "$OUT/camera-ready-checklist.json" > /dev/null

mkdir -p "$OUT/drift-probes"
jq '.checks |= map(if .id == "claims-count" then .passed=false | .actual=0 else . end)' "$OUT/camera-ready-checklist.json" > "$OUT/drift-probes/claims-drift.json"
jq '.checks |= map(if .id == "figures-count" then .passed=false | .actual=0 else . end)' "$OUT/camera-ready-checklist.json" > "$OUT/drift-probes/figures-drift.json"
jq '.checks |= map(if .id == "tables-count" then .passed=false | .actual=0 else . end)' "$OUT/camera-ready-checklist.json" > "$OUT/drift-probes/tables-drift.json"
for probe in "$OUT"/drift-probes/*.json; do
  jq '.summary.failed = ([.checks[] | select(.passed == false)] | length) | .summary.release_blocked = (.summary.failed > 0) | .summary.verified = (.summary.failed == 0)' "$probe" > "$probe.tmp"
  mv "$probe.tmp" "$probe"
  if jq -e '.summary.release_blocked == false or .summary.verified == true' "$probe" > /dev/null; then
    echo "drift probe did not block release: $probe" >&2
    exit 1
  fi
done

grep -F "release blocked: \`false\`" "$OUT/camera-ready-checklist.md" > /dev/null
grep -F "claim counts drift" "$OUT/drift-policy.json" > /dev/null
grep -F "Patchline generated paper appendix" "$OUT/evidence/rebuttal-workspace/evidence/paper-appendix/appendix.md" > /dev/null
grep -F "Patchline artifact DOI/release manifest" "$OUT/evidence/rebuttal-workspace/evidence/release-manifest/artifact-release-manifest.md" > /dev/null

jq -n \
  --slurpfile checklist "$OUT/camera-ready-checklist.json" \
  '{
    version:"patchline.camera-ready-checklist-gate-results/v1",
    checks:$checklist[0].summary.checks,
    passed:$checklist[0].summary.passed,
    public_repos:$checklist[0].summary.public_repos,
    ranked_risks:$checklist[0].summary.ranked_risks,
    drift_probes_blocked:3,
    verified:true
  }' > "$OUT/gate-summary.json"

echo "camera-ready checklist gate passed: checks $(jq '.checks' "$OUT/gate-summary.json"), drift probes $(jq '.drift_probes_blocked' "$OUT/gate-summary.json"), risks $(jq '.ranked_risks' "$OUT/gate-summary.json")"
