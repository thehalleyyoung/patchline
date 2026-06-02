#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/artifact-container-profile-gate.json}"
OUT="${2:-results/generated/artifact-container-profile-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '
  .version == "patchline.artifact-container-profile-gate/v1" and
  (.one_command == "bash scripts/artifact-container-rebuild.sh") and
  (.container.dockerfile | startswith("packaging/artifact/")) and
  (.public_results.required_outputs | length) >= 7 and
  (.host_independence.forbidden_path_prefixes | length) >= 4
' "$SPEC" > /dev/null

for phrase in "one-command artifact VM/container profile" "host-specific assumptions" "public results" "make artifact-container-profile-gate"; do
  grep -F "$phrase" docs/artifact-container-profile.md README.md > /dev/null
done

dockerfile="$(jq -r '.container.dockerfile' "$SPEC")"
grep -Eq '^FROM golang:' "$dockerfile"
grep -F 'CMD ["bash", "scripts/artifact-container-rebuild.sh"]' "$dockerfile" > /dev/null
grep -F 'PATCHLINE_ARTIFACT_MODE=container' "$dockerfile" > /dev/null
while IFS= read -r package; do
  grep -F "$package" "$dockerfile" > /dev/null
done < <(jq -r '.container.required_packages[]' "$SPEC")

if grep -R -nE '(/Users/|/home/|C:\\\\|Darwin|halleyyoung)' "$dockerfile" scripts/artifact-container-rebuild.sh docs/artifact-container-profile.md; then
  echo "artifact container profile contains host-specific assumptions" >&2
  exit 1
fi

bash scripts/artifact-container-rebuild.sh "$SPEC" "$OUT" > "$OUT.run.log"

while read -r expected; do
  test -s "$OUT/$expected"
done < <(jq -r '.public_results.required_outputs[]' "$SPEC")

min_repos="$(jq '.minimum_public_repos' "$SPEC")"
min_risks="$(jq '.minimum_ranked_risks' "$SPEC")"
min_generated="$(jq '.minimum_generated_files' "$SPEC")"
jq -e --argjson min_repos "$min_repos" --argjson min_risks "$min_risks" --argjson min_generated "$min_generated" '
  . as $root |
  .version == "patchline.artifact-container-rebuild/v1" and
  .summary.public_repos >= $min_repos and
  .summary.ranked_risks >= $min_risks and
  .summary.generated_files >= $min_generated and
  .summary.rejected_examples >= 2 and
  .summary.evidence_artifact_types >= 6 and
  .summary.no_production_credentials == true and
  .summary.no_host_database_services == true and
  .summary.no_host_language_toolchains == true and
  .summary.verified == true and
  (.profile.workspace | startswith("/workspace/")) and
  (.profile.results_dir | startswith($root.profile.workspace))
' "$OUT/rebuild-summary.json" > /dev/null

grep -F "docker build -f packaging/artifact/Dockerfile" "$OUT/profile/rebuild-command.sh" > /dev/null
grep -F "docker run --rm patchline-artifact-rebuild:local" "$OUT/profile/rebuild-command.sh" > /dev/null
grep -F "Patchline release-quality capstone demo" "$OUT/public-results/capstone/session.md" > /dev/null
grep -F "Host-independence guarantees" "$OUT/README.md" > /dev/null

container_status="recipe-verified"
container_log="$OUT/container.log"
if command -v docker >/dev/null 2>&1 && docker version --format '{{.Server.Version}}' >/dev/null 2>&1; then
  image="patchline-artifact-profile-gate:$(date +%s)"
  docker build -f "$dockerfile" -t "$image" . > "$container_log" 2>&1
  docker run --rm "$image" bash scripts/artifact-container-rebuild.sh "$SPEC" results/generated/artifact-container-profile-container-run >> "$container_log" 2>&1
  container_status="executed"
else
  printf 'docker daemon unavailable; validated artifact Dockerfile and executed the same rebuild command on the current host\n' > "$container_log"
fi

jq -n \
  --slurpfile rebuild "$OUT/rebuild-summary.json" \
  --arg container_status "$container_status" \
  --arg container_log "$container_log" \
  '{
    version:"patchline.artifact-container-profile-gate-results/v1",
    container_status:$container_status,
    container_log:$container_log,
    public_repos:$rebuild[0].summary.public_repos,
    ranked_risks:$rebuild[0].summary.ranked_risks,
    generated_files:$rebuild[0].summary.generated_files,
    host_independent:($rebuild[0].summary.no_production_credentials and $rebuild[0].summary.no_host_database_services and $rebuild[0].summary.no_host_language_toolchains),
    verified:true
  }' > "$OUT/gate-summary.json"

echo "artifact container profile gate passed: repos $(jq '.public_repos' "$OUT/gate-summary.json"), risks $(jq '.ranked_risks' "$OUT/gate-summary.json"), container $(jq -r '.container_status' "$OUT/gate-summary.json")"
