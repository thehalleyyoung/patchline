#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/artifact-release-manifest-gate.json}"
OUT="${2:-results/generated/artifact-release-manifest-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '
  .version == "patchline.artifact-release-manifest-gate/v1" and
  (.doi_prefix | startswith("10.")) and
  (.required_outputs | length) >= 6
' "$SPEC" > /dev/null

for phrase in "artifact DOI/release manifest" "exact refs" "archives" "checksums" "command versions" "make artifact-release-manifest-gate"; do
  grep -F "$phrase" docs/artifact-release-manifest.md README.md > /dev/null
done

bash scripts/generate-artifact-release-manifest.sh "$SPEC" "$OUT" > "$OUT.run.log"

while read -r output; do
  test -s "$OUT/$output"
done < <(jq -r '.required_outputs[]' "$SPEC")

min_repos="$(jq '.minimum_public_repos' "$SPEC")"
min_archives="$(jq '.minimum_archives' "$SPEC")"
min_checksums="$(jq '.minimum_checksums' "$SPEC")"
min_commands="$(jq '.minimum_command_versions' "$SPEC")"
jq -e --argjson min_repos "$min_repos" --argjson min_archives "$min_archives" --argjson min_checksums "$min_checksums" --argjson min_commands "$min_commands" '
  . as $root |
  .version == "patchline.artifact-release-manifest/v1" and
  (.release.doi | startswith($root.release.doi_prefix + ".")) and
  (.release.content_hash | test("^sha256:[0-9a-f]{64}$")) and
  .summary.public_repos >= $min_repos and
  .summary.archives >= $min_archives and
  .summary.archive_checksums >= $min_archives and
  .summary.artifact_checksums >= $min_checksums and
  .summary.command_versions >= $min_commands and
  .summary.ranked_risks > 100 and
  .summary.verified == true and
  all(.archives[]; (.url | startswith("https://codeload.github.com/")) and (.archive_hash | test("^sha256:[0-9a-f]{64}$")) and (.resolved_commit | test("^[0-9a-f]{40}$"))) and
  all(.command_versions[]; (.name | length) > 0 and (.version | length) > 0) and
  all(.generated_artifact_checksums[]; (.sha256 | test("^[0-9a-f]{64}$")) and (.path | length) > 0)
' "$OUT/artifact-release-manifest.json" > /dev/null

for repo in lobsters/lobsters django/django grafana/grafana apache/airflow; do
  grep -F "$repo" "$OUT/artifact-release-manifest.md" > /dev/null
done
for tool in go git bash jq make; do
  grep -F "\"name\": \"$tool\"" "$OUT/command-versions.json" > /dev/null
done
grep -F "DOI candidate" "$OUT/artifact-release-manifest.md" > /dev/null
grep -F "Patchline release-quality capstone demo" "$OUT/evidence/artifact-container-rebuild/public-results/capstone/session.md" > /dev/null

jq -n \
  --slurpfile manifest "$OUT/artifact-release-manifest.json" \
  '{
    version:"patchline.artifact-release-manifest-gate-results/v1",
    doi:$manifest[0].release.doi,
    archives:$manifest[0].summary.archives,
    artifact_checksums:$manifest[0].summary.artifact_checksums,
    command_versions:$manifest[0].summary.command_versions,
    public_repos:$manifest[0].summary.public_repos,
    verified:true
  }' > "$OUT/gate-summary.json"

echo "artifact release manifest gate passed: archives $(jq '.archives' "$OUT/gate-summary.json"), checksums $(jq '.artifact_checksums' "$OUT/gate-summary.json"), doi $(jq -r '.doi' "$OUT/gate-summary.json")"
