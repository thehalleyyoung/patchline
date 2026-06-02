#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/certificate-interop-dashboard-gate.json}"
OUT="${2:-results/generated/certificate-interop-dashboard}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.certificate-interop-dashboard-gate/v1"
  and (.claim | length) > 300
  and (.checkers | sort) == ["go", "python", "rust", "typescript"]
  and .minimum_cases >= 4
' "$SPEC" > /dev/null

CORPUS="$(jq -r .corpus "$SPEC")"
test -f "$CORPUS"
CASE_COUNT="$(jq '.cases | length' "$CORPUS")"
VECTOR_DIR="$OUT/corpus-vectors"
mkdir -p "$VECTOR_DIR/vectors/valid" "$VECTOR_DIR/vectors/invalid"

while IFS=$'\t' read -r id positive negative; do
  cp "$(dirname "$CORPUS")/$positive" "$VECTOR_DIR/vectors/valid/$id.plci"
  cp "$(dirname "$CORPUS")/$negative" "$VECTOR_DIR/vectors/invalid/$id.plci"
done < <(jq -r '.cases[] | [.id, .positive, .negative_control] | @tsv' "$CORPUS")

for phrase in "certificate interop dashboard" "make certificate-interop-dashboard-gate"; do
  grep -F "$phrase" docs/certificate-interop-dashboard.md README.md > /dev/null
done

go test ./internal/certlang ./internal/certconformance ./tools/certinteropdashboard > "$OUT/go-test.log"
go run ./tools/certconformance/check --corpus "$CORPUS" --root . --json > "$OUT/conformance.json"
go run ./tools/certlang/go-checker --spec-dir "$VECTOR_DIR" --root . --json > "$OUT/go.json"
rustc tools/certlang/rust/check.rs -o "$OUT/rust-checker"
"$OUT/rust-checker" --spec-dir "$VECTOR_DIR" --root . --json > "$OUT/rust.json"
python3 tools/certlang/python/check.py --spec-dir "$VECTOR_DIR" --root . --json > "$OUT/python.json"
node --no-warnings --experimental-strip-types tools/certlang/typescript/check.ts --spec-dir "$VECTOR_DIR" --root . --json > "$OUT/typescript.json"

for report in "$OUT/go.json" "$OUT/rust.json" "$OUT/python.json" "$OUT/typescript.json"; do
  jq -e --argjson cases "$CASE_COUNT" '
    .version == "PLCI/1"
    and .all_ok == true
    and .total_valid == $cases
    and .total_invalid == $cases
    and all(.vectors[] | select(.expected == "valid");
      .accepted == true
      and (.certificate_id | type == "string")
      and (.verdict | type == "string")
      and (.risk_bps | type == "number")
      and (.canonical_sha256 | test("^[0-9a-f]{64}$")))
    and all(.vectors[] | select(.expected == "invalid");
      .accepted == false
      and (.error | type == "string"))
  ' "$report" > /dev/null
done

go run ./tools/certinteropdashboard \
  --corpus "$CORPUS" \
  --root . \
  --checker "go=$OUT/go.json" \
  --checker "rust=$OUT/rust.json" \
  --checker "python=$OUT/python.json" \
  --checker "typescript=$OUT/typescript.json" \
  --out-json "$OUT/dashboard.json" \
  --out-md "$OUT/dashboard.md"

jq -e --argjson cases "$CASE_COUNT" '
  .version == "patchline.certificate-interop-dashboard/v1"
  and .all_ok == true
  and .total_cases == $cases
  and .total_checkers == 4
  and .signed_references_verified == $cases
  and all(.drift_totals | to_entries[]; .value == 0)
  and all(.checkers[]; .all_ok == true and .cases_ok == $cases)
' "$OUT/dashboard.json" > /dev/null

jq '(.vectors[] | select(.path == "valid/safe-cli-dispatch.plci") | .canonical_sha256) = "0000000000000000000000000000000000000000000000000000000000000000"' \
  "$OUT/python.json" > "$OUT/python-tampered.json"

if go run ./tools/certinteropdashboard \
  --corpus "$CORPUS" \
  --root . \
  --checker "go=$OUT/go.json" \
  --checker "rust=$OUT/rust.json" \
  --checker "python=$OUT/python-tampered.json" \
  --checker "typescript=$OUT/typescript.json" \
  --out-json "$OUT/tampered-dashboard.json" \
  --out-md "$OUT/tampered-dashboard.md" \
  2> "$OUT/tampered.err"; then
  echo "tampered checker report did not produce dashboard drift" >&2
  exit 1
fi

jq -e '
  .all_ok == false
  and .drift_totals.canonical_sha256 >= 1
  and any(.cases[].checkers[]; (.drift // []) | index("canonical_sha256"))
' "$OUT/tampered-dashboard.json" > /dev/null

jq -n \
  --arg spec "$SPEC" \
  --arg corpus "$CORPUS" \
  --slurpfile dashboard "$OUT/dashboard.json" \
  --slurpfile tampered "$OUT/tampered-dashboard.json" '
  {
    version: "patchline.certificate-interop-dashboard-gate-results/v1",
    spec: $spec,
    corpus: $corpus,
    checkers: ($dashboard[0].checkers | map(.name)),
    total_cases: $dashboard[0].total_cases,
    drift_totals: $dashboard[0].drift_totals,
    tamper_canonical_drift: $tampered[0].drift_totals.canonical_sha256,
    dashboard: "dashboard.md",
    verified: ($dashboard[0].all_ok == true and $tampered[0].all_ok == false and $tampered[0].drift_totals.canonical_sha256 >= 1)
  }
' > "$OUT/gate-summary.json"

echo "certificate-interop-dashboard gate passed: frozen corpus rerun across Go, Rust, Python, and TypeScript with signed-reference drift deltas"
