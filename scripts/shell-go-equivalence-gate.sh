#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/shell-go-equivalence-gate.json}"; OUT="${2:-results/generated/shell-go-equivalence}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.shell-go-equivalence-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "equivalence" "make shell-go-equivalence-gate"; do grep -F "$phrase" docs/shell-go-equivalence.md README.md > /dev/null; done
bash scripts/shell-go-equivalence.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.shell-go-equivalence/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.shell-go-equivalence-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "shell-go-equivalence gate passed: shell and Go agree on every fixture, seeded mismatch detected"
