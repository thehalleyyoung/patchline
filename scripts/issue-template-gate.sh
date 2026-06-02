#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

OUT="${1:-results/generated/issue-template-gate}"
rm -rf "$OUT"
mkdir -p "$OUT/samples" "$OUT/cache"

LABELS=".github/labels.yml"
TEMPLATE_DIR=".github/ISSUE_TEMPLATE"

required_labels=(
  triage
  ecosystem
  parser
  false-positive
  false-negative
  artifact-regression
  corpus
  real-repo
  needs-repro
  security-review
)

for label in "${required_labels[@]}"; do
  grep -Eq "^- name: ${label}$" "$LABELS"
  grep -A2 -E "^- name: ${label}$" "$LABELS" | grep -Eq '^  color: "[0-9a-f]{6}"$'
  grep -A3 -E "^- name: ${label}$" "$LABELS" | grep -Eq '^  description: .{20,}$'
done

validate_template() {
  local file="$1"
  shift
  test -s "$TEMPLATE_DIR/$file"
  grep -Eq '^name: .+' "$TEMPLATE_DIR/$file"
  grep -Eq '^description: .{30,}$' "$TEMPLATE_DIR/$file"
  grep -q '^labels:$' "$TEMPLATE_DIR/$file"
  grep -q '^  - triage$' "$TEMPLATE_DIR/$file"
  grep -q '^  - needs-repro$' "$TEMPLATE_DIR/$file"
  for label in "$@"; do
    grep -q "^  - ${label}$" "$TEMPLATE_DIR/$file"
  done
  local required_count
  required_count="$(grep -c 'required: true' "$TEMPLATE_DIR/$file")"
  test "$required_count" -ge 5
}

validate_template real-repo-nomination.yml corpus real-repo
validate_template ecosystem-support.yml ecosystem
validate_template parser-request.yml parser
validate_template false-positive.yml false-positive
validate_template false-negative.yml false-negative
validate_template artifact-regression.yml artifact-regression

check_fields() {
  local file="$1"
  shift
  for field in "$@"; do
    grep -q "id: $field" "$TEMPLATE_DIR/$file"
  done
}

check_fields real-repo-nomination.yml repository pinned-ref subpath ecosystem failure-mode evidence expected-support safety
check_fields ecosystem-support.yml ecosystem public-repo pinned-ref subpath current-result expected-support safety
check_fields parser-request.yml parser-scope public-repo pinned-ref file-path expected-fact current-output safety
check_fields false-positive.yml finding-id reproduction evidence affected-surface safety
check_fields false-negative.yml public-repo pinned-ref path missed-behavior current-output safety
check_fields artifact-regression.yml artifact-kind reproduction expected-artifact actual-artifact impact safety

grep -q '^blank_issues_enabled: false$' "$TEMPLATE_DIR/config.yml"

case_out="$OUT/real-code"
mkdir -p "$case_out"
go run ./cmd/patchline repo analyze \
  --github bytebase/bytebase \
  --ref 0765652ea2dbdf8e93ae44bff5acafc1b97a92cc \
  --subpath backend/migrator/migration \
  --download-dir "$OUT/cache" \
  --stages inventory,baseline,propose,compare \
  --proposal-kind all \
  --budget files=4,lines=80,tokens=12000,changes=2 \
  --no-llm \
  --out "$case_out/analyze" \
  --json > "$case_out/stdout.json"

jq -e '
  .summary.files_scanned > 0 and
  .summary.ranked_risks > 0 and
  .summary.generated_files > 0 and
  .summary.intervention_loops > 0
' "$case_out/analyze/analyze.json" > /dev/null

top_risk_json="$(jq -c '.risks[0]' "$case_out/analyze/baseline/baseline.json")"
top_stable_id="$(jq -r '.stable_id // .id' <<<"$top_risk_json")"
top_path="$(jq -r '.path // "unknown"' <<<"$top_risk_json")"
top_kind="$(jq -r '.kind // "risk"' <<<"$top_risk_json")"
generated_path="$(jq -r '.generated_files[0].path // "patchline-proposals/"' "$case_out/analyze/proposal/proposal.json")"

