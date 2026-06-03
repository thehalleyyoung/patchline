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

func TestBaselineRoutesRisksWithCodeowners(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".github/CODEOWNERS", "* @org/global\n/db/migrate/ @org/db-team @alice\n")
	writeFile(t, root, "db/migrate/001_backfill.sql", "UPDATE accounts SET status = 'active';\n")
	inv, err := InventoryPath(InventoryOptions{Path: filepath.Join(root, "db/migrate")})
	if err != nil {
		t.Fatal(err)
	}
	intakeReport, err := intake.Run(context.Background(), intake.Options{Path: filepath.Join(root, "db/migrate")})
	if err != nil {
		t.Fatal(err)
	}
	baseline := Baseline(inv, inv.Facts, intakeReport)
	if baseline.Summary.OwnerRoutes == 0 || baseline.Summary.OwnerRouteOwners != 2 {
		t.Fatalf("expected CODEOWNERS owner routing summary, got %#v", baseline.Summary)
	}
	if len(baseline.OwnerRoutes) == 0 || strings.Join(baseline.OwnerRoutes[0].Owners, ",") != "@alice,@org/db-team" {
		t.Fatalf("expected db-team route, got %#v", baseline.OwnerRoutes)
	}
	if !strings.Contains(baseline.Markdown, "CODEOWNERS routing") {
		t.Fatalf("expected routing section in markdown: %s", baseline.Markdown)
	}
}

func TestProposalRoutesGeneratedInterventionsToRiskOwners(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".github/CODEOWNERS", "/db/migrate/ @org/db-team\n")
	writeFile(t, root, "db/migrate/001_delete.sql", "DELETE FROM accounts;\n")
	inv, err := InventoryPath(InventoryOptions{Path: filepath.Join(root, "db/migrate")})
	if err != nil {
		t.Fatal(err)
	}
	intakeReport, err := intake.Run(context.Background(), intake.Options{Path: filepath.Join(root, "db/migrate")})
	if err != nil {
		t.Fatal(err)
	}
	baseline := Baseline(inv, inv.Facts, intakeReport)
	baselineDir := filepath.Join(t.TempDir(), "baseline")
	if err := WriteBaseline(baselineDir, baseline); err != nil {
		t.Fatal(err)
	}
	proposal, err := Propose(ProposalOptions{BaselinePath: baselineDir, Kind: "guards", OutDir: filepath.Join(t.TempDir(), "proposal"), NoLLM: true, BudgetRisks: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(proposal.GeneratedFiles) == 0 || strings.Join(proposal.GeneratedFiles[0].Reviewers, ",") != "@org/db-team" {
		t.Fatalf("expected generated file reviewers from CODEOWNERS, got %#v", proposal.GeneratedFiles)
	}
	if len(proposal.OwnerRoutes) == 0 || proposal.OwnerRoutes[0].SubjectKind != "generated_file" {
		t.Fatalf("expected generated owner routes, got %#v", proposal.OwnerRoutes)
	}
	if !strings.Contains(proposal.Markdown, "likely reviewers") {
		t.Fatalf("expected reviewers in proposal markdown: %s", proposal.Markdown)
	}
}

func TestProposalPromptContextMinimizesUnselectedEvidence(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "db/migrate/001_delete.sql", strings.Join([]string{
		"CREATE TABLE accounts(id int);",
		"DELETE FROM accounts;",
		"INSERT INTO audit_log VALUES (1);",
	}, "\n"))
	writeFile(t, root, "db/migrate/002_drop.sql", "DROP TABLE invoices;\n")
	baseline := BaselineReport{
		Version:       BaselineVersion,
		InventoryRoot: root,
		Risks: []BaselineRisk{
			{ID: "risk-delete", Path: "db/migrate/001_delete.sql", Statement: 2, Kind: "broad-delete", Table: "accounts", Severity: "high", Score: 90, Rationale: "delete all accounts"},
			{ID: "risk-drop", Path: "db/migrate/002_drop.sql", Statement: 1, Kind: "drop-table", Table: "invoices", Severity: "high", Score: 80, Rationale: "drop invoices"},
		},
		EvidenceLinks: []EvidenceLink{
			{RiskID: "risk-delete", FactID: "fact-delete", Path: "db/migrate/001_delete.sql", Confidence: "high"},
			{RiskID: "risk-drop", FactID: "fact-drop", Path: "db/migrate/002_drop.sql", Confidence: "high"},
		},
		Provenance: []ProvenanceSlice{
			{ID: "slice-delete", RiskID: "risk-delete", MigrationPath: "db/migrate/001_delete.sql", NativeCommands: []Command{{Command: "go test ./accounts", Reason: "accounts checks"}}, StagesPresent: []string{"migration"}, Confidence: "high"},
			{ID: "slice-drop", RiskID: "risk-drop", MigrationPath: "db/migrate/002_drop.sql", NativeCommands: []Command{{Command: "go test ./invoices", Reason: "invoice checks"}}, StagesPresent: []string{"migration"}, Confidence: "high"},
		},
		NativeChecks: []Command{
			{Command: "go test ./accounts", Reason: "accounts checks"},
			{Command: "go test ./invoices", Reason: "invoice checks"},
		},
	}
	baseline.Hash = baselineHash(baseline)
	proposal, err := Propose(ProposalOptions{BaselinePath: writeBaselineForTest(t, baseline), Kind: "guards", BudgetRisks: 1})
	if err != nil {
		t.Fatal(err)
	}
	if got := proposal.ContextMin; got.SelectedRisks != 1 || got.ExcludedRisks != 1 || got.IncludedEvidenceLinks != 1 || got.ExcludedEvidenceLinks != 1 || got.IncludedProvenanceSlices != 1 || got.ExcludedProvenanceSlices != 1 {
		t.Fatalf("unexpected context minimization counts: %#v", got)
	}
	if len(proposal.Context.Risks) != 1 || proposal.Context.Risks[0].ID != "risk-delete" {
		t.Fatalf("expected only selected risk in context: %#v", proposal.Context.Risks)
	}
	if strings.Contains(strings.Join(proposal.Context.Risks[0].EvidencePaths, ","), "002_drop") || strings.Contains(proposal.Prompt, "invoices") {
		t.Fatalf("unselected evidence leaked into prompt context: paths=%v prompt=%s", proposal.Context.Risks[0].EvidencePaths, proposal.Prompt)
	}
	if !strings.Contains(proposal.Context.Risks[0].Excerpt, "DELETE FROM accounts") || strings.Contains(proposal.Context.Risks[0].Excerpt, "CREATE TABLE") {
		t.Fatalf("expected risk-focused excerpt, got %q", proposal.Context.Risks[0].Excerpt)
	}
	if !strings.Contains(proposal.Prompt, "Context minimization") || !strings.Contains(proposal.Prompt, "excluded=1") {
		t.Fatalf("expected prompt minimization counts, got %s", proposal.Prompt)
	}
}

func TestInventoryDetectsNoSQLDestructiveChanges(t *testing.T) {
	cases := []struct {
		rel      string
		content  string
		engine   string
		op       string
		destruct bool
	}{
		{"migrations/001_drop.js", "module.exports.up = async (db) => { await db.collection('users').drop(); };", "mongodb", "dropCollection", true},
		{"migrations/002_unset.js", "db.users.updateMany({}, { $unset: { legacy: 1 } });", "mongodb", "unsetField", true},
		{"schema/001_drop.cql", "DROP TABLE accounts;\nDROP KEYSPACE billing;", "cassandra", "dropTable", true},
		{"scripts/reindex.sh", "curl -X DELETE \"http://localhost:9200/orders\"\n# _bulk reindex", "elasticsearch", "deleteIndex", true},
		{"ops/flush.redis", "redis-cli FLUSHALL\nDEL session:1", "redis", "flushAll", true},
		{"infra/dynamo.sh", "aws dynamodb delete-table --table-name Orders", "dynamodb", "deleteTable", true},
	}
	for _, c := range cases {
		root := t.TempDir()
		writeFile(t, root, c.rel, c.content)
		inv, err := InventoryPath(InventoryOptions{Path: root})
		if err != nil {
			t.Fatalf("%s: %v", c.rel, err)
		}
		found := false
		for _, f := range inv.NoSQLChanges {
			if f.Kind == c.engine+":"+c.op {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s: expected %s:%s, got %#v", c.rel, c.engine, c.op, inv.NoSQLChanges)
		}
		// Destructive operations must be flagged as such in a fact.
		destructiveFact := false
		for _, fact := range inv.Facts {
			if fact.Kind == "nosql_change" && fact.Properties["engine"] == c.engine && fact.Properties["destructive"] == "true" {
				destructiveFact = true
			}
		}
		if c.destruct && !destructiveFact {
			t.Fatalf("%s: expected destructive nosql_change fact for %s", c.rel, c.engine)
		}
	}
}

func TestInventoryDoesNotFlagNoSQLWithoutSignal(t *testing.T) {
	root := t.TempDir()
	// A plain prose file mentioning "drop" must not be classified as a NoSQL change.
	writeFile(t, root, "docs/notes.md", "We had to drop the old plan and rename our approach. FLUSHALL of ideas.\n")
	inv, err := InventoryPath(InventoryOptions{Path: root})
	if err != nil {
		t.Fatal(err)
	}
	// "redis" signal could match ".redis" only; notes.md has no redis-cli/redis signal in path or
	// text, so no redis change should be recorded.
	for _, f := range inv.NoSQLChanges {
		if strings.HasPrefix(f.Kind, "redis:") || strings.HasPrefix(f.Kind, "mongodb:") {
			t.Fatalf("unexpected NoSQL change from prose: %#v", inv.NoSQLChanges)
		}
	}
}

func TestInventoryDetectsSchemaCompatRisks(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "proto/user.proto", `syntax = "proto2";
message User {
  required string id = 1;
  optional string name = 2;
}
`)
	writeFile(t, root, "schemas/event.avsc", `{
  "type": "record",
  "name": "Event",
  "fields": [
    {"name": "id", "type": "string"},
    {"name": "ts", "type": "long"}
  ]
}
`)
	writeFile(t, root, "buf.yaml", "version: v1\nbreaking:\n  use:\n    - FILE\n")
	inv, err := InventoryPath(InventoryOptions{Path: root})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		"protobuf_required_field":    false,
		"avro_field_without_default": false,
		"schema_registry_config":     false,
	}
	for _, f := range inv.SchemaCompat {
		if _, ok := want[f.Kind]; ok {
			want[f.Kind] = true
		}
	}
	for k, seen := range want {
		if !seen {
			t.Fatalf("expected schema-compat finding %s, got %#v", k, inv.SchemaCompat)
		}
	}
	breakingFacts := 0
	for _, fact := range inv.Facts {
		if fact.Kind == "schema_compatibility" && fact.Properties["breaking"] == "true" {
			breakingFacts++
		}
	}
	if breakingFacts < 2 {
		t.Fatalf("expected at least 2 breaking schema_compatibility facts, got %d", breakingFacts)
	}
}

