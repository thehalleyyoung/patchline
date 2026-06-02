#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/incremental-cache-gate.json}"
OUT="${2:-results/generated/incremental-cache}"
rm -rf "$OUT"
mkdir -p "$OUT/cache"

jq -e '.version == "patchline.incremental-cache-gate/v1"' "$SPEC" > /dev/null

archive_path="$(jq -r '.archive_path' "$SPEC")"
subpath="$(jq -r '.subpath' "$SPEC")"
parser_version="$(jq -r '.parser_version' "$SPEC")"
config="$(jq -c '.config' "$SPEC")"
test -f "$archive_path"

archive_hash="$(shasum -a 256 "$archive_path" | cut -d' ' -f1)"

# Deterministic cache key over the four load-bearing inputs.
cache_key() {
  printf '%s|%s|%s|%s' "$1" "$2" "$3" "$4" | shasum -a 256 | cut -c1-32
}

# The "analysis" is any pure function of the archive; we use a stable content digest
# so that a cache hit can be checked for result equality against a fresh computation.
analyze() {
  shasum -a 256 "$archive_path" | cut -c1-16
}

run_cached() {
  local key="$1"
  local entry="$OUT/cache/${key}.json"
  if [ -f "$entry" ]; then
    echo "hit $(jq -r '.result' "$entry")"
  else
    local result; result="$(analyze)"
    jq -n --arg result "$result" '{result:$result}' > "$entry"
    echo "miss $result"
  fi
}

base_key="$(cache_key "$archive_hash" "$subpath" "$parser_version" "$config")"

cold="$(run_cached "$base_key")"          # expect miss
warm="$(run_cached "$base_key")"          # expect hit
cold_status="${cold%% *}"; cold_result="${cold##* }"
warm_status="${warm%% *}"; warm_result="${warm##* }"

# Each of the four components must be load-bearing: perturb one at a time.
k_archive="$(cache_key "${archive_hash}x" "$subpath" "$parser_version" "$config")"
k_subpath="$(cache_key "$archive_hash" "${subpath}x" "$parser_version" "$config")"
k_parser="$(cache_key "$archive_hash" "$subpath" "${parser_version}x" "$config")"
k_config="$(cache_key "$archive_hash" "$subpath" "$parser_version" "${config}x")"

distinct=true
for k in "$k_archive" "$k_subpath" "$k_parser" "$k_config"; do
  [ "$k" != "$base_key" ] || distinct=false
done

# Key determinism: recomputing from identical inputs yields the same key.
base_key2="$(cache_key "$archive_hash" "$subpath" "$parser_version" "$config")"
key_stable=false; [ "$base_key" = "$base_key2" ] && key_stable=true

result_equal=false; [ "$cold_result" = "$warm_result" ] && result_equal=true

jq -n \
  --arg base_key "$base_key" \
  --arg cold_status "$cold_status" \
  --arg warm_status "$warm_status" \
  --argjson key_stable "$key_stable" \
  --argjson distinct "$distinct" \
  --argjson result_equal "$result_equal" '
  {
    version: "patchline.incremental-cache/v1",
    cache_key: $base_key,
    cold_status: $cold_status,
    warm_status: $warm_status,
    key_stable: $key_stable,
    components_load_bearing: $distinct,
    warm_result_equals_cold: $result_equal
  }' > "$OUT/incremental-cache.json"

{
  echo "# Incremental analysis cache"
  echo
  echo "Key: \`$base_key\` (archive hash + subpath + parser version + config)"
  echo
  echo "- Cold run: $cold_status"
  echo "- Warm run: $warm_status"
  echo "- Each key component load-bearing: $distinct"
} > "$OUT/incremental-cache.md"
cp "$OUT/incremental-cache.md" "$OUT/README.md"

echo "incremental-cache worker: cold=$cold_status warm=$warm_status stable=$key_stable load-bearing=$distinct"
