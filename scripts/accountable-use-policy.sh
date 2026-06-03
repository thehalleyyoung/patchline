#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/accountable-use-policy-gate.json}"
OUT="${2:-results/generated/accountable-use-policy}"

rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '.version == "patchline.accountable-use-policy-gate/v1"' "$SPEC" > /dev/null

jq '
  def oversight_mapped:
    ((.kind == "autonomous" or .kind == "blocking") | not)
    or (((.human_role // "") | length) > 0
      and ((.decision_point // "") | length) > 0
      and ((.review_artifact // "") | length) > 0
      and ((.evidence // "") | length) > 0);

  .capabilities as $C
  | [ $C[] | select(.kind == "autonomous" or .kind == "blocking") ] as $required
  | [ $required[] | select(oversight_mapped) ] as $mapped
  | {version:"patchline.accountable-use-policy/v1",
     total:($C | length),
     oversight_required:($required | length),
     oversight_mapped:($mapped | length),
     all_required_mapped:(($mapped | length) == ($required | length)),
     bad_mapped:(.bad | oversight_mapped),
     mappings: [ $C[] | {
       id,
       kind,
       requires_oversight:(.kind == "autonomous" or .kind == "blocking"),
       human_role,
       decision_point,
       review_artifact,
       evidence,
       mapped: oversight_mapped
     } ]}
' "$SPEC" > "$OUT/out.json"

{
  echo "# Accountable-use policy"
  echo
  echo "Mapped $(jq -r .oversight_mapped "$OUT/out.json")/$(jq -r .oversight_required "$OUT/out.json") autonomous or blocking capabilities to human oversight."
  echo
  jq -r '.mappings[] | "- \(.id): \(.kind), oversight=\(.mapped), role=\(.human_role // "n/a"), evidence=\(.evidence)"' "$OUT/out.json"
} > "$OUT/out.md"

cp "$OUT/out.md" "$OUT/README.md"

echo "accountable-use-policy worker: all_required_mapped=$(jq -r .all_required_mapped "$OUT/out.json")"
