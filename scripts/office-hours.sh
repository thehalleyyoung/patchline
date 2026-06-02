#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/office-hours-gate.json}"
OUT="${2:-results/generated/office-hours}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '.version == "patchline.office-hours-gate/v1" and (.recent_failures|length) >= 1 and (.roadmap_cards|length) >= 1' "$SPEC" > /dev/null
session="$(jq -r '.session' "$SPEC")"

nfail="$(jq '.recent_failures | length' "$SPEC")"
ncard="$(jq '.roadmap_cards | length' "$SPEC")"

{
  echo "# Public office hours: ${session}"
  echo
  echo "## Review (reproducibility failures)"
  echo
  echo "Walk through recent gate failures and reproduce them live."
  echo
  jq -r '.recent_failures[] | "- `make \(.gate)` — \(.summary)"' "$SPEC"
  echo
  echo "## Triage"
  echo
  echo "Assign owners and decide which failures block the next release."
  echo
  echo "## Planning (roadmap cards)"
  echo
  jq -r '.roadmap_cards[] | "- " + .title' "$SPEC"
} > "$OUT/agenda.md"
cp "$OUT/agenda.md" "$OUT/README.md"

failures_json="$(jq '[.recent_failures[].gate] | sort' "$SPEC")"

jq -n \
  --arg session "$session" \
  --argjson nfail "$nfail" \
  --argjson ncard "$ncard" \
  --argjson failures "$failures_json" '
  {
    version: "patchline.office-hours/v1",
    session: $session,
    failures: $failures,
    failure_count: $nfail,
    roadmap_count: $ncard
  }' > "$OUT/office-hours.json"

echo "office-hours worker: ${session} with ${nfail} failure item(s), ${ncard} roadmap card(s)"
