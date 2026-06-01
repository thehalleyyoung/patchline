#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

GATES="${1:-examples/stale-ref-gates.json}"
OUT="${2:-results/generated/stale-ref-gate}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.stale-ref-gates/v1" and
  (.sources | length) >= 4 and
  all(.sources[];
    (.repo | contains("/")) and
    (.ref | test("^[0-9a-f]{40}$")) and
    (.expected_archive_hash | startswith("sha256:"))
  )
' "$GATES" > /dev/null

rows=()
while IFS=$'\t' read -r id repo ref subpath expected; do
  case_out="$OUT/$id"
  mkdir -p "$case_out"
  resolved=""
  if command -v gh >/dev/null 2>&1; then
    resolved="$(gh api "repos/$repo/commits/$ref" --jq '.sha' 2>/dev/null || true)"
  fi
  if [ -z "$resolved" ]; then
    resolved="$(curl -L -sS "https://api.github.com/repos/$repo/commits/$ref" | jq -r '.sha // empty')"
  fi
  if [ -z "$resolved" ]; then
    resolved="$(git ls-remote "https://github.com/$repo.git" | awk '{print $1}' | grep -m1 "^$ref$" || true)"
  fi
  if [ "$resolved" != "$ref" ]; then
    echo "stale or missing ref: $repo $ref resolved=$resolved" >&2
    exit 1
  fi
  go run ./cmd/patchline repo fetch "$repo" --ref "$ref" --subpath "$subpath" --out "$case_out/fetch" --download-dir "$OUT/cache" --json > "$case_out/fetch.json"
  actual="$(jq -r '.source.archive_hash' "$case_out/fetch.json")"
  if [ "$actual" != "$expected" ]; then
    echo "archive hash mismatch for $id expected=$expected actual=$actual" >&2
    exit 1
  fi
  jq -n \
    --arg id "$id" \
    --arg repo "$repo" \
    --arg ref "$ref" \
    --arg subpath "$subpath" \
    --arg expected "$expected" \
    --arg actual "$actual" \
    '{
      id: $id,
      repo: $repo,
      ref: $ref,
      subpath: $subpath,
      ref_resolves: true,
      expected_archive_hash: $expected,
      actual_archive_hash: $actual,
      hash_matches: ($expected == $actual),
      verified: true
    }' > "$case_out/row.json"
  rows+=("$case_out/row.json")
done < <(jq -r '.sources[] | [.id, .repo, .ref, .subpath, .expected_archive_hash] | @tsv' "$GATES")

jq -s '{version:"patchline.stale-ref-gate-results/v1", sources: .}' "${rows[@]}" > "$OUT/summary.json"
jq -e '(.sources | length) >= 4 and all(.sources[]; .verified == true and .ref_resolves == true and .hash_matches == true)' "$OUT/summary.json" > /dev/null
echo "stale-ref gate passed: $(jq '.sources | length' "$OUT/summary.json") pinned refs resolved and archive hashes matched"
