package project

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/thehalleyyoung/patchline/internal/intake"
	"github.com/thehalleyyoung/patchline/internal/migration"
)

func TestParseGitHubRepoAcceptsURLAndOwnerRepo(t *testing.T) {
	tests := []string{"bytebase/bytebase", "https://github.com/django/django.git"}
	for _, input := range tests {
		owner, repo, err := ParseGitHubRepo(input)
		if err != nil {
			t.Fatalf("ParseGitHubRepo(%q): %v", input, err)
		}
		if owner == "" || repo == "" {
			t.Fatalf("expected owner and repo for %q", input)
		}
	}
	if _, _, err := ParseGitHubRepo("../bad/repo"); err == nil {
		t.Fatalf("expected unsafe repo to fail")
	}
}

func TestFetchLocalCopiesRepoAndWritesSourceMetadata(t *testing.T) {
	src := t.TempDir()
	writeFile(t, src, "db/migrate/20250101_add_accounts.sql", "create table accounts(id int);")
	writeFile(t, src, ".git/config", "ignored")
	out := filepath.Join(t.TempDir(), "fetched")

	result, err := Fetch(context.Background(), FetchOptions{Input: src, OutDir: out, Subpath: "db/migrate"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Source.Mode != "local" || result.Source.ScannedRoot != filepath.ToSlash(filepath.Join(out, "db/migrate")) {
		t.Fatalf("unexpected source metadata: %#v", result.Source)
	}
	if _, err := os.Stat(filepath.Join(out, ".git")); !os.IsNotExist(err) {
		t.Fatalf("expected .git to be skipped, got err=%v", err)
	}
	data, err := os.ReadFile(filepath.Join(out, "source.json"))
	if err != nil {
		t.Fatal(err)
	}
	var source Source
	if err := json.Unmarshal(data, &source); err != nil {
		t.Fatal(err)
	}
	if source.Subpath != "db/migrate" || len(source.SkippedDirs) != 1 || source.SkippedDirs[0] != ".git" || source.ToolVersion == "" {
		t.Fatalf("unexpected source.json: %s", string(data))
	}
	if _, err := time.Parse(time.RFC3339, source.FetchedAt); err != nil {
		t.Fatalf("unexpected source.json: %s", string(data))
	}
}

func TestFetchLocalGitRecordsResolvedCommit(t *testing.T) {
	src := t.TempDir()
	writeFile(t, src, "db/migrate/001.sql", "create table accounts(id int);")
	runGit(t, src, "init", "--quiet")
	runGit(t, src, "config", "user.email", "patchline@example.com")
	runGit(t, src, "config", "user.name", "Patchline")
	runGit(t, src, "add", ".")
	runGit(t, src, "commit", "--quiet", "-m", "initial")
	expected := strings.TrimSpace(runGitOutput(t, src, "rev-parse", "HEAD"))

	result, err := Fetch(context.Background(), FetchOptions{Input: src, OutDir: filepath.Join(t.TempDir(), "fetched")})
	if err != nil {
		t.Fatal(err)
	}
	if result.Source.ResolvedCommit != expected {
		t.Fatalf("expected resolved commit %s, got %#v", expected, result.Source)
	}
}

func TestFetchArchiveURLUsesContentAddressedCache(t *testing.T) {
	archive := tarGzForTest(t, map[string]string{"repo-main/db/migrate/001.sql": "create table accounts(id int);"})
	var requests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&requests, 1) > 1 {
			http.Error(w, "cache miss", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/gzip")
		_, _ = w.Write(archive)
	}))
	defer server.Close()
	cacheDir := filepath.Join(t.TempDir(), "cache")

	first, err := Fetch(context.Background(), FetchOptions{Input: server.URL + "/repo.tar.gz", OutDir: filepath.Join(t.TempDir(), "first"), DownloadDir: cacheDir})
	if err != nil {
		t.Fatal(err)
	}
	second, err := Fetch(context.Background(), FetchOptions{Input: server.URL + "/repo.tar.gz", OutDir: filepath.Join(t.TempDir(), "second"), DownloadDir: cacheDir})
	if err != nil {
		t.Fatal(err)
	}
	if first.Source.CacheHit || !second.Source.CacheHit || second.Source.CachePath == "" || second.Source.ArchiveHash != first.Source.ArchiveHash || atomic.LoadInt32(&requests) != 1 {
		t.Fatalf("unexpected cache behavior: first=%#v second=%#v requests=%d", first.Source, second.Source, requests)
	}
	if _, err := os.Stat(filepath.Join(filepath.FromSlash(second.Source.ScannedRoot), "db/migrate/001.sql")); err != nil {
		t.Fatalf("expected cached archive extraction: %v", err)
	}
}

