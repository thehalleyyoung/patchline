#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/fn-discovery-gate.json}"
OUT="${2:-results/generated/fn-discovery}"
rm -rf "$OUT"
mkdir -p "$OUT/corpus/db/migrate" "$OUT/cache"

jq -e '
  .version == "patchline.fn-discovery-gate/v1" and
  (.claim | length) > 100 and
  (.hazards | length) >= .minimum_hazards and
  all(.hazards[]; (.file | length) > 0 and (.sql | length) > 0 and (.expected_specific_kind | length) > 0)
' "$SPEC" > /dev/null

# Materialize the seeded-hazard corpus (real SQL migration files) plus the benign control.
hazard_count="$(jq '.hazards | length' "$SPEC")"
for ((h=0; h<hazard_count; h++)); do
  file="$(jq -r ".hazards[$h].file" "$SPEC")"
  jq -r ".hazards[$h].sql" "$SPEC" > "$OUT/corpus/db/migrate/$file"
done
bfile="$(jq -r '.benign_control.file' "$SPEC")"
jq -r '.benign_control.sql' "$SPEC" > "$OUT/corpus/db/migrate/$bfile"

# Run Patchline's static baseline over the seeded corpus.
go run ./cmd/patchline repo analyze "$OUT/corpus" \
  --stages inventory,baseline --no-llm --out "$OUT/seed-analysis" --json > "$OUT/seed-analyze.log"

seed_baseline="$OUT/seed-analysis/baseline/baseline.json"

# Classify each hazard: specific (matched expected kind), generic-only (only an alter/
# untyped risk on the file), or missed (no risk on the file at all).
recall_jsonl="$OUT/hazard-recall.jsonl"
: > "$recall_jsonl"
for ((h=0; h<hazard_count; h++)); do
  id="$(jq -r ".hazards[$h].id" "$SPEC")"
  file="$(jq -r ".hazards[$h].file" "$SPEC")"
  pat="$(jq -r ".hazards[$h].expected_specific_kind" "$SPEC")"
  analogue="$(jq -r ".hazards[$h].incident_analogue" "$SPEC")"
  jq -c --arg id "$id" --arg file "$file" --arg pat "$pat" --arg analogue "$analogue" '
    [ .risks[] | select(.path | endswith($file)) ] as $rs |
    ($rs | map(select(.kind | test($pat))) | length) as $specific |
    ($rs | length) as $total |
    ($rs | map(.severity) | (index("high") != null)) as $any_high |
    {
      hazard: $id,
      incident_analogue: $analogue,
      risks_on_file: $total,
      specific_matches: $specific,
      escalated_high: $any_high,
      detection: (if $specific > 0 then "specific" elif $total > 0 then "generic-only" else "missed" end)
    }
  ' "$seed_baseline" >> "$recall_jsonl"
done

# Benign control must not be escalated to high severity.
benign_file="$(jq -r '.benign_control.file' "$SPEC")"
benign_high="$(jq --arg f "$benign_file" '[.risks[] | select((.path | endswith($f)) and .severity == "high")] | length' "$seed_baseline")"

# Real-code cross-validation: the same hazard detectors must fire on a real public repo.
xv_repo="$(jq -r '.real_cross_validation.repo' "$SPEC")"
xv_ref="$(jq -r '.real_cross_validation.ref' "$SPEC")"
xv_sub="$(jq -r '.real_cross_validation.subpath' "$SPEC")"
xv_pat="$(jq -r '.real_cross_validation.expected_kind_pattern' "$SPEC")"
go run ./cmd/patchline repo analyze \
  --github "$xv_repo" --ref "$xv_ref" --subpath "$xv_sub" \
  --download-dir "$OUT/cache" --stages inventory,baseline --no-llm \
  --out "$OUT/real-analysis" --json > "$OUT/real-analyze.log"
xv_matches="$(jq --arg pat "$xv_pat" '[.risks[] | select(.kind | test($pat))] | length' "$OUT/real-analysis/baseline/baseline.json")"

# Aggregate.
jq -s --argjson benign_high "$benign_high" --argjson xv_matches "$xv_matches" \
  --arg xv_repo "$xv_repo" '
  . as $rows | ($rows | length) as $n |
  ($rows | map(select(.detection == "specific")) | length) as $specific |
  ($rows | map(select(.detection == "generic-only")) | length) as $generic |
  ($rows | map(select(.detection == "missed")) | length) as $missed |
  {
    version: "patchline.fn-discovery/v1",
    hazards: $n,
    specific_detections: $specific,
    generic_only_detections: $generic,
    missed_detections: $missed,
    specific_recall: (($specific / $n * 1000 | round) / 1000),
    any_detection_recall: ((($specific + $generic) / $n * 1000 | round) / 1000),
    false_negatives: ($rows | map(select(.detection != "specific")) | map({hazard, incident_analogue, detection})),
    benign_control_high_severity_risks: $benign_high,
    real_cross_validation: { repo: $xv_repo, matching_kind_risks: $xv_matches }
  }
' "$recall_jsonl" > "$OUT/fn-discovery.json"

{
  echo "# False-negative discovery (seeded hazards + real cross-validation)"
  echo
  echo "Known public-incident hazard analogues are seeded as real migration files and run through Patchline's static baseline. Recall is measured so under-escalated and missed hazards are surfaced explicitly."
  echo
  echo "## Recall"
  jq -r '"- hazards seeded: `" + (.hazards|tostring) + "`\n- specific detections: `" + (.specific_detections|tostring) + "` (recall `" + (.specific_recall|tostring) + "`)\n- generic-only (under-escalated): `" + (.generic_only_detections|tostring) + "`\n- missed (no risk emitted): `" + (.missed_detections|tostring) + "`\n- any-detection recall: `" + (.any_detection_recall|tostring) + "`"' "$OUT/fn-discovery.json"
  echo
  echo "## Surfaced false negatives (discovery output)"
  echo
  echo "| Hazard | Incident analogue | Detection |"
  echo "| --- | --- | --- |"
  jq -r '.false_negatives[] | "| " + .hazard + " | " + .incident_analogue + " | " + .detection + " |"' "$OUT/fn-discovery.json"
  echo
  echo "## Controls"
  jq -r '"- benign control high-severity risks: `" + (.benign_control_high_severity_risks|tostring) + "` (expected 0)\n- real cross-validation (`" + (.real_cross_validation.repo) + "`) matching-kind risks: `" + (.real_cross_validation.matching_kind_risks|tostring) + "`"' "$OUT/fn-discovery.json"
} > "$OUT/fn-discovery.md"

cp "$OUT/fn-discovery.md" "$OUT/README.md"
echo "fn discovery complete: hazards $(jq '.hazards' "$OUT/fn-discovery.json"), specific $(jq '.specific_detections' "$OUT/fn-discovery.json"), missed/under $(jq '.generic_only_detections + .missed_detections' "$OUT/fn-discovery.json"), real xv matches $(jq '.real_cross_validation.matching_kind_risks' "$OUT/fn-discovery.json")"
