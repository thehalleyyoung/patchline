#!/usr/bin/env bash
# new-ecosystem.sh — scaffold a new Patchline ecosystem gate in minutes.
# Usage: scripts/new-ecosystem.sh <name> [dest-dir]
# Creates an example spec, a worker script, a gate script, and a doc stub following the
# established gate pattern so a contributor can add a new ecosystem in under one hour.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
NAME="${1:?usage: new-ecosystem.sh <name> [dest-dir]}"
DEST="${2:-$ROOT}"

# Normalize the name to a safe kebab-case slug.
slug="$(printf '%s' "$NAME" | tr '[:upper:] _' '[:lower:]--' | tr -cd 'a-z0-9-')"
if [ -z "$slug" ]; then echo "invalid ecosystem name" >&2; exit 1; fi

mkdir -p "$DEST/examples" "$DEST/scripts" "$DEST/docs"

EX="$DEST/examples/$slug-gate.json"
WORK="$DEST/scripts/$slug.sh"
GATE="$DEST/scripts/$slug-gate.sh"
DOC="$DEST/docs/$slug.md"

cat > "$EX" <<JSON
{
  "version": "patchline.$slug-gate/v1",
  "claim": "TODO: describe in >200 characters the data-change risk this $slug ecosystem detector surfaces, and how it is proven against a real public repository plus a deterministic unit matrix with a no-false-positive rule. Replace this placeholder before opening a pull request.",
  "real_repo": {
    "repo": "OWNER/REPO",
    "ref": "main",
    "minimum_findings": 1
  }
}
JSON

cat > "$WORK" <<SH
#!/usr/bin/env bash
set -euo pipefail
ROOT="\$(cd "\$(dirname "\${BASH_SOURCE[0]}")/.." && pwd)"
cd "\$ROOT"
SPEC="\${1:-examples/$slug-gate.json}"
OUT="\${2:-results/generated/$slug}"
rm -rf "\$OUT"; mkdir -p "\$OUT/cache" "\$OUT/analysis"
jq -e '.version == "patchline.$slug-gate/v1" and (.claim|length) > 200' "\$SPEC" > /dev/null
repo="\$(jq -r '.real_repo.repo' "\$SPEC")"
ref="\$(jq -r '.real_repo.ref' "\$SPEC")"
# TODO: run the analyzer and extract the findings your detector emits.
go run ./cmd/patchline repo analyze --github "\$repo" --ref "\$ref" \\
  --download-dir "\$OUT/cache" --stages inventory --no-llm --out "\$OUT/analysis" --json > "\$OUT/analyze.log"
INV="\$OUT/analysis/inventory/inventory.json"; test -s "\$INV"
# TODO: replace the jq selector below with your ecosystem's fact/finding kind.
findings="\$(jq '[.facts[]? | select(.kind == "$slug")] | length' "\$INV" 2>/dev/null || echo 0)"
jq -n --arg repo "\$repo" --argjson findings "\${findings:-0}" \\
  '{version:"patchline.$slug/v1", real_repo:\$repo, findings:\$findings, real_repo_detected:(\$findings>=1)}' > "\$OUT/$slug.json"
echo "$slug worker complete: \$findings findings on real repo"
SH

cat > "$GATE" <<SH
#!/usr/bin/env bash
set -euo pipefail
ROOT="\$(cd "\$(dirname "\${BASH_SOURCE[0]}")/.." && pwd)"
cd "\$ROOT"
SPEC="\${1:-examples/$slug-gate.json}"
OUT="\${2:-results/generated/$slug-gate}"
mkdir -p "\$(dirname "\$OUT")"
jq -e '.version == "patchline.$slug-gate/v1"' "\$SPEC" > /dev/null
for phrase in "make $slug-gate"; do grep -F "\$phrase" docs/$slug.md README.md > /dev/null; done
bash scripts/$slug.sh "\$SPEC" "\$OUT" > "\$OUT.run.log"
test -s "\$OUT/$slug.json"
jq -e '.version == "patchline.$slug/v1" and .real_repo_detected == true' "\$OUT/$slug.json" > /dev/null
echo "$slug gate passed"
SH

cat > "$DOC" <<MD
# $NAME

TODO: explain the data-change risk this $slug detector surfaces and how it is proven on real code.

\`\`\`
make $slug-gate
\`\`\`

Outputs land in \`results/generated/$slug/\`.
MD

chmod +x "$WORK" "$GATE"

echo "scaffolded ecosystem '$slug':"
echo "  $EX"
echo "  $WORK"
echo "  $GATE"
echo "  $DOC"
echo "next: implement the detector, add a Makefile target '$slug-gate', a README mention, and a unit test."
