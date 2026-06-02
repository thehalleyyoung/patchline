#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/a11y-i18n-output-gate.json}"; OUT="${2:-results/generated/a11y-i18n-output}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.a11y-i18n-output-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "accessibility" "make a11y-i18n-output-gate"; do grep -F "$phrase" docs/a11y-i18n-output.md README.md > /dev/null; done
bash scripts/a11y-i18n-output.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.a11y-i18n-output/v1" and .all_accessible==true and .all_localizable==true and .bad_accessible==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.a11y-i18n-output-gate-results/v1",all_accessible:$r[0].all_accessible,all_localizable:$r[0].all_localizable,bad_rejected:($r[0].bad_accessible|not),verified:true}' > "$OUT/gate-summary.json"
echo "a11y-i18n-output gate passed: all output accessible and localizable, color-only message rejected"
