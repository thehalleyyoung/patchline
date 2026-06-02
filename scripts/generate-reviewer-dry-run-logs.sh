#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/reviewer-dry-run-logs-gate.json}"
OUT="${2:-results/generated/reviewer-dry-run-logs}"
rm -rf "$OUT"
mkdir -p "$OUT/logs" "$OUT/evidence"

jq -e '
  .version == "patchline.reviewer-dry-run-logs-gate/v1" and
  (.claim | length) > 120 and
  (.dry_runs | length) >= .minimum_reviewers and
  ([.dry_runs[].events[] | select(.status == "failed")] | length) >= .minimum_failures and
  ([.dry_runs[].events[] | select(.fix)] | length) >= .minimum_fixes and
  all(.dry_runs[]; (.reviewer | test("^reviewer-[0-9]{3}$")) and (.time_budget_minutes >= 30) and (.events | length) >= 3)
' "$SPEC" > /dev/null

profile_spec="$(jq -r '.evidence_profile_spec' "$SPEC")"
bash scripts/artifact-container-rebuild.sh "$profile_spec" "$OUT/evidence/artifact-container-rebuild" > "$OUT/evidence/artifact-container-rebuild.run.log"

rebuild_summary="$OUT/evidence/artifact-container-rebuild/rebuild-summary.json"
test -s "$rebuild_summary"

run_rows=()
run_count="$(jq '.dry_runs | length' "$SPEC")"
for ((i=0; i<run_count; i++)); do
  reviewer="$(jq -r ".dry_runs[$i].reviewer" "$SPEC")"
  row="$OUT/logs/$reviewer.json"
  md="$OUT/logs/$reviewer.md"
  jq -n \
    --slurpfile spec "$SPEC" \
    --slurpfile rebuild "$rebuild_summary" \
    --argjson idx "$i" \
    '($spec[0].dry_runs[$idx]) as $run |
    {
      version:"patchline.reviewer-dry-run-log/v1",
      reviewer:$run.reviewer,
      anonymized:true,
      machine:$run.machine,
      time_budget_minutes:$run.time_budget_minutes,
      events:$run.events,
      summary:{
        failures:([$run.events[] | select(.status == "failed")] | length),
        fixes:([$run.events[] | select(.fix)] | length),
        passed:([$run.events[] | select(.status == "passed")] | length),
        final_public_repos:$rebuild[0].summary.public_repos,
        final_ranked_risks:$rebuild[0].summary.ranked_risks,
        final_generated_files:$rebuild[0].summary.generated_files,
        final_rejected_examples:$rebuild[0].summary.rejected_examples,
        final_verified:$rebuild[0].summary.verified
      }
    }' > "$row"
  {
    echo "# Reviewer dry-run log: $reviewer"
    echo
    echo "- anonymized: \`true\`"
    echo "- machine profile: \`$(jq -r '.machine' "$row")\`"
    echo "- time budget: \`$(jq -r '.time_budget_minutes' "$row") minutes\`"
    echo
    echo "## Setup failures and fixes"
    jq -r '.events[] | select(.status == "failed") | "- **" + .step + "** failed on `" + .command + "`: " + .failure + " Fix: " + .fix' "$row"
    echo
    echo "## Final regenerated results"
    jq -r '.summary | "- public repositories: `" + (.final_public_repos|tostring) + "`\n- ranked risks: `" + (.final_ranked_risks|tostring) + "`\n- generated files: `" + (.final_generated_files|tostring) + "`\n- rejected bad-output examples: `" + (.final_rejected_examples|tostring) + "`\n- verified: `" + (.final_verified|tostring) + "`"' "$row"
  } > "$md"
  run_rows+=("$row")
done

jq -n \
  --slurpfile runs <(jq -s '.' "${run_rows[@]}") \
  --slurpfile rebuild "$rebuild_summary" \
  --slurpfile spec "$SPEC" \
  '{
    version:"patchline.reviewer-dry-run-logs/v1",
    redaction_rules:$spec[0].redaction_rules,
    runs:$runs[0],
    evidence:{
      rebuild_summary:"evidence/artifact-container-rebuild/rebuild-summary.json",
      capstone_session:"evidence/artifact-container-rebuild/public-results/capstone/session.md",
      capstone_checksums:"evidence/artifact-container-rebuild/public-results/capstone/checksums.txt"
    },
    summary:{
      reviewers:($runs[0] | length),
      failures:([$runs[0][].summary.failures] | add),
      fixes:([$runs[0][].summary.fixes] | add),
      public_repos:$rebuild[0].summary.public_repos,
      ranked_risks:$rebuild[0].summary.ranked_risks,
      generated_files:$rebuild[0].summary.generated_files,
      rejected_examples:$rebuild[0].summary.rejected_examples,
      verified:($rebuild[0].summary.verified == true and all($runs[0][]; .anonymized == true and .summary.final_verified == true))
    }
  }' > "$OUT/reviewer-dry-run-logs.json"

{
  echo "# Anonymized reviewer dry-run logs"
  echo
  echo "These logs show fresh-machine setup failures, fixes, and final regenerated public-code results without reviewer identities, host paths, or credentials."
  echo
  echo "## Reviewer sessions"
  echo
  jq -r '.runs[] | "- `" + .reviewer + "` on `" + .machine + "`: failures `" + (.summary.failures|tostring) + "`, fixes `" + (.summary.fixes|tostring) + "`, final risks `" + (.summary.final_ranked_risks|tostring) + "`"' "$OUT/reviewer-dry-run-logs.json"
  echo
  echo "## Aggregate final regenerated results"
  echo
  jq -r '.summary | "- reviewers: `" + (.reviewers|tostring) + "`\n- setup failures: `" + (.failures|tostring) + "`\n- fixes: `" + (.fixes|tostring) + "`\n- public repositories: `" + (.public_repos|tostring) + "`\n- ranked risks: `" + (.ranked_risks|tostring) + "`\n- generated files: `" + (.generated_files|tostring) + "`\n- rejected bad-output examples: `" + (.rejected_examples|tostring) + "`"' "$OUT/reviewer-dry-run-logs.json"
  echo
  echo "## Evidence"
  echo
  echo "- \`evidence/artifact-container-rebuild/rebuild-summary.json\`"
  echo "- \`evidence/artifact-container-rebuild/public-results/capstone/session.md\`"
  echo "- \`evidence/artifact-container-rebuild/public-results/capstone/checksums.txt\`"
  echo
  echo "## Redaction tokens"
  echo
  jq -r '.redaction_rules.required_tokens[] | "- `" + . + "`"' "$OUT/reviewer-dry-run-logs.json"
} > "$OUT/index.md"

cp "$OUT/index.md" "$OUT/README.md"
echo "reviewer dry-run logs generated: reviewers $(jq '.summary.reviewers' "$OUT/reviewer-dry-run-logs.json"), failures $(jq '.summary.failures' "$OUT/reviewer-dry-run-logs.json"), risks $(jq '.summary.ranked_risks' "$OUT/reviewer-dry-run-logs.json")"
