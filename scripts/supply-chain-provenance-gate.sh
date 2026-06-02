#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/supply-chain-provenance-gate.json}"
OUT="${2:-results/generated/supply-chain-provenance-gate}"
rm -rf "$OUT"
mkdir -p "$OUT/bin" "$OUT/release" "$OUT/cache" "$OUT/public-corpus"

jq -e '
  .version == "patchline.supply-chain-provenance-gate/v1" and
  (.claim | length) > 100 and
  (.real_code.repo | length) > 0 and
  (.real_code.ref | test("^[0-9a-f]{40}$")) and
  (.real_code.subpath | length) > 0 and
  (.required_kinds | sort) == ["binary","generated_experiment_artifact","public_corpus_download","release_archive"]
' "$SPEC" > /dev/null

grep -F "patchline supply-chain provenance" docs/supply-chain-provenance.md > /dev/null
grep -F "download-provenance.json" docs/supply-chain-provenance.md > /dev/null
grep -F "make supply-chain-provenance-gate" README.md > /dev/null

go test ./cmd/patchline -run TestSupplyChainProvenanceCommandCoversRequiredArtifactKinds > "$OUT/go-test.log"

go build -o "$OUT/bin/patchline" ./cmd/patchline
tar -czf "$OUT/release/patchline-local.tar.gz" -C "$OUT/bin" patchline

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
  --out "$OUT/experiment/analyze" \
  --json > "$OUT/experiment-stdout.json"

PATCHLINE_PUBLIC_CORPUS_OUT="$OUT/public-corpus/downloads" \
PATCHLINE_PUBLIC_CORPUS_REPORT_DIR="$OUT/public-corpus/report" \
  bash scripts/fetch-public-corpus.sh > "$OUT/public-corpus-fetch.log"

go run ./cmd/patchline supply-chain provenance \
  --subject "patchline-supply-chain-gate" \
  --source "repo=$repo@$ref:$subpath" \
  --source "public-corpus=examples/public-corpus/sources.json" \
  --command "go build -o $OUT/bin/patchline ./cmd/patchline" \
  --command "go run ./cmd/patchline repo analyze --github $repo --ref $ref --subpath $subpath --stages inventory,baseline,propose,compare --no-llm" \
  --command "PATCHLINE_PUBLIC_CORPUS_OUT=$OUT/public-corpus/downloads bash scripts/fetch-public-corpus.sh" \
  --artifact "binary=$OUT/bin/patchline" \
  --artifact "release_archive=$OUT/release/patchline-local.tar.gz" \
  --artifact "generated_experiment_artifact=$OUT/experiment/analyze" \
  --artifact "public_corpus_download=$OUT/public-corpus/downloads" \
  --out "$OUT/supply-chain-provenance.json" \
  --json > "$OUT/supply-chain-provenance.stdout.json"

jq -e --slurpfile spec "$SPEC" '
  . as $report |
  .version == "patchline.supply-chain-provenance/v1" and
  .verification.complete == true and
  (.verification.missing_kinds | length) == 0 and
  (.summary.artifacts == 4) and
  (.summary.binaries == 1) and
  (.summary.release_archives == 1) and
  (.summary.generated_experiment_artifacts == 1) and
  (.summary.public_corpus_downloads == 1) and
  (.summary.bytes > 0) and
  (.report_hash | length) > 0 and
  all($spec[0].required_kinds[]; . as $kind | any($report.artifacts[]; .kind == $kind))
' "$OUT/supply-chain-provenance.json" > /dev/null

jq -e '
  .version == "patchline.supply-chain-provenance/v1" and
  .verification.complete == false and
  (.summary.public_corpus_downloads == 1) and
  any(.artifacts[]; .kind == "public_corpus_download" and .files >= 5)
' "$OUT/public-corpus/report/download-provenance.json" > /dev/null

jq -n \
  --slurpfile provenance "$OUT/supply-chain-provenance.json" \
  --slurpfile corpus "$OUT/public-corpus/report/download-provenance.json" \
  --slurpfile analyze "$OUT/experiment/analyze/analyze.json" \
  '{
    version:"patchline.supply-chain-provenance-gate-results/v1",
    subject:$provenance[0].subject,
    artifacts:$provenance[0].summary.artifacts,
    bytes:$provenance[0].summary.bytes,
    required_complete:$provenance[0].verification.complete,
    report_hash:$provenance[0].report_hash,
    corpus_download_files:$corpus[0].artifacts[0].files,
    real_code_files_scanned:$analyze[0].summary.files_scanned,
    real_code_ranked_risks:$analyze[0].summary.ranked_risks,
    verified:true
  }' > "$OUT/summary.json"

jq -e '.verified == true and .required_complete == true and .artifacts == 4 and .corpus_download_files >= 5 and .real_code_ranked_risks > 0' "$OUT/summary.json" > /dev/null

echo "supply-chain provenance gate passed: $(jq '.artifacts' "$OUT/summary.json") artifact classes, corpus files $(jq '.corpus_download_files' "$OUT/summary.json"), real risks $(jq '.real_code_ranked_risks' "$OUT/summary.json")"
