#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
out_dir="$root/examples/public-corpus/downloads"
mkdir -p "$out_dir"

fetch_one() {
  local id="$1"
  local url="$2"
  local sha="$3"
  local out="$4"
  local path="$out_dir/$out"

  curl -fsSL "$url" -o "$path"
  local actual
  actual="$(shasum -a 256 "$path" | awk '{print $1}')"
  if [[ "$actual" != "$sha" ]]; then
    echo "sha256 mismatch for $id: expected $sha got $actual" >&2
    exit 1
  fi
  echo "$id $actual $path"
}

fetch_one \
  "bytebase-sheet-blob" \
  "https://raw.githubusercontent.com/bytebase/bytebase/47d2522552ce44271680424bf31a4cddd8a50ab1/backend/migrator/migration/3.1/0000%23%23sheet_blob.sql" \
  "3ea36a1d57832319f241a32f361e32c53e85bba34f67350ba3cd7512f1c40fa5" \
  "bytebase-sheet-blob.sql"

fetch_one \
  "bytebase-replica-heartbeat" \
  "https://raw.githubusercontent.com/bytebase/bytebase/47d2522552ce44271680424bf31a4cddd8a50ab1/backend/migrator/migration/3.14/0027%23%23replica_heartbeat.sql" \
  "1ab807c25f64eec2c05cfe3e0d4af218ff5f469bcaeb8c199d9559f6d0058c83" \
  "bytebase-replica-heartbeat.sql"

fetch_one \
  "bytebase-workspace" \
  "https://raw.githubusercontent.com/bytebase/bytebase/47d2522552ce44271680424bf31a4cddd8a50ab1/backend/migrator/migration/3.17/0009%23%23add_workspace_table.sql" \
  "276ad1148252ce21849054acf1d6ac62db5d72df3f88c9c2b4b42c28e7f5e98c" \
  "bytebase-workspace.sql"

fetch_one \
  "bytebase-drop-sheet-table" \
  "https://raw.githubusercontent.com/bytebase/bytebase/47d2522552ce44271680424bf31a4cddd8a50ab1/backend/migrator/migration/3.15/0007%23%23drop_sheet_table.sql" \
  "456248872b099828f958143f0ee5ce5d006d9705c7e85345486ad3430d140f13" \
  "bytebase-drop-sheet-table.sql"

fetch_one \
  "bytebase-drop-unused-id-columns" \
  "https://raw.githubusercontent.com/bytebase/bytebase/47d2522552ce44271680424bf31a4cddd8a50ab1/backend/migrator/migration/3.16/0002%23%23drop_unused_id_columns.sql" \
  "e9d46ecb1e229aa55b02426b052891783dd637492655b22c8ce385d5b0ac5492" \
  "bytebase-drop-unused-id-columns.sql"
