#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/disaster-recovery-exercise-gate.json}"
OUT="${2:-results/generated/disaster-recovery-exercise}"

rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.disaster-recovery-exercise-gate/v1"
  and (.mirrors | length) == 2
  and (.components | length) == 4
' "$SPEC" > /dev/null

SOURCE="$OUT/source"
MIRRORS="$OUT/mirrors"
RESTORED="$OUT/restored"
REPORTS="$OUT/component-reports"
TRUSTED="$OUT/trusted-manifests"
mkdir -p "$SOURCE" "$MIRRORS" "$RESTORED" "$REPORTS" "$TRUSTED"

component_manifest=".patchline-dr-checksums.sha256"

component_spec_value() {
  local component="$1"
  local field="$2"
  jq -r --arg component "$component" --arg field "$field" \
    '.components[] | select(.id == $component) | .[$field]' "$SPEC"
}

write_component_checksums() {
  local dir="$1"
  (
    cd "$dir"
    find . -type f ! -name "$component_manifest" -print |
      LC_ALL=C sort |
      while IFS= read -r file; do
        shasum -a 256 "$file"
      done
  ) > "$dir/$component_manifest"
}

verify_component() {
  local dir="$1"
  local log="$2"
  local component="${3:-}"
  if [[ ! -d "$dir" || ! -s "$dir/$component_manifest" ]]; then
    printf 'missing component or checksum manifest: %s\n' "$dir" > "$log"
    return 1
  fi
  if [[ -n "$component" && -s "$TRUSTED/$component.sha256" ]]; then
    if ! cmp -s "$TRUSTED/$component.sha256" "$dir/$component_manifest"; then
      printf 'checksum manifest mismatch: %s\n' "$dir" > "$log"
      return 1
    fi
  fi
  (
    cd "$dir"
    shasum -a 256 -c "$component_manifest"
  ) > "$log" 2>&1
}

copy_component() {
  local src="$1"
  local dest="$2"
  rm -rf "$dest"
  mkdir -p "$(dirname "$dest")"
  cp -R "$src" "$dest"
}

inject_failure() {
  local component="$1"
  local action="$2"
  local rel="$3"
  local target="$MIRRORS/primary/$component/$rel"
  case "$action" in
    none)
      ;;
    tamper)
      printf '\nPATCHLINE_DRILL_TAMPER=%s\n' "$component" >> "$target"
      ;;
    delete)
      rm -f "$target"
      ;;
    *)
      echo "unsupported primary failure action $action for $component" >&2
      return 1
      ;;
  esac
}

restore_from_mirrors() {
  local component="$1"
  local mirror_root="$2"
  local restore_root="$3"
  local report_path="${4:-}"
  local restored_from=""
  local failed_file
  local mirror
  local failed_mirrors=()

  failed_file="$(mktemp "$OUT/${component}.failed.XXXXXX")"
  : > "$failed_file"

  while IFS= read -r mirror; do
    local candidate="$mirror_root/$mirror/$component"
    local verify_log="$OUT/verify-${component}-${mirror}.log"
    if verify_component "$candidate" "$verify_log" "$component"; then
      restored_from="$mirror"
      copy_component "$candidate" "$restore_root/$component"
      break
    fi
    failed_mirrors+=("$mirror")
    printf '%s\n' "$mirror" >> "$failed_file"
  done < <(jq -r '.mirrors[]' "$SPEC")

  if [[ -z "$restored_from" ]]; then
    if [[ -n "$report_path" ]]; then
      jq -n \
        --arg id "$component" \
        --slurpfile failed <(jq -R . "$failed_file" | jq -s '.') \
        '{
          id: $id,
          restored: false,
          restored_from: null,
          failed_mirrors: $failed[0],
          checksum_verified: false
        }' > "$report_path"
    fi
    rm -f "$failed_file"
    return 1
  fi

  if [[ -n "$report_path" ]]; then
    jq -n \
      --arg id "$component" \
      --arg restored_from "$restored_from" \
      --slurpfile failed <(jq -R . "$failed_file" | jq -s '.') \
      '{
        id: $id,
        restored: true,
        restored_from: $restored_from,
        failed_mirrors: $failed[0],
        checksum_verified: true
      }' > "$report_path"
  fi
  rm -f "$failed_file"
}

echo "building source artifacts"
corpus_spec="$(component_spec_value public-corpus source_spec)"
docs_spec="$(component_spec_value docs-site source_spec)"
release_spec="$(component_spec_value release-manifest source_spec)"

bash scripts/corpus-release-gate.sh "$corpus_spec" "$SOURCE/public-corpus" > "$OUT/public-corpus.source.log"
bash scripts/docs-site-gate.sh "$docs_spec" "$SOURCE/docs-site" > "$OUT/docs-site.source.log"
bash scripts/generate-artifact-release-manifest.sh "$release_spec" "$SOURCE/release-manifest" > "$OUT/release-manifest.source.log"
rm -rf "$SOURCE/release-manifest/evidence" "$SOURCE/release-manifest/manifest-payload.json"
go test ./internal/certrevocation -run TestReplayDynamicDisasterRecoveryCertificateLog -count=1 > "$OUT/certificate-logs.source.log"
go run ./tools/disasterrecoverycertlog --root . --out "$SOURCE/certificate-logs" >> "$OUT/certificate-logs.source.log"
cp examples/hardware-signing/logs/certificate-log.jsonl "$SOURCE/certificate-logs/hardware-certificate-log.jsonl"

while IFS= read -r component; do
  write_component_checksums "$SOURCE/$component"
  cp "$SOURCE/$component/$component_manifest" "$TRUSTED/$component.sha256"
