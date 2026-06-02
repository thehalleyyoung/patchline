#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/replication-leaderboard-gate.json}"; OUT="${2:-results/generated/replication-leaderboard}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.replication-leaderboard-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "leaderboard" "make replication-leaderboard-gate"; do grep -F "$phrase" docs/replication-leaderboard.md README.md > /dev/null; done
bash scripts/replication-leaderboard.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.replication-leaderboard/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.replication-leaderboard-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "replication-leaderboard gate passed: every entry reproduced and valid, unreproduced entry rejected"
