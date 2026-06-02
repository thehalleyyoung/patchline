#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/rejection-taxonomy-gate.json}"
OUT="${2:-results/generated/rejection-taxonomy}"
rm -rf "$OUT"
mkdir -p "$OUT/cache"

jq -e '
  .version == "patchline.rejection-taxonomy-gate/v1" and
  (.claim | length) > 100 and
  (.categories | length) == 4
' "$SPEC" > /dev/null

repo="$(jq -r '.real_repo.repo' "$SPEC")"
ref="$(jq -r '.real_repo.ref' "$SPEC")"
subpath="$(jq -r '.real_repo.subpath' "$SPEC")"

go run ./cmd/patchline repo analyze \
  --github "$repo" --ref "$ref" --subpath "$subpath" \
  --download-dir "$OUT/cache" --stages inventory,baseline --no-llm \
  --out "$OUT/analysis" --json > "$OUT/analyze.log"

BASE="$OUT/analysis/baseline/baseline.json"

# Deterministic rejection taxonomy. Each category is a closed set of signals derived from
# real ranking factors and risk kinds. A candidate is REJECTED for a category iff any of its
# signals are present. A candidate with no category signals is ACCEPTED (safe to review).
classify() {
  jq -c '
    (.risks // [])[]
    | { rid:.id, table:.table, kind:.kind,
        factors:([.factors[].name]) }
    | . as $r
    | {
        risk_id: $r.rid,
        table: $r.table,
        kind: $r.kind,
        rejections: (
          [
            (if ($r.factors | any(. == "high-risk-sql" or . == "destructive-effect" or . == "destructive-code-path"))
                or ($r.kind | test("delete|drop_table"))
             then "unsafe-sql" else empty end),
            (if ($r.factors | any(. == "broad-write" or . == "schema-write-breadth" or . == "write-breadth-unknown"))
             then "broad-write" else empty end),
            (if ($r.factors | any(. == "weak-rollback-signal"))
             then "missing-rollback" else empty end),
            (if ($r.factors | any(. == "missing-idempotency" or . == "missing-transaction-boundary"))
             then "unbounded-runtime" else empty end)
          ] | unique
        )
      }
    | . + { rejected: ((.rejections | length) > 0) }
  ' "$BASE"
}

classify > "$OUT/classifications.jsonl"
classify > "$OUT/classifications.rerun.jsonl"

if diff -q "$OUT/classifications.jsonl" "$OUT/classifications.rerun.jsonl" > /dev/null; then
  stable=true
else
  stable=false
fi

# Negative control: a synthetic safe candidate (bounded, reversible, transactional, idempotent)
# carries none of the rejection signals and must yield zero rejection codes.
safe_codes="$(jq -nc '
  { factors: ["linked-project-evidence","persistent-create-path"], kind: "code-path:add_column" } as $r |
  [
    (if ($r.factors | any(. == "high-risk-sql" or . == "destructive-effect" or . == "destructive-code-path")) or ($r.kind | test("delete|drop_table")) then "unsafe-sql" else empty end),
    (if ($r.factors | any(. == "broad-write" or . == "schema-write-breadth" or . == "write-breadth-unknown")) then "broad-write" else empty end),
    (if ($r.factors | any(. == "weak-rollback-signal")) then "missing-rollback" else empty end),
    (if ($r.factors | any(. == "missing-idempotency" or . == "missing-transaction-boundary")) then "unbounded-runtime" else empty end)
  ] | length
')"

jq -s --argjson stable "$stable" --argjson safe_codes "$safe_codes" '
  . as $c |
  {
    version: "patchline.rejection-taxonomy/v1",
    candidates: ($c | length),
    rejected: ($c | map(select(.rejected)) | length),
    accepted: ($c | map(select(.rejected | not)) | length),
    by_category: {
      "unsafe-sql": ($c | map(select(.rejections | index("unsafe-sql"))) | length),
      "broad-write": ($c | map(select(.rejections | index("broad-write"))) | length),
      "missing-rollback": ($c | map(select(.rejections | index("missing-rollback"))) | length),
      "unbounded-runtime": ($c | map(select(.rejections | index("unbounded-runtime"))) | length)
    },
    stable: $stable,
    negative_control_safe_codes: $safe_codes
  } |
  . + {
    every_category_fires: (.by_category | to_entries | all(.value > 0)),
    negative_control_clean: (.negative_control_safe_codes == 0)
  }
' "$OUT/classifications.jsonl" > "$OUT/rejection-taxonomy.json"

{
  echo "# Deterministic rejection taxonomy"
  echo
  jq -r '"Classified `" + (.candidates|tostring) + "` real risk candidates into a closed 4-category rejection taxonomy: `" + (.rejected|tostring) + "` rejected, `" + (.accepted|tostring) + "` accepted."' "$OUT/rejection-taxonomy.json"
  echo
  echo "## Rejections by category"
  jq -r '.by_category | to_entries[] | "- `" + .key + "`: " + (.value|tostring)' "$OUT/rejection-taxonomy.json"
  echo
  echo "## Guarantees"
  jq -r '"- every category fires on real evidence: `" + (.every_category_fires|tostring) + "`\n- classification stable across reruns: `" + (.stable|tostring) + "`\n- negative control (synthetic safe candidate) rejection codes: `" + (.negative_control_safe_codes|tostring) + "`"' "$OUT/rejection-taxonomy.json"
  echo
  echo "Rejections are deterministic and reason-bearing: each comes from a closed set of signals (unsafe SQL, broad writes, missing rollback, unbounded runtime), and a candidate carrying none of those signals is never rejected."
} > "$OUT/rejection-taxonomy.md"
cp "$OUT/rejection-taxonomy.md" "$OUT/README.md"

echo "rejection taxonomy complete: $(jq '.rejected' "$OUT/rejection-taxonomy.json")/$(jq '.candidates' "$OUT/rejection-taxonomy.json") rejected, every_category_fires $(jq '.every_category_fires' "$OUT/rejection-taxonomy.json")"
