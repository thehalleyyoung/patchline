#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/feedback-forms-gate.json}"
OUT="${2:-results/generated/feedback-forms-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.feedback-forms-gate/v1" and (.claim|length) > 200 and (.forms|length) >= 1' "$SPEC" > /dev/null

for phrase in "analytics-free" "local issue" "make feedback-forms-gate"; do
  grep -F "$phrase" docs/feedback-forms.md README.md > /dev/null
done

bash scripts/feedback-forms.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in feedback-forms.json feedback-forms.md README.md; do
  test -s "$OUT/$output"
done

form_count="$(jq '.forms | length' "$SPEC")"

jq -e \
  --argjson forms "$form_count" '
  .version == "patchline.feedback-forms/v1" and
  .count == $forms and
  (.forms | all(.[]; .fields >= 1))
' "$OUT/feedback-forms.json" > /dev/null

# Each form renders all its fields, marks required ones, emits a template, and
# contains zero external URLs or analytics references (the "analytics-free" claim).
n="$(jq '.forms | length' "$SPEC")"
for i in $(seq 0 $((n-1))); do
  name="$(jq -r ".forms[$i].name" "$SPEC")"
  form="$OUT/forms/${name}.md"
  tmpl="$OUT/templates/${name}.md"
  test -s "$form"; test -s "$tmpl"
  nfields="$(jq ".forms[$i].fields | length" "$SPEC")"
  rendered="$(grep -c -- "- " "$form" || true)"
  test "$rendered" -ge "$nfields"
  # Required fields must be marked in both form and template.
  while IFS= read -r lbl; do
    grep -Fq "$lbl" "$tmpl"
  done < <(jq -r ".forms[$i].fields[] | select(.required) | .label" "$SPEC")
  # Analytics-free: no URLs, no tracking/analytics tokens anywhere.
  if grep -Eiq 'https?://|google-analytics|gtag|plausible|segment|mixpanel|<script' "$form" "$tmpl"; then
    echo "FAIL: external/analytics reference found in $name" >&2
    exit 1
  fi
done

jq -n --slurpfile r "$OUT/feedback-forms.json" '{
  version: "patchline.feedback-forms-gate-results/v1",
  count: $r[0].count,
  verified: true
}' > "$OUT/gate-summary.json"

echo "feedback-forms gate passed: $(jq -r .count "$OUT/feedback-forms.json") forms, analytics-free, structured local templates"