func TestFetchRejectsEscapingSubpath(t *testing.T) {
	src := t.TempDir()
	writeFile(t, src, "README.md", "hello")
	out := filepath.Join(t.TempDir(), "fetched")
	if _, err := Fetch(context.Background(), FetchOptions{Input: src, OutDir: out, Subpath: "../"}); err == nil {
		t.Fatalf("expected escaping subpath to fail")
	}
}

func TestInventoryDetectsProjectNativeSignals(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "go.mod", "module example.com/app\n")
	writeFile(t, root, ".github/workflows/ci.yml", "name: ci\n")
	writeFile(t, root, "db/migrate/20250101_fix_accounts.sql", "update accounts set disabled = false;")
	writeFile(t, root, "docs/incident-42.md", "rollback account repair")
	writeFile(t, root, "exports/deploy-events.jsonl", `{"deploy":"prod"}`)

	inv, err := InventoryPath(InventoryOptions{Path: root})
	if err != nil {
		t.Fatal(err)
	}
	if inv.FilesScanned != 5 {
		t.Fatalf("expected five files scanned, got %d", inv.FilesScanned)
	}
	if len(inv.Frameworks) == 0 || len(inv.MigrationRoots) == 0 || len(inv.CI) == 0 {
		t.Fatalf("expected framework, migration, and CI findings: %#v", inv)
	}
	if len(inv.OperationalDocs) == 0 || len(inv.EvidenceExports) == 0 || len(inv.NextCommands) == 0 {
		t.Fatalf("expected docs/evidence/next commands: %#v", inv)
	}
	if len(inv.NativeCommands) == 0 {
		t.Fatalf("expected native command suggestions: %#v", inv)
	}
	if len(inv.Facts) < inv.FilesScanned {
		t.Fatalf("expected fact stream to include at least file facts: facts=%d files=%d", len(inv.Facts), inv.FilesScanned)
	}
	if !strings.Contains(inv.ProjectMap, "facts.jsonl") {
		t.Fatalf("expected project map to point at facts.jsonl:\n%s", inv.ProjectMap)
	}
}

func TestInventoryDetectsNativeMigrationCommands(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "manage.py", "print('django')")
	writeFile(t, root, "alembic.ini", "[alembic]\n")
	writeFile(t, root, "prisma/migrations/001/migration.sql", "create table accounts(id int);")
	writeFile(t, root, "flyway.conf", "flyway.url=jdbc:postgresql://example\n")
	inv, err := InventoryPath(InventoryOptions{Path: root})
	if err != nil {
		t.Fatal(err)
	}
	for _, command := range []string{"python manage.py migrate", "alembic upgrade head", "npx prisma migrate status", "flyway migrate"} {
		if !hasCommand(inv.NativeCommands, command) {
			t.Fatalf("missing native command %q in %#v", command, inv.NativeCommands)
		}
	}
	if !hasCommand(inv.NextCommands, "python manage.py migrate") {
		t.Fatalf("expected native commands to be surfaced as next commands: %#v", inv.NextCommands)
	}
}

func TestInventoryPreservesUnknownStructuredFields(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "exports/event.json", `{"deploy":"prod-123","service":{"name":"billing"},"rows":42}`)
	writeFile(t, root, ".github/workflows/ci.yml", "name: ci\non: push\njobs:\n  test:\n    runs-on: ubuntu-latest\n")
	writeFile(t, root, "config/app.toml", "queue = \"billing.reconcile\"\nrollback_available = true\n")
	writeFile(t, root, "logs/app.log", "time=2025-01-02T03:04:05Z level=error error=PaymentRepairError deploy=prod-123")
	inv, err := InventoryPath(InventoryOptions{Path: root})
	if err != nil {
		t.Fatal(err)
	}
	if len(inv.FieldEvidence) == 0 {
		t.Fatalf("expected field evidence findings: %#v", inv)
	}
	expected := map[string]string{
		"deploy":             "prod-123",
		"service.name":       "billing",
		"queue":              "billing.reconcile",
		"rollback_available": "true",
		"error":              "PaymentRepairError",
	}
	for field, preview := range expected {
		if !hasFieldEvidence(inv.Facts, field, preview) {
			t.Fatalf("missing field evidence %s=%s in %#v", field, preview, inv.Facts)
		}
	}
}

