#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/artifact-badge-audit-gate.json}"; OUT="${2:-results/generated/artifact-badge-audit}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.artifact-badge-audit-gate/v1"' "$SPEC" > /dev/null
jq '

  def r4: (.*10000|round)/10000;
  .badges as $B | .unearned_badge as $U
  | ([ $B[] | select(.met and ((.evidence|length)>0)) ]|length) as $ok
  | {version:"patchline.artifact-badge-audit/v1",
     badges:($B|length), earned:$ok,
     earn_rate:(($ok/($B|length))|r4),
     all_earned:($ok==($B|length)),
     unearned_met:($U.met and (($U.evidence|length)>0))}

' "$SPEC" > "$OUT/out.json"
{ echo "# Artifact-badge self-audit"; echo; echo "Earned $(jq -r .earned "$OUT/out.json")/$(jq -r .badges "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "artifact-badge-audit worker: all_earned=$(jq -r .all_earned "$OUT/out.json")"
