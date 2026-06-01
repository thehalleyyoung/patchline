#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

GATES="${1:-examples/public-gallery-gates.json}"
OUT="${2:-results/generated/public-gallery}"
rm -rf "$OUT"
mkdir -p "$OUT/cases"

jq -e '
  .version == "patchline.public-gallery-gates/v1" and
  (.gates | length) >= 4 and
  all(.gates[];
    (.id | length) > 0 and
    (.real_repo | length) > 0 and
    (.subpath | length) > 0 and
    (.gallery_claim | length) > 90
  )
' "$GATES" > /dev/null

jq -n --slurpfile gates "$GATES" --slurpfile slices examples/real-repo-slices.json '
  ($slices[0].slices | map(.repo + ":" + .subpath)) as $known |
  $gates[0].gates
  | all(.[]; (.real_repo + ":" + .subpath) as $key | ($known | index($key)))
' | jq -e '. == true' > /dev/null

hash_file() {
  shasum -a 256 "$1" | awk '{print $1}'
}

hash_dir() {
  find "$1" -type f -print | LC_ALL=C sort | while read -r file; do
    rel="${file#$1/}"
    printf '%s  %s\n' "$(hash_file "$file")" "$rel"
  done | shasum -a 256 | awk '{print $1}'
}

rows=()
while IFS=$'\t' read -r id repo subpath gallery_claim; do
  case_out="$OUT/cases/$id"
  mkdir -p "$case_out"
  ref="$(jq -r --arg repo "$repo" --arg subpath "$subpath" '.slices[] | select(.repo == $repo and .subpath == $subpath) | .ref' examples/real-repo-slices.json)"
  test -n "$ref"
  go run ./cmd/patchline repo analyze --github "$repo" --ref "$ref" --subpath "$subpath" --stages inventory,baseline,propose,compare --no-llm --redact --out "$case_out/analysis" --json > "$case_out/analysis.json"
  cp -R "$case_out/analysis/analysis-bundle" "$case_out/bundle"
  bundle_hash="$(hash_dir "$case_out/bundle")"
  analyze_hash="$(jq -r '.hash' "$case_out/analysis/analyze.json")"
  risks="$(jq -r '.summary.ranked_risks' "$case_out/analysis/analyze.json")"
  generated="$(jq -r '.summary.generated_files' "$case_out/analysis/analyze.json")"
  checks_failed="$(jq -r '.summary.compare_checks_failed' "$case_out/analysis/analyze.json")"
  top_risk="$(jq -r '.risks[0].stable_id' "$case_out/analysis/baseline/baseline.json")"
  screenshot="$case_out/screenshot.svg"
  {
    printf '<svg xmlns="http://www.w3.org/2000/svg" width="760" height="270" role="img" aria-label="Patchline gallery card for %s">\n' "$repo"
    printf '<rect width="760" height="270" fill="#0b1020"/>\n'
    printf '<text x="28" y="45" fill="#f8fafc" font-family="Arial" font-size="24">Patchline public gallery</text>\n'
    printf '<text x="28" y="82" fill="#cbd5e1" font-family="Arial" font-size="18">%s @ %s</text>\n' "$repo" "$ref"
    printf '<text x="28" y="114" fill="#93c5fd" font-family="Arial" font-size="16">subpath: %s</text>\n' "$subpath"
    printf '<text x="28" y="150" fill="#fbbf24" font-family="Arial" font-size="18">top risk: %s</text>\n' "$top_risk"
    printf '<text x="28" y="184" fill="#a7f3d0" font-family="Arial" font-size="17">ranked risks: %s  generated artifacts: %s  failed checks: %s</text>\n' "$risks" "$generated" "$checks_failed"
    printf '<text x="28" y="220" fill="#d1d5db" font-family="Arial" font-size="13">redacted bundle hash: %s</text>\n' "$bundle_hash"
    printf '</svg>\n'
  } > "$screenshot"
  screenshot_hash="$(hash_file "$screenshot")"
  jq -n \
    --arg id "$id" \
    --arg repo "$repo" \
    --arg ref "$ref" \
    --arg subpath "$subpath" \
    --arg gallery_claim "$gallery_claim" \
    --arg bundle "cases/$id/bundle" \
    --arg screenshot "cases/$id/screenshot.svg" \
    --arg bundle_hash "sha256:$bundle_hash" \
    --arg screenshot_hash "sha256:$screenshot_hash" \
    --arg analyze_hash "$analyze_hash" \
    --arg top_risk "$top_risk" \
    --argjson risks "$risks" \
    --argjson generated "$generated" \
    --argjson checks_failed "$checks_failed" \
    '{
      id: $id,
      repo: $repo,
      pinned_commit: $ref,
      subpath: $subpath,
      gallery_claim: $gallery_claim,
      redacted_bundle: $bundle,
      redacted_bundle_hash: $bundle_hash,
      screenshot: $screenshot,
      screenshot_hash: $screenshot_hash,
      analyze_hash: $analyze_hash,
      top_risk: $top_risk,
      ranked_risks: $risks,
      generated_artifacts: $generated,
      deterministic_check_failures: $checks_failed,
      verified: true
    }' > "$case_out/row.json"
  rows+=("$case_out/row.json")
done < <(jq -r '.gates[] | [.id, .real_repo, .subpath, .gallery_claim] | @tsv' "$GATES")

jq -s '{version:"patchline.public-gallery/v1", generated_at:"deterministic-gate", cases: .}' "${rows[@]}" > "$OUT/gallery.json"
{
  printf '<!doctype html><meta charset="utf-8"><title>Patchline public gallery</title><h1>Patchline public gallery</h1>\n'
  jq -r '.cases[] | "<section><h2>\(.repo)</h2><p><code>\(.pinned_commit)</code> / <code>\(.subpath)</code></p><p>bundle hash: <code>\(.redacted_bundle_hash)</code></p><img src=\"\(.screenshot)\" alt=\"Patchline screenshot for \(.repo)\"></section>"' "$OUT/gallery.json"
} > "$OUT/index.html"

jq -e '
  (.cases | length) >= 4 and
  all(.cases[];
    .verified == true and
    (.pinned_commit | length) >= 40 and
    (.redacted_bundle_hash | startswith("sha256:")) and
    (.screenshot_hash | startswith("sha256:")) and
    (.top_risk | startswith("stable-risk:")) and
    (.ranked_risks > 0)
  )
' "$OUT/gallery.json" > /dev/null
test -s "$OUT/index.html"
echo "public-gallery gate passed: $(jq '.cases | length' "$OUT/gallery.json") redacted public gallery entries generated"