func TestWriteInventoryEmitsFactsAndProjectMap(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "db/migrate/2025-01-01_fix_accounts.sql", "update accounts set disabled = false;")
	writeFile(t, root, "docs/incident-42.md", "Incident 42 rollback for accounts")
	inv, err := InventoryPath(InventoryOptions{Path: root})
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "out")
	if err := WriteInventory(out, inv); err != nil {
		t.Fatal(err)
	}
	facts, err := os.Open(filepath.Join(out, "facts.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer facts.Close()
	var sawFile, sawTable, sawIncident bool
	scanner := bufio.NewScanner(facts)
	for scanner.Scan() {
		var fact Fact
		if err := json.Unmarshal(scanner.Bytes(), &fact); err != nil {
			t.Fatal(err)
		}
		if fact.Kind == "file" && strings.HasPrefix(fact.ID, "fact:") {
			sawFile = true
		}
		for _, id := range fact.Identifiers {
			if id.Kind == "table" && id.Value == "accounts" {
				sawTable = true
			}
			if id.Kind == "incident" && strings.Contains(id.Value, "42") {
				sawIncident = true
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if !sawFile || !sawTable || !sawIncident {
		t.Fatalf("expected file/table/incident facts, got file=%v table=%v incident=%v", sawFile, sawTable, sawIncident)
	}
	projectMap, err := os.ReadFile(filepath.Join(out, "project-map.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(projectMap), "Patchline project map") {
		t.Fatalf("unexpected project map:\n%s", string(projectMap))
	}
}

func TestIdentifiersIgnoreSQLExistenceKeywords(t *testing.T) {
	ids := identifiersFromText("CREATE TABLE IF NOT EXISTS sheet_blob (id int); ALTER TABLE IF EXISTS sheet ADD COLUMN x int; update the record")
	var values []string
	for _, id := range ids {
		if id.Kind == "table" {
			values = append(values, id.Value)
		}
	}
	if strings.Join(values, ",") != "sheet,sheet_blob" {
		t.Fatalf("unexpected table identifiers: %#v", ids)
	}
}

func TestIdentifiersExtractProjectNativeSignals(t *testing.T) {
	text := `
class Invoice < ApplicationRecord
type LedgerEntry struct {}
model Account {
ALTER TABLE invoices ADD COLUMN status text;
UPDATE invoices SET repaired_at = now() WHERE account_id = 42;
POST /api/v1/invoices/{id}
queue: billing.reconcile
worker: InvoiceRepairWorker
report: revenue-drift
deploy: prod-20250102
panic=PaymentRepairError
incident-42 PR-77 2025-01-02T03:04:05Z 0123456789abcdef0123456789abcdef01234567
`
	ids := identifiersFromText(text)
	expected := map[string]string{
		"model":        "invoice",
		"column":       "status",
		"endpoint":     "/api/v1/invoices/{id}",
		"queue":        "billing.reconcile",
		"job":          "invoicerepairworker",
		"report":       "revenue-drift",
		"deploy":       "prod-20250102",
		"error":        "paymentrepairerror",
		"timestamp":    "2025-01-02t03:04:05z",
		"commit":       "0123456789abcdef0123456789abcdef01234567",
		"pull_request": "pr-77",
	}
	for kind, value := range expected {
		if !hasIdentifier(ids, kind, value) {
			t.Fatalf("missing %s=%s in %#v", kind, value, ids)
		}
	}
}

func TestInventoryInfersSchemaEvolutionFromMigrationsAndORM(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "db/migrate/001.sql", "CREATE TABLE accounts (id int, email text); ALTER TABLE accounts ADD COLUMN status text;")
	writeFile(t, root, "app/migrations/0002_user.py", `from django.db import migrations
class Migration(migrations.Migration):
    operations = [
        migrations.CreateModel(name='Profile', fields=[('id', models.BigAutoField(primary_key=True))]),
        migrations.AddField(model_name='Profile', name='display_name', field=models.CharField(max_length=100)),
    ]`)
	writeFile(t, root, "prisma/schema.prisma", `model Invoice {
  id Int @id
  total Decimal
}`)

	inv, err := InventoryPath(InventoryOptions{Path: root})
	if err != nil {
		t.Fatal(err)
	}
	if len(inv.SchemaEvolution) == 0 {
		t.Fatalf("expected schema evolution findings: %#v", inv)
	}
	var sawSQLColumn, sawDjangoField, sawPrismaField bool
	for _, fact := range inv.Facts {
		if fact.Kind != "schema_evolution" {
			continue
		}
		if fact.Properties["source"] == "sql" && fact.Properties["table"] == "accounts" && fact.Properties["column"] == "status" {
			sawSQLColumn = true
		}
		if fact.Properties["source"] == "django-migration" && fact.Properties["table"] == "profile" && fact.Properties["column"] == "display_name" {
			sawDjangoField = true
		}
		if fact.Properties["source"] == "prisma-schema" && fact.Properties["table"] == "invoice" && fact.Properties["column"] == "total" {
			sawPrismaField = true
		}
	}
	if !sawSQLColumn || !sawDjangoField || !sawPrismaField {
		t.Fatalf("missing schema facts sql=%v django=%v prisma=%v facts=%#v", sawSQLColumn, sawDjangoField, sawPrismaField, inv.Facts)
	}
}

func TestInventoryTreatsMigrationsRootAsMigrationEvidence(t *testing.T) {
	root := filepath.Join(t.TempDir(), "django", "contrib", "auth", "migrations")
	writeFile(t, root, "0001_initial.py", "class Migration: pass")
	inv, err := InventoryPath(InventoryOptions{Path: root})
	if err != nil {
		t.Fatal(err)
	}
	if len(inv.MigrationRoots) == 0 || inv.MigrationRoots[0].Path != "." {
		t.Fatalf("expected root migration evidence, got %#v", inv.MigrationRoots)
	}
}

func TestInventoryTreatsSingularMigrationRootAsMigrationEvidence(t *testing.T) {
	root := filepath.Join(t.TempDir(), "backend", "migrator", "migration")
	writeFile(t, root, "0001.sql", "create table accounts(id int)")
	inv, err := InventoryPath(InventoryOptions{Path: root})
	if err != nil {
		t.Fatal(err)
	}
	if len(inv.MigrationRoots) == 0 || inv.MigrationRoots[0].Kind != "migration" {
		t.Fatalf("expected singular migration root evidence, got %#v", inv.MigrationRoots)
	}
}

func TestExtractZipRejectsUnsafePaths(t *testing.T) {
	archive := filepath.Join(t.TempDir(), "unsafe.zip")
	file, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(file)
	w, err := zw.Create("../escape.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("bad")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := extractZip(archive, t.TempDir()); err == nil {
		t.Fatalf("expected unsafe zip path to fail")
	}
}

func TestBaselineRanksRisksAndLinksFactsWithUnderscores(t *testing.T) {
	inv := Inventory{
		Root:         filepath.ToSlash(t.TempDir()),
		TestCommands: []Command{{Command: "go test ./...", Reason: "Go tests"}},
		Facts: []Fact{
			{Version: Version, ID: "fact:table", Kind: "file", Path: "db/migrate/001.sql", Confidence: "observed", Identifiers: []Identifier{{Kind: "table", Value: "auth_user"}}},
			{Version: Version, ID: "fact:doc", Kind: "operational_doc", Path: "docs/incident.md", Confidence: "path", Identifiers: []Identifier{{Kind: "table", Value: "auth_user"}}},
		},
	}
	report := intake.Report{
		Source: intake.Source{Input: "fixture"},
		TimeSignals: []intake.TimeSignal{
			{ID: "time:migration", Path: "db/migrate/001_accounts.sql", Timestamp: "2025-01-15", Source: "path", Confidence: "temporal", Identifiers: []string{"table:accounts"}},
			{ID: "time:incident", Path: "docs/inc-42.md", Timestamp: "2025-01-16", Source: "content", Confidence: "temporal", Identifiers: []string{"table:accounts", "incident:incident 42"}},
			{ID: "time:repair", Path: "scripts/rollback_accounts.sql", Timestamp: "2025-01-17", Source: "path", Confidence: "temporal", Identifiers: []string{"table:accounts"}},
		},
		SQL: []intake.SQLFinding{{
			Path:       "db/migrate/001.sql",
			SourceKind: "sql_file",
			Statements: migrationStatementsForTest{{
				Index: 0, Kind: "update", Table: "auth_user", Risk: "high", Reasons: []string{"unbounded update can rewrite an entire table"},
			}}.asMigrationStatements(),
		}},
		Problems: []intake.ProblemCandidate{{ID: "problem:1", Path: "db/migrate/001.sql", Kind: "high-risk-sql", Severity: "high", Table: "auth_user", Identifiers: []string{"table:auth_user"}, Rationale: "risky update"}},
	}
	baseline := Baseline(inv, inv.Facts, report)
	if baseline.Summary.RankedRisks == 0 || baseline.Summary.EvidenceLinks == 0 || baseline.Summary.IdentifierOnlyLinks == 0 {
		t.Fatalf("expected ranked risks and identifier links: %#v", baseline.Summary)
	}
	if len(baseline.NativeChecks) != 1 {
		t.Fatalf("expected native check passthrough: %#v", baseline.NativeChecks)
	}
	if !strings.Contains(baseline.Markdown, "Patchline repo baseline") {
		t.Fatalf("expected baseline markdown, got %q", baseline.Markdown)
	}
}

func TestBaselineRanksPersistentWriteCodePaths(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "app/jobs/repair.rb", `class RepairJob
  def perform
    Account.update_all(disabled: false)
  end
end`)
	inv := Inventory{
		Root: filepath.ToSlash(root),
		Facts: []Fact{
			{Version: Version, ID: "fact:account", Kind: "file", Path: "app/jobs/repair.rb", Confidence: "observed", Identifiers: []Identifier{{Kind: "table", Value: "accounts"}}},
		},
	}
	report := intake.Report{
		Source: intake.Source{Input: "fixture"},
		SourceSQL: migration.SourceSQLReport{
			Root: filepath.ToSlash(root),
			Observations: []migration.SourceSQLObservation{{
				Path:        "app/jobs/repair.rb",
				Language:    "Ruby",
				Detector:    "ruby.rails-active-record",
				Line:        3,
				Kind:        "orm_query",
				Framework:   "rails",
				Operation:   "update",
				Table:       "accounts",
				Confidence:  "medium",
				SnippetHash: "snippet-hash",
			}},
		},
	}
	baseline := Baseline(inv, inv.Facts, report)
	if baseline.Summary.CodePathRankedRisks == 0 {
		t.Fatalf("expected code path risks: %#v", baseline)
	}
	var sawMissingTransaction bool
	for _, risk := range baseline.Risks {
		if !strings.HasPrefix(risk.Kind, "code-path:") {
			continue
		}
		for _, factor := range risk.Factors {
			if factor.Name == "missing-transaction-boundary" {
				sawMissingTransaction = true
			}
		}
	}
	if !sawMissingTransaction {
		t.Fatalf("expected missing transaction factor: %#v", baseline.Risks)
	}
}

func TestBaselineBuildsProvenanceSlices(t *testing.T) {
	root := t.TempDir()
	inv := Inventory{
		Root: filepath.ToSlash(root),
		TestCommands: []Command{
			{Command: "go test ./...", Reason: "Go tests"},
		},
		NativeCommands: []Command{
			{Command: "go test ./...", Reason: "Go native check"},
		},
		Facts: []Fact{
			{Version: Version, ID: "fact:migration", Kind: "schema_evolution", Path: "db/migrate/001_accounts.sql", Confidence: "derived", Identifiers: []Identifier{{Kind: "table", Value: "accounts"}}},
			{Version: Version, ID: "fact:source", Kind: "file", Path: "app/jobs/account_repair.go", Confidence: "observed", Identifiers: []Identifier{{Kind: "table", Value: "accounts"}}, Properties: map[string]string{"language": "Go"}},
			{Version: Version, ID: "fact:incident", Kind: "operational_doc", Path: "docs/inc-42.md", Confidence: "path", Identifiers: []Identifier{{Kind: "table", Value: "accounts"}, {Kind: "incident", Value: "incident 42"}}},
			{Version: Version, ID: "fact:repair", Kind: "file", Path: "scripts/rollback_accounts.sql", Confidence: "observed", Identifiers: []Identifier{{Kind: "table", Value: "accounts"}}},
		},
	}
	report := intake.Report{
		Source: intake.Source{Input: "fixture"},
		TimeSignals: []intake.TimeSignal{
			{ID: "time:migration", Path: "db/migrate/001_accounts.sql", Timestamp: "2025-01-15", Source: "path", Confidence: "temporal", Identifiers: []string{"table:accounts"}},
			{ID: "time:incident", Path: "docs/inc-42.md", Timestamp: "2025-01-16", Source: "content", Confidence: "temporal", Identifiers: []string{"table:accounts", "incident:incident 42"}},
			{ID: "time:repair", Path: "scripts/rollback_accounts.sql", Timestamp: "2025-01-17", Source: "path", Confidence: "temporal", Identifiers: []string{"table:accounts"}},
		},
		SQL: []intake.SQLFinding{{
			Path:       "db/migrate/001_accounts.sql",
			SourceKind: "sql_file",
			Statements: migrationStatementsForTest{{
				Index: 0, Kind: "update", Table: "accounts", Risk: "high", Reasons: []string{"unbounded update can rewrite an entire table"},
			}}.asMigrationStatements(),
		}, {
			Path:       "db/migrate/002_accounts_retry.sql",
			SourceKind: "sql_file",
			Statements: migrationStatementsForTest{{
				Index: 0, Kind: "update", Table: "accounts", Risk: "high", Reasons: []string{"unbounded update can rewrite an entire table"},
			}}.asMigrationStatements(),
		}},
		Causes: []intake.CauseCandidate{{
			ID: "cause:incident", Path: "docs/inc-42.md", Kind: "incident-or-postmortem-text", Identifiers: []string{"table:accounts", "incident:incident 42"}, Rationale: "incident text",
		}},
		RepairCandidates: []intake.RepairCandidate{{
			ID: "repair:rollback", Path: "scripts/rollback_accounts.sql", Kind: "repair-like-sql", Table: "accounts", Identifiers: []string{"table:accounts"}, Rationale: "rollback SQL",
		}},
	}
	baseline := Baseline(inv, inv.Facts, report)
	if baseline.Summary.ProvenanceSlices == 0 {
		t.Fatalf("expected provenance slices: %#v", baseline)
	}
	if baseline.Summary.DatalogQueries != 4 || baseline.Summary.DatalogRows == 0 {
		t.Fatalf("expected datalog-style query rows: %#v", baseline.Summary)
	}
	if baseline.Summary.AbstractEffects == 0 || baseline.Summary.AbstractOperations == 0 || baseline.Summary.AbstractProofHoles == 0 {
		t.Fatalf("expected static abstract effects with proof holes: %#v", baseline.Summary)
	}
	if baseline.Summary.SymbolicChecks == 0 || baseline.Summary.SymbolicFailed == 0 {
		t.Fatalf("expected symbolic checks with unresolved obligations: %#v", baseline.Summary)
	}
	if baseline.Summary.TemporalWindows == 0 || baseline.Summary.TemporalSignals < 3 {
		t.Fatalf("expected temporal windows over migration/incident/repair signals: %#v", baseline.Summary)
	}
	if baseline.Summary.Recurrences == 0 || baseline.Summary.RecurringRisks < 2 {
		t.Fatalf("expected recurring risk patterns: %#v", baseline.Summary)
	}
	var fullSlice ProvenanceSlice
	for _, slice := range baseline.Provenance {
		if slice.Table == "accounts" {
			fullSlice = slice
			break
		}
	}
	if fullSlice.ID == "" {
		t.Fatalf("missing accounts provenance slice: %#v", baseline.Provenance)
	}
	for _, stage := range []string{"migration", "table", "source", "test", "native-check", "incident", "repair"} {
		if !stringSliceContains(fullSlice.StagesPresent, stage) {
			t.Fatalf("expected stage %q in %#v", stage, fullSlice)
		}
	}
	if !hasDatalogRow(baseline.DatalogQueries, "repair_lineage") {
		t.Fatalf("expected repair lineage query row: %#v", baseline.DatalogQueries)
	}
}

func TestLoadInventoryAcceptsInventoryJSONPath(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "db/migrate/001.sql", "update auth_user set is_active = false;")
	inv, err := InventoryPath(InventoryOptions{Path: root})
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "inventory")
	if err := WriteInventory(out, inv); err != nil {
		t.Fatal(err)
	}
	loaded, _, err := LoadInventory(filepath.Join(out, "inventory.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Facts) == 0 {
		t.Fatalf("expected facts loaded from sibling facts.jsonl")
	}
}

func TestProposeWritesIsolatedPatchAndMetadata(t *testing.T) {
	baseline := BaselineReport{
		Version:       BaselineVersion,
		InventoryRoot: t.TempDir(),
		IntakeSource:  "fixture",
		Hash:          "baseline-hash",
		Risks: []BaselineRisk{{
			ID:        "risk:auth_user",
			Path:      "db/migrate/001.sql",
			Kind:      "update",
			Table:     "auth_user",
			Severity:  "high",
			Score:     130,
			Rationale: "unbounded update can rewrite an entire table",
			Factors:   []ScoreFactor{{Name: "high-risk-sql", Weight: 100, Reason: "high risk"}},
		}},
		NativeChecks: []Command{{Command: "go test ./...", Reason: "Go tests"}},
	}
	baseline.Hash = baselineHash(baseline)
	baselineDir := filepath.Join(t.TempDir(), "baseline")
	if err := WriteBaseline(baselineDir, baseline); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "proposal")
	proposal, err := Propose(ProposalOptions{BaselinePath: baselineDir, Kind: "all", OutDir: out, BudgetRisks: 1})
	if err != nil {
		t.Fatal(err)
	}
	if proposal.Trust != "untrusted-generated-proposal" || len(proposal.GeneratedFiles) != 5 || proposal.ContextHash == "" || proposal.OutputHash == "" {
		t.Fatalf("unexpected proposal metadata: %#v", proposal)
	}
	if err := WriteProposal(out, proposal); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"proposal.json", "proposal.md", "proposal.patch", "prompt-context.json", "prompt.txt"} {
		if _, err := os.Stat(filepath.Join(out, rel)); err != nil {
			t.Fatalf("missing %s: %v", rel, err)
		}
	}
	repo := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "-C", repo, "init", "--quiet").Run(); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("git", "-C", repo, "apply", "--check", filepath.Join(out, "proposal.patch")).CombinedOutput(); err != nil {
		t.Fatalf("proposal patch should apply cleanly: %v\n%s", err, string(output))
	}
}

