#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/independent-replication-gate.json}"
OUT="${2:-results/generated/independent-replication}"
rm -rf "$OUT"
mkdir -p "$OUT/evidence" "$OUT/archives"

jq -e '
  .version == "patchline.independent-replication-gate/v1" and
  (.claim | length) > 120 and
  (.release_manifest_spec | startswith("examples/")) and
  (.forbidden_tools | length) >= 5 and
  (.forbidden_environment | length) >= 4 and
  (.required_outputs | length) >= 6
' "$SPEC" > /dev/null

release_spec="$(jq -r '.release_manifest_spec' "$SPEC")"
env -u GITHUB_TOKEN -u GH_TOKEN -u GITHUB_PAT -u DATADOG_API_KEY \
  bash scripts/generate-artifact-release-manifest.sh "$release_spec" "$OUT/evidence/release-manifest" > "$OUT/evidence/release-manifest.run.log"

MANIFEST="$OUT/evidence/release-manifest/artifact-release-manifest.json"
test -s "$MANIFEST"

env_rows=()
for tool in bash curl git go jq make shasum; do
  row="$OUT/tool-$tool.json"
  path="$(command -v "$tool" 2>/dev/null || true)"
  jq -n --arg tool "$tool" --arg path "$path" '{tool:$tool,path:$path,available:($path != "")}' > "$row"
  env_rows+=("$row")
done

jq -n \
  --slurpfile tools <(jq -s '.' "${env_rows[@]}") \
  --slurpfile spec "$SPEC" \
  '{
    version:"patchline.no-credential-environment/v1",
    tools:$tools[0],
    forbidden_tools:$spec[0].forbidden_tools,
    forbidden_environment:$spec[0].forbidden_environment,
    credentials_required:false,
    proprietary_tooling_required:false,
    github_cli_required:false
  }' > "$OUT/no-credential-environment.json"

archive_rows=()
count="$(jq '.archives | length' "$MANIFEST")"
for ((i=0; i<count; i++)); do
  key="$(jq -r ".archives[$i].key" "$MANIFEST")"
  url="$(jq -r ".archives[$i].url" "$MANIFEST")"
  expected="$(jq -r ".archives[$i].archive_hash" "$MANIFEST")"
  archive_path="$(jq -r ".archives[$i].archive_path" "$MANIFEST")"
  actual=""
  verified=false
  if [ -s "$archive_path" ]; then
    actual="sha256:$(shasum -a 256 "$archive_path" | awk '{print $1}')"
    if [ "$actual" = "$expected" ]; then
      verified=true
    fi
  fi
  row="$OUT/archive-$i.json"
  jq -n \
    --arg key "$key" \
    --arg url "$url" \
    --arg expected "$expected" \
    --arg actual "$actual" \
    --arg archive_path "$archive_path" \
    --argjson verified "$verified" \
    '{
      key:$key,
      anonymous_archive_url:$url,
      expected_hash:$expected,
      actual_hash:$actual,
      archive_path:$archive_path,
      verified:$verified,
      credential_free:($url | startswith("https://codeload.github.com/"))
    }' > "$row"
  archive_rows+=("$row")
done

jq -n \
  --slurpfile archives <(jq -s '.' "${archive_rows[@]}") \
  '{
    version:"patchline.independent-archive-verification/v1",
    archives:$archives[0],
    summary:{
      archives:($archives[0] | length),
      verified:($archives[0] | map(select(.verified == true)) | length),
      credential_free_urls:($archives[0] | map(select(.credential_free == true)) | length)
    }
  }' > "$OUT/archive-verification.json"

