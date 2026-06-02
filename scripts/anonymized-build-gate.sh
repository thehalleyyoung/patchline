#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/anonymized-build-gate.json}"; OUT="${2:-results/generated/anonymized-build}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.anonymized-build-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "anonymized" "make anonymized-build-gate"; do grep -F "$phrase" docs/anonymized-build.md README.md > /dev/null; done
bash scripts/anonymized-build.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.anonymized-build/v1" and .clean==true and .leaks==0 and .raw_detected==true' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.anonymized-build-gate-results/v1",clean:$r[0].clean,raw_detected:$r[0].raw_detected,verified:true}' > "$OUT/gate-summary.json"
echo "anonymized-build gate passed: anonymized build identity-free, un-anonymized control detected"
