#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

GATES="${1:-examples/parser-fact-gates.json}"
OUT="${2:-results/generated/parser-fact-gate}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.parser-fact-gates/v1" and
  (.parsers | length) > 0 and
  all(.parsers[];
    (.id | length) > 0 and
    (.real_repo | length) > 0 and
    (.subpath | length) > 0 and
    (.expected_fact_kind | length) > 0 and
    (.new_fact | length) > 0
  )
' "$GATES" > /dev/null

rows=()
while IFS=$'\t' read -r id repo subpath expected_kind sample_property; do
  case_out="$OUT/$id"
  mkdir -p "$case_out"
  ref="$(jq -r --arg repo "$repo" --arg subpath "$subpath" '.slices[] | select(.repo == $repo and .subpath == $subpath) | .ref' examples/real-repo-slices.json)"
  test -n "$ref"
  go run ./cmd/patchline repo fetch "$repo" --ref "$ref" --subpath "$subpath" --out "$case_out/fetch" --json > "$case_out/fetch.json"
  scan_root="$(jq -r '.source.scanned_root' "$case_out/fetch.json")"
  go run ./cmd/patchline repo inventory "$scan_root" --out "$case_out/inventory" --json > "$case_out/inventory.json"
  jq -e --arg kind "$expected_kind" --arg prop "$sample_property" '
    select(.kind == $kind) |
    if $prop == "" then true else (.properties[$prop] // "" | length > 0) end
  ' "$case_out/inventory/facts.jsonl" > "$case_out/matched-facts.jsonl"
  test -s "$case_out/matched-facts.jsonl"
  jq -n \
    --arg id "$id" \
    --arg repo "$repo" \
    --arg subpath "$subpath" \
    --arg expected_fact_kind "$expected_kind" \
    --argjson matched_facts "$(wc -l < "$case_out/matched-facts.jsonl" | tr -d ' ')" \
    '{id: $id, repo: $repo, subpath: $subpath, expected_fact_kind: $expected_fact_kind, matched_facts: $matched_facts}' > "$case_out/row.json"
  rows+=("$case_out/row.json")
done < <(jq -r '.parsers[] | [.id, .real_repo, .subpath, .expected_fact_kind, (.sample_fact_property // "")] | @tsv' "$GATES")

jq -s '{version:"patchline.parser-fact-gate-results/v1", parsers: .}' "${rows[@]}" > "$OUT/summary.json"
jq -e 'all(.parsers[]; .matched_facts > 0)' "$OUT/summary.json" > /dev/null
echo "parser fact gate passed: $(jq '.parsers | length' "$OUT/summary.json") parser entries proved on real repo slices"