func TestProposeLLMCommandCapturesOutputAsUntrustedArtifact(t *testing.T) {
	baseline := BaselineReport{Version: BaselineVersion, Hash: "baseline-hash", Risks: []BaselineRisk{{ID: "risk:1", Path: "db/migrate/001.sql", Table: "accounts", Severity: "high", Score: 100, Rationale: "risk"}}}
	baseline.Hash = baselineHash(baseline)
	baselineDir := filepath.Join(t.TempDir(), "baseline")
	if err := WriteBaseline(baselineDir, baseline); err != nil {
		t.Fatal(err)
	}
	proposal, err := Propose(ProposalOptions{BaselinePath: baselineDir, Kind: "tests", LLMCommand: "cat", BudgetRisks: 1})
	if err != nil {
		t.Fatal(err)
	}
	if proposal.Generator != "llm-command" || len(proposal.GeneratedFiles) != 1 || proposal.GeneratedFiles[0].Kind != "llm-output" {
		t.Fatalf("unexpected llm proposal: %#v", proposal)
	}
}

func TestCompareChecksGeneratedProposalCoverage(t *testing.T) {
	baseline := BaselineReport{
		Version: BaselineVersion,
		Hash:    "baseline-hash",
		Risks: []BaselineRisk{{
			ID:        "risk:accounts",
			Path:      "db/migrate/001.sql",
			Kind:      "update",
			Table:     "accounts",
			Severity:  "high",
			Score:     130,
			Rationale: "unbounded update",
		}},
	}
	baseline.Hash = baselineHash(baseline)
	proposal, err := Propose(ProposalOptions{BaselinePath: writeBaselineForTest(t, baseline), Kind: "all", BudgetRisks: 1})
	if err != nil {
		t.Fatal(err)
	}
	compare := Compare(baseline, proposal)
	if compare.Summary.GeneratedFiles != 5 || compare.Summary.RisksWithCoverage != 1 || compare.Summary.PatchlineChecksFailed != 0 || compare.Summary.NewHighRiskSQL != 0 {
		t.Fatalf("unexpected compare summary: %#v", compare.Summary)
	}
	if !strings.Contains(compare.Markdown, "Patchline repo compare") {
		t.Fatalf("expected compare markdown, got %q", compare.Markdown)
	}
}

