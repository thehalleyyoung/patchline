#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

if ! command -v jq >/dev/null 2>&1; then
  echo "artifact-negative-cases requires jq" >&2
  exit 1
fi

OUT="results/generated/artifact-negative-cases"
rm -rf "$OUT"
mkdir -p "$OUT"

bash scripts/validate-ground-truth.sh >/dev/null

jq -s '{
  negative_cases: map({
    case_id,
    case_type,
    phase,
    expected_result: .labels.expected_result,
    risk: .labels.risk,
    evidence_count: (.evidence | length)
  })
}' benchmarks/ground_truth/negative/*.json > "$OUT/negative-cases.json"

jq -e '
  (.negative_cases | length) >= 5 and
  ([.negative_cases[].expected_result] | index("unsupported_fragment") != null) and
  ([.negative_cases[].expected_result] | index("insufficient_evidence") != null) and
  ([.negative_cases[].expected_result] | index("cannot_prove") != null) and
  ([.negative_cases[].expected_result] | index("pass") != null)
' "$OUT/negative-cases.json" >/dev/null

echo "negative_cases_output=$OUT/negative-cases.json"
