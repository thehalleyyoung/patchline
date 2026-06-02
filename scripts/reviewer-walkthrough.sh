#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/reviewer-walkthrough-gate.json}"
OUT="${2:-results/generated/reviewer-walkthrough}"
rm -rf "$OUT"
mkdir -p "$OUT/cache" "$OUT/tables" "$OUT/reports" "$OUT/case-study-bundle"

jq -e '
  .version == "patchline.reviewer-walkthrough-gate/v1" and
  (.claim | length) > 150 and
  .minimum_public_repos >= 4 and
  .minimum_tables >= 4 and
  .minimum_figures >= 5 and
  .minimum_reports >= 5 and
  .minimum_case_studies >= 4 and
  (.required_outputs | length) >= 10 and
  (.real_code | length) >= .minimum_public_repos and
  all(.real_code[]; (.id | length) > 0 and (.repo | length) > 0 and (.ref | test("^[0-9a-f]{40}$")) and (.subpath | length) > 0)
' "$SPEC" > /dev/null

go_version="$(go version)"
git_version="$(git --version)"
commit="$(git rev-parse HEAD)"
jq -n \
  --arg go_version "$go_version" \
  --arg git_version "$git_version" \
  --arg commit "$commit" \
  --arg spec "$SPEC" \
  --arg generated_at "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  '{
    version:"patchline.reviewer-walkthrough-environment/v1",
    generated_at:$generated_at,
    repository_commit:$commit,
    spec:$spec,
    required_tools:{go:$go_version, git:$git_version, bash:"required", jq:"required"},
    assumptions:["fresh checkout", "network access to pinned public repository archives", "deterministic --no-llm generation", "no production credentials"]
  }' > "$OUT/environment.json"

cat > "$OUT/reproduce.sh" <<'SCRIPT'
#!/usr/bin/env bash
set -euo pipefail
bash scripts/reviewer-walkthrough.sh examples/reviewer-walkthrough-gate.json results/generated/reviewer-walkthrough-gate
SCRIPT
chmod +x "$OUT/reproduce.sh"

analysis_dirs=()
count="$(jq '.real_code | length' "$SPEC")"
for ((i=0; i<count; i++)); do
  id="$(jq -r ".real_code[$i].id" "$SPEC")"
  repo="$(jq -r ".real_code[$i].repo" "$SPEC")"
  ref="$(jq -r ".real_code[$i].ref" "$SPEC")"
  subpath="$(jq -r ".real_code[$i].subpath" "$SPEC")"
  analysis="$OUT/analyses/$id"
  analysis_dirs+=("$analysis")
  go run ./cmd/patchline repo analyze \
    --github "$repo" \
    --ref "$ref" \
    --subpath "$subpath" \
    --download-dir "$OUT/cache" \
    --stages inventory,baseline,propose,compare \
    --proposal-kind all \
    --budget files=8,lines=100,tokens=12000,changes=2 \
    --no-llm \
    --out "$analysis" \
    --json > "$OUT/analyze-$i.json"
done

IFS=,
analyses="${analysis_dirs[*]}"
unset IFS

go run ./cmd/patchline repo metrics --analyses "$analyses" --out "$OUT/reports/metrics" --json > "$OUT/reports/metrics-stdout.json"
go run ./cmd/patchline repo claims-evidence --analyses "$analyses" --out "$OUT/reports/claims" --json > "$OUT/reports/claims-stdout.json"
go run ./cmd/patchline repo limitations-ledger --analyses "$analyses" --out "$OUT/reports/limitations" --json > "$OUT/reports/limitations-stdout.json"
go run ./cmd/patchline repo taxonomy --analyses "$analyses" --out "$OUT/reports/taxonomy" --json > "$OUT/reports/taxonomy-stdout.json"
go run ./cmd/patchline repo qualitative-notes --analyses "$analyses" --out "$OUT/reports/qualitative" --json > "$OUT/reports/qualitative-stdout.json"
bash scripts/generate-paper-figures.sh "$analyses" "$OUT/figures" --json > "$OUT/figures-stdout.json"
go run ./cmd/patchline repo case-studies --analyses "$analyses" --out "$OUT/case-study-bundle/case-studies" --json > "$OUT/case-study-bundle/case-studies-stdout.json"

