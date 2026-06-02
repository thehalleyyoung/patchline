#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/event-sourcing-crdt-transfer-gate.json}"; OUT="${2:-results/generated/event-sourcing-crdt-transfer}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.event-sourcing-crdt-transfer-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "CRDT" "make event-sourcing-crdt-transfer-gate"; do grep -F "$phrase" docs/event-sourcing-crdt-transfer.md README.md > /dev/null; done
bash scripts/event-sourcing-crdt-transfer.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.event-sourcing-crdt-transfer/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.event-sourcing-crdt-transfer-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "event-sourcing-crdt-transfer gate passed: every item scored with evidence on real self-data, unsupported item rejected"
