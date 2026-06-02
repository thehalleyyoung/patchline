#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/transaction-boundary-gate.json}"
OUT="${2:-results/generated/transaction-boundary}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '.version == "patchline.transaction-boundary-gate/v1"' "$SPEC" > /dev/null

jq '
  # A step is atomic if it is inside a transaction or carries an explicit compensation.
  def analyze($plan):
    ([ $plan[] | select((.txn == null) and (.compensation == null)) | .id ]) as $unguarded
    | {atomic: (($unguarded | length) == 0), unguarded_steps: $unguarded};
  {
    version: "patchline.transaction-boundary/v1",
    safe_result: analyze(.safe_plan),
    unsafe_result: analyze(.unsafe_plan)
  }
' "$SPEC" > "$OUT/txn.json"

{
  echo "# Transaction-boundary analyzer"
  echo
  echo "Safe plan atomic: $(jq -r '.safe_result.atomic' "$OUT/txn.json")"
  echo "Unsafe plan unguarded steps: $(jq -rc '.unsafe_result.unguarded_steps' "$OUT/txn.json")"
} > "$OUT/txn.md"
cp "$OUT/txn.md" "$OUT/README.md"

echo "transaction-boundary worker: safe_atomic=$(jq -r '.safe_result.atomic' "$OUT/txn.json") unguarded=$(jq -rc '.unsafe_result.unguarded_steps' "$OUT/txn.json")"
