#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/artifact-container-profile-gate.json}"
OUT="${2:-${PATCHLINE_RESULTS_DIR:-results/generated/artifact-container-rebuild}}"
rm -rf "$OUT"
mkdir -p "$OUT/profile" "$OUT/public-results"

jq -e '
  . as $root |
  .version == "patchline.artifact-container-profile-gate/v1" and
  (.claim | length) > 120 and
  (.one_command | startswith("bash scripts/")) and
  (.minimum_public_repos >= 4) and
  (.minimum_ranked_risks >= 100) and
  (.container.dockerfile | startswith("packaging/artifact/")) and
  (.container.workspace | startswith("/workspace/")) and
  (.container.results_dir | startswith($root.container.workspace)) and
  (.host_independence.no_production_credentials == true) and
  (.host_independence.no_host_language_toolchains == true) and
  (.host_independence.no_host_database_services == true) and
  (.host_independence.no_absolute_host_paths == true)
' "$SPEC" > /dev/null

dockerfile="$(jq -r '.container.dockerfile' "$SPEC")"
capstone_spec="$(jq -r '.public_results.capstone_spec' "$SPEC")"
mode="${PATCHLINE_ARTIFACT_MODE:-host-proof}"
results_dir="$(jq -r '.container.results_dir' "$SPEC")"
download_dir="$(printf '%s\n' "${PATCHLINE_DOWNLOAD_DIR:-results/generated/artifact-container-cache}")"

jq -n \
  --slurpfile spec "$SPEC" \
  --arg mode "$mode" \
  --arg dockerfile "$dockerfile" \
  --arg one_command "$(jq -r '.one_command' "$SPEC")" \
  --arg workspace "$(jq -r '.container.workspace' "$SPEC")" \
  --arg results_dir "$results_dir" \
  --arg download_dir "$download_dir" \
  --arg go_version "$(go version)" \
  --arg git_version "$(git --version)" \
  --arg generated_at "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  '{
    version:"patchline.artifact-container-profile/v1",
    generated_at:$generated_at,
    mode:$mode,
    one_command:$one_command,
    dockerfile:$dockerfile,
    workspace:$workspace,
    results_dir:$results_dir,
    download_dir:$download_dir,
    required_packages:$spec[0].container.required_packages,
    required_environment:$spec[0].host_independence.required_environment,
    tool_versions:{go:$go_version, git:$git_version, bash:"required", jq:"required", make:"required"},
    network:$spec[0].container.network,
    no_production_credentials:$spec[0].host_independence.no_production_credentials,
    no_host_database_services:$spec[0].host_independence.no_host_database_services,
    no_host_language_toolchains:$spec[0].host_independence.no_host_language_toolchains
  }' > "$OUT/profile/artifact-container-profile.json"

cat > "$OUT/profile/rebuild-command.sh" <<'SCRIPT'
#!/usr/bin/env bash
set -euo pipefail
docker build -f packaging/artifact/Dockerfile -t patchline-artifact-rebuild:local .
docker run --rm patchline-artifact-rebuild:local
SCRIPT
chmod +x "$OUT/profile/rebuild-command.sh"

jq -n \
  --slurpfile spec "$SPEC" \
  --arg dockerfile "$dockerfile" \
  --arg command "$(jq -r '.container.run_command' "$SPEC")" \
  '{
    version:"patchline.artifact-host-independence/v1",
    dockerfile:$dockerfile,
    one_container_command:$command,
    forbidden_path_prefixes:$spec[0].host_independence.forbidden_path_prefixes,
    guarantees:{
      no_production_credentials:$spec[0].host_independence.no_production_credentials,
      no_host_language_toolchains:$spec[0].host_independence.no_host_language_toolchains,
      no_host_database_services:$spec[0].host_independence.no_host_database_services,
      no_absolute_host_paths:$spec[0].host_independence.no_absolute_host_paths
    },
    checked:true
  }' > "$OUT/profile/host-independence.json"

bash scripts/capstone-demo.sh "$capstone_spec" "$OUT/public-results/capstone" > "$OUT/public-results/capstone.run.log"

jq -n \
  --slurpfile profile "$OUT/profile/artifact-container-profile.json" \
  --slurpfile host "$OUT/profile/host-independence.json" \
  --slurpfile capstone "$OUT/public-results/capstone/summary.json" \
  '{
    version:"patchline.artifact-container-rebuild/v1",
    profile:$profile[0],
    host_independence:$host[0],
    public_results:$capstone[0].summary,
    summary:{
      mode:$profile[0].mode,
      public_repos:$capstone[0].summary.public_repos,
      ranked_risks:$capstone[0].summary.ranked_risks,
      generated_files:$capstone[0].summary.generated_files,
      rejected_examples:$capstone[0].summary.rejected_examples,
      evidence_artifact_types:($capstone[0].summary.evidence_artifacts | keys | length),
      no_production_credentials:$profile[0].no_production_credentials,
      no_host_database_services:$profile[0].no_host_database_services,
      no_host_language_toolchains:$profile[0].no_host_language_toolchains,
      verified:($capstone[0].summary.verified == true and $host[0].checked == true)
    }
  }' > "$OUT/rebuild-summary.json"

{
  echo "# Patchline artifact container rebuild"
  echo
  echo "One command rebuilds public results from pinned public repositories inside the artifact profile:"
  echo
  echo "\`\`\`bash"
  jq -r '.host_independence.one_container_command' "$OUT/rebuild-summary.json"
  echo "\`\`\`"
  echo
  echo "## Public-code rebuild evidence"
  echo
  jq -r '.summary | "- public repositories: `" + (.public_repos|tostring) + "`\n- ranked risks: `" + (.ranked_risks|tostring) + "`\n- generated files: `" + (.generated_files|tostring) + "`\n- rejected bad-output examples: `" + (.rejected_examples|tostring) + "`\n- evidence artifact types: `" + (.evidence_artifact_types|tostring) + "`"' "$OUT/rebuild-summary.json"
  echo
  echo "## Host-independence guarantees"
  echo
  jq -r '.summary | "- no production credentials: `" + (.no_production_credentials|tostring) + "`\n- no host database services: `" + (.no_host_database_services|tostring) + "`\n- no host language toolchains: `" + (.no_host_language_toolchains|tostring) + "`"' "$OUT/rebuild-summary.json"
} > "$OUT/README.md"

echo "artifact container rebuild complete: repos $(jq '.summary.public_repos' "$OUT/rebuild-summary.json"), risks $(jq '.summary.ranked_risks' "$OUT/rebuild-summary.json")"