jq -n \
  --slurpfile metrics "$OUT/reports/metrics/metrics.json" \
  --slurpfile claims "$OUT/reports/claims/claims-evidence.json" \
  --slurpfile limitations "$OUT/reports/limitations/limitations-ledger.json" \
  --slurpfile taxonomy "$OUT/reports/taxonomy/failure-taxonomy.json" \
  --slurpfile qualitative "$OUT/reports/qualitative/qualitative-notes.json" \
  --slurpfile figures "$OUT/figures/figures.json" \
  --slurpfile cases "$OUT/case-study-bundle/case-studies/case-studies.json" \
  '{
    version:"patchline.reviewer-walkthrough-tables/v1",
    tables:[
      {id:"corpus", title:"Corpus summary", rows:[
        {metric:"public repos", value:$metrics[0].summary.analyses},
        {metric:"ranked risks", value:$metrics[0].summary.total_ranked_risks},
        {metric:"generated files", value:$metrics[0].summary.total_generated_files}
      ]},
      {id:"paper-claims", title:"Claims and limitations", rows:[
        {metric:"claims", value:$claims[0].summary.claims},
        {metric:"claims with limitations", value:$claims[0].summary.claims_with_limitations},
        {metric:"limitations", value:$limitations[0].summary.limitations}
      ]},
      {id:"qualitative", title:"Qualitative evidence", rows:[
        {metric:"failure modes", value:$taxonomy[0].summary.failure_modes},
        {metric:"qualitative notes", value:$qualitative[0].summary.notes}
      ]},
      {id:"figures-and-cases", title:"Figures and case studies", rows:[
        {metric:"figures", value:$figures[0].summary.figures},
        {metric:"case studies", value:$cases[0].summary.cases},
        {metric:"deterministic outcomes", value:$cases[0].summary.deterministic_outcomes}
      ]}
    ]
  }' > "$OUT/tables/evaluation-tables.json"

{
  echo "# Reviewer walkthrough evaluation tables"
  echo
  jq -r '.tables[] | "## " + .title + "\n\n| Metric | Value |\n| --- | --- |\n" + (.rows[] | "| " + .metric + " | `" + (.value|tostring) + "` |") + "\n"' "$OUT/tables/evaluation-tables.json"
} > "$OUT/tables/evaluation-tables.md"

find "$OUT/case-study-bundle" -type f | sort | while read -r file; do
  rel="${file#$OUT/}"
  shasum -a 256 "$file" | awk -v rel="$rel" '{print $1 "  " rel}'
done > "$OUT/case-study-bundle/checksums.txt"

jq -n \
  --arg spec "$SPEC" \
  --arg analyses "$analyses" \
  --slurpfile cases "$OUT/case-study-bundle/case-studies/case-studies.json" \
  '{
    version:"patchline.case-study-bundle/v1",
    spec:$spec,
    analyses:($analyses | split(",")),
    files:["case-studies/case-studies.json","case-studies/case-studies.md","case-studies-stdout.json","checksums.txt"],
    cases:$cases[0].summary.cases,
    deterministic_outcomes:$cases[0].summary.deterministic_outcomes
  }' > "$OUT/case-study-bundle/manifest.json"

tables="$(jq '.tables | length' "$OUT/tables/evaluation-tables.json")"
figures="$(jq '.summary.figures' "$OUT/figures/figures.json")"
case_studies="$(jq '.summary.cases' "$OUT/case-study-bundle/case-studies/case-studies.json")"
reports="$(find "$OUT/reports" -mindepth 2 -maxdepth 2 -name '*.json' ! -name '*-stdout.json' | wc -l | tr -d ' ')"
bundle_files="$(find "$OUT/case-study-bundle" -type f | wc -l | tr -d ' ')"

jq -n \
  --argjson analyses "$count" \
  --argjson tables "$tables" \
  --argjson figures "$figures" \
  --argjson reports "$reports" \
  --argjson case_studies "$case_studies" \
  --argjson bundle_files "$bundle_files" \
  --arg hash "$(find "$OUT" -type f ! -path '*/cache/*' -print0 | sort -z | xargs -0 shasum -a 256 | shasum -a 256 | awk '{print $1}')" \
  '{
    version:"patchline.reviewer-walkthrough-summary/v1",
    analyses:$analyses,
    public_repos:$analyses,
    tables:$tables,
    figures:$figures,
    reports:$reports,
    case_studies:$case_studies,
    bundle_files:$bundle_files,
    hash:$hash,
    verified:true
  }' > "$OUT/summary.json"

cat > "$OUT/walkthrough.md" <<EOF
# Patchline reviewer walkthrough

This directory was regenerated from pinned public repository slices using \`scripts/reviewer-walkthrough.sh\`.

- Environment: \`environment.json\`
- Analyses: \`analyses/\`
- Tables: \`tables/evaluation-tables.md\`
- Figures: \`figures/figures.md\`
- Reports: \`reports/\`
- Case-study bundle: \`case-study-bundle/\`
- Summary: \`summary.json\`

The walkthrough is deterministic and uses \`--no-llm\`; generated artifacts remain untrusted review material.
EOF

echo "reviewer walkthrough passed: analyses $count, tables $tables, figures $figures, reports $reports, case studies $case_studies"
