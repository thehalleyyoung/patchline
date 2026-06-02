#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/incremental-reanalysis-gate.json}"; OUT="${2:-results/generated/incremental-reanalysis}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.incremental-reanalysis-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "incremental" "make incremental-reanalysis-gate"; do grep -F "$phrase" docs/incremental-reanalysis.md README.md > /dev/null; done
bash scripts/incremental-reanalysis.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '
  .version == "patchline.incremental-reanalysis/v1" and
  (.reprocess == ["alembic/alembic","rails/rails"]) and
  (.unchanged == ["django/django","gorm/gorm","prisma/prisma"]) and
  .strictly_smaller == true
' "$OUT/incr.json" > /dev/null
jq -n --slurpfile r "$OUT/incr.json" '{version:"patchline.incremental-reanalysis-gate-results/v1", reprocess:$r[0].reprocess, skipped:$r[0].unchanged, strictly_smaller:$r[0].strictly_smaller, verified:true}' > "$OUT/gate-summary.json"
echo "incremental-reanalysis gate passed: only changed+new reprocessed, unchanged skipped"
