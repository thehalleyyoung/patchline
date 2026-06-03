#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/open-problem-bounty-board-gate.json}"; OUT="${2:-results/generated/open-problem-bounty-board}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.open-problem-bounty-board-gate/v1"' "$SPEC" > /dev/null
missing_reproductions=0
while IFS= read -r reproduction; do
  case "$reproduction" in
    ""|/*|../*|*"/../"*|*".." )
      missing_reproductions=$((missing_reproductions + 1))
      ;;
    *)
      if [ ! -s "$reproduction" ]; then
        missing_reproductions=$((missing_reproductions + 1))
      fi
      ;;
  esac
done < <(jq -r '.items[]?.minimized_reproduction // empty' "$SPEC")

jq --argjson missing_reproductions "$missing_reproductions" '
  def eligible:
    ((.category == "false_negative") or (.category == "ecosystem_gap")) and
    ((.payout_usd // 0) > 0) and
    ((.minimized_reproduction // "" | length) > 0) and
    ((.evidence // "" | length) > 0) and
    ((.payment_status // "") == "approved");
  .items as $I | .bad as $B
  | ([ $I[] | select(eligible) ]|length) as $ok
  | ([ $I[] | select(eligible) | .payout_usd ] | add // 0) as $payout
  | ([ $I[] | select(eligible and .category == "false_negative") ]|length) as $fn
  | ([ $I[] | select(eligible and .category == "ecosystem_gap") ]|length) as $gap
  | {version:"patchline.open-problem-bounty-board/v1",
     total:($I|length), ok:$ok,
     false_negative_bounties:$fn,
     ecosystem_gap_bounties:$gap,
     payout_usd_total:$payout,
     missing_reproductions:$missing_reproductions,
     all_ok:($ok==($I|length) and $missing_reproductions == 0),
     bad_ok:($B|eligible)}
' "$SPEC" > "$OUT/out.json"
{ echo "# Open-problem bounty board"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json") payout-approved minimized reproductions; total payout \$$(jq -r .payout_usd_total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "open-problem-bounty-board worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
