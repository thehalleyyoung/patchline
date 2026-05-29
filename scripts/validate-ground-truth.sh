#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

if ! command -v jq >/dev/null 2>&1; then
  echo "validate-ground-truth requires jq" >&2
  exit 1
fi

required='
  .case_id and
  .case_type and
  .phase and
  .labels.expected_result and
  .labels.risk and
  (.evidence | type == "array" and length > 0) and
  (.allowed_inputs | type == "array") and
  (.excluded_inputs | type == "array")
'

evidence_required='
  all(.evidence[]; .kind and .locator and .rationale)
'

phase_safe='
  if .phase == "pre_deploy" then
    ([.evidence[] | select(.kind == "postmortem")] | length) == 0 and
    (.excluded_inputs | index("postmortem_text")) != null
  else
    true
  end
'

count=0
while IFS= read -r file; do
  jq -e "$required" "$file" >/dev/null
  jq -e "$evidence_required" "$file" >/dev/null
  jq -e "$phase_safe" "$file" >/dev/null
  count=$((count + 1))
done < <(find benchmarks/ground_truth -name '*.json' | sort)

manifest_count=0
while IFS= read -r manifest; do
  jq -e '.version and .dataset_id and (.cases | type == "array" and length > 0)' "$manifest" >/dev/null
  while IFS=$'\t' read -r case_id gt; do
    gt_path="$(cd "$(dirname "$manifest")" && cd "$(dirname "$gt")" && pwd)/$(basename "$gt")"
    if [[ ! -f "$gt_path" ]]; then
      echo "missing ground truth: manifest=$manifest case=$case_id path=$gt" >&2
      exit 1
    fi
    actual="$(jq -r '.case_id' "$gt_path")"
    if [[ "$actual" != "$case_id" ]]; then
      echo "case_id mismatch: manifest=$manifest case=$case_id ground_truth=$actual" >&2
      exit 1
    fi
  done < <(jq -r '.cases[] | [.case_id, .ground_truth] | @tsv' "$manifest")
  manifest_count=$((manifest_count + 1))
done < <(find benchmarks/manifests -name '*.json' | sort)

echo "validated ground_truth_files=$count manifests=$manifest_count"
