#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/incremental-reanalysis-gate.json}"; OUT="${2:-results/generated/incremental-reanalysis}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.incremental-reanalysis-gate/v1"' "$SPEC" > /dev/null
jq '
  .cached as $c | .current as $n
  | ([ $n | keys[] | select(($c[.] == null)) ]) as $new
  | ([ $n | keys[] | select(($c[.] != null) and ($c[.] != $n[.])) ]) as $changed
  | ([ $n | keys[] | select(($c[.] != null) and ($c[.] == $n[.])) ]) as $unchanged
  | {
      version: "patchline.incremental-reanalysis/v1",
      total: ($n|keys|length),
      new: ($new|sort), changed: ($changed|sort), unchanged: ($unchanged|sort),
      reprocess: (($new + $changed)|sort),
      reprocess_count: (($new + $changed)|length),
      full_count: ($n|keys|length),
      strictly_smaller: ((($new + $changed)|length) < ($n|keys|length))
    }
' "$SPEC" > "$OUT/incr.json"
{ echo "# Incremental re-analysis cache"; echo; echo "Reprocess: $(jq -rc '.reprocess' "$OUT/incr.json") ($(jq -r '.reprocess_count' "$OUT/incr.json")/$(jq -r '.full_count' "$OUT/incr.json"))"; echo "Unchanged skipped: $(jq -rc '.unchanged' "$OUT/incr.json")"; } > "$OUT/incr.md"
cp "$OUT/incr.md" "$OUT/README.md"
echo "incremental-reanalysis worker: reprocess=$(jq -rc '.reprocess' "$OUT/incr.json") smaller=$(jq -r '.strictly_smaller' "$OUT/incr.json")"
