#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/research-experiment-driver.json}"
OUT="${2:-results/generated/research-experiment-driver-gate}"
rm -rf "$OUT"
mkdir -p "$OUT"

bash scripts/research-experiment-driver.sh "$SPEC" "$OUT/run" > "$OUT/driver.log"

jq -e '
  .version == "patchline.research-experiment-ledger/v1" and
  (.commit | test("^[0-9a-f]{40}$")) and
  .clean_checkout == true and
  .immutable == true and
  (.output_hash | startswith("sha256:")) and
  .research_question_summary.rq_coverage.fact_extraction == true and
  .research_question_summary.rq_coverage.risk_ranking == true and
  .research_question_summary.rq_coverage.evidence_linking == true and
  .research_question_summary.rq_coverage.generated_safety == true and
  .research_question_summary.rq_coverage.before_after == true
' "$OUT/run/experiment-ledger.json" > /dev/null
test -s "$OUT/run/result-checksums.sha256"

echo "research experiment driver gate passed: clean checkout ledger $(jq -r '.output_hash' "$OUT/run/experiment-ledger.json")"
