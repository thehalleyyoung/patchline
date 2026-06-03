#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/localized-teaching-examples.json}"
OUT="${2:-results/generated/localized-teaching-examples-gate}"

rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.localized-teaching/v1" and
  (.claim | length) > 250 and
  (.examples | length) == 2 and
  (.criteria.required_locales | sort) == ["es","fr"] and
  (.criteria.required_audiences | sort) == ["app-developer","dba"] and
  (.criteria.required_accessibility_checks | sort) == ["alt-text","plain-language","reading-order"] and
  .criteria.require_technical_terms == true and
  .criteria.require_equivalence_checks == true and
  .criteria.require_accessibility_checks == true
' "$SPEC" > /dev/null

for phrase in "Localized teaching examples" "make localized-teaching-examples-gate" "technical equivalence" "accessibility"; do
  grep -F "$phrase" docs/localized-teaching-examples.md README.md > /dev/null
done

go test ./internal/education -run 'TestBuildLocalizedTeachingReport|TestReadLocalizedTeachingSpec|TestWriteLocalizedTeachingArtifacts' -count=1
go test ./cmd/patchline -run TestLocalizedTeachingExamplesCommandWritesReports -count=1

go run ./cmd/patchline localized-teaching-examples \
  --spec "$SPEC" \
  --root . \
  --out "$OUT/safe" \
  --json > "$OUT/safe.stdout.json"

test -s "$OUT/safe/localized-teaching-examples.json"
test -s "$OUT/safe/localized-teaching-examples.md"

jq -e '
  .version == "patchline.localized-teaching-report/v1" and
  .ok == true and
  .summary.examples == 2 and
  .summary.translations == 4 and
  .summary.locales_covered == 2 and
  .summary.audiences_covered == 2 and
  .summary.reproducible_examples == 2 and
  .summary.technical_terms == 12 and
  .summary.equivalence_checks == 8 and
  .summary.accessibility_checks == 12 and
  .summary.evidence_artifacts == 20 and
  .summary.negative_controls == 2 and
  ([.examples[].translations[].evidence[] | select(.sha256 | length == 64)] | length) == 16
' "$OUT/safe/localized-teaching-examples.json" > /dev/null

jq '
  .examples = .examples[:1] |
  .examples[0].concepts = [.examples[0].concepts[0]] |
  .examples[0].reproduce_commands = [] |
  .examples[0].evidence_paths = ["docs/missing-localized-teaching-evidence.md"] |
  .examples[0].translations = [.examples[0].translations[0]] |
  .examples[0].translations[0].technical_terms = [{"id":"risk-token","source":"risk_id","translation":"riesgo_id","must_preserve":true}] |
  .examples[0].translations[0].equivalence_checks = [{"id":"broken-equivalence","kind":"identifier-preservation","source_quote":"missing source quote","translated_quote":"missing translated quote","preserved_tokens":["missing_token"]}] |
  .examples[0].translations[0].accessibility_checks = [] |
  .examples[0].negative_controls = []
' "$SPEC" > "$OUT/bad-spec.json"

go run ./cmd/patchline localized-teaching-examples \
  --spec "$OUT/bad-spec.json" \
  --root . \
  --out "$OUT/bad" \
  --json > "$OUT/bad.stdout.json"

jq -e '
  .ok == false and
  (.counterexamples | any(.kind == "insufficient_examples")) and
  (.counterexamples | any(.kind == "missing_required_audience")) and
  (.counterexamples | any(.kind == "missing_required_locale")) and
  (.counterexamples | any(.kind == "insufficient_concepts")) and
  (.counterexamples | any(.kind == "missing_reproducible_command")) and
  (.counterexamples | any(.kind == "missing_evidence")) and
  (.counterexamples | any(.kind == "insufficient_translations")) and
  (.counterexamples | any(.kind == "missing_technical_terms")) and
  (.counterexamples | any(.kind == "missing_preserved_technical_token")) and
  (.counterexamples | any(.kind == "missing_equivalence_check")) and
  (.counterexamples | any(.kind == "missing_equivalence_quote")) and
  (.counterexamples | any(.kind == "missing_preserved_equivalence_token")) and
  (.counterexamples | any(.kind == "insufficient_accessibility_checks")) and
  (.counterexamples | any(.kind == "missing_required_accessibility_check")) and
  (.counterexamples | any(.kind == "missing_negative_control"))
' "$OUT/bad/localized-teaching-examples.json" > /dev/null

first_hash="$(jq -r '.hash' "$OUT/safe/localized-teaching-examples.json")"
go run ./cmd/patchline localized-teaching-examples \
  --spec "$SPEC" \
  --root . \
  --out "$OUT/repeat" \
  --json > "$OUT/repeat.stdout.json"
second_hash="$(jq -r '.hash' "$OUT/repeat/localized-teaching-examples.json")"
if [[ "$first_hash" != "$second_hash" ]]; then
  echo "FAIL: localized teaching hash is not deterministic" >&2
  exit 1
fi

jq -n --slurpfile safe "$OUT/safe/localized-teaching-examples.json" --slurpfile bad "$OUT/bad/localized-teaching-examples.json" '{
  version: "patchline.localized-teaching-examples-gate-results/v1",
  examples: $safe[0].summary.examples,
  translations: $safe[0].summary.translations,
  locales_covered: $safe[0].summary.locales_covered,
  technical_terms: $safe[0].summary.technical_terms,
  accessibility_checks: $safe[0].summary.accessibility_checks,
  bad_counterexamples: $bad[0].counterexamples,
  deterministic_hash: $safe[0].hash,
  verified: true
}' > "$OUT/gate-summary.json"

echo "localized teaching examples gate passed: translations preserve technical tokens, cite evidence, cover accessibility, and fail negative controls"
