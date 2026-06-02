package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thehalleyyoung/patchline/internal/artifact"
	"github.com/thehalleyyoung/patchline/internal/evidence"
	"github.com/thehalleyyoung/patchline/internal/intake"
	"github.com/thehalleyyoung/patchline/internal/project"
)

func TestExitCodeDefaultsToUsageOrGenericFailure(t *testing.T) {
	if got := exitCode(errors.New("boom")); got != 1 {
		t.Fatalf("expected default exit code 1, got %d", got)
	}
}

func TestExitCodeUsesCodedError(t *testing.T) {
	err := codedError{code: 3, err: errors.New("threshold failed")}
	if got := exitCode(err); got != 3 {
		t.Fatalf("expected coded exit code 3, got %d", got)
	}
}

func TestParseGateOptionsRejectsInvalidThresholds(t *testing.T) {
	_, err := parseGateOptions([]string{"--min-precision", "1.5"})
	if err == nil {
		t.Fatal("expected invalid threshold error")
	}
}

func TestPluginsListAndProbeCommands(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "db/migrate/001_backfill.sql", "UPDATE accounts SET status = 'active';\n")
	out := filepath.Join(t.TempDir(), "probe")
	if err := run([]string{"plugins", "list", "--json"}); err != nil {
		t.Fatalf("plugins list failed: %v", err)
	}
	if err := run([]string{"plugins", "probe", root, "--out", out, "--json"}); err != nil {
		t.Fatalf("plugins probe failed: %v", err)
	}
	for _, rel := range []string{"plugin-probe.json", "plugin-probe.md", "baseline/baseline.json", "proposal/proposal.json", "compare/compare.json", "rendered/baseline.json", "rendered/baseline.md"} {
		if stat, err := os.Stat(filepath.Join(out, rel)); err != nil || stat.Size() == 0 {
			t.Fatalf("expected %s to be written, stat=%#v err=%v", rel, stat, err)
		}
	}
}

func TestGoldenFixtureGenerateCommand(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "db/migrate/001_delete_events.sql", "DELETE FROM account_events;\n")
	writeMainTestFile(t, root, "db/migrate/002_backfill.sql", "UPDATE accounts SET status = 'active';\n")
	out := filepath.Join(t.TempDir(), "golden")
	if err := run([]string{"golden-fixture", "generate", root, "--out", out, "--max-files", "2", "--json"}); err != nil {
		t.Fatalf("golden fixture generation failed: %v", err)
	}
	for _, rel := range []string{"generated_golden_test.go", "go.mod", "golden-fixture.json", "golden-fixture.md"} {
		if stat, err := os.Stat(filepath.Join(out, rel)); err != nil || stat.Size() == 0 {
			t.Fatalf("expected %s to be written, stat=%#v err=%v", rel, stat, err)
		}
	}
	cmd := exec.Command("go", "test", ".")
	cmd.Dir = out
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated fixture test failed: %v\n%s", err, string(output))
	}
}

