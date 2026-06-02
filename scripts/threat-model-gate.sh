#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/threat-model-gate.json}"
OUT="${2:-results/generated/threat-model-gate}"
rm -rf "$OUT"
mkdir -p "$OUT/cache" "$OUT/adapter" "$OUT/offline"

jq -e '
  .version == "patchline.threat-model-gate/v1" and
  (.claim | length) > 100 and
  (.real_code.repo | length) > 0 and
  (.real_code.ref | test("^[0-9a-f]{40}$")) and
  (.real_code.subpath | length) > 0 and
  (.adapter_fixture | length) > 0 and
  (.required_boundaries | length) >= 5
' "$SPEC" > /dev/null

while IFS= read -r boundary; do
  grep -F "$boundary" docs/threat-model.md > /dev/null
done < <(jq -r '.required_boundaries[]' "$SPEC")

for term in \
  "untrusted-generated-proposal" \
  "--run-native-tests" \
  "adapter result version" \
  "archive hash" \
  "repo offline" \
  "release checksums" \
  "make threat-model-gate"; do
  grep -F -- "$term" docs/threat-model.md > /dev/null
done
grep -F "make threat-model-gate" README.md > /dev/null

read -r repo ref subpath < <(jq -r '[.real_code.repo, .real_code.ref, .real_code.subpath] | @tsv' "$SPEC")
go run ./cmd/patchline repo analyze \
  --github "$repo" \
  --ref "$ref" \
  --subpath "$subpath" \
  --download-dir "$OUT/cache" \
  --stages inventory,baseline,propose,compare \
  --proposal-kind all \
  --budget files=4,lines=80,tokens=12000,changes=2 \
  --no-llm \
  --ci \
  --out "$OUT/analyze" \
  --json > "$OUT/analyze-stdout.json"

jq -e --arg ref "$ref" --arg subpath "$subpath" '
  .version == "patchline.project/v1" and
  .mode == "github" and
  .resolved_commit == $ref and
  .subpath == $subpath and
  (.archive_hash | length) > 20 and
  (.cache_path | length) > 0 and
  (.scanned_root | length) > 0
' "$OUT/analyze/fetch/source.json" > /dev/null

jq -e '
  .trust == "untrusted-generated-proposal" and
  .deterministic_only == true and
  (.generated_files | length) > 0 and
  .prompt_context_minimization.applied == true
' "$OUT/analyze/proposal/proposal.json" > /dev/null

jq -e '
  .version == "patchline.repo-compare/v1" and
  .intervention_loop.proposal_stage == "generated-untrusted" and
  (.intervention_loop.required_next_actions | index("review generated artifacts as an untrusted intervention")) and
  .intervention_loop.native_checks_run == 0 and
  (.generated_checks | length) > 0 and
  (.native_tests == null or (.native_tests | length) == 0)
' "$OUT/analyze/compare/compare.json" > /dev/null

adapter_fixture="$(jq -r '.adapter_fixture' "$SPEC")"
go run ./cmd/patchline adapt-evidence datadog "$adapter_fixture" \
  --out "$OUT/adapter/events.jsonl" \
  --json > "$OUT/adapter/result.json"

jq -e '
  .version == "patchline.evidence-adapter/v1" and
  .ok == true and
  .adapter == "datadog" and
  .event_count == (.events | length) and
  (.input_hash | length) > 20
' "$OUT/adapter/result.json" > /dev/null

go run ./cmd/patchline repo offline \
  --analysis "$OUT/analyze" \
  --adapter "$OUT/adapter/result.json" \
  --out "$OUT/offline" \
  --json > "$OUT/offline-stdout.json"

jq -e '
  .ok == true and
  .summary.cache_inputs_valid >= 1 and
  .summary.adapters_valid == 1 and
  .summary.generated_artifacts > 0 and
  .summary.network_operations == 0
' "$OUT/offline/offline.json" > /dev/null

test -s "$OUT/analyze/analysis-bundle/summary.sarif"

jq -n \
  --slurpfile source "$OUT/analyze/fetch/source.json" \
  --slurpfile proposal "$OUT/analyze/proposal/proposal.json" \
  --slurpfile compare "$OUT/analyze/compare/compare.json" \
  --slurpfile adapter "$OUT/adapter/result.json" \
  --slurpfile offline "$OUT/offline/offline.json" \
  '{
    version:"patchline.threat-model-gate-results/v1",
    repo:$source[0].input,
    archive_hash:$source[0].archive_hash,
    generated_files:($proposal[0].generated_files | length),
    proposal_stage:$compare[0].intervention_loop.proposal_stage,
    adapter_events:$adapter[0].event_count,
    adapter_input_hash:$adapter[0].input_hash,
    offline_cache_inputs_valid:$offline[0].summary.cache_inputs_valid,
    offline_adapters_valid:$offline[0].summary.adapters_valid,
    network_operations:$offline[0].summary.network_operations,
    verified:true
  }' > "$OUT/summary.json"

jq -e '.verified == true and .generated_files > 0 and .proposal_stage == "generated-untrusted" and .adapter_events > 0 and .offline_adapters_valid == 1 and .network_operations == 0' "$OUT/summary.json" > /dev/null

echo "threat model gate passed: repo $(jq -r '.repo' "$OUT/summary.json"), generated files $(jq '.generated_files' "$OUT/summary.json"), adapter events $(jq '.adapter_events' "$OUT/summary.json")"
