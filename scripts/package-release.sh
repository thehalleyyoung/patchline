#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

VERSION="${1:-v0.0.0-dev}"
OUT="${2:-dist/release-distribution}"
SEED_HEX="${3:-${PATCHLINE_RELEASE_SEED_HEX:-000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f}}"

rm -rf "$OUT"
mkdir -p "$OUT/staging" "$OUT/homebrew"

targets=(
  "Darwin amd64"
  "Darwin arm64"
  "Linux amd64"
  "Linux arm64"
)

artifacts=()
for target in "${targets[@]}"; do
  os="${target%% *}"
  arch="${target##* }"
  name="patchline_${os}_${arch}"
  stage="$OUT/staging/$name"
  mkdir -p "$stage"
  CGO_ENABLED=0 GOOS="$(tr '[:upper:]' '[:lower:]' <<< "$os")" GOARCH="$arch" GOFLAGS='-trimpath -buildvcs=false' \
    go build -ldflags='-buildid=' -o "$stage/patchline" ./cmd/patchline
  cp README.md "$stage/README.md"
  (cd "$stage" && tar -czf "../../$name.tar.gz" patchline README.md)
  artifacts+=("$OUT/$name.tar.gz")
done

checksum_cmd=(go run ./cmd/patchline release checksums --subject "patchline-release-$VERSION" --seed-hex "$SEED_HEX" --out "$OUT/checksums")
for artifact in "${artifacts[@]}"; do
  checksum_cmd+=(--artifact "$artifact")
done
"${checksum_cmd[@]}" > "$OUT/checksums-stdout.txt"

formula="$OUT/homebrew/patchline.rb"
cp packaging/homebrew/patchline.rb "$formula"
formula_version="${VERSION#v}"
darwin_amd64_sha="$(shasum -a 256 "$OUT/patchline_Darwin_amd64.tar.gz" | awk '{print $1}')"
darwin_arm64_sha="$(shasum -a 256 "$OUT/patchline_Darwin_arm64.tar.gz" | awk '{print $1}')"
linux_amd64_sha="$(shasum -a 256 "$OUT/patchline_Linux_amd64.tar.gz" | awk '{print $1}')"
linux_arm64_sha="$(shasum -a 256 "$OUT/patchline_Linux_arm64.tar.gz" | awk '{print $1}')"
sed -i.bak \
  -e "s/version \"0.0.0\"/version \"$formula_version\"/" \
  -e "s/releases\\/download\\/v0.0.0/releases\\/download\\/$VERSION/g" \
  -e "s/REPLACE_WITH_DARWIN_AMD64_SHA256/$darwin_amd64_sha/" \
  -e "s/REPLACE_WITH_DARWIN_ARM64_SHA256/$darwin_arm64_sha/" \
  -e "s/REPLACE_WITH_LINUX_AMD64_SHA256/$linux_amd64_sha/" \
  -e "s/REPLACE_WITH_LINUX_ARM64_SHA256/$linux_arm64_sha/" \
  "$formula"
rm -f "$formula.bak"

cat > "$OUT/install.md" <<EOF
# Patchline $VERSION install paths

## Go install

\`\`\`bash
go install github.com/thehalleyyoung/patchline/cmd/patchline@${VERSION}
\`\`\`

## GitHub Releases

\`\`\`bash
curl -L -o patchline.tar.gz https://github.com/thehalleyyoung/patchline/releases/download/${VERSION}/patchline_\$(uname -s)_\$(uname -m).tar.gz
tar -xzf patchline.tar.gz
./patchline --help
\`\`\`

## Homebrew

\`\`\`bash
brew tap thehalleyyoung/patchline
brew install patchline
\`\`\`

Formula candidate: \`homebrew/patchline.rb\`.

## Docker

\`\`\`bash
docker build -f packaging/docker/Dockerfile -t patchline:${VERSION} .
docker run --rm patchline:${VERSION} --help
\`\`\`
EOF

jq -n \
  --arg version "$VERSION" \
  --argjson archives "$(printf '%s\n' "${artifacts[@]##*/}" | jq -R . | jq -s .)" \
  '{
    version:"patchline.release-distribution/v1",
    release_version:$version,
    archives:$archives,
    install_paths:["go install","GitHub Releases","Homebrew","Docker"],
    homebrew_formula:"homebrew/patchline.rb",
    dockerfile:"packaging/docker/Dockerfile",
    github_release_workflow:".github/workflows/release.yml",
    checksums:"checksums/checksums.sha256",
    attestation:"checksums/checksums.attestation.json"
  }' > "$OUT/release-manifest.json"

echo "release distribution packaged: ${#artifacts[@]} archives in $OUT"