jq -n \
  --arg template "false-positive.yml" \
  --arg finding "$top_stable_id" \
  --arg path "$top_path" \
  --arg kind "$top_kind" \
  --arg command "go run ./cmd/patchline repo analyze --github bytebase/bytebase --ref 0765652ea2dbdf8e93ae44bff5acafc1b97a92cc --subpath backend/migrator/migration --no-llm" \
  '{template:$template, labels:["triage","false-positive","needs-repro"], finding_id:$finding, path:$path, kind:$kind, reproduction_command:$command, public_safe:true}' \
  > "$OUT/samples/false-positive.json"

jq -n \
  --arg template "false-negative.yml" \
  --arg repo "bytebase/bytebase" \
  --arg ref "0765652ea2dbdf8e93ae44bff5acafc1b97a92cc" \
  --arg path "$top_path" \
  '{template:$template, labels:["triage","false-negative","needs-repro"], repo:$repo, ref:$ref, path:$path, missed_behavior:"Expected additional evidence link or risk classification for this public data-change path.", public_safe:true}' \
  > "$OUT/samples/false-negative.json"

jq -n \
  --arg template "artifact-regression.yml" \
  --arg generated "$generated_path" \
  --slurpfile analyze "$case_out/analyze/analyze.json" \
  '{template:$template, labels:["triage","artifact-regression","needs-repro"], artifact_kind:"proposal or compare report", generated_artifact:$generated, analyze_hash:$analyze[0].hash, public_safe:true}' \
  > "$OUT/samples/artifact-regression.json"

jq -n \
  --arg template "parser-request.yml" \
  --arg repo "bytebase/bytebase" \
  --arg ref "0765652ea2dbdf8e93ae44bff5acafc1b97a92cc" \
  --arg path "backend/migrator/migration" \
  '{template:$template, labels:["triage","parser","needs-repro"], parser_scope:"Bytebase migration SQL facts", repo:$repo, ref:$ref, path:$path, expected_fact:"Normalized table/write facts from public migrations.", public_safe:true}' \
  > "$OUT/samples/parser-request.json"

jq -n \
  --arg template "ecosystem-support.yml" \
  --arg repo "bytebase/bytebase" \
  --arg ref "0765652ea2dbdf8e93ae44bff5acafc1b97a92cc" \
  --arg path "backend/migrator/migration" \
  '{template:$template, labels:["triage","ecosystem","needs-repro"], ecosystem:"Go migrations", repo:$repo, ref:$ref, path:$path, expected_support:"Facts, risks, generated interventions, and compare checks for public Go migration slices.", public_safe:true}' \
  > "$OUT/samples/ecosystem-support.json"

jq -n \
  --argjson labels "$(printf '%s\n' "${required_labels[@]}" | jq -R . | jq -s .)" \
  --argjson templates "$(printf '%s\n' artifact-regression.yml ecosystem-support.yml false-negative.yml false-positive.yml parser-request.yml real-repo-nomination.yml | jq -R . | jq -s .)" \
  --slurpfile analyze "$case_out/analyze/analyze.json" \
  '{
    version:"patchline.issue-template-gate-results/v1",
    labels:$labels,
    templates:$templates,
    real_code:{
      repo:"bytebase/bytebase",
      files_scanned:$analyze[0].summary.files_scanned,
      ranked_risks:$analyze[0].summary.ranked_risks,
      generated_files:$analyze[0].summary.generated_files,
      intervention_loops:$analyze[0].summary.intervention_loops
    },
    sample_payloads:5,
    verified:true
  }' > "$OUT/template.json"

jq -e '.verified == true and (.labels | length) >= 10 and (.templates | length) >= 6 and .real_code.ranked_risks > 0 and .sample_payloads == 5' "$OUT/template.json" > /dev/null

echo "issue-template gate passed: $(jq '.templates | length' "$OUT/template.json") templates, $(jq '.labels | length' "$OUT/template.json") labels, real risks $(jq '.real_code.ranked_risks' "$OUT/template.json")"
