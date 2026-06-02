#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/conformance-failure-minimizer-gate.json}"
OUT="${2:-results/generated/conformance-failure-minimizer}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.conformance-failure-minimizer-gate/v1"
  and (.claim | length) > 300
  and (.checkers | sort) == ["go", "python"]
  and .primary_drift == "canonical_sha256"
' "$SPEC" > /dev/null

CORPUS="$(jq -r .corpus "$SPEC")"
test -f "$CORPUS"
for phrase in "conformance failure minimizer" "make conformance-failure-minimizer-gate"; do
  grep -F "$phrase" docs/conformance-failure-minimizer.md README.md > /dev/null
done

VECTOR_DIR="$OUT/corpus-vectors"
mkdir -p "$VECTOR_DIR/vectors/valid" "$VECTOR_DIR/vectors/invalid"
while IFS=$'\t' read -r id positive negative; do
  cp "$(dirname "$CORPUS")/$positive" "$VECTOR_DIR/vectors/valid/$id.plci"
  cp "$(dirname "$CORPUS")/$negative" "$VECTOR_DIR/vectors/invalid/$id.plci"
done < <(jq -r '.cases[] | [.id, .positive, .negative_control] | @tsv' "$CORPUS")

go test ./tools/certinteropdashboard ./internal/certlang ./internal/certconformance > "$OUT/go-test.log"
go run ./tools/certlang/go-checker --spec-dir "$VECTOR_DIR" --root . --json > "$OUT/go.json"
python3 tools/certlang/python/check.py --spec-dir "$VECTOR_DIR" --root . --json > "$OUT/python.json"

jq '(.vectors[] | select(.path == "valid/safe-cli-dispatch.plci") | .canonical_sha256) = "0000000000000000000000000000000000000000000000000000000000000000"' \
  "$OUT/python.json" > "$OUT/python-tampered.json"

if go run ./tools/certinteropdashboard \
  --corpus "$CORPUS" \
  --root . \
  --checker "go=$OUT/go.json" \
  --checker "python=$OUT/python-tampered.json" \
  --out-json "$OUT/dashboard.json" \
  --out-md "$OUT/dashboard.md" \
  --minimize-dir "$OUT/minimized" \
  2> "$OUT/dashboard.err"; then
  echo "tampered checker report did not fail dashboard/minimizer run" >&2
  exit 1
fi

jq -e '
  .version == "patchline.certificate-interop-dashboard/v1"
  and .all_ok == false
  and .drift_totals.canonical_sha256 == 1
' "$OUT/dashboard.json" > /dev/null

jq -e '
  .version == "patchline.conformance-failure-minimizer/v1"
  and .status == "minimized"
  and .all_ok == false
  and .checker == "python"
  and .case_id == "safe-cli-dispatch"
  and .drift_kind == "canonical_sha256"
  and .vector_kind == "positive"
  and .witness_path == "witness.plci"
  and .witness_source == "checker-vector"
  and (.minimized_units == ["checker","case","vector","certificate"])
  and .reference.canonical_sha256 != .observed.canonical_sha256
  and (.witness_sha256 | test("^[0-9a-f]{64}$"))
' "$OUT/minimized/witness.json" > /dev/null
cmp "$VECTOR_DIR/vectors/valid/safe-cli-dispatch.plci" "$OUT/minimized/witness.plci"

go run ./tools/certinteropdashboard \
  --corpus "$CORPUS" \
  --root . \
  --checker "go=$OUT/go.json" \
  --checker "python=$OUT/python.json" \
  --out-json "$OUT/clean-dashboard.json" \
  --out-md "$OUT/clean-dashboard.md" \
  --minimize-dir "$OUT/clean-minimized" \
  > "$OUT/clean-dashboard.stdout"

jq -e '
  .version == "patchline.conformance-failure-minimizer/v1"
  and .status == "no_failure"
  and .all_ok == true
  and (.witness_path // "") == ""
' "$OUT/clean-minimized/witness.json" > /dev/null
test ! -f "$OUT/clean-minimized/witness.plci"

jq -n \
  --arg spec "$SPEC" \
  --arg corpus "$CORPUS" \
  --slurpfile witness "$OUT/minimized/witness.json" \
  --slurpfile clean "$OUT/clean-minimized/witness.json" '
  {
    version: "patchline.conformance-failure-minimizer-gate-results/v1",
    spec: $spec,
    corpus: $corpus,
    checker: $witness[0].checker,
    case_id: $witness[0].case_id,
    drift_kind: $witness[0].drift_kind,
    minimized_units: $witness[0].minimized_units,
    witness_sha256: $witness[0].witness_sha256,
    clean_status: $clean[0].status,
    verified: (
      $witness[0].status == "minimized"
      and $witness[0].case_id == "safe-cli-dispatch"
      and $witness[0].drift_kind == "canonical_sha256"
      and $clean[0].status == "no_failure"
    )
  }
' > "$OUT/gate-summary.json"

echo "conformance-failure-minimizer gate passed: tampered cross-checker drift reduced to one certificate witness"
