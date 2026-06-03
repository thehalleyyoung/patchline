#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/role-specific-tutorials-gate.json}"
OUT="${2:-results/generated/role-specific-tutorials}"

mkdir -p "$(dirname "$OUT")"

jq -e '
  .version == "patchline.role-specific-tutorials-gate/v1" and
  (.claim | length) > 200 and
  (.required_roles | sort) == ["app-developer","dba","engineering-manager","security-reviewer","sre"] and
  (.roles | length) == 5
' "$SPEC" > /dev/null

for phrase in "role-specific tutorials" "app developers" "DBAs" "SREs" "security reviewers" "engineering managers" "make role-specific-tutorials-gate"; do
  grep -F "$phrase" docs/role-specific-tutorials.md README.md > /dev/null
done

bash scripts/role-specific-tutorials.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in README.md tutorials.json; do
  test -s "$OUT/$output"
done

jq -e '
  .version == "patchline.role-specific-tutorials/v1" and
  .count == 5 and
  .all_ok == true and
  .bad_ok == false and
  ([.roles[].id] | sort) == ["app-developer","dba","engineering-manager","security-reviewer","sre"] and
  all(.roles[]; .complete == true and .commands >= 2 and .success_checks >= 3 and .hazard_classes >= 1)
' "$OUT/tutorials.json" > /dev/null

while IFS=$'\t' read -r id path; do
  test -s "$path"
  tutorial="$OUT/tutorials/$id.md"
  test -s "$tutorial"
  for section in "## Mission" "## Real code evidence" "## Tutorial path" "## Commands" "## Gate-backed success criteria" "## Review decision" "## Handoff"; do
    grep -Fq "$section" "$tutorial"
  done
  grep -Fq "$path" "$tutorial"
done < <(jq -r '.roles[] | [.id, .real_code.local_path] | @tsv' "$SPEC")

jq -n --slurpfile r "$OUT/tutorials.json" '{
  version: "patchline.role-specific-tutorials-gate-results/v1",
  roles: $r[0].count,
  all_ok: $r[0].all_ok,
  generic_tutorial_rejected: ($r[0].bad_ok | not),
  verified: true
}' > "$OUT/gate-summary.json"

echo "role-specific tutorials gate passed: five role tutorials generated, real evidence paths verified, generic tutorial rejected"
