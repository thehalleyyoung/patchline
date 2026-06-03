#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

OUT="${1:-results/generated/reproducibility-chaos}"
rm -rf "$OUT"
mkdir -p "$OUT"

for phrase in "Reproducibility chaos" "make reproducibility-chaos-gate"; do
  grep -F "$phrase" docs/reproducibility-chaos.md README.md > /dev/null
done

for phrase in "cache metadata" "archive mirrors" "optional VCS tools"; do
  grep -F "$phrase" docs/reproducibility-chaos.md > /dev/null
done

go test ./internal/project -run '^(TestFetchArchiveURLChaosRecoversFromRandomCacheDeletion|TestFetchLocalVCSChaosSurvivesMissingOptionalTools)$' -count=1
go test ./internal/evidencemarketplace -run '^TestWriteReportChaosRegeneratesDeletedMirrorFiles$' -count=1

jq -n '{
  version: "patchline.reproducibility-chaos-gate-results/v1",
  verified: true,
  seed: 780,
  chaos_targets: ["archive cache metadata", "archive cache bytes", "archive mirrors", "optional VCS tools"],
  tests: [
    "internal/project.TestFetchArchiveURLChaosRecoversFromRandomCacheDeletion",
    "internal/project.TestFetchLocalVCSChaosSurvivesMissingOptionalTools",
    "internal/evidencemarketplace.TestWriteReportChaosRegeneratesDeletedMirrorFiles"
  ]
}' > "$OUT/gate-summary.json"

echo "reproducibility-chaos gate passed: cache, mirror, and optional-tool removals degrade safely"
