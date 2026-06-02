package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func FuzzAnalyzeBytesWithDialect(f *testing.F) {
	for _, seed := range []string{
		"UPDATE accounts SET disabled = true;",
		"DELETE FROM ledger_entries WHERE id = 1;",
		"CREATE INDEX CONCURRENTLY idx_users_email ON users(email);",
		"ALTER TABLE invoices ADD COLUMN status text;",
		"SELECT * FROM accounts WHERE id = 1;",
	} {
		f.Add(seed, "postgres")
	}
	f.Fuzz(func(t *testing.T, sql, dialectValue string) {
		if len(sql) > 4096 {
			t.Skip()
		}
		dialects := []Dialect{DialectGeneric, DialectPostgres, DialectMySQL, DialectSQLite, DialectSQLServer, DialectOracle, DialectBigQuery}
		dialect := dialects[len(dialectValue)%len(dialects)]
		report, err := AnalyzeBytesWithDialect("fuzz.sql", []byte(sql), dialect)
		if err != nil {
			t.Fatalf("AnalyzeBytesWithDialect returned error for valid dialect: %v", err)
		}
		if report.Version != Version || report.Summary.TotalStatements != len(report.Statements) {
			t.Fatalf("inconsistent report summary: %#v", report)
		}
		for _, statement := range report.Statements {
			if statement.Fingerprint == "" || statement.Index < 0 {
				t.Fatalf("statement missing stable identity: %#v", statement)
			}
		}
	})
}

func FuzzExtractSourceSQL(f *testing.F) {
	seeds := map[string]string{
		"job.py":     `sql = "UPDATE invoices SET status = 'ready' WHERE id = 1"`,
		"worker.rb":  `Invoice.where(status: "stale").update_all(status: "ready")`,
		"service.go": "`SELECT * FROM accounts WHERE id = $1`",
		"query.sql":  "DELETE FROM sessions WHERE expired = true;",
	}
	for name, content := range seeds {
		f.Add(name, content)
	}
	f.Fuzz(func(t *testing.T, name, content string) {
		if len(content) > 4096 {
			t.Skip()
		}
		name = strings.Map(func(r rune) rune {
			if r == 0 || r < 32 || r == ':' {
				return '-'
			}
			return r
		}, name)
		if filepath.Base(name) == "." || filepath.Base(name) == string(filepath.Separator) {
			name = "input.sql"
		}
		root := t.TempDir()
		path := filepath.Join(root, filepath.Base(name))
		if filepath.Ext(path) == "" {
			path += ".sql"
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		report, err := ExtractSourceSQL(root)
		if err != nil {
			t.Fatalf("ExtractSourceSQL returned error: %v", err)
		}
		if report.Version == "" {
			t.Fatalf("missing report version: %#v", report)
		}
		for _, observation := range report.Observations {
			if observation.Path == "" || observation.SnippetHash == "" {
				t.Fatalf("observation missing identity: %#v", observation)
			}
		}
	})
}
