#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/resumable-gates-gate.json}"
OUT="${2:-results/generated/resumable-gates}"
rm -rf "$OUT"
mkdir -p "$OUT/state"

jq -e '.version == "patchline.resumable-gates-gate/v1" and (.repos|length) >= 3' "$SPEC" > /dev/null

interrupt_after="$(jq -r '.interrupt_after' "$SPEC")"
repos=()
while IFS= read -r line; do repos+=("$line"); done < <(jq -r '.repos[]' "$SPEC")
total="${#repos[@]}"

# A single repo "analysis" persists a completion marker. Resume logic skips any
# repo whose marker already exists.
process_repo() {
  local name="$1"
  if [ -f "$OUT/state/${name}.done" ]; then
    echo "skip"
    return 0
  fi
  echo "$name" >> "$OUT/processed.log"
  echo "done" > "$OUT/state/${name}.done"
  echo "process"
}

# Pass 1: interrupted after `interrupt_after` repositories.
run1_processed=0
idx=0
for r in "${repos[@]}"; do
  if [ "$idx" -ge "$interrupt_after" ]; then break; fi
  res="$(process_repo "$r")"
  [ "$res" = "process" ] && run1_processed=$((run1_processed+1))
  idx=$((idx+1))
done

# Snapshot which repos were completed by the interrupted run.
ls "$OUT/state" | sed 's/\.done$//' | sort > "$OUT/.completed_after_interrupt"
completed_after_interrupt="$(wc -l < "$OUT/.completed_after_interrupt" | tr -d ' ')"

# Pass 2: resume. Iterate the whole corpus; preserved repos must be skipped.
run2_processed=0
run2_skipped=0
for r in "${repos[@]}"; do
  res="$(process_repo "$r")"
  if [ "$res" = "process" ]; then run2_processed=$((run2_processed+1)); else run2_skipped=$((run2_skipped+1)); fi
done

# Each repo must appear in the processed log exactly once across both passes.
processed_unique="$(sort -u "$OUT/processed.log" | wc -l | tr -d ' ')"
processed_total="$(wc -l < "$OUT/processed.log" | tr -d ' ')"
no_recompute=false
[ "$processed_unique" = "$processed_total" ] && [ "$processed_total" = "$total" ] && no_recompute=true

jq -n \
  --argjson total "$total" \
  --argjson interrupt_after "$interrupt_after" \
  --argjson completed_after_interrupt "$completed_after_interrupt" \
  --argjson run2_processed "$run2_processed" \
  --argjson run2_skipped "$run2_skipped" \
  --argjson no_recompute "$no_recompute" '
  {
    version: "patchline.resumable-gates/v1",
    total: $total,
    completed_after_interrupt: $completed_after_interrupt,
    resume_processed: $run2_processed,
    resume_skipped: $run2_skipped,
    each_repo_processed_once: $no_recompute
  }' > "$OUT/resumable-gates.json"

{
  echo "# Resumable gates"
  echo
  echo "- Corpus size: $total"
  echo "- Completed before interruption: $completed_after_interrupt"
  echo "- Resume processed: $run2_processed, skipped (preserved): $run2_skipped"
} > "$OUT/resumable-gates.md"
cp "$OUT/resumable-gates.md" "$OUT/README.md"

echo "resumable-gates worker: interrupted at $completed_after_interrupt/$total, resume processed $run2_processed, skipped $run2_skipped"