func TestRepoAnalyzeTraceWritesDiagnostics(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "db/migrate/001_backfill.sql", "UPDATE accounts SET status = 'active';\n")
	out := filepath.Join(t.TempDir(), "analysis")
	if err := run([]string{"repo", "analyze", root, "--stages", "inventory,baseline,propose,compare,deep", "--proposal-kind", "all", "--budget", "files=2,lines=60,tokens=4000,changes=1", "--no-llm", "--trace", "--out", out, "--json"}); err != nil {
		t.Fatalf("repo analyze trace failed: %v", err)
	}
	for _, rel := range []string{"diagnostics/events.jsonl", "diagnostics/summary.json", "analyze.json", "analyze.md"} {
		if stat, err := os.Stat(filepath.Join(out, rel)); err != nil || stat.Size() == 0 {
			t.Fatalf("expected %s to be written, stat=%#v err=%v", rel, stat, err)
		}
	}
	var summary struct {
		Version     string `json:"version"`
		Spans       int    `json:"spans"`
		Logs        int    `json:"logs"`
		FailedSpans int    `json:"failed_spans"`
		EventsPath  string `json:"events_path"`
	}
	readMainTestJSON(t, filepath.Join(out, "diagnostics/summary.json"), &summary)
	if summary.Version != "patchline.diagnostics/v1" || summary.Spans < 8 || summary.Logs < 2 || summary.FailedSpans != 0 || summary.EventsPath == "" {
		t.Fatalf("unexpected diagnostics summary: %#v", summary)
	}
	eventsData, err := os.ReadFile(filepath.Join(out, "diagnostics/events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	events := string(eventsData)
	for _, name := range []string{`"name":"repo.analyze"`, `"name":"inventory"`, `"name":"intake"`, `"name":"baseline"`, `"name":"proposal"`, `"name":"compare"`, `"name":"triage"`, `"name":"analysis-bundle"`} {
		if !strings.Contains(events, name) {
			t.Fatalf("expected diagnostics event %s in:\n%s", name, events)
		}
	}
	var analyze struct {
		Outputs     map[string]string `json:"outputs"`
		Diagnostics struct {
			SummaryPath string `json:"summary_path"`
			FailedSpans int    `json:"failed_spans"`
		} `json:"diagnostics"`
	}
	readMainTestJSON(t, filepath.Join(out, "analyze.json"), &analyze)
	if analyze.Outputs["diagnostics"] == "" || analyze.Diagnostics.SummaryPath == "" || analyze.Diagnostics.FailedSpans != 0 {
		t.Fatalf("expected diagnostics in analyze report: %#v", analyze)
	}
	markdown, err := os.ReadFile(filepath.Join(out, "analyze.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(markdown), "diagnostics") {
		t.Fatalf("expected diagnostics summary in analyze markdown:\n%s", string(markdown))
	}
}

func TestRepoAnalyzeTraceFlushesDiagnosticsOnError(t *testing.T) {
	out := filepath.Join(t.TempDir(), "analysis")
	missing := filepath.Join(t.TempDir(), "missing")
	err := run([]string{"repo", "analyze", missing, "--stages", "inventory", "--trace", "--out", out, "--json"})
	if err == nil {
		t.Fatal("expected missing input to fail")
	}
	var summary struct {
		FailedSpans int `json:"failed_spans"`
	}
	readMainTestJSON(t, filepath.Join(out, "diagnostics/summary.json"), &summary)
	if summary.FailedSpans == 0 {
		t.Fatalf("expected failed span summary, got %#v", summary)
	}
	eventsData, err := os.ReadFile(filepath.Join(out, "diagnostics/events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(eventsData), `"status":"error"`) {
		t.Fatalf("expected error span in diagnostics:\n%s", string(eventsData))
	}
}

func TestOnePositionalWithFlagsAllowsFlagsAfterPath(t *testing.T) {
	pos, flags, err := onePositionalWithFlags([]string{"django/django", "--subpath", "django/contrib/auth/migrations", "--json"}, map[string]bool{"--json": true})
	if err != nil {
		t.Fatal(err)
	}
	if pos != "django/django" {
		t.Fatalf("unexpected positional %q", pos)
	}
	if len(flags) != 3 || flags[0] != "--subpath" || flags[1] != "django/contrib/auth/migrations" || flags[2] != "--json" {
		t.Fatalf("unexpected flags %#v", flags)
	}
}

func TestDBDryRunRejectsNonLocalDSN(t *testing.T) {
	root := t.TempDir()
	manifest := filepath.Join(root, "repair.json")
	writeMainTestFile(t, root, "repair.json", `{
  "version": "patchline.repair/v1",
  "name": "cli-db-dry-run",
  "incident": "inc_1",
  "scope": {"table": "accounts", "where": {"id": "acct_1"}},
  "preconditions": [{"kind": "sql", "expr": "select 1", "expect": "1"}],
  "operations": [{"id": "fix", "kind": "update", "table": "accounts", "where": {"id": "acct_1"}, "set": {"status": "ok"}}],
  "postconditions": [{"kind": "sql", "expr": "select 1", "expect": "1"}],
  "rollback": {"strategy": "snapshot", "snapshot_required": true}
}`)
	err := dbDryRun(manifest, []string{"--dialect", "postgres", "--dsn", "postgres://user:secret@prod-db.example.com/app", "--json"}, true)
	if err == nil || !strings.Contains(err.Error(), "refusing non-local database target") {
		t.Fatalf("expected non-local DSN rejection, got %v", err)
	}
}

func TestRepoHookPreCommitScansOnlyStagedChangedFiles(t *testing.T) {
	root := initMainTestGitRepo(t)
	writeMainTestFile(t, root, "db/migrate/001_backfill.sql", "UPDATE accounts SET status = 'active';\n")
	runMainTestGit(t, root, "add", "db/migrate/001_backfill.sql")

	report, err := buildRepoHookReport("pre-commit", root, "", filepath.Join(t.TempDir(), "hook"))
	if err != nil {
		t.Fatal(err)
	}
	if report.Version != "patchline.repo-hook/v1" || report.Mode != "pre-commit" || report.Network || report.Summary.NetworkOperations != 0 {
		t.Fatalf("unexpected hook report metadata: %#v", report)
	}
	if report.Summary.ChangedFiles != 1 || report.Summary.ScannedFiles != 1 || report.Summary.RankedRisks == 0 {
		t.Fatalf("expected staged changed-file risks: %#v", report.Summary)
	}
	if len(report.FindingDeltas) == 0 || report.FindingDeltas[0].Path != "db/migrate/001_backfill.sql" {
		t.Fatalf("expected finding delta mapped to repo-relative staged file: %#v", report.FindingDeltas)
	}
}

func TestRepoHookPrePushScansBranchDelta(t *testing.T) {
	root := initMainTestGitRepo(t)
	writeMainTestFile(t, root, "db/migrate/001_create_accounts.sql", "CREATE TABLE accounts(id int);\n")
	runMainTestGit(t, root, "add", ".")
	runMainTestGit(t, root, "commit", "-m", "initial")
	writeMainTestFile(t, root, "db/migrate/002_delete_accounts.sql", "DELETE FROM accounts;\n")
	runMainTestGit(t, root, "add", ".")
	runMainTestGit(t, root, "commit", "-m", "delete accounts")

	report, err := buildRepoHookReport("pre-push", root, "HEAD~1", filepath.Join(t.TempDir(), "hook"))
	if err != nil {
		t.Fatal(err)
	}
	if report.Mode != "pre-push" || report.Base != "HEAD~1" || report.Network {
		t.Fatalf("unexpected pre-push report metadata: %#v", report)
	}
	if report.Summary.ChangedFiles != 1 || report.Summary.RankedRisks == 0 || report.FindingDeltas[0].Path != "db/migrate/002_delete_accounts.sql" {
		t.Fatalf("expected branch delta finding: summary=%#v deltas=%#v", report.Summary, report.FindingDeltas)
	}
}

func TestRepoOfflineValidatesLocalReportsAndAdapters(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "db/migrate/001_backfill.sql", "UPDATE accounts SET status = 'active';\n")
	analysis := filepath.Join(t.TempDir(), "analysis")
	inv, err := project.InventoryPath(project.InventoryOptions{Path: root})
	if err != nil {
		t.Fatal(err)
	}
	intakeReport, err := intake.Run(context.Background(), intake.Options{Path: root})
	if err != nil {
		t.Fatal(err)
	}
	baseline := project.Baseline(inv, inv.Facts, intakeReport)
	if err := project.WriteInventory(filepath.Join(analysis, "inventory"), inv); err != nil {
		t.Fatal(err)
	}
	if err := intake.WriteReport(filepath.Join(analysis, "intake"), intakeReport); err != nil {
		t.Fatal(err)
	}
	if err := project.WriteBaseline(filepath.Join(analysis, "baseline"), baseline); err != nil {
		t.Fatal(err)
	}
	proposal, err := project.Propose(project.ProposalOptions{BaselinePath: filepath.Join(analysis, "baseline"), Kind: "guards", OutDir: filepath.Join(analysis, "proposal"), NoLLM: true, BudgetRisks: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := project.WriteProposal(filepath.Join(analysis, "proposal"), proposal); err != nil {
		t.Fatal(err)
	}
	writeMainTestFile(t, analysis, "analyze.json", `{"version":"patchline.repo-analyze/v1","summary":{"ranked_risks":1}}`)
	writeMainTestFile(t, analysis, "fetch/source.json", fmt.Sprintf(`{"version":"patchline.project/v1","mode":"local","input":"%s","scanned_root":"%s"}`, root, root))
	adapterPath := filepath.Join(t.TempDir(), "adapter.json")
	writeMainTestFile(t, filepath.Dir(adapterPath), filepath.Base(adapterPath), `{"version":"patchline.evidence-adapter/v1","adapter":"github","ok":true,"event_count":1,"input_hash":"sha256:test","events":[{"type":"deploy"}]}`)

	report := buildRepoOfflineReport(analysis, []string{adapterPath})
	if !report.OK || report.Network || report.Summary.NetworkOperations != 0 {
		t.Fatalf("expected offline validation to pass without network: %#v", report)
	}
	if report.Summary.ReportsValid < 5 || report.Summary.AdaptersValid != 1 || report.Summary.CacheInputsValid != 1 || report.Hash == "" {
		t.Fatalf("unexpected offline summary: %#v", report.Summary)
	}
	if !strings.Contains(report.Markdown, "offline validation") {
		t.Fatalf("expected markdown report: %s", report.Markdown)
	}
}

func TestRepoOfflineRejectsCacheHashMismatch(t *testing.T) {
	root := t.TempDir()
	analysis := filepath.Join(root, "analysis")
	cacheArchive := filepath.Join(root, "cache", "archives", "bad.tar.gz")
	writeMainTestFile(t, filepath.Dir(cacheArchive), filepath.Base(cacheArchive), "cached archive")
	writeMainTestFile(t, analysis, "fetch/source.json", fmt.Sprintf(`{
  "version":"patchline.project/v1",
  "mode":"github",
  "input":"owner/repo",
  "cache_path":%q,
  "archive_hash":"sha256:0000",
  "scanned_root":%q
}`, cacheArchive, root))
	writeMainTestFile(t, analysis, "analyze.json", `{"version":"patchline.repo-analyze/v1"}`)
	report := buildRepoOfflineReport(analysis, nil)
	if report.OK || len(report.Errors) == 0 || !strings.Contains(strings.Join(report.Errors, "\n"), "hash") {
		t.Fatalf("expected cache hash mismatch failure, got %#v", report)
	}
}

func TestRepoOfflineRejectsInvalidAdapterResult(t *testing.T) {
	root := t.TempDir()
	analysis := filepath.Join(root, "analysis")
	writeMainTestFile(t, analysis, "analyze.json", `{"version":"patchline.repo-analyze/v1"}`)
	writeMainTestFile(t, analysis, "fetch/source.json", fmt.Sprintf(`{"version":"patchline.project/v1","mode":"local","input":"%s","scanned_root":"%s"}`, root, root))
	adapterPath := filepath.Join(root, "adapter.json")
	data, _ := json.Marshal(evidence.AdaptResult{Version: evidence.AdapterVersion, Adapter: "github", OK: true, EventCount: 2, InputHash: "sha256:test", Events: []map[string]string{{"type": "deploy"}}})
	writeMainTestFile(t, root, "adapter.json", string(data))
	report := buildRepoOfflineReport(analysis, []string{adapterPath})
	if report.OK || len(report.Adapters) != 1 || report.Adapters[0].Valid {
		t.Fatalf("expected invalid adapter count failure, got %#v", report)
	}
}

func initMainTestGitRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	runMainTestGit(t, root, "init")
	runMainTestGit(t, root, "config", "user.email", "patchline@example.com")
	runMainTestGit(t, root, "config", "user.name", "Patchline Test")
	return root
}

func runMainTestGit(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(output))
	}
}

func TestRepoDoctorReportsLocalPreflight(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "go.mod", "module example.com/doctor\n\ngo 1.22\n")
	writeMainTestFile(t, root, "db/migrate/001_create_accounts.sql", "create table accounts(id int);\n")

	out := filepath.Join(t.TempDir(), "doctor")
	report, err := buildRepoDoctorReport(root, false, "", "", out, "")
	if err != nil {
		t.Fatal(err)
	}
	if report.Version != "patchline.repo-doctor/v1" || report.Summary.FilesScanned == 0 || report.Summary.Facts == 0 || !report.Summary.ReadyForAnalyze {
		t.Fatalf("unexpected doctor report: %#v", report)
	}
	if len(report.Tools) == 0 || report.Hash == "" || !strings.Contains(report.Markdown, "Patchline repo doctor") {
		t.Fatalf("expected tools, hash, and markdown: %#v", report)
	}
	if err := writeRepoDoctorReport(out, report); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(out, "doctor.json")); err != nil {
		t.Fatal(err)
	}
}

