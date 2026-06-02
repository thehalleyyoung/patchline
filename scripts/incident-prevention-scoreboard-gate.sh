#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/incident-prevention-scoreboard-gate.json}"; OUT="${2:-results/generated/incident-prevention-scoreboard}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.incident-prevention-scoreboard-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "anonymized" "make incident-prevention-scoreboard-gate"; do grep -F "$phrase" docs/incident-prevention-scoreboard.md README.md > /dev/null; done
bash scripts/incident-prevention-scoreboard.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.incident-prevention-scoreboard/v1" and .total_consistent==true and .all_privacy_safe==true and .leaky_safe==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.incident-prevention-scoreboard-gate-results/v1",total_consistent:$r[0].total_consistent,all_privacy_safe:$r[0].all_privacy_safe,leaky_flagged:($r[0].leaky_safe|not),verified:true}' > "$OUT/gate-summary.json"
echo "incident-prevention-scoreboard gate passed: aggregate total consistent and privacy-safe, identity-leaking entry flagged"
