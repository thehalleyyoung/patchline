#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/artifact-release-manifest-gate.json}"
OUT="${2:-results/generated/artifact-release-manifest}"
rm -rf "$OUT"
mkdir -p "$OUT/evidence"

jq -e '
  .version == "patchline.artifact-release-manifest-gate/v1" and
  (.claim | length) > 120 and
  (.release_name | length) > 8 and
  (.release_tag | test("^[a-z0-9.-]+$")) and
  (.doi_prefix | startswith("10.")) and
  (.required_outputs | length) >= 6
' "$SPEC" > /dev/null

profile_spec="$(jq -r '.evidence_profile_spec' "$SPEC")"
bash scripts/artifact-container-rebuild.sh "$profile_spec" "$OUT/evidence/artifact-container-rebuild" > "$OUT/evidence/artifact-container-rebuild.run.log"

REBUILD="$OUT/evidence/artifact-container-rebuild/rebuild-summary.json"
CAPSTONE="$OUT/evidence/artifact-container-rebuild/public-results/capstone"
test -s "$REBUILD"
test -s "$CAPSTONE/checksums.txt"

source_files=("$CAPSTONE"/cache/sources/*.json)
jq -s '
  {
    version:"patchline.artifact-release-archives/v1",
    archives:(map({
      key,
      url,
      archive_hash,
      archive_path,
      kind,
      resolved_commit,
      top
    }) | sort_by(.key)),
    summary:{
      archives:length,
      sha256_archives:([.[].archive_hash | startswith("sha256:")] | map(select(. == true)) | length)
    }
  }' "${source_files[@]}" > "$OUT/archives.json"

go_version="$(go version)"
git_version="$(git --version)"
bash_version="${BASH_VERSION%%(*}"
jq_version="$(jq --version)"
make_version="$(make --version | sed -n '1p')"
patchline_commit="$(git rev-parse HEAD)"

jq -n \
  --arg go "$go_version" \
  --arg git "$git_version" \
  --arg bash "$bash_version" \
  --arg jq "$jq_version" \
  --arg make "$make_version" \
  --arg commit "$patchline_commit" \
  '{
    version:"patchline.artifact-command-versions/v1",
    commands:[
      {name:"go", version:$go},
      {name:"git", version:$git},
      {name:"bash", version:$bash},
      {name:"jq", version:$jq},
      {name:"make", version:$make}
    ],
    repository_commit:$commit
  }' > "$OUT/command-versions.json"

find "$OUT/evidence/artifact-container-rebuild" -type f \
  ! -path '*/cache/fetch/*' \
  ! -path '*/cache/archives/*' \
  ! -path '*/fetch/*' \
  | sort \
  | while read -r file; do
      rel="${file#$OUT/}"
      shasum -a 256 "$file" | awk -v rel="$rel" '{print $1 "  " rel}'
    done > "$OUT/artifact-checksums.sha256"

jq -n \
  --slurpfile spec "$SPEC" \
  --slurpfile rebuild "$REBUILD" \
  --slurpfile archives "$OUT/archives.json" \
  --slurpfile commands "$OUT/command-versions.json" \
  --rawfile checksums "$OUT/artifact-checksums.sha256" \
  '{
    version:"patchline.artifact-release-manifest/v1",
    release:{
      name:$spec[0].release_name,
      tag:$spec[0].release_tag,
      doi_prefix:$spec[0].doi_prefix,
      status:"doi-candidate-unregistered",
      repository_commit:$commands[0].repository_commit
    },
    exact_refs:($archives[0].archives | map({key, resolved_commit, archive_url:.url, archive_hash, subpath:null})),
    archives:$archives[0].archives,
    command_versions:$commands[0].commands,
    generated_artifact_checksums:($checksums | split("\n") | map(select(length > 0)) | map(capture("(?<sha256>[0-9a-f]{64})  (?<path>.+)"))),
    public_results:$rebuild[0].summary,
    reproduction_commands:[
      "bash scripts/artifact-container-rebuild.sh examples/artifact-container-profile-gate.json results/generated/artifact-container-rebuild",
      "make artifact-container-profile-gate",
      "make reviewer-dry-run-logs-gate",
      "make paper-appendix-gate"
    ],
    summary:{
      public_repos:$rebuild[0].summary.public_repos,
      ranked_risks:$rebuild[0].summary.ranked_risks,
      generated_files:$rebuild[0].summary.generated_files,
      archives:$archives[0].summary.archives,
      archive_checksums:$archives[0].summary.sha256_archives,
      command_versions:($commands[0].commands | length),
      artifact_checksums:($checksums | split("\n") | map(select(length > 0)) | length),
      verified:$rebuild[0].summary.verified
    }
  }' > "$OUT/manifest-payload.json"

payload_hash="$(jq -S . "$OUT/manifest-payload.json" | shasum -a 256 | awk '{print $1}')"
doi_suffix="$(printf '%s' "$payload_hash" | cut -c1-16)"
jq --arg hash "$payload_hash" --arg suffix "$doi_suffix" '
  .release.content_hash = ("sha256:" + $hash) |
  .release.doi = (.release.doi_prefix + "." + $suffix) |
  .summary.content_hash = ("sha256:" + $hash)
' "$OUT/manifest-payload.json" > "$OUT/artifact-release-manifest.json"
rm "$OUT/manifest-payload.json"

{
  echo "# Patchline artifact DOI/release manifest"
  echo
  jq -r '.release | "- release: `" + .name + "`\n- tag: `" + .tag + "`\n- DOI candidate: `" + .doi + "`\n- content hash: `" + .content_hash + "`\n- repository commit: `" + .repository_commit + "`"' "$OUT/artifact-release-manifest.json"
  echo
  echo "## Exact refs and archives"
  echo
  echo "| Source | Resolved commit | Archive URL | Archive checksum |"
  echo "| --- | --- | --- | --- |"
  jq -r '.archives[] | "| `" + .key + "` | `" + .resolved_commit + "` | `" + .url + "` | `" + .archive_hash + "` |"' "$OUT/artifact-release-manifest.json"
  echo
  echo "## Command versions"
  echo
  jq -r '.command_versions[] | "- `" + .name + "`: " + .version' "$OUT/artifact-release-manifest.json"
  echo
  echo "## Reproduction commands"
  echo
  jq -r '.reproduction_commands[] | "- `" + . + "`"' "$OUT/artifact-release-manifest.json"
} > "$OUT/artifact-release-manifest.md"

cp "$OUT/artifact-release-manifest.md" "$OUT/README.md"
echo "artifact release manifest generated: archives $(jq '.summary.archives' "$OUT/artifact-release-manifest.json"), checksums $(jq '.summary.artifact_checksums' "$OUT/artifact-release-manifest.json"), doi $(jq -r '.release.doi' "$OUT/artifact-release-manifest.json")"
