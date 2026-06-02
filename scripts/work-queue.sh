#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/work-queue-gate.json}"; OUT="${2:-results/generated/work-queue}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.work-queue-gate/v1" and (.tasks|length) >= 1' "$SPEC" > /dev/null
jq '
  def wk($id; $n): ([ $id | explode[] ] | add) % $n;
  def assign($tasks; $n): reduce $tasks[] as $t ({}; .["w\(wk($t; $n))"] = ((.["w\(wk($t; $n))"] // []) + [$t]));
  def all_queued($a): [ $a[][] ] | sort;
  def has_overlap($a): ([ $a[][] ] | length) != ([ $a[][] ] | unique | length);
  .tasks as $T | .workers as $n
  | (assign($T; $n)) as $a1 | (assign($T; $n)) as $a2
  | {
      version: "patchline.work-queue/v1",
      workers: $n,
      assignment: $a1,
      deterministic: ($a1 == $a2),
      complete: (all_queued($a1) == ($T|sort)),
      disjoint: (has_overlap($a1) | not),
      corrupt_has_overlap: has_overlap(.corrupt_assignment)
    }
' "$SPEC" > "$OUT/queue.json"
{ echo "# Distributed work queue"; echo; echo "Deterministic: $(jq -r '.deterministic' "$OUT/queue.json"); complete: $(jq -r '.complete' "$OUT/queue.json"); disjoint: $(jq -r '.disjoint' "$OUT/queue.json")"; } > "$OUT/queue.md"
cp "$OUT/queue.md" "$OUT/README.md"
echo "work-queue worker: det=$(jq -r '.deterministic' "$OUT/queue.json") complete=$(jq -r '.complete' "$OUT/queue.json") disjoint=$(jq -r '.disjoint' "$OUT/queue.json")"
