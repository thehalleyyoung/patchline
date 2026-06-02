#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/certificate-interchange-language-gate.json}"
OUT="${2:-results/generated/certificate-interchange-language}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.certificate-interchange-language-gate/v1"
  and (.claim | length) > 300
  and (.checkers | length) == 4
  and (.vectors.valid >= 2)
  and (.vectors.invalid >= 4)
' "$SPEC" > /dev/null

SPEC_DIR="$(jq -r .spec_dir "$SPEC")"
GRAMMAR="$(jq -r .grammar "$SPEC")"
test -f "$GRAMMAR"
for phrase in "certificate interchange language" "make certificate-interchange-language-gate"; do
  grep -F "$phrase" docs/certificate-interchange-language.md README.md > /dev/null
done

go test ./internal/certlang > "$OUT/go-test.log"
go run ./tools/certlang/go-checker --spec-dir "$SPEC_DIR" --root . --json > "$OUT/go.json"
rustc tools/certlang/rust/check.rs -o "$OUT/rust-checker"
"$OUT/rust-checker" --spec-dir "$SPEC_DIR" --root . --json > "$OUT/rust.json"
python3 tools/certlang/python/check.py --spec-dir "$SPEC_DIR" --root . --json > "$OUT/python.json"
node --no-warnings --experimental-strip-types tools/certlang/typescript/check.ts --spec-dir "$SPEC_DIR" --root . --json > "$OUT/typescript.json"

for report in "$OUT/go.json" "$OUT/rust.json" "$OUT/python.json" "$OUT/typescript.json"; do
  jq -e --argjson valid "$(jq -r '.vectors.valid' "$SPEC")" --argjson invalid "$(jq -r '.vectors.invalid' "$SPEC")" '
    .version == "PLCI/1"
    and .all_ok == true
    and .total_valid == $valid
    and .total_invalid == $invalid
  ' "$report" > /dev/null
done

jq -n \
  --arg spec "$SPEC" \
  --arg grammar "$GRAMMAR" \
  --slurpfile go "$OUT/go.json" \
  --slurpfile rust "$OUT/rust.json" \
  --slurpfile python "$OUT/python.json" \
  --slurpfile typescript "$OUT/typescript.json" '
  def matrix_agreement($reports):
    ([ $reports[].vectors[].path ] | unique) as $paths
    | all($paths[]; . as $p | ([ $reports[].vectors[] | select(.path == $p) | .accepted ] | unique | length) == 1);
  [$go[0], $rust[0], $python[0], $typescript[0]] as $reports
  | matrix_agreement($reports) as $matrix
  | {
      version: "patchline.certificate-interchange-language-gate-results/v1",
      spec: $spec,
      grammar: $grammar,
      checkers: ($reports | map({
        checker,
        total_valid,
        total_invalid,
        accepted,
        rejected,
        all_ok
      })),
      matrix_agreement: $matrix,
      bad_rejected: (all($reports[].vectors[]; (.expected != "invalid") or (.accepted == false))),
      all_ok: ((all($reports[]; .all_ok == true)) and $matrix),
      verified: ((all($reports[]; .all_ok == true)) and $matrix)
    }
' > "$OUT/gate-summary.json"

jq -e '.verified == true and .bad_rejected == true and .matrix_agreement == true' "$OUT/gate-summary.json" > /dev/null
{
  echo "# Certificate interchange language gate"
  echo
  echo "PLCI/1 grammar: $GRAMMAR"
  echo "Checkers: Go, Rust, Python, TypeScript"
  echo "Matrix agreement: $(jq -r .matrix_agreement "$OUT/gate-summary.json")"
  echo "Verified: $(jq -r .verified "$OUT/gate-summary.json")"
} > "$OUT/README.md"

echo "certificate-interchange-language gate passed: PLCI/1 grammar vectors agree across Go, Rust, Python, and TypeScript"