func TestRepoDoctorRejectsMissingInput(t *testing.T) {
	if err := repoDoctor([]string{"--json"}); err == nil {
		t.Fatal("expected missing input usage error")
	}
}

func TestQuickstartEmitsExactlyThreeCommands(t *testing.T) {
	out := filepath.Join(t.TempDir(), "quickstart")
	report := buildQuickstartReport("lobsters/lobsters", "abc123", "db/migrate", out)
	if report.Version != "patchline.quickstart/v1" || len(report.Commands) != 3 || len(report.ExpectedArtifacts) == 0 || report.Hash == "" {
		t.Fatalf("unexpected quickstart report: %#v", report)
	}
	if !strings.Contains(report.Commands[0].Command, "doctor --github") || !strings.Contains(report.Commands[1].Command, "repo analyze --github") || !strings.HasPrefix(report.Commands[2].Command, "test -s ") {
		t.Fatalf("unexpected commands: %#v", report.Commands)
	}
	if err := writeQuickstartReport(out, report); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"quickstart.json", "quickstart.md", "commands.sh"} {
		if _, err := os.Stat(filepath.Join(out, name)); err != nil {
			t.Fatal(err)
		}
	}
}

func TestQuickstartRequiresGitHubAndSubpath(t *testing.T) {
	if err := quickstart([]string{"--github", "lobsters/lobsters", "--json"}); err == nil {
		t.Fatal("expected missing subpath usage error")
	}
}

