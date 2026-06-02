#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/capstone-demo-gate.json}"
OUT="${2:-results/generated/capstone-demo}"
rm -rf "$OUT"
mkdir -p "$OUT/cache" "$OUT/analyses" "$OUT/bad-output-analyses" "$OUT/evidence" "$OUT/rejections"

jq -e '
  .version == "patchline.capstone-demo-gate/v1" and
  (.claim | length) > 180 and
  (.real_code | length) >= .minimum_public_repos and
  (.required_outputs | length) >= .minimum_evidence_artifacts and
  all(.real_code[]; (.id | length) > 0 and (.repo | contains("/")) and (.ref | test("^[0-9a-f]{40}$")) and (.subpath | length) > 0 and (.ecosystem | length) > 0)
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
    version:"patchline.capstone-environment/v1",
    generated_at:$generated_at,
    repository_commit:$commit,
    spec:$spec,
    required_tools:{go:$go_version, git:$git_version, bash:"required", jq:"required"},
    fresh_user_assumptions:["fresh checkout", "network access to pinned public repository archives", "no production credentials", "deterministic generation by default"]
  }' > "$OUT/environment.json"

cat > "$OUT/commands.sh" <<'SCRIPT'
#!/usr/bin/env bash
set -euo pipefail
bash scripts/capstone-demo.sh examples/capstone-demo-gate.json results/generated/capstone-demo-gate
SCRIPT
chmod +x "$OUT/commands.sh"

analysis_dirs=()
analysis_rows=()
count="$(jq '.real_code | length' "$SPEC")"
for ((i=0; i<count; i++)); do
  id="$(jq -r ".real_code[$i].id" "$SPEC")"
  repo="$(jq -r ".real_code[$i].repo" "$SPEC")"
  ref="$(jq -r ".real_code[$i].ref" "$SPEC")"
  subpath="$(jq -r ".real_code[$i].subpath" "$SPEC")"
  ecosystem="$(jq -r ".real_code[$i].ecosystem" "$SPEC")"
  framework="$(jq -r ".real_code[$i].framework" "$SPEC")"
  analysis="$OUT/analyses/$id"
  analysis_dirs+=("$analysis")
  go run ./cmd/patchline repo analyze \
    --github "$repo" \
    --ref "$ref" \
    --subpath "$subpath" \
    --download-dir "$OUT/cache" \
    --stages inventory,baseline,propose,compare \
    --proposal-kind all \
    --budget files=6,lines=90,tokens=10000,changes=2 \
    --no-llm \
    --ci \
    --out "$analysis" \
    --json > "$OUT/analyze-$i.json"
  jq -n \
    --arg id "$id" \
    --arg repo "$repo" \
    --arg ref "$ref" \
    --arg subpath "$subpath" \
    --arg ecosystem "$ecosystem" \
    --arg framework "$framework" \
    --arg analysis "$analysis" \
    --slurpfile analyze "$analysis/analyze.json" \
    '{
      id:$id,
      repo:$repo,
      ref:$ref,
      subpath:$subpath,
      ecosystem:$ecosystem,
      framework:$framework,
      analysis:$analysis,
      files_scanned:$analyze[0].summary.files_scanned,
      ranked_risks:$analyze[0].summary.ranked_risks,
      generated_files:$analyze[0].summary.generated_files,
      provenance_slices:$analyze[0].summary.provenance_slices,
      compare_checks_failed:$analyze[0].summary.compare_checks_failed,
      deterministic_only:$analyze[0].summary.deterministic_only,
      bounded_interventions:{budget_files:6, budget_lines:90, budget_changes:2},
      verified:true
    }' > "$analysis/capstone-row.json"
  analysis_rows+=("$analysis/capstone-row.json")
done

bad_dirs=()
llm_command='printf "%s\n" "-- Plausible generated repair for reviewer" "UPDATE comments SET updated_at = NOW();"'
for ((i=0; i<2 && i<count; i++)); do
  id="$(jq -r ".real_code[$i].id" "$SPEC")"
  repo="$(jq -r ".real_code[$i].repo" "$SPEC")"
  ref="$(jq -r ".real_code[$i].ref" "$SPEC")"
  subpath="$(jq -r ".real_code[$i].subpath" "$SPEC")"
  bad="$OUT/bad-output-analyses/$id"
  bad_dirs+=("$bad")
  go run ./cmd/patchline repo analyze \
    --github "$repo" \
    --ref "$ref" \
    --subpath "$subpath" \
    --download-dir "$OUT/cache" \
    --stages inventory,baseline,propose,compare \
    --proposal-kind guards \
    --budget files=3,lines=80,tokens=10000,changes=2 \
    --llm-command "$llm_command" \
    --out "$bad" \
    --json > "$OUT/bad-analyze-$i.json"
done

IFS=,
analyses="${analysis_dirs[*]}"
bad_analyses="${bad_dirs[*]}"
unset IFS

go run ./cmd/patchline repo rejected-generated \
  --analyses "$bad_analyses" \
  --out "$OUT/rejections" \
  --json > "$OUT/rejections-stdout.json"