func TestCompareRunsSafeNativeChecks(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "go.mod", "module example.com/nativecheck\n\ngo 1.22\n")
	writeFile(t, root, "native_test.go", `package nativecheck

import "testing"

func TestNative(t *testing.T) {}
`)
	baseline := BaselineReport{
		Version:       BaselineVersion,
		InventoryRoot: root,
		Hash:          "baseline-hash",
		NativeChecks:  []Command{{Command: "go test ./...", Reason: "Go module test command"}},
	}
	baseline.Hash = baselineHash(baseline)
	compare := CompareWithOptions(baseline, ProposalReport{OutputHash: "proposal-hash"}, CompareOptions{RunNativeTests: true})
	if compare.Summary.NativeChecksRun != 1 || compare.Summary.NativeChecksPassed != 1 || compare.Summary.NativeChecksFailed != 0 {
		t.Fatalf("unexpected native check summary: %#v", compare.Summary)
	}
	if len(compare.NativeResults) != 1 || compare.NativeResults[0].Status != "pass" || compare.NativeResults[0].LogHash == "" {
		t.Fatalf("expected passing native result with log hash: %#v", compare.NativeResults)
	}
}

func writeBaselineForTest(t *testing.T, baseline BaselineReport) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "baseline")
	if err := WriteBaseline(dir, baseline); err != nil {
		t.Fatal(err)
	}
	return dir
}

func stringSliceContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func hasDatalogRow(queries []DatalogQuery, name string) bool {
	for _, query := range queries {
		if query.Name == name && len(query.Rows) > 0 {
			return true
		}
	}
	return false
}

type migrationStatementForTest struct {
	Index   int
	Kind    string
	Table   string
	Risk    string
	Reasons []string
}

type migrationStatementsForTest []migrationStatementForTest

func (items migrationStatementsForTest) asMigrationStatements() []migration.Statement {
	out := make([]migration.Statement, 0, len(items))
	for _, item := range items {
		out = append(out, migration.Statement{Index: item.Index, Kind: item.Kind, Table: item.Table, Risk: migration.Risk(item.Risk), Reasons: item.Reasons})
	}
	return out
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runGit(t *testing.T, root string, args ...string) {
	t.Helper()
	if output, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(output))
	}
}

func runGitOutput(t *testing.T, root string, args ...string) string {
	t.Helper()
	output, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(output))
	}
	return string(output)
}

func tarGzForTest(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		header := &tar.Header{Name: filepath.ToSlash(name), Mode: 0o644, Size: int64(len(content))}
		if err := tw.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(tw, content); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func hasIdentifier(ids []Identifier, kind, value string) bool {
	for _, id := range ids {
		if id.Kind == kind && id.Value == value {
			return true
		}
	}
	return false
}

func hasCommand(commands []Command, command string) bool {
	for _, item := range commands {
		if item.Command == command {
			return true
		}
	}
	return false
}

func hasFieldEvidence(facts []Fact, field, preview string) bool {
	for _, fact := range facts {
		if fact.Kind == "field_evidence" && fact.Properties["field"] == field && fact.Properties["value_preview"] == preview {
			return true
		}
	}
	return false
}
