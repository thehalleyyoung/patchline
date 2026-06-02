#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/paper-appendix-gate.json}"
OUT="${2:-results/generated/paper-appendix-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '
  .version == "patchline.paper-appendix-gate/v1" and
  (.required_sections | length) >= 5 and
  (.required_tables | length) >= .minimum_tables and
  (.required_outputs | length) >= 8
' "$SPEC" > /dev/null

for phrase in "paper appendix" "claims" "limitations" "figures" "tables" "reproduction commands" "make paper-appendix-gate"; do
  grep -F "$phrase" docs/paper-appendix.md README.md > /dev/null
done

bash scripts/generate-paper-appendix.sh "$SPEC" "$OUT" > "$OUT.run.log"

while read -r output; do
  test -s "$OUT/$output"
done < <(jq -r '.required_outputs[]' "$SPEC")

min_repos="$(jq '.minimum_public_repos' "$SPEC")"
min_claims="$(jq '.minimum_claims' "$SPEC")"
min_limitations="$(jq '.minimum_limitations' "$SPEC")"
min_figures="$(jq '.minimum_figures' "$SPEC")"
min_tables="$(jq '.minimum_tables' "$SPEC")"
min_commands="$(jq '.minimum_reproduction_commands' "$SPEC")"
jq -e --argjson min_repos "$min_repos" --argjson min_claims "$min_claims" --argjson min_limitations "$min_limitations" --argjson min_figures "$min_figures" --argjson min_tables "$min_tables" --argjson min_commands "$min_commands" '
  .version == "patchline.paper-appendix/v1" and
  .summary.public_repos >= $min_repos and
  .summary.claims >= $min_claims and
  .summary.limitations >= $min_limitations and
  .summary.figures >= $min_figures and
  .summary.tables >= $min_tables and
  .summary.reproduction_commands >= $min_commands and
  .summary.ranked_risks > 100 and
  .summary.generated_files > 0 and
  .summary.verified == true and
  (.source_artifacts | keys | length) >= 6 and
  all(.tables[]; (.rows | length) > 0)
' "$OUT/appendix.json" > /dev/null

while read -r section; do
  grep -F "## $section" "$OUT/appendix.md" > /dev/null
done < <(jq -r '.required_sections[]' "$SPEC")
while read -r table; do
  test -s "$OUT/tables/$table.md"
  test -s "$OUT/tables/$table.json"
  grep -F "### $table" "$OUT/appendix.md" > /dev/null
done < <(jq -r '.required_tables[]' "$SPEC")

grep -F "Patchline release-quality capstone demo" "$OUT/evidence/artifact-container-rebuild/public-results/capstone/session.md" > /dev/null
grep -F "make artifact-container-profile-gate" "$OUT/appendix.md" > /dev/null
grep -F "bash scripts/capstone-demo.sh" "$OUT/appendix.md" > /dev/null

jq -n \
  --slurpfile appendix "$OUT/appendix.json" \
  '{
    version:"patchline.paper-appendix-gate-results/v1",
    public_repos:$appendix[0].summary.public_repos,
    claims:$appendix[0].summary.claims,
    limitations:$appendix[0].summary.limitations,
    figures:$appendix[0].summary.figures,
    tables:$appendix[0].summary.tables,
    reproduction_commands:$appendix[0].summary.reproduction_commands,
    verified:true
  }' > "$OUT/gate-summary.json"

echo "paper appendix gate passed: claims $(jq '.claims' "$OUT/gate-summary.json"), limitations $(jq '.limitations' "$OUT/gate-summary.json"), figures $(jq '.figures' "$OUT/gate-summary.json")"