func TestMaintainerTriageGroupsOwnerSurfaces(t *testing.T) {
	baseline := project.BaselineReport{
		Hash: "baseline-hash",
		Risks: []project.BaselineRisk{
			{ID: "risk:migration", Path: "db/migrate/001.sql", Kind: "schema_evolution", Severity: "high", Score: 90},
			{ID: "risk:job", Path: "app/jobs/backfill.rb", Kind: "code_path", Severity: "medium", Score: 60},
		},
		NativeChecks: []project.Command{{Command: "go test ./...", Reason: "Go tests"}},
		Provenance: []project.ProvenanceSlice{{
			RiskID:        "risk:migration",
			IncidentPaths: []string{"docs/incidents/backfill.md"},
			RepairPaths:   []string{"docs/runbooks/rollback.md"},
		}},
	}
	proposal := project.ProposalReport{OutputHash: "proposal-hash", GeneratedFiles: []project.GeneratedFile{{Path: "patchline-proposals/tests/risk.md", Kind: "tests"}}}
	triage := buildMaintainerTriage(baseline, proposal, project.CompareReport{Hash: "compare-hash"})
	if triage.Version != "patchline.maintainer-triage/v1" || triage.Summary.Groups != 7 || triage.Summary.GeneratedInterventions != 1 || triage.Hash == "" {
		t.Fatalf("unexpected triage summary: %#v", triage)
	}
	if !strings.Contains(triage.Markdown, "generated_interventions") {
		t.Fatalf("expected generated intervention group in markdown: %s", triage.Markdown)
	}
}