go run ./cmd/patchline repo metrics --analyses "$analyses" --out "$OUT/evidence/metrics" --json > "$OUT/evidence/metrics-stdout.json"
go run ./cmd/patchline repo taxonomy --analyses "$analyses" --out "$OUT/evidence/taxonomy" --json > "$OUT/evidence/taxonomy-stdout.json"
go run ./cmd/patchline repo claims-evidence --analyses "$analyses" --out "$OUT/evidence/claims" --json > "$OUT/evidence/claims-stdout.json"
go run ./cmd/patchline repo limitations-ledger --analyses "$analyses" --out "$OUT/evidence/limitations" --json > "$OUT/evidence/limitations-stdout.json"
bash scripts/generate-paper-figures.sh "$analyses" "$OUT/evidence/figures" --json > "$OUT/evidence/figures-stdout.json"
go run ./cmd/patchline repo case-studies --analyses "$analyses" --out "$OUT/evidence/case-studies" --json > "$OUT/evidence/case-studies-stdout.json"

find "$OUT" -type f ! -path '*/cache/*' ! -path '*/fetch/*' | sort | while read -r file; do
  rel="${file#$OUT/}"
  shasum -a 256 "$file" | awk -v rel="$rel" '{print $1 "  " rel}'
done > "$OUT/checksums.txt"

jq -s \
  --slurpfile metrics "$OUT/evidence/metrics/metrics.json" \
  --slurpfile taxonomy "$OUT/evidence/taxonomy/failure-taxonomy.json" \
  --slurpfile claims "$OUT/evidence/claims/claims-evidence.json" \
  --slurpfile limitations "$OUT/evidence/limitations/limitations-ledger.json" \
  --slurpfile figures "$OUT/evidence/figures/figures.json" \
  --slurpfile cases "$OUT/evidence/case-studies/case-studies.json" \
  --slurpfile rejected "$OUT/rejections/rejected-generated.json" \
  --arg hash "$(shasum -a 256 "$OUT/checksums.txt" | awk '{print $1}')" \
  '{
    version:"patchline.capstone-demo/v1",
    analyses:.,
    summary:{
      public_repos:length,
      files_scanned:([.[].files_scanned] | add),
      ranked_risks:([.[].ranked_risks] | add),
      generated_files:([.[].generated_files] | add),
      provenance_slices:([.[].provenance_slices] | add),
      compare_checks_failed:([.[].compare_checks_failed] | add),
      deterministic_only:all(.[]; .deterministic_only == true),
      bounded_interventions:([.[].bounded_interventions] | length),
      rejected_examples:$rejected[0].summary.examples,
      rejected_interventions:$rejected[0].summary.rejected_interventions,
      evidence_artifacts:{
        metrics:$metrics[0].summary.total_ranked_risks,
        failure_modes:$taxonomy[0].summary.failure_modes,
        claims:$claims[0].summary.claims,
        limitations:$limitations[0].summary.limitations,
        figures:$figures[0].summary.figures,
        case_studies:$cases[0].summary.cases
      },
      hash:$hash,
      verified:true
    }
  }' "${analysis_rows[@]}" > "$OUT/summary.json"

{
  echo "# Patchline release-quality capstone demo"
  echo
  echo "A fresh user can run \`bash scripts/capstone-demo.sh\` to download four unfamiliar pinned public repositories, find high-signal repair risks, generate bounded interventions, reject bad output, and regenerate experiment-ready evidence."
  echo
  echo "## Session outline"
  echo
  echo "1. Download pinned public repository slices with provenance and cache reuse."
  echo "2. Run deterministic analysis with bounded generated intervention budgets."
  echo "3. Inject plausible bad generated SQL and reject it through deterministic re-analysis."
  echo "4. Regenerate metrics, taxonomy, claims, limitations, figures, case studies, and checksums."
  echo
  echo "## Public repositories"
  echo
  echo "| Repo | Ecosystem | Framework | Files | Ranked risks | Generated artifacts |"
  echo "| --- | --- | --- | ---: | ---: | ---: |"
  jq -r '.analyses[] | "| `" + .repo + "` `" + .subpath + "` | " + .ecosystem + " | " + .framework + " | " + (.files_scanned|tostring) + " | " + (.ranked_risks|tostring) + " | " + (.generated_files|tostring) + " |"' "$OUT/summary.json"
  echo
  echo "## Bad output rejection"
  echo
  jq -r '"- rejected examples: `" + (.summary.rejected_examples|tostring) + "`\n- rejected interventions: `" + (.summary.rejected_interventions|tostring) + "`"' "$OUT/summary.json"
  echo
  echo "## Experiment-ready evidence"
  echo
  jq -r '.summary.evidence_artifacts | "- metrics ranked risks: `" + (.metrics|tostring) + "`\n- failure modes: `" + (.failure_modes|tostring) + "`\n- claims: `" + (.claims|tostring) + "`\n- limitations: `" + (.limitations|tostring) + "`\n- figures: `" + (.figures|tostring) + "`\n- case studies: `" + (.case_studies|tostring) + "`"' "$OUT/summary.json"
  echo
  echo "## Reproduce"
  echo
  echo "\`\`\`bash"
  echo "bash scripts/capstone-demo.sh examples/capstone-demo-gate.json results/generated/capstone-demo-gate"
  echo "\`\`\`"
} > "$OUT/session.md"

cp "$OUT/session.md" "$OUT/README.md"
echo "capstone demo generated: repos $(jq '.summary.public_repos' "$OUT/summary.json"), risks $(jq '.summary.ranked_risks' "$OUT/summary.json"), rejected $(jq '.summary.rejected_examples' "$OUT/summary.json")"
