#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/plugin-interface-gate.json}"
OUT="${2:-results/generated/plugin-interface-gate}"
DOC="${3:-docs/plugin-interfaces.md}"
rm -rf "$OUT"
mkdir -p "$OUT/cases" "$OUT/cache"

jq -e '
  .version == "patchline.plugin-interface-gate/v1" and
  .minimum_public_repos >= 4 and
  (.slices | length) >= .minimum_public_repos and
  (.required_kinds | sort) == (["compare-check","fact-extractor","linker","parser","proposal-generator","ranker","report-renderer"] | sort) and
  all(.slices[]; (.id | length) > 0 and (.repo | length) > 0 and (.ref | length) == 40 and (.subpath | length) > 0)
' "$SPEC" > /dev/null

for term in \
  "Parser" \
  "FactExtractor" \
  "Linker" \
  "Ranker" \
  "ProposalGenerator" \
  "CompareCheck" \
  "ReportRenderer" \
  "plugins probe"; do
  grep -F "$term" "$DOC" > /dev/null
done

go test ./internal/plugins ./cmd/patchline

catalog="$OUT/catalog.json"
go run ./cmd/patchline plugins list --json > "$catalog"
jq -e '
  .version == "patchline.plugin-catalog/v1" and
  (.hash | length) > 0 and
  ([.plugins[].kind] | unique | sort) == (["compare-check","fact-extractor","linker","parser","proposal-generator","ranker","report-renderer"] | sort) and
  all(.plugins[]; .deterministic == true and (.name | length) > 0 and (.description | length) > 0)
' "$catalog" > /dev/null

rows=()
while IFS=$'\t' read -r id repo ref subpath; do
  case_out="$OUT/cases/$id"
  mkdir -p "$case_out"
  go run ./cmd/patchline plugins probe \
    --github "$repo" \
    --ref "$ref" \
    --subpath "$subpath" \
    --download-dir "$OUT/cache" \
    --out "$case_out/probe" \
    --proposal-kind all \
    --budget files=4,lines=80,tokens=12000,changes=2 \
    --json > "$case_out/stdout.json"

  test -s "$case_out/probe/fetch/source.json"
  test -s "$case_out/probe/plugin-probe.json"
  test -s "$case_out/probe/plugin-probe.md"
  test -s "$case_out/probe/baseline/baseline.json"
  test -s "$case_out/probe/proposal/proposal.json"
  test -s "$case_out/probe/compare/compare.json"
  test -s "$case_out/probe/rendered/baseline.json"
  test -s "$case_out/probe/rendered/proposal.json"
  test -s "$case_out/probe/rendered/compare.json"
  test -s "$case_out/probe/rendered/baseline.md"

  jq -e '
    .version == "patchline.plugin-probe/v1" and
    (.catalog.hash | length) > 0 and
    .summary.parsers >= 1 and
    .summary.fact_extractors >= 1 and
    .summary.linkers >= 1 and
    .summary.rankers >= 1 and
    .summary.proposal_generators >= 1 and
    .summary.compare_checks >= 1 and
    .summary.report_renderers >= 2 and
    .summary.files_scanned > 0 and
    .summary.facts > 0 and
    .summary.ranked_risks > 0 and
    .summary.generated_files > 0 and
    .summary.generated_checks > 0 and
    .summary.rendered_reports == 4 and
    (.rendered | length) == 4 and
    all(.rendered[]; (.hash | length) > 0 and .bytes > 0) and
    (.hash | length) > 0
  ' "$case_out/probe/plugin-probe.json" > /dev/null

  jq -n \
    --arg id "$id" \
    --arg repo "$repo" \
    --arg subpath "$subpath" \
    --slurpfile probe "$case_out/probe/plugin-probe.json" \
    '{
      id:$id,
      repo:$repo,
      subpath:$subpath,
      files_scanned:$probe[0].summary.files_scanned,
      ranked_risks:$probe[0].summary.ranked_risks,
      generated_files:$probe[0].summary.generated_files,
      generated_checks:$probe[0].summary.generated_checks,
      rendered_reports:$probe[0].summary.rendered_reports,
      catalog_hash:$probe[0].catalog.hash,
      verified:true
    }' > "$case_out/row.json"
  rows+=("$case_out/row.json")
done < <(jq -r '.slices[] | [.id, .repo, .ref, .subpath] | @tsv' "$SPEC")

jq -n \
  --slurpfile spec "$SPEC" \
  --slurpfile rows <(jq -s '.' "${rows[@]}") \
  '{
    version:"patchline.plugin-interface-gate-results/v1",
    claim:$spec[0].claim,
    slices:$rows[0],
    summary:{
      public_repos:($rows[0] | map(.repo) | unique | length),
      verified:($rows[0] | map(select(.verified == true)) | length),
      files_scanned:($rows[0] | map(.files_scanned) | add),
      ranked_risks:($rows[0] | map(.ranked_risks) | add),
      generated_files:($rows[0] | map(.generated_files) | add),
      generated_checks:($rows[0] | map(.generated_checks) | add),
      rendered_reports:($rows[0] | map(.rendered_reports) | add),
      catalog_hashes:($rows[0] | map(.catalog_hash) | unique)
    }
  }' > "$OUT/summary.json"

jq -e --slurpfile spec "$SPEC" '
  .version == "patchline.plugin-interface-gate-results/v1" and
  .summary.public_repos >= $spec[0].minimum_public_repos and
  .summary.verified >= $spec[0].minimum_public_repos and
  .summary.files_scanned > 0 and
  .summary.ranked_risks > 0 and
  .summary.generated_files > 0 and
  .summary.generated_checks > 0 and
  .summary.rendered_reports >= 16 and
  (.summary.catalog_hashes | length) == 1
' "$OUT/summary.json" > /dev/null

echo "plugin interface gate passed: $(jq '.summary.public_repos' "$OUT/summary.json") public repo slices exercised all plugin interfaces"
