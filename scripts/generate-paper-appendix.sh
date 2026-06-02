#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/paper-appendix-gate.json}"
OUT="${2:-results/generated/paper-appendix}"
rm -rf "$OUT"
mkdir -p "$OUT/evidence" "$OUT/tables"

jq -e '
  .version == "patchline.paper-appendix-gate/v1" and
  (.claim | length) > 120 and
  (.required_sections | length) >= 5 and
  (.required_tables | length) >= .minimum_tables and
  (.required_outputs | length) >= 8 and
  (.evidence_profile_spec | startswith("examples/"))
' "$SPEC" > /dev/null

profile_spec="$(jq -r '.evidence_profile_spec' "$SPEC")"
bash scripts/artifact-container-rebuild.sh "$profile_spec" "$OUT/evidence/artifact-container-rebuild" > "$OUT/evidence/artifact-container-rebuild.run.log"

CAP="$OUT/evidence/artifact-container-rebuild/public-results/capstone"
CLAIMS="$CAP/evidence/claims/claims-evidence.json"
LIMITATIONS="$CAP/evidence/limitations/limitations-ledger.json"
FIGURES="$CAP/evidence/figures/figures.json"
METRICS="$CAP/evidence/metrics/metrics.json"
TAXONOMY="$CAP/evidence/taxonomy/failure-taxonomy.json"
REBUILD="$OUT/evidence/artifact-container-rebuild/rebuild-summary.json"

for file in "$CLAIMS" "$LIMITATIONS" "$FIGURES" "$METRICS" "$TAXONOMY" "$REBUILD"; do
  test -s "$file"
done

jq -n \
  --slurpfile rebuild "$REBUILD" \
  --slurpfile metrics "$METRICS" \
  --slurpfile taxonomy "$TAXONOMY" \
  '{
    version:"patchline.paper-appendix-table/v1",
    id:"artifact-summary",
    rows:[
      {metric:"public repositories", value:$rebuild[0].summary.public_repos},
      {metric:"files scanned lower bound", value:$metrics[0].summary.total_files_scanned_lower_bound},
      {metric:"ranked risks", value:$rebuild[0].summary.ranked_risks},
      {metric:"generated files", value:$rebuild[0].summary.generated_files},
      {metric:"failure modes", value:$taxonomy[0].summary.failure_modes},
      {metric:"rejected bad-output examples", value:$rebuild[0].summary.rejected_examples}
    ]
  }' > "$OUT/tables/artifact-summary.json"

{
  echo "| Metric | Value |"
  echo "| --- | ---: |"
  jq -r '.rows[] | "| " + .metric + " | " + (.value|tostring) + " |"' "$OUT/tables/artifact-summary.json"
} > "$OUT/tables/artifact-summary.md"

jq '{
  version:"patchline.paper-appendix-table/v1",
  id:"risk-taxonomy",
  rows:[.failure_modes[] | {failure_mode:.title, occurrences, high_severity, public_repos, generated_interventions}]
}' "$TAXONOMY" > "$OUT/tables/risk-taxonomy.json"
{
  echo "| Failure mode | Occurrences | High severity | Public repos | Generated links |"
  echo "| --- | ---: | ---: | ---: | ---: |"
  jq -r '.rows[] | "| " + .failure_mode + " | " + (.occurrences|tostring) + " | " + (.high_severity|tostring) + " | " + (.public_repos|tostring) + " | " + (.generated_interventions|tostring) + " |"' "$OUT/tables/risk-taxonomy.json"
} > "$OUT/tables/risk-taxonomy.md"

jq '{
  version:"patchline.paper-appendix-table/v1",
  id:"claim-status",
  rows:(.claims | group_by(.section + ":" + .status) | map({section:.[0].section, status:.[0].status, claims:length}))
}' "$CLAIMS" > "$OUT/tables/claim-status.json"
{
  echo "| Section | Status | Claims |"
  echo "| --- | --- | ---: |"
  jq -r '.rows[] | "| " + .section + " | " + .status + " | " + (.claims|tostring) + " |"' "$OUT/tables/claim-status.json"
} > "$OUT/tables/claim-status.md"

jq '{
  version:"patchline.paper-appendix-table/v1",
  id:"limitation-categories",
  rows:(.summary.by_category | to_entries | map({category:.key, limitations:.value}))
}' "$LIMITATIONS" > "$OUT/tables/limitation-categories.json"
{
  echo "| Limitation category | Count |"
  echo "| --- | ---: |"
  jq -r '.rows[] | "| `" + .category + "` | " + (.limitations|tostring) + " |"' "$OUT/tables/limitation-categories.json"
} > "$OUT/tables/limitation-categories.md"

