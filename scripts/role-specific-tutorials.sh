#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/role-specific-tutorials-gate.json}"
OUT="${2:-results/generated/role-specific-tutorials}"

rm -rf "$OUT"
mkdir -p "$OUT/tutorials"

jq -e '.version == "patchline.role-specific-tutorials-gate/v1"' "$SPEC" > /dev/null

while IFS= read -r path; do
  test -s "$path"
done < <(jq -r '.roles[].real_code.local_path' "$SPEC")

jq '
  def complete($r):
    ([$r.id, $r.title, $r.mission, $r.real_code.local_path, $r.real_code.proof,
      $r.review_decision, $r.handoff] | all(length > 0))
    and (($r.commands | length) >= 2)
    and (($r.success_checks | length) >= 3)
    and (($r.hazard_classes | length) >= 1)
    and (($r.steps | length) >= 3);
  {
    version: "patchline.role-specific-tutorials/v1",
    count: (.roles | length),
    required_roles,
    roles: [
      .roles[] | {
        id,
        title,
        real_code: .real_code.local_path,
        proof: .real_code.proof,
        hazard_classes: (.hazard_classes | length),
        commands: (.commands | length),
        success_checks: (.success_checks | length),
        complete: complete(.)
      }
    ],
    all_ok: all(.roles[]; complete(.)),
    bad_ok: complete(.bad)
  }
' "$SPEC" > "$OUT/tutorials.json"

{
  echo "# Role-specific tutorials"
  echo
  echo "| Role | Real code evidence | Commands | Success checks |"
  echo "| --- | --- | ---: | ---: |"
  jq -r '.roles[] | "| \(.title) | `\(.real_code)` | \(.commands) | \(.success_checks) |"' "$OUT/tutorials.json"
} > "$OUT/README.md"

while IFS=$'\t' read -r id title; do
  role_file="$OUT/tutorials/$id.md"
  jq -r --arg id "$id" '
    .roles[] | select(.id == $id) |
    "# \(.title) tutorial\n\n" +
    "## Mission\n\n\(.mission)\n\n" +
    "## Real code evidence\n\n- `\(.real_code.local_path)`: \(.real_code.proof)\n\n" +
    "## Hazard classes\n\n" +
    (.hazard_classes | map("- " + .) | join("\n")) + "\n\n" +
    "## Tutorial path\n\n" +
    (.steps | to_entries | map("\(.key + 1). \(.value)") | join("\n")) + "\n\n" +
    "## Commands\n\n" +
    (.commands | map("- `" + . + "`") | join("\n")) + "\n\n" +
    "## Gate-backed success criteria\n\n" +
    (.success_checks | map("- " + .) | join("\n")) + "\n\n" +
    "## Review decision\n\n\(.review_decision)\n\n" +
    "## Handoff\n\n\(.handoff)\n"
  ' "$SPEC" > "$role_file"
done < <(jq -r '.roles[] | [.id, .title] | @tsv' "$SPEC")

echo "role-specific tutorials generated: $(jq -r .count "$OUT/tutorials.json") roles"
