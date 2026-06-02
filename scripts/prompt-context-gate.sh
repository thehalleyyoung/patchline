#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/prompt-context-gate.json}"
OUT="${2:-results/generated/prompt-context-gate}"
rm -rf "$OUT"
mkdir -p "$OUT/cache"

jq -e '
  .version == "patchline.prompt-context-gate/v1" and
  (.claim | length) > 80 and
  (.real_code.repo | length) > 0 and
  (.real_code.ref | test("^[0-9a-f]{40}$")) and
  (.real_code.subpath | length) > 0 and
  (.required_artifacts | length) >= 4
' "$SPEC" > /dev/null

grep -F "make prompt-context-gate" docs/prompt-context-minimization.md > /dev/null
grep -F "prompt_context_minimization" docs/prompt-context-minimization.md > /dev/null
grep -F "prompt-context.json" README.md > /dev/null

go test ./internal/project -run TestProposalPromptContextMinimizesUnselectedEvidence > "$OUT/go-test.log"

read -r repo ref subpath < <(jq -r '[.real_code.repo, .real_code.ref, .real_code.subpath] | @tsv' "$SPEC")
go run ./cmd/patchline repo analyze \
  --github "$repo" \
  --ref "$ref" \
  --subpath "$subpath" \
  --download-dir "$OUT/cache" \
  --stages inventory,baseline,propose \
  --proposal-kind all \
  --budget files=4,lines=80,tokens=12000,changes=1 \
  --no-llm \
  --out "$OUT/analyze" \
  --json > "$OUT/stdout.json"

while IFS= read -r artifact; do
  test -s "$OUT/analyze/$artifact"
done < <(jq -r '.required_artifacts[]' "$SPEC")

jq -e '
  .prompt_context_minimization.applied == true and
  .prompt_context_minimization.selected_risks == 1 and
  .prompt_context_minimization.excluded_risks > 0 and
  .prompt_context_minimization.excluded_evidence_links >= 0 and
  .prompt_context_minimization.excluded_provenance_slices >= 0 and
  .prompt_context_minimization.excluded_excerpt_lines > 0 and
  (.target_risk_ids | length) == 1
' "$OUT/analyze/proposal/proposal.json" > /dev/null

jq -e '
  .minimization.applied == true and
  .minimization.selected_risks == 1 and
  .minimization.excluded_risks > 0 and
  (.risks | length) == 1 and
  all(.risks[]; (.fact_hashes | length) <= 8 and (.evidence_paths | length) <= 8)
' "$OUT/analyze/proposal/prompt-context.json" > /dev/null

grep -F "Context minimization" "$OUT/analyze/proposal/prompt.txt" > /dev/null
grep -F "selected=1" "$OUT/analyze/proposal/prompt.txt" > /dev/null
grep -F "excluded=" "$OUT/analyze/proposal/prompt.txt" > /dev/null

jq -n \
  --slurpfile proposal "$OUT/analyze/proposal/proposal.json" \
  --slurpfile baseline "$OUT/analyze/baseline/baseline.json" \
  --slurpfile spec "$SPEC" \
  '{
    version:"patchline.prompt-context-gate-results/v1",
    repo:$spec[0].real_code.repo,
    baseline_risks:($baseline[0].risks | length),
    selected_risks:$proposal[0].prompt_context_minimization.selected_risks,
    excluded_risks:$proposal[0].prompt_context_minimization.excluded_risks,
    included_evidence_links:$proposal[0].prompt_context_minimization.included_evidence_links,
    excluded_evidence_links:$proposal[0].prompt_context_minimization.excluded_evidence_links,
    included_provenance_slices:$proposal[0].prompt_context_minimization.included_provenance_slices,
    excluded_provenance_slices:$proposal[0].prompt_context_minimization.excluded_provenance_slices,
    excluded_excerpt_lines:$proposal[0].prompt_context_minimization.excluded_excerpt_lines,
    verified:true
  }' > "$OUT/summary.json"

jq -e '.verified == true and .baseline_risks > .selected_risks and .excluded_risks > 0 and .excluded_excerpt_lines > 0' "$OUT/summary.json" > /dev/null

echo "prompt context gate passed: selected $(jq '.selected_risks' "$OUT/summary.json") risk, excluded $(jq '.excluded_risks' "$OUT/summary.json") risks on $(jq -r '.repo' "$OUT/summary.json")"
