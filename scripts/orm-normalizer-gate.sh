#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/orm-normalizer-gate.json}"
OUT="${2:-results/generated/orm-normalizer}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.orm-normalizer-gate/v1" and (.claim|length) > 200 and (.operations|length) >= 4' "$SPEC" > /dev/null

for phrase in "canonical" "make orm-normalizer-gate"; do
  grep -F "$phrase" docs/orm-normalizer.md README.md > /dev/null
done

bash scripts/orm-normalizer.sh "$SPEC" "$OUT" > "$OUT.run.log"

# All four dialects converge on one non-null canonical tuple; the unknown dialect is rejected.
jq -e '
  .version == "patchline.orm-normalizer/v1" and
  .all_recognized == true and
  .converge == true and
  (.canonical_form.op == "add_column") and
  (.canonical_form.nullable == false) and
  .unknown_rejected == true
' "$OUT/normalized.json" > /dev/null

jq -n --slurpfile r "$OUT/normalized.json" '{
  version: "patchline.orm-normalizer-gate-results/v1",
  canonical_form: $r[0].canonical_form,
  dialects_converge: $r[0].converge,
  unknown_rejected: $r[0].unknown_rejected,
  verified: true
}' > "$OUT/gate-summary.json"

echo "orm-normalizer gate passed: four dialects converge on one canonical IR, unknown dialect rejected"
