#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/generated-code-quarantine-gate.json}"
OUT="${2:-results/generated/generated-code-quarantine-gate}"
rm -rf "$OUT"
mkdir -p "$OUT/cache"

jq -e '
  .version == "patchline.generated-code-quarantine-gate/v1" and
  (.claim | length) > 100 and
  (.focused_tests | length) >= 3 and
  (.real_code.repo | length) > 0 and
  (.real_code.ref | test("^[0-9a-f]{40}$")) and
  (.real_code.subpath | length) > 0 and
  (.required_controls | length) >= 5
' "$SPEC" > /dev/null

while IFS= read -r control; do
  grep -F "$control" docs/generated-code-quarantine.md > /dev/null
done < <(jq -r '.required_controls[]' "$SPEC")
grep -F "make generated-code-quarantine-gate" README.md > /dev/null

go test ./internal/project -run 'Test(GeneratedCodeQuarantineSkipsNativeChecksByDefault|WriteProposalForcesGeneratedArtifactsNonExecutable|CompareRunsSafeNativeChecks)$' > "$OUT/go-test.log"

read -r repo ref subpath < <(jq -r '[.real_code.repo, .real_code.ref, .real_code.subpath] | @tsv' "$SPEC")
go run ./cmd/patchline repo analyze \
  --github "$repo" \
  --ref "$ref" \
  --subpath "$subpath" \
  --download-dir "$OUT/cache" \
  --stages inventory,baseline,propose,compare \
  --proposal-kind all \
  --budget files=4,lines=80,tokens=12000,changes=2 \
  --no-llm \
  --out "$OUT/analyze-default" \
  --json > "$OUT/analyze-default.json"

jq -e '
  .quarantine.status == "enforced" and
  .quarantine.trust == "untrusted-generated-proposal" and
  .quarantine.generated_artifacts_executable == false and
  .quarantine.generated_artifacts_applied == false and
  .quarantine.native_checks_require_opt_in == true and
  .quarantine.safe_native_checks_enabled == false and
  .quarantine.required_flag == "--run-native-tests" and
  (.quarantine.quarantined_paths | length) > 0 and
  .summary.native_checks_run == 0
' "$OUT/analyze-default/compare/compare.json" > /dev/null

jq -e '
  .quarantine.status == "enforced" and
  .quarantine.generated_artifacts_executable == false and
  .quarantine.generated_artifacts_applied == false and
  .quarantine.native_checks_require_opt_in == true and
  (.quarantine.quarantined_paths | length) > 0
' "$OUT/analyze-default/proposal/proposal.json" > /dev/null

while IFS= read -r rel; do
  path="$OUT/analyze-default/proposal/$rel"
  test -f "$path"
  mode="$(stat -f '%Lp' "$path" 2>/dev/null || stat -c '%a' "$path")"
  if (( (8#$mode & 0111) != 0 )); then
    echo "generated artifact is executable: $path mode=$mode" >&2
    exit 1
  fi
done < <(jq -r '.quarantine.quarantined_paths[]' "$OUT/analyze-default/proposal/proposal.json")

go run ./cmd/patchline repo compare \
  --before "$OUT/analyze-default/baseline" \
  --after "$OUT/analyze-default/proposal" \
  --run-native-tests \
  --out "$OUT/compare-explicit" \
  --json > "$OUT/compare-explicit.json"

jq -e '
  .quarantine.status == "enforced" and
  .quarantine.safe_native_checks_enabled == true and
  .quarantine.native_execution_mode == "safe-native-checks-enabled" and
  .quarantine.generated_artifacts_executable == false and
  .quarantine.generated_artifacts_applied == false
' "$OUT/compare-explicit/compare.json" > /dev/null

grep -F "Generated-code quarantine" "$OUT/analyze-default/compare/compare.md" > /dev/null
grep -F "generated artifacts executable: \`false\`" "$OUT/analyze-default/compare/compare.md" > /dev/null

jq -n \
  --slurpfile proposal "$OUT/analyze-default/proposal/proposal.json" \
  --slurpfile defaultCompare "$OUT/analyze-default/compare/compare.json" \
  --slurpfile explicitCompare "$OUT/compare-explicit/compare.json" \
  '{
    version:"patchline.generated-code-quarantine-gate-results/v1",
    generated_files:($proposal[0].generated_files | length),
    quarantined_paths:($proposal[0].quarantine.quarantined_paths | length),
    default_safe_native_checks_enabled:$defaultCompare[0].quarantine.safe_native_checks_enabled,
    explicit_safe_native_checks_enabled:$explicitCompare[0].quarantine.safe_native_checks_enabled,
    generated_artifacts_executable:$defaultCompare[0].quarantine.generated_artifacts_executable,
    generated_artifacts_applied:$defaultCompare[0].quarantine.generated_artifacts_applied,
    native_checks_require_opt_in:$defaultCompare[0].quarantine.native_checks_require_opt_in,
    verified:true
  }' > "$OUT/summary.json"

jq -e '.verified == true and .generated_files > 0 and .quarantined_paths == .generated_files and .default_safe_native_checks_enabled == false and .explicit_safe_native_checks_enabled == true and .generated_artifacts_executable == false and .generated_artifacts_applied == false and .native_checks_require_opt_in == true' "$OUT/summary.json" > /dev/null

echo "generated-code quarantine gate passed: generated $(jq '.generated_files' "$OUT/summary.json"), quarantined $(jq '.quarantined_paths' "$OUT/summary.json")"
