#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/rebuttal-response-workspace-gate.json}"
OUT="${2:-results/generated/rebuttal-response-workspace-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '
  .version == "patchline.rebuttal-response-workspace-gate/v1" and
  (.concerns | length) >= .minimum_concerns and
  (.required_outputs | length) >= (.concerns | length)
' "$SPEC" > /dev/null

for phrase in "rebuttal-response workspace" "reviewer concerns" "evidence" "limitations" "make rebuttal-response-workspace-gate"; do
  grep -F "$phrase" docs/rebuttal-response-workspace.md README.md > /dev/null
done

bash scripts/generate-rebuttal-response-workspace.sh "$SPEC" "$OUT" > "$OUT.run.log"

while read -r output; do
  test -s "$OUT/$output"
done < <(jq -r '.required_outputs[]' "$SPEC")

min_concerns="$(jq '.minimum_concerns' "$SPEC")"
min_repos="$(jq '.minimum_public_repos' "$SPEC")"
min_evidence="$(jq '.minimum_evidence_links_per_concern' "$SPEC")"
min_limitations="$(jq '.minimum_limitations_linked' "$SPEC")"
jq -e --argjson min_concerns "$min_concerns" --argjson min_repos "$min_repos" --argjson min_evidence "$min_evidence" --argjson min_limitations "$min_limitations" '
  .version == "patchline.rebuttal-response-workspace/v1" and
  .summary.concerns >= $min_concerns and
  .summary.answerable == .summary.concerns and
  .summary.public_repos >= $min_repos and
  .summary.ranked_risks > 100 and
  .summary.linked_limitations >= $min_limitations and
  .summary.verified == true and
  (.evidence.release_doi | startswith("10.")) and
  all(.responses[]; (.evidence_links | length) >= $min_evidence and (.limitations | length) > 0 and (.reviewer_check | length) > 40 and (.status == "answerable-with-linked-limitation"))
' "$OUT/rebuttal-workspace.json" > /dev/null

for concern in reproducibility availability functional-correctness scope-of-claims generated-output-safety ecosystem-generalization paper-figure-drift; do
  test -s "$OUT/responses/$concern.md"
  grep -F "## Evidence links" "$OUT/responses/$concern.md" > /dev/null
  grep -F "## Linked limitations" "$OUT/responses/$concern.md" > /dev/null
  grep -F "## Reproduction commands" "$OUT/responses/$concern.md" > /dev/null
done

grep -F "Patchline generated paper appendix" "$OUT/evidence/paper-appendix/appendix.md" > /dev/null
grep -F "Patchline artifact DOI/release manifest" "$OUT/evidence/release-manifest/artifact-release-manifest.md" > /dev/null
grep -F "Response index" "$OUT/rebuttal-workspace.md" > /dev/null

jq -n \
  --slurpfile workspace "$OUT/rebuttal-workspace.json" \
  '{
    version:"patchline.rebuttal-response-workspace-gate-results/v1",
    concerns:$workspace[0].summary.concerns,
    answerable:$workspace[0].summary.answerable,
    linked_limitations:$workspace[0].summary.linked_limitations,
    public_repos:$workspace[0].summary.public_repos,
    ranked_risks:$workspace[0].summary.ranked_risks,
    verified:true
  }' > "$OUT/gate-summary.json"

echo "rebuttal workspace gate passed: concerns $(jq '.concerns' "$OUT/gate-summary.json"), limitations $(jq '.linked_limitations' "$OUT/gate-summary.json"), risks $(jq '.ranked_risks' "$OUT/gate-summary.json")"
