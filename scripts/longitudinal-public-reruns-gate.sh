#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/longitudinal-public-reruns-gate.json}"
OUT="${2:-results/generated/longitudinal-public-reruns-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '
  . as $root |
  .version == "patchline.longitudinal-public-reruns-gate/v1" and
  (.slices | length) >= .minimum_repositories and
  all(.slices[]; (.commits | length) >= $root.minimum_commits_per_repository)
' "$SPEC" > /dev/null

for phrase in "longitudinal public-corpus reruns" "historical commits" "public repository" "risk delta" "make longitudinal-public-reruns-gate"; do
  grep -F "$phrase" docs/longitudinal-public-reruns.md README.md > /dev/null
done

bash scripts/longitudinal-public-reruns.sh "$SPEC" "$OUT" > "$OUT.run.log"

while read -r output; do
  test -s "$OUT/$output"
done < <(jq -r '.required_outputs[]' "$SPEC")

min_repos="$(jq '.minimum_repositories' "$SPEC")"
min_commits="$(jq '.minimum_commits_per_repository' "$SPEC")"
min_runs="$(jq '.minimum_total_runs' "$SPEC")"
min_risks="$(jq '.minimum_ranked_risks' "$SPEC")"
jq -e --argjson min_repos "$min_repos" --argjson min_commits "$min_commits" --argjson min_runs "$min_runs" --argjson min_risks "$min_risks" '
  .version == "patchline.longitudinal-public-reruns/v1" and
  .summary.repositories >= $min_repos and
  .summary.total_runs >= $min_runs and
  .summary.commits_per_repository_min >= $min_commits and
  .summary.files_scanned > 0 and
  .summary.ranked_risks >= $min_risks and
  .summary.verified == true and
  all(.runs[]; (.ref | test("^[0-9a-f]{40}$")) and (.date | test("^[0-9]{4}-[0-9]{2}-[0-9]{2}T")) and .verified == true) and
  all(.repository_trends[]; .commits >= $min_commits and (.hashes | length) >= $min_commits)
' "$OUT/longitudinal-reruns.json" > /dev/null

for repo in lobsters/lobsters django/django apache/airflow; do
  grep -F "$repo" "$OUT/longitudinal-reruns.md" > /dev/null
done
test "$(wc -l < "$OUT/runs.jsonl" | tr -d ' ')" -ge "$min_runs"
grep -F "Repository trends" "$OUT/longitudinal-reruns.md" > /dev/null

jq -n \
  --slurpfile report "$OUT/longitudinal-reruns.json" \
  '{
    version:"patchline.longitudinal-public-reruns-gate-results/v1",
    repositories:$report[0].summary.repositories,
    total_runs:$report[0].summary.total_runs,
    ranked_risks:$report[0].summary.ranked_risks,
    provenance_slices:$report[0].summary.provenance_slices,
    repositories_with_risk_delta:$report[0].summary.repositories_with_risk_delta,
    verified:true
  }' > "$OUT/gate-summary.json"

echo "longitudinal public reruns gate passed: repos $(jq '.repositories' "$OUT/gate-summary.json"), runs $(jq '.total_runs' "$OUT/gate-summary.json"), risks $(jq '.ranked_risks' "$OUT/gate-summary.json")"
