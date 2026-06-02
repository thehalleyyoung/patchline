#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/reviewer-dry-run-logs-gate.json}"
OUT="${2:-results/generated/reviewer-dry-run-logs-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '
  .version == "patchline.reviewer-dry-run-logs-gate/v1" and
  (.dry_runs | length) >= .minimum_reviewers and
  (.redaction_rules.forbidden_patterns | length) >= 6 and
  (.redaction_rules.required_tokens | length) >= 5
' "$SPEC" > /dev/null

for phrase in "anonymized reviewer dry-run logs" "fresh-machine setup failures" "fixes" "final regenerated results" "make reviewer-dry-run-logs-gate"; do
  grep -F "$phrase" docs/reviewer-dry-run-logs.md README.md > /dev/null
done

bash scripts/generate-reviewer-dry-run-logs.sh "$SPEC" "$OUT" > "$OUT.run.log"

min_reviewers="$(jq '.minimum_reviewers' "$SPEC")"
min_failures="$(jq '.minimum_failures' "$SPEC")"
min_fixes="$(jq '.minimum_fixes' "$SPEC")"
min_repos="$(jq '.minimum_public_repos' "$SPEC")"
min_risks="$(jq '.minimum_ranked_risks' "$SPEC")"
jq -e --argjson min_reviewers "$min_reviewers" --argjson min_failures "$min_failures" --argjson min_fixes "$min_fixes" --argjson min_repos "$min_repos" --argjson min_risks "$min_risks" '
  .version == "patchline.reviewer-dry-run-logs/v1" and
  .summary.reviewers >= $min_reviewers and
  .summary.failures >= $min_failures and
  .summary.fixes >= $min_fixes and
  .summary.public_repos >= $min_repos and
  .summary.ranked_risks >= $min_risks and
  .summary.generated_files > 0 and
  .summary.rejected_examples >= 2 and
  .summary.verified == true and
  all(.runs[]; .anonymized == true and (.events | length) >= 3 and .summary.final_verified == true)
' "$OUT/reviewer-dry-run-logs.json" > /dev/null

while read -r token; do
  grep -R -F "$token" "$OUT/index.md" "$OUT/logs" > /dev/null
done < <(jq -r '.redaction_rules.required_tokens[]' "$SPEC")

if grep -R -nE '([A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+|/Users/[^ ]+|/home/[^ ]+|ghp_[A-Za-z0-9_]+|github_pat_[A-Za-z0-9_]+|BEGIN [A-Z ]*PRIVATE KEY)' "$OUT/index.md" "$OUT/logs"; then
  echo "reviewer dry-run logs contain non-anonymized sensitive material" >&2
  exit 1
fi

for reviewer in reviewer-001 reviewer-002 reviewer-003; do
  test -s "$OUT/logs/$reviewer.json"
  test -s "$OUT/logs/$reviewer.md"
  grep -F "Setup failures and fixes" "$OUT/logs/$reviewer.md" > /dev/null
  grep -F "Final regenerated results" "$OUT/logs/$reviewer.md" > /dev/null
done

grep -F "Patchline release-quality capstone demo" "$OUT/evidence/artifact-container-rebuild/public-results/capstone/session.md" > /dev/null
test -s "$OUT/evidence/artifact-container-rebuild/public-results/capstone/checksums.txt"
grep -F "Aggregate final regenerated results" "$OUT/index.md" > /dev/null

jq -n \
  --slurpfile logs "$OUT/reviewer-dry-run-logs.json" \
  '{
    version:"patchline.reviewer-dry-run-logs-gate-results/v1",
    reviewers:$logs[0].summary.reviewers,
    failures:$logs[0].summary.failures,
    fixes:$logs[0].summary.fixes,
    public_repos:$logs[0].summary.public_repos,
    ranked_risks:$logs[0].summary.ranked_risks,
    verified:true
  }' > "$OUT/gate-summary.json"

echo "reviewer dry-run logs gate passed: reviewers $(jq '.reviewers' "$OUT/gate-summary.json"), failures $(jq '.failures' "$OUT/gate-summary.json"), risks $(jq '.ranked_risks' "$OUT/gate-summary.json")"
