#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

TEMPLATE="${1:-.github/ISSUE_TEMPLATE/real-repo-nomination.yml}"
OUT="${2:-results/generated/issue-template-gate}"
rm -rf "$OUT"
mkdir -p "$OUT"

grep -q '^name: Real repository' "$TEMPLATE"
grep -q '^  - corpus$' "$TEMPLATE"
grep -q '^  - real-repo$' "$TEMPLATE"

required_fields=(repository pinned-ref subpath ecosystem failure-mode evidence expected-support safety)
for field in "${required_fields[@]}"; do
  grep -q "id: $field" "$TEMPLATE"
done

required_count="$(grep -c 'required: true' "$TEMPLATE")"
if [ "$required_count" -lt 9 ]; then
  echo "expected at least 9 required validations, found $required_count" >&2
  exit 1
fi

jq -n \
  --argjson fields "$(printf '%s\n' "${required_fields[@]}" | jq -R . | jq -s .)" \
  --argjson required_count "$required_count" \
  '{version:"patchline.issue-template-gate-results/v1", fields:$fields, required_validations:$required_count, verified:true}' > "$OUT/template.json"

echo "issue-template gate passed: real-repo nomination form has ${#required_fields[@]} required field groups"
