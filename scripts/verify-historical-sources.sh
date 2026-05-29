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
  document_count="$(jq ".cases[$i].source_documents // [] | length" "$suite")"
  if [[ "$document_count" -eq 0 ]]; then
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
    continue
  fi

  while IFS=$'\t' read -r document_id url assertion_id phrase; do
    body="$tmpdir/$id-$document_id.body"
    if [[ ! -f "$body" ]]; then
      curl --fail --location --silent --show-error "$url" --output "$body"
    fi
    if ! grep -F -- "$phrase" "$body" >/dev/null; then
      echo "missing source phrase: case=$id document=$document_id assertion=$assertion_id" >&2
      exit 1
    fi
    echo "verified source phrase: case=$id document=$document_id assertion=$assertion_id"
  done < <(
    jq -r ".cases[$i] as \$case | \$case.source_documents[] as \$doc | (\$doc.assertions // [])[] as \$assertion_id | \$case.source_assertions[] | select(.id == \$assertion_id) | [\$doc.id, \$doc.url, .id, .phrase] | @tsv" "$suite"
  )
done