done < <(jq -r '.components[].id' "$SPEC")

echo "publishing mirrors"
while IFS= read -r mirror; do
  while IFS= read -r component; do
    copy_component "$SOURCE/$component" "$MIRRORS/$mirror/$component"
  done < <(jq -r '.components[].id' "$SPEC")
done < <(jq -r '.mirrors[]' "$SPEC")

while IFS=$'\t' read -r component action rel; do
  inject_failure "$component" "$action" "$rel"
done < <(jq -r '.components[] | [.id, .primary_failure.action, (.primary_failure.path // "")] | @tsv' "$SPEC")

echo "restoring from mirrors"
while IFS= read -r component; do
  restore_from_mirrors "$component" "$MIRRORS" "$RESTORED" "$REPORTS/$component.json"
done < <(jq -r '.components[].id' "$SPEC")

go run ./cmd/patchline cert revoke-verify \
  "$RESTORED/certificate-logs/revocation-bundle.json" \
  --json > "$RESTORED/certificate-logs/replay-restored.json"

jq -e '
  .version == "patchline.certificate-revocation-replay/v1"
  and .all_ok == true
  and .records == 1
  and .revoked == 1
' "$RESTORED/certificate-logs/replay-restored.json" > /dev/null

NEG_MIRRORS="$OUT/negative-mirrors"
NEG_RESTORED="$OUT/negative-restored"
rm -rf "$NEG_MIRRORS" "$NEG_RESTORED"
mkdir -p "$NEG_MIRRORS" "$NEG_RESTORED"
while IFS= read -r mirror; do
  copy_component "$MIRRORS/$mirror" "$NEG_MIRRORS/$mirror"
done < <(jq -r '.mirrors[]' "$SPEC")

neg_component="$(jq -r '.negative_control.component' "$SPEC")"
neg_action="$(jq -r '.negative_control.action' "$SPEC")"
neg_path="$(jq -r '.negative_control.path' "$SPEC")"
while IFS= read -r mirror; do
  target="$NEG_MIRRORS/$mirror/$neg_component/$neg_path"
  case "$neg_action" in
    delete)
      rm -f "$target"
      ;;
    tamper)
      printf '\nPATCHLINE_NEGATIVE_DRILL_TAMPER=%s\n' "$neg_component" >> "$target"
      ;;
    *)
      echo "unsupported negative-control action $neg_action" >&2
      exit 1
      ;;
  esac
done < <(jq -r '.mirrors[]' "$SPEC")

negative_rejected=false
set +e
restore_from_mirrors "$neg_component" "$NEG_MIRRORS" "$NEG_RESTORED" "$OUT/negative-control-restore.json"
negative_status=$?
set -e
if [[ "$negative_status" -ne 0 ]]; then
  negative_rejected=true
fi

jq -n \
  --arg component "$neg_component" \
  --arg action "$neg_action" \
  --arg path "$neg_path" \
  --argjson rejected "$negative_rejected" \
  '{
    component: $component,
    action: $action,
    path: $path,
    rejected: $rejected
  }' > "$OUT/negative-control.json"

source_tip="$(jq -r '.checkpoint.tip_hash' "$SOURCE/certificate-logs/replay.json")"
restored_tip="$(jq -r '.checkpoint.tip_hash' "$RESTORED/certificate-logs/replay-restored.json")"

jq -s \
  --argjson negative "$(cat "$OUT/negative-control.json")" \
  --arg source_tip "$source_tip" \
  --arg restored_tip "$restored_tip" \
  '{
    version: "patchline.disaster-recovery-exercise/v1",
    summary: {
      components: length,
      all_restored: all(.[]; .restored == true),
      primary_restores: (map(select(.restored_from == "primary")) | length),
      secondary_restores: (map(select(.restored_from == "secondary")) | length),
      rejected_primary_mirrors: ([.[] | .failed_mirrors[]? | select(. == "primary")] | length)
    },
    certificate_replay: {
      source_tip_hash: $source_tip,
      restored_tip_hash: $restored_tip,
      matches_source: ($source_tip == $restored_tip)
    },
    components: .,
    negative_control: $negative
  }' "$REPORTS/public-corpus.json" "$REPORTS/docs-site.json" "$REPORTS/release-manifest.json" "$REPORTS/certificate-logs.json" \
  > "$OUT/disaster-recovery-exercise.json"

{
  echo "# Disaster recovery exercise"
  echo
  echo "Patchline rebuilt the public corpus, docs site, release manifest, and certificate logs, published them to two mirrors, injected primary-mirror failures, and restored every component from the first checksum-valid mirror."
  echo
  echo "| Component | Restored from | Failed mirrors |"
  echo "| --- | --- | --- |"
  jq -r '.components[] | "| `" + .id + "` | `" + .restored_from + "` | `" + ((.failed_mirrors | join(", ")) // "") + "` |"' "$OUT/disaster-recovery-exercise.json"
  echo
  jq -r '"- certificate log replay tip: `" + .certificate_replay.restored_tip_hash + "`", "- negative control rejected: `" + (.negative_control.rejected|tostring) + "`"' "$OUT/disaster-recovery-exercise.json"
} > "$OUT/disaster-recovery-exercise.md"

cp "$OUT/disaster-recovery-exercise.md" "$OUT/README.md"
echo "disaster recovery exercise restored $(jq -r '.summary.components' "$OUT/disaster-recovery-exercise.json") components; secondary restores $(jq -r '.summary.secondary_restores' "$OUT/disaster-recovery-exercise.json")"