func TestEvaluateSuppressionsClassifiesStates(t *testing.T) {
	risk := project.BaselineRisk{ID: "risk:1", StableID: "stable-risk:1234567890abcdef", Kind: "high-risk-sql", Table: "accounts", Severity: "high"}
	activeHash := suppressionEvidenceHash(risk)
	report := evaluateSuppressions(project.BaselineReport{Hash: "baseline-hash", Risks: []project.BaselineRisk{risk}}, suppressionLedger{
		Version: "patchline.suppressions/v1",
		Suppressions: []suppressionEntry{
			{StableID: risk.StableID, Owner: "db-team", Rationale: "accepted for test", Expires: "2999-01-01", EvidenceHash: activeHash},
			{StableID: risk.StableID, Owner: "db-team", Rationale: "old", Expires: "2000-01-01", EvidenceHash: activeHash},
			{StableID: risk.StableID, Owner: "db-team", Rationale: "stale", Expires: "2999-01-01", EvidenceHash: "sha256:bad"},
			{StableID: "stable-risk:ffffffffffffffff", Owner: "db-team", Rationale: "gone", Expires: "2999-01-01", EvidenceHash: activeHash},
			{StableID: risk.StableID, Expires: "2999-01-01", EvidenceHash: activeHash},
		},
	})
	if report.Summary.Active != 1 || report.Summary.Expired != 1 || report.Summary.Stale != 1 || report.Summary.Unmatched != 1 || report.Summary.Invalid != 1 {
		t.Fatalf("unexpected suppression summary: %#v", report.Summary)
	}
	if report.Hash == "" || !strings.Contains(report.Markdown, "Patchline suppressions") {
		t.Fatalf("expected hash and markdown: %#v", report)
	}
}

func TestBuildWhyNowReportFindsNewRisks(t *testing.T) {
	oldRisk := project.BaselineRisk{ID: "risk:old", StableID: "stable-risk:old0000000000000", Path: "db/migrate/old.sql", Severity: "medium", Score: 50}
	newRisk := project.BaselineRisk{ID: "risk:new", StableID: "stable-risk:new0000000000000", Path: "db/migrate/new.sql", Severity: "high", Score: 90}
	report := buildWhyNowReport(
		project.BaselineReport{Hash: "previous", Risks: []project.BaselineRisk{oldRisk}},
		project.BaselineReport{Hash: "current", Risks: []project.BaselineRisk{oldRisk, newRisk}},
	)
	if report.Summary.NewRisks != 1 || report.Summary.PersistingRisks != 1 || report.Summary.ResolvedRisks != 0 || report.NewRisks[0].StableID != newRisk.StableID {
		t.Fatalf("unexpected why-now report: %#v", report)
	}
	if report.Hash == "" || !strings.Contains(report.Markdown, "Newly introduced risks") {
		t.Fatalf("expected hash and markdown: %#v", report)
	}
}

func TestBuildPRCommentReportOnlyIncludesNewAndChangedRisks(t *testing.T) {
	unchanged := project.BaselineRisk{ID: "risk:unchanged", StableID: "stable-risk:unchanged000", Path: "db/migrate/001.sql", Kind: "sql", Table: "accounts", Severity: "medium", Score: 60, Rationale: "same"}
	changedBase := project.BaselineRisk{ID: "risk:changed", StableID: "stable-risk:changed0000", Path: "db/migrate/002.sql", Kind: "sql", Table: "users", Severity: "medium", Score: 50, Rationale: "old"}
	changedHead := changedBase
	changedHead.Severity = "high"
	changedHead.Score = 90
	changedHead.Rationale = "new broader write"
	newRisk := project.BaselineRisk{ID: "risk:new", StableID: "stable-risk:new000000000", Path: "db/migrate/003.sql", Kind: "sql", Table: "payments", Severity: "high", Score: 95, Rationale: "new risk"}
	report := buildPRCommentReport(
		project.BaselineReport{Hash: "base", Risks: []project.BaselineRisk{unchanged, changedBase}},
		project.BaselineReport{Hash: "head", Risks: []project.BaselineRisk{unchanged, changedHead, newRisk}},
		10,
	)
	if report.Summary.NewFindings != 1 || report.Summary.ChangedFindings != 1 || report.Summary.UnchangedRisks != 1 || len(report.Findings) != 2 {
		t.Fatalf("unexpected PR comment report: %#v", report)
	}
	if strings.Contains(report.Markdown, "stable-risk:unchanged") || !strings.Contains(report.Markdown, "Only new or changed") || !strings.Contains(report.Markdown, "severity medium -> high") {
		t.Fatalf("unexpected markdown:\n%s", report.Markdown)
	}
	out := filepath.Join(t.TempDir(), "pr-comment")
	if err := writePRCommentReport(out, report); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"pr-comment.json", "pr-comment.md"} {
		if _, err := os.Stat(filepath.Join(out, name)); err != nil {
			t.Fatal(err)
		}
	}
}

