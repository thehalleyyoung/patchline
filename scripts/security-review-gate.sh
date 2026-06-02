#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/security-review-gate.json}"
OUT="${2:-results/generated/security-review-gate}"
rm -rf "$OUT"
mkdir -p "$OUT/cache"

jq -e '
  .version == "patchline.security-review-gate/v1" and
  (.claim | length) > 100 and
  (.protected_surfaces | length) == 4 and
  (.required_gates | length) >= 8 and
  (.real_code.repo | length) > 0 and
  (.real_code.ref | test("^[0-9a-f]{40}$")) and
  (.real_code.subpath | length) > 0 and
  (.example_changed_files | length) >= 4
' "$SPEC" > /dev/null

while IFS= read -r surface; do
  grep -F "$surface" docs/security-review-gates.md > /dev/null
done < <(jq -r '.protected_surfaces[]' "$SPEC")

while IFS= read -r gate; do
  grep -F "$gate" docs/security-review-gates.md > /dev/null
done < <(jq -r '.required_gates[]' "$SPEC")
grep -F "make security-review-gate" README.md > /dev/null

go test ./cmd/patchline -run 'TestSecurityReview' > "$OUT/go-test.log"

changed="$(jq -r '.example_changed_files | join(",")' "$SPEC")"
if go run ./cmd/patchline security review \
  --changed-files "$changed" \
  --passed-gates threat-model-gate \
  --out "$OUT/blocked" \
  --json > "$OUT/blocked-stdout.json" 2> "$OUT/blocked-stderr.txt"; then
  echo "security review unexpectedly passed with missing proof gates" >&2
  exit 1
fi

jq -e '
  .version == "patchline.security-review/v1" and
  .summary.success == false and
  .summary.protected_surfaces == 4 and
  .summary.blocked_surfaces == 4 and
  .summary.missing_gates > 0
' "$OUT/blocked/security-review.json" > /dev/null

passed="$(jq -r '.required_gates | join(",")' "$SPEC")"
go run ./cmd/patchline security review \
  --changed-files "$changed" \
  --passed-gates "$passed" \
  --out "$OUT/passed" \
  --json > "$OUT/passed-stdout.json"

jq -e '
  .version == "patchline.security-review/v1" and
  .summary.success == true and
  .summary.protected_surfaces == 4 and
  .summary.blocked_surfaces == 0 and
  .summary.missing_gates == 0 and
  (.surfaces | length) == 4
' "$OUT/passed/security-review.json" > /dev/null

read -r repo ref subpath < <(jq -r '[.real_code.repo, .real_code.ref, .real_code.subpath] | @tsv' "$SPEC")
go run ./cmd/patchline repo analyze \
  --github "$repo" \
  --ref "$ref" \
  --subpath "$subpath" \
  --download-dir "$OUT/cache" \
  --stages inventory,baseline,propose,compare \
  --proposal-kind all \
  --budget files=3,lines=70,tokens=10000,changes=2 \
  --no-llm \
  --out "$OUT/analyze" \
  --json > "$OUT/analyze-stdout.json"

jq -e '
  .version == "patchline.repo-compare/v1" and
  .quarantine.status == "enforced" and
  .intervention_loop.proposal_stage == "generated-untrusted" and
  (.generated_checks | length) > 0
' "$OUT/analyze/compare/compare.json" > /dev/null

grep -F "Patchline security review" "$OUT/passed/security-review.md" > /dev/null
grep -F "archive-security-gate" "$OUT/passed/security-review.md" > /dev/null

jq -n \
  --slurpfile review "$OUT/passed/security-review.json" \
  --slurpfile compare "$OUT/analyze/compare/compare.json" \
  '{
    version:"patchline.security-review-gate-results/v1",
    protected_surfaces:$review[0].summary.protected_surfaces,
    required_gates:$review[0].summary.required_gates,
    missing_gates:$review[0].summary.missing_gates,
    real_generated_checks:($compare[0].generated_checks | length),
    real_quarantine:$compare[0].quarantine.status,
    verified:true
  }' > "$OUT/summary.json"

jq -e '.verified == true and .protected_surfaces == 4 and .missing_gates == 0 and .real_generated_checks > 0 and .real_quarantine == "enforced"' "$OUT/summary.json" > /dev/null

echo "security review gate passed: surfaces $(jq '.protected_surfaces' "$OUT/summary.json"), required gates $(jq '.required_gates' "$OUT/summary.json")"
