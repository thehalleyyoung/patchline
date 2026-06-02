#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/secret-leak-scanner-gate.json}"; OUT="${2:-results/generated/secret-leak-scanner}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.secret-leak-scanner-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "zero-tolerance" "make secret-leak-scanner-gate"; do grep -F "$phrase" docs/secret-leak-scanner.md README.md > /dev/null; done
bash scripts/secret-leak-scanner.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.secret-leak-scanner/v1" and .clean==true and .leaks==0 and .leaky_detected==true' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.secret-leak-scanner-gate-results/v1",clean:$r[0].clean,leak_detected:$r[0].leaky_detected,verified:true}' > "$OUT/gate-summary.json"
echo "secret-leak-scanner gate passed: clean artifacts leak-free, seeded secret detected"
