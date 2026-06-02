#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/release-distribution-gate.json}"
OUT="${2:-results/generated/release-distribution-gate}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.release-distribution-gate/v1" and
  (.claim | length) > 150 and
  (.version_tag | test("^v")) and
  (.seed_hex | test("^[0-9a-f]{64}$")) and
  (.required_archives | length) == 4 and
  (.required_install_paths | length) == 4 and
  (.real_code.repo | length) > 0 and
  (.real_code.ref | test("^[0-9a-f]{40}$")) and
  (.real_code.subpath | length) > 0
' "$SPEC" > /dev/null

for phrase in "Go install" "GitHub Releases" "Homebrew" "Docker" "make release-distribution-gate"; do
  grep -F "$phrase" docs/release-distribution.md README.md > /dev/null
done
grep -F "packaging/docker/Dockerfile" docs/release-distribution.md > /dev/null
grep -F "softprops/action-gh-release" .github/workflows/release.yml > /dev/null
grep -F "class Patchline < Formula" packaging/homebrew/patchline.rb > /dev/null
grep -F "FROM gcr.io/distroless/static-debian12:nonroot" packaging/docker/Dockerfile > /dev/null

version="$(jq -r '.version_tag' "$SPEC")"
seed="$(jq -r '.seed_hex' "$SPEC")"
bash scripts/package-release.sh "$version" "$OUT/dist" "$seed" > "$OUT/package.log"

while read -r archive; do
  test -s "$OUT/dist/$archive"
  tar -tzf "$OUT/dist/$archive" | grep -F "patchline" > /dev/null
done < <(jq -r '.required_archives[]' "$SPEC")

jq -e '
  .version == "patchline.release-distribution/v1" and
  (.archives | length) == 4 and
  (.install_paths | index("go install")) != null and
  (.install_paths | index("GitHub Releases")) != null and
  (.install_paths | index("Homebrew")) != null and
  (.install_paths | index("Docker")) != null
' "$OUT/dist/release-manifest.json" > /dev/null

jq -e '.signature_verified == true and (.artifacts | length) == 4' "$OUT/dist/checksums/release-checksums.json" > /dev/null
test -s "$OUT/dist/checksums/checksums.sha256"
test -s "$OUT/dist/checksums/checksums.attestation.json"
grep -F "go install github.com/thehalleyyoung/patchline/cmd/patchline" "$OUT/dist/install.md" > /dev/null
grep -F "GitHub Releases" "$OUT/dist/install.md" > /dev/null
grep -F "brew install patchline" "$OUT/dist/install.md" > /dev/null
grep -F "docker build -f packaging/docker/Dockerfile" "$OUT/dist/install.md" > /dev/null
grep -F "version \"${version#v}\"" "$OUT/dist/homebrew/patchline.rb" > /dev/null
grep -F "releases/download/$version" "$OUT/dist/homebrew/patchline.rb" > /dev/null
if grep -F "REPLACE_WITH" "$OUT/dist/homebrew/patchline.rb" > /dev/null; then
  echo "Homebrew formula still contains placeholder checksums" >&2
  exit 1
fi

host_os="$(uname -s)"
host_arch="$(uname -m)"
case "$host_arch" in
  x86_64) host_arch="amd64" ;;
  aarch64|arm64) host_arch="arm64" ;;
esac
host_archive="$OUT/dist/patchline_${host_os}_${host_arch}.tar.gz"
test -s "$host_archive"
mkdir -p "$OUT/host-bin"
tar -xzf "$host_archive" -C "$OUT/host-bin"
"$OUT/host-bin/patchline" --help > "$OUT/host-help.txt"

repo="$(jq -r '.real_code.repo' "$SPEC")"
ref="$(jq -r '.real_code.ref' "$SPEC")"
subpath="$(jq -r '.real_code.subpath' "$SPEC")"
min_risks="$(jq '.real_code.minimum_ranked_risks' "$SPEC")"
"$OUT/host-bin/patchline" repo analyze \
  --github "$repo" \
  --ref "$ref" \
  --subpath "$subpath" \
  --download-dir "$OUT/cache" \
  --stages inventory,baseline,propose,compare \
  --proposal-kind all \
  --budget files=4,lines=80,tokens=8000,changes=1 \
  --no-llm \
  --out "$OUT/host-analysis" \
  --json > "$OUT/host-analysis-stdout.json"

jq -e --argjson min_risks "$min_risks" '.summary.ranked_risks >= $min_risks and .summary.generated_files > 0 and .summary.deterministic_only == true' "$OUT/host-analysis/analyze.json" > /dev/null

jq -n \
  --slurpfile manifest "$OUT/dist/release-manifest.json" \
  --slurpfile analysis "$OUT/host-analysis/analyze.json" \
  '{
    version:"patchline.release-distribution-gate-results/v1",
    archives:($manifest[0].archives | length),
    install_paths:($manifest[0].install_paths | length),
    host_ranked_risks:$analysis[0].summary.ranked_risks,
    host_generated_files:$analysis[0].summary.generated_files,
    verified:true
  }' > "$OUT/summary.json"

echo "release distribution gate passed: archives $(jq '.archives' "$OUT/summary.json"), install paths $(jq '.install_paths' "$OUT/summary.json"), host risks $(jq '.host_ranked_risks' "$OUT/summary.json")"
