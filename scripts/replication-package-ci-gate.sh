#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/replication-package-ci-gate.json}"; OUT="${2:-results/generated/replication-package-ci}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.replication-package-ci-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "CI provider" "make replication-package-ci-gate"; do grep -F "$phrase" docs/replication-package-ci.md README.md > /dev/null; done
bash scripts/replication-package-ci.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.replication-package-ci/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.replication-package-ci-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "replication-package-ci gate passed: every CI provider reproduces identically, divergent provider rejected"
