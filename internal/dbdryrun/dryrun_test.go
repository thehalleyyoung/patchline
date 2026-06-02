package dbdryrun

import (
	"strings"
	"testing"

	"github.com/thehalleyyoung/patchline/internal/repair"
)

func TestBuildPostgresSchemaOnlyDryRun(t *testing.T) {
	report, err := Build(testManifest(), Options{Dialect: "postgres"})
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || report.Mode != "schema-only" || report.Container.Image != "postgres:16" || !report.Container.NoProduction {
		t.Fatalf("unexpected report: %#v", report)
	}
	for _, want := range []string{
		`CREATE TEMP TABLE "invoices" ("id" TEXT, "repair_marker" TEXT, "total_cents" TEXT) ON COMMIT DROP;`,
		`EXPLAIN UPDATE "invoices" SET "repair_marker" = 'inc_1', "total_cents" = '4200' WHERE "id" = 'inv_1' AND "total_cents" = '0';`,
		`ROLLBACK;`,
	} {
		if !strings.Contains(report.Script, want) {
			t.Fatalf("expected script to contain %q:\n%s", want, report.Script)
		}
	}
	if report.Hash == "" || len(report.Schema) != 1 || len(report.Statements) != 1 {
		t.Fatalf("expected hash/schema/statements: %#v", report)
	}
}

func TestBuildMySQLSchemaOnlyDryRun(t *testing.T) {
	report, err := Build(testManifest(), Options{Dialect: "mysql"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"CREATE TEMPORARY TABLE `invoices` (`id` TEXT, `repair_marker` TEXT, `total_cents` TEXT);",
		"EXPLAIN UPDATE `invoices` SET `repair_marker` = 'inc_1', `total_cents` = '4200' WHERE `id` = 'inv_1' AND `total_cents` = '0';",
	} {
		if !strings.Contains(report.Script, want) {
			t.Fatalf("expected script to contain %q:\n%s", want, report.Script)
		}
	}
}

func TestBuildExecuteOptionSuppressesSkippedWarning(t *testing.T) {
	report, err := Build(testManifest(), Options{Dialect: "postgres", DSN: "postgres://postgres@127.0.0.1:55432/patchline", Execute: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, warning := range report.Warnings {
		if strings.Contains(warning, "execution skipped") {
			t.Fatalf("did not expect skipped warning when execute is requested: %#v", report.Warnings)
		}
	}
}

func TestRejectsNonLocalDSN(t *testing.T) {
	if _, err := Build(testManifest(), Options{Dialect: "postgres", DSN: "postgres://user:secret@prod-db.example.com/app"}); err == nil {
		t.Fatal("expected non-local DSN to be rejected")
	}
	for _, dsn := range []string{
		"postgres://postgres@127.0.0.1:55432/patchline",
		"postgres://postgres@localhost:55432/patchline",
		"user:pass@tcp(127.0.0.1:3307)/patchline",
		"host=/tmp dbname=patchline",
	} {
		if !IsLocalDSN(dsn) {
			t.Fatalf("expected local DSN: %s", dsn)
		}
	}
}

func TestMySQLClientArgsTranslateLocalDSNs(t *testing.T) {
	args := strings.Join(mysqlClientArgs("user:pass@tcp(127.0.0.1:3307)/patchline", "/tmp/script.sql"), " ")
	for _, want := range []string{"--host=127.0.0.1", "--port=3307", "--user=user", "--password=pass", "patchline"} {
		if !strings.Contains(args, want) {
			t.Fatalf("expected %q in mysql args: %s", want, args)
		}
	}
	args = strings.Join(mysqlClientArgs("mysql://root@localhost:3307/patchline", "/tmp/script.sql"), " ")
	for _, want := range []string{"--host=localhost", "--port=3307", "--user=root", "patchline"} {
		if !strings.Contains(args, want) {
			t.Fatalf("expected %q in mysql URL args: %s", want, args)
		}
	}
}

func testManifest() repair.Manifest {
	return repair.Manifest{
		Version:       repair.Version,
		Name:          "db-dry-run-test",
		Incident:      "inc_1",
		Preconditions: []repair.Check{{Kind: "sql", Expr: "select 1", Expect: "1"}},
		Scope: repair.Scope{
			Table: "invoices",
			Where: map[string]string{"id": "inv_1"},
		},
		Operations: []repair.Operation{{
			ID:    "restore",
			Kind:  "update",
			Table: "invoices",
			Where: map[string]string{"id": "inv_1", "total_cents": "0"},
			Set:   map[string]string{"repair_marker": "inc_1", "total_cents": "4200"},
		}},
		Postconditions: []repair.Check{{Kind: "sql", Expr: "select 1", Expect: "1"}},
		Rollback:       repair.Rollback{Strategy: "snapshot", SnapshotRequired: true},
	}
}