func TestWriteRepoAnalyzeCIArtifactsIncludesGitLabAndBitbucketOutputs(t *testing.T) {
	out := t.TempDir()
	baseline := project.BaselineReport{
		Hash: "baseline-hash",
		Risks: []project.BaselineRisk{{
			ID:        "risk:1",
			StableID:  "stable-risk:ci000000000000",
			Path:      "db/migrate/001.sql",
			Kind:      "sql",
			Table:     "accounts",
			Severity:  "high",
			Score:     99,
			Rationale: "broad account update",
		}},
	}
	if err := project.WriteBaseline(filepath.Join(out, "baseline"), baseline); err != nil {
		t.Fatal(err)
	}
	artifacts, err := writeRepoAnalyzeCIArtifacts(out, repoAnalyzeReport{Summary: repoAnalyzeSummary{RankedRisks: 1}})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{artifacts.GitLabCodeQualityPath, artifacts.BitbucketInsightsPath, artifacts.GitLabSnippet, artifacts.BitbucketSnippet} {
		if _, err := os.Stat(path); err != nil {
			t.Fatal(err)
		}
	}
	gitlabData, err := os.ReadFile(artifacts.GitLabCodeQualityPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(gitlabData), `"fingerprint": "stable-risk:ci000000000000"`) || !strings.Contains(string(gitlabData), `"severity": "major"`) {
		t.Fatalf("unexpected GitLab code quality report:\n%s", string(gitlabData))
	}
	bitbucketData, err := os.ReadFile(artifacts.BitbucketInsightsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(bitbucketData), `"result": "FAILED"`) || !strings.Contains(string(bitbucketData), `"severity": "HIGH"`) {
		t.Fatalf("unexpected Bitbucket insights report:\n%s", string(bitbucketData))
	}
}

func TestBuildChangesReportComparesAnalysisArtifacts(t *testing.T) {
	root := t.TempDir()
	previous := filepath.Join(root, "previous")
	current := filepath.Join(root, "current")
	writeAnalysisSnapshotForTest(t, previous, "fact-old", "stable-risk:old0000000000000", "generated/old.md", "sha256:old", 1, 2)
	writeAnalysisSnapshotForTest(t, current, "fact-new", "stable-risk:new0000000000000", "generated/new.md", "sha256:new", 0, 3)

	report, err := buildChangesReport(previous, current)
	if err != nil {
		t.Fatal(err)
	}
	if report.Facts.Added != 1 || report.Facts.Removed != 1 || report.Risks.Added != 1 || report.Risks.Removed != 1 || report.Links.Added != 1 || report.Generated.Added != 1 {
		t.Fatalf("unexpected changes report: %#v", report)
	}
	if report.Checks.FailureDelta != -1 || report.Checks.PassDelta != 1 || report.Hash == "" || !strings.Contains(report.Markdown, "Deterministic checks") {
		t.Fatalf("expected deterministic check deltas and markdown: %#v", report)
	}
}

func TestBuildNotifySummaryReportKeepsSlackAndGitHubCompact(t *testing.T) {
	root := t.TempDir()
	writeAnalysisSnapshotForTest(t, root, "fact-top", "stable-risk:top0000000000000", "generated/top.md", "sha256:top", 0, 3)
	writeMainTestFile(t, root, "analyze.json", `{
  "version": "patchline.repo-analyze/v1",
  "input": "bytebase/bytebase",
  "subpath": "backend/migrator/migration",
  "outputs": {"analysis_bundle":"`+filepath.ToSlash(filepath.Join(root, "analysis-bundle"))+`"},
  "source": {"mode":"github","owner":"bytebase","repo":"bytebase","ref":"main"}
}
`)
	report, err := buildNotifySummaryReport(root, "https://example.test/bundle")
	if err != nil {
		t.Fatal(err)
	}
	if report.TopMaintainerAction == "" || report.TopRisk.StableID != "stable-risk:top0000000000000" || !strings.Contains(report.ReproductionCommand, "--github bytebase/bytebase") {
		t.Fatalf("unexpected notify summary: %#v", report)
	}
	if !strings.Contains(report.SlackText, "top risk") || !strings.Contains(report.GitHubMarkdown, "**Top action:**") || report.Hash == "" {
		t.Fatalf("expected Slack/GitHub output and hash: %#v", report)
	}
}

