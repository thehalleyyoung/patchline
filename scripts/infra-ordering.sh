#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/infra-ordering-gate.json}"
OUT="${2:-results/generated/infra-ordering}"
rm -rf "$OUT"
mkdir -p "$OUT/cache" "$OUT/analysis"

jq -e '
  .version == "patchline.infra-ordering-gate/v1" and
  (.claim | length) > 200
' "$SPEC" > /dev/null

repo="$(jq -r '.real_repo.repo' "$SPEC")"
ref="$(jq -r '.real_repo.ref' "$SPEC")"
min_seq="$(jq -r '.real_repo.minimum_sequenced' "$SPEC")"
min_unordered="$(jq -r '.real_repo.minimum_unordered' "$SPEC")"

go run ./cmd/patchline repo analyze \
  --github "$repo" --ref "$ref" \
  --download-dir "$OUT/cache" --stages inventory --no-llm \
  --out "$OUT/analysis" --json > "$OUT/analyze.log"

INV="$OUT/analysis/inventory/inventory.json"
test -s "$INV"

jq '.infra_data_ordering // []' "$INV" > "$OUT/infra-ordering.json"

sequenced="$(jq '[.[] | select(.kind == "infra_data_ordering_sequenced")] | length' "$OUT/infra-ordering.json")"
unordered="$(jq '[.[] | select(.kind == "infra_data_ordering_unordered")] | length' "$OUT/infra-ordering.json")"

real_repo_detected=false
if [ "$sequenced" -ge "$min_seq" ] && [ "$unordered" -ge "$min_unordered" ]; then
  real_repo_detected=true
fi

go test ./internal/project/ -run 'TestInventoryAnalyzesInfraDataOrdering' \
  > "$OUT/unit-tests.log" 2>&1 && unit_ok=true || unit_ok=false
rm -rf internal/project/results

jq -n \
  --arg repo "$repo" \
  --argjson sequenced "$sequenced" \
  --argjson unordered "$unordered" \
  --argjson real_detected "$real_repo_detected" \
  --argjson unit_ok "$unit_ok" '
  {
    version: "patchline.infra-ordering/v1",
    real_repo: $repo,
    sequenced_jobs: $sequenced,
    unordered_jobs: $unordered,
    real_repo_detected: $real_detected,
    ordering_matrix_verified: $unit_ok
  }
' > "$OUT/infra-ordering-summary.json"

{
  echo "# Infrastructure/data ordering analysis"
  echo
  jq -r '"In the real `" + .real_repo + "` repository Patchline classified `" + (.sequenced_jobs|tostring) + "` migration/database jobs as sequenced by explicit deploy-ordering markers and `" + (.unordered_jobs|tostring) + "` as unordered data-change risks that can race the rollout."' "$OUT/infra-ordering-summary.json"
  echo
  echo "## Guarantees"
  jq -r '"- real-repo sequenced and unordered jobs both detected: `" + (.real_repo_detected|tostring) + "`\n- ordered/unordered classification verified by unit tests: `" + (.ordering_matrix_verified|tostring) + "`"' "$OUT/infra-ordering-summary.json"
  echo
  echo "Patchline correlates Helm hooks, Argo sync-waves, initContainers, and Terraform depends_on/waits with the migration and database jobs on the same manifest, so reviewers can immediately see whether a data change is sequenced relative to its deploy or able to race it."
} > "$OUT/infra-ordering.md"
cp "$OUT/infra-ordering.md" "$OUT/README.md"

echo "infra-ordering analysis complete: $(jq '.sequenced_jobs' "$OUT/infra-ordering-summary.json") sequenced / $(jq '.unordered_jobs' "$OUT/infra-ordering-summary.json") unordered on real repo, matrix verified $(jq '.ordering_matrix_verified' "$OUT/infra-ordering-summary.json")"
