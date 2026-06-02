#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/governance-gate.json}"
OUT="${2:-results/generated/governance}"
rm -rf "$OUT"
mkdir -p "$OUT/charters"

jq -e '.version == "patchline.governance-gate/v1" and (.roles|length) >= 4' "$SPEC" > /dev/null

index="$OUT/roles.jsonl"
: > "$index"

n="$(jq '.roles | length' "$SPEC")"
for i in $(seq 0 $((n-1))); do
  role="$(jq -r ".roles[$i].role" "$SPEC")"
  scope="$(jq -r ".roles[$i].scope" "$SPEC")"
  esc="$(jq -r ".roles[$i].escalation" "$SPEC")"
  nresp="$(jq ".roles[$i].responsibilities | length" "$SPEC")"
  ngates="$(jq ".roles[$i].accountable_gates | length" "$SPEC")"

  charter="$OUT/charters/${role}.md"
  {
    echo "# Governance charter: ${role}"
    echo
    echo "## Scope"
    echo
    echo "$scope"
    echo
    echo "## Responsibilities"
    echo
    jq -r ".roles[$i].responsibilities[] | \"- \" + ." "$SPEC"
    echo
    echo "## Escalation path"
    echo
    echo "$esc"
    echo
    echo "## Accountable gates"
    echo
    jq -r ".roles[$i].accountable_gates[] | \"- \`make \" + . + \"\`\"" "$SPEC"
  } > "$charter"

  jq -n --arg role "$role" --arg scope "$scope" --arg esc "$esc" \
    --argjson nresp "$nresp" --argjson ngates "$ngates" -c \
    '{role:$role, scope:$scope, escalation:$esc, responsibilities:$nresp, accountable_gates:$ngates}' \
    >> "$index"
done

sort -o "$index" "$index"

{
  echo "# Patchline governance"
  echo
  echo "Role-specific charters so responsibilities are explicit and reproducible."
  echo
  echo "| Role | Responsibilities | Accountable gates |"
  echo "|---|---|---|"
  jq -r '"| `\(.role)` | \(.responsibilities) | \(.accountable_gates) |"' "$index"
} > "$OUT/governance.md"
cp "$OUT/governance.md" "$OUT/README.md"

jq -s '{
  version: "patchline.governance/v1",
  roles: sort_by(.role),
  count: length
}' "$index" > "$OUT/governance.json"

echo "governance worker: $(jq -r .count "$OUT/governance.json") role charters generated"