{
  echo "#!/usr/bin/env bash"
  echo "set -euo pipefail"
  echo "mkdir -p archives"
  jq -r '.archives[] | @base64' "$MANIFEST" | while read -r encoded; do
    archive="$(printf '%s' "$encoded" | base64 --decode)"
    key="$(printf '%s' "$archive" | jq -r '.key')"
    url="$(printf '%s' "$archive" | jq -r '.url')"
    hash="$(printf '%s' "$archive" | jq -r '.archive_hash')"
    file="$(printf '%s' "$key" | tr ':@/' '---').tar.gz"
    echo "curl -L '$url' -o 'archives/$file'"
    echo "printf '%s  %s\\n' '${hash#sha256:}' 'archives/$file' | shasum -a 256 -c -"
  done
  echo "bash scripts/artifact-container-rebuild.sh examples/artifact-container-profile-gate.json results/generated/independent-replication/rebuild"
  echo "make paper-appendix-gate"
} > "$OUT/replication-commands.sh"
chmod +x "$OUT/replication-commands.sh"

jq -n \
  --slurpfile manifest "$MANIFEST" \
  --slurpfile archives "$OUT/archive-verification.json" \
  --slurpfile env "$OUT/no-credential-environment.json" \
  '{
    version:"patchline.independent-replication/v1",
    release_manifest:"evidence/release-manifest/artifact-release-manifest.json",
    archive_verification:$archives[0],
    environment:$env[0],
    instructions:[
      "Use anonymous codeload archive URLs from the release manifest.",
      "Verify every archive against its SHA-256 before analysis.",
      "Run Patchline artifact rebuild commands; do not use proprietary services.",
      "Unset GitHub and vendor credentials to prove the path is public-data only.",
      "Compare generated public repository counts, ranked risks, and checksums."
    ],
    reproduction_commands:[
      "bash results/generated/independent-replication/replication-commands.sh",
      "bash scripts/artifact-container-rebuild.sh examples/artifact-container-profile-gate.json results/generated/independent-replication/rebuild",
      "make artifact-release-manifest-gate",
      "make paper-appendix-gate",
      "make camera-ready-checklist-gate"
    ],
    summary:{
      public_repos:$manifest[0].summary.public_repos,
      ranked_risks:$manifest[0].summary.ranked_risks,
      archives:$archives[0].summary.archives,
      verified_archives:$archives[0].summary.verified,
      credential_free_urls:$archives[0].summary.credential_free_urls,
      commands:5,
      credentials_required:false,
      proprietary_tooling_required:false,
      verified:($manifest[0].summary.verified == true and $archives[0].summary.archives == $archives[0].summary.verified)
    }
  }' > "$OUT/independent-replication.json"

{
  echo "# Independent replication instructions"
  echo
  echo "These instructions are for reviewers who do not have GitHub credentials or proprietary tooling. The path uses anonymous public archive URLs, SHA-256 checksums, and ordinary command-line tools."
  echo
  echo "## No-credential setup"
  echo
  echo '```bash'
  echo "unset GITHUB_TOKEN GH_TOKEN GITHUB_PAT DATADOG_API_KEY"
  echo "bash results/generated/independent-replication/replication-commands.sh"
  echo '```'
  echo
  echo "## Archive verification"
  echo
  echo "| Source | Anonymous archive URL | Expected SHA-256 | Verified |"
  echo "| --- | --- | --- | --- |"
  jq -r '.archive_verification.archives[] | "| `" + .key + "` | `" + .anonymous_archive_url + "` | `" + .expected_hash + "` | `" + (.verified|tostring) + "` |"' "$OUT/independent-replication.json"
  echo
  echo "## Final public-code evidence"
  echo
  jq -r '.summary | "- public repositories: `" + (.public_repos|tostring) + "`\n- ranked risks: `" + (.ranked_risks|tostring) + "`\n- verified archives: `" + (.verified_archives|tostring) + "`\n- credentials required: `" + (.credentials_required|tostring) + "`\n- proprietary tooling required: `" + (.proprietary_tooling_required|tostring) + "`"' "$OUT/independent-replication.json"
} > "$OUT/independent-replication.md"

cp "$OUT/independent-replication.md" "$OUT/README.md"
echo "independent replication generated: archives $(jq '.summary.verified_archives' "$OUT/independent-replication.json"), risks $(jq '.summary.ranked_risks' "$OUT/independent-replication.json")"
