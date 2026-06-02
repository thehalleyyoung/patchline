#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/security-threat-model-gate.json}"; OUT="${2:-results/generated/security-threat-model}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.security-threat-model-gate/v1"' "$SPEC" > /dev/null
jq '

  def r4: (.*10000|round)/10000;
  .threats as $T | .unmitigated_threat as $U
  | ([ $T[] | select(.present==true) ]|length) as $cov
  | {version:"patchline.security-threat-model/v1",
     threats:($T|length), mitigated:$cov,
     coverage:(($cov/($T|length))|r4),
     all_mitigated:($cov==($T|length)),
     unmitigated_present:($U.present==true)}

' "$SPEC" > "$OUT/out.json"
{ echo "# Security threat model"; echo; echo "Threats $(jq -r .threats "$OUT/out.json"); mitigated $(jq -r .mitigated "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "security-threat-model worker: all_mitigated=$(jq -r .all_mitigated "$OUT/out.json")"
