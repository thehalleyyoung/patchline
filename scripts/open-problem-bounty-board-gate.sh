#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/open-problem-bounty-board-gate.json}"; OUT="${2:-results/generated/open-problem-bounty-board}"
mkdir -p "$(dirname "$OUT")"
jq -e '
  .version == "patchline.open-problem-bounty-board-gate/v1" and
  (.claim|length) > 200 and
  (.items | length) >= 2 and
  any(.items[]; .category == "false_negative") and
  any(.items[]; .category == "ecosystem_gap") and
  all(.items[];
    (.payout_usd // 0) > 0 and
    .payment_status == "approved" and
    (.minimized_reproduction | length) > 0 and
    (.evidence | length) > 0
  )
' "$SPEC" > /dev/null
for phrase in "bounty board" "minimized reproductions" "false negatives" "ecosystem gaps" "make open-problem-bounty-board-gate"; do grep -F "$phrase" docs/open-problem-bounty-board.md README.md > /dev/null; done
bash scripts/open-problem-bounty-board.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.open-problem-bounty-board/v1" and .all_ok==true and .bad_ok==false and .missing_reproductions == 0 and .false_negative_bounties >= 1 and .ecosystem_gap_bounties >= 1 and .payout_usd_total >= 1000' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.open-problem-bounty-board-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,false_negative_bounties:$r[0].false_negative_bounties,ecosystem_gap_bounties:$r[0].ecosystem_gap_bounties,payout_usd_total:$r[0].payout_usd_total,missing_reproductions:$r[0].missing_reproductions,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "open-problem-bounty-board gate passed: minimized false-negative and ecosystem-gap reproductions are payout-approved; unsupported item rejected"
