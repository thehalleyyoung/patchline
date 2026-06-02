#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/independent-replication-gate.json}"
OUT="${2:-results/generated/independent-replication-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '
  .version == "patchline.independent-replication-gate/v1" and
  (.forbidden_tools | length) >= 5 and
  (.forbidden_environment | length) >= 4 and
  (.required_outputs | length) >= 6
' "$SPEC" > /dev/null

for phrase in "independent replication instructions" "GitHub credentials" "proprietary tooling" "anonymous public archive" "make independent-replication-gate"; do
  grep -F "$phrase" docs/independent-replication.md README.md > /dev/null
done

env -u GITHUB_TOKEN -u GH_TOKEN -u GITHUB_PAT -u DATADOG_API_KEY \
  bash scripts/generate-independent-replication.sh "$SPEC" "$OUT" > "$OUT.run.log"

while read -r output; do
  test -s "$OUT/$output"
done < <(jq -r '.required_outputs[]' "$SPEC")

min_repos="$(jq '.minimum_public_repos' "$SPEC")"
min_archives="$(jq '.minimum_archives' "$SPEC")"
min_verified="$(jq '.minimum_verified_archives' "$SPEC")"
min_commands="$(jq '.minimum_commands' "$SPEC")"
jq -e --argjson min_repos "$min_repos" --argjson min_archives "$min_archives" --argjson min_verified "$min_verified" --argjson min_commands "$min_commands" '
  .version == "patchline.independent-replication/v1" and
  .summary.public_repos >= $min_repos and
  .summary.archives >= $min_archives and
  .summary.verified_archives >= $min_verified and
  .summary.credential_free_urls == .summary.archives and
  .summary.commands >= $min_commands and
  .summary.credentials_required == false and
  .summary.proprietary_tooling_required == false and
  .summary.ranked_risks > 100 and
  .summary.verified == true and
  all(.archive_verification.archives[]; .verified == true and .credential_free == true and (.anonymous_archive_url | startswith("https://codeload.github.com/")))
' "$OUT/independent-replication.json" > /dev/null

if grep -R -nE '(^|[[:space:]])gh([[:space:]]|$)|GITHUB_TOKEN=|GH_TOKEN=|GITHUB_PAT=|DATADOG_API_KEY=' "$OUT/independent-replication.md" "$OUT/replication-commands.sh"; then
  echo "independent replication path includes credentials or forbidden CLI usage" >&2
  exit 1
fi

for tool in bash curl git go jq make shasum; do
  grep -F "\"tool\": \"$tool\"" "$OUT/no-credential-environment.json" > /dev/null
done
grep -F "curl -L 'https://codeload.github.com/" "$OUT/replication-commands.sh" > /dev/null
grep -F "shasum -a 256 -c -" "$OUT/replication-commands.sh" > /dev/null
grep -F "credentials required: \`false\`" "$OUT/independent-replication.md" > /dev/null
grep -F "Patchline artifact DOI/release manifest" "$OUT/evidence/release-manifest/artifact-release-manifest.md" > /dev/null

jq -n \
  --slurpfile replication "$OUT/independent-replication.json" \
  '{
    version:"patchline.independent-replication-gate-results/v1",
    public_repos:$replication[0].summary.public_repos,
    archives:$replication[0].summary.archives,
    verified_archives:$replication[0].summary.verified_archives,
    ranked_risks:$replication[0].summary.ranked_risks,
    credentials_required:$replication[0].summary.credentials_required,
    proprietary_tooling_required:$replication[0].summary.proprietary_tooling_required,
    verified:true
  }' > "$OUT/gate-summary.json"

echo "independent replication gate passed: archives $(jq '.verified_archives' "$OUT/gate-summary.json"), risks $(jq '.ranked_risks' "$OUT/gate-summary.json"), credentials $(jq '.credentials_required' "$OUT/gate-summary.json")"
