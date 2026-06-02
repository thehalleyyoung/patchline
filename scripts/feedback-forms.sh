#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/feedback-forms-gate.json}"
OUT="${2:-results/generated/feedback-forms}"
rm -rf "$OUT"
mkdir -p "$OUT/forms" "$OUT/templates"

jq -e '.version == "patchline.feedback-forms-gate/v1" and (.forms|length) >= 1' "$SPEC" > /dev/null

index="$OUT/forms.jsonl"
: > "$index"

n="$(jq '.forms | length' "$SPEC")"
for i in $(seq 0 $((n-1))); do
  name="$(jq -r ".forms[$i].name" "$SPEC")"
  title="$(jq -r ".forms[$i].title" "$SPEC")"
  label="$(jq -r ".forms[$i].label" "$SPEC")"
  nfields="$(jq ".forms[$i].fields | length" "$SPEC")"

  # Plain-Markdown form: no scripts, no external assets, no analytics.
  form="$OUT/forms/${name}.md"
  {
    echo "# Feedback: ${title}"
    echo
    echo "_This form is analytics-free. Nothing is sent anywhere. Fill it in and paste"
    echo "the structured template below into a local issue._"
    echo
    jq -r ".forms[$i].fields[] | \"- \" + .label + (if .required then \" (required)\" else \" (optional)\" end)" "$SPEC"
  } > "$form"

  # Structured local issue template a reader can copy/paste.
  tmpl="$OUT/templates/${name}.md"
  {
    echo "---"
    echo "name: ${title}"
    echo "labels: [${label}]"
    echo "---"
    echo
    jq -r ".forms[$i].fields[] | .label + (if .required then \" (required):\" else \" (optional):\" end)" "$SPEC"
  } > "$tmpl"

  jq -n --arg name "$name" --arg title "$title" --arg label "$label" --argjson nfields "$nfields" -c \
    '{name:$name, title:$title, label:$label, fields:$nfields}' >> "$index"
done

sort -o "$index" "$index"

{
  echo "# Documentation feedback forms"
  echo
  echo "Analytics-free forms that produce structured local issue templates."
  echo
  echo "| Form | Label | Fields |"
  echo "|---|---|---|"
  jq -r '"| `\(.name)` | \(.label) | \(.fields) |"' "$index"
} > "$OUT/feedback-forms.md"
cp "$OUT/feedback-forms.md" "$OUT/README.md"

jq -s '{
  version: "patchline.feedback-forms/v1",
  forms: sort_by(.name),
  count: length
}' "$index" > "$OUT/feedback-forms.json"

echo "feedback-forms worker: $(jq -r .count "$OUT/feedback-forms.json") analytics-free forms with local templates"
