#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/doi-pinned-artifact-snapshot-gate.json}"; OUT="${2:-results/generated/doi-pinned-artifact-snapshot}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.doi-pinned-artifact-snapshot-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "DOI-pinned" "make doi-pinned-artifact-snapshot-gate"; do grep -F "$phrase" docs/doi-pinned-artifact-snapshot.md README.md > /dev/null; done
bash scripts/doi-pinned-artifact-snapshot.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.doi-pinned-artifact-snapshot/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.doi-pinned-artifact-snapshot-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "doi-pinned-artifact-snapshot gate passed: every item scored with evidence on real self-data, unsupported item rejected"
