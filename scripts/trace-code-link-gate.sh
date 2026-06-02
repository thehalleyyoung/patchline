#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/trace-code-links.json}"
OUT="${2:-results/generated/trace-code-link-gate}"
rm -rf "$OUT"
mkdir -p "$OUT/cases"

jq -e '.version == "patchline.trace-code-links/v1" and .minimum_public_repos >= 4 and (.required_kinds | index("opentelemetry")) != null and (.required_kinds | index("datadog")) != null and (.required_kinds | index("structured_log")) != null and (.required_kinds | index("incident_timeline")) != null' "$SPEC" > /dev/null

rows=()
while IFS=$'\t' read -r id repo ref subpath; do
  case_out="$OUT/cases/$id"
  mkdir -p "$case_out"
  go run ./cmd/patchline repo analyze --github "$repo" --ref "$ref" --subpath "$subpath" --stages inventory,baseline --out "$case_out/analyze" --json > "$case_out/analyze.json"
  jq -e '
    .summary.trace_code_links > 0 and
    (.trace_code_links | length) == .summary.trace_code_links and
    any(.trace_code_links[]; (.source_path | length) > 0 and (.code_path | length) > 0 and ((.identifiers | length) > 0 or (.signals | length) > 0))
  ' "$case_out/analyze/baseline/baseline.json" > /dev/null
  jq -n \
    --arg id "$id" \
    --arg repo "$repo" \
    --arg ref "$ref" \
    --arg subpath "$subpath" \
    --slurpfile baseline "$case_out/analyze/baseline/baseline.json" \
    '{
      id:$id,
      repo:$repo,
      ref:$ref,
      subpath:$subpath,
      kind:"repo-slice",
      links:$baseline[0].summary.trace_code_links,
      exact:$baseline[0].summary.trace_links_exact,
      causal:$baseline[0].summary.trace_links_causal,
      temporal:$baseline[0].summary.trace_links_temporal,
      inferred:$baseline[0].summary.trace_links_inferred,
      kinds:($baseline[0].trace_code_links | map(.kind) | unique),
      verified:true
    }' > "$case_out/row.json"
  rows+=("$case_out/row.json")
done < <(jq -r '.slices[] | [.id, .repo, .ref, .subpath] | @tsv' "$SPEC")

jq -n \
  --slurpfile spec "$SPEC" \
  --slurpfile rows <(jq -s '.' "${rows[@]}") \
  '{
    version:"patchline.trace-code-links-results/v1",
    claim:$spec[0].claim,
    slices:$rows[0],
    summary:{
      public_repos:($rows[0] | map(.repo) | unique | length),
      verified:($rows[0] | map(select(.verified == true)) | length),
      links:($rows[0] | map(.links // 0) | add),
      exact:($rows[0] | map(.exact // 0) | add),
      causal:($rows[0] | map(.causal // 0) | add),
      temporal:($rows[0] | map(.temporal // 0) | add),
      inferred:($rows[0] | map(.inferred // 0) | add),
      kinds:($rows[0] | map(.kinds[]) | unique)
    }
  }' > "$OUT/trace-code-links.json"

jq -e --slurpfile spec "$SPEC" '
  .summary.public_repos >= $spec[0].minimum_public_repos and
  .summary.verified == (.slices | length) and
  .summary.links >= (.slices | length) and
  (.summary.kinds as $kinds | all($spec[0].required_kinds[]; $kinds | index(.)))
' "$OUT/trace-code-links.json" > /dev/null

echo "trace-code link gate passed: $(jq '.summary.public_repos' "$OUT/trace-code-links.json") public repos, $(jq '.summary.links' "$OUT/trace-code-links.json") links (kinds=$(jq -r '.summary.kinds | join(",")' "$OUT/trace-code-links.json"), exact=$(jq '.summary.exact' "$OUT/trace-code-links.json"), temporal=$(jq '.summary.temporal' "$OUT/trace-code-links.json"), inferred=$(jq '.summary.inferred' "$OUT/trace-code-links.json"))"
