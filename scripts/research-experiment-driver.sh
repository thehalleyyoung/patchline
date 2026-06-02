#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SPEC="${1:-examples/research-experiment-driver.json}"
OUT="${2:-results/generated/research-experiment-driver}"
rm -rf "$OUT"
mkdir -p "$OUT"

cd "$ROOT"
jq -e '.version == "patchline.research-experiment-driver/v1" and .driver.clean_checkout == true and (.driver.ledger_claim | contains("output hashes"))' "$SPEC" > /dev/null

commit="$(git rev-parse HEAD)"
checkout="$OUT/checkout"
ledger="$OUT/ledger"
mkdir -p "$ledger"
git worktree add --detach "$checkout" "$commit" > "$OUT/worktree-add.log"
cleanup() {
  cd "$ROOT"
  git worktree remove --force "$checkout" > /dev/null 2>&1 || true
}
trap cleanup EXIT

dirty="$(cd "$checkout" && git status --short)"
if [ -n "$dirty" ]; then
  echo "clean checkout is dirty: $dirty" >&2
  exit 1
fi

(cd "$checkout" && bash scripts/research-question-gate.sh examples/research-questions.json "$ROOT/$ledger/research-questions") > "$OUT/research-question-gate.log"

(
  cd "$ledger"
  find . -type f \( -name '*.json' -o -name '*.jsonl' -o -name '*.md' -o -name '*.sarif' -o -name '*.log' \) -print | LC_ALL=C sort | while read -r file; do
    shasum -a 256 "$file"
  done
) > "$OUT/result-checksums.sha256"

jq -n \
  --arg commit "$commit" \
  --arg checkout "$checkout" \
  --arg ledger "$ledger" \
  --arg checksums "result-checksums.sha256" \
  --slurpfile rq "$ledger/research-questions/summary.json" \
  '{
    version:"patchline.research-experiment-ledger/v1",
    commit:$commit,
    clean_checkout:true,
    checkout_path:$checkout,
    ledger_path:$ledger,
    immutable:true,
    checksums:$checksums,
    research_question_summary:$rq[0],
    output_hash:""
  }' /dev/null > "$OUT/experiment-ledger.json"

# Replace placeholder output_hash with a deterministic hash of the checksum ledger.
ledger_hash="$(shasum -a 256 "$OUT/result-checksums.sha256" | awk '{print $1}')"
jq --arg hash "sha256:$ledger_hash" '.output_hash = $hash' "$OUT/experiment-ledger.json" > "$OUT/experiment-ledger.tmp"
mv "$OUT/experiment-ledger.tmp" "$OUT/experiment-ledger.json"

echo "research experiment driver completed for commit $commit"
