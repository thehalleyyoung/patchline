package intake

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRunScansCurrentProjectDataWithoutLabels(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "db/migrate/001_bad.sql", "UPDATE invoices SET total_cents = 0;")
	writeFile(t, root, "app/worker.py", `sql = "DELETE FROM ledger_entries"`)
	writeFile(t, root, "exports/weird-observability.json", `{
	  "whatever": [{"traceish": "abc", "commit_sha": "8f3c2ab", "query_text": "UPDATE accounts SET disabled = true"}],
	  "deployish": "prod-42"
	}`)
	writeFile(t, root, "repairs/fix.json", `{
	  "version": "patchline.repair/v1",
	  "name": "fix invoices",
	  "incident": "incident:billing",
	  "scope": {"table": "invoices", "where": {"id": "inv_1002"}},
	  "operations": [{"id": "restore", "kind": "restore", "table": "invoices", "where": {"id": "inv_1002"}, "set": {"total_cents": "1000"}}],
	  "rollback": {"strategy": "snapshot", "snapshot_required": true}
	}`)

	report, err := Run(context.Background(), Options{Path: root})
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.FilesScanned != 4 {
		t.Fatalf("expected four files scanned, got %#v", report.Summary)
	}
	if report.Summary.SQLFiles != 1 || report.Summary.HighRiskSQLStatements == 0 {
		t.Fatalf("expected high-risk SQL file finding: %#v", report.Summary)
	}
	if report.Summary.SourceSQLObservations == 0 {
		t.Fatalf("expected source SQL extraction to find embedded SQL")
	}
	if report.Summary.GenericEvidenceSignals == 0 {
		t.Fatalf("expected arbitrary JSON export to produce generic evidence signals: %#v", report.Evidence)
	}
	if report.Summary.RepairManifests != 1 {
		t.Fatalf("expected repair manifest finding: %#v", report.Repairs)
	}
	if len(report.Suggestions) == 0 {
		t.Fatalf("expected runnable next commands")
	}
}

func TestRunDerivesProblemCauseRepairCandidatesFromExistingSources(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "migrations/20260601_accounts.sql", "DELETE FROM accounts;")
	writeFile(t, root, "incidents/sev-42.md", `# SEV-42 postmortem
Root cause: commit abc1234 deployed a bad migration against table accounts.
Customer rows were corrupted during the outage.
Remediation: restore table accounts from backup and reconcile missing rows.`)
	writeFile(t, root, "ops/restore_accounts.sql", "UPDATE accounts SET disabled = false WHERE id = 'acct_1';")

	report, err := Run(context.Background(), Options{Path: root})
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.ProblemCandidates == 0 {
		t.Fatalf("expected problem candidates: %#v", report)
	}
	if report.Summary.CauseCandidates == 0 {
		t.Fatalf("expected cause candidates: %#v", report.Causes)
	}
	if report.Summary.RepairCandidates == 0 {
		t.Fatalf("expected repair candidates: %#v", report.RepairCandidates)
	}
	if report.Summary.LinkedCandidates == 0 {
		t.Fatalf("expected identifier-grounded links: problems=%#v causes=%#v repairs=%#v", report.Problems, report.Causes, report.RepairCandidates)
	}
	for _, link := range report.Links {
		if len(link.Identifiers) == 0 {
			t.Fatalf("link must be identifier-grounded: %#v", link)
		}
	}
}

func TestParseGitHubRepo(t *testing.T) {
	owner, repo, err := parseGitHubRepo("https://github.com/thehalleyyoung/patchline.git")
	if err != nil {
		t.Fatal(err)
	}
	if owner != "thehalleyyoung" || repo != "patchline" {
		t.Fatalf("unexpected parsed repo %s/%s", owner, repo)
	}
	if _, _, err := parseGitHubRepo("../bad/repo"); err == nil {
		t.Fatalf("expected unsafe repo to fail")
	}
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
