package project

import (
	"archive/zip"
	"bufio"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

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
	if source.Subpath != "db/migrate" || len(source.SkippedDirs) != 1 || source.SkippedDirs[0] != ".git" {
		t.Fatalf("unexpected source.json: %s", string(data))
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
	if len(inv.Facts) < inv.FilesScanned {
		t.Fatalf("expected fact stream to include at least file facts: facts=%d files=%d", len(inv.Facts), inv.FilesScanned)
	}
	if !strings.Contains(inv.ProjectMap, "facts.jsonl") {
		t.Fatalf("expected project map to point at facts.jsonl:\n%s", inv.ProjectMap)
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

func writeBaselineForTest(t *testing.T, baseline BaselineReport) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "baseline")
	if err := WriteBaseline(dir, baseline); err != nil {
		t.Fatal(err)
	}
	return dir
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
