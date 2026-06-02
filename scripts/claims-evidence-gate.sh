#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/claims-evidence-gate.json}"
OUT="${2:-results/generated/claims-evidence-gate}"
rm -rf "$OUT"
mkdir -p "$OUT/cache"

jq -e '
  .version == "patchline.claims-evidence-gate/v1" and
  (.claim | length) > 150 and
  (.required_sections | length) == 3 and
  (.required_fields | length) >= 10 and
  .minimum_public_repos >= 4 and
  .minimum_claims >= 6 and
  (.real_code | length) >= .minimum_public_repos and
  all(.real_code[]; (.repo | length) > 0 and (.ref | test("^[0-9a-f]{40}$")) and (.subpath | length) > 0)
' "$SPEC" > /dev/null

for field in claims section evidence artifacts limitations missing_evidence paper_wording reviewer_check expected_paper_slot abstract introduction evaluation; do
  grep -F "$field" docs/claims-evidence.md > /dev/null
done
grep -F "make claims-evidence-gate" README.md > /dev/null

go test ./cmd/patchline -run TestRepoClaimsEvidenceMapsPaperClaimsToArtifacts > "$OUT/go-test.log"

analysis_dirs=()
count="$(jq '.real_code | length' "$SPEC")"
for ((i=0; i<count; i++)); do
  id="$(jq -r ".real_code[$i].id" "$SPEC")"
  repo="$(jq -r ".real_code[$i].repo" "$SPEC")"
  ref="$(jq -r ".real_code[$i].ref" "$SPEC")"
  subpath="$(jq -r ".real_code[$i].subpath" "$SPEC")"
  analysis="$OUT/analyses/$id"
  analysis_dirs+=("$analysis")
  go run ./cmd/patchline repo analyze \
    --github "$repo" \
    --ref "$ref" \
    --subpath "$subpath" \
    --download-dir "$OUT/cache" \
    --stages inventory,baseline,propose,compare \
    --proposal-kind all \
    --budget files=8,lines=100,tokens=12000,changes=2 \
    --no-llm \
    --out "$analysis" \
    --json > "$OUT/analyze-$i.json"
done

IFS=,
analyses="${analysis_dirs[*]}"
unset IFS

go run ./cmd/patchline repo claims-evidence \
  --analyses "$analyses" \
  --out "$OUT/claims" \
  --json > "$OUT/claims-stdout.json"

min_repos="$(jq '.minimum_public_repos' "$SPEC")"
min_claims="$(jq '.minimum_claims' "$SPEC")"
sections="$(jq -c '.required_sections' "$SPEC")"
jq -e --argjson min_repos "$min_repos" --argjson min_claims "$min_claims" --argjson sections "$sections" '
  .version == "patchline.repo-claims-evidence/v1" and
  .summary.analyses >= $min_repos and
  .summary.public_repos >= $min_repos and
  .summary.claims >= $min_claims and
  .summary.supported_claims > 0 and
  .summary.claims_with_limitations == .summary.claims and
  . as $report |
  all($sections[]; ($report.summary.by_section[.] // 0) > 0) and
  all(.claims[];
    (.section | IN("abstract", "introduction", "evaluation")) and
    (.claim | length) > 40 and
    (.status | IN("supported", "qualified")) and
    (.evidence | length) > 0 and
    (.artifacts | length) > 0 and
    (.limitations | length) > 0 and
    (.missing_evidence | length) > 0 and
    (.paper_wording | length) > 40 and
    (.reviewer_check | length) > 30 and
    (.expected_paper_slot | length) > 0
  )
' "$OUT/claims/claims-evidence.json" > /dev/null

for repo in $(jq -r '.real_code[].repo' "$SPEC"); do
  grep -F "$repo" "$OUT/claims/claims-evidence.md" > /dev/null
done
for section in $(jq -r '.required_sections[]' "$SPEC"); do
  grep -F "$section" "$OUT/claims/claims-evidence.md" > /dev/null
done
grep -F "claims-to-evidence map" "$OUT/claims/claims-evidence.md" > /dev/null
grep -F "missing evidence" "$OUT/claims/claims-evidence.md" > /dev/null

jq -n \
  --slurpfile claims "$OUT/claims/claims-evidence.json" \
  '{
    version:"patchline.claims-evidence-gate-results/v1",
    analyses:$claims[0].summary.analyses,
    public_repos:$claims[0].summary.public_repos,
    claims:$claims[0].summary.claims,
    supported_claims:$claims[0].summary.supported_claims,
    qualified_claims:$claims[0].summary.qualified_claims,
    claims_with_limitations:$claims[0].summary.claims_with_limitations,
    hash:$claims[0].hash,
    verified:true
  }' > "$OUT/summary.json"

jq -e --argjson min_repos "$min_repos" --argjson min_claims "$min_claims" '.verified == true and .public_repos >= $min_repos and .claims >= $min_claims and .claims_with_limitations == .claims' "$OUT/summary.json" > /dev/null

echo "claims evidence gate passed: claims $(jq '.claims' "$OUT/summary.json"), public repos $(jq '.public_repos' "$OUT/summary.json")"