func TestBuildFindingExplainReportJoinsEvidenceAndProofHoles(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "inventory/facts.jsonl", `{"id":"fact-risk","kind":"sql","path":"db/migrate/001.sql","confidence":"high"}`+"\n")
	writeMainTestFile(t, root, "baseline/baseline.json", `{
  "version": "patchline.baseline/v1",
  "risks": [
    {
      "id":"risk:top",
      "stable_id":"stable-risk:top0000000000000",
      "path":"db/migrate/001.sql",
      "kind":"sql",
      "table":"accounts",
      "severity":"high",
      "score":99,
      "rationale":"broad update",
      "next_command":"patchline analyze-migration db/migrate/001.sql --json",
      "factors":[{"name":"write_breadth","weight":40,"reason":"broad write"}]
    },
    {"id":"risk:alt","stable_id":"stable-risk:alt0000000000000","severity":"medium","score":50}
  ],
  "ranking_explanations": [{"risk_id":"risk:top","score":99,"severity":"high","top_feature":"write_breadth","rationale":"top-ranked broad write"}],
  "evidence_links": [{"risk_id":"risk:top","fact_id":"fact-risk","fact_kind":"sql","path":"db/migrate/001.sql","confidence":"high"}],
  "symbolic_checks": [{"id":"sym:1","risk_id":"risk:top","property":"scope","status":"fail","expression":"where required","reason":"missing concrete row bounds"}],
  "repair_proof_summaries": [{"id":"proof:1","risk_id":"risk:top","table":"accounts","status":"open","scope_status":"fail","frame_status":"warn","obligations":["scope"],"proof_holes":["no rollback witness"],"rationale":"needs repair"}],
  "hash": "baseline"
}
`)
	report, err := buildFindingExplainReport("stable-risk:top0000000000000", root)
	if err != nil {
		t.Fatal(err)
	}
	if report.Risk.ID != "risk:top" || len(report.Evidence) != 1 || len(report.RankingFactors) != 1 || len(report.Alternatives) != 1 || len(report.ProofHoles) < 2 {
		t.Fatalf("unexpected finding explanation: %#v", report)
	}
	if len(report.Verification) == 0 || !strings.Contains(report.Markdown, "Verification commands") || report.Hash == "" {
		t.Fatalf("expected verification commands and markdown: %#v", report)
	}
}

func TestBuildCorpusMinimizerReportCopiesPreservingSlice(t *testing.T) {
	root := t.TempDir()
	analysis := filepath.Join(root, "analysis")
	sourceRoot := filepath.Join(analysis, "fetch", "repo", "db", "migrate")
	writeMainTestFile(t, sourceRoot, "001_accounts.sql", "update accounts set active = true;\n")
	writeMainTestFile(t, analysis, "fetch/source.json", `{
  "version": "patchline.project/v1",
  "mode": "github",
  "input": "example/repo",
  "owner": "example",
  "repo": "repo",
  "ref": "abc",
  "subpath": "db/migrate",
  "scanned_root": "`+filepath.ToSlash(sourceRoot)+`"
}
`)
	writeMainTestFile(t, analysis, "baseline/baseline.json", `{
  "version": "patchline.baseline/v1",
  "risks": [{"id":"risk:top","stable_id":"stable-risk:top0000000000000","path":"001_accounts.sql","severity":"high","score":100}],
  "evidence_links": [{"risk_id":"risk:top","fact_id":"fact:top","fact_kind":"sql","path":"001_accounts.sql","confidence":"high"}],
  "hash": "baseline"
}
`)
	writeMainTestFile(t, analysis, "proposal/proposal.json", `{
  "version": "patchline.proposal/v1",
  "generated_files": [{"path":"generated/guard.sql","content_hash":"sha256:guard","risk_ids":["risk:top"]}],
  "hash": "proposal"
}
`)
	out := filepath.Join(root, "minimized")
	report, err := buildCorpusMinimizerReport(analysis, out)
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.Risks != 1 || report.Summary.UniqueSourceFiles != 1 || report.Summary.EvidenceLinks != 1 || report.Summary.GeneratedFiles != 1 || report.Summary.CopiedFiles != 1 {
		t.Fatalf("unexpected minimizer summary: %#v", report.Summary)
	}
	if report.Entries[0].PublicSubpath != "db/migrate" || len(report.Entries[0].GeneratedFiles) != 1 || report.Hash == "" || !strings.Contains(report.Markdown, "Minimal public subpaths") {
		t.Fatalf("unexpected minimizer report: %#v", report)
	}
	if _, err := os.Stat(filepath.Join(out, "minimized-source", "001_accounts.sql")); err != nil {
		t.Fatalf("expected copied minimized source: %v", err)
	}
}

