#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/repository-size-stratification-gate.json}"
OUT="${2:-results/generated/repository-size-stratification}"
rm -rf "$OUT"
mkdir -p "$OUT/analyses" "$OUT/cache"

CATALOG="$(jq -r '.source_catalog' "$SPEC")"

jq -e '
  .version == "patchline.repository-size-stratification-gate/v1" and
  (.claim | length) > 100 and
  (.required_strata | length) == 4 and
  (.proof_slices | length) == 4 and
  all(.proof_slices[]; (.repo | contains("/")) and (.ref | test("^[0-9a-f]{40}$")) and (.subpath | length) > 0)
' "$SPEC" > /dev/null

jq -e '.version == "patchline.real-repo-catalog/v1" and (.slices | length) >= 25' "$CATALOG" > /dev/null

# Classify every catalog slice into one of the four repository-size/type strata.
jq -n \
  --slurpfile spec "$SPEC" \
  --slurpfile catalog "$CATALOG" \
  '
  ($spec[0].infrastructure_frameworks) as $infra |
  ($catalog[0].slices | map(
    . as $s |
    (if $s.repo_size_class == "small" then "small-app"
     elif $s.repo_size_class == "medium" then "medium-service"
     elif ($infra | index($s.migration_framework)) then "infrastructure-heavy"
     elif $s.monorepo then "monorepo"
     else "medium-service" end) as $stratum |
    {id: $s.id, repo: $s.repo, ref: $s.ref, subpath: $s.subpath, ecosystem: $s.ecosystem, migration_framework: $s.migration_framework, repo_size_class: $s.repo_size_class, monorepo: $s.monorepo, size_stratum: $stratum}
  )) as $classified |
  {
    version: "patchline.repository-size-stratification/v1",
    strata: (["small-app","medium-service","monorepo","infrastructure-heavy"] | map(. as $st | {
      stratum: $st,
      count: ([$classified[] | select(.size_stratum == $st)] | length),
      slices: [$classified[] | select(.size_stratum == $st) | {id, repo, subpath, ref, ecosystem, migration_framework}]
    })),
    classified: $classified
  }
  ' > "$OUT/size-strata.json"

# Prove each stratum with a representative real download.
proof_jsonl="$OUT/proof.jsonl"
: > "$proof_jsonl"
proof_count="$(jq '.proof_slices | length' "$SPEC")"
for ((p=0; p<proof_count; p++)); do
  stratum="$(jq -r ".proof_slices[$p].stratum" "$SPEC")"
  repo="$(jq -r ".proof_slices[$p].repo" "$SPEC")"
  ref="$(jq -r ".proof_slices[$p].ref" "$SPEC")"
  subpath="$(jq -r ".proof_slices[$p].subpath" "$SPEC")"
  ecosystem="$(jq -r ".proof_slices[$p].ecosystem" "$SPEC")"
  id="$(printf '%s-%s' "$stratum" "${repo//\//-}" | tr -c 'A-Za-z0-9_.-' '-')"
  analysis="$OUT/analyses/$id"
  mkdir -p "$analysis"
  go run ./cmd/patchline repo analyze \
    --github "$repo" \
    --ref "$ref" \
    --subpath "$subpath" \
    --download-dir "$OUT/cache" \
    --stages inventory,baseline \
    --no-llm \
    --out "$analysis" \
    --json > "$OUT/analyze-$id.json"
  jq -n \
    --arg stratum "$stratum" --arg repo "$repo" --arg ref "$ref" \
    --arg subpath "$subpath" --arg ecosystem "$ecosystem" \
    --slurpfile analyze "$analysis/analyze.json" \
    '{
      stratum:$stratum, repo:$repo, ref:$ref, subpath:$subpath, ecosystem:$ecosystem,
      files_scanned:$analyze[0].summary.files_scanned,
      facts:$analyze[0].summary.facts,
      ranked_risks:$analyze[0].summary.ranked_risks,
      hash:$analyze[0].hash,
      verified:(($analyze[0].summary.files_scanned > 0) and ($analyze[0].summary.facts > 0) and (($analyze[0].hash | length) > 0))
    }' >> "$proof_jsonl"
done

jq -n \
  --slurpfile strata "$OUT/size-strata.json" \
  --slurpfile proof <(jq -s '.' "$proof_jsonl") \
  '{
    version: "patchline.repository-size-stratification/v1",
    strata: ($strata[0].strata | map({stratum, count})),
    proof: {
      slices: $proof[0],
      strata_proven: ($proof[0] | map(.stratum) | unique | length),
      total_files: ([$proof[0][].files_scanned] | add // 0),
      total_facts: ([$proof[0][].facts] | add // 0),
      total_ranked_risks: ([$proof[0][].ranked_risks] | add // 0),
      verified: (all($proof[0][]; .verified == true))
    },
    summary: {
      strata: ($strata[0].strata | length),
      strata_populated: ([$strata[0].strata[] | select(.count > 0)] | length),
      catalog_slices: ($strata[0].classified | length),
      proof_slices: ($proof[0] | length),
      total_facts: ([$proof[0][].facts] | add // 0),
      verified: (([$strata[0].strata[] | select(.count > 0)] | length) == 4 and (all($proof[0][]; .verified == true)))
    }
  }' > "$OUT/repository-size-stratification.json"

{
  echo "# Repository-size stratification"
  echo
  echo "Patchline stratifies the real-repo catalog into repository-size/type strata (small apps, medium services, monorepos, and infrastructure-heavy repos) and proves a representative real download from each stratum analyzes end-to-end."
  echo
  echo "## Strata (catalog classification)"
  echo
  echo "| Stratum | Catalog slices |"
  echo "| --- | ---: |"
  jq -r '.strata[] | "| " + .stratum + " | " + (.count|tostring) + " |"' "$OUT/repository-size-stratification.json"
  echo
  echo "## Real-code proof sample"
  echo
  echo "| Stratum | Repo | Ecosystem | Files | Facts | Ranked risks | Verified |"
  echo "| --- | --- | --- | ---: | ---: | ---: | :---: |"
  jq -r '.proof.slices[] | "| " + .stratum + " | `" + .repo + "` | " + .ecosystem + " | " + (.files_scanned|tostring) + " | " + (.facts|tostring) + " | " + (.ranked_risks|tostring) + " | " + (if .verified then "yes" else "no" end) + " |"' "$OUT/repository-size-stratification.json"
} > "$OUT/repository-size-stratification.md"

cp "$OUT/repository-size-stratification.md" "$OUT/README.md"
echo "repository-size stratification complete: strata populated $(jq '.summary.strata_populated' "$OUT/repository-size-stratification.json")/4, proof strata $(jq '.proof.strata_proven' "$OUT/repository-size-stratification.json"), proof facts $(jq '.summary.total_facts' "$OUT/repository-size-stratification.json")"
