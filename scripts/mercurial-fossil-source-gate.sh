#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/mercurial-fossil-source-gate.json}"
OUT="${2:-results/generated/mercurial-fossil-source-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.mercurial-fossil-source-gate/v1" and (.vcs_cases|length) >= 2' "$SPEC" > /dev/null

for phrase in "Mercurial" "Fossil" "vcs" "content-addressed" "provenance" "make mercurial-fossil-source-gate"; do
  grep -F "$phrase" docs/mercurial-fossil-source.md README.md > /dev/null
done

bash scripts/mercurial-fossil-source.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in mercurial-fossil-source.json mercurial-fossil-source.md README.md hg-first.json hg-second.json fossil-first.json; do
  test -s "$OUT/$output"
done

jq -e '
  .version == "patchline.mercurial-fossil-source/v1" and
  .mercurial.vcs == "mercurial" and
  .fossil.vcs == "fossil" and
  .mercurial_provenance_ok == true and
  .fossil_provenance_ok == true and
  .cache_semantics_ok == true and
  .risk_survives_ingestion == true
' "$OUT/mercurial-fossil-source.json" > /dev/null

# Independently re-verify cache semantics from the raw fetch records.
first_hit="$(jq -r '.cache_hit // false' "$OUT/hg-first.json")"
second_hit="$(jq -r '.cache_hit // false' "$OUT/hg-second.json")"
if [ "$first_hit" != "false" ] || [ "$second_hit" != "true" ]; then
  echo "cache semantics not satisfied: first=$first_hit second=$second_hit"; exit 1
fi

jq -n --slurpfile r "$OUT/mercurial-fossil-source.json" '{
  version: "patchline.mercurial-fossil-source-gate-results/v1",
  mercurial_vcs: $r[0].mercurial.vcs,
  fossil_vcs: $r[0].fossil.vcs,
  destructive_detected: $r[0].mercurial.destructive_migrations_detected,
  verified: true
}' > "$OUT/gate-summary.json"

echo "mercurial/fossil source gate passed: hg+fossil ingested, cache semantics verified, destructive migration surfaced"
