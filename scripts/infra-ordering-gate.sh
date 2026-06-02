#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/infra-ordering-gate.json}"
OUT="${2:-results/generated/infra-ordering-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.infra-ordering-gate/v1"' "$SPEC" > /dev/null

for phrase in "Helm" "sync-wave" "initContainers" "Terraform" "sequenced" "unordered" "make infra-ordering-gate"; do
  grep -F "$phrase" docs/infra-ordering.md README.md > /dev/null
done

bash scripts/infra-ordering.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in infra-ordering-summary.json infra-ordering.md README.md infra-ordering.json; do
  test -s "$OUT/$output"
done

jq -e '
  .version == "patchline.infra-ordering/v1" and
  .real_repo_detected == true and
  .ordering_matrix_verified == true and
  .sequenced_jobs >= 5 and
  .unordered_jobs >= 1
' "$OUT/infra-ordering-summary.json" > /dev/null

# Independently confirm a real migration job is sequenced by a helm hook (e.g. Kong migrations).
kong="$(jq '[.[] | select(.kind == "infra_data_ordering_sequenced" and (.path | test("migration|upgrade_job|migrations")))] | length' "$OUT/infra-ordering.json")"
if [ "$kong" -lt 1 ]; then echo "no sequenced migration job detected"; exit 1; fi

jq -n --slurpfile r "$OUT/infra-ordering-summary.json" '{
  version: "patchline.infra-ordering-gate-results/v1",
  real_repo: $r[0].real_repo,
  sequenced_jobs: $r[0].sequenced_jobs,
  unordered_jobs: $r[0].unordered_jobs,
  verified: true
}' > "$OUT/gate-summary.json"

echo "infra-ordering gate passed: sequenced and unordered data-change jobs classified on real repo, ordering matrix verified"