jq -n \
  --arg appendix "make paper-appendix-gate" \
  --arg container "make artifact-container-profile-gate" \
  --arg badges "make artifact-badges-gate" \
  --arg capstone "bash scripts/capstone-demo.sh examples/capstone-demo-gate.json results/generated/capstone-demo-gate" \
  --arg rebuild "bash scripts/artifact-container-rebuild.sh examples/artifact-container-profile-gate.json results/generated/artifact-container-rebuild" \
  '{
    version:"patchline.paper-appendix-table/v1",
    id:"reproduction-commands",
    rows:[
      {purpose:"regenerate this appendix", command:$appendix},
      {purpose:"verify container rebuild profile", command:$container},
      {purpose:"verify artifact badges", command:$badges},
      {purpose:"run capstone directly", command:$capstone},
      {purpose:"run rebuild script directly", command:$rebuild}
    ]
  }' > "$OUT/tables/reproduction-commands.json"
{
  echo "| Purpose | Command |"
  echo "| --- | --- |"
  jq -r '.rows[] | "| " + .purpose + " | `" + .command + "` |"' "$OUT/tables/reproduction-commands.json"
} > "$OUT/tables/reproduction-commands.md"

jq -n \
  --slurpfile claims "$CLAIMS" \
  --slurpfile limitations "$LIMITATIONS" \
  --slurpfile figures "$FIGURES" \
  --slurpfile metrics "$METRICS" \
  --slurpfile taxonomy "$TAXONOMY" \
  --slurpfile rebuild "$REBUILD" \
  --slurpfile artifact_table "$OUT/tables/artifact-summary.json" \
  --slurpfile risk_table "$OUT/tables/risk-taxonomy.json" \
  --slurpfile claim_table "$OUT/tables/claim-status.json" \
  --slurpfile limitation_table "$OUT/tables/limitation-categories.json" \
  --slurpfile command_table "$OUT/tables/reproduction-commands.json" \
  '{
    version:"patchline.paper-appendix/v1",
    source_artifacts:{
      claims:"evidence/artifact-container-rebuild/public-results/capstone/evidence/claims/claims-evidence.json",
      limitations:"evidence/artifact-container-rebuild/public-results/capstone/evidence/limitations/limitations-ledger.json",
      figures:"evidence/artifact-container-rebuild/public-results/capstone/evidence/figures/figures.json",
      metrics:"evidence/artifact-container-rebuild/public-results/capstone/evidence/metrics/metrics.json",
      taxonomy:"evidence/artifact-container-rebuild/public-results/capstone/evidence/taxonomy/failure-taxonomy.json",
      rebuild:"evidence/artifact-container-rebuild/rebuild-summary.json"
    },
    claims:$claims[0].claims,
    limitations:$limitations[0].limitations,
    figures:$figures[0].figures,
    tables:[$artifact_table[0], $risk_table[0], $claim_table[0], $limitation_table[0], $command_table[0]],
    reproduction_commands:$command_table[0].rows,
    summary:{
      public_repos:$rebuild[0].summary.public_repos,
      ranked_risks:$rebuild[0].summary.ranked_risks,
      generated_files:$rebuild[0].summary.generated_files,
      claims:$claims[0].summary.claims,
      limitations:$limitations[0].summary.limitations,
      figures:$figures[0].summary.figures,
      tables:5,
      reproduction_commands:($command_table[0].rows | length),
      failure_modes:$taxonomy[0].summary.failure_modes,
      verified:($rebuild[0].summary.verified == true)
    }
  }' > "$OUT/appendix.json"

{
  echo "# Patchline generated paper appendix"
  echo
  echo "This appendix is rendered from current generated artifacts rather than hand-maintained paper notes."
  echo
  echo "## Claims"
  echo
  jq -r '.claims[] | "- **" + .section + "** (`" + .status + "`): " + .paper_wording' "$OUT/appendix.json"
  echo
  echo "## Limitations"
  echo
  jq -r '.limitations[0:12][] | "- **" + .category + "** (`" + .severity + "`): " + .observation + " " + .why_it_matters' "$OUT/appendix.json"
  echo
  echo "## Figures"
  echo
  jq -r '.figures[] | "- **" + .title + "** (`" + .kind + "`): " + .caption + " Source artifacts: `" + ((.source_artifacts | length)|tostring) + "`."' "$OUT/appendix.json"
  echo
  echo "## Tables"
  for table in artifact-summary risk-taxonomy claim-status limitation-categories reproduction-commands; do
    echo
    echo "### $table"
    echo
    cat "$OUT/tables/$table.md"
  done
  echo
  echo "## Reproduction commands"
  echo
  jq -r '.reproduction_commands[] | "- " + .purpose + ": `" + .command + "`"' "$OUT/appendix.json"
} > "$OUT/appendix.md"

cp "$OUT/appendix.md" "$OUT/README.md"
echo "paper appendix generated: claims $(jq '.summary.claims' "$OUT/appendix.json"), limitations $(jq '.summary.limitations' "$OUT/appendix.json"), figures $(jq '.summary.figures' "$OUT/appendix.json")"
