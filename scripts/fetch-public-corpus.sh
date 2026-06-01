#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
sources="${PATCHLINE_PUBLIC_CORPUS_SOURCES:-$root/examples/public-corpus/sources.json}"
out_dir="${PATCHLINE_PUBLIC_CORPUS_OUT:-$root/examples/public-corpus/downloads}"
report_dir="${PATCHLINE_PUBLIC_CORPUS_REPORT_DIR:-$root/results/generated/public-corpus}"
offline="${PATCHLINE_PUBLIC_CORPUS_OFFLINE:-0}"

if ! command -v jq >/dev/null 2>&1; then
  echo "fetch-public-corpus requires jq" >&2
  exit 1
fi

mkdir -p "$out_dir" "$report_dir"

source_hash="$(shasum -a 256 "$sources" | awk '{print $1}')"
files_jsonl="$(mktemp)"
trap 'rm -f "$files_jsonl"' EXIT

relative_path() {
  local path="$1"
  case "$path" in
    "$root"/*) printf '%s\n' "${path#$root/}" ;;
    *) printf '%s\n' "$path" ;;
  esac
}

verify_hash() {
  local id="$1"
  local expected="$2"
  local path="$3"
  local actual
  actual="$(shasum -a 256 "$path" | awk '{print $1}')"
  if [[ "$actual" != "$expected" ]]; then
    echo "sha256 mismatch for $id: expected $expected got $actual" >&2
    return 1
  fi
  printf '%s\n' "$actual"
}

fetch_one() {
  local id="$1"
  local url="$2"
  local sha="$3"
  local out="$4"
  local path="$out_dir/$out"
  local status="downloaded"
  local actual=""

  if [[ -f "$path" ]]; then
    actual="$(shasum -a 256 "$path" | awk '{print $1}')"
    if [[ "$actual" == "$sha" ]]; then
      status="cached"
    elif [[ "$offline" == "1" ]]; then
      echo "cached file hash mismatch for $id in offline mode: expected $sha got $actual" >&2
      exit 1
    else
      status="refetched"
      curl -fsSL "$url" -o "$path"
      actual="$(verify_hash "$id" "$sha" "$path")"
    fi
  elif [[ "$offline" == "1" ]]; then
    echo "missing cached public corpus file for $id in offline mode: $path" >&2
    exit 1
  else
    curl -fsSL "$url" -o "$path"
    actual="$(verify_hash "$id" "$sha" "$path")"
  fi

  if [[ -z "$actual" ]]; then
    actual="$(verify_hash "$id" "$sha" "$path")"
  fi

  jq -n \
    --arg id "$id" \
    --arg url "$url" \
    --arg expected_sha256 "$sha" \
    --arg actual_sha256 "$actual" \
    --arg status "$status" \
    --arg path "$(relative_path "$path")" \
    '{id: $id, url: $url, path: $path, status: $status, expected_sha256: $expected_sha256, actual_sha256: $actual_sha256, ok: ($expected_sha256 == $actual_sha256)}' \
    >> "$files_jsonl"

  echo "$id $status $actual $path"
}

while IFS=$'\t' read -r id url sha out; do
  fetch_one "$id" "$url" "$sha" "$out"
done < <(jq -r '.sources[] | [.id, .url, .sha256, .out] | @tsv' "$sources")

report_path="$report_dir/fetch-report.json"
jq -s \
  --arg version "patchline.public-corpus-fetch/v1" \
  --arg source_manifest "$(relative_path "$sources")" \
  --arg source_manifest_sha256 "$source_hash" \
  --arg output_dir "$(relative_path "$out_dir")" \
  --arg offline "$offline" \
  '{
    version: $version,
    source_manifest: $source_manifest,
    source_manifest_sha256: $source_manifest_sha256,
    output_dir: $output_dir,
    offline: ($offline == "1"),
    files: .,
    ok: all(.[]; .ok)
  }' "$files_jsonl" > "$report_path"

jq -e '.ok == true' "$report_path" >/dev/null
echo "public corpus fetch report $(relative_path "$report_path")"
