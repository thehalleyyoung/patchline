#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/accountable-use-policy-gate.json}"
OUT="${2:-results/generated/accountable-use-policy}"

mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.accountable-use-policy-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null

for phrase in "human oversight" "make accountable-use-policy-gate"; do
  grep -F "$phrase" docs/accountable-use-policy.md README.md > /dev/null
done

bash scripts/accountable-use-policy.sh "$SPEC" "$OUT" > "$OUT.run.log"

jq -e '
  .version=="patchline.accountable-use-policy/v1" and
  .oversight_required == 3 and
  .oversight_mapped == 3 and
  .all_required_mapped == true and
  .bad_mapped == false
' "$OUT/out.json" > /dev/null

jq -n --slurpfile r "$OUT/out.json" '{
  version:"patchline.accountable-use-policy-gate-results/v1",
  oversight_required:$r[0].oversight_required,
  oversight_mapped:$r[0].oversight_mapped,
  bad_rejected:($r[0].bad_mapped|not),
  verified:true
}' > "$OUT/gate-summary.json"

echo "accountable-use-policy gate passed: autonomous/blocking capabilities mapped to human oversight, unmapped blocker rejected"
