#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/certificate-migration-gate.json}"
OUT="${2:-results/generated/certificate-migration}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.certificate-migration-gate/v1"
  and (.claim | length) > 300
  and .source_version == "PLCI/0"
  and .target_version == "PLCI/1"
  and .legacy_valid_vectors == 4
  and .legacy_invalid_vectors >= 2
' "$SPEC" > /dev/null

SPEC_DIR="$(jq -r .spec_dir "$SPEC")"
LEGACY_GRAMMAR="$(jq -r .legacy_grammar "$SPEC")"
test -d "$SPEC_DIR"
test -f "$LEGACY_GRAMMAR"

for phrase in "backward-compatible certificate migration" "make certificate-migration-gate"; do
  grep -F "$phrase" docs/certificate-migration.md README.md > /dev/null
done

go test ./internal/certlang -run 'TestLegacy|TestCurrentCertificateMigration' > "$OUT/go-test.log"
go run ./tools/certmigration --spec-dir "$SPEC_DIR" --root . --json > "$OUT/report.json"

jq -e '
  .version == "PLCI/1-migration-results"
  and .source_version == "PLCI/0"
  and .target_version == "PLCI/1"
  and .all_ok == true
  and .total_legacy_valid == 4
  and .total_legacy_invalid >= 2
  and .migrated == .total_legacy_valid
  and .rejected == .total_legacy_invalid
  and .verdicts.safe >= 1
  and .verdicts.guarded >= 1
  and .verdicts.blocked >= 1
  and .verdicts.unsupported >= 1
  and all(.vectors[] | select(.expected == "legacy-valid"); .accepted == true and .source_sha256 != .target_sha256 and .source_canonical_sha256 != .target_canonical_sha256)
  and all(.vectors[] | select(.expected == "legacy-invalid"); .accepted == false)
' "$OUT/report.json" > /dev/null

jq -n \
  --arg spec "$SPEC" \
  --arg legacy_grammar "$LEGACY_GRAMMAR" \
  --slurpfile report "$OUT/report.json" '
  {
    version: "patchline.certificate-migration-gate-results/v1",
    spec: $spec,
    legacy_grammar: $legacy_grammar,
    source_version: $report[0].source_version,
    target_version: $report[0].target_version,
    migrated: $report[0].migrated,
    rejected: $report[0].rejected,
    verdicts: $report[0].verdicts,
    verified: $report[0].all_ok
  }
' > "$OUT/gate-summary.json"

echo "certificate-migration gate passed: PLCI/0 verdicts migrate to checkable PLCI/1 certificates across all verdict classes"