func TestInventoryDoesNotFlagSchemaCompatForPlainJSON(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "config/settings.json", `{"type": "production", "record": false, "name": "app"}`)
	inv, err := InventoryPath(InventoryOptions{Path: root})
	if err != nil {
		t.Fatal(err)
	}
	if len(inv.SchemaCompat) != 0 {
		t.Fatalf("plain .json must not be treated as an Avro schema: %#v", inv.SchemaCompat)
	}
}

func TestInventoryAnalyzesInfraDataOrdering(t *testing.T) {
	root := t.TempDir()
	// An unordered migration job: a Kubernetes Job that runs migrations with no ordering marker.
	writeFile(t, root, "k8s/migrate-job.yaml", `apiVersion: batch/v1
kind: Job
metadata:
  name: db-migrate
spec:
  template:
    spec:
      containers:
        - name: migrate
          command: ["sh", "-c", "alembic upgrade head"]
`)
	// An ordered migration job: a Helm-hooked Job that runs migrations.
	writeFile(t, root, "helm/templates/migrate-hook.yaml", `apiVersion: batch/v1
kind: Job
metadata:
  name: db-migrate-hook
  annotations:
    "helm.sh/hook": pre-upgrade
spec:
  template:
    spec:
      initContainers:
        - name: wait
          image: busybox
      containers:
        - name: migrate
          command: ["sh", "-c", "flyway migrate"]
`)
	inv, err := InventoryPath(InventoryOptions{Path: root})
	if err != nil {
		t.Fatal(err)
	}
	var sawUnordered, sawSequenced bool
	for _, f := range inv.InfraDataOrdering {
		if f.Kind == "infra_data_ordering_unordered" && strings.Contains(f.Path, "migrate-job") {
			sawUnordered = true
		}
		if f.Kind == "infra_data_ordering_sequenced" && strings.Contains(f.Path, "migrate-hook") {
			sawSequenced = true
		}
	}
	if !sawUnordered {
		t.Fatalf("expected an unordered infra/data ordering finding, got %#v", inv.InfraDataOrdering)
	}
	if !sawSequenced {
		t.Fatalf("expected a sequenced infra/data ordering finding, got %#v", inv.InfraDataOrdering)
	}
	// The unordered job must be recorded as an ordered=false fact.
	var unorderedFact bool
	for _, fact := range inv.Facts {
		if fact.Kind == "infra_data_ordering" && fact.Properties["ordered"] == "false" && fact.Properties["job"] == "migration" {
			unorderedFact = true
		}
	}
	if !unorderedFact {
		t.Fatalf("expected an ordered=false infra_data_ordering migration fact")
	}
}

func TestInventoryDetectsDataPipelineChanges(t *testing.T) {
	cases := []struct {
		rel       string
		content   string
		framework string
		op        string
		destruct  bool
	}{
		{"dags/backfill.py", "from airflow import DAG\nwith DAG('backfill') as dag:\n    pass\n# clear_task_instances reset", "airflow", "backfillOrReset", true},
		{"dags/etl.py", "from airflow.operators.postgres_operator import PostgresOperator\nPostgresOperator(task_id='t', sql='SELECT 1')", "airflow", "sqlOperator", false},
		{"models/orders.sql", "{{ config(materialized='table') }}\nselect * from {{ ref('raw_orders') }}", "dbt", "fullRefreshOrTable", true},
		{"jobs/transform.py", "from pyspark.sql import SparkSession\ndf.write.mode('overwrite').saveAsTable('warehouse.orders')", "spark", "writeOverwriteOrTable", true},
		{"consumers/listener.py", "from kafka import KafkaConsumer\nc = KafkaConsumer('topic', auto_offset_reset='earliest')", "kafka", "offsetReset", true},
	}
	for _, c := range cases {
		root := t.TempDir()
		writeFile(t, root, c.rel, c.content)
		inv, err := InventoryPath(InventoryOptions{Path: root})
		if err != nil {
			t.Fatalf("%s: %v", c.rel, err)
		}
		found := false
		for _, f := range inv.DataPipelines {
			if f.Kind == c.framework+":"+c.op {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s: expected %s:%s, got %#v", c.rel, c.framework, c.op, inv.DataPipelines)
		}
		if c.destruct {
			destructiveFact := false
			for _, fact := range inv.Facts {
				if fact.Kind == "data_pipeline_change" && fact.Properties["framework"] == c.framework && fact.Properties["destructive"] == "true" {
					destructiveFact = true
				}
			}
			if !destructiveFact {
				t.Fatalf("%s: expected destructive data_pipeline_change fact for %s", c.rel, c.framework)
			}
		}
	}
}

func TestInventoryDoesNotFlagDataPipelineWithoutSignal(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "docs/notes.md", "We overwrite the whiteboard and reset our plans each sprint. The spark of an idea.\n")
	inv, err := InventoryPath(InventoryOptions{Path: root})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range inv.DataPipelines {
		t.Fatalf("unexpected data-pipeline change from prose: %#v", f)
	}
}

