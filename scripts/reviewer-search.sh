#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/reviewer-search-gate.json}"
OUT="${2:-results/generated/reviewer-search}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '.version == "patchline.reviewer-search-gate/v1" and (.corpus|length) >= 1 and (.queries|length) >= 1' "$SPEC" > /dev/null

# Token-overlap retrieval over the typed corpus. Stopwords removed so matches are
# driven by content terms; an empty overlap returns no result (no hallucination).
jq '
  def stop: ["is","the","are","of","a","an","what","where","and","to","for","across","without","every"];
  def toks($s): ($s | ascii_downcase | gsub("[^a-z0-9 ]"; " ") | split(" ")
                  | map(select(length > 1 and (. as $t | (stop | index($t)) == null))) | unique);
  .corpus as $corpus
  | {
      version: "patchline.reviewer-search/v1",
      answers: [
        .queries[] | . as $query
        | (toks($query.q)) as $qt
        | ([ $corpus[] | . as $e | (toks($e.text)) as $et
             | { type: $e.type, id: $e.id, score: ([ $qt[] | select(. as $w | ($et | index($w)) != null) ] | length) }
           ] | map(select(.score > 0)) | sort_by(-.score)) as $hits
        | {
            q: $query.q,
            expect_type: $query.expect_type,
            expect_id: $query.expect_id,
            top: ($hits[0] // null),
            hit_count: ($hits | length)
          }
      ]
    }
' "$SPEC" > "$OUT/reviewer-search.json"

{
  echo "# Reviewer-question search"
  echo
  echo "| Query | Top type | Top id | Hits |"
  echo "|---|---|---|---|"
  jq -r '.answers[] | "| \(.q) | \(.top.type // "-") | \(.top.id // "-") | \(.hit_count) |"' "$OUT/reviewer-search.json"
} > "$OUT/reviewer-search.md"
cp "$OUT/reviewer-search.md" "$OUT/README.md"

echo "reviewer-search worker: answered $(jq -r '.answers|length' "$OUT/reviewer-search.json") queries"
