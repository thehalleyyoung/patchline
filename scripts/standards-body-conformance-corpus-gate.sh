#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/standards-body-conformance-corpus-gate.json}"
OUT="${2:-results/generated/standards-body-conformance-corpus}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.standards-body-conformance-corpus-gate/v1"
  and (.claim | length) > 300
  and .minimum_cases >= 4
  and ((.required_verdicts | sort) == ["blocked", "guarded", "safe", "unsupported"])
' "$SPEC" > /dev/null

CORPUS="$(jq -r .corpus "$SPEC")"
jq -e '
  .version == "patchline.certificate-conformance-corpus/v1"
  and .standard == "PLCI/1"
  and (.cases | length) >= 4
' "$CORPUS" > /dev/null

for phrase in "standards-body conformance corpus" "make standards-body-conformance-corpus-gate"; do
  grep -F "$phrase" docs/standards-body-conformance-corpus.md README.md > /dev/null
done

go test ./internal/certconformance > "$OUT/go-test.log"
go run ./tools/certconformance/check --corpus "$CORPUS" --root . --json > "$OUT/report.json"

jq -e '
  .version == "patchline.certificate-conformance-corpus-results/v1"
  and .all_ok == true
  and .total_cases >= 4
  and .positives_accepted == .total_cases
  and .negatives_rejected == .total_cases
  and .references_verified == .total_cases
  and (["safe", "guarded", "blocked", "unsupported"] - [.cases[].verdict] | length) == 0
  and all(.cases[]; .positive_accepted and .negative_rejected and .reference_verified and .ok)
' "$OUT/report.json" > /dev/null

TAMPER="$OUT/tampered-corpus"
mkdir -p "$TAMPER"
cp -R "$(dirname "$CORPUS")/." "$TAMPER/"
jq '.signature.value = ("0" + (.signature.value[1:]))' \
  "$TAMPER/cases/safe-cli-dispatch/reference-output.json" > "$TAMPER/reference-output.tmp"
mv "$TAMPER/reference-output.tmp" "$TAMPER/cases/safe-cli-dispatch/reference-output.json"
if go run ./tools/certconformance/check --corpus "$TAMPER/corpus.json" --root . --json > "$OUT/tamper-report.json" 2> "$OUT/tamper.err"; then
  echo "tampered signed reference output was accepted" >&2
  exit 1
fi
grep -F "signature" "$OUT/tamper.err" > /dev/null

jq -n --slurpfile report "$OUT/report.json" --rawfile tamper "$OUT/tamper.err" '{
  version: "patchline.standards-body-conformance-corpus-gate-results/v1",
  corpus: $report[0].corpus,
  total_cases: $report[0].total_cases,
  positives_accepted: $report[0].positives_accepted,
  negatives_rejected: $report[0].negatives_rejected,
  references_verified: $report[0].references_verified,
  tamper_rejected: ($tamper | contains("signature")),
  verified: ($report[0].all_ok and ($tamper | contains("signature")))
}' > "$OUT/gate-summary.json"

echo "standards-body-conformance-corpus gate passed: positive proofs, negative controls, signed references, and tamper rejection verified"
