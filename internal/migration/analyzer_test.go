package migration

import "testing"

func TestAnalyzeClassifiesUnboundedUpdateAsHighRisk(t *testing.T) {
	report, err := AnalyzeBytes("bad.sql", []byte("update invoices set total_cents = 0;"))
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.HighRisk != 1 {
		t.Fatalf("expected one high-risk statement, got %#v", report.Summary)
	}
	statement := report.Statements[0]
	if statement.Kind != "update" || statement.Table != "invoices" || statement.HasWhere {
		t.Fatalf("unexpected statement: %#v", statement)
	}
	if statement.Effect != string("destructive") {
		t.Fatalf("expected destructive effect, got %s", statement.Effect)
	}
}

func TestAnalyzePreservesSemicolonsInsideStrings(t *testing.T) {
	report, err := AnalyzeBytes("strings.sql", []byte("insert into audit_log (msg) values ('one;two'); update invoices set total_cents = 1 where id = 'inv;1';"))
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Statements) != 2 {
		t.Fatalf("expected two statements, got %d: %#v", len(report.Statements), report.Statements)
	}
	if report.Statements[1].Kind != "update" || !report.Statements[1].HasWhere {
		t.Fatalf("unexpected second statement: %#v", report.Statements[1])
	}
	if report.Statements[1].Risk != RiskMedium {
		t.Fatalf("expected id-scoped update to be medium risk, got %s", report.Statements[1].Risk)
	}
}

func TestAnalyzeClassifiesBroadUpdatePredicateAsHighRisk(t *testing.T) {
	report, err := AnalyzeBytes("broad.sql", []byte("update invoices set total_cents = 0 where status = 'issued';"))
	if err != nil {
		t.Fatal(err)
	}
	if report.Statements[0].Risk != RiskHigh {
		t.Fatalf("expected broad update to be high risk: %#v", report.Statements[0])
	}
}

func TestFingerprintNormalizesLiteralsAndWhitespace(t *testing.T) {
	first := Fingerprint("UPDATE invoices\nSET total_cents = 4200 WHERE id = 'inv_1002'")
	second := Fingerprint(" update invoices set total_cents = 7 where id = 'other' ")
	if first != second {
		t.Fatalf("fingerprints differ:\n%s\n%s", first, second)
	}
}

func TestAnalyzePostgresDialectRules(t *testing.T) {
	report, err := AnalyzeBytesWithDialect("pg.sql", []byte("create index concurrently idx_invoices_status on invoices(status); alter table invoices add column repaired_at timestamptz default now();"), DialectPostgres)
	if err != nil {
		t.Fatal(err)
	}
	if report.Dialect != DialectPostgres {
		t.Fatalf("expected postgres dialect, got %q", report.Dialect)
	}
	if report.Statements[0].Risk != RiskLow {
		t.Fatalf("expected concurrent index to be low risk: %#v", report.Statements[0])
	}
	if report.Statements[1].Risk != RiskHigh {
		t.Fatalf("expected defaulted add-column to be high risk: %#v", report.Statements[1])
	}
}

func TestAnalyzeMySQLDialectRules(t *testing.T) {
	report, err := AnalyzeBytesWithDialect("mysql.sql", []byte("replace into `users` (id, email) values (1, 'a@example.com'); alter table `users` add column tier varchar(10), algorithm=copy;"), DialectMySQL)
	if err != nil {
		t.Fatal(err)
	}
	if report.Statements[0].Kind != "replace" || report.Statements[0].Table != "users" || report.Statements[0].Risk != RiskHigh {
		t.Fatalf("expected mysql replace to be destructive high risk: %#v", report.Statements[0])
	}
	if report.Statements[1].Table != "users" || report.Statements[1].Risk != RiskHigh {
		t.Fatalf("expected mysql algorithm copy alter to be high risk: %#v", report.Statements[1])
	}
}

func TestAnalyzeSQLiteDialectRules(t *testing.T) {
	report, err := AnalyzeBytesWithDialect("sqlite.sql", []byte("pragma foreign_keys = off; alter table invoices drop column total_cents;"), DialectSQLite)
	if err != nil {
		t.Fatal(err)
	}
	if report.Statements[0].Kind != "pragma" || report.Statements[0].Risk != RiskHigh {
		t.Fatalf("expected foreign_keys off pragma to be high risk: %#v", report.Statements[0])
	}
	if report.Statements[1].Risk != RiskHigh {
		t.Fatalf("expected sqlite drop column to be high risk: %#v", report.Statements[1])
	}
}

func TestAnalyzeSQLServerDialectRules(t *testing.T) {
	report, err := AnalyzeBytesWithDialect("sqlserver.sql", []byte("update top (10) [dbo].[Invoices] set total_cents = 0; delete top (5) from [dbo].[LedgerEntries];"), DialectSQLServer)
	if err != nil {
		t.Fatal(err)
	}
	if report.Statements[0].Table != "dbo.invoices" || report.Statements[0].Risk != RiskHigh {
		t.Fatalf("expected sqlserver update top table and high risk: %#v", report.Statements[0])
	}
	if report.Statements[1].Table != "dbo.ledgerentries" || report.Statements[1].Risk != RiskHigh {
		t.Fatalf("expected sqlserver delete top table and high risk: %#v", report.Statements[1])
	}
}

func TestAnalyzeRejectsUnknownDialect(t *testing.T) {
	if _, err := AnalyzeBytesWithDialect("bad.sql", []byte("select 1"), Dialect("oracle")); err == nil {
		t.Fatal("expected unknown dialect error")
	}
}

func TestParseStatementASTExtractsStructure(t *testing.T) {
	tokens := tokenize("update invoices set total_cents = 0 from adjustments where invoices.id = adjustments.invoice_id returning invoices.id")
	ast := parseStatementAST(tokens)
	if ast.Kind != "update" || ast.Table != "invoices" || !ast.HasWhere {
		t.Fatalf("unexpected AST header: %#v", ast)
	}
	kinds := clauseKinds(ast.Clauses)
	want := []string{"set", "from", "where", "returning"}
	if len(kinds) != len(want) {
		t.Fatalf("unexpected clause count: %#v", ast.Clauses)
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Fatalf("unexpected clause order: got %#v want %#v", kinds, want)
		}
	}
}

func TestAnalyzerUsesASTForDropAndTruncateTables(t *testing.T) {
	report, err := AnalyzeBytes("ddl.sql", []byte("drop table invoices; truncate table ledger_entries;"))
	if err != nil {
		t.Fatal(err)
	}
	if report.Statements[0].Table != "invoices" || report.Statements[1].Table != "ledger_entries" {
		t.Fatalf("expected AST table extraction for DDL statements: %#v", report.Statements)
	}
}

func clauseKinds(clauses []Clause) []string {
	out := make([]string, 0, len(clauses))
	for _, clause := range clauses {
		out = append(out, clause.Kind)
	}
	return out
}
