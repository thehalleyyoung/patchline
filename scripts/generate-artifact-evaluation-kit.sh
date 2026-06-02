#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/artifact-evaluation-kit-gate.json}"
OUT="${2:-results/generated/artifact-evaluation-kit}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.artifact-evaluation-kit-gate/v1" and
  (.claim | length) > 140 and
  (.roles | length) >= .minimum_roles and
  (.expected_outputs | length) >= .minimum_expected_outputs and
  (.pass_fail_criteria | length) >= .minimum_pass_fail_criteria and
  (.capstone_spec | startswith("examples/")) and
  all(.roles[]; (.id | test("^[a-z0-9-]+$")) and (.title | length) > 5 and (.time_budget_minutes >= 15) and (.goal | length) > 40) and
  all(.pass_fail_criteria[]; (.id | test("^[a-z0-9-]+$")) and (.description | length) > 30)
' "$SPEC" > /dev/null

capstone_spec="$(jq -r '.capstone_spec' "$SPEC")"
bash scripts/capstone-demo.sh "$capstone_spec" "$OUT/capstone" > "$OUT/capstone.run.log"

jq '.roles' "$SPEC" > "$OUT/roles.json"
jq '.expected_outputs' "$SPEC" > "$OUT/expected-outputs.json"
jq '.pass_fail_criteria' "$SPEC" > "$OUT/pass-fail-criteria.json"

jq -n \
  --slurpfile roles "$OUT/roles.json" \
  --slurpfile expected "$OUT/expected-outputs.json" \
  --slurpfile criteria "$OUT/pass-fail-criteria.json" \
  --slurpfile capstone "$OUT/capstone/summary.json" \
  '{
    version:"patchline.artifact-evaluation-kit/v1",
    roles:$roles[0],
    expected_outputs:$expected[0],
    pass_fail_criteria:$criteria[0],
    capstone_summary:$capstone[0].summary,
    summary:{
      roles:($roles[0] | length),
      total_time_budget_minutes:([$roles[0][].time_budget_minutes] | add),
      expected_outputs:($expected[0] | length),
      pass_fail_criteria:($criteria[0] | length),
      public_repos:$capstone[0].summary.public_repos,
      ranked_risks:$capstone[0].summary.ranked_risks,
      generated_files:$capstone[0].summary.generated_files,
      rejected_examples:$capstone[0].summary.rejected_examples,
      evidence_artifact_types:($capstone[0].summary.evidence_artifacts | keys | length),
      verified:($capstone[0].summary.verified == true)
    }
  }' > "$OUT/artifact-evaluation-kit.json"

{
  echo "# Patchline artifact-evaluation landing kit"
  echo
  echo "This landing kit gives reviewers concrete roles, time budgets, expected outputs, and pass/fail criteria backed by regenerated public-code capstone evidence."
  echo
  echo "## Reviewer roles and time budgets"
  echo
  echo "| Role | Time budget | Goal |"
  echo "| --- | ---: | --- |"
  jq -r '.roles[] | "| " + .title + " | " + (.time_budget_minutes|tostring) + " minutes | " + .goal + " |"' "$OUT/artifact-evaluation-kit.json"
  echo
  echo "## Expected outputs"
  echo
  jq -r '.expected_outputs[] | "- `" + . + "`"' "$OUT/artifact-evaluation-kit.json"
  echo
  echo "## Pass/fail criteria"
  echo
  jq -r '.pass_fail_criteria[] | "- `" + .id + "`: " + .description' "$OUT/artifact-evaluation-kit.json"
  echo
  echo "## Regenerated public-code evidence"
  echo
  jq -r '.summary | "- public repositories: `" + (.public_repos|tostring) + "`\n- ranked risks: `" + (.ranked_risks|tostring) + "`\n- generated artifacts: `" + (.generated_files|tostring) + "`\n- rejected bad-output examples: `" + (.rejected_examples|tostring) + "`\n- evidence artifact types: `" + (.evidence_artifact_types|tostring) + "`"' "$OUT/artifact-evaluation-kit.json"
} > "$OUT/landing.md"

{
  echo "# Artifact reviewer guide"
  echo
  echo "1. Start with \`landing.md\` and choose the role matching your review depth."
  echo "2. Run or inspect \`capstone/session.md\` to confirm the public-code session."
  echo "3. Check \`expected-outputs.json\` and verify every expected output exists."
  echo "4. Apply \`pass-fail-criteria.json\` before trusting claims in papers, docs, or demos."
  echo "5. Use \`capstone/checksums.txt\` to confirm generated evidence did not drift during review."
} > "$OUT/reviewer-guide.md"

cp "$OUT/landing.md" "$OUT/README.md"
echo "artifact evaluation kit generated: roles $(jq '.summary.roles' "$OUT/artifact-evaluation-kit.json"), outputs $(jq '.summary.expected_outputs' "$OUT/artifact-evaluation-kit.json"), criteria $(jq '.summary.pass_fail_criteria' "$OUT/artifact-evaluation-kit.json")"
