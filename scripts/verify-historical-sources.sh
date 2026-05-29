#!/usr/bin/env bash
set -euo pipefail

suite="${1:-examples/historical-failures/suite.json}"
tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required to verify historical source assertions" >&2
  exit 1
fi

case_count="$(jq '.cases | length' "$suite")"
for ((i = 0; i < case_count; i++)); do
  id="$(jq -r ".cases[$i].id" "$suite")"
  url="$(jq -r ".cases[$i].source_url" "$suite")"
  body="$tmpdir/$id.html"
  curl --fail --location --silent --show-error "$url" --output "$body"
  assertion_count="$(jq ".cases[$i].source_assertions | length" "$suite")"
  for ((j = 0; j < assertion_count; j++)); do
    assertion_id="$(jq -r ".cases[$i].source_assertions[$j].id" "$suite")"
    phrase="$(jq -r ".cases[$i].source_assertions[$j].phrase" "$suite")"
    if ! grep -F -- "$phrase" "$body" >/dev/null; then
      echo "missing source phrase: case=$id assertion=$assertion_id" >&2
      exit 1
    fi
    echo "verified source phrase: case=$id assertion=$assertion_id"
  done
done
