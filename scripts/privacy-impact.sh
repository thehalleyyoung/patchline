#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/privacy-impact-gate.json}"
OUT="${2:-results/generated/privacy-impact}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '.version == "patchline.privacy-impact-gate/v1" and (.operations|length) >= 1 and (.columns|length) >= 1' "$SPEC" > /dev/null

# Join each operation to the PII classification of its column and assign impact.
jq '
  .columns as $cols
  | def is_pii($t; $c): (($cols[] | select(.table==$t and .column==$c) | .pii) // false);
  def impact($o):
    if (is_pii($o.table; $o.column) | not) then
      { impact: "none", reason: "column is not classified as PII" }
    elif $o.type == "export" then
      { impact: "high", reason: "exports personal data off-platform" }
    elif $o.type == "delete" then
      { impact: "erasure_relevant", reason: "deletes personal data (right-to-erasure relevant)" }
    elif $o.type == "anonymize" then
      { impact: "mitigating", reason: "anonymization reduces personal-data exposure" }
    elif $o.type == "retention_change" then
      { impact: "relevant", reason: "changes how long personal data is retained" }
    else
      { impact: "relevant", reason: "touches personal data" }
    end;
  {
    version: "patchline.privacy-impact/v1",
    findings: [ .operations[] | . as $o | (impact($o)) as $i
      | { id: $o.id, type: $o.type, table: $o.table, column: $o.column, impact: $i.impact, reason: $i.reason } ]
  }
' "$SPEC" > "$OUT/privacy-impact.json"

{
  echo "# Privacy-impact inference"
  echo
  echo "| Operation | Type | Column | Impact | Reason |"
  echo "|---|---|---|---|---|"
  jq -r '.findings[] | "| \(.id) | \(.type) | \(.table).\(.column) | \(.impact) | \(.reason) |"' "$OUT/privacy-impact.json"
} > "$OUT/privacy-impact.md"
cp "$OUT/privacy-impact.md" "$OUT/README.md"

echo "privacy-impact worker: $(jq -r '.findings|length' "$OUT/privacy-impact.json") findings"
