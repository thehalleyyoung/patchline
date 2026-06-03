#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

OUT="${1:-results/generated/remediation-playbook-gate}"
FIXTURE="$OUT/fixture"
rm -rf "$OUT"
mkdir -p "$FIXTURE/.github" "$FIXTURE/db/migrate" "$FIXTURE/docs"

cat > "$FIXTURE/.github/CODEOWNERS" <<'OWNERS'
/db/migrate/ @org/db-team
OWNERS

cat > "$FIXTURE/db/migrate/001_accounts.sql" <<'SQL'
UPDATE accounts SET status = 'disabled';
SQL

cat > "$FIXTURE/docs/inc-42.md" <<'DOC'
# Incident 42

The accounts migration needs a rollback owner, lock monitoring, and a manual stop condition.
DOC

go run ./cmd/patchline repo inventory "$FIXTURE" --out "$OUT/inventory" --json > "$OUT/inventory.stdout.json"
go run ./cmd/patchline intake "$FIXTURE" --out "$OUT/intake" --json > "$OUT/intake.stdout.json"
go run ./cmd/patchline repo baseline --inventory "$OUT/inventory" --intake "$OUT/intake" --out "$OUT/baseline" --json > "$OUT/baseline.stdout.json"
go run ./cmd/patchline repo playbook --baseline "$OUT/baseline" --out "$OUT/playbook" --json > "$OUT/playbook.stdout.json"

grep -q '"class": "broad-write"' "$OUT/playbook/playbook.json"
grep -q '"class": "transaction-boundary"' "$OUT/playbook/playbook.json"
grep -q '"stage": "before-execution"' "$OUT/playbook/playbook.json"
grep -q '"stage": "during-execution"' "$OUT/playbook/playbook.json"
grep -q '@org/db-team' "$OUT/playbook/playbook.json"
grep -q 'Rollback points' "$OUT/playbook/playbook.md"
grep -q 'owner handoffs' "$OUT/playbook/playbook.md"

echo "remediation playbook gate passed: $(grep -c '"risk_id"' "$OUT/playbook/playbook.json") playbook entries"
