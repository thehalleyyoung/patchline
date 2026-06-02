#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/roadmap-burndown-gate.json}"; OUT="${2:-results/generated/roadmap-burndown}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.roadmap-burndown-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "milestone" "make roadmap-burndown-gate"; do grep -F "$phrase" docs/roadmap-burndown.md README.md > /dev/null; done
bash scripts/roadmap-burndown.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.roadmap-burndown/v1" and .open_all_backed==true and .evidence_free_ok==false and .done==2' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.roadmap-burndown-gate-results/v1",burndown:$r[0].burndown,open_all_backed:$r[0].open_all_backed,evidence_free_rejected:($r[0].evidence_free_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "roadmap-burndown gate passed: burndown consistent and open milestones gate-backed, evidence-free completion rejected"
