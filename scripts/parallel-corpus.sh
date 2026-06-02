#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/parallel-corpus-gate.json}"
OUT="${2:-results/generated/parallel-corpus}"
rm -rf "$OUT"
mkdir -p "$OUT/tasks"

jq -e '.version == "patchline.parallel-corpus-gate/v1" and (.repos|length) >= 2' "$SPEC" > /dev/null

n="$(jq '.repos | length' "$SPEC")"

# Schedule every repo task concurrently. Each writes its own result file, so a
# failure in one task cannot corrupt or abort another (failure isolation).
completion_log="$OUT/completion-order.txt"
: > "$completion_log"
for i in $(seq 0 $((n-1))); do
  name="$(jq -r ".repos[$i].name" "$SPEC")"
  delay="$(jq -r ".repos[$i].delay" "$SPEC")"
  should_fail="$(jq -r ".repos[$i].should_fail" "$SPEC")"
  (
    sleep "$delay"
    if [ "$should_fail" = "true" ]; then
      status="failed"
    else
      status="ok"
    fi
    jq -n --arg name "$name" --arg status "$status" -c '{name:$name, status:$status}' \
      > "$OUT/tasks/${name}.json"
    # Record real completion order (will differ from name order due to delays).
    echo "$name" >> "$completion_log"
  ) &
done
wait

# Collate deterministically by repository identity, never by completion order.
collate() {
  cat "$OUT"/tasks/*.json | jq -s 'sort_by(.name)'
}
collate > "$OUT/collated.json"
# Run the collation a second time to prove byte-identical ordering.
collate > "$OUT/collated2.json"

ordering_stable=false
if cmp -s "$OUT/collated.json" "$OUT/collated2.json"; then ordering_stable=true; fi

# Completion order must differ from the sorted name order (proves real concurrency
# with out-of-order completion, so deterministic ordering is non-trivial).
sorted_names="$(jq -r '.[].name' "$OUT/collated.json" | tr '\n' ',')"
completion_names="$(tr '\n' ',' < "$completion_log")"
order_differs=false
[ "$sorted_names" != "$completion_names" ] && order_differs=true

ok_count="$(jq '[.[] | select(.status=="ok")] | length' "$OUT/collated.json")"
failed_count="$(jq '[.[] | select(.status=="failed")] | length' "$OUT/collated.json")"

jq -n \
  --argjson total "$n" \
  --argjson ok "$ok_count" \
  --argjson failed "$failed_count" \
  --argjson ordering_stable "$ordering_stable" \
  --argjson order_differs "$order_differs" \
  --slurpfile collated "$OUT/collated.json" '
  {
    version: "patchline.parallel-corpus/v1",
    total: $total,
    ok: $ok,
    failed: $failed,
    deterministic_order: $ordering_stable,
    completion_order_differed: $order_differs,
    collated: $collated[0]
  }' > "$OUT/parallel-corpus.json"

{
  echo "# Parallel public-corpus execution"
  echo
  echo "Tasks run concurrently; results are collated by repository identity."
  echo
  echo "| Repo | Status |"
  echo "|---|---|"
  jq -r '.collated[] | "| \(.name) | \(.status) |"' "$OUT/parallel-corpus.json"
} > "$OUT/parallel-corpus.md"
cp "$OUT/parallel-corpus.md" "$OUT/README.md"

echo "parallel-corpus worker: $ok_count ok, $failed_count failed, deterministic_order=$ordering_stable"
