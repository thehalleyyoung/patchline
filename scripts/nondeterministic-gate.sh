#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

GATES="${1:-examples/nondeterministic-gates.json}"
OUT="${2:-results/generated/nondeterministic-gate}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.nondeterministic-gates/v1" and
  (.gates | length) >= 4 and
  all(.gates[];
    (.id | length) > 0 and
    (.real_repo | length) > 0 and
    (.subpath | length) > 0 and
    (.llm_command | length) > 0 and
    (.budget | test("files=[0-9]+,lines=[0-9]+,tokens=[0-9]+,changes=[0-9]+")) and
    (.audit_claim | length) > 40
  )
' "$GATES" > /dev/null

jq -n --slurpfile gates "$GATES" --slurpfile slices examples/real-repo-slices.json '
  ($slices[0].slices | map(.repo + ":" + .subpath)) as $known |
  $gates[0].gates
  | all(.[]; (.real_repo + ":" + .subpath) as $key | ($known | index($key)))
' | jq -e '. == true' > /dev/null

rows=()
while IFS=$'\t' read -r id repo subpath llm_command budget audit_claim; do
  case_out="$OUT/$id"
  mkdir -p "$case_out"
  ref="$(jq -r --arg repo "$repo" --arg subpath "$subpath" '.slices[] | select(.repo == $repo and .subpath == $subpath) | .ref' examples/real-repo-slices.json)"
  test -n "$ref"

  go run ./cmd/patchline repo fetch "$repo" --ref "$ref" --subpath "$subpath" --out "$case_out/fetch" --json > "$case_out/fetch.json"
  scan_root="$(jq -r '.source.scanned_root' "$case_out/fetch.json")"
  go run ./cmd/patchline repo inventory "$scan_root" --out "$case_out/inventory" --json > "$case_out/inventory.json"
  go run ./cmd/patchline intake "$scan_root" --out "$case_out/intake" --json > "$case_out/intake.json"
  go run ./cmd/patchline repo baseline --inventory "$case_out/inventory" --intake "$case_out/intake" --out "$case_out/baseline" --json > "$case_out/baseline.json"

  go run ./cmd/patchline repo propose --from-report "$case_out/baseline" --proposal-kind tests --budget "$budget" --no-llm --out "$case_out/no-llm-proposal" --json > "$case_out/no-llm-proposal.json"
  jq -e '.deterministic_only == true and .generator == "patchline-template" and (.generated_files | length) > 0' "$case_out/no-llm-proposal.json" > /dev/null

  go run ./cmd/patchline repo propose --from-report "$case_out/baseline" --proposal-kind tests --budget "$budget" --llm-command "$llm_command" --out "$case_out/llm-proposal" --json > "$case_out/llm-proposal.json"
  go run ./cmd/patchline repo compare --before "$case_out/baseline" --after "$case_out/llm-proposal" --out "$case_out/llm-compare" --json > "$case_out/llm-compare.json"
  jq -e --arg budget "$budget" '
    .deterministic_only == false and
    .generator == "llm-command" and
    .trust == "untrusted-generated-proposal" and
    .scope_budget.raw == $budget and
    (.prompt_hash | length) > 0 and
    (.output_hash | length) > 0 and
    (.generated_files | length) <= .scope_budget.files
  ' "$case_out/llm-proposal.json" > /dev/null
  jq -e '.summary.intervention_loops > 0 and (.intervention_loop.rationale | contains("not as trusted completion output"))' "$case_out/llm-compare.json" > /dev/null

  jq -n \
    --arg id "$id" \
    --arg repo "$repo" \
    --arg subpath "$subpath" \
    --arg budget "$budget" \
    --arg audit_claim "$audit_claim" \
    --slurpfile no_llm "$case_out/no-llm-proposal.json" \
    --slurpfile llm "$case_out/llm-proposal.json" \
    --slurpfile compare "$case_out/llm-compare.json" \
    '{
      id: $id,
      repo: $repo,
      subpath: $subpath,
      budget: $budget,
      audit_claim: $audit_claim,
      optional_no_llm_generator: $no_llm[0].generator,
      llm_generator: $llm[0].generator,
      prompt_hash: $llm[0].prompt_hash,
      output_hash: $llm[0].output_hash,
      generated_files: ($llm[0].generated_files | length),
      deterministic_intervention_loops: $compare[0].summary.intervention_loops,
      verified: true
    }' > "$case_out/row.json"
  rows+=("$case_out/row.json")
done < <(jq -r '.gates[] | [.id, .real_repo, .subpath, .llm_command, .budget, .audit_claim] | @tsv' "$GATES")

jq -s '{version:"patchline.nondeterministic-gate-results/v1", gates: .}' "${rows[@]}" > "$OUT/summary.json"
jq -e '(.gates | length) >= 4 and all(.gates[]; .verified == true and .optional_no_llm_generator == "patchline-template" and .llm_generator == "llm-command" and .deterministic_intervention_loops > 0)' "$OUT/summary.json" > /dev/null
echo "nondeterministic gate passed: $(jq '.gates | length' "$OUT/summary.json") public repo slices proved optional, bounded, auditable generation with deterministic follow-up"