func TestInventoryDetectsMultiEcosystemMigrationFrameworks(t *testing.T) {
	cases := []struct {
		rel    string
		system string
		native string
	}{
		{"database/migrations/2024_01_01_000000_create_users_table.php", "laravel", "php artisan migrate"},
		{"priv/repo/migrations/20240101000000_create_accounts.exs", "ecto", "mix ecto.migrate"},
		{"migrations/2024-01-01-000000_create_users/up.sql", "diesel", "diesel migration run"},
		{"diesel.toml", "diesel", "diesel migration run"},
		{"knexfile.js", "knex", "npx knex migrate:latest"},
		{".sequelizerc", "sequelize", "npx sequelize-cli db:migrate"},
		{"src/Migrations/Version20240101000000.php", "doctrine", "php bin/console doctrine:migrations:migrate"},
		{"db/animals_migrate/20240101000000_create_pets.rb", "rails-multi-db", "bundle exec rails db:migrate:animals"},
	}
	for _, c := range cases {
		root := t.TempDir()
		writeFile(t, root, c.rel, "-- migration body\nDROP TABLE legacy;\n")
		inv, err := InventoryPath(InventoryOptions{Path: root})
		if err != nil {
			t.Fatalf("%s: %v", c.rel, err)
		}
		found := false
		for _, s := range inv.MigrationSystems {
			if s.Kind == c.system {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s: expected migration system %q, got %#v", c.rel, c.system, inv.MigrationSystems)
		}
		nativeFound := false
		for _, cmd := range inv.NativeCommands {
			if cmd.Command == c.native {
				nativeFound = true
			}
		}
		if !nativeFound {
			t.Fatalf("%s: expected native command %q, got %#v", c.rel, c.native, inv.NativeCommands)
		}
	}
}

func TestInventoryDetectsMonorepoPackageBoundaries(t *testing.T) {
	root := t.TempDir()
	// Bazel workspace with two packages.
	writeFile(t, root, "WORKSPACE", "workspace(name = \"mono\")\n")
	writeFile(t, root, "services/billing/BUILD.bazel", "go_library(name = \"billing\")\n")
	writeFile(t, root, "services/ledger/BUILD.bazel", "go_library(name = \"ledger\")\n")
	// Maven module.
	writeFile(t, root, "java/pom.xml", "<project><artifactId>core</artifactId></project>\n")
	// Gradle subproject.
	writeFile(t, root, "android/build.gradle", "apply plugin: 'java'\n")
	// Go workspace + module.
	writeFile(t, root, "go.work", "go 1.22\nuse ./svc\n")
	writeFile(t, root, "svc/go.mod", "module example.com/svc\n")
	// Nx workspace + JS package.
	writeFile(t, root, "nx.json", "{\"npmScope\":\"mono\"}\n")
	writeFile(t, root, "apps/web/project.json", "{\"name\":\"web\"}\n")

	inv, err := InventoryPath(InventoryOptions{Path: root})
	if err != nil {
		t.Fatal(err)
	}
	systems := map[string]int{}
	paths := map[string]bool{}
	for _, b := range inv.PackageBoundaries {
		systems[b.System]++
		paths[b.System+":"+b.Path] = true
	}
	for _, want := range []string{"bazel", "maven", "gradle", "go-workspace", "nx"} {
		if systems[want] == 0 {
			t.Fatalf("expected %s package boundary, got %#v", want, inv.PackageBoundaries)
		}
	}
	for _, want := range []string{"bazel:services/billing", "bazel:services/ledger", "maven:java", "gradle:android", "go-workspace:svc", "nx:apps/web"} {
		if !paths[want] {
			t.Fatalf("expected boundary %s, got %#v", want, inv.PackageBoundaries)
		}
	}
}

func TestInventoryIgnoresBuildFilesWithoutWorkspace(t *testing.T) {
	root := t.TempDir()
	// A lone BUILD.bazel with no WORKSPACE marker must not be treated as a Bazel package.
	writeFile(t, root, "docs/BUILD.bazel", "# unrelated\n")
	writeFile(t, root, "web/package.json", "{\"name\":\"web\"}\n")
	inv, err := InventoryPath(InventoryOptions{Path: root})
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range inv.PackageBoundaries {
		if b.System == "bazel" {
			t.Fatalf("did not expect bazel boundary without workspace marker: %#v", inv.PackageBoundaries)
		}
		if b.System == "nx" || b.System == "turborepo" {
			t.Fatalf("did not expect JS package boundary without nx/turbo workspace: %#v", inv.PackageBoundaries)
		}
	}
}

func TestFetchLocalMercurialRecordsVCSAndCaches(t *testing.T) {
	src := t.TempDir()
	writeFile(t, src, "db/migrate/001.sql", "drop table accounts;")
	// A faithful Mercurial dirstate header: 20-byte parent node id followed by data.
	node := make([]byte, 40)
	for i := range node {
		node[i] = byte(i + 1)
	}
	writeFile(t, src, ".hg/dirstate", string(node))
	writeFile(t, src, ".hg/requires", "revlogv1\n")

	cacheDir := filepath.Join(t.TempDir(), "cache")
	first, err := Fetch(context.Background(), FetchOptions{Input: src, OutDir: filepath.Join(t.TempDir(), "a"), DownloadDir: cacheDir})
	if err != nil {
		t.Fatal(err)
	}
	if first.Source.VCS != "mercurial" {
		t.Fatalf("expected mercurial vcs, got %#v", first.Source)
	}
	if first.Source.ResolvedCommit == "" || first.Source.ArchiveHash == "" {
		t.Fatalf("expected provenance revision and tree hash, got %#v", first.Source)
	}
	if first.Source.CacheHit {
		t.Fatalf("first fetch must be a cache miss")
	}
	second, err := Fetch(context.Background(), FetchOptions{Input: src, OutDir: filepath.Join(t.TempDir(), "b"), DownloadDir: cacheDir})
	if err != nil {
		t.Fatal(err)
	}
	if !second.Source.CacheHit || second.Source.ArchiveHash != first.Source.ArchiveHash {
		t.Fatalf("expected content-addressed cache hit, got first=%#v second=%#v", first.Source, second.Source)
	}
	// .hg metadata must not leak into the scanned tree hash basis.
	if _, err := os.Stat(filepath.Join(filepath.FromSlash(second.Source.ScannedRoot), ".hg", "dirstate")); err == nil {
		// copyDir may copy it, but the hash must ignore it: mutate metadata and confirm stable hash.
		writeFile(t, src, ".hg/dirstate", string(node)+"changed-metadata")
		third, err := Fetch(context.Background(), FetchOptions{Input: src, OutDir: filepath.Join(t.TempDir(), "c"), DownloadDir: cacheDir})
		if err != nil {
			t.Fatal(err)
		}
		if third.Source.ArchiveHash != first.Source.ArchiveHash {
			t.Fatalf("tree hash must ignore VCS metadata: %s vs %s", third.Source.ArchiveHash, first.Source.ArchiveHash)
		}
	}
}

func TestFetchLocalFossilRecordsVCS(t *testing.T) {
	src := t.TempDir()
	writeFile(t, src, "migrations/002.sql", "alter table users drop column email;")
	writeFile(t, src, "_FOSSIL_", "fossil checkout database stub bytes for revision derivation")
	result, err := Fetch(context.Background(), FetchOptions{Input: src, OutDir: filepath.Join(t.TempDir(), "f"), DownloadDir: filepath.Join(t.TempDir(), "cache")})
	if err != nil {
		t.Fatal(err)
	}
	if result.Source.VCS != "fossil" {
		t.Fatalf("expected fossil vcs, got %#v", result.Source)
	}
	if result.Source.ResolvedCommit == "" {
		t.Fatalf("expected fossil revision provenance, got %#v", result.Source)
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

	result, err := Fetch(context.Background(), FetchOptions{Input: src, OutDir: filepath.Join(t.TempDir(), "fetched"), DownloadDir: filepath.Join(t.TempDir(), "cache")})
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

func TestInventoryScansKubernetesAndTerraformInfrastructure(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "k8s/jobs/migrate.yaml", `apiVersion: batch/v1
kind: Job
metadata:
  name: billing-migrate
spec:
  template:
    spec:
      initContainers:
      - name: wait-db
        image: postgres:16
      containers:
      - name: migrate
        image: app
        command: ["bundle", "exec", "rails", "db:migrate"]
        env:
        - name: DATABASE_URL
          valueFrom:
            secretKeyRef:
              name: billing-db
              key: url
`)
	writeFile(t, root, "k8s/jobs/repair-cron.yaml", `apiVersion: batch/v1
kind: CronJob
metadata:
  name: account-repair
spec:
  schedule: "*/15 * * * *"
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: repair
            args: ["python", "manage.py", "reconcile_accounts", "--rollback-window=1h"]
`)
	writeFile(t, root, "infra/main.tf", `resource "helm_release" "app" {
  name       = "billing"
  wait       = true
  atomic     = true
  depends_on = [kubernetes_job.billing_migrate]
  set_sensitive {
    name  = "database.password"
    value = var.database_password
  }
}

resource "kubernetes_job" "billing_migrate" {
  metadata { name = "billing-migrate" }
  spec {
    template {
      spec {
        container {
          name    = "migrate"
          command = ["alembic", "upgrade", "head"]
        }
      }
    }
  }
}
`)

	inv, err := InventoryPath(InventoryOptions{Path: root})
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{
		"kubernetes_migration_job",
		"kubernetes_database_job",
		"kubernetes_secret_reference",
		"kubernetes_deploy_ordering",
		"kubernetes_cron_repair",
		"terraform_migration_job",
		"terraform_secret_reference",
		"terraform_deploy_ordering",
	} {
		if !hasFindingKind(inv.Infrastructure, kind) {
			t.Fatalf("missing infrastructure finding %s in %#v", kind, inv.Infrastructure)
		}
	}
	if inv.SummaryByCategory["infrastructure"] != len(inv.Infrastructure) {
		t.Fatalf("expected infrastructure summary count, got %#v", inv.SummaryByCategory)
	}
	baseline := Baseline(inv, inv.Facts, intake.Report{Source: intake.Source{Input: root, ScannedRoot: root}})
	if baseline.Summary.InfraFindings < 8 || baseline.Summary.InfraMigrationJobs == 0 || baseline.Summary.InfraSecretRefs == 0 || baseline.Summary.InfraDeployOrdering == 0 || baseline.Summary.InfraCronRepairs == 0 {
		t.Fatalf("expected baseline infrastructure summary, got %#v findings=%#v", baseline.Summary, baseline.Infrastructure)
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

func TestLanguageAwareTestPlacement(t *testing.T) {
	tests := []struct {
		name     string
		context  ProposalContext
		risk     ProposalRiskContext
		wantPath string
		wantText string
	}{
		{
			name:     "rails",
			risk:     ProposalRiskContext{ID: "risk:1", Path: "db/migrate/001_drop_users.rb", Table: "users"},
			wantPath: "test/patchline/risk-1-db-migrate-001-drop-users-rb_test.rb",
			wantText: "# Untrusted generated test proposal",
		},
		{
			name:     "django",
			context:  ProposalContext{NativeChecks: []Command{{Command: "python manage.py test"}}},
			risk:     ProposalRiskContext{ID: "risk:2", Path: "django/contrib/auth/migrations/0001_initial.py", Table: "auth_user"},
			wantPath: "tests/patchline/test_risk-2-django-contrib-auth-migrations-0001-initial-py.py",
			wantText: "# Untrusted generated test proposal",
		},
		{
			name:     "go",
			context:  ProposalContext{NativeChecks: []Command{{Command: "go test ./..."}}},
			risk:     ProposalRiskContext{ID: "risk:3", Path: "backend/migrator/migration/001.sql", Table: "instances"},
			wantPath: "patchline-proposals/tests/risk-3-backend-migrator-migration-001-sql_test.go",
			wantText: "// Untrusted generated test proposal",
		},
		{
			name:     "java",
			risk:     ProposalRiskContext{ID: "risk:4", Path: "src/main/resources/db/migration/V1__init.sql", Table: "accounts"},
			wantPath: "src/test/java/patchline/Risk4SrcMainResourcesDbMigrationV1InitSqlTest.java",
			wantText: "// Untrusted generated test proposal",
		},
		{
			name:     "node",
			context:  ProposalContext{NativeChecks: []Command{{Command: "npm test"}}},
			risk:     ProposalRiskContext{ID: "risk:5", Path: "packages/prisma/migrations/001/migration.sql", Table: "users"},
			wantPath: "test/patchline/risk-5-packages-prisma-migrations-001-migration-sql.test.js",
			wantText: "// Untrusted generated test proposal",
		},
		{
			name:     "fallback",
			risk:     ProposalRiskContext{ID: "risk:6", Path: "migrations/001.sql", Table: "users"},
			wantPath: "patchline-proposals/tests/risk-6-migrations-001-sql.md",
			wantText: "# Untrusted generated test proposal",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slug := safeProposalSlug(tt.risk.ID + "-" + tt.risk.Path)
			placement := languageAwareTestPlacement(tt.context, tt.risk, slug)
			if placement.Path != tt.wantPath {
				t.Fatalf("path mismatch: got %q want %q", placement.Path, tt.wantPath)
			}
			content := renderTestProposal(tt.risk, placement.Language)
			if !strings.Contains(content, tt.wantText) {
				t.Fatalf("expected %q in content:\n%s", tt.wantText, content)
			}
		})
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

func TestExtractTarGzRejectsUnsafePaths(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	header := &tar.Header{Name: "../escape.txt", Mode: 0o644, Size: int64(len("bad"))}
	if err := tw.WriteHeader(header); err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(tw, "bad"); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := extractTarGz(bytes.NewReader(buf.Bytes()), t.TempDir()); err == nil {
		t.Fatalf("expected unsafe tar path to fail")
	}
}

func TestExtractArchivesIgnoreSymlinkEscapes(t *testing.T) {
	t.Run("tar.gz", func(t *testing.T) {
		base := t.TempDir()
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		tw := tar.NewWriter(gz)
		if err := tw.WriteHeader(&tar.Header{Name: "repo/link", Typeflag: tar.TypeSymlink, Linkname: "../../escape.txt", Mode: 0o777}); err != nil {
			t.Fatal(err)
		}
		if err := tw.WriteHeader(&tar.Header{Name: "repo/good.sql", Typeflag: tar.TypeReg, Mode: 0o644, Size: int64(len("select 1;"))}); err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(tw, "select 1;"); err != nil {
			t.Fatal(err)
		}
		if err := tw.Close(); err != nil {
			t.Fatal(err)
		}
		if err := gz.Close(); err != nil {
			t.Fatal(err)
		}
		root, _, err := extractTarGz(bytes.NewReader(buf.Bytes()), filepath.Join(base, "extract"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := os.Lstat(filepath.Join(root, "link")); !os.IsNotExist(err) {
			t.Fatalf("expected tar symlink entry to be skipped, got err=%v", err)
		}
		if _, err := os.Stat(filepath.Join(base, "escape.txt")); !os.IsNotExist(err) {
			t.Fatalf("symlink escape created outside file, err=%v", err)
		}
	})
	t.Run("zip", func(t *testing.T) {
		base := t.TempDir()
		archive := filepath.Join(base, "symlink.zip")
		file, err := os.Create(archive)
		if err != nil {
			t.Fatal(err)
		}
		zw := zip.NewWriter(file)
		linkHeader := &zip.FileHeader{Name: "repo/link"}
		linkHeader.SetMode(os.ModeSymlink | 0o777)
		w, err := zw.CreateHeader(linkHeader)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte("../../escape.txt")); err != nil {
			t.Fatal(err)
		}
		good, err := zw.Create("repo/good.sql")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := good.Write([]byte("select 1;")); err != nil {
			t.Fatal(err)
		}
		if err := zw.Close(); err != nil {
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		root, err := extractZip(archive, filepath.Join(base, "extract"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := os.Lstat(filepath.Join(root, "link")); !os.IsNotExist(err) {
			t.Fatalf("expected zip symlink entry to be skipped, got err=%v", err)
		}
		if _, err := os.Stat(filepath.Join(base, "escape.txt")); !os.IsNotExist(err) {
			t.Fatalf("symlink escape created outside file, err=%v", err)
		}
	})
}

func TestExtractArchivesRejectMalformedInputs(t *testing.T) {
	if _, _, err := extractTarGz(bytes.NewReader([]byte("not a gzip tar")), t.TempDir()); err == nil {
		t.Fatalf("expected malformed tar.gz to fail")
	}
	archive := filepath.Join(t.TempDir(), "bad.zip")
	if err := os.WriteFile(archive, []byte("not a zip"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := extractZip(archive, t.TempDir()); err == nil {
		t.Fatalf("expected malformed zip to fail")
	}
}

func TestExtractArchivesRejectBombs(t *testing.T) {
	t.Run("tar oversized content", func(t *testing.T) {
		withArchiveLimits(t, archiveExtractionLimits{MaxEntries: 10, MaxUncompressed: 3})
		data := tarGzForTest(t, map[string]string{"repo/big.sql": "select 1;"})
		_, _, err := extractTarGz(bytes.NewReader(data), t.TempDir())
		if err == nil || !strings.Contains(err.Error(), "uncompressed size exceeds") {
			t.Fatalf("expected tar size bomb rejection, got %v", err)
		}
	})
	t.Run("tar too many entries", func(t *testing.T) {
		withArchiveLimits(t, archiveExtractionLimits{MaxEntries: 1, MaxUncompressed: 1024})
		data := tarGzForTest(t, map[string]string{"repo/a.sql": "select 1;", "repo/b.sql": "select 2;"})
		_, _, err := extractTarGz(bytes.NewReader(data), t.TempDir())
		if err == nil || !strings.Contains(err.Error(), "too many entries") {
			t.Fatalf("expected tar entry bomb rejection, got %v", err)
		}
	})
	t.Run("zip oversized content", func(t *testing.T) {
		withArchiveLimits(t, archiveExtractionLimits{MaxEntries: 10, MaxUncompressed: 3})
		archive := zipForTest(t, map[string]string{"repo/big.sql": "select 1;"})
		_, err := extractZip(archive, t.TempDir())
		if err == nil || !strings.Contains(err.Error(), "uncompressed size exceeds") {
			t.Fatalf("expected zip size bomb rejection, got %v", err)
		}
	})
	t.Run("zip too many entries", func(t *testing.T) {
		withArchiveLimits(t, archiveExtractionLimits{MaxEntries: 1, MaxUncompressed: 1024})
		archive := zipForTest(t, map[string]string{"repo/a.sql": "select 1;", "repo/b.sql": "select 2;"})
		_, err := extractZip(archive, t.TempDir())
		if err == nil || !strings.Contains(err.Error(), "too many entries") {
			t.Fatalf("expected zip entry bomb rejection, got %v", err)
		}
	})
}

func TestExtractArchivesAcceptValidRepoFiles(t *testing.T) {
	tarRoot, top, err := extractTarGz(bytes.NewReader(tarGzForTest(t, map[string]string{
		"repo/db/migrate/001.sql": "select 1;",
	})), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if top != "repo" {
		t.Fatalf("expected tar top repo, got %q", top)
	}
	if content, err := os.ReadFile(filepath.Join(tarRoot, "db", "migrate", "001.sql")); err != nil || string(content) != "select 1;" {
		t.Fatalf("expected tar file content, got %q err=%v", string(content), err)
	}
	zipRoot, err := extractZip(zipForTest(t, map[string]string{
		"repo/db/migrate/001.sql": "select 1;",
	}), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if content, err := os.ReadFile(filepath.Join(zipRoot, "db", "migrate", "001.sql")); err != nil || string(content) != "select 1;" {
		t.Fatalf("expected zip file content, got %q err=%v", string(content), err)
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
	writeFile(t, root, "app/jobs/transactional_repair.rb", `class TransactionalRepairJob
  def perform
    ActiveRecord::Base.transaction do
      Account.where(id: account_id).update_all(disabled: false)
    end
  end
end`)
	writeFile(t, root, "app/jobs/idempotent_backfill.rb", `class IdempotentBackfill
  def perform
    Account.upsert_all(rows, unique_by: :index_accounts_on_id)
  end
end`)
	inv := Inventory{
		Root: filepath.ToSlash(root),
		Facts: []Fact{
			{Version: Version, ID: "fact:account", Kind: "file", Path: "app/jobs/repair.rb", Confidence: "observed", Identifiers: []Identifier{{Kind: "table", Value: "accounts"}}},
			{Version: Version, ID: "fact:transactional-account", Kind: "file", Path: "app/jobs/transactional_repair.rb", Confidence: "observed", Identifiers: []Identifier{{Kind: "table", Value: "accounts"}}},
			{Version: Version, ID: "fact:idempotent-account", Kind: "file", Path: "app/jobs/idempotent_backfill.rb", Confidence: "observed", Identifiers: []Identifier{{Kind: "table", Value: "accounts"}}},
		},
	}
	report := intake.Report{
		Source: intake.Source{Input: "fixture", ScannedRoot: filepath.ToSlash(root)},
		SourceSQL: migration.SourceSQLReport{
			Root: filepath.ToSlash(root),
			Observations: []migration.SourceSQLObservation{
				{
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
				},
				{
					Path:        "app/jobs/transactional_repair.rb",
					Language:    "Ruby",
					Detector:    "ruby.rails-active-record",
					Line:        4,
					Kind:        "orm_query",
					Framework:   "rails",
					Operation:   "update",
					Table:       "accounts",
					Confidence:  "medium",
					SnippetHash: "transactional-snippet-hash",
				},
				{
					Path:        "app/jobs/idempotent_backfill.rb",
					Language:    "Ruby",
					Detector:    "ruby.rails-active-record",
					Line:        3,
					Kind:        "orm_query",
					Framework:   "rails",
					Operation:   "insert",
					Table:       "accounts",
					Confidence:  "medium",
					SnippetHash: "idempotent-snippet-hash",
				},
			},
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
	if baseline.Summary.Transactions == 0 || baseline.Summary.TransactionMissing == 0 || baseline.Summary.TransactionExplicit == 0 {
		t.Fatalf("expected missing and explicit transaction boundaries: %#v", baseline.Transactions)
	}
	if baseline.Summary.IdempotencyClasses == 0 || baseline.Summary.IdempotencyProven == 0 || baseline.Summary.IdempotencyUnsafe == 0 {
		t.Fatalf("expected proven and non-idempotent classifications: %#v", baseline.Idempotency)
	}
}

func TestBaselineClassifiesRunbookIdempotencyWithoutRisk(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "docs/runbook.md", "Runbook command: rerun the cleanup until complete. The command is idempotent and safe to retry.")
	inv := Inventory{
		Root: filepath.ToSlash(root),
		Facts: []Fact{{
			Version:    Version,
			ID:         "fact:runbook",
			Kind:       "operational_doc",
			Path:       "docs/runbook.md",
			Confidence: "path",
			Rationale:  "runbook path",
		}},
	}
	baseline := Baseline(inv, inv.Facts, intake.Report{Source: intake.Source{Input: "fixture", ScannedRoot: filepath.ToSlash(root)}})
	if baseline.Summary.IdempotencyClasses != 1 || baseline.Summary.IdempotencyProven != 1 {
		t.Fatalf("expected proven runbook idempotency classification: summary=%#v classes=%#v", baseline.Summary, baseline.Idempotency)
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
	if baseline.Summary.RankingExplanations == 0 || baseline.Summary.RankingFeatures == 0 {
		t.Fatalf("expected inspectable ranking explanations: %#v", baseline.Summary)
	}
	if baseline.Summary.PolicyChecks == 0 || baseline.Summary.PolicyFailed == 0 {
		t.Fatalf("expected failing policy obligations for high-risk changes: %#v", baseline.Summary)
	}
	if baseline.Summary.RepairProofs == 0 || baseline.Summary.RepairProofRefuted == 0 {
		t.Fatalf("expected proof-carrying repair summaries with unresolved scope/frame obligations: %#v", baseline.Summary)
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

func TestRemediationPlaybookMapsHazardsToRunbookRollbackAndOwners(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".github/CODEOWNERS", "/db/migrate/ @org/db-team\n")
	writeFile(t, root, "db/migrate/001_accounts.sql", "UPDATE accounts SET status = 'disabled';\n")
	writeFile(t, root, "docs/inc-42.md", "Incident 42 accounts rollback runbook mentions accounts.")
	inv, err := InventoryPath(InventoryOptions{Path: root})
	if err != nil {
		t.Fatal(err)
	}
	intakeReport, err := intake.Run(context.Background(), intake.Options{Path: root})
	if err != nil {
		t.Fatal(err)
	}
	baseline := Baseline(inv, inv.Facts, intakeReport)
	if baseline.Summary.LockHazards == 0 || baseline.Summary.ProofMinimizations == 0 {
		t.Fatalf("fixture should exercise hazard subreports: %#v", baseline.Summary)
	}
	report := BuildRemediationPlaybook(baseline)
	again := BuildRemediationPlaybook(baseline)
	if report.Hash == "" || report.Hash != again.Hash {
		t.Fatalf("expected deterministic non-empty playbook hash: %q vs %q", report.Hash, again.Hash)
	}
	if report.Version != RemediationPlaybookVersion || report.Summary.Playbooks == 0 || report.Summary.RollbackPoints == 0 || report.Summary.OwnerHandoffs == 0 {
		t.Fatalf("unexpected playbook summary: %#v", report)
	}
	var found RemediationPlaybook
	for _, playbook := range report.Playbooks {
		if playbook.Table == "accounts" {
			found = playbook
			break
		}
	}
	if found.ID == "" {
		t.Fatalf("missing accounts playbook: %#v", report.Playbooks)
	}
	for _, class := range []string{"broad-write", "transaction-boundary", "idempotency", "lock-concurrency", "proof-hole"} {
		if !playbookHasHazardClass(found, class) {
			t.Fatalf("expected hazard class %q in %#v", class, found.HazardClasses)
		}
	}
	if !playbookHasOwner(found, "@org/db-team") {
		t.Fatalf("expected CODEOWNERS handoff in %#v", found.OwnerHandoffs)
	}
	if !playbookHasRollbackStage(found, "before-execution") || !playbookHasRollbackStage(found, "during-execution") {
		t.Fatalf("expected before and during rollback points: %#v", found.RollbackPoints)
	}
	if len(found.RunbookSteps) < 4 || !strings.Contains(report.Markdown, "owner handoffs") || !strings.Contains(report.Markdown, "Rollback points") {
		t.Fatalf("expected markdown and runbook steps, got steps=%#v markdown=%s", found.RunbookSteps, report.Markdown)
	}
}

func TestRemediationPlaybookHandlesEmptyBaseline(t *testing.T) {
	report := BuildRemediationPlaybook(BaselineReport{Version: BaselineVersion, Hash: "baseline-empty"})
	if report.Version != RemediationPlaybookVersion || report.Summary.Playbooks != 0 || report.Hash == "" {
		t.Fatalf("unexpected empty playbook report: %#v", report)
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
	if !proposal.Deterministic || proposal.Generator != "patchline-template" {
		t.Fatalf("expected deterministic template proposal: %#v", proposal)
	}
	if proposal.Intervention.ID == "" || proposal.Intervention.Stage != "generated-untrusted" || len(proposal.Intervention.RequiredReanalysis) == 0 {
		t.Fatalf("expected generated proposal to be recorded as an untrusted intervention: %#v", proposal.Intervention)
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
	budgeted, err := Propose(ProposalOptions{BaselinePath: baselineDir, Kind: "all", Budget: "files=2,lines=8,tokens=300,changes=1", BudgetRisks: 3})
	if err != nil {
		t.Fatal(err)
	}
	if budgeted.ScopeBudget.MaxFiles != 2 || budgeted.BudgetRisks != 1 || len(budgeted.GeneratedFiles) != 2 || len(budgeted.Warnings) == 0 {
		t.Fatalf("expected bounded proposal artifacts: %#v", budgeted)
	}
	for _, artifact := range budgeted.Generated {
		if len(strings.Split(strings.TrimSuffix(artifact.Content, "\n"), "\n")) > 8 {
			t.Fatalf("artifact exceeded line budget: %s\n%s", artifact.Path, artifact.Content)
		}
	}
	if _, err := Propose(ProposalOptions{BaselinePath: baselineDir, Kind: "all", Budget: "files=nope"}); err == nil {
		t.Fatal("expected invalid budget to fail")
	}
}

func TestProposeAddsSanitizedProvenanceComments(t *testing.T) {
	baseline := BaselineReport{
		Version: BaselineVersion,
		Hash:    "baseline-hash",
		Risks: []BaselineRisk{{
			ID:        "risk:top",
			Path:      "db/migrate/001_update_accounts.sql",
			Kind:      "update",
			Table:     "accounts",
			Severity:  "high",
			Score:     120,
			Rationale: "unbounded update",
		}},
		EvidenceLinks: []EvidenceLink{{
			RiskID:   "risk:top",
			FactID:   "fact:accounts-source",
			FactKind: "source",
			Path:     "app/services/customer_secret_token_repair.go",
		}},
		Provenance: []ProvenanceSlice{{
			ID:            "slice:top",
			RiskID:        "risk:top",
			MigrationPath: "db/migrate/001_update_accounts.sql",
			SourcePaths:   []string{"app/jobs/account_backfill.go"},
			Links: []EvidenceLink{{
				RiskID:   "risk:top",
				FactID:   "fact:accounts-migration",
				FactKind: "migration",
				Path:     "config/private_key/accounts.sql",
			}},
		}},
	}
	baseline.Hash = baselineHash(baseline)
	baselineDir := filepath.Join(t.TempDir(), "baseline")
	if err := WriteBaseline(baselineDir, baseline); err != nil {
		t.Fatal(err)
	}
	proposal, err := Propose(ProposalOptions{BaselinePath: baselineDir, Kind: "guards", BudgetRisks: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(proposal.Generated) != 1 {
		t.Fatalf("expected one guard proposal, got %#v", proposal.Generated)
	}
	content := proposal.Generated[0].Content
	for _, want := range []string{"-- risk: risk:top", "-- fact-hashes: sha256:", "-- evidence-paths:", "redacted-"} {
		if !strings.Contains(content, want) {
			t.Fatalf("expected generated guard to contain %q:\n%s", want, content)
		}
	}
	for _, leaked := range []string{"customer_secret_token_repair", "private_key"} {
		if strings.Contains(content, leaked) || strings.Contains(proposal.Prompt, leaked) {
			t.Fatalf("generated provenance leaked secret-like path component %q\ncontent:\n%s\nprompt:\n%s", leaked, content, proposal.Prompt)
		}
	}
}

func TestMinimizeGeneratedProposalRemovesNonImprovingArtifacts(t *testing.T) {
	baseline := BaselineReport{
		Version: BaselineVersion,
		Hash:    "baseline-hash",
		Risks: []BaselineRisk{{
			ID:        "risk:accounts",
			Path:      "db/migrate/001.sql",
			Kind:      "update",
			Table:     "accounts",
			Severity:  "high",
			Score:     120,
			Rationale: "unbounded update",
		}},
	}
	baseline.Hash = baselineHash(baseline)
	baselineDir := filepath.Join(t.TempDir(), "baseline")
	if err := WriteBaseline(baselineDir, baseline); err != nil {
		t.Fatal(err)
	}
	proposal, err := Propose(ProposalOptions{BaselinePath: baselineDir, Kind: "all", BudgetRisks: 1})
	if err != nil {
		t.Fatal(err)
	}
	proposal.Generated = append(proposal.Generated,
		GeneratedArtifact{Path: "patchline-proposals/duplicates/redundant.sql", Kind: proposal.Generated[0].Kind, Content: proposal.Generated[0].Content, RiskIDs: append([]string(nil), proposal.Generated[0].RiskIDs...)},
		GeneratedArtifact{Path: "patchline-proposals/orphans/no-coverage.md", Kind: "tests", Content: "# untrusted generated\n\nSuggested assertions:\n", RiskIDs: []string{"risk:not-in-baseline"}},
	)
	proposal.GeneratedFiles = generatedFilesForArtifacts(proposal.Generated)
	proposal.Patch = renderProposalPatch(proposal.Generated)
	minimized := MinimizeGeneratedProposal(baseline, proposal)
	if !minimized.Minimization.Applied || minimized.Minimization.RemovedFiles == 0 || len(minimized.Generated) >= len(proposal.Generated) {
		t.Fatalf("expected generated proposal to be minimized: %#v", minimized.Minimization)
	}
	reasons := map[string]bool{}
	for _, removed := range minimized.Minimization.Removed {
		reasons[removed.Reason] = true
	}
	if !reasons["no-new-risk-coverage"] || !reasons["no-target-risk-coverage"] {
		t.Fatalf("expected non-improving artifacts to be removed, got %#v", minimized.Minimization.Removed)
	}
	compare := Compare(baseline, minimized)
	if compare.Summary.RisksWithCoverage != 1 || compare.Summary.PatchlineChecksFailed != 0 {
		t.Fatalf("minimized proposal should preserve coverage and checks: %#v", compare.Summary)
	}
	if !strings.Contains(minimized.Markdown, "## Minimization") {
		t.Fatalf("expected minimization markdown, got %s", minimized.Markdown)
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
	if proposal.Deterministic {
		t.Fatalf("expected llm-command proposal to be marked non-deterministic: %#v", proposal)
	}
	if proposal.PromptMode != "fact-grounded" || !strings.Contains(proposal.Generated[0].Content, "accounts") {
		t.Fatalf("expected fact-grounded prompt to include risk facts: mode=%q content=%s", proposal.PromptMode, proposal.Generated[0].Content)
	}
	noFacts, err := Propose(ProposalOptions{BaselinePath: baselineDir, Kind: "tests", LLMCommand: "cat", PromptNoFacts: true, BudgetRisks: 1})
	if err != nil {
		t.Fatal(err)
	}
	if noFacts.PromptMode != "without-facts" || strings.Contains(noFacts.Generated[0].Content, "accounts") || !strings.Contains(noFacts.Generated[0].Content, "withheld for ablation") {
		t.Fatalf("expected fact-free prompt to withhold risk facts: mode=%q content=%s", noFacts.PromptMode, noFacts.Generated[0].Content)
	}
	if _, err := Propose(ProposalOptions{BaselinePath: baselineDir, Kind: "tests", LLMCommand: "cat", NoLLM: true, BudgetRisks: 1}); err == nil {
		t.Fatal("expected --no-llm to reject llm-command")
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
	if compare.Summary.InterventionLoops != 1 || compare.Summary.InterventionAccepted != 1 || compare.Intervention.Status != "accepted-for-review" {
		t.Fatalf("expected accepted intervention loop: summary=%#v loop=%#v", compare.Summary, compare.Intervention)
	}
	if !strings.Contains(compare.Markdown, "Patchline repo compare") {
		t.Fatalf("expected compare markdown, got %q", compare.Markdown)
	}
}

func TestBaselineClassifiesLockConcurrencyHazards(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "db/migrate/001_blocking.sql", "ALTER TABLE accounts ADD COLUMN processed_at timestamptz DEFAULT now();\nCREATE INDEX idx_accounts_email ON accounts(email);\n")
	writeFile(t, root, "app/jobs/account_backfill.rb", "class AccountBackfill\n  include Sidekiq::Worker\n  def perform\n    Account.where(active: true).update_all(flagged: true)\n  end\nend\n")
	inv, err := InventoryPath(InventoryOptions{Path: root})
	if err != nil {
		t.Fatal(err)
	}
	intakeReport, err := intake.Run(context.Background(), intake.Options{Path: root})
	if err != nil {
		t.Fatal(err)
	}
	baseline := Baseline(inv, inv.Facts, intakeReport)
	if baseline.Summary.LockHazards == 0 || baseline.Summary.LockHazardHigh == 0 {
		t.Fatalf("expected high lock/concurrency hazards: summary=%#v hazards=%#v", baseline.Summary, baseline.LockHazards)
	}
	if !hasLockHazardMarker(baseline.LockHazards, "table-rewrite") || !hasLockHazardMarker(baseline.LockHazards, "job-contention") {
		t.Fatalf("expected schema and job contention markers: %#v", baseline.LockHazards)
	}
	if !strings.Contains(baseline.Markdown, "Lock and concurrency hazards") {
		t.Fatalf("expected lock hazards in markdown:\n%s", baseline.Markdown)
	}
}

func TestCompareClassifiesGeneratedLockHazards(t *testing.T) {
	baseline := BaselineReport{
		Version: BaselineVersion,
		Hash:    "baseline-hash",
		Risks: []BaselineRisk{{
			ID:       "risk:accounts",
			Path:     "db/migrate/001.sql",
			Kind:     "alter",
			Table:    "accounts",
			Severity: "high",
			Score:    120,
		}},
	}
	proposal := ProposalReport{
		OutputHash:    "proposal-hash",
		TargetRiskIDs: []string{"risk:accounts"},
		GeneratedFiles: []GeneratedFile{{
			Path:    "patchline-proposals/repair/blocking.sql",
			Kind:    "repair",
			RiskIDs: []string{"risk:accounts"},
		}},
		Generated: []GeneratedArtifact{{
			Path:    "patchline-proposals/repair/blocking.sql",
			Kind:    "repair",
			RiskIDs: []string{"risk:accounts"},
			Content: "ALTER TABLE accounts ADD COLUMN repaired_at timestamptz DEFAULT now();\nLOCK TABLE accounts IN ACCESS EXCLUSIVE MODE;\n",
		}},
	}
	compare := Compare(baseline, proposal)
	if compare.Summary.LockHazards != 1 || compare.Summary.LockHazardCritical != 1 {
		t.Fatalf("expected generated critical lock hazard: summary=%#v hazards=%#v", compare.Summary, compare.LockHazards)
	}
	if !strings.Contains(compare.Markdown, "Generated lock and concurrency hazards") {
		t.Fatalf("expected generated lock hazards in markdown:\n%s", compare.Markdown)
	}
}

func TestBaselineClassifiesDataRetentionPrivacyHazards(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "db/migrate/001_privacy.sql", "DELETE FROM users;\nUPDATE customer_profiles SET email = NULL, phone = NULL;\n")
	writeFile(t, root, "scripts/export_users.py", "import pandas as pd\nusers.to_csv('users.csv')\n")
	writeFile(t, root, "docs/privacy-runbook.md", "GDPR erasure runbook: dry-run first, snapshot backup, then delete users older than retention_days.")
	inv, err := InventoryPath(InventoryOptions{Path: root})
	if err != nil {
		t.Fatal(err)
	}
	intakeReport, err := intake.Run(context.Background(), intake.Options{Path: root})
	if err != nil {
		t.Fatal(err)
	}
	baseline := Baseline(inv, inv.Facts, intakeReport)
	if baseline.Summary.PrivacyHazards == 0 || baseline.Summary.PrivacyHigh == 0 {
		t.Fatalf("expected privacy hazards: summary=%#v hazards=%#v", baseline.Summary, baseline.PrivacyHazards)
	}
	if !hasPrivacyHazardMarker(baseline.PrivacyHazards, "broad-delete") || !hasPrivacyHazardMarker(baseline.PrivacyHazards, "export-script") {
		t.Fatalf("expected delete and export markers: %#v", baseline.PrivacyHazards)
	}
	if !strings.Contains(baseline.Markdown, "Data-retention and privacy hazards") {
		t.Fatalf("expected privacy hazards in markdown:\n%s", baseline.Markdown)
	}
}

func TestBaselineMinesInvariantCandidates(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "db/migrate/001_users.sql", "CREATE TABLE users (id int PRIMARY KEY, email text NOT NULL UNIQUE, status text CHECK (status in ('active','disabled')));")
	writeFile(t, root, "app/models/user.rb", "class User < ApplicationRecord\n  validates :email, presence: true, uniqueness: true\nend\n")
	writeFile(t, root, "spec/models/user_spec.rb", "RSpec.describe User do\n  it 'requires email' do\n    expect(user.email).not_to be_nil\n  end\nend\n")
	writeFile(t, root, "test/fixtures/users.yml", "alice:\n  id: 1\n  email: alice@example.com\n  status: active\n")
	inv, err := InventoryPath(InventoryOptions{Path: root})
	if err != nil {
		t.Fatal(err)
	}
	intakeReport, err := intake.Run(context.Background(), intake.Options{Path: root})
	if err != nil {
		t.Fatal(err)
	}
	baseline := Baseline(inv, inv.Facts, intakeReport)
	if baseline.Summary.Invariants == 0 || baseline.Summary.InvariantSchema == 0 || baseline.Summary.InvariantValidation == 0 || baseline.Summary.InvariantTests == 0 || baseline.Summary.InvariantFixtures == 0 {
		t.Fatalf("expected invariant candidates from schema, validation, tests, and fixtures: summary=%#v invariants=%#v", baseline.Summary, baseline.Invariants)
	}
	for _, want := range []string{"not-null", "unique", "check-constraint", "example-non-null"} {
		if !hasInvariantKind(baseline.Invariants, want) {
			t.Fatalf("missing invariant kind %q in %#v", want, baseline.Invariants)
		}
	}
	if !strings.Contains(baseline.Markdown, "Invariant candidates") {
		t.Fatalf("expected invariant candidates in markdown:\n%s", baseline.Markdown)
	}
}

func TestBaselineBuildsTraceToCodeLinks(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "app/jobs/account_backfill_worker.rb", "class AccountBackfillWorker\n  def perform\n    Account.where(active: true).update_all(flagged: true)\n  end\nend\n")
	writeFile(t, root, "db/migrate/001_accounts.sql", "UPDATE accounts SET flagged = true;")
	writeFile(t, root, "observability/otel.json", `{"resourceSpans":[{"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"billing"}}]},"scopeSpans":[{"spans":[{"traceId":"abc","spanId":"def","name":"AccountBackfillWorker perform","attributes":[{"key":"code.filepath","value":{"stringValue":"app/jobs/account_backfill_worker.rb"}},{"key":"db.system","value":{"stringValue":"postgresql"}},{"key":"db.sql.table","value":{"stringValue":"accounts"}}]}]}]}]}`)
	writeFile(t, root, "observability/datadog.json", `{"traces":[[{"trace_id":"123","span_id":"456","service":"billing","resource":"AccountBackfillWorker","meta":{"code.filepath":"app/jobs/account_backfill_worker.rb","db.table":"accounts"}}]]}`)
	writeFile(t, root, "logs/app.log", `time=2025-01-02T03:04:05Z level=error service=billing job=AccountBackfillWorker trace_id=abc table=accounts deploy=prod-20250102`)
	writeFile(t, root, "docs/deploy.md", "Deployment prod-20250102 commit abc123 ran AccountBackfillWorker for accounts.")
	writeFile(t, root, "docs/incident-42.md", "Incident 42 timeline: 2025-01-02T03:04:05Z deploy prod-20250102 AccountBackfillWorker updated accounts.")
	inv, err := InventoryPath(InventoryOptions{Path: root})
	if err != nil {
		t.Fatal(err)
	}
	intakeReport, err := intake.Run(context.Background(), intake.Options{Path: root})
	if err != nil {
		t.Fatal(err)
	}
	baseline := Baseline(inv, inv.Facts, intakeReport)
	if baseline.Summary.TraceCodeLinks == 0 || baseline.Summary.TraceLinkExact == 0 {
		t.Fatalf("expected exact trace-to-code links: summary=%#v links=%#v", baseline.Summary, baseline.TraceLinks)
	}
	for _, want := range []string{"opentelemetry", "datadog", "structured_log", "deploy_marker", "incident_timeline"} {
		if !hasTraceLinkKind(baseline.TraceLinks, want) {
			t.Fatalf("missing trace link kind %q in %#v", want, baseline.TraceLinks)
		}
	}
	if !strings.Contains(baseline.Markdown, "Trace-to-code links") {
		t.Fatalf("expected trace links in markdown:\n%s", baseline.Markdown)
	}
}

func TestBaselineEstimatesBlastRadius(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "db/migrate/001_schema.sql", `
CREATE TABLE users (id int PRIMARY KEY);
CREATE TABLE accounts (id int PRIMARY KEY, user_id int REFERENCES users(id));
CREATE TABLE invoices (id int PRIMARY KEY, account_id int REFERENCES accounts(id));
ALTER TABLE payments ADD CONSTRAINT payments_account_fk FOREIGN KEY (account_id) REFERENCES accounts(id);
UPDATE accounts SET status = 'disabled';
`)
	writeFile(t, root, "app/models/account.rb", "class Account < ApplicationRecord\n  has_many :invoices\nend\n")
	writeFile(t, root, "app/jobs/account_backfill_worker.rb", "class AccountBackfillWorker\n  def perform\n    Account.connection.execute(\"SELECT * FROM accounts JOIN invoices ON invoices.account_id = accounts.id\")\n    Account.update_all(status: 'active')\n  end\nend\n")
	writeFile(t, root, "reports/account_usage.sql", "SELECT accounts.id, invoices.id FROM accounts JOIN invoices ON invoices.account_id = accounts.id;")
	inv, err := InventoryPath(InventoryOptions{Path: root})
	if err != nil {
		t.Fatal(err)
	}
	intakeReport, err := intake.Run(context.Background(), intake.Options{Path: root})
	if err != nil {
		t.Fatal(err)
	}
	baseline := Baseline(inv, inv.Facts, intakeReport)
	if baseline.Summary.BlastRadius == 0 || baseline.Summary.BlastRadiusHigh == 0 {
		t.Fatalf("expected high blast-radius estimates: summary=%#v estimates=%#v", baseline.Summary, baseline.BlastRadius)
	}
	estimate, ok := blastEstimateForTable(baseline.BlastRadius, "accounts")
	if !ok {
		t.Fatalf("missing accounts blast-radius estimate in %#v", baseline.BlastRadius)
	}
	if estimate.FKReachability < 2 || estimate.CodePathFanout < 2 || estimate.QueryUsage < 2 || estimate.TableCentrality < 2 {
		t.Fatalf("expected reachability/fanout/query/centrality evidence, got %#v", estimate)
	}
	if !strings.Contains(baseline.Markdown, "Blast-radius estimates") {
		t.Fatalf("expected blast-radius section in markdown:\n%s", baseline.Markdown)
	}
}

func TestBaselineRanksProofHoleMinimizations(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "db/migrate/001_unbounded_accounts.sql", "UPDATE accounts SET status = 'disabled';")
	writeFile(t, root, "docs/incident-7.md", "Incident 7: accounts backfill needs maintainer approval and a dry-run before retry.")
	inv, err := InventoryPath(InventoryOptions{Path: root})
	if err != nil {
		t.Fatal(err)
	}
	intakeReport, err := intake.Run(context.Background(), intake.Options{Path: root})
	if err != nil {
		t.Fatal(err)
	}
	baseline := Baseline(inv, inv.Facts, intakeReport)
	if baseline.Summary.ProofMinimizations == 0 || baseline.Summary.ProofMinCritical == 0 {
		t.Fatalf("expected critical proof-hole minimizations: summary=%#v minimizers=%#v", baseline.Summary, baseline.ProofMinimizers)
	}
	for _, want := range []string{"approval-record", "scope-bound", "rollback-witness"} {
		if !hasProofMinimizationEvidence(baseline.ProofMinimizers, want) {
			t.Fatalf("missing proof minimization evidence %q in %#v", want, baseline.ProofMinimizers)
		}
	}
	first := baseline.ProofMinimizers[0]
	if first.Effort > 2 {
		t.Fatalf("expected smallest evidence first, got %#v", first)
	}
	if !strings.Contains(baseline.Markdown, "Proof-hole minimizations") {
		t.Fatalf("expected proof-hole minimization section in markdown:\n%s", baseline.Markdown)
	}
}

func TestCompareClassifiesGeneratedPrivacyHazards(t *testing.T) {
	baseline := BaselineReport{
		Version: BaselineVersion,
		Hash:    "baseline-hash",
		Risks: []BaselineRisk{{
			ID:       "risk:users",
			Path:     "db/migrate/001.sql",
			Kind:     "delete",
			Table:    "users",
			Severity: "high",
			Score:    130,
		}},
	}
	proposal := ProposalReport{
		OutputHash:    "proposal-hash",
		TargetRiskIDs: []string{"risk:users"},
		GeneratedFiles: []GeneratedFile{{
			Path:    "patchline-proposals/repair/privacy.sql",
			Kind:    "repair",
			RiskIDs: []string{"risk:users"},
		}},
		Generated: []GeneratedArtifact{{
			Path:    "patchline-proposals/repair/privacy.sql",
			Kind:    "repair",
			RiskIDs: []string{"risk:users"},
			Content: "DELETE FROM users;\nCOPY (SELECT * FROM users) TO 'users.csv' CSV;\n-- no rollback available\n",
		}},
	}
	compare := Compare(baseline, proposal)
	if compare.Summary.PrivacyHazards != 1 || compare.Summary.PrivacyCritical != 1 {
		t.Fatalf("expected generated privacy hazard: summary=%#v hazards=%#v", compare.Summary, compare.PrivacyHazards)
	}
	if !strings.Contains(compare.Markdown, "Generated data-retention and privacy hazards") {
		t.Fatalf("expected generated privacy hazards in markdown:\n%s", compare.Markdown)
	}
}

func TestCompareRejectsMutatedGeneratedGuards(t *testing.T) {
	good := GeneratedArtifact{
		Path: "patchline-proposals/guards/risk.sql",
		Kind: "guards",
		Content: `-- untrusted generated guard
BEGIN;
SELECT 1 FROM accounts LIMIT 1;
SELECT count(*) AS patchline_candidate_rows FROM accounts;
ROLLBACK;
`,
	}
	for _, tc := range []struct {
		name    string
		content string
	}{
		{name: "missing-table-existence", content: strings.ReplaceAll(good.Content, "SELECT 1 FROM", "MUTATED_REQUIRED_CHECK")},
		{name: "missing-row-count", content: strings.ReplaceAll(good.Content, "count(*)", "MUTATED_REQUIRED_CHECK")},
		{name: "missing-rollback-statement", content: strings.ReplaceAll(good.Content, "ROLLBACK;", "-- rollback note only")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mutated := good
			mutated.Content = tc.content
			checks := checkGeneratedArtifacts([]GeneratedArtifact{mutated})
			if len(checks) != 1 || checks[0].Status != "fail" {
				t.Fatalf("expected mutated guard to fail, got %#v", checks)
			}
		})
	}
	checks := checkGeneratedArtifacts([]GeneratedArtifact{good})
	if len(checks) != 1 || checks[0].Status != "pass" {
		t.Fatalf("expected complete guard to pass, got %#v", checks)
	}
}

func TestCompareChecksRepairManifestSchema(t *testing.T) {
	risk := ProposalRiskContext{
		ID:            "risk:accounts",
		Path:          "db/migrate/001.sql",
		Table:         "accounts",
		FactHashes:    []string{"sha256:abc123def4567890"},
		EvidencePaths: []string{"db/migrate/001.sql"},
	}
	valid := GeneratedArtifact{Path: "patchline-proposals/repair/risk.json", Kind: "repair", Content: renderRepairProposal(risk), RiskIDs: []string{risk.ID}}
	checks := checkGeneratedArtifacts([]GeneratedArtifact{valid})
	if len(checks) != 1 || checks[0].Status != "pass" {
		t.Fatalf("expected valid repair manifest to pass, got %#v", checks)
	}
	var manifest map[string]any
	if err := json.Unmarshal([]byte(valid.Content), &manifest); err != nil {
		t.Fatal(err)
	}
	delete(manifest, "validation_commands")
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	mutated := valid
	mutated.Content = string(data)
	checks = checkGeneratedArtifacts([]GeneratedArtifact{mutated})
	if len(checks) != 1 || checks[0].Status != "fail" || !strings.Contains(strings.Join(checks[0].Findings, "\n"), "validation commands") {
		t.Fatalf("expected missing validation commands to fail, got %#v", checks)
	}
}

func TestCompareInfersGeneratedTransactionBoundaries(t *testing.T) {
	baseline := BaselineReport{Version: BaselineVersion, Hash: "baseline-hash"}
	baseline.Hash = baselineHash(baseline)
	proposal := ProposalReport{
		BaselineHash: baseline.Hash,
		OutputHash:   "proposal-hash",
		GeneratedFiles: []GeneratedFile{
			{Path: "patchline-proposals/sql/update_accounts.sql", Kind: "repair", ContentHash: "sha256:abc", RiskIDs: []string{"risk:accounts"}},
		},
		Generated: []GeneratedArtifact{{
			Path: "patchline-proposals/sql/update_accounts.sql",
			Kind: "repair",
			Content: `BEGIN;
UPDATE accounts SET disabled = false WHERE id = 1;
COMMIT;`,
			RiskIDs: []string{"risk:accounts"},
		}},
		TargetRiskIDs: []string{"risk:accounts"},
	}
	compare := Compare(baseline, proposal)
	if compare.Summary.TransactionBoundaries != 1 || compare.Summary.TransactionExplicit != 1 {
		t.Fatalf("expected explicit generated transaction boundary: summary=%#v boundaries=%#v", compare.Summary, compare.Transactions)
	}
	if len(compare.Transactions) != 1 || compare.Transactions[0].Surface != "generated_repair" || compare.Transactions[0].Status != "explicit" {
		t.Fatalf("unexpected generated transaction boundary: %#v", compare.Transactions)
	}
	if compare.Summary.IdempotencyClasses != 1 || compare.Summary.IdempotencyGuarded != 1 {
		t.Fatalf("expected guarded generated idempotency classification: summary=%#v classes=%#v", compare.Summary, compare.Idempotency)
	}
}

func TestCompareRejectsGeneratedRiskBudgetOverrun(t *testing.T) {
	baseline := BaselineReport{
		Version: BaselineVersion,
		Hash:    "baseline-hash",
		Risks: []BaselineRisk{{
			ID:        "risk:accounts",
			Path:      "db/migrate/001.sql",
			Kind:      "update",
			Table:     "accounts",
			Severity:  "high",
			Score:     120,
			Rationale: "baseline risk",
		}},
	}
	baseline.Hash = baselineHash(baseline)
	proposal := ProposalReport{
		BaselineHash:  baseline.Hash,
		OutputHash:    "proposal-hash",
		TargetRiskIDs: []string{"risk:accounts"},
		GeneratedFiles: []GeneratedFile{{
			Path:    "patchline-proposals/explain/risky.sql",
			Kind:    "explain",
			RiskIDs: []string{"risk:accounts"},
		}},
		Generated: []GeneratedArtifact{{
			Path: "patchline-proposals/explain/risky.sql",
			Kind: "explain",
			Content: `-- Untrusted generated explain proposal
EXPLAIN SELECT * FROM accounts LIMIT 1;
SELECT count(*) AS patchline_candidate_rows FROM accounts;
UPDATE accounts SET admin = true;
DELETE FROM accounts;
`,
			RiskIDs: []string{"risk:accounts"},
		}},
	}
	compare := Compare(baseline, proposal)
	if compare.Summary.PatchlineChecksFailed != 0 || !compare.Summary.RiskBudgetRejected || compare.Summary.RiskBudgetAdded <= compare.Summary.RiskBudgetCovered {
		t.Fatalf("expected generated risk budget overrun without shape-check failure: %#v", compare.Summary)
	}
	if compare.Intervention.Status != "rejected-by-deterministic-checks" {
		t.Fatalf("expected rejected intervention loop, got %#v", compare.Intervention)
	}
	if compare.ReviewBadge.Safe || compare.ReviewBadge.Status != "not-safe-to-review" {
		t.Fatalf("expected unsafe review badge, got %#v", compare.ReviewBadge)
	}
}

func TestCompareAddsSafeToReviewBadgeWithListedProofHoles(t *testing.T) {
	baseline := BaselineReport{
		Version: BaselineVersion,
		Hash:    "baseline-hash",
		Risks: []BaselineRisk{{
			ID:        "risk:accounts",
			Path:      "db/migrate/001.sql",
			Kind:      "update",
			Table:     "accounts",
			Severity:  "high",
			Score:     120,
			Rationale: "baseline risk",
		}},
		NativeChecks: []Command{{Command: "go test ./...", Reason: "Go checks"}},
		SymbolicChecks: []SymbolicCheck{{
			ID:       "symbolic:scope",
			RiskID:   "risk:accounts",
			Property: "scope_preservation",
			Status:   "warn",
			Reason:   "row bound unavailable",
		}},
		PolicyChecks: []PolicyCheck{{
			ID:      "policy:accounts",
			RiskID:  "risk:accounts",
			Policy:  "guard-rollback-approval-dryrun-test",
			Status:  "fail",
			Missing: []string{"approval"},
		}},
	}
	baseline.Hash = baselineHash(baseline)
	proposal := ProposalReport{
		BaselineHash:  baseline.Hash,
		OutputHash:    "proposal-hash",
		TargetRiskIDs: []string{"risk:accounts"},
		GeneratedFiles: []GeneratedFile{{
			Path:    "patchline-proposals/tests/accounts.md",
			Kind:    "tests",
			RiskIDs: []string{"risk:accounts"},
		}},
		Generated: []GeneratedArtifact{{
			Path:    "patchline-proposals/tests/accounts.md",
			Kind:    "tests",
			Content: "# Untrusted generated test\n\nSuggested assertions:\n",
			RiskIDs: []string{"risk:accounts"},
		}},
	}
	compare := Compare(baseline, proposal)
	if !compare.ReviewBadge.Safe || compare.ReviewBadge.Status != "safe-to-review" {
		t.Fatalf("expected safe review badge, got %#v", compare.ReviewBadge)
	}
	if len(compare.ReviewBadge.ProofHoles) == 0 || !strings.Contains(strings.Join(compare.ReviewBadge.ProofHoles, "\n"), "approval") {
		t.Fatalf("expected listed proof holes, got %#v", compare.ReviewBadge.ProofHoles)
	}
	if !strings.Contains(compare.Markdown, "## Review badge") {
		t.Fatalf("expected review badge in markdown: %s", compare.Markdown)
	}
}

func TestCompareRunsSafeNativeChecks(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "go.mod", "module example.com/nativecheck\n\ngo 1.22\n")
	writeFile(t, root, "native_test.go", `package nativecheck

import "testing"
import "os"

func TestNative(t *testing.T) {
	if os.Getenv("GOPROXY") != "off" {
		t.Fatalf("expected offline go proxy, got %q", os.Getenv("GOPROXY"))
	}
	if os.Getenv("NO_PROXY") != "*" {
		t.Fatalf("expected network-off no-proxy marker, got %q", os.Getenv("NO_PROXY"))
	}
	if os.Getenv("HOME") == "" || os.Getenv("TMPDIR") == "" {
		t.Fatal("expected isolated HOME and TMPDIR")
	}
}
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
	if compare.NativeResults[0].Sandbox == nil || compare.NativeResults[0].Sandbox.Name != "go-offline-tests" || compare.NativeResults[0].Sandbox.NetworkEnabled {
		t.Fatalf("expected offline go sandbox profile: %#v", compare.NativeResults[0].Sandbox)
	}
	if compare.Quarantine.Status != "enforced" || !compare.Quarantine.SafeNativeChecksEnabled || compare.Quarantine.GeneratedArtifactsExecutable || compare.Quarantine.GeneratedArtifactsApplied {
		t.Fatalf("expected explicit safe-native quarantine state, got %#v", compare.Quarantine)
	}
}

func TestGeneratedCodeQuarantineSkipsNativeChecksByDefault(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "go.mod", "module example.com/quarantine\n\ngo 1.22\n")
	baseline := BaselineReport{
		Version:       BaselineVersion,
		InventoryRoot: root,
		Hash:          "baseline-hash",
		Risks: []BaselineRisk{{
			ID:        "risk:accounts",
			Path:      "db/migrate/001.sql",
			Kind:      "update",
			Table:     "accounts",
			Severity:  "high",
			Score:     120,
			Rationale: "unbounded update",
		}},
		NativeChecks: []Command{{Command: "go test ./...", Reason: "Go checks"}},
	}
	baseline.Hash = baselineHash(baseline)
	proposal := ProposalReport{
		Version:       ProposalVersion,
		BaselineHash:  baseline.Hash,
		OutputHash:    "proposal-hash",
		TargetRiskIDs: []string{"risk:accounts"},
		Generated: []GeneratedArtifact{{
			Path:    "patchline-proposals/tests/accounts_test.go",
			Kind:    "tests",
			Content: "// Untrusted generated test proposal\n// Suggested assertions: account repairs preserve row counts.\n",
			RiskIDs: []string{"risk:accounts"},
		}},
	}
	compare := Compare(baseline, proposal)
	if compare.Summary.NativeChecksRun != 0 || compare.Summary.NativeChecksSkipped != 1 || len(compare.NativeResults) != 1 {
		t.Fatalf("expected native checks skipped by default: summary=%#v results=%#v", compare.Summary, compare.NativeResults)
	}
	if !strings.Contains(compare.NativeResults[0].SkippedReason, "--run-native-tests") {
		t.Fatalf("expected explicit opt-in skip reason, got %#v", compare.NativeResults[0])
	}
	if compare.Quarantine.Status != "enforced" || compare.Quarantine.SafeNativeChecksEnabled || compare.Quarantine.GeneratedArtifactsExecutable || compare.Quarantine.GeneratedArtifactsApplied {
		t.Fatalf("expected enforced default quarantine, got %#v", compare.Quarantine)
	}
	if compare.Quarantine.RequiredFlag != "--run-native-tests" || len(compare.Quarantine.QuarantinedPaths) != 1 {
		t.Fatalf("expected quarantined generated path and opt-in flag, got %#v", compare.Quarantine)
	}
	if !strings.Contains(compare.Markdown, "Generated-code quarantine") || !strings.Contains(compare.Markdown, "safe native checks enabled: `false`") {
		t.Fatalf("expected quarantine markdown, got %s", compare.Markdown)
	}
}

func TestWriteProposalForcesGeneratedArtifactsNonExecutable(t *testing.T) {
	out := t.TempDir()
	artifactPath := filepath.Join(out, "patchline-proposals", "tests", "generated_test.sh")
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(artifactPath, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	report := ProposalReport{
		Version:       ProposalVersion,
		BaselineHash:  "baseline-hash",
		OutputHash:    "proposal-hash",
		TargetRiskIDs: []string{"risk:accounts"},
		Generated: []GeneratedArtifact{{
			Path:    "patchline-proposals/tests/generated_test.sh",
			Kind:    "tests",
			Content: "# Untrusted generated test proposal\n# Suggested assertions: never executable by default.\n",
			RiskIDs: []string{"risk:accounts"},
		}},
	}
	report.Quarantine = buildGeneratedQuarantine(report.Generated, false)
	report.GeneratedFiles = generatedFilesForArtifacts(report.Generated)
	report.Intervention = buildRepairIntervention(report.BaselineHash, report.OutputHash, report.TargetRiskIDs, report.Generated)
	report.Markdown = renderProposalMarkdown(report)
	if err := WriteProposal(out, report); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(artifactPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o111 != 0 {
		t.Fatalf("expected generated artifact to be non-executable, got mode %v", info.Mode().Perm())
	}
	loaded, err := LoadProposal(out)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Quarantine.Status != "enforced" || loaded.Quarantine.GeneratedArtifactsExecutable {
		t.Fatalf("expected persisted quarantine metadata, got %#v", loaded.Quarantine)
	}
}

func TestStableRiskIDIgnoresPathAndLineDrift(t *testing.T) {
	left := BaselineRisk{
		ID:        "risk:left",
		Path:      "db/migrate/20240101010101_add_accounts.sql",
		Statement: 3,
		Kind:      "high-risk-sql",
		Table:     "accounts",
		Factors:   []ScoreFactor{{Name: "destructive-sql", Weight: 70}},
		Identifiers: []Identifier{
			{Kind: "table", Value: "accounts"},
		},
	}
	right := left
	right.ID = "risk:right"
	right.Path = "eng/database/moved/999_add_accounts.sql"
	right.Statement = 42
	leftSlice := ProvenanceSlice{RiskID: left.ID, Table: "accounts", StagesPresent: []string{"migration", "source"}, Identifiers: []Identifier{{Kind: "table", Value: "accounts"}}}
	rightSlice := leftSlice
	rightSlice.RiskID = right.ID
	rightSlice.MigrationPath = "another/path.sql"

	if stableRiskID(left, leftSlice) != stableRiskID(right, rightSlice) {
		t.Fatalf("expected stable IDs to survive path and line drift")
	}
	right.Table = "profiles"
	if stableRiskID(left, leftSlice) == stableRiskID(right, rightSlice) {
		t.Fatalf("expected table changes to produce a different stable ID")
	}
}

func TestHostedArchiveInputsParseAndBuildURLs(t *testing.T) {
	tests := []struct {
		input     string
		host      string
		nested    bool
		wantOwner string
		wantRepo  string
		wantURL   string
	}{
		{
			input:     "gitlab:gitlab-org/security-products/analyzers/secrets",
			host:      "gitlab",
			nested:    true,
			wantOwner: "gitlab-org/security-products/analyzers",
			wantRepo:  "secrets",
			wantURL:   "https://gitlab.com/gitlab-org/security-products/analyzers/secrets/-/archive/main/secrets-main.tar.gz",
		},
		{
			input:     "bitbucket:atlassian/python-bitbucket",
			host:      "bitbucket",
			wantOwner: "atlassian",
			wantRepo:  "python-bitbucket",
			wantURL:   "https://bitbucket.org/atlassian/python-bitbucket/get/main.tar.gz",
		},
		{
			input:     "sourcehut:sircmpwn/scdoc",
			host:      "sourcehut",
			wantOwner: "~sircmpwn",
			wantRepo:  "scdoc",
			wantURL:   "https://git.sr.ht/~sircmpwn/scdoc/archive/main.tar.gz",
		},
	}
	for _, tc := range tests {
		owner, repo, err := parsePrefixedRepo(tc.input, tc.host, tc.nested)
		if err != nil {
			t.Fatal(err)
		}
		if owner != tc.wantOwner || repo != tc.wantRepo {
			t.Fatalf("%s parsed to %s/%s", tc.input, owner, repo)
		}
		got, kind, err := hostedArchiveURL(tc.host, owner, repo, "main")
		if err != nil {
			t.Fatal(err)
		}
		if got != tc.wantURL || kind != "tar.gz" {
			t.Fatalf("%s URL = %s kind=%s", tc.input, got, kind)
		}
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

func playbookHasHazardClass(playbook RemediationPlaybook, class string) bool {
	for _, hazard := range playbook.HazardClasses {
		if hazard.Class == class {
			return true
		}
	}
	return false
}

func playbookHasOwner(playbook RemediationPlaybook, owner string) bool {
	for _, handoff := range playbook.OwnerHandoffs {
		for _, candidate := range handoff.Owners {
			if candidate == owner {
				return true
			}
		}
	}
	return false
}

func playbookHasRollbackStage(playbook RemediationPlaybook, stage string) bool {
	for _, rollback := range playbook.RollbackPoints {
		if rollback.Stage == stage {
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

func zipForTest(t *testing.T, files map[string]string) string {
	t.Helper()
	archive := filepath.Join(t.TempDir(), "fixture.zip")
	file, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(file)
	for name, content := range files {
		writer, err := zw.Create(filepath.ToSlash(name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(writer, content); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return archive
}

func withArchiveLimits(t *testing.T, limits archiveExtractionLimits) {
	t.Helper()
	previous := archiveLimits
	archiveLimits = limits
	t.Cleanup(func() {
		archiveLimits = previous
	})
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

func hasFindingKind(findings []Finding, kind string) bool {
	for _, finding := range findings {
		if finding.Kind == kind {
			return true
		}
	}
	return false
}

func hasLockHazardMarker(hazards []LockHazard, marker string) bool {
	for _, hazard := range hazards {
		for _, item := range hazard.Markers {
			if item == marker {
				return true
			}
		}
	}
	return false
}

func hasPrivacyHazardMarker(hazards []PrivacyHazard, marker string) bool {
	for _, hazard := range hazards {
		for _, item := range hazard.Markers {
			if item == marker {
				return true
			}
		}
	}
	return false
}

func hasInvariantKind(items []InvariantCandidate, kind string) bool {
	for _, item := range items {
		if item.Kind == kind {
			return true
		}
	}
	return false
}

func hasTraceLinkKind(items []TraceCodeLink, kind string) bool {
	for _, item := range items {
		if item.Kind == kind {
			return true
		}
	}
	return false
}

func blastEstimateForTable(items []BlastRadiusEstimate, table string) (BlastRadiusEstimate, bool) {
	for _, item := range items {
		if item.Table == table {
			return item, true
		}
	}
	return BlastRadiusEstimate{}, false
}

func hasProofMinimizationEvidence(items []ProofHoleMinimization, missingEvidence string) bool {
	for _, item := range items {
		if item.MissingEvidence == missingEvidence {
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