func TestBuildRecurrenceReportRedactsPathsAcrossProjects(t *testing.T) {
	root := t.TempDir()
	left := filepath.Join(root, "left")
	right := filepath.Join(root, "right")
	writeRecurrenceAnalysisForTest(t, left, "owner/left", "risk:left", "stable-risk:left000000000000", "db/migrate/private_left.sql")
	writeRecurrenceAnalysisForTest(t, right, "owner/right", "risk:right", "stable-risk:right00000000000", "db/migrate/private_right.sql")

	report, err := buildRecurrenceReport([]string{left, right})
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.Repeated != 1 || report.Recurrences[0].ProjectCount != 2 || report.Hash == "" {
		t.Fatalf("unexpected recurrence report: %#v", report)
	}
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "private_left.sql") || strings.Contains(string(data), "private_right.sql") {
		t.Fatalf("recurrence report leaked source paths: %s", data)
	}
}

func writeRecurrenceAnalysisForTest(t *testing.T, root, project, riskID, stableID, path string) {
	t.Helper()
	writeMainTestFile(t, root, "analyze.json", `{"input":"`+project+`"}`)
	writeMainTestFile(t, root, "baseline/baseline.json", `{
  "version": "patchline.baseline/v1",
  "risks": [{
    "id":"`+riskID+`",
    "stable_id":"`+stableID+`",
    "path":"`+path+`",
    "kind":"high-risk-sql",
    "severity":"high",
    "score":90,
    "factors":[{"name":"destructive-sql","weight":80,"reason":"test"}]
  }],
  "hash": "baseline"
}
`)
}

func writeAnalysisSnapshotForTest(t *testing.T, root, factID, stableID, generatedPath, generatedHash string, failures, passed int) {
	t.Helper()
	writeMainTestFile(t, root, "inventory/facts.jsonl", `{"id":"`+factID+`","kind":"sql","path":"db/migrate/001.sql"}`+"\n")
	writeMainTestFile(t, root, "baseline/baseline.json", `{
  "version": "patchline.baseline/v1",
  "summary": {"ranked_risks": 1},
  "risks": [{"id":"risk:test","stable_id":"`+stableID+`","severity":"high","score":90}],
  "evidence_links": [{"risk_id":"risk:test","fact_id":"`+factID+`"}],
  "hash": "baseline"
}
`)
	writeMainTestFile(t, root, "proposal/proposal.json", `{
  "version": "patchline.proposal/v1",
  "generated_files": [{"path":"`+generatedPath+`","content_hash":"`+generatedHash+`"}],
  "hash": "proposal"
}
`)
	writeMainTestFile(t, root, "compare/compare.json", `{
  "version": "patchline.compare/v1",
  "summary": {"patchline_checks_failed": `+itoaForTest(failures)+`, "patchline_checks_passed": `+itoaForTest(passed)+`},
  "hash": "compare"
}
`)
}

func itoaForTest(value int) string {
	return fmt.Sprintf("%d", value)
}

func TestPhaseCheckInputKindResolvesImplicitInputs(t *testing.T) {
	tests := []struct {
		name string
		c    artifact.ManifestCase
		want string
	}{
		{
			name: "migration",
			c:    artifact.ManifestCase{CaseType: "migration"},
			want: "migration_text",
		},
		{
			name: "incident override",
			c:    artifact.ManifestCase{CaseType: "incident", InputKind: "source_observations"},
			want: "source_observations",
		},
		{
			name: "inline postmortem",
			c:    artifact.ManifestCase{CaseType: "incident", Fixture: "inline:phase-guard"},
			want: "postmortem_text",
		},
		{
			name: "repair",
			c:    artifact.ManifestCase{CaseType: "repair"},
			want: "repair_plan",
		},
		{
			name: "archive regression",
			c:    artifact.ManifestCase{CaseType: "regression", Fixture: "archive.json"},
			want: "prior_archive",
		},
		{
			name: "migration regression",
			c:    artifact.ManifestCase{CaseType: "regression", Fixture: "fix.sql"},
			want: "migration_text",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := phaseCheckInputKind(tt.c); got != tt.want {
				t.Fatalf("phaseCheckInputKind() = %q, want %q", got, tt.want)
			}
		})
	}
}

func writeMainTestFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readMainTestJSON(t *testing.T, path string, target any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, target); err != nil {
		t.Fatalf("failed to decode %s: %v\n%s", path, err, string(data))
	}
}
