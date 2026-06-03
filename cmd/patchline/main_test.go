package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thehalleyyoung/patchline/internal/acceptancestudy"
	"github.com/thehalleyyoung/patchline/internal/artifact"
	"github.com/thehalleyyoung/patchline/internal/attest"
	"github.com/thehalleyyoung/patchline/internal/backfillplanner"
	"github.com/thehalleyyoung/patchline/internal/canaryvalidate"
	"github.com/thehalleyyoung/patchline/internal/certificationrenewal"
	"github.com/thehalleyyoung/patchline/internal/changemanagement"
	"github.com/thehalleyyoung/patchline/internal/education"
	"github.com/thehalleyyoung/patchline/internal/ethicsreview"
	"github.com/thehalleyyoung/patchline/internal/evidence"
	"github.com/thehalleyyoung/patchline/internal/evidencemarketplace"
	"github.com/thehalleyyoung/patchline/internal/expandcontract"
	"github.com/thehalleyyoung/patchline/internal/explainabilityaudit"
	"github.com/thehalleyyoung/patchline/internal/feedback"
	"github.com/thehalleyyoung/patchline/internal/governancerisk"
	"github.com/thehalleyyoung/patchline/internal/incidentdrill"
	"github.com/thehalleyyoung/patchline/internal/incidentpostmortem"
	"github.com/thehalleyyoung/patchline/internal/intake"
	"github.com/thehalleyyoung/patchline/internal/misuseresistance"
	"github.com/thehalleyyoung/patchline/internal/patchseries"
	"github.com/thehalleyyoung/patchline/internal/practitionercertification"
	"github.com/thehalleyyoung/patchline/internal/project"
	"github.com/thehalleyyoung/patchline/internal/remediationcost"
	"github.com/thehalleyyoung/patchline/internal/repairescrow"
	"github.com/thehalleyyoung/patchline/internal/reviewerfairness"
	"github.com/thehalleyyoung/patchline/internal/rollbackplanner"
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

func TestEvidenceMarketplacePublishCommandWritesReports(t *testing.T) {
	root := t.TempDir()
	registry := mainTestEvidenceMarketplaceRegistry(t, root)
	registryPath := filepath.Join(root, "registry.json")
	writeMainTestJSONFile(t, registryPath, registry)
	out := filepath.Join(t.TempDir(), "marketplace")
	if err := run([]string{"evidence-marketplace", "publish", "--registry", registryPath, "--out", out, "--json"}); err != nil {
		t.Fatalf("evidence marketplace publish failed: %v", err)
	}
	var report evidencemarketplace.Report
	readMainTestJSON(t, filepath.Join(out, "marketplace.json"), &report)
	if !report.OK || report.Summary.Published != 1 || report.Summary.ArtifactsVerified != 2 || report.Summary.MirroredArtifacts != 2 {
		t.Fatalf("unexpected marketplace report: %#v", report)
	}
	for _, rel := range []string{"marketplace.json", "marketplace.md", "index.html", "archive-mirror.json"} {
		if stat, err := os.Stat(filepath.Join(out, rel)); err != nil || stat.Size() == 0 {
			t.Fatalf("expected %s to be written, stat=%#v err=%v", rel, stat, err)
		}
	}
	if len(report.ArchiveMirror.Entries) != 2 {
		t.Fatalf("expected two mirror entries, got %#v", report.ArchiveMirror)
	}
	for _, entry := range report.ArchiveMirror.Entries {
		if got := mainTestFileHash(t, filepath.Join(out, filepath.FromSlash(entry.MirrorPath))); got != entry.Checksum {
			t.Fatalf("mirrored artifact checksum mismatch for %s: got %s want %s", entry.MirrorPath, got, entry.Checksum)
		}
		if entry.LicenseSPDX == "" || entry.Withdrawal.Status != "active" || entry.Withdrawal.WithdrawalID == "" {
			t.Fatalf("mirror entry missing license or withdrawal metadata: %#v", entry)
		}
	}

	bad := registry
	bad.Examples[0].Certificate.SubjectHash = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
	badPath := filepath.Join(root, "bad-registry.json")
	writeMainTestJSONFile(t, badPath, bad)
	badOut := filepath.Join(t.TempDir(), "bad-marketplace")
	if err := run([]string{"evidence-marketplace", "publish", "--registry", badPath, "--out", badOut, "--json"}); err == nil || exitCode(err) != 2 {
		t.Fatalf("expected rejected marketplace with exit code 2, got %v", err)
	}
	var rejected evidencemarketplace.Report
	readMainTestJSON(t, filepath.Join(badOut, "marketplace.json"), &rejected)
	if rejected.OK || len(rejected.Rejected) != 1 {
		t.Fatalf("expected rejected report, got %#v", rejected)
	}
	var rejectedMirror evidencemarketplace.ArchiveMirror
	readMainTestJSON(t, filepath.Join(badOut, "archive-mirror.json"), &rejectedMirror)
	if rejectedMirror.Summary.Artifacts != 0 || len(rejectedMirror.Entries) != 0 {
		t.Fatalf("expected rejected report to write an empty mirror manifest, got %#v", rejectedMirror)
	}
}

func TestEvidenceMarketplaceChallengeCommandWritesScoreboard(t *testing.T) {
	root := t.TempDir()
	registry := mainTestChallengeRegistry(t, root)
	registryPath := filepath.Join(root, "challenge-registry.json")
	writeMainTestJSONFile(t, registryPath, registry)
	out := filepath.Join(t.TempDir(), "challenge")
	if err := run([]string{"evidence-marketplace", "challenge", "--registry", registryPath, "--out", out, "--json"}); err != nil {
		t.Fatalf("evidence marketplace challenge failed: %v", err)
	}
	var report evidencemarketplace.ChallengeReport
	readMainTestJSON(t, filepath.Join(out, "challenge.json"), &report)
	if !report.OK || report.Summary.ScoreboardEntries != 1 || report.Scoreboard[0].MigrationAnalysis.HighRisk != 1 {
		t.Fatalf("unexpected challenge report: %#v", report)
	}
	for _, rel := range []string{"challenge.json", "challenge.md", "index.html"} {
		if stat, err := os.Stat(filepath.Join(out, rel)); err != nil || stat.Size() == 0 {
			t.Fatalf("expected %s to be written, stat=%#v err=%v", rel, stat, err)
		}
	}

	bad := registry
	bad.Examples[0].Challenge.Disclosure.Status = "embargoed"
	bad.Examples[0].Challenge.Disclosure.PublicReleaseAllowed = false
	bad.Examples[0].Certificate.SubjectHash = evidencemarketplace.ExpectedSubjectHash(bad.Examples[0])
	badPath := filepath.Join(root, "challenge-registry.bad.json")
	writeMainTestJSONFile(t, badPath, bad)
	badOut := filepath.Join(t.TempDir(), "bad-challenge")
	if err := run([]string{"evidence-marketplace", "challenge", "--registry", badPath, "--out", badOut, "--json"}); err == nil || exitCode(err) != 2 {
		t.Fatalf("expected rejected challenge with exit code 2, got %v", err)
	}
	var rejected evidencemarketplace.ChallengeReport
	readMainTestJSON(t, filepath.Join(badOut, "challenge.json"), &rejected)
	if rejected.OK || len(rejected.Rejected) != 1 {
		t.Fatalf("expected rejected challenge report, got %#v", rejected)
	}
}

func TestEvidenceMarketplaceGovernCommandWritesBoardReview(t *testing.T) {
	root := t.TempDir()
	registry := mainTestGovernanceRegistry(t, root)
	registryPath := filepath.Join(root, "governance-registry.json")
	writeMainTestJSONFile(t, registryPath, registry)
	spec := mainTestBoardReviewSpec(registry)
	specPath := filepath.Join(root, "governance-board.json")
	writeMainTestJSONFile(t, specPath, spec)

	out := filepath.Join(t.TempDir(), "governance")
	if err := run([]string{"evidence-marketplace", "govern", "--spec", specPath, "--out", out, "--json"}); err != nil {
		t.Fatalf("evidence marketplace govern failed: %v", err)
	}
	var report evidencemarketplace.BoardReviewReport
	readMainTestJSON(t, filepath.Join(out, "governance-board.json"), &report)
	if !report.OK || report.Summary.Accepted != 1 || report.Summary.Deprecated != 1 || report.Summary.Quarantined != 1 || report.Summary.PreservedArchiveArtifacts != 4 {
		t.Fatalf("unexpected governance board report: %#v", report.Summary)
	}
	for _, rel := range []string{"governance-board.json", "governance-board.md", "index.html"} {
		if stat, err := os.Stat(filepath.Join(out, rel)); err != nil || stat.Size() == 0 {
			t.Fatalf("expected %s to be written, stat=%#v err=%v", rel, stat, err)
		}
	}

	bad := spec
	bad.Decisions[0].Reviewers = bad.Decisions[0].Reviewers[:1]
	badPath := filepath.Join(root, "governance-board.bad.json")
	writeMainTestJSONFile(t, badPath, bad)
	badOut := filepath.Join(t.TempDir(), "bad-governance")
	if err := run([]string{"evidence-marketplace", "govern", "--spec", badPath, "--out", badOut, "--json"}); err == nil || exitCode(err) != 2 {
		t.Fatalf("expected rejected governance board with exit code 2, got %v", err)
	}
	var rejected evidencemarketplace.BoardReviewReport
	readMainTestJSON(t, filepath.Join(badOut, "governance-board.json"), &rejected)
	if rejected.OK || len(rejected.Rejected) == 0 {
		t.Fatalf("expected rejected governance report, got %#v", rejected)
	}
}

func TestEvidenceMarketplaceAppealCommandWritesAppealWorkflow(t *testing.T) {
	root := t.TempDir()
	registry := mainTestGovernanceRegistry(t, root)
	registryPath := filepath.Join(root, "governance-registry.json")
	writeMainTestJSONFile(t, registryPath, registry)
	boardSpec := mainTestBoardReviewSpec(registry)
	boardPath := filepath.Join(root, "governance-board.json")
	writeMainTestJSONFile(t, boardPath, boardSpec)
	spec := mainTestAppealWorkflowSpec(t, registry, root)
	specPath := filepath.Join(root, "appeal-workflow.json")
	writeMainTestJSONFile(t, specPath, spec)

	out := filepath.Join(t.TempDir(), "appeal")
	if err := run([]string{"evidence-marketplace", "appeal", "--spec", specPath, "--out", out, "--json"}); err != nil {
		t.Fatalf("evidence marketplace appeal failed: %v", err)
	}
	var report evidencemarketplace.AppealWorkflowReport
	readMainTestJSON(t, filepath.Join(out, "appeal-workflow.json"), &report)
	if !report.OK || report.Summary.ProcessedAppeals != 3 || report.Summary.Upheld != 1 || report.Summary.Modified != 1 || report.Summary.Overturned != 1 {
		t.Fatalf("unexpected appeal workflow report: %#v", report.Summary)
	}
	if report.Summary.PreservedArtifacts != 6 || report.Summary.ReviewerRationales != 9 || report.Summary.BoardBindings != 3 {
		t.Fatalf("appeal workflow did not preserve the expected audit trail: %#v", report.Summary)
	}
	for _, rel := range []string{"appeal-workflow.json", "appeal-workflow.md", "index.html"} {
		if stat, err := os.Stat(filepath.Join(out, rel)); err != nil || stat.Size() == 0 {
			t.Fatalf("expected %s to be written, stat=%#v err=%v", rel, stat, err)
		}
	}

	bad := spec
	bad.Appeals[0].ReviewerRationales[0].Reviewer.Name = "Database Reliability Guild"
	bad.Appeals[0].ReviewerRationales[0].Reviewer.Affiliation = "Database Reliability Guild"
	badPath := filepath.Join(root, "appeal-workflow.bad.json")
	writeMainTestJSONFile(t, badPath, bad)
	badOut := filepath.Join(t.TempDir(), "bad-appeal")
	if err := run([]string{"evidence-marketplace", "appeal", "--spec", badPath, "--out", badOut, "--json"}); err == nil || exitCode(err) != 2 {
		t.Fatalf("expected rejected appeal workflow with exit code 2, got %v", err)
	}
	var rejected evidencemarketplace.AppealWorkflowReport
	readMainTestJSON(t, filepath.Join(badOut, "appeal-workflow.json"), &rejected)
	if rejected.OK || len(rejected.Rejected) == 0 {
		t.Fatalf("expected rejected appeal workflow, got %#v", rejected)
	}
}

func TestArtifactBenchmarkImportMarketplaceCommandWritesRunnableBenchmark(t *testing.T) {
	root := t.TempDir()
	registry := mainTestEvidenceMarketplaceRegistry(t, root)
	registry.Examples[0].HazardClass = "submitter-claimed-safe-maintenance"
	registry.Examples[0].Certificate.SubjectHash = evidencemarketplace.ExpectedSubjectHash(registry.Examples[0])
	registryPath := filepath.Join(root, "registry.json")
	writeMainTestJSONFile(t, registryPath, registry)

	out := filepath.Join(t.TempDir(), "marketplace-benchmark")
	if err := run([]string{"artifact-benchmark", "import-marketplace", "--registry", registryPath, "--out", out, "--json"}); err != nil {
		t.Fatalf("marketplace import failed: %v", err)
	}
	var report artifact.MarketplaceBenchmarkImportReport
	readMainTestJSON(t, filepath.Join(out, "marketplace-import.json"), &report)
	if !report.OK || report.Summary.Imported != 1 || report.Cases[0].SubmitterLabelsTrusted {
		t.Fatalf("unexpected marketplace benchmark import report: %#v", report)
	}
	if report.Cases[0].ClaimedHazardClass == report.Cases[0].DerivedHazardClass {
		t.Fatalf("expected importer to derive an independent label: %#v", report.Cases[0])
	}
	manifestPath := filepath.Join(out, filepath.FromSlash(report.Manifest))
	if err := run([]string{"artifact-benchmark", "validate", manifestPath, "--json"}); err != nil {
		t.Fatalf("generated marketplace benchmark did not validate: %v", err)
	}
	runOut := filepath.Join(out, "run.json")
	if err := run([]string{"artifact-benchmark", "run", manifestPath, "--out", runOut, "--json"}); err != nil {
		t.Fatalf("generated marketplace benchmark did not run: %v", err)
	}
	var benchmark artifact.BenchmarkRunReport
	readMainTestJSON(t, runOut, &benchmark)
	if !benchmark.OK || benchmark.Metrics.Total != 1 || benchmark.Cases[0].ActualResult != artifact.ResultFlag {
		t.Fatalf("unexpected generated benchmark report: %#v", benchmark)
	}
}

func TestArtifactBenchmarkFederatedCommandsPublishSignedAggregateOnly(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "fixtures/private-1.sql", "UPDATE accounts SET repaired = true;\n")
	writeMainTestFile(t, root, "fixtures/private-2.sql", "UPDATE users SET repaired = true;\n")
	writeMainTestFile(t, root, "fixtures/private-3.sql", "UPDATE invoices SET repaired = true;\n")
	for _, id := range []string{"private-1", "private-2", "private-3"} {
		writeMainTestFile(t, root, "ground_truth/"+id+".json", `{
  "case_id": "`+id+`",
  "case_type": "migration",
  "phase": "pre_deploy",
  "labels": {"expected_result": "flag", "risk": "high"},
  "evidence": [{"kind": "fixture", "locator": "fixtures/`+id+`.sql", "rationale": "unscoped update should be flagged"}],
  "allowed_inputs": ["migration_text"],
  "excluded_inputs": ["postmortem_text"]
}`)
	}
	writeMainTestFile(t, root, "manifests/federated.json", `{
  "version": "patchline.artifact-benchmark/v1",
  "dataset_id": "federated-cli-test",
  "description": "federated CLI test",
  "cases": [
    {"case_id": "private-1", "case_type": "migration", "available_at": "pre_deploy", "fixture": "../fixtures/private-1.sql", "ground_truth": "../ground_truth/private-1.json"},
    {"case_id": "private-2", "case_type": "migration", "available_at": "pre_deploy", "fixture": "../fixtures/private-2.sql", "ground_truth": "../ground_truth/private-2.json"},
    {"case_id": "private-3", "case_type": "migration", "available_at": "pre_deploy", "fixture": "../fixtures/private-3.sql", "ground_truth": "../ground_truth/private-3.json"}
  ]
}`)
	out := t.TempDir()
	splitPath := filepath.Join(out, "split.json")
	if err := run([]string{
		"artifact-benchmark", "federated-split",
		"--manifest", filepath.Join(root, "manifests", "federated.json"),
		"--out", splitPath,
		"--adopter-id", "cli-adopter",
		"--min-private-cases", "3",
		"--partition-salt", strings.Repeat("0c", 16),
		"--json",
	}); err != nil {
		t.Fatalf("federated split failed: %v", err)
	}
	aggregatePath := filepath.Join(out, "aggregate.json")
	if err := run([]string{
		"artifact-benchmark", "federated-run",
		"--split", splitPath,
		"--seed-hex", strings.Repeat("01", 32),
		"--out", aggregatePath,
		"--json",
	}); err != nil {
		t.Fatalf("federated run failed: %v", err)
	}
	var aggregate artifact.FederatedBenchmarkAggregate
	readMainTestJSON(t, aggregatePath, &aggregate)
	if aggregate.Metrics.Buckets["matched"] != 3 || aggregate.Metrics.Buckets["actual:flag"] != 3 {
		t.Fatalf("unexpected aggregate metrics: %#v", aggregate.Metrics)
	}
	raw, err := os.ReadFile(aggregatePath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "private-1") || strings.Contains(string(raw), "fixture") || strings.Contains(string(raw), "ground_truth") {
		t.Fatalf("aggregate leaked private case details:\n%s", string(raw))
	}
	if err := run([]string{"artifact-benchmark", "federated-verify", "--report", aggregatePath, "--json"}); err != nil {
		t.Fatalf("federated verify failed: %v", err)
	}
}

func TestExpandContractTemplateCommandWritesReports(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "rails/app/models/invoice.rb", `class Invoice < ApplicationRecord
  before_validation :dual_write_external_id
  def dual_write_external_id
    self.external_id ||= legacy_external_id
  end
end
`)
	writeMainTestFile(t, root, "rails/db/migrate/20260101010101_expand_invoice_external_id.rb", `class ExpandInvoiceExternalId < ActiveRecord::Migration[7.1]
  def change
    add_column :invoices, :external_id, :string, null: true
  end
end
`)
	writeMainTestFile(t, root, "rails/db/migrate/20260101010202_backfill_invoice_external_id.rb", `class BackfillInvoiceExternalId < ActiveRecord::Migration[7.1]
  def up
    Invoice.where(external_id: nil).find_each { |invoice| invoice.update_all(external_id: invoice.legacy_external_id) }
  end
end
`)
	writeMainTestFile(t, root, "rails/db/migrate/20260101010303_contract_invoice_external_id.rb", `class ContractInvoiceExternalId < ActiveRecord::Migration[7.1]
  def change
    change_column_null :invoices, :external_id, false
    remove_column :invoices, :legacy_external_id
  end
end
`)
	specPath := filepath.Join(root, "expand-contract.json")
	writeMainTestFile(t, root, "expand-contract.json", `{
  "version": "patchline.expand-contract/v1",
  "name": "invoice external id expand/contract",
  "invariant_spec": {
    "version": "patchline.invariants/v1",
    "name": "invoice invariants",
    "invariants": [
      {"id":"invoice-external-id-unique","kind":"unique","table":"invoices","column":"external_id"}
    ]
  },
  "templates": [
    {"id":"invoice-external-id","invariant_id":"invoice-external-id-unique","legacy_column":"legacy_external_id","new_column":"external_id","backfill_expression":"legacy_external_id"}
  ],
  "orm_projects": [
    {"name":"rails","ecosystem":"rails","root":"rails","table":"invoices","column":"external_id","legacy_column":"legacy_external_id"}
  ]
}`)
	out := filepath.Join(t.TempDir(), "expand-contract")
	if err := run([]string{"expand-contract-template", "--spec", specPath, "--out", out, "--json"}); err != nil {
		t.Fatalf("expand-contract-template failed: %v", err)
	}
	var report expandcontract.Report
	readMainTestJSON(t, filepath.Join(out, "expand-contract-template.json"), &report)
	if !report.OK || report.Summary.Templates != 1 || report.Summary.ProjectsVerified != 1 {
		t.Fatalf("unexpected expand/contract report: %#v", report)
	}
	for _, rel := range []string{"expand-contract-template.md", "expand-contract-template.sql"} {
		if stat, err := os.Stat(filepath.Join(out, rel)); err != nil || stat.Size() == 0 {
			t.Fatalf("expected %s to be written, stat=%#v err=%v", rel, stat, err)
		}
	}
}

func TestBackfillPlanCommandWritesReports(t *testing.T) {
	root := t.TempDir()
	specPath := filepath.Join(root, "backfill-plan.json")
	storePath := filepath.Join(root, "store.json")
	writeMainTestFile(t, root, "backfill-plan.json", `{
  "version": "patchline.backfill-plan/v1",
  "name": "invoice external id staged backfill",
  "table": "invoices",
  "primary_key": "id",
  "source_column": "legacy_external_id",
  "target_column": "external_id",
  "expected_rows": 2,
  "compatibility_code_refs": ["app/models/invoice.rb:dual_write_external_id"],
  "stages": [
    {"id":"expand","kind":"expand","command":"add nullable external_id"},
    {"id":"backfill","kind":"backfill","depends_on":["expand"],"command":"copy legacy_external_id"},
    {"id":"validate","kind":"validate","depends_on":["backfill"],"command":"run validation SQL"},
    {"id":"contract","kind":"contract","depends_on":["validate"],"tightens_constraint":true,"command":"set NOT NULL"},
    {"id":"delete-compatibility","kind":"delete_compatibility","depends_on":["validate"],"deletes_compatibility":true,"command":"remove dual write"}
  ]
}`)
	writeMainTestFile(t, root, "store.json", `{
  "tables": {
    "invoices": {
      "1": {"id":"1","legacy_external_id":"inv-1","external_id":"inv-1"},
      "2": {"id":"2","legacy_external_id":"inv-2","external_id":"inv-2"}
    }
  }
}`)
	out := filepath.Join(t.TempDir(), "backfill-plan")
	if err := run([]string{"backfill-plan", "--spec", specPath, "--store", storePath, "--out", out, "--json"}); err != nil {
		t.Fatalf("backfill-plan failed: %v", err)
	}
	var report backfillplanner.Report
	readMainTestJSON(t, filepath.Join(out, "backfill-plan.json"), &report)
	if !report.OK || report.Proof.Status != "checked" || report.Summary.RowsChecked != 2 {
		t.Fatalf("unexpected backfill plan report: %#v", report)
	}
	for _, rel := range []string{"backfill-plan.md", "backfill-plan.sql"} {
		if stat, err := os.Stat(filepath.Join(out, rel)); err != nil || stat.Size() == 0 {
			t.Fatalf("expected %s to be written, stat=%#v err=%v", rel, stat, err)
		}
	}
}

func TestCanaryValidateCommandWritesReports(t *testing.T) {
	root := t.TempDir()
	specPath := filepath.Join(root, "canary-validation.json")
	beforePath := filepath.Join(root, "before.json")
	afterPath := filepath.Join(root, "after.json")
	writeMainTestFile(t, root, "canary-validation.json", `{
  "version": "patchline.canary-validation/v1",
  "name": "invoice external id canary validation",
  "sample_policy": {
    "source": "redacted billing replica sample",
    "redacted": true,
    "production_like": true,
    "sampling_basis": "deterministic tenant-stratified hash sample",
    "expected_rows": 2,
    "min_rows": 2,
    "min_matched_rows": 2,
    "redaction_salt": "main-test-canary-salt"
  },
  "invariants": [
    {"id":"invoice-row-count","kind":"row_count","table":"invoices","allowed_delta":0},
    {"id":"external-id-not-null","kind":"not_null","table":"invoices","columns":["external_id"]},
    {"id":"external-id-unique","kind":"unique","table":"invoices","columns":["external_id"]},
    {"id":"external-id-derived","kind":"equals","table":"invoices","source_column":"legacy_external_id","target_column":"external_id"},
    {"id":"stable-business-fields","kind":"unchanged","table":"invoices","columns":["account_id","amount_cents","status"]},
    {"id":"only-external-id-changes","kind":"changed_only","table":"invoices","allowed_change_columns":["external_id"]}
  ]
}`)
	writeMainTestFile(t, root, "before.json", `{
  "tables": {
    "invoices": {
      "1": {"id":"1","account_id":"acct-a","amount_cents":"1000","status":"paid","legacy_external_id":"inv-1","external_id":""},
      "2": {"id":"2","account_id":"acct-b","amount_cents":"2000","status":"open","legacy_external_id":"inv-2","external_id":""}
    }
  }
}`)
	writeMainTestFile(t, root, "after.json", `{
  "tables": {
    "invoices": {
      "1": {"id":"1","account_id":"acct-a","amount_cents":"1000","status":"paid","legacy_external_id":"inv-1","external_id":"inv-1"},
      "2": {"id":"2","account_id":"acct-b","amount_cents":"2000","status":"open","legacy_external_id":"inv-2","external_id":"inv-2"}
    }
  }
}`)
	out := filepath.Join(t.TempDir(), "canary-validation")
	if err := run([]string{"canary-validate", "--spec", specPath, "--before", beforePath, "--after", afterPath, "--out", out, "--json"}); err != nil {
		t.Fatalf("canary-validate failed: %v", err)
	}
	var report canaryvalidate.Report
	readMainTestJSON(t, filepath.Join(out, "canary-validation.json"), &report)
	if !report.OK || report.Summary.Checked != 6 || report.Summary.MatchedRows != 2 || !report.Privacy.HashOnlyEvidence {
		t.Fatalf("unexpected canary validation report: %#v", report)
	}
	for _, rel := range []string{"canary-validation.md", "canary-validation.sql"} {
		if stat, err := os.Stat(filepath.Join(out, rel)); err != nil || stat.Size() == 0 {
			t.Fatalf("expected %s to be written, stat=%#v err=%v", rel, stat, err)
		}
	}
}

func TestRepairEscrowCommandWritesReports(t *testing.T) {
	root := t.TempDir()
	specPath := filepath.Join(root, "repair-escrow.json")
	writeMainTestFile(t, root, "repair-escrow.json", `{
  "version": "patchline.repair-escrow/v1",
  "name": "main test repair escrow",
  "thresholds": {"manual_reviews": 2, "certificates": 1, "evidence": 1},
  "repairs": [
    {"id":"release-fix","title":"release safe fix","artifact_hash":"sha256:release","risk_class":"constraint-tightening"},
    {"id":"hold-fix","title":"hold until review","artifact_hash":"sha256:hold","risk_class":"broad-write"}
  ],
  "reviews": [
    {"id":"review-release-a","repair_id":"release-fix","artifact_hash":"sha256:release","reviewer":"alice","decision":"approved"},
    {"id":"review-release-b","repair_id":"release-fix","artifact_hash":"sha256:release","reviewer":"bob","decision":"approved"},
    {"id":"review-hold-a","repair_id":"hold-fix","artifact_hash":"sha256:hold","reviewer":"alice","decision":"approved"}
  ],
  "certificates": [
    {"id":"cert-release","repair_id":"release-fix","artifact_hash":"sha256:release","issuer":"plci","status":"valid"},
    {"id":"cert-hold","repair_id":"hold-fix","artifact_hash":"sha256:hold","issuer":"plci","status":"valid"}
  ],
  "evidence": [
    {"id":"evidence-release","repair_id":"release-fix","artifact_hash":"sha256:release","kind":"canary-validation","verdict":"pass"},
    {"id":"evidence-hold","repair_id":"hold-fix","artifact_hash":"sha256:hold","kind":"canary-validation","verdict":"pass"}
  ]
}`)
	out := filepath.Join(t.TempDir(), "repair-escrow")
	if err := run([]string{"repair-escrow", "--spec", specPath, "--out", out, "--json"}); err != nil {
		t.Fatalf("repair-escrow failed: %v", err)
	}
	var report repairescrow.Report
	readMainTestJSON(t, filepath.Join(out, "repair-escrow.json"), &report)
	byID := map[string]repairescrow.RepairReport{}
	for _, repair := range report.Repairs {
		byID[repair.ID] = repair
	}
	hold := byID["hold-fix"]
	if report.OK || report.Summary.Released != 1 || report.Summary.Held != 1 || len(hold.Obligations) != 1 || hold.Obligations[0].ID != "manual_review.threshold" {
		t.Fatalf("unexpected repair escrow report: %#v", report)
	}
	if stat, err := os.Stat(filepath.Join(out, "repair-escrow.md")); err != nil || stat.Size() == 0 {
		t.Fatalf("expected repair-escrow.md to be written, stat=%#v err=%v", stat, err)
	}
}

func TestIncidentPostmortemImportCommandWritesReports(t *testing.T) {
	root := mainTestRepoRoot(t)
	specPath := filepath.Join(root, "examples", "incident-postmortem-import.json")
	out := filepath.Join(t.TempDir(), "incident-postmortem-import")
	if err := run([]string{"incident-postmortem-import", "--spec", specPath, "--out", out, "--json"}); err != nil {
		t.Fatalf("incident-postmortem-import failed: %v", err)
	}
	var report incidentpostmortem.Report
	readMainTestJSON(t, filepath.Join(out, "incident-postmortem-import.json"), &report)
	if !report.OK || report.Summary.Cases != 2 || report.Summary.Regressions < 20 || report.Summary.Failed != 0 {
		t.Fatalf("unexpected incident postmortem import report: %#v", report.Summary)
	}
	for _, rel := range []string{
		"incident-postmortem-import.md",
		"detector-regressions.json",
		"generated-tests/incident_postmortem_regression_test.go",
		report.Regressions[0].Positive.Path,
		report.Regressions[0].Negatives[0].Path,
	} {
		if stat, err := os.Stat(filepath.Join(out, filepath.FromSlash(rel))); err != nil || stat.Size() == 0 {
			t.Fatalf("expected %s to be written, stat=%#v err=%v", rel, stat, err)
		}
	}
}

func TestIncidentResponseDrillCommandWritesReports(t *testing.T) {
	root := t.TempDir()
	for rel, contents := range map[string]string{
		"evidence/public-report.md":            "Public report for a Patchline false negative drill.\n",
		"evidence/detection-log.json":          `{"reproduced":true}` + "\n",
		"evidence/triage.md":                   "Triage confirmed missed nullable-column hazard.\n",
		"evidence/status.md":                   "Public status update with mitigation and remediation timing.\n",
		"evidence/mitigation.md":               "Mitigation queued writes behind a safety guard.\n",
		"evidence/remediation.md":              "Remediation adds detector regression and repair notes.\n",
		"evidence/regression-gate-report.json": `{"version":"patchline.gate-report/v1","gate_id":"incident-postmortem-importer-gate","status":"pass","checked_at":"2026-02-02T00:30:00Z"}` + "\n",
		"evidence/postmortem.md":               "Public postmortem with disclosure and remediation timeline.\n",
		"evidence/incident-commander.md":       "Incident commander ownership evidence.\n",
		"evidence/database-responder.md":       "Database responder ownership evidence.\n",
		"evidence/communications-owner.md":     "Communications owner evidence.\n",
		"evidence/data-repair-owner.md":        "Data repair owner evidence.\n",
	} {
		writeMainTestFile(t, root, rel, contents)
	}
	gateHash := mainTestFileHash(t, filepath.Join(root, "evidence/regression-gate-report.json"))
	specPath := filepath.Join(root, "incident-response-drill.json")
	writeMainTestFile(t, root, "incident-response-drill.json", fmt.Sprintf(`{
  "version": "patchline.incident-response-drill/v1",
  "name": "main test incident-response drill",
  "criteria": {
    "max_detection_minutes": 60,
    "max_public_disclosure_hours": 6,
    "max_mitigation_hours": 12,
    "max_remediation_hours": 48,
    "min_distinct_roles": 4,
    "require_public_disclosure": true,
    "require_customer_impact_statement": true,
    "require_regression_gate": true,
    "require_postmortem": true
  },
  "drill": {
    "drill_id": "fn-drill-main-test",
    "title": "Main test Patchline false negative drill",
    "scenario": "Patchline under-escalated a nullable billing migration and rehearses public response.",
    "severity": "high",
    "false_negative": {
      "detector_id": "db-semantics-nullability",
      "missed_signal_id": "nullable-column-backfill-gap",
      "original_patchline_command": "patchline repo analyze --github example/billing --subpath db/migrate --no-llm",
      "public_report_at": "2026-02-01T12:00:00Z",
      "discovered_at": "2026-02-01T13:00:00Z",
      "affected_systems": ["billing"],
      "customer_impact": "Invoices may be delayed until the guarded backfill completes."
    },
    "timeline": [
      {"id":"detected","phase":"detected","at":"2026-02-01T12:30:00Z","owner":"incident-commander","summary":"Report reproduced against the pinned migration.","evidence_path":"evidence/detection-log.json"},
      {"id":"triaged","phase":"triaged","at":"2026-02-01T13:15:00Z","owner":"database-responder","summary":"False negative classified and routed.","evidence_path":"evidence/triage.md"},
      {"id":"public-disclosure","phase":"public_disclosure","at":"2026-02-01T15:00:00Z","owner":"communications-owner","summary":"Public disclosure posted with mitigation timing.","evidence_path":"evidence/status.md"},
      {"id":"mitigated","phase":"mitigated","at":"2026-02-01T17:00:00Z","owner":"data-repair-owner","summary":"Guarded writes until backfill completed.","evidence_path":"evidence/mitigation.md"},
      {"id":"regression-added","phase":"regression_added","at":"2026-02-02T00:30:00Z","owner":"database-responder","summary":"Regression gate added.","evidence_path":"evidence/regression-gate-report.json"},
      {"id":"remediated","phase":"remediated","at":"2026-02-02T01:00:00Z","owner":"database-responder","summary":"Detector remediation complete.","evidence_path":"evidence/remediation.md"},
      {"id":"postmortem-published","phase":"postmortem_published","at":"2026-02-02T18:00:00Z","owner":"incident-commander","summary":"Public postmortem published.","evidence_path":"evidence/postmortem.md"}
    ],
    "disclosures": [
      {"id":"status-page","audience":"public","channel":"status page","planned_at":"2026-02-01T14:30:00Z","published_at":"2026-02-01T15:00:00Z","summary":"Public update names impact, mitigation, and remediation timing.","evidence_path":"evidence/status.md"}
    ],
    "remediations": [
      {"id":"detector-regression","kind":"regression_gate","owner":"database-responder","due_at":"2026-02-03T13:00:00Z","completed_at":"2026-02-02T01:00:00Z","command":"make incident-postmortem-importer-gate","gate_report_path":"evidence/regression-gate-report.json","gate_report_sha256":"%s","evidence_path":"evidence/remediation.md"},
      {"id":"customer-mitigation","kind":"customer_repair","owner":"data-repair-owner","due_at":"2026-02-02T13:00:00Z","completed_at":"2026-02-01T17:00:00Z","command":"make canary-validation-gate","evidence_path":"evidence/mitigation.md"}
    ],
    "roles": [
      {"role":"incident commander","owner":"ivy-incident","backup":"sam-backup","evidence_path":"evidence/incident-commander.md"},
      {"role":"database responder","owner":"robin-db","backup":"devon-db","evidence_path":"evidence/database-responder.md"},
      {"role":"communications owner","owner":"casey-comms","backup":"lee-comms","evidence_path":"evidence/communications-owner.md"},
      {"role":"data repair owner","owner":"drew-data","backup":"riley-data","evidence_path":"evidence/data-repair-owner.md"}
    ],
    "evidence_paths": ["evidence/public-report.md", "evidence/postmortem.md"]
  }
}`, gateHash))
	out := filepath.Join(t.TempDir(), "incident-response-drill")
	if err := run([]string{"incident-response-drill", "--spec", specPath, "--root", root, "--out", out, "--json"}); err != nil {
		t.Fatalf("incident-response-drill failed: %v", err)
	}
	var report incidentdrill.Report
	readMainTestJSON(t, filepath.Join(out, "incident-response-drill.json"), &report)
	if !report.OK || report.Summary.RegressionGates != 1 || report.Summary.PublicDisclosureHours != 2 {
		t.Fatalf("unexpected incident-response drill report: %#v", report)
	}
	if !mainTestHasMatchingIncidentRegressionGate(report) {
		t.Fatalf("expected regression gate hash to match: %#v", report.Drill.Remediations)
	}
	if stat, err := os.Stat(filepath.Join(out, "incident-response-drill.md")); err != nil || stat.Size() == 0 {
		t.Fatalf("expected incident-response-drill.md to be written, stat=%#v err=%v", stat, err)
	}

	badSpecPath := filepath.Join(root, "incident-response-drill.bad.json")
	writeMainTestFile(t, root, "incident-response-drill.bad.json", strings.ReplaceAll(mustReadMainTestFile(t, specPath), `"published_at":"2026-02-01T15:00:00Z"`, `"published_at":"2026-02-03T03:00:00Z"`))
	badOut := filepath.Join(t.TempDir(), "bad-incident-response-drill")
	if err := run([]string{"incident-response-drill", "--spec", badSpecPath, "--root", root, "--out", badOut, "--json"}); err != nil {
		t.Fatalf("incident-response-drill negative control should write an ok=false report, got %v", err)
	}
	var rejected incidentdrill.Report
	readMainTestJSON(t, filepath.Join(badOut, "incident-response-drill.json"), &rejected)
	if rejected.OK || !mainTestHasIncidentDrillCounterexample(rejected, "public_disclosure_deadline_exceeded") {
		t.Fatalf("expected rejected incident drill report, got %#v", rejected)
	}
}

func TestMultiServiceRollbackPlanCommandWritesReports(t *testing.T) {
	root := t.TempDir()
	specPath := filepath.Join(root, "multi-service-rollback-plan.json")
	writeMainTestFile(t, root, "multi-service-rollback-plan.json", `{
  "version": "patchline.multi-service-rollback-plan/v1",
  "name": "main test multi-service rollback",
  "dependency_bound": {"max_depth": 3, "max_fanout": 2, "max_waves": 3},
  "data_loss_bound": {"max_rows": 0, "max_critical_rows": 0, "max_affected_services": 0},
  "services": [
    {"id":"billing","owners":["@org/billing"],"downstream_services":["ledger"]},
    {"id":"ledger","owners":["@org/ledger"],"upstream_services":["billing"],"downstream_services":["api"]},
    {"id":"api","owners":["@org/api"],"upstream_services":["ledger"]}
  ],
  "migrations": [
    {"id":"billing-expand","service_id":"billing","stage":"expand","kind":"schema","operation":"add nullable external_id","rollback_action":"drop external_id before contract","rollback_verified":true},
    {"id":"ledger-shadow","service_id":"ledger","stage":"dual-write","kind":"application","operation":"write ledger shadow id","depends_on":["billing-expand"],"estimated_rows":7,"rollback_action":"disable dual-write flag and replay from billing snapshot","rollback_verified":true},
    {"id":"api-read-shift","service_id":"api","stage":"read-shift","kind":"application","operation":"read ledger shadow id","depends_on":["ledger-shadow"],"estimated_rows":7,"rollback_action":"restore API legacy read flag","rollback_verified":true}
  ]
}`)
	out := filepath.Join(t.TempDir(), "multi-service-rollback")
	if err := run([]string{"multi-service-rollback-plan", "--spec", specPath, "--out", out, "--json"}); err != nil {
		t.Fatalf("multi-service-rollback-plan failed: %v", err)
	}
	var report rollbackplanner.Report
	readMainTestJSON(t, filepath.Join(out, "multi-service-rollback-plan.json"), &report)
	if !report.OK || report.Summary.RollbackWaves != 3 || report.DependencyProof.MaxDepth != 3 || report.Summary.DataLossRows != 0 {
		t.Fatalf("unexpected multi-service rollback report: %#v", report)
	}
	if got, want := strings.Join(report.DependencyProof.RollbackOrder, ","), "api-read-shift,ledger-shadow,billing-expand"; got != want {
		t.Fatalf("unexpected rollback order: got %s want %s", got, want)
	}
	if stat, err := os.Stat(filepath.Join(out, "multi-service-rollback-plan.md")); err != nil || stat.Size() == 0 {
		t.Fatalf("expected multi-service-rollback-plan.md to be written, stat=%#v err=%v", stat, err)
	}
}

func TestRemediationCostCommandWritesReports(t *testing.T) {
	root := t.TempDir()
	specPath := filepath.Join(root, "remediation-cost-optimizer.json")
	writeMainTestFile(t, root, "remediation-cost-optimizer.json", `{
  "version": "patchline.remediation-cost/v1",
  "name": "main test remediation-cost optimizer",
  "thresholds": {"max_residual_loss": 500, "max_uncertainty": 0.5},
  "cases": [
    {
      "id": "runtime-guard",
      "hazard_class": "broad-write",
      "affected_rows": 100,
      "probability": 0.2,
      "impact_per_row": 100,
      "uncertainty": 0.1,
      "evidence": {"runtime_guard": true, "backfill_proof": true, "invariant_template": true, "orm_check": true, "canary_validation": true},
      "options": [
        {"id": "guard", "kind": "guard", "direct_cost": 100, "risk_reduction": 0.9, "requires": ["runtime_guard", "canary_validation"]},
        {"id": "backfill", "kind": "backfill", "direct_cost": 600, "risk_reduction": 0.95, "requires": ["backfill_proof"]},
        {"id": "expand-contract", "kind": "expand_contract", "direct_cost": 800, "risk_reduction": 0.97, "requires": ["invariant_template", "orm_check"]},
        {"id": "manual", "kind": "manual_review", "direct_cost": 1200, "risk_reduction": 0.85}
      ]
    },
    {
      "id": "verified-backfill",
      "hazard_class": "partial-backfill",
      "affected_rows": 100,
      "probability": 0.4,
      "impact_per_row": 100,
      "uncertainty": 0.05,
      "evidence": {"runtime_guard": true, "backfill_proof": true, "invariant_template": true, "orm_check": true, "canary_validation": true},
      "options": [
        {"id": "guard", "kind": "guard", "direct_cost": 300, "risk_reduction": 0.7, "requires": ["runtime_guard", "canary_validation"]},
        {"id": "backfill", "kind": "backfill", "direct_cost": 200, "risk_reduction": 0.93, "requires": ["backfill_proof"]},
        {"id": "expand-contract", "kind": "expand_contract", "direct_cost": 800, "risk_reduction": 0.97, "requires": ["invariant_template", "orm_check"]},
        {"id": "manual", "kind": "manual_review", "direct_cost": 1000, "risk_reduction": 0.9}
      ]
    },
    {
      "id": "expand-contract",
      "hazard_class": "constraint-tightening",
      "affected_rows": 200,
      "probability": 0.3,
      "impact_per_row": 50,
      "uncertainty": 0.05,
      "evidence": {"runtime_guard": true, "backfill_proof": true, "invariant_template": true, "orm_check": true, "canary_validation": true},
      "options": [
        {"id": "guard", "kind": "guard", "direct_cost": 200, "risk_reduction": 0.8, "requires": ["runtime_guard", "canary_validation"]},
        {"id": "backfill", "kind": "backfill", "direct_cost": 600, "risk_reduction": 0.85, "requires": ["backfill_proof"]},
        {"id": "expand-contract", "kind": "expand_contract", "direct_cost": 400, "risk_reduction": 0.95, "requires": ["invariant_template", "orm_check"]},
        {"id": "manual", "kind": "manual_review", "direct_cost": 1000, "risk_reduction": 0.9}
      ]
    },
    {
      "id": "uncertain-remedy",
      "hazard_class": "ambiguous-cross-service-effect",
      "affected_rows": 100,
      "probability": 0.5,
      "impact_per_row": 100,
      "uncertainty": 0.75,
      "evidence": {},
      "options": [
        {"id": "guard", "kind": "guard", "direct_cost": 100, "risk_reduction": 0.95, "requires": ["runtime_guard", "canary_validation"]},
        {"id": "backfill", "kind": "backfill", "direct_cost": 100, "risk_reduction": 0.95, "requires": ["backfill_proof"]},
        {"id": "expand-contract", "kind": "expand_contract", "direct_cost": 100, "risk_reduction": 0.95, "requires": ["invariant_template", "orm_check"]},
        {"id": "manual", "kind": "manual_review", "direct_cost": 900, "risk_reduction": 0.95}
      ]
    }
  ]
}`)
	out := filepath.Join(t.TempDir(), "remediation-cost")
	if err := run([]string{"remediation-cost", "--spec", specPath, "--out", out, "--json"}); err != nil {
		t.Fatalf("remediation-cost failed: %v", err)
	}
	var report remediationcost.Report
	readMainTestJSON(t, filepath.Join(out, "remediation-cost.json"), &report)
	if !report.OK || report.Summary.Guard != 1 || report.Summary.Backfill != 1 || report.Summary.ExpandContract != 1 || report.Summary.ManualReview != 1 {
		t.Fatalf("unexpected remediation-cost report: %#v", report.Summary)
	}
	if stat, err := os.Stat(filepath.Join(out, "remediation-cost.md")); err != nil || stat.Size() == 0 {
		t.Fatalf("expected remediation-cost.md to be written, stat=%#v err=%v", stat, err)
	}
}

func TestPatchSeriesVerifyCommandWritesReports(t *testing.T) {
	root := t.TempDir()
	specPath := filepath.Join(root, "patch-series.json")
	writeMainTestFile(t, root, "patch-series.json", `{
  "version": "patchline.patch-series/v1",
  "name": "main test patch-series verifier",
  "initial_schema": {
    "version": "patchline.schema/v1",
    "tables": [{
      "name": "invoices",
      "columns": [
        {"name":"id","type":"uuid"},
        {"name":"total_cents","type":"integer"},
        {"name":"legacy_external_id","type":"text"}
      ]
    }]
  },
  "invariants": [
    {"id":"invoices-table","kind":"table_exists","table":"invoices"},
    {"id":"invoice-id-preserved","kind":"column_exists","table":"invoices","column":"id"},
    {"id":"invoice-total-preserved","kind":"column_exists","table":"invoices","column":"total_cents"}
  ],
  "pull_requests": [
    {
      "id":"billing-expand",
      "migrations":[{"path":"db/migrate/001.sql","sql":"ALTER TABLE invoices ADD COLUMN external_id text;"}]
    },
    {
      "id":"ledger-shadow",
      "depends_on":["billing-expand"],
      "migrations":[{"path":"db/migrate/002.sql","sql":"ALTER TABLE invoices ADD COLUMN external_id_shadow text; ALTER TABLE invoices ADD COLUMN external_id_verified_at timestamp;"}]
    }
  ]
}`)
	out := filepath.Join(t.TempDir(), "patch-series")
	if err := run([]string{"patch-series-verify", "--spec", specPath, "--out", out, "--json"}); err != nil {
		t.Fatalf("patch-series-verify failed: %v", err)
	}
	var report patchseries.Report
	readMainTestJSON(t, filepath.Join(out, "patch-series.json"), &report)
	if !report.OK || report.Summary.PullRequests != 2 || report.Summary.Statements != 3 || report.Summary.Intermediate != 4 {
		t.Fatalf("unexpected patch-series report: %#v", report)
	}
	if got, want := strings.Join(report.SequenceProof.Order, ","), "billing-expand,ledger-shadow"; got != want {
		t.Fatalf("unexpected patch-series order: got %s want %s", got, want)
	}
	if stat, err := os.Stat(filepath.Join(out, "patch-series.md")); err != nil || stat.Size() == 0 {
		t.Fatalf("expected patch-series.md to be written, stat=%#v err=%v", stat, err)
	}
}

func TestMaintainerAcceptanceStudyCommandWritesReports(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "db/migrate/001_backfill.sql", "UPDATE invoices SET external_id = legacy_external_id WHERE external_id IS NULL;\n")
	writeMainTestFile(t, root, "docs/remediation-plan.md", "Review bounded backfill, rollback window, and post-backfill validation.\n")
	specPath := filepath.Join(root, "maintainer-acceptance-study.json")
	writeMainTestFile(t, root, "maintainer-acceptance-study.json", `{
  "version": "patchline.maintainer-acceptance-study/v1",
  "name": "main test maintainer acceptance study",
  "criteria": {
    "min_pairs": 2,
    "min_review_time_reduction_pct": 20,
    "min_generated_uncertainty_recall": 0.95,
    "max_uncertainty_recall_drop": 0.05,
    "max_confidence_increase": 0.15
  },
  "tasks": [{
    "id": "invoice-backfill",
    "repo": "example/billing",
    "hazard_class": "partial-backfill",
    "artifact_paths": ["db/migrate/001_backfill.sql", "docs/remediation-plan.md"],
    "ground_truth_uncertainties": ["batch-size-bound", "post-backfill-validation"]
  }],
  "observations": [
    {"participant_id":"m-1","role":"dba","task_id":"invoice-backfill","condition":"baseline","review_minutes":34,"decision":"request_changes","correct_decision":true,"confidence":0.63,"uncertainty_items_identified":["batch-size-bound"]},
    {"participant_id":"m-1","role":"dba","task_id":"invoice-backfill","condition":"generated_plan","review_minutes":20,"decision":"request_changes","correct_decision":true,"confidence":0.70,"uncertainty_items_identified":["batch-size-bound","post-backfill-validation"],"generated_plan_uncertainties":["batch-size-bound","post-backfill-validation"]},
    {"participant_id":"m-2","role":"sre","task_id":"invoice-backfill","condition":"baseline","review_minutes":31,"decision":"request_changes","correct_decision":true,"confidence":0.60,"uncertainty_items_identified":["post-backfill-validation"]},
    {"participant_id":"m-2","role":"sre","task_id":"invoice-backfill","condition":"generated_plan","review_minutes":19,"decision":"request_changes","correct_decision":true,"confidence":0.68,"uncertainty_items_identified":["batch-size-bound","post-backfill-validation"],"generated_plan_uncertainties":["batch-size-bound","post-backfill-validation"]}
  ]
}`)
	out := filepath.Join(t.TempDir(), "maintainer-acceptance-study")
	if err := run([]string{"maintainer-acceptance-study", "--spec", specPath, "--root", root, "--out", out, "--json"}); err != nil {
		t.Fatalf("maintainer-acceptance-study failed: %v", err)
	}
	var report acceptancestudy.Report
	readMainTestJSON(t, filepath.Join(out, "maintainer-acceptance-study.json"), &report)
	if !report.OK || report.Summary.Pairs != 2 || report.Summary.GeneratedUncertaintyRecall != 1 {
		t.Fatalf("unexpected maintainer acceptance report: %#v", report)
	}
	if report.Tasks[0].Artifacts[0].SHA256 == "" {
		t.Fatalf("expected artifact hashes, got %#v", report.Tasks[0].Artifacts)
	}
	if stat, err := os.Stat(filepath.Join(out, "maintainer-acceptance-study.md")); err != nil || stat.Size() == 0 {
		t.Fatalf("expected maintainer-acceptance-study.md to be written, stat=%#v err=%v", stat, err)
	}
}

func TestPractitionerCertificationCommandWritesReports(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "Makefile", "staged-backfill-planner-gate:\n\tbash scripts/staged-backfill-planner-gate.sh\n\ncanary-validation-gate:\n\tbash scripts/canary-validation-gate.sh\n")
	writeMainTestFile(t, root, "scripts/staged-backfill-planner-gate.sh", "#!/usr/bin/env bash\nset -euo pipefail\n")
	writeMainTestFile(t, root, "scripts/canary-validation-gate.sh", "#!/usr/bin/env bash\nset -euo pipefail\n")
	writeMainTestFile(t, root, "docs/staged-backfill-planner.md", "Backfill plan gates NOT NULL on complete replay-store validation.\n")
	writeMainTestFile(t, root, "examples/staged-backfill-plan.json", `{"version":"patchline.backfill-plan/v1","table":"invoices"}`)
	writeMainTestFile(t, root, "docs/canary-validation.md", "Canary validation reports hash-only counterexamples over redacted production-like snapshots.\n")
	writeMainTestFile(t, root, "examples/canary-validation-gate.json", `{"version":"patchline.canary-validation/v1","name":"fixture"}`)
	specPath := filepath.Join(root, "practitioner-certification.json")
	writeMainTestFile(t, root, "practitioner-certification.json", `{
  "version": "patchline.practitioner-certification/v1",
  "name": "main test practitioner certification",
  "claim": "Patchline grades practitioner certification attempts against hands-on gate-backed migration safety scenarios, evidence hashes, expected safety decisions, and reproducible commands instead of a prose-only badge.",
  "criteria": {
    "min_scenarios": 2,
    "min_total_points": 20,
    "passing_score_pct": 85,
    "min_gate_backed_scenarios": 2,
    "require_reproducible_commands": true
  },
  "scenarios": [
    {
      "id": "backfill-contract",
      "title": "Review a staged backfill before NOT NULL",
      "role": "database reviewer",
      "repo": "example/billing",
      "hazard_class": "partial-backfill",
      "prompt": "Decide whether contract can proceed.",
      "evidence_paths": ["docs/staged-backfill-planner.md", "examples/staged-backfill-plan.json"],
      "gate": "staged-backfill-planner-gate",
      "reproduce_commands": ["make staged-backfill-planner-gate"],
      "expected_decision": "request_changes_until_validation_proof",
      "rubric": [
        {"id":"proof","description":"Requires backfill proof before NOT NULL","points":5,"required_concepts":["backfill-completeness","not-null-contract"]},
        {"id":"compat","description":"Keeps compatibility code until validation","points":5,"required_concepts":["validate-before-contract","compatibility-code"]}
      ]
    },
    {
      "id": "canary-review",
      "title": "Review canary validation",
      "role": "SRE reviewer",
      "repo": "example/billing",
      "hazard_class": "canary-regression",
      "prompt": "Decide whether canary evidence is acceptable.",
      "evidence_paths": ["docs/canary-validation.md", "examples/canary-validation-gate.json"],
      "gate": "canary-validation-gate",
      "reproduce_commands": ["make canary-validation-gate"],
      "expected_decision": "approve_with_hash_only_counterexample_review",
      "rubric": [
        {"id":"sample","description":"Requires redacted production-like sample","points":5,"required_concepts":["redacted-production-like-sample"]},
        {"id":"hashes","description":"Keeps failures hash only","points":5,"required_concepts":["hash-only-counterexamples"]}
      ]
    }
  ],
  "attempts": [
    {"candidate_id":"candidate-a","scenario_id":"backfill-contract","decision":"request_changes_until_validation_proof","concepts":["backfill-completeness","not-null-contract","validate-before-contract","compatibility-code"],"commands":["make staged-backfill-planner-gate"]},
    {"candidate_id":"candidate-a","scenario_id":"canary-review","decision":"approve_with_hash_only_counterexample_review","concepts":["redacted-production-like-sample","hash-only-counterexamples"],"commands":["make canary-validation-gate"]}
  ]
}`)
	out := filepath.Join(t.TempDir(), "practitioner-certification")
	if err := run([]string{"practitioner-certification", "--spec", specPath, "--root", root, "--out", out, "--json"}); err != nil {
		t.Fatalf("practitioner-certification failed: %v", err)
	}
	var report practitionercertification.Report
	readMainTestJSON(t, filepath.Join(out, "practitioner-certification.json"), &report)
	if !report.OK || report.Summary.Scenarios != 2 || report.Summary.GateBackedScenarios != 2 || report.Summary.PassedCandidates != 1 {
		t.Fatalf("unexpected practitioner certification report: %#v", report)
	}
	if len(report.Scenarios[0].Evidence) == 0 || report.Scenarios[0].Evidence[0].SHA256 == "" {
		t.Fatalf("expected evidence hashes, got %#v", report.Scenarios)
	}
	if stat, err := os.Stat(filepath.Join(out, "practitioner-certification.md")); err != nil || stat.Size() == 0 {
		t.Fatalf("expected practitioner-certification.md to be written, stat=%#v err=%v", stat, err)
	}
}

func TestCertificationRenewalCommandWritesReports(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "Makefile", "certification-renewal-gate:\n\tbash scripts/certification-renewal-gate.sh\n")
	writeMainTestFile(t, root, "scripts/certification-renewal-gate.sh", "#!/usr/bin/env bash\nset -euo pipefail\n")
	writeMainTestFile(t, root, "docs/db-version-semantics.md", "Database version semantics evidence.\n")
	writeMainTestFile(t, root, "docs/db-semantics-reproducibility.md", "Database semantics reproducibility evidence.\n")
	writeMainTestFile(t, root, "docs/replication-lag-risk.md", "Replication lag evidence.\n")
	writeMainTestFile(t, root, "docs/query-plan-regression.md", "Query plan regression evidence.\n")
	writeMainTestFile(t, root, "docs/certification-renewal.md", "Certification renewal evidence.\n")
	writeMainTestFile(t, root, "docs/practitioner-certification-exam.md", "Practitioner attestation evidence.\n")
	writeMainTestFile(t, root, "examples/db-rollback-feasibility-gate.json", `{"version":"patchline.db-rollback-feasibility/v1"}`)
	writeMainTestFile(t, root, "examples/db-dry-run-gate.json", `{"version":"patchline.db-dry-run/v1"}`)
	writeMainTestFile(t, root, "examples/replication-lag-risk-gate.json", `{"version":"patchline.replication-lag-risk/v1"}`)
	writeMainTestFile(t, root, "examples/query-plan-regression-gate.json", `{"version":"patchline.query-plan-regression/v1"}`)
	specPath := filepath.Join(root, "certification-renewal.json")
	writeMainTestFile(t, root, "certification-renewal.json", `{
  "version": "patchline.certification-renewal/v1",
  "name": "main test certification renewal",
  "claim": "Patchline renews practitioner credentials only when active credentials cover newly introduced database-engine semantics, newly discovered hazard classes, evidence hashes, and reproducible gates.",
  "as_of": "2026-03-20",
  "criteria": {
    "min_engine_semantics_updates": 2,
    "min_new_hazard_classes": 2,
    "passing_score_pct": 85,
    "require_evidence_hashes": true,
    "require_reproducible_gates": true
  },
  "engine_semantics": [
    {
      "id": "postgres-16-lock-modes",
      "engine": "postgres",
      "engine_version": "16",
      "effective_date": "2026-02-15",
      "source": "docs/db-version-semantics.md",
      "summary": "PostgreSQL lock and rollback semantics are part of renewal.",
      "required_topics": ["postgres-lock-modes","transactional-ddl"],
      "evidence_paths": ["docs/db-version-semantics.md","examples/db-rollback-feasibility-gate.json"]
    },
    {
      "id": "mysql-8-online-ddl",
      "engine": "mysql",
      "engine_version": "8.0",
      "effective_date": "2026-02-20",
      "source": "docs/db-semantics-reproducibility.md",
      "summary": "MySQL online DDL and implicit commit behavior are renewal-critical.",
      "required_topics": ["mysql-online-ddl","implicit-commit-rollback"],
      "evidence_paths": ["docs/db-semantics-reproducibility.md","examples/db-dry-run-gate.json"]
    }
  ],
  "hazard_classes": [
    {
      "id": "replication-lag-risk",
      "hazard_class": "replication-lag-risk",
      "discovered_at": "2026-02-25",
      "severity": "high",
      "source": "docs/replication-lag-risk.md",
      "summary": "Renewal covers replica, CDC, and event-stream lag obligations.",
      "required_topics": ["replication-lag-obligations","cdc-delay-hazards"],
      "evidence_paths": ["docs/replication-lag-risk.md","examples/replication-lag-risk-gate.json"]
    },
    {
      "id": "query-plan-regression",
      "hazard_class": "query-plan-regression",
      "discovered_at": "2026-03-01",
      "severity": "medium",
      "source": "docs/query-plan-regression.md",
      "summary": "Renewal covers representative workload checks for index and column changes.",
      "required_topics": ["representative-workloads","plan-regression-controls"],
      "evidence_paths": ["docs/query-plan-regression.md","examples/query-plan-regression-gate.json"]
    }
  ],
  "credentials": [
    {"practitioner_id":"practitioner-a","credential_id":"patchline-migration-safety-2025","status":"active","issued_at":"2025-03-01","expires_at":"2027-03-01","track":"migration-safety"}
  ],
  "attempts": [
    {
      "practitioner_id":"practitioner-a",
      "credential_id":"patchline-migration-safety-2025",
      "submitted_at":"2026-03-15",
      "score_pct":96,
      "gate":"certification-renewal-gate",
      "commands":["make certification-renewal-gate"],
      "covered_engine_semantics":["postgres-16-lock-modes","mysql-8-online-ddl"],
      "covered_hazard_classes":["replication-lag-risk","query-plan-regression"],
      "covered_topics":["postgres-lock-modes","transactional-ddl","mysql-online-ddl","implicit-commit-rollback","replication-lag-obligations","cdc-delay-hazards","representative-workloads","plan-regression-controls"],
      "evidence_paths":["docs/certification-renewal.md"],
      "reviewer_evidence_hash":"17bdec798ae611ee11adfdca96dae07952cbd1ef3a18dde7ab56255652e4ca28",
      "reviewer_attestation_path":"docs/practitioner-certification-exam.md"
    }
  ]
}`)
	out := filepath.Join(t.TempDir(), "certification-renewal")
	if err := run([]string{"certification-renewal", "--spec", specPath, "--root", root, "--out", out, "--json"}); err != nil {
		t.Fatalf("certification-renewal failed: %v", err)
	}
	var report certificationrenewal.Report
	readMainTestJSON(t, filepath.Join(out, "certification-renewal.json"), &report)
	if !report.OK || report.Summary.EngineSemanticsUpdates != 2 || report.Summary.NewHazardClasses != 2 || report.Summary.RenewedCredentials != 1 {
		t.Fatalf("unexpected certification renewal report: %#v", report)
	}
	if len(report.EngineSemantics[0].Evidence) == 0 || report.EngineSemantics[0].Evidence[0].SHA256 == "" {
		t.Fatalf("expected evidence hashes, got %#v", report.EngineSemantics)
	}
	if stat, err := os.Stat(filepath.Join(out, "certification-renewal.md")); err != nil || stat.Size() == 0 {
		t.Fatalf("expected certification-renewal.md to be written, stat=%#v err=%v", stat, err)
	}
}

func TestClassroomLabKitsCommandWritesReports(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "Makefile", "staged-backfill-planner-gate:\n\tbash scripts/staged-backfill-planner-gate.sh\n\npatch-series-verifier-gate:\n\tbash scripts/patch-series-verifier-gate.sh\n\nsymexec-gate:\n\tbash scripts/symexec-gate.sh\n\ninfra-ordering-gate:\n\tbash scripts/infra-ordering-gate.sh\n")
	for _, gate := range []string{"staged-backfill-planner-gate", "patch-series-verifier-gate", "symexec-gate", "infra-ordering-gate"} {
		writeMainTestFile(t, root, "scripts/"+gate+".sh", "#!/usr/bin/env bash\nset -euo pipefail\n")
	}
	writeMainTestFile(t, root, "docs/staged-backfill-planner.md", "Backfill planner proof.\n")
	writeMainTestFile(t, root, "examples/staged-backfill-plan.json", `{"version":"patchline.backfill-plan/v1"}`)
	writeMainTestFile(t, root, "docs/patch-series-verifier.md", "Patch-series verifier proof.\n")
	writeMainTestFile(t, root, "examples/patch-series-verifier.json", `{"version":"patchline.patch-series/v1"}`)
	writeMainTestFile(t, root, "docs/symexec.md", "Symbolic execution proof.\n")
	writeMainTestFile(t, root, "examples/symexec-gate.json", `{"version":"patchline.symexec-gate/v1"}`)
	writeMainTestFile(t, root, "docs/infra-ordering.md", "Infrastructure ordering proof.\n")
	writeMainTestFile(t, root, "examples/infra-ordering-gate.json", `{"version":"patchline.infra-ordering-gate/v1"}`)
	specPath := filepath.Join(root, "classroom-lab-kits.json")
	writeMainTestFile(t, root, "classroom-lab-kits.json", `{
  "version": "patchline.classroom-lab-kit/v1",
  "name": "main test classroom lab kits",
  "claim": "Patchline validates classroom lab kits with instructor solution gates, evidence hashes, reproducible commands, expected outputs, and negative controls across database, software engineering, programming languages, and DevOps courses.",
  "criteria": {
    "required_audiences": ["database","software-engineering","programming-languages","devops"],
    "min_courses": 4,
    "min_labs_per_course": 1,
    "min_objectives_per_lab": 2,
    "min_evidence_artifacts_per_lab": 2,
    "require_instructor_solution_gate": true,
    "require_reproducible_command": true,
    "require_negative_control": true
  },
  "courses": [
    {"id":"database","audience":"database","title":"Database systems","repo":"example/repo","labs":[{"id":"backfill","title":"Backfill proof","hazard_class":"partial-backfill","student_prompt":"Run the gate and explain the proof.","timebox_minutes":45,"objectives":["trace evidence","explain failure"],"evidence_paths":["docs/staged-backfill-planner.md"],"instructor_solution":{"gate":"staged-backfill-planner-gate","commands":["make staged-backfill-planner-gate"],"solution_outline":["run gate"],"evidence_paths":["examples/staged-backfill-plan.json"],"expected_artifacts":["gate-summary.json"]},"negative_controls":[{"id":"missing-row","mutation":"remove row","expected_counterexample":"missing row is rejected"}]}]},
    {"id":"se","audience":"software-engineering","title":"Software engineering","repo":"example/repo","labs":[{"id":"series","title":"Patch series","hazard_class":"intermediate-state","student_prompt":"Run the gate and explain the proof.","timebox_minutes":45,"objectives":["trace invariant","explain failure"],"evidence_paths":["docs/patch-series-verifier.md"],"instructor_solution":{"gate":"patch-series-verifier-gate","commands":["make patch-series-verifier-gate"],"solution_outline":["run gate"],"evidence_paths":["examples/patch-series-verifier.json"],"expected_artifacts":["gate-summary.json"]},"negative_controls":[{"id":"unsafe-state","mutation":"unsafe state","expected_counterexample":"invariant is rejected"}]}]},
    {"id":"pl","audience":"programming-languages","title":"Programming languages","repo":"example/repo","labs":[{"id":"symexec","title":"Symbolic guard","hazard_class":"symbolic-guard","student_prompt":"Run the gate and explain the proof.","timebox_minutes":45,"objectives":["trace path","explain witness"],"evidence_paths":["docs/symexec.md"],"instructor_solution":{"gate":"symexec-gate","commands":["make symexec-gate"],"solution_outline":["run gate"],"evidence_paths":["examples/symexec-gate.json"],"expected_artifacts":["symexec.json"]},"negative_controls":[{"id":"bad-guard","mutation":"remove guard","expected_counterexample":"unsafe path is reachable"}]}]},
    {"id":"devops","audience":"devops","title":"DevOps","repo":"example/repo","labs":[{"id":"ordering","title":"Infra ordering","hazard_class":"migration-job-ordering","student_prompt":"Run the gate and explain the proof.","timebox_minutes":45,"objectives":["trace hook","explain race"],"evidence_paths":["docs/infra-ordering.md"],"instructor_solution":{"gate":"infra-ordering-gate","commands":["make infra-ordering-gate"],"solution_outline":["run gate"],"evidence_paths":["examples/infra-ordering-gate.json"],"expected_artifacts":["gate-summary.json"]},"negative_controls":[{"id":"remove-hook","mutation":"remove hook","expected_counterexample":"unordered job is rejected"}]}]}
  ]
}`)
	out := filepath.Join(t.TempDir(), "classroom-lab-kits")
	if err := run([]string{"classroom-lab-kits", "--spec", specPath, "--root", root, "--out", out, "--json"}); err != nil {
		t.Fatalf("classroom-lab-kits failed: %v", err)
	}
	var report education.LabKitReport
	readMainTestJSON(t, filepath.Join(out, "classroom-lab-kits.json"), &report)
	if !report.OK || report.Summary.Courses != 4 || report.Summary.Labs != 4 || report.Summary.GateBackedLabs != 4 || report.Summary.AudiencesCovered != 4 {
		t.Fatalf("unexpected classroom lab kit report: %#v", report)
	}
	if report.Summary.EvidenceArtifacts != 8 || len(report.Courses[0].LabReports[0].Evidence[0].SHA256) != 64 {
		t.Fatalf("expected evidence hashes, got %#v", report.Summary)
	}
	if stat, err := os.Stat(filepath.Join(out, "classroom-lab-kits.md")); err != nil || stat.Size() == 0 {
		t.Fatalf("expected classroom-lab-kits.md to be written, stat=%#v err=%v", stat, err)
	}
}

func TestLongitudinalEducationStudyCommandWritesReports(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "Makefile", "staged-backfill-planner-gate:\n\tbash scripts/staged-backfill-planner-gate.sh\n\npatch-series-verifier-gate:\n\tbash scripts/patch-series-verifier-gate.sh\n\ncanary-validation-gate:\n\tbash scripts/canary-validation-gate.sh\n")
	for _, gate := range []string{"staged-backfill-planner-gate", "patch-series-verifier-gate", "canary-validation-gate"} {
		writeMainTestFile(t, root, "scripts/"+gate+".sh", "#!/usr/bin/env bash\nset -euo pipefail\n")
	}
	writeMainTestFile(t, root, "docs/staged-backfill-planner.md", "Backfill planner proof.\n")
	writeMainTestFile(t, root, "examples/staged-backfill-plan.json", `{"version":"patchline.backfill-plan/v1"}`)
	writeMainTestFile(t, root, "docs/patch-series-verifier.md", "Patch-series verifier proof.\n")
	writeMainTestFile(t, root, "examples/patch-series-verifier.json", `{"version":"patchline.patch-series/v1"}`)
	writeMainTestFile(t, root, "docs/canary-validation.md", "Canary validation proof.\n")
	writeMainTestFile(t, root, "examples/canary-validation-gate.json", `{"version":"patchline.canary-validation/v1"}`)
	specPath := filepath.Join(root, "longitudinal-education-study.json")
	hazards := []education.LongitudinalHazard{{
		ID: "backfill", Title: "Backfill proof", Repo: "example/repo", HazardClass: "partial-backfill", RealHazard: true, HeldOut: true, Gate: "staged-backfill-planner-gate", ReproduceCommands: []string{"make staged-backfill-planner-gate"}, ExpectedDecision: "request_changes_until_validation_proof", EvidencePaths: []string{"docs/staged-backfill-planner.md", "examples/staged-backfill-plan.json"},
	}, {
		ID: "series", Title: "Patch series proof", Repo: "example/repo", HazardClass: "intermediate-state", RealHazard: true, HeldOut: true, Gate: "patch-series-verifier-gate", ReproduceCommands: []string{"make patch-series-verifier-gate"}, ExpectedDecision: "escalate_until_intermediate_invariants_pass", EvidencePaths: []string{"docs/patch-series-verifier.md", "examples/patch-series-verifier.json"},
	}, {
		ID: "canary", Title: "Canary proof", Repo: "example/repo", HazardClass: "canary-regression", RealHazard: true, HeldOut: true, Gate: "canary-validation-gate", ReproduceCommands: []string{"make canary-validation-gate"}, ExpectedDecision: "approve_with_hash_only_counterexample_review", EvidencePaths: []string{"docs/canary-validation.md", "examples/canary-validation-gate.json"},
	}}
	observations := []education.LongitudinalObservation{}
	for _, hazard := range hazards {
		observations = append(observations,
			mainLongitudinalObservation("trained", "trained-a", 0, hazard, true),
			mainLongitudinalObservation("trained", "trained-b", 6, hazard, true),
			mainLongitudinalObservation("control", "control-a", 0, hazard, hazard.ID == "backfill"),
			mainLongitudinalObservation("control", "control-b", 6, hazard, hazard.ID == "backfill"),
		)
	}
	writeMainTestJSONFile(t, specPath, education.LongitudinalStudySpec{
		Version: education.LongitudinalStudySpecVersion,
		Name:    "main test longitudinal education study",
		Claim:   "Patchline compares Patchline-trained reviewers against a control cohort months later on real gate-backed hazards, counting only evidence-cited and command-reproduced detections.",
		Criteria: education.LongitudinalCriteria{
			MinCohorts: 2, MinRealHazards: 3, MinHeldOutHazards: 3, MinFollowupMonths: 6, MinObservationsPerCohortTimepoint: 3, MinRetentionLiftPoints: 20,
			RequireControlCohort: true, RequireTrainedCohort: true, RequireBlindReview: true, RequireGateBackedHazards: true, RequireReproducibleCommands: true, RequireEvidenceCitations: true, RequireGateCommandUseForDetections: true, RequireBaseline: true,
		},
		Protocol: education.LongitudinalProtocol{RandomizationUnit: "reviewer", OutcomeDefinition: "qualified detections", BlindReview: true, FollowupMonths: []int{0, 6}},
		Hazards:  hazards,
		Cohorts: []education.LongitudinalCohort{{
			ID: "trained", Kind: "trained", Description: "Patchline-trained reviewers", Participants: []string{"trained-a", "trained-b"},
		}, {
			ID: "control", Kind: "control", Description: "ordinary onboarding reviewers", Participants: []string{"control-a", "control-b"},
		}},
		Observations: observations,
	})
	out := filepath.Join(t.TempDir(), "longitudinal-education-study")
	if err := run([]string{"longitudinal-education-study", "--spec", specPath, "--root", root, "--out", out, "--json"}); err != nil {
		t.Fatalf("longitudinal-education-study failed: %v", err)
	}
	var report education.LongitudinalStudyReport
	readMainTestJSON(t, filepath.Join(out, "longitudinal-education-study.json"), &report)
	if !report.OK || report.Summary.Cohorts != 2 || report.Summary.RealHazards != 3 || report.Summary.RetentionLiftPoints != 66.67 {
		t.Fatalf("unexpected longitudinal education report: %#v", report)
	}
	if len(report.Hazards[0].Evidence) == 0 || report.Hazards[0].Evidence[0].SHA256 == "" {
		t.Fatalf("expected evidence hashes, got %#v", report.Hazards)
	}
	if stat, err := os.Stat(filepath.Join(out, "longitudinal-education-study.md")); err != nil || stat.Size() == 0 {
		t.Fatalf("expected longitudinal-education-study.md to be written, stat=%#v err=%v", stat, err)
	}
}

func mainLongitudinalObservation(cohort, reviewer string, month int, hazard education.LongitudinalHazard, detected bool) education.LongitudinalObservation {
	obs := education.LongitudinalObservation{CohortID: cohort, ReviewerID: reviewer, TimepointMonth: month, HazardID: hazard.ID, Detected: detected}
	if detected {
		obs.Decision = hazard.ExpectedDecision
		obs.EvidenceCitations = []string{hazard.EvidencePaths[0]}
		obs.Commands = []string{"make " + hazard.Gate}
	}
	return obs
}

func TestWorkforceImpactStudyCommandWritesReports(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "Makefile", "reviewer-fairness-audit-gate:\n\tbash scripts/reviewer-fairness-audit-gate.sh\n\nlongitudinal-education-study-gate:\n\tbash scripts/longitudinal-education-study-gate.sh\n")
	writeMainTestFile(t, root, "scripts/reviewer-fairness-audit-gate.sh", "#!/usr/bin/env bash\nset -euo pipefail\n")
	writeMainTestFile(t, root, "scripts/longitudinal-education-study-gate.sh", "#!/usr/bin/env bash\nset -euo pipefail\n")
	writeMainTestFile(t, root, "docs/reviewer-fairness-audit.md", "Reviewer fairness audit checks burden, false positives, and escalation parity.\n")
	writeMainTestFile(t, root, "docs/longitudinal-education-study.md", "Longitudinal education study checks held-out reviewer learning.\n")
	specPath := filepath.Join(root, "workforce-impact-study.json")
	writeMainTestJSONFile(t, specPath, education.WorkforceImpactSpec{
		Version: education.WorkforceImpactSpecVersion,
		Name:    "main test workforce impact study",
		Claim:   "Patchline compares treated and control reviewer cohorts before and after gate-backed automation to measure ownership, escalation load, and learning outcomes without trusting raw before-after deltas.",
		Criteria: education.WorkforceImpactCriteria{
			MinCohorts:                          2,
			MinAutomationReferences:             2,
			MinObservationsPerCohortPeriod:      2,
			MinOwnershipDiffInDiffPoints:        25,
			MinEscalationDiffInDiffPoints:       20,
			MinLearningDiffInDiffPoints:         15,
			MinHeldOutDetectionDiffInDiffPoints: 15,
			MaxControlOwnershipShiftPoints:      10,
			MaxControlEscalationReductionPoints: 10,
			MaxControlLearningLiftPoints:        10,
			MaxDefectRateIncreasePoints:         0,
			MaxAttritionRate:                    0,
			RequireControlCohort:                true,
			RequireTreatedCohort:                true,
			RequireBeforeAfterPeriods:           true,
			RequireEvidenceCitations:            true,
			RequireGateCommandUse:               true,
			RequirePrivacyPreservingIDs:         true,
			RequireAutomationGateBacked:         true,
			RequireHeldOutDetectionLift:         true,
			RequireQualityGuard:                 true,
		},
		Protocol: education.WorkforceImpactProtocol{
			InterventionName:  "Patchline review automation",
			BeforePeriod:      "pre-automation",
			AfterPeriod:       "post-automation",
			AssignmentUnit:    "reviewer",
			OwnershipOutcome:  "primary owning team leads migration review",
			EscalationOutcome: "review requires DBA or SRE escalation",
			LearningOutcome:   "assessment score corroborated by held-out detection",
			QualityOutcome:    "downstream misses after review",
		},
		Automations: []education.WorkforceAutomation{{
			ID: "fairness-audit", Gate: "reviewer-fairness-audit-gate", Description: "fairness audit automation", Commands: []string{"make reviewer-fairness-audit-gate"}, EvidencePaths: []string{"docs/reviewer-fairness-audit.md", "scripts/reviewer-fairness-audit-gate.sh"},
		}, {
			ID: "longitudinal-education", Gate: "longitudinal-education-study-gate", Description: "education retention automation", Commands: []string{"make longitudinal-education-study-gate"}, EvidencePaths: []string{"docs/longitudinal-education-study.md", "scripts/longitudinal-education-study-gate.sh"},
		}},
		Cohorts: []education.WorkforceCohort{{
			ID: "treated", Kind: "treated", Description: "Patchline automation users", Participants: []string{"wf-treated-01", "wf-treated-02"},
		}, {
			ID: "control", Kind: "control", Description: "ordinary review workflow", Participants: []string{"wf-control-01", "wf-control-02"},
		}},
		Observations: []education.WorkforceObservation{
			mainWorkforceObservation("treated-before-01", "treated", "wf-treated-01", "pre-automation", false, 1, 0, 60, 1, 3, nil),
			mainWorkforceObservation("treated-before-02", "treated", "wf-treated-02", "pre-automation", false, 1, 0, 62, 1, 3, nil),
			mainWorkforceObservation("treated-after-01", "treated", "wf-treated-01", "post-automation", true, 0, 0, 86, 3, 3, []string{"fairness-audit", "longitudinal-education"}),
			mainWorkforceObservation("treated-after-02", "treated", "wf-treated-02", "post-automation", true, 0, 0, 88, 3, 3, []string{"fairness-audit", "longitudinal-education"}),
			mainWorkforceObservation("control-before-01", "control", "wf-control-01", "pre-automation", true, 1, 0, 61, 1, 3, nil),
			mainWorkforceObservation("control-before-02", "control", "wf-control-02", "pre-automation", false, 0, 0, 63, 1, 3, nil),
			mainWorkforceObservation("control-after-01", "control", "wf-control-01", "post-automation", true, 1, 0, 66, 1, 3, []string{"fairness-audit"}),
			mainWorkforceObservation("control-after-02", "control", "wf-control-02", "post-automation", false, 0, 0, 64, 1, 3, []string{"fairness-audit"}),
		},
	})
	out := filepath.Join(t.TempDir(), "workforce-impact-study")
	if err := run([]string{"workforce-impact-study", "--spec", specPath, "--root", root, "--out", out, "--json"}); err != nil {
		t.Fatalf("workforce-impact-study failed: %v", err)
	}
	var report education.WorkforceImpactReport
	readMainTestJSON(t, filepath.Join(out, "workforce-impact-study.json"), &report)
	if !report.OK || report.Summary.OwnershipDiffInDiffPoints != 100 || report.Summary.EscalationDiffInDiffPoints != 100 || report.Summary.LearningDiffInDiffPoints != 23 {
		t.Fatalf("unexpected workforce impact report: %#v", report)
	}
	if report.Summary.GateBackedAutomations != 2 || len(report.Observations[0].Evidence[0].SHA256) != 64 {
		t.Fatalf("expected gate-backed evidence hashes, got %#v", report.Summary)
	}
	if stat, err := os.Stat(filepath.Join(out, "workforce-impact-study.md")); err != nil || stat.Size() == 0 {
		t.Fatalf("expected workforce-impact-study.md to be written, stat=%#v err=%v", stat, err)
	}
}

func mainWorkforceObservation(reviewID, cohortID, participantID, period string, owned bool, escalations int, misses int, score float64, detections int, opportunities int, automations []string) education.WorkforceObservation {
	observation := education.WorkforceObservation{
		ReviewID:                reviewID,
		CohortID:                cohortID,
		ParticipantID:           participantID,
		Period:                  period,
		Team:                    "payments",
		Ecosystem:               "rails",
		OwnedByPrimaryTeam:      owned,
		Escalations:             escalations,
		DownstreamMisses:        misses,
		LearningAssessmentScore: score,
		HeldOutDetections:       detections,
		HeldOutOpportunities:    opportunities,
		AutomationRefs:          automations,
		EvidencePaths:           []string{"docs/reviewer-fairness-audit.md", "docs/longitudinal-education-study.md"},
	}
	for _, automation := range automations {
		switch automation {
		case "fairness-audit":
			observation.Commands = append(observation.Commands, "make reviewer-fairness-audit-gate")
		case "longitudinal-education":
			observation.Commands = append(observation.Commands, "make longitudinal-education-study-gate")
		}
	}
	return observation
}

func TestContributorApprenticeshipCommandWritesReports(t *testing.T) {
	root := t.TempDir()
	gates := []string{"query-plan-regression-gate", "online-schema-change-adapters-gate", "staged-backfill-planner-gate"}
	var makefile strings.Builder
	for _, gate := range gates {
		fmt.Fprintf(&makefile, "%s:\n\tbash scripts/%s.sh\n\n", gate, gate)
		writeMainTestFile(t, root, "scripts/"+gate+".sh", "#!/usr/bin/env bash\nset -euo pipefail\n")
	}
	writeMainTestFile(t, root, "Makefile", makefile.String())
	writeMainTestFile(t, root, "internal/dbsemantics/semantics.go", "package dbsemantics\n\ntype QueryPlanRegression struct{}\ntype OnlineSchemaChange struct{}\nfunc detectQueryPlanRegression() *QueryPlanRegression { return nil }\nfunc detectOnlineSchemaChange() *OnlineSchemaChange { return nil }\n")
	writeMainTestFile(t, root, "internal/dbsemantics/semantics_test.go", "package dbsemantics\n")
	writeMainTestFile(t, root, "internal/backfillplanner/planner.go", "package backfillplanner\n\ntype CompletenessProof struct{}\nfunc BuildPlan() CompletenessProof { return CompletenessProof{} }\n")
	writeMainTestFile(t, root, "internal/backfillplanner/planner_test.go", "package backfillplanner\n")
	writeMainTestFile(t, root, "docs/query-plan-regression.md", "query-plan regression evidence; run make query-plan-regression-gate\n")
	writeMainTestFile(t, root, "docs/online-schema-change-adapters.md", "online-schema-change adapters evidence; run make online-schema-change-adapters-gate\n")
	writeMainTestFile(t, root, "docs/staged-backfill-planner.md", "Staged data-backfill plan evidence; run make staged-backfill-planner-gate\n")
	writeMainTestFile(t, root, "examples/query-plan-regression-gate.json", `{"version":"patchline.query-plan-regression-gate/v1"}`)
	writeMainTestFile(t, root, "examples/query-plan-negative-control.json", `{"version":"patchline.apprenticeship-negative-control/v1"}`)
	writeMainTestFile(t, root, "examples/online-schema-change-adapters-gate.json", `{"version":"patchline.online-schema-change-adapters-gate/v1"}`)
	writeMainTestFile(t, root, "examples/online-schema-negative-control.json", `{"version":"patchline.apprenticeship-negative-control/v1"}`)
	writeMainTestFile(t, root, "examples/staged-backfill-plan.json", `{"version":"patchline.backfill-plan/v1"}`)
	writeMainTestFile(t, root, "examples/staged-backfill-store-incomplete.json", `{"tables":{"invoices":{"3":{"external_id":""}}}}`)

	specPath := filepath.Join(root, "contributor-apprenticeship.json")
	writeMainTestJSONFile(t, specPath, education.ApprenticeshipSpec{
		Version: education.ApprenticeshipSpecVersion,
		Name:    "main test contributor apprenticeship",
		Claim:   "Patchline contributor apprenticeship graduation requires detector code, a gate, documentation, minimized fixtures, negative controls, mentor signoff, and independent reviews.",
		Criteria: education.ApprenticeshipCriteria{
			MinTracks:                   3,
			RequiredDeliverables:        []string{"detector", "gate", "doc", "fixture"},
			MinReviewers:                2,
			MaxFixtureBytes:             8192,
			RequireMentorSignoff:        true,
			RequireReproducibleGate:     true,
			RequireMinimizedFixture:     true,
			RequireDetectorSymbol:       true,
			RequireNegativeControl:      true,
			RequireDocumentationPhrases: true,
		},
		Tracks: []education.ApprenticeshipTrack{
			mainApprenticeshipTrack("query", "query-plan-regression", "internal/dbsemantics/semantics.go", "detectQueryPlanRegression", "QueryPlanRegression", "query-plan-regression-gate", "docs/query-plan-regression.md", []string{"query-plan regression", "make query-plan-regression-gate"}, "examples/query-plan-regression-gate.json", "examples/query-plan-negative-control.json", "internal/dbsemantics/semantics_test.go"),
			mainApprenticeshipTrack("online", "online-schema-change", "internal/dbsemantics/semantics.go", "detectOnlineSchemaChange", "OnlineSchemaChange", "online-schema-change-adapters-gate", "docs/online-schema-change-adapters.md", []string{"online-schema-change adapters", "make online-schema-change-adapters-gate"}, "examples/online-schema-change-adapters-gate.json", "examples/online-schema-negative-control.json", "internal/dbsemantics/semantics_test.go"),
			mainApprenticeshipTrack("backfill", "partial-backfill", "internal/backfillplanner/planner.go", "BuildPlan", "CompletenessProof", "staged-backfill-planner-gate", "docs/staged-backfill-planner.md", []string{"Staged data-backfill plan", "make staged-backfill-planner-gate"}, "examples/staged-backfill-plan.json", "examples/staged-backfill-store-incomplete.json", "internal/backfillplanner/planner_test.go"),
		},
	})
	out := filepath.Join(t.TempDir(), "contributor-apprenticeship")
	if err := run([]string{"contributor-apprenticeship", "--spec", specPath, "--root", root, "--out", out, "--json"}); err != nil {
		t.Fatalf("contributor-apprenticeship failed: %v", err)
	}
	var report education.ApprenticeshipReport
	readMainTestJSON(t, filepath.Join(out, "contributor-apprenticeship.json"), &report)
	if !report.OK || report.Summary.Tracks != 3 || report.Summary.GraduatedTracks != 3 || report.Summary.DeliverablesVerified != 12 {
		t.Fatalf("unexpected contributor apprenticeship report: %#v", report)
	}
	if report.Summary.EvidenceArtifacts != 15 || len(report.Tracks[0].Evidence[0].SHA256) != 64 {
		t.Fatalf("expected evidence hashes, got %#v", report.Summary)
	}
	if stat, err := os.Stat(filepath.Join(out, "contributor-apprenticeship.md")); err != nil || stat.Size() == 0 {
		t.Fatalf("expected contributor-apprenticeship.md to be written, stat=%#v err=%v", stat, err)
	}
}

func mainApprenticeshipTrack(id, hazard, detectorPath, symbol, signal, gate, doc string, phrases []string, fixture, negative, evidencePath string) education.ApprenticeshipTrack {
	return education.ApprenticeshipTrack{
		ID:            id,
		Title:         id + " apprenticeship",
		HazardClass:   hazard,
		ContributorID: id + "-contributor",
		MentorID:      id + "-mentor",
		Repo:          "example/repo",
		Detector: education.ApprenticeshipDetector{
			Path:           detectorPath,
			Symbol:         symbol,
			ExpectedSignal: signal,
			EvidencePaths:  []string{evidencePath},
		},
		Gate: education.ApprenticeshipGate{
			Name:              gate,
			Commands:          []string{"make " + gate},
			ExpectedArtifacts: []string{"gate-summary.json"},
			NegativeControls: []education.ApprenticeshipNegativeControl{{
				ID:                     "negative",
				Mutation:               "remove the proof marker",
				ExpectedCounterexample: "missing proof is rejected",
			}},
		},
		Documentation: education.ApprenticeshipDocumentation{Path: doc, RequiredPhrases: phrases},
		Fixture:       education.ApprenticeshipFixture{Path: fixture, Minimized: true, NegativeControlPath: negative},
		Review:        education.ApprenticeshipReview{Reviewers: []string{id + "-reviewer-a", id + "-reviewer-b"}, MentorSignoff: true, MergedPRs: []string{"local:" + id}},
	}
}

func TestSkillsTaxonomyCommandWritesReports(t *testing.T) {
	root := t.TempDir()
	gates := []string{"staged-backfill-planner-gate", "query-plan-regression-gate", "online-schema-change-adapters-gate", "data-retention-privacy-gate", "infra-ordering-gate"}
	var makefile strings.Builder
	for _, gate := range gates {
		fmt.Fprintf(&makefile, "%s:\n\tbash scripts/%s.sh\n\n", gate, gate)
		writeMainTestFile(t, root, "scripts/"+gate+".sh", "#!/usr/bin/env bash\nset -euo pipefail\n")
	}
	writeMainTestFile(t, root, "Makefile", makefile.String())
	writeMainTestFile(t, root, "docs/staged-backfill-planner.md", "Backfill proof.\n")
	writeMainTestFile(t, root, "examples/staged-backfill-plan.json", `{"version":"patchline.backfill-plan/v1"}`)
	writeMainTestFile(t, root, "docs/query-plan-regression.md", "Query-plan proof.\n")
	writeMainTestFile(t, root, "examples/query-plan-regression-gate.json", `{"version":"patchline.query-plan-regression-gate/v1"}`)
	writeMainTestFile(t, root, "docs/online-schema-change-adapters.md", "Online schema proof.\n")
	writeMainTestFile(t, root, "examples/online-schema-change-adapters-gate.json", `{"version":"patchline.online-schema-change-adapters-gate/v1"}`)
	writeMainTestFile(t, root, "docs/privacy-impact.md", "Privacy proof.\n")
	writeMainTestFile(t, root, "examples/data-retention-privacy-hazards.json", `{"version":"patchline.data-retention-privacy-hazards/v1"}`)
	writeMainTestFile(t, root, "docs/infra-ordering.md", "Infra ordering proof.\n")
	writeMainTestFile(t, root, "examples/infra-ordering-gate.json", `{"version":"patchline.infra-ordering-gate/v1"}`)

	specPath := filepath.Join(root, "skills-taxonomy.json")
	writeMainTestJSONFile(t, specPath, education.SkillsTaxonomySpec{
		Version: education.SkillsTaxonomySpecVersion,
		Name:    "main test skills taxonomy",
		Claim:   "Patchline maps gate-backed data-change hazard classes to reviewer concepts, prerequisites, assessment prompts, role audiences, evidence hashes, certification crosswalks, and negative controls.",
		Criteria: education.SkillsTaxonomyCriteria{
			RequiredAudiences:             []string{"app-developer", "dba", "sre", "security-reviewer", "engineering-manager"},
			MinHazardClasses:              5,
			MinConceptsPerHazard:          2,
			MinPrerequisitesPerConcept:    2,
			MinEvidenceArtifactsPerHazard: 3,
			RequireGate:                   true,
			RequireReproducibleCommand:    true,
			RequireNegativeControl:        true,
			RequireAssessmentPrompt:       true,
			RequireCrosswalk:              true,
		},
		HazardClasses: []education.SkillHazardClass{
			mainSkillHazard("partial-backfill", []string{"app-developer", "dba"}, "staged-backfill-planner-gate", "docs/staged-backfill-planner.md", "examples/staged-backfill-plan.json", "scripts/staged-backfill-planner-gate.sh"),
			mainSkillHazard("query-plan-regression", []string{"app-developer", "dba"}, "query-plan-regression-gate", "docs/query-plan-regression.md", "examples/query-plan-regression-gate.json", "scripts/query-plan-regression-gate.sh"),
			mainSkillHazard("online-schema-change", []string{"dba", "sre"}, "online-schema-change-adapters-gate", "docs/online-schema-change-adapters.md", "examples/online-schema-change-adapters-gate.json", "scripts/online-schema-change-adapters-gate.sh"),
			mainSkillHazard("data-retention-privacy", []string{"security-reviewer", "engineering-manager"}, "data-retention-privacy-gate", "docs/privacy-impact.md", "examples/data-retention-privacy-hazards.json", "scripts/data-retention-privacy-gate.sh"),
			mainSkillHazard("migration-job-ordering", []string{"sre", "engineering-manager"}, "infra-ordering-gate", "docs/infra-ordering.md", "examples/infra-ordering-gate.json", "scripts/infra-ordering-gate.sh"),
		},
	})
	out := filepath.Join(t.TempDir(), "skills-taxonomy")
	if err := run([]string{"skills-taxonomy", "--spec", specPath, "--root", root, "--out", out, "--json"}); err != nil {
		t.Fatalf("skills-taxonomy failed: %v", err)
	}
	var report education.SkillsTaxonomyReport
	readMainTestJSON(t, filepath.Join(out, "skills-taxonomy.json"), &report)
	if !report.OK || report.Summary.HazardClasses != 5 || report.Summary.Concepts != 10 || report.Summary.GateBackedHazards != 5 {
		t.Fatalf("unexpected skills taxonomy report: %#v", report)
	}
	if report.Summary.EvidenceArtifacts != 15 || len(report.HazardClasses[0].Evidence[0].SHA256) != 64 {
		t.Fatalf("expected evidence hashes, got %#v", report.Summary)
	}
	if stat, err := os.Stat(filepath.Join(out, "skills-taxonomy.md")); err != nil || stat.Size() == 0 {
		t.Fatalf("expected skills-taxonomy.md to be written, stat=%#v err=%v", stat, err)
	}
}

func mainSkillHazard(hazard string, audiences []string, gate, docPath, fixturePath, scriptPath string) education.SkillHazardClass {
	return education.SkillHazardClass{
		HazardClass:       hazard,
		Title:             hazard + " skills",
		SeverityBand:      "high",
		ReviewerAudiences: audiences,
		Concepts: []education.ReviewerConcept{{
			ID:               hazard + "-evidence",
			Title:            "Trace " + hazard + " evidence",
			Description:      "Reviewer can trace evidence to the hazard class.",
			Prerequisites:    []string{"repo evidence navigation", "data-change failure modes"},
			AssessmentPrompt: "Explain the evidence hash for " + hazard + ".",
			EvidencePaths:    []string{docPath},
		}, {
			ID:               hazard + "-control",
			Title:            "Apply " + hazard + " control",
			Description:      "Reviewer can apply the negative control for the hazard.",
			Prerequisites:    []string{"gate output reading", "negative-control reasoning"},
			AssessmentPrompt: "Run the gate and describe the negative control for " + hazard + ".",
			EvidencePaths:    []string{fixturePath},
		}},
		Gates: []education.TaxonomyGate{{
			Name:          gate,
			Commands:      []string{"make " + gate},
			EvidencePaths: []string{scriptPath},
			NegativeControls: []education.TaxonomyNegativeControl{{
				ID:                     "negative",
				Mutation:               "remove required taxonomy evidence",
				ExpectedCounterexample: "missing evidence is rejected",
			}},
		}},
		RelatedTutorials:              []string{"tutorial:" + hazard},
		RelatedCertificationScenarios: []string{"certification:" + hazard},
	}
}

func TestLocalizedTeachingExamplesCommandWritesReports(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "Makefile", "localized-teaching-examples-gate:\n\tbash scripts/localized-teaching-examples-gate.sh\n\n")
	writeMainTestFile(t, root, "scripts/localized-teaching-examples-gate.sh", "#!/usr/bin/env bash\nset -euo pipefail\n")
	writeMainTestFile(t, root, "docs/classroom-lab-kits.md", "Classroom lab kit evidence.\n")
	writeMainTestFile(t, root, "docs/db-semantics-reproducibility.md", "Database semantics evidence.\n")
	writeMainTestFile(t, root, "docs/a11y-i18n-output.md", "Accessibility and i18n evidence.\n")
	writeMainTestFile(t, root, "docs/accessibility-conformance.md", "WCAG conformance evidence.\n")
	writeMainTestFile(t, root, "docs/localized-teaching-examples.md", "Localized teaching examples evidence.\n")
	specPath := filepath.Join(root, "localized-teaching-examples.json")
	writeMainTestJSONFile(t, specPath, education.LocalizedTeachingSpec{
		Version: education.LocalizedTeachingSpecVersion,
		Name:    "main test localized teaching examples",
		Claim:   "Patchline validates localized teaching examples by preserving byte-identical commands and identifiers across translated lessons, hashing real evidence, checking accessibility requirements, and rejecting negative controls.",
		Criteria: education.LocalizedTeachingCriteria{
			RequiredLocales:                      []string{"es", "fr"},
			RequiredAudiences:                    []string{"app-developer", "dba"},
			RequiredAccessibilityChecks:          []string{"plain-language", "alt-text", "reading-order"},
			MinExamples:                          2,
			MinTranslationsPerExample:            2,
			MinConceptsPerExample:                2,
			MinTechnicalTermsPerTranslation:      3,
			MinEquivalenceChecksPerTranslation:   2,
			MinAccessibilityChecksPerTranslation: 3,
			RequireTechnicalTerms:                true,
			RequireEquivalenceChecks:             true,
			RequireAccessibilityChecks:           true,
			RequireReproducibleCommand:           true,
			RequireNegativeControl:               true,
		},
		Examples: []education.LocalizedExample{
			mainLocalizedExample("app", "app-developer", "make classroom-lab-kits-gate", "risk_id", "evidence_hash", "docs/classroom-lab-kits.md"),
			mainLocalizedExample("dba", "dba", "make db-semantics-reproducibility-gate", "engine_version", "rollback_feasibility", "docs/db-semantics-reproducibility.md"),
		},
	})
	out := filepath.Join(t.TempDir(), "localized-teaching-examples")
	if err := run([]string{"localized-teaching-examples", "--spec", specPath, "--root", root, "--out", out, "--json"}); err != nil {
		t.Fatalf("localized-teaching-examples failed: %v", err)
	}
	var report education.LocalizedTeachingReport
	readMainTestJSON(t, filepath.Join(out, "localized-teaching-examples.json"), &report)
	if !report.OK || report.Summary.Examples != 2 || report.Summary.Translations != 4 || report.Summary.TechnicalTerms != 12 || report.Summary.AccessibilityChecks != 12 {
		t.Fatalf("unexpected localized teaching report: %#v", report)
	}
	if report.Summary.EvidenceArtifacts != 20 || len(report.Examples[0].Translations[0].Evidence[0].SHA256) != 64 {
		t.Fatalf("expected evidence hashes, got %#v", report.Summary)
	}
	if stat, err := os.Stat(filepath.Join(out, "localized-teaching-examples.md")); err != nil || stat.Size() == 0 {
		t.Fatalf("expected localized-teaching-examples.md to be written, stat=%#v err=%v", stat, err)
	}
}

func mainLocalizedExample(id, audience, command, firstIdentifier, secondIdentifier, evidencePath string) education.LocalizedExample {
	sourceText := "Run `" + command + "` and keep `" + firstIdentifier + "` plus `" + secondIdentifier + "` visible in the Patchline lesson."
	return education.LocalizedExample{
		ID:                id,
		Audience:          audience,
		SourceLocale:      "en",
		Title:             id + " localized lesson",
		SourceText:        sourceText,
		Concepts:          []string{"technical equivalence", "accessible teaching"},
		EvidencePaths:     []string{evidencePath, "docs/a11y-i18n-output.md"},
		ReproduceCommands: []string{"make localized-teaching-examples-gate", "make " + strings.TrimPrefix(command, "make ")},
		Translations: []education.LocalizedTranslation{
			mainLocalizedTranslation("es", command, firstIdentifier, secondIdentifier, evidencePath),
			mainLocalizedTranslation("fr", command, firstIdentifier, secondIdentifier, evidencePath),
		},
		NegativeControls: []education.LocalizedControl{{
			ID:                     "translate-token",
			Mutation:               "translate a preserved identifier",
			ExpectedCounterexample: "missing preserved technical token",
		}},
	}
}

func mainLocalizedTranslation(locale, command, firstIdentifier, secondIdentifier, evidencePath string) education.LocalizedTranslation {
	joiner := "y"
	text := "Ejecute `" + command + "` y mantenga `" + firstIdentifier + "` y `" + secondIdentifier + "` visibles en la leccion Patchline."
	if locale == "fr" {
		joiner = "et"
		text = "Executez `" + command + "` et gardez `" + firstIdentifier + "` et `" + secondIdentifier + "` visibles dans la lecon Patchline."
	}
	return education.LocalizedTranslation{
		Locale: locale,
		Title:  locale + " localized lesson",
		Text:   text,
		TechnicalTerms: []education.LocalizedTechnicalTerm{{
			ID: "patchline", Source: "Patchline", Translation: "Patchline", MustPreserve: true,
		}, {
			ID: "first-identifier", Source: firstIdentifier, Translation: firstIdentifier, MustPreserve: true,
		}, {
			ID: "second-identifier", Source: secondIdentifier, Translation: secondIdentifier, MustPreserve: true,
		}},
		EquivalenceChecks: []education.LocalizedEquivalenceCheck{{
			ID: "command-preserved", Kind: "command-preservation", SourceQuote: "`" + command + "`", TranslatedQuote: "`" + command + "`", PreservedTokens: []string{command},
		}, {
			ID: "identifiers-preserved", Kind: "identifier-preservation", SourceQuote: "`" + firstIdentifier + "` plus `" + secondIdentifier + "`", TranslatedQuote: "`" + firstIdentifier + "` " + joiner + " `" + secondIdentifier + "`", PreservedTokens: []string{firstIdentifier, secondIdentifier},
		}},
		AccessibilityChecks: []education.LocalizedAccessibilityCheck{{
			ID: "plain-language", Type: "plain-language", Requirement: "plain language", EvidencePaths: []string{"docs/a11y-i18n-output.md"},
		}, {
			ID: "alt-text", Type: "alt-text", Requirement: "alt text names the command", EvidencePaths: []string{"docs/accessibility-conformance.md"},
		}, {
			ID: "reading-order", Type: "reading-order", Requirement: "command appears before decision", EvidencePaths: []string{"docs/localized-teaching-examples.md"},
		}},
		EvidencePaths: []string{evidencePath},
	}
}

func TestOpenTextbookCompanionCommandWritesReports(t *testing.T) {
	root := t.TempDir()
	spec := education.TextbookCompanionSpec{
		Version: education.TextbookCompanionSpecVersion,
		Name:    "main test open textbook companion",
		Claim:   "Patchline validates an open textbook companion by checking executable notebooks with exact regeneration commands, hashed source evidence, generated report artifacts, teaching objectives, and negative controls for every teaching example.",
		Criteria: education.TextbookCompanionCriteria{
			RequiredChapters:                 []string{"classroom-labs", "reviewer-skills", "localized-lessons"},
			MinChapters:                      3,
			MinNotebooksPerChapter:           1,
			MinExamplesPerNotebook:           1,
			MinCommandsPerNotebook:           1,
			MinLearningObjectivesPerExample:  2,
			MinEvidenceArtifactsPerNotebook:  3,
			MinGeneratedArtifactsPerNotebook: 2,
			RequireExecutableNotebook:        true,
			RequireReproducibleCommands:      true,
			RequireGeneratedArtifacts:        true,
			RequireNegativeControl:           true,
		},
		Chapters: []education.TextbookChapter{
			mainTextbookChapter("classroom-labs", "Classroom labs", "educator", "examples/textbook-companion/01-classroom-lab-kits.ipynb", "go run ./cmd/patchline classroom-lab-kits --spec examples/classroom-lab-kits.json --root . --out results/generated/textbook-companion/classroom-lab-kits --json", "docs/classroom-lab-kits.md", "examples/classroom-lab-kits.json", "results/generated/textbook-companion/classroom-lab-kits/classroom-lab-kits.json", "results/generated/textbook-companion/classroom-lab-kits/classroom-lab-kits.md"),
			mainTextbookChapter("reviewer-skills", "Reviewer skills", "reviewer", "examples/textbook-companion/02-skills-taxonomy.ipynb", "go run ./cmd/patchline skills-taxonomy --spec examples/skills-taxonomy.json --root . --out results/generated/textbook-companion/skills-taxonomy --json", "docs/skills-taxonomy.md", "examples/skills-taxonomy.json", "results/generated/textbook-companion/skills-taxonomy/skills-taxonomy.json", "results/generated/textbook-companion/skills-taxonomy/skills-taxonomy.md"),
			mainTextbookChapter("localized-lessons", "Localized lessons", "translator", "examples/textbook-companion/03-localized-teaching-examples.ipynb", "go run ./cmd/patchline localized-teaching-examples --spec examples/localized-teaching-examples.json --root . --out results/generated/textbook-companion/localized-teaching-examples --json", "docs/localized-teaching-examples.md", "examples/localized-teaching-examples.json", "results/generated/textbook-companion/localized-teaching-examples/localized-teaching-examples.json", "results/generated/textbook-companion/localized-teaching-examples/localized-teaching-examples.md"),
		},
	}
	for _, chapter := range spec.Chapters {
		notebook := chapter.Notebooks[0]
		writeMainTestFile(t, root, notebook.Path, mainTextbookNotebookJSON(notebook.ExecuteCommands[0]))
		for _, path := range notebook.EvidencePaths {
			writeMainTestFile(t, root, path, "Evidence for "+path+".\n")
		}
		for _, path := range notebook.ExpectedArtifacts {
			writeMainTestFile(t, root, path, "Generated artifact for "+path+".\n")
		}
	}
	specPath := filepath.Join(root, "open-textbook-companion.json")
	writeMainTestJSONFile(t, specPath, spec)
	out := filepath.Join(t.TempDir(), "open-textbook-companion")
	if err := run([]string{"open-textbook-companion", "--spec", specPath, "--root", root, "--out", out, "--json"}); err != nil {
		t.Fatalf("open-textbook-companion failed: %v", err)
	}
	var report education.TextbookCompanionReport
	readMainTestJSON(t, filepath.Join(out, "open-textbook-companion.json"), &report)
	if !report.OK || report.Summary.Chapters != 3 || report.Summary.ExecutableNotebooks != 3 || report.Summary.GeneratedArtifacts != 6 {
		t.Fatalf("unexpected textbook companion report: %#v", report)
	}
	if len(report.Chapters[0].NotebookReports[0].GeneratedArtifacts[0].SHA256) != 64 {
		t.Fatalf("expected generated artifact hashes, got %#v", report.Chapters[0].NotebookReports[0].GeneratedArtifacts)
	}
	if stat, err := os.Stat(filepath.Join(out, "open-textbook-companion.md")); err != nil || stat.Size() == 0 {
		t.Fatalf("expected open-textbook-companion.md to be written, stat=%#v err=%v", stat, err)
	}
}

func mainTextbookChapter(id, title, audience, notebookPath, command, docPath, specPath, jsonOut, mdOut string) education.TextbookChapter {
	return education.TextbookChapter{
		ID:       id,
		Title:    title,
		Audience: audience,
		Summary:  "Executable notebook chapter for " + title + ".",
		Concepts: []string{"regeneration", "evidence hashing"},
		Notebooks: []education.TextbookNotebook{{
			ID:              id + "-notebook",
			Title:           title + " notebook",
			Path:            notebookPath,
			Runtime:         "python3",
			ExecuteCommands: []string{command},
			TeachingExamples: []education.TextbookTeachingExample{{
				ID:                 id + "-example",
				Title:              title + " example",
				SourceCommand:      command,
				LearningObjectives: []string{"run the exact regeneration command", "inspect the generated evidence hashes"},
				EvidencePaths:      []string{docPath, specPath},
				ExpectedArtifacts:  []string{jsonOut, mdOut},
			}},
			EvidencePaths:     []string{docPath, specPath},
			ExpectedArtifacts: []string{jsonOut, mdOut},
			NegativeControls: []education.TextbookNegativeControl{{
				ID:                     "remove-command",
				Mutation:               "delete the regeneration command from the notebook",
				ExpectedCounterexample: "missing_executable_cell",
			}},
		}},
	}
}

func mainTextbookNotebookJSON(command string) string {
	return `{
  "cells": [
    {
      "cell_type": "code",
      "execution_count": null,
      "metadata": {},
      "outputs": [],
      "source": ["import subprocess\n", "subprocess.run(\"` + command + `\", shell=True, check=True)\n"]
    }
  ],
  "metadata": {"kernelspec": {"display_name": "Python 3", "language": "python", "name": "python3"}},
  "nbformat": 4,
  "nbformat_minor": 5
}`
}

func TestReviewerFairnessAuditCommandWritesReports(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "docs/acceptance-study.md", "paired reviews with adjudicated false positives\n")
	writeMainTestFile(t, root, "docs/escalation-log.md", "owner-routed escalation handoffs\n")
	specPath := filepath.Join(root, "reviewer-fairness-audit.json")
	writeMainTestFile(t, root, "reviewer-fairness-audit.json", `{
  "version": "patchline.reviewer-fairness-audit/v1",
  "name": "main test reviewer fairness audit",
  "criteria": {
    "min_teams": 2,
    "min_ecosystems": 2,
    "min_reviews_per_group": 2,
    "max_burden_ratio": 1.2,
    "max_false_positive_rate_gap": 0.3,
    "max_escalation_rate_gap": 0.2
  },
  "observations": [
    {"review_id":"payments-rails","reviewer_id":"r-pay-1","team":"payments","ecosystem":"rails","review_minutes":30,"findings_reported":4,"false_positives":1,"escalated":true,"evidence_paths":["docs/acceptance-study.md","docs/escalation-log.md"]},
    {"review_id":"payments-django","reviewer_id":"r-pay-2","team":"payments","ecosystem":"django","review_minutes":28,"findings_reported":3,"false_positives":0,"escalated":false,"evidence_paths":["docs/acceptance-study.md"]},
    {"review_id":"platform-rails","reviewer_id":"r-plat-1","team":"platform","ecosystem":"rails","review_minutes":31,"findings_reported":4,"false_positives":1,"escalated":false,"evidence_paths":["docs/acceptance-study.md","docs/escalation-log.md"]},
    {"review_id":"platform-django","reviewer_id":"r-plat-2","team":"platform","ecosystem":"django","review_minutes":29,"findings_reported":3,"false_positives":0,"escalated":true,"evidence_paths":["docs/escalation-log.md"]}
  ]
}`)
	out := filepath.Join(t.TempDir(), "reviewer-fairness-audit")
	if err := run([]string{"reviewer-fairness-audit", "--spec", specPath, "--root", root, "--out", out, "--json"}); err != nil {
		t.Fatalf("reviewer-fairness-audit failed: %v", err)
	}
	var report reviewerfairness.Report
	readMainTestJSON(t, filepath.Join(out, "reviewer-fairness-audit.json"), &report)
	if !report.OK || report.Summary.Reviews != 4 || report.Summary.TeamEscalationRateGap != 0 {
		t.Fatalf("unexpected reviewer fairness report: %#v", report)
	}
	if len(report.Teams) != 2 || len(report.Teams[0].Evidence) == 0 || report.Teams[0].Evidence[0].SHA256 == "" {
		t.Fatalf("expected grouped evidence hashes, got %#v", report.Teams)
	}
	if stat, err := os.Stat(filepath.Join(out, "reviewer-fairness-audit.md")); err != nil || stat.Size() == 0 {
		t.Fatalf("expected reviewer-fairness-audit.md to be written, stat=%#v err=%v", stat, err)
	}
}

func TestExplainabilityAuditCommandWritesReports(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "docs/claims-evidence.md", "claims ledger maps README and paper statements to generated artifacts\n")
	writeMainTestFile(t, root, "docs/reviewer-walkthrough.md", "reviewer walkthrough regenerates reports, tables, and artifact bundles\n")
	specPath := filepath.Join(root, "explainability-audit.json")
	writeMainTestFile(t, root, "explainability-audit.json", `{
  "version": "patchline.explainability-audit/v1",
  "name": "main test explainability audit",
  "criteria": {
    "min_reviewers": 2,
    "min_verdicts": 2,
    "min_reviews_per_verdict": 2,
    "min_agreement_rate": 1,
    "min_supported_rate": 1,
    "max_unsupported_rate": 0,
    "require_independent_reviewers": true
  },
  "reviews": [
    {"review_id":"claims-a","verdict_id":"claims-evidence","reviewer_id":"reviewer-a","reviewer_role":"artifact reviewer","independent":true,"stated_verdict":"claims map to evidence","judgment":"supported","evidence_paths":["docs/claims-evidence.md"],"rationale":"the claim ledger names the concrete artifacts"},
    {"review_id":"claims-b","verdict_id":"claims-evidence","reviewer_id":"reviewer-b","reviewer_role":"maintainer reviewer","independent":true,"stated_verdict":"claims map to evidence","judgment":"supported","evidence_paths":["docs/claims-evidence.md"],"rationale":"the evidence trail is explicit and reproducible"},
    {"review_id":"walkthrough-a","verdict_id":"reviewer-walkthrough","reviewer_id":"reviewer-a","reviewer_role":"artifact reviewer","independent":true,"stated_verdict":"walkthrough is reproducible","judgment":"supported","evidence_paths":["docs/reviewer-walkthrough.md"],"rationale":"the walkthrough gives commands and expected outputs"},
    {"review_id":"walkthrough-b","verdict_id":"reviewer-walkthrough","reviewer_id":"reviewer-b","reviewer_role":"maintainer reviewer","independent":true,"stated_verdict":"walkthrough is reproducible","judgment":"supported","evidence_paths":["docs/reviewer-walkthrough.md"],"rationale":"the reviewer path cites generated reports"}
  ]
}`)
	out := filepath.Join(t.TempDir(), "explainability-audit")
	if err := run([]string{"explainability-audit", "--spec", specPath, "--root", root, "--out", out, "--json"}); err != nil {
		t.Fatalf("explainability-audit failed: %v", err)
	}
	var report explainabilityaudit.Report
	readMainTestJSON(t, filepath.Join(out, "explainability-audit.json"), &report)
	if !report.OK || report.Summary.Reviews != 4 || report.Summary.Verdicts != 2 || report.Summary.SupportedRate != 1 {
		t.Fatalf("unexpected explainability audit report: %#v", report)
	}
	if len(report.Verdicts) != 2 || len(report.Verdicts[0].Evidence) == 0 || report.Verdicts[0].Evidence[0].SHA256 == "" {
		t.Fatalf("expected per-verdict evidence hashes, got %#v", report.Verdicts)
	}
	if stat, err := os.Stat(filepath.Join(out, "explainability-audit.md")); err != nil || stat.Size() == 0 {
		t.Fatalf("expected explainability-audit.md to be written, stat=%#v err=%v", stat, err)
	}
}

func TestChangeManagementVerifyCommandWritesReports(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "evidence/gate-report.json", `{"version":"patchline.gate-report/v1","gate_id":"patchline-reviewer-fairness","status":"pass","finding_ids":["PL-DB-001"],"checked_at":"2026-01-15T12:00:00Z"}`+"\n")
	writeMainTestFile(t, root, "evidence/approval-db.txt", "database reliability approval reviewed the passed Patchline gate\n")
	writeMainTestFile(t, root, "evidence/approval-svc.txt", "service owner approval reviewed the same gate and rollback plan\n")
	writeMainTestFile(t, root, "evidence/rollback.md", "rollback plan restores the pre-change snapshot\n")
	writeMainTestFile(t, root, "evidence/change-ticket.md", "CHG-2026-001 binds Patchline gate evidence to CAB approval\n")
	gateHash := mainTestFileHash(t, filepath.Join(root, "evidence/gate-report.json"))
	specPath := filepath.Join(root, "change-management.json")
	writeMainTestFile(t, root, "change-management.json", fmt.Sprintf(`{
  "version": "patchline.change-management/v1",
  "name": "main test change-management integration",
  "criteria": {
    "min_approval_steps": 2,
    "require_distinct_approvers": true,
    "require_evidence_hashes": true,
    "require_patchline_gate_binding": true,
    "require_change_ticket": true,
    "require_rollback_plan": true,
    "require_emergency_expiry": true,
    "require_workflow_evidence_paths": true
  },
  "workflows": [
    {
      "workflow_id": "chg-2026-001-expand-contract",
      "title": "Customers expand-contract migration",
      "change_ticket": "CHG-2026-001",
      "risk_level": "high",
      "patchline_findings": ["PL-DB-001"],
      "gates": [
        {"gate_id":"patchline-reviewer-fairness","command":"make reviewer-fairness-audit-gate","status":"pass","report_path":"evidence/gate-report.json","report_sha256":"%s","blocks_change":true}
      ],
      "approvals": [
        {"step_id":"database-review","role":"database reliability","approver":"robin-db","approved_at":"2026-01-15T13:00:00Z","decision":"approved","evidence_path":"evidence/approval-db.txt","gate_ids":["patchline-reviewer-fairness"]},
        {"step_id":"service-owner","role":"service owner","approver":"sam-service","approved_at":"2026-01-15T13:10:00Z","decision":"approved","evidence_path":"evidence/approval-svc.txt","gate_ids":["patchline-reviewer-fairness"]}
      ],
      "deployment_controls": {"change_window":"2026-01-15T22:00:00Z/2026-01-15T23:00:00Z","rollback_plan_path":"evidence/rollback.md"},
      "evidence_paths": ["evidence/change-ticket.md"]
    }
  ]
}`, gateHash))
	out := filepath.Join(t.TempDir(), "change-management")
	if err := run([]string{"change-management-verify", "--spec", specPath, "--root", root, "--out", out, "--json"}); err != nil {
		t.Fatalf("change-management-verify failed: %v", err)
	}
	var report changemanagement.Report
	readMainTestJSON(t, filepath.Join(out, "change-management.json"), &report)
	if !report.OK || report.Summary.Workflows != 1 || report.Summary.PassedBlockingGates != 1 || report.Summary.ApprovedSteps != 2 {
		t.Fatalf("unexpected change-management report: %#v", report)
	}
	if report.Workflows[0].Gates[0].Report == nil || !report.Workflows[0].Gates[0].HashMatches {
		t.Fatalf("expected hashed gate report evidence, got %#v", report.Workflows[0].Gates[0])
	}
	if stat, err := os.Stat(filepath.Join(out, "change-management.md")); err != nil || stat.Size() == 0 {
		t.Fatalf("expected change-management.md to be written, stat=%#v err=%v", stat, err)
	}

	badSpecPath := filepath.Join(root, "change-management.bad.json")
	writeMainTestFile(t, root, "change-management.bad.json", strings.ReplaceAll(mustReadMainTestFile(t, specPath), `"gate_ids":["patchline-reviewer-fairness"]`, `"gate_ids":["missing-gate"]`))
	badOut := filepath.Join(t.TempDir(), "bad-change-management")
	if err := run([]string{"change-management-verify", "--spec", badSpecPath, "--root", root, "--out", badOut, "--json"}); err != nil {
		t.Fatalf("change-management-verify negative control should write an ok=false report, got %v", err)
	}
	var rejected changemanagement.Report
	readMainTestJSON(t, filepath.Join(badOut, "change-management.json"), &rejected)
	if rejected.OK || rejected.Summary.Counterexamples == 0 {
		t.Fatalf("expected rejected change-management report, got %#v", rejected)
	}
}

func TestCertCommandsNormalizeAndDiffRealFixtures(t *testing.T) {
	root := mainTestRepoRoot(t)
	certPath := filepath.Join(root, "specs/certificate-interchange/v1/vectors/valid/patchline-proof-frame.plci")
	outPath := filepath.Join(t.TempDir(), "normalized.plci")
	if err := run([]string{"cert", "normalize", certPath, "--root", root, "--out", outPath, "--json"}); err != nil {
		t.Fatalf("cert normalize failed: %v", err)
	}
	if stat, err := os.Stat(outPath); err != nil || stat.Size() == 0 {
		t.Fatalf("expected normalized certificate output, stat=%#v err=%v", stat, err)
	}
	weakenedData, err := os.ReadFile(certPath)
	if err != nil {
		t.Fatal(err)
	}
	weakened := strings.Replace(string(weakenedData), "obl.frame kind=frame status=checked", "obl.frame kind=frame status=assumed", 1)
	weakened = recomputeMainTestCanonicalHash(weakened)
	weakenedPath := filepath.Join(t.TempDir(), "weakened.plci")
	if err := os.WriteFile(weakenedPath, []byte(weakened), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"cert", "diff", certPath, weakenedPath, "--root", root, "--json"}); err != nil {
		t.Fatalf("cert diff failed: %v", err)
	}
}

func TestRepoMetricsWritesPrivacyPreservingAggregates(t *testing.T) {
	rootA := t.TempDir()
	writeMainTestFile(t, rootA, "db/migrate/001_backfill.sql", "UPDATE accounts SET status = 'active';\n")
	outA := filepath.Join(t.TempDir(), "analysis-a")
	if err := run([]string{"repo", "analyze", rootA, "--stages", "inventory,baseline,propose,compare", "--proposal-kind", "all", "--budget", "files=2,lines=60,tokens=4000,changes=1", "--no-llm", "--out", outA, "--json"}); err != nil {
		t.Fatalf("first repo analyze failed: %v", err)
	}

	rootB := t.TempDir()
	writeMainTestFile(t, rootB, "db/migrate/001_backfill.sql", "UPDATE accounts SET status = 'active';\n")
	writeMainTestFile(t, rootB, "db/migrate/002_delete.sql", "DELETE FROM account_events;\n")
	outB := filepath.Join(t.TempDir(), "analysis-b")
	if err := run([]string{"repo", "analyze", rootB, "--stages", "inventory,baseline,propose,compare", "--proposal-kind", "all", "--budget", "files=3,lines=60,tokens=4000,changes=2", "--no-llm", "--out", outB, "--json"}); err != nil {
		t.Fatalf("second repo analyze failed: %v", err)
	}

	metricsOut := filepath.Join(t.TempDir(), "metrics")
	if err := run([]string{"repo", "metrics", "--analyses", outA + "," + outB, "--salt", "team-local-salt", "--out", metricsOut, "--json"}); err != nil {
		t.Fatalf("repo metrics failed: %v", err)
	}
	var report repoMetricsReport
	readMainTestJSON(t, filepath.Join(metricsOut, "metrics.json"), &report)
	if report.Version != "patchline.repo-metrics/v1" || !report.Shareable || !report.Privacy.SourceFree || !report.Privacy.RawEvidenceFree || !report.Privacy.PathFree {
		t.Fatalf("expected shareable privacy-preserving metrics, got %#v", report)
	}
	if report.Summary.Analyses != 2 || len(report.Analyses) != 2 || len(report.TrendDeltas) != 1 {
		t.Fatalf("expected two analyses and one delta, got summary=%#v analyses=%d deltas=%d", report.Summary, len(report.Analyses), len(report.TrendDeltas))
	}
	if report.Analyses[0].CohortID == report.Analyses[1].CohortID || strings.Contains(report.Analyses[0].CohortID, rootA) || strings.Contains(report.Analyses[1].CohortID, rootB) {
		t.Fatalf("expected salted opaque cohort IDs, got %#v", report.Analyses)
	}
	data, err := os.ReadFile(filepath.Join(metricsOut, "metrics.json"))
	if err != nil {
		t.Fatal(err)
	}
	serialized := string(data)
	for _, forbidden := range []string{rootA, rootB, "db/migrate", "UPDATE accounts", "DELETE FROM", "001_backfill.sql", "002_delete.sql"} {
		if strings.Contains(serialized, forbidden) {
			t.Fatalf("metrics leaked forbidden raw/source value %q:\n%s", forbidden, serialized)
		}
	}
	markdown, err := os.ReadFile(filepath.Join(metricsOut, "metrics.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(markdown), "privacy-preserving aggregate metrics") || !strings.Contains(string(markdown), "Suppressed fields") {
		t.Fatalf("expected metrics markdown privacy summary:\n%s", string(markdown))
	}
}

func TestFeedbackThresholdUpdateRequiresBoundGateForCandidatePolicy(t *testing.T) {
	root := t.TempDir()
	inputPath := filepath.Join(root, "feedback-input.json")
	writeMainTestFile(t, root, "feedback-input.json", `{
  "version": "patchline.live-feedback-ingestion/v1",
  "adopter_id": "team-gamma",
  "salt": "threshold-update-secret-salt",
  "min_group_size": 3,
  "outcomes": [
    {"finding_id":"finding-001","detector":"orm.write-breadth","release":"v1.0.0","confidence":0.73,"verdict":"false_positive","action":"dismissed","burden_minutes":4,"evidence_hash":"ev-001","reviewer_role":"maintainer"},
    {"finding_id":"finding-002","detector":"orm.write-breadth","release":"v1.0.0","confidence":0.76,"verdict":"false_positive","action":"dismissed","burden_minutes":4,"evidence_hash":"ev-002","reviewer_role":"dba"},
    {"finding_id":"finding-003","detector":"orm.write-breadth","release":"v1.0.0","confidence":0.78,"verdict":"false_positive","action":"dismissed","burden_minutes":4,"evidence_hash":"ev-003","reviewer_role":"sre"},
    {"finding_id":"finding-004","detector":"sql.destructive-ddl","release":"v1.0.0","confidence":0.93,"verdict":"confirmed","action":"blocked","burden_minutes":9,"evidence_hash":"ev-004","reviewer_role":"maintainer"},
    {"finding_id":"finding-005","detector":"sql.destructive-ddl","release":"v1.0.0","confidence":0.91,"verdict":"confirmed","action":"blocked","burden_minutes":9,"evidence_hash":"ev-005","reviewer_role":"dba"},
    {"finding_id":"finding-006","detector":"sql.destructive-ddl","release":"v1.0.0","confidence":0.95,"verdict":"confirmed","action":"blocked","burden_minutes":9,"evidence_hash":"ev-006","reviewer_role":"sre"}
  ]
}`)
	ingestOut := filepath.Join(t.TempDir(), "feedback")
	if err := run([]string{"feedback", "ingest", inputPath, "--out", ingestOut, "--json"}); err != nil {
		t.Fatalf("feedback ingest failed: %v", err)
	}

	policyPath := filepath.Join(root, "threshold-policy.json")
	writeMainTestFile(t, root, "threshold-policy.json", `{
  "version": "patchline.threshold-policy/v1",
  "name": "stage63-policy",
  "thresholds": [
    {"detector":"orm.write-breadth","blocking_threshold":0.70},
    {"detector":"sql.destructive-ddl","blocking_threshold":0.90}
  ]
}`)
	policyBefore, err := os.ReadFile(policyPath)
	if err != nil {
		t.Fatal(err)
	}

	advisoryOut := filepath.Join(t.TempDir(), "threshold-advisory")
	if err := run([]string{"feedback", "threshold-update", "--feedback", filepath.Join(ingestOut, "live-feedback.json"), "--policy", policyPath, "--out", advisoryOut, "--json"}); err != nil {
		t.Fatalf("threshold update failed: %v", err)
	}
	var advisory feedback.ThresholdUpdateReport
	readMainTestJSON(t, filepath.Join(advisoryOut, "threshold-update.json"), &advisory)
	if advisory.PolicyChangeAllowed || advisory.CandidatePolicy != nil || advisory.BlockingPolicyChanged {
		t.Fatalf("expected advisory-only update without gate: %#v", advisory)
	}
	if _, err := os.Stat(filepath.Join(advisoryOut, "candidate-threshold-policy.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("candidate policy should be absent without gate, stat err=%v", err)
	}

	staleGatePath := filepath.Join(root, "stale-gate.json")
	writeMainTestFile(t, root, "stale-gate.json", fmt.Sprintf(`{
  "version": "patchline.threshold-policy-gate/v1",
  "gate": "drift-threshold-update-gate",
  "ok": true,
  "policy_hash": "stale-policy",
  "feedback_hash": %q,
  "allows_blocking_policy_change": true
}`, advisory.FeedbackHash))
	staleOut := filepath.Join(t.TempDir(), "threshold-stale")
	if err := run([]string{"feedback", "threshold-update", "--feedback", filepath.Join(ingestOut, "live-feedback.json"), "--policy", policyPath, "--gate", staleGatePath, "--out", staleOut, "--json"}); err != nil {
		t.Fatalf("threshold update with stale gate failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(staleOut, "candidate-threshold-policy.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("candidate policy should be absent with stale gate, stat err=%v", err)
	}

	validGatePath := filepath.Join(root, "valid-gate.json")
	writeMainTestFile(t, root, "valid-gate.json", fmt.Sprintf(`{
  "version": "patchline.threshold-policy-gate/v1",
  "gate": "drift-threshold-update-gate",
  "ok": true,
  "policy_hash": %q,
  "feedback_hash": %q,
  "allows_blocking_policy_change": true
}`, advisory.PolicyHash, advisory.FeedbackHash))
	gatedOut := filepath.Join(t.TempDir(), "threshold-gated")
	if err := run([]string{"feedback", "threshold-update", "--feedback", filepath.Join(ingestOut, "live-feedback.json"), "--policy", policyPath, "--gate", validGatePath, "--out", gatedOut, "--json"}); err != nil {
		t.Fatalf("threshold update with valid gate failed: %v", err)
	}
	var gated feedback.ThresholdUpdateReport
	readMainTestJSON(t, filepath.Join(gatedOut, "threshold-update.json"), &gated)
	if !gated.PolicyChangeAllowed || gated.CandidatePolicy == nil || gated.BlockingPolicyChanged {
		t.Fatalf("expected separate candidate policy under valid gate: %#v", gated)
	}
	if _, err := os.Stat(filepath.Join(gatedOut, "candidate-threshold-policy.json")); err != nil {
		t.Fatalf("expected candidate policy file: %v", err)
	}
	policyAfter, err := os.ReadFile(policyPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(policyBefore) != string(policyAfter) {
		t.Fatalf("threshold update mutated input policy:\n%s\n---\n%s", policyBefore, policyAfter)
	}
}

func TestFeedbackCounterfactualLogWritesPreviousReleaseRecommendations(t *testing.T) {
	root := t.TempDir()
	inputPath := filepath.Join(root, "feedback-input.json")
	writeMainTestFile(t, root, "feedback-input.json", `{
  "version": "patchline.live-feedback-ingestion/v1",
  "adopter_id": "team-delta",
  "salt": "counterfactual-secret-salt",
  "min_group_size": 3,
  "outcomes": [
    {"finding_id":"finding-001","detector":"orm.write-breadth","release":"v1.0.0","confidence":0.73,"verdict":"false_positive","action":"dismissed","burden_minutes":4,"evidence_hash":"ev-001","reviewer_role":"maintainer"},
    {"finding_id":"finding-002","detector":"orm.write-breadth","release":"v1.0.0","confidence":0.76,"verdict":"false_positive","action":"dismissed","burden_minutes":4,"evidence_hash":"ev-002","reviewer_role":"dba"},
    {"finding_id":"finding-003","detector":"orm.write-breadth","release":"v1.0.0","confidence":0.78,"verdict":"false_positive","action":"dismissed","burden_minutes":4,"evidence_hash":"ev-003","reviewer_role":"sre"},
    {"finding_id":"finding-004","detector":"sql.destructive-ddl","release":"v1.0.0","confidence":0.93,"verdict":"confirmed","action":"blocked","burden_minutes":9,"evidence_hash":"ev-004","reviewer_role":"maintainer"},
    {"finding_id":"finding-005","detector":"sql.destructive-ddl","release":"v1.0.0","confidence":0.91,"verdict":"confirmed","action":"blocked","burden_minutes":9,"evidence_hash":"ev-005","reviewer_role":"dba"},
    {"finding_id":"finding-006","detector":"sql.destructive-ddl","release":"v1.0.0","confidence":0.95,"verdict":"confirmed","action":"blocked","burden_minutes":9,"evidence_hash":"ev-006","reviewer_role":"sre"}
  ]
}`)
	ingestOut := filepath.Join(t.TempDir(), "feedback")
	if err := run([]string{"feedback", "ingest", inputPath, "--out", ingestOut, "--json"}); err != nil {
		t.Fatalf("feedback ingest failed: %v", err)
	}

	historyPath := filepath.Join(root, "counterfactual-history.json")
	writeMainTestFile(t, root, "counterfactual-history.json", `{
  "version": "patchline.counterfactual-policy-history/v1",
  "name": "stage63-counterfactual-history",
  "policies": [
    {
      "release": "v0.8.0",
      "thresholds": [
        {"detector":"orm.write-breadth","blocking_threshold":0.80},
        {"detector":"sql.destructive-ddl","blocking_threshold":0.99}
      ]
    },
    {
      "release": "v0.9.0",
      "thresholds": [
        {"detector":"orm.write-breadth","blocking_threshold":0.70},
        {"detector":"sql.destructive-ddl","blocking_threshold":0.90}
      ]
    },
    {
      "release": "v1.0.0",
      "thresholds": [
        {"detector":"orm.write-breadth","blocking_threshold":0.70},
        {"detector":"sql.destructive-ddl","blocking_threshold":0.90}
      ]
    }
  ]
}`)
	out := filepath.Join(t.TempDir(), "counterfactual")
	if err := run([]string{"feedback", "counterfactual-log", "--feedback", filepath.Join(ingestOut, "live-feedback.json"), "--history", historyPath, "--out", out, "--json"}); err != nil {
		t.Fatalf("counterfactual log failed: %v", err)
	}
	var log feedback.CounterfactualLog
	readMainTestJSON(t, filepath.Join(out, "counterfactual-log.json"), &log)
	if log.Version != feedback.CounterfactualLogVersion || !log.OK || log.Summary.CounterfactualGroupsCompared != 4 || log.Summary.ComparedRecords != 12 {
		t.Fatalf("unexpected counterfactual log: %#v", log.Summary)
	}
	if log.Summary.ConfirmedWouldBlock != 3 || log.Summary.FalsePositiveWouldBlock != 3 || log.Summary.FalsePositiveWouldSpare != 3 || log.Summary.ConfirmedBoundaryAmbiguous != 3 {
		t.Fatalf("unexpected counterfactual recommendation counts: %#v", log.Summary.CounterfactualCounters)
	}
	if _, err := os.Stat(filepath.Join(out, "counterfactual-log.md")); err != nil {
		t.Fatalf("expected counterfactual markdown: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(out, "counterfactual-log.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"finding-001", "ev-001", "counterfactual-secret-salt", "source_code", "evidence_hash"} {
		if strings.Contains(string(data), forbidden) {
			t.Fatalf("counterfactual CLI output leaked %q:\n%s", forbidden, data)
		}
	}
}

func TestGovernanceRiskRegisterCommandWritesReports(t *testing.T) {
	root := t.TempDir()
	for rel, contents := range map[string]string{
		"evidence/governance.md":     "governance charters, maintainer rotation, and escalation logs\n",
		"evidence/funding.md":        "funding commitments, reserve controls, and sponsor concentration caps\n",
		"evidence/infrastructure.md": "release CI, docs mirrors, registry ownership, and recovery drills\n",
		"evidence/benchmark.md":      "benchmark curation, challenge scoring, and release freeze governance\n",
	} {
		writeMainTestFile(t, root, rel, contents)
	}
	specPath := filepath.Join(root, "governance-risk-register.json")
	writeMainTestFile(t, root, "governance-risk-register.json", `{
  "version": "patchline.governance-risk-register/v1",
  "name": "main test governance-risk register",
  "as_of_date": "2026-03-01T00:00:00Z",
  "criteria": {
    "required_domains": ["maintainership", "funding", "infrastructure", "benchmark_control"],
    "max_owner_share": 0.6,
    "max_organization_share": 0.65,
    "min_independent_owners_per_domain": 2,
    "min_independent_orgs_per_domain": 2,
    "min_mitigations_per_high_risk_domain": 2,
    "require_evidence_paths": true,
    "require_rotation_plan": true,
    "review_cadence_days": 120
  },
  "entries": [
    {"asset_id":"maint-release","domain":"maintainership","asset_name":"Release approvals","owner":"release council","organization":"Patchline Maintainers","control_type":"merge authority","weight":55,"last_reviewed":"2026-02-01T00:00:00Z","rotation_plan":"quarterly release captain rotation","mitigations":["named backup","public review log"],"evidence_paths":["evidence/governance.md"]},
    {"asset_id":"maint-security","domain":"maintainership","asset_name":"Security triage","owner":"security reviewers","organization":"Independent Security WG","control_type":"security escalation","weight":45,"last_reviewed":"2026-02-01T00:00:00Z","rotation_plan":"backup reviewer each drill","mitigations":["named backup","public review log"],"evidence_paths":["evidence/governance.md"]},
    {"asset_id":"fund-grant","domain":"funding","asset_name":"Public-good grant","owner":"grant committee","organization":"Research Commons Fund","control_type":"grant control","weight":55,"last_reviewed":"2026-02-01T00:00:00Z","rotation_plan":"annual independent renewal","mitigations":["sponsor cap","reserve signer split"],"evidence_paths":["evidence/funding.md"]},
    {"asset_id":"fund-reserve","domain":"funding","asset_name":"Reserve budget","owner":"treasury signers","organization":"Patchline Foundation","control_type":"reserve control","weight":45,"last_reviewed":"2026-02-01T00:00:00Z","rotation_plan":"two backup signers per withdrawal","mitigations":["sponsor cap","reserve signer split"],"evidence_paths":["evidence/funding.md"]},
    {"asset_id":"infra-ci","domain":"infrastructure","asset_name":"Release CI","owner":"ci rotation","organization":"GitHub Actions Maintainers","control_type":"runner administration","weight":55,"last_reviewed":"2026-02-01T00:00:00Z","rotation_plan":"mirror release jobs before cutover","mitigations":["runner backup","artifact mirror"],"evidence_paths":["evidence/infrastructure.md"]},
    {"asset_id":"infra-mirror","domain":"infrastructure","asset_name":"Docs mirror","owner":"mirror stewards","organization":"University Mirror Network","control_type":"mirror administration","weight":45,"last_reviewed":"2026-02-01T00:00:00Z","rotation_plan":"quarterly mirror succession test","mitigations":["runner backup","artifact mirror"],"evidence_paths":["evidence/infrastructure.md"]},
    {"asset_id":"bench-corpus","domain":"benchmark_control","asset_name":"Corpus curation","owner":"corpus board","organization":"Benchmark Working Group","control_type":"case admission","weight":55,"last_reviewed":"2026-02-01T00:00:00Z","rotation_plan":"quorum excludes submitters","mitigations":["external dispute path","release freeze"],"evidence_paths":["evidence/benchmark.md"]},
    {"asset_id":"bench-scoring","domain":"benchmark_control","asset_name":"Scoring rules","owner":"scorecard maintainers","organization":"External Replication Lab","control_type":"scorecard control","weight":45,"last_reviewed":"2026-02-01T00:00:00Z","rotation_plan":"scoring changes require release-candidate freeze","mitigations":["external dispute path","release freeze"],"evidence_paths":["evidence/benchmark.md"]}
  ]
}`)
	out := filepath.Join(t.TempDir(), "governance-risk-register")
	if err := run([]string{"governance-risk-register", "--spec", specPath, "--root", root, "--out", out, "--json"}); err != nil {
		t.Fatalf("governance-risk-register failed: %v", err)
	}
	var report governancerisk.Report
	readMainTestJSON(t, filepath.Join(out, "governance-risk-register.json"), &report)
	if !report.OK || report.Summary.Domains != 4 || report.Summary.HighRiskDomains != 0 || report.Summary.MaxOwnerShare > 0.6 {
		t.Fatalf("unexpected governance-risk report: %#v", report)
	}
	if len(report.Domains) != 4 || len(report.Domains[0].Evidence) == 0 || report.Domains[0].Evidence[0].SHA256 == "" {
		t.Fatalf("expected domain evidence hashes, got %#v", report.Domains)
	}
	if stat, err := os.Stat(filepath.Join(out, "governance-risk-register.md")); err != nil || stat.Size() == 0 {
		t.Fatalf("expected governance-risk-register.md to be written, stat=%#v err=%v", stat, err)
	}
}

func TestEthicsReviewTemplateCommandWritesReports(t *testing.T) {
	root := t.TempDir()
	for rel, contents := range map[string]string{
		"evidence/data-source.md":   "source license review, consent basis, minimization, retention, and withdrawal path\n",
		"evidence/live-feedback.md": "local feedback loop with human oversight and no source-code collection\n",
		"evidence/outcome-study.md": "outcome study preregistration, reviewer burden protocol, and evidence hashes\n",
	} {
		writeMainTestFile(t, root, rel, contents)
	}
	specPath := filepath.Join(root, "ethics-review-template.json")
	writeMainTestFile(t, root, "ethics-review-template.json", `{
  "version": "patchline.ethics-review-template/v1",
  "name": "main test ethics review template",
  "as_of_date": "2026-03-01T00:00:00Z",
  "criteria": {
    "required_review_areas": ["new_data_source", "live_feedback_loop", "adopter_outcome_study"],
    "min_independent_reviewers": 2,
    "max_risk_score": 0.7,
    "review_cadence_days": 120,
    "require_consent_basis": true,
    "require_privacy_review": true,
    "require_retention_policy": true,
    "require_minimization": true,
    "require_withdrawal_path": true,
    "require_security_owner": true,
    "require_evidence_paths": true,
    "require_human_oversight_for_feedback": true,
    "require_preregistration_for_outcome_studies": true,
    "min_mitigations_per_high_risk_entry": 2
  },
  "entries": [
    {"review_id":"data-source-public-incidents","area":"new_data_source","title":"Public incident corpus","owner":"corpus steward","data_sources":["public incident notes"],"purpose":"Expand public data-change hazard evidence without collecting private source.","risk_score":0.45,"last_reviewed":"2026-02-01T00:00:00Z","reviewer_roles":["ethics reviewer","security reviewer"],"consent_basis":"public-license review and consent check","privacy_review":"approved aggregate-only release","retention_policy":"raw notes expire after 90 days","minimization":"hashes and bucketed counts only","withdrawal_path":"submitter withdrawal before release","security_owner":"security-oncall","mitigations":["license audit","withdrawal audit"],"evidence_paths":["evidence/data-source.md"]},
    {"review_id":"live-feedback-calibration","area":"live_feedback_loop","title":"Adopter-local calibration","owner":"learning steward","data_sources":["source-free adopter outcomes"],"purpose":"Monitor calibration drift without collecting source code.","risk_score":0.55,"last_reviewed":"2026-02-01T00:00:00Z","reviewer_roles":["ethics reviewer","maintainer"],"consent_basis":"adopter opt-in for aggregate metrics","privacy_review":"k-anonymous cohort export only","retention_policy":"operational feedback expires after 120 days","minimization":"confidence deciles and outcomes only","withdrawal_path":"adopter can pause export immediately","security_owner":"feedback-security-owner","human_oversight":"release council approves blocking policy changes","mitigations":["shadow mode","human approval"],"evidence_paths":["evidence/live-feedback.md"]},
    {"review_id":"adopter-review-time-study","area":"adopter_outcome_study","title":"Review-time outcome study","owner":"study steward","data_sources":["paired reviewer timing aggregates"],"purpose":"Measure whether generated plans reduce review time without hiding uncertainty.","risk_score":0.65,"last_reviewed":"2026-02-01T00:00:00Z","reviewer_roles":["ethics reviewer","methods reviewer"],"consent_basis":"study participant opt-in and public aggregate release","privacy_review":"no individual reviewer identifiers in release","retention_policy":"raw timing logs expire after analysis close","minimization":"paired deltas and confidence intervals only","withdrawal_path":"participant withdrawal before aggregate lock","security_owner":"study-security-owner","preregistration":"registered-report-2026-02","mitigations":["preregistered analysis","independent methods review"],"evidence_paths":["evidence/outcome-study.md"]}
  ]
}`)
	out := filepath.Join(t.TempDir(), "ethics-review-template")
	if err := run([]string{"ethics-review-template", "--spec", specPath, "--root", root, "--out", out, "--json"}); err != nil {
		t.Fatalf("ethics-review-template failed: %v", err)
	}
	var report ethicsreview.Report
	readMainTestJSON(t, filepath.Join(out, "ethics-review-template.json"), &report)
	if !report.OK || report.Summary.Areas != 3 || report.Summary.Entries != 3 || report.Summary.EvidenceFiles != 3 {
		t.Fatalf("unexpected ethics review template report: %#v", report)
	}
	if len(report.Areas) != 3 || len(report.Areas[0].Evidence) == 0 || report.Areas[0].Evidence[0].SHA256 == "" {
		t.Fatalf("expected area evidence hashes, got %#v", report.Areas)
	}
	if stat, err := os.Stat(filepath.Join(out, "ethics-review-template.md")); err != nil || stat.Size() == 0 {
		t.Fatalf("expected ethics-review-template.md to be written, stat=%#v err=%v", stat, err)
	}
}

func TestMisuseResistanceCommandWritesReports(t *testing.T) {
	root := t.TempDir()
	for rel, contents := range map[string]string{
		"evidence/certificates.md": "certificate normalization, proof obligations, verifier quorum, and negative controls\n",
		"evidence/scoreboard.md":   "scoreboard duplicate checks, signed reproduction logs, and disclosure holds\n",
		"evidence/adoption.md":     "signed aggregate adoption metrics, fairness audits, and citation-quality review\n",
		"evidence/governance.md":   "independent reviewer recusal and governance board minutes\n",
		"evidence/simulation.md":   "forged certificate, duplicate scoreboard, and inflated adoption negative controls\n",
	} {
		writeMainTestFile(t, root, rel, contents)
	}
	specPath := filepath.Join(root, "misuse-resistance.json")
	writeMainTestFile(t, root, "misuse-resistance.json", `{
  "version": "patchline.misuse-resistance/v1",
  "name": "main test misuse-resistance analysis",
  "as_of_date": "2026-03-01T00:00:00Z",
  "criteria": {
    "required_surfaces": ["certificates", "scoreboards", "adoption_metrics"],
    "min_independent_reviewers": 2,
    "min_controls_per_scenario": 3,
    "min_control_types_per_scenario": 3,
    "max_risk_score": 0.8,
    "review_cadence_days": 120,
    "require_evidence_paths": true,
    "require_simulation": true,
    "require_public_failure_mode": true,
    "require_control_owner": true,
    "require_passed_simulation": true
  },
  "scenarios": [
    {"scenario_id":"certificate-proof-stuffing","surface":"certificates","adversary":"malicious submitter","attack_goal":"smuggle assumed obligations into a passing certificate","attack_vectors":["stale hash replay","unchecked obligation"],"target_asset":"certificate verifier","public_failure_mode":"weakened certificate is accepted without independent proof","risk_score":0.7,"last_reviewed":"2026-02-01T00:00:00Z","reviewer_roles":["certificate reviewer","external verifier"],"controls":[{"control_id":"cert-hash","type":"hash_binding","description":"bind obligations to canonical hashes","owner":"verifier","evidence_paths":["evidence/certificates.md"]},{"control_id":"cert-review","type":"independent_review","description":"require independent conformance review","owner":"board","evidence_paths":["evidence/governance.md"]},{"control_id":"cert-negative","type":"negative_control","description":"keep weakened certificate fixtures failing","owner":"gate maintainer","evidence_paths":["evidence/simulation.md"]}],"simulations":[{"simulation_id":"cert-forgery","attempted_vector":"replace checked with assumed obligation","expected_outcome":"rejected before release","observed_outcome":"rejected by negative control","passed":true}]},
    {"scenario_id":"scoreboard-sybil-submission","surface":"scoreboards","adversary":"benchmark gamer","attack_goal":"inflate public rank with duplicate submissions","attack_vectors":["duplicate evidence","missing log"],"target_asset":"challenge scoreboard","public_failure_mode":"rank changes without reproducible logs or duplicate collapse","risk_score":0.72,"last_reviewed":"2026-02-01T00:00:00Z","reviewer_roles":["benchmark reviewer","replication reviewer"],"controls":[{"control_id":"score-dedup","type":"deduplication","description":"collapse duplicate submissions","owner":"curator","evidence_paths":["evidence/scoreboard.md"]},{"control_id":"score-log","type":"reproducibility_log","description":"require signed reproduction logs","owner":"challenge maintainer","evidence_paths":["evidence/scoreboard.md"]},{"control_id":"score-disclosure","type":"responsible_disclosure","description":"hold undisclosed examples","owner":"governance board","evidence_paths":["evidence/governance.md"]}],"simulations":[{"simulation_id":"score-duplicate","attempted_vector":"submit same hazard twice","expected_outcome":"duplicate rejected","observed_outcome":"duplicate rejected before publication","passed":true}]},
    {"scenario_id":"adoption-metric-inflation","surface":"adoption_metrics","adversary":"growth marketer","attack_goal":"overstate incident prevention with self reports","attack_vectors":["superficial citation","unsigned aggregate"],"target_asset":"adoption dashboard","public_failure_mode":"impact claims increase without signed aggregate and fairness evidence","risk_score":0.68,"last_reviewed":"2026-02-01T00:00:00Z","reviewer_roles":["methods reviewer","fairness reviewer"],"controls":[{"control_id":"adoption-aggregate","type":"signed_aggregate","description":"require signed source-free aggregates","owner":"adoption steward","evidence_paths":["evidence/adoption.md"]},{"control_id":"adoption-burden","type":"burden_audit","description":"audit reviewer burden parity","owner":"fairness maintainer","evidence_paths":["evidence/adoption.md"]},{"control_id":"adoption-citation","type":"citation_quality","description":"separate meaningful adoption from superficial mentions","owner":"impact reviewer","evidence_paths":["evidence/adoption.md","evidence/governance.md"]}],"simulations":[{"simulation_id":"adoption-forgery","attempted_vector":"report prevented incidents without signed aggregate","expected_outcome":"claim excluded","observed_outcome":"claim excluded before publication","passed":true}]}
  ]
}`)
	out := filepath.Join(t.TempDir(), "misuse-resistance")
	if err := run([]string{"misuse-resistance", "--spec", specPath, "--root", root, "--out", out, "--json"}); err != nil {
		t.Fatalf("misuse-resistance failed: %v", err)
	}
	var report misuseresistance.Report
	readMainTestJSON(t, filepath.Join(out, "misuse-resistance.json"), &report)
	if !report.OK || report.Summary.Surfaces != 3 || report.Summary.Scenarios != 3 || report.Summary.Controls != 9 || report.Summary.FailedSimulations != 0 {
		t.Fatalf("unexpected misuse-resistance report: %#v", report)
	}
	if len(report.Surfaces) != 3 || len(report.Surfaces[0].Evidence) == 0 || report.Surfaces[0].Evidence[0].SHA256 == "" {
		t.Fatalf("expected surface evidence hashes, got %#v", report.Surfaces)
	}
	if stat, err := os.Stat(filepath.Join(out, "misuse-resistance.md")); err != nil || stat.Size() == 0 {
		t.Fatalf("expected misuse-resistance.md to be written, stat=%#v err=%v", stat, err)
	}
}

func TestFeedbackLiveLearningCommandsWriteReports(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "feedback-input.json", `{
  "version": "patchline.live-feedback-ingestion/v1",
  "adopter_id": "team-omega",
  "salt": "live-learning-secret-salt",
  "min_group_size": 3,
  "outcomes": [
    {"finding_id":"finding-001","detector":"orm.write-breadth","release":"v1.0.0","confidence":0.73,"verdict":"false_positive","action":"dismissed","burden_minutes":4,"evidence_hash":"ev-001","reviewer_role":"maintainer"},
    {"finding_id":"finding-002","detector":"orm.write-breadth","release":"v1.0.0","confidence":0.76,"verdict":"false_positive","action":"dismissed","burden_minutes":4,"evidence_hash":"ev-002","reviewer_role":"dba"},
    {"finding_id":"finding-003","detector":"orm.write-breadth","release":"v1.0.0","confidence":0.78,"verdict":"false_positive","action":"dismissed","burden_minutes":4,"evidence_hash":"ev-003","reviewer_role":"sre"},
    {"finding_id":"finding-004","detector":"sql.destructive-ddl","release":"v1.0.0","confidence":0.93,"verdict":"confirmed","action":"blocked","burden_minutes":9,"evidence_hash":"ev-004","reviewer_role":"maintainer"},
    {"finding_id":"finding-005","detector":"sql.destructive-ddl","release":"v1.0.0","confidence":0.91,"verdict":"confirmed","action":"blocked","burden_minutes":9,"evidence_hash":"ev-005","reviewer_role":"dba"},
    {"finding_id":"finding-006","detector":"sql.destructive-ddl","release":"v1.0.0","confidence":0.95,"verdict":"confirmed","action":"blocked","burden_minutes":9,"evidence_hash":"ev-006","reviewer_role":"sre"}
  ]
}`)
	ingestOut := filepath.Join(t.TempDir(), "feedback")
	if err := run([]string{"feedback", "ingest", filepath.Join(root, "feedback-input.json"), "--out", ingestOut, "--json"}); err != nil {
		t.Fatalf("feedback ingest failed: %v", err)
	}

	writeMainTestFile(t, root, "online-evaluation.json", `{
  "version": "patchline.safe-online-evaluation/v1",
  "claim": "Candidate detectors run in shadow mode against live aggregate feedback until precision, recall, and burden gates pass, while policy mutation remains impossible without a separate human gate.",
  "candidate_detectors": [
    {"detector":"orm.write-breadth","release":"v1.0.0","min_evidence":3,"min_precision_bp":9000,"min_recall_bp":9000,"max_average_burden_minutes":8,"requires_human_gate":true},
    {"detector":"sql.destructive-ddl","release":"v1.0.0","min_evidence":3,"min_precision_bp":9000,"min_recall_bp":9000,"max_average_burden_minutes":12,"requires_human_gate":true}
  ]
}`)
	onlineOut := filepath.Join(t.TempDir(), "online")
	if err := run([]string{"feedback", "online-eval", "--feedback", filepath.Join(ingestOut, "live-feedback.json"), "--spec", filepath.Join(root, "online-evaluation.json"), "--out", onlineOut, "--json"}); err != nil {
		t.Fatalf("online evaluation failed: %v", err)
	}
	var online feedback.OnlineEvaluationReport
	readMainTestJSON(t, filepath.Join(onlineOut, "online-evaluation.json"), &online)
	if online.Summary.PromotionCandidates != 1 || online.Summary.ShadowOnly != 1 || online.PolicyMutationAllowed {
		t.Fatalf("unexpected online evaluation: %#v", online.Summary)
	}

	writeMainTestFile(t, root, "detector-deprecation.json", `{
  "version": "patchline.detector-deprecation/v1",
  "claim": "Detector deprecation is transparent: Patchline evaluates source-free reviewer evidence against precision and burden thresholds, then requires public notice, independent review, appeal time, and migration guidance before retiring a detector.",
  "as_of_date": "2026-06-15",
  "min_evidence": 3,
  "min_precision_bp": 9000,
  "max_average_burden_minutes": 12,
  "min_notice_days": 30,
  "min_reviewer_roles": 2,
  "min_appeal_window_days": 14,
  "required_public_channels": ["release-notes", "public-roadmap"],
  "detectors": [
    {"detector":"orm.write-breadth","release":"v1.0.0","owner":"detector-maintainers","public_notice_id":"notice-orm-write-breadth-v1","notice_opened_at":"2026-05-01","public_channels":["release-notes","public-roadmap"],"reviewer_roles":["maintainer","dba"],"replacement_detector":"sql.destructive-ddl","migration_guide":"docs/detector-deprecation.md","appeal_window_days":21},
    {"detector":"sql.destructive-ddl","release":"v1.0.0","owner":"detector-maintainers","public_notice_id":"notice-sql-destructive-v1","notice_opened_at":"2026-05-01","public_channels":["release-notes","public-roadmap"],"reviewer_roles":["maintainer","dba"],"replacement_detector":"none","migration_guide":"docs/detector-deprecation.md","appeal_window_days":21}
  ]
}`)
	deprecationOut := filepath.Join(t.TempDir(), "deprecation")
	if err := run([]string{"feedback", "detector-deprecation", "--feedback", filepath.Join(ingestOut, "live-feedback.json"), "--spec", filepath.Join(root, "detector-deprecation.json"), "--out", deprecationOut, "--json"}); err != nil {
		t.Fatalf("detector deprecation failed: %v", err)
	}
	var deprecation feedback.DetectorDeprecationReport
	readMainTestJSON(t, filepath.Join(deprecationOut, "detector-deprecation.json"), &deprecation)
	if !deprecation.OK || deprecation.Summary.ReadyToDeprecate != 1 || deprecation.Summary.Retained != 1 {
		t.Fatalf("unexpected deprecation report: %#v", deprecation.Summary)
	}
	if stat, err := os.Stat(filepath.Join(deprecationOut, "detector-deprecation.md")); err != nil || stat.Size() == 0 {
		t.Fatalf("expected detector-deprecation.md to be written, stat=%#v err=%v", stat, err)
	}

	writeMainTestFile(t, root, "active-learning.json", `{
  "version": "patchline.adopter-active-learning/v1",
  "claim": "Adopter-local active learning ranks uncertain examples for local reviewer attention while publishing only aggregate uncertainty and information-gain metrics outside the organization.",
  "min_uncertainty_bp": 2500,
  "min_information_gain_bp": 3000,
  "max_queue_size": 2,
  "cases": [
    {"local_case_id":"case-001","detector":"sql.destructive-ddl","release":"v1.0.0","confidence_bp":5100,"uncertainty_bp":4900,"expected_information_gain_bp":8100,"estimated_burden_minutes":8},
    {"local_case_id":"case-002","detector":"orm.write-breadth","release":"v1.0.0","confidence_bp":5400,"uncertainty_bp":4600,"expected_information_gain_bp":7600,"estimated_burden_minutes":6},
    {"local_case_id":"case-003","detector":"migration.guard","release":"v1.0.0","confidence_bp":9000,"uncertainty_bp":1000,"expected_information_gain_bp":1800,"estimated_burden_minutes":4}
  ]
}`)
	activeOut := filepath.Join(t.TempDir(), "active")
	if err := run([]string{"feedback", "active-learning-queue", "--spec", filepath.Join(root, "active-learning.json"), "--out", activeOut, "--json"}); err != nil {
		t.Fatalf("active-learning queue failed: %v", err)
	}
	var active feedback.ActiveLearningReport
	readMainTestJSON(t, filepath.Join(activeOut, "active-learning-queue.json"), &active)
	if active.Shareable || active.Aggregate.QueuedCases != 2 || active.LocalQueue.Shareable {
		t.Fatalf("unexpected active-learning report: %#v", active)
	}
	aggregateData, err := os.ReadFile(filepath.Join(activeOut, "active-learning-aggregate.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(aggregateData), "case-001") || strings.Contains(string(aggregateData), "local_case_id") {
		t.Fatalf("shareable active-learning aggregate leaked local IDs:\n%s", aggregateData)
	}

	writeMainTestFile(t, root, "policy-freeze.json", `{
  "version": "patchline.policy-freeze/v1",
  "claim": "High-stakes organizations are pinned to audited detector releases during active incidents so experimental detector updates cannot alter blocking policy until incident review completes.",
  "as_of_date": "2026-06-02",
  "audited_releases": [
    {"release":"v1.0.0","audit_hash":"audit-v1-0-0","approved_for_high_stakes":true},
    {"release":"v1.1.0","audit_hash":"audit-v1-1-0","approved_for_high_stakes":true}
  ],
  "organizations": [
    {"organization":"critical-payments","high_stakes":true,"incident_active":true,"incident_id":"inc-2026-06","current_release":"v1.0.0","proposed_release":"v1.1.0"}
  ]
}`)
	policyOut := filepath.Join(t.TempDir(), "policy")
	if err := run([]string{"feedback", "policy-freeze", "--spec", filepath.Join(root, "policy-freeze.json"), "--out", policyOut, "--json"}); err != nil {
		t.Fatalf("policy freeze failed: %v", err)
	}
	var freeze feedback.PolicyFreezeReport
	readMainTestJSON(t, filepath.Join(policyOut, "policy-freeze.json"), &freeze)
	if !freeze.OK || freeze.Summary.PinnedOrganizations != 1 || freeze.Decisions[0].PolicyChangeAllowed {
		t.Fatalf("unexpected policy freeze: %#v", freeze)
	}

	writeMainTestFile(t, root, "calibration-monitor.json", `{
  "version": "patchline.live-calibration-monitor/v1",
  "claim": "A live calibration monitor watches confidence deciles against reviewer-confirmation rates and alerts only when drift exceeds a pre-registered tolerance.",
  "pre_registered_tolerance_bp": 2000,
  "min_evidence": 3,
  "alert_channels": ["artifact-review"]
}`)
	calibrationOut := filepath.Join(t.TempDir(), "calibration")
	if err := run([]string{"feedback", "calibration-monitor", "--feedback", filepath.Join(ingestOut, "live-feedback.json"), "--spec", filepath.Join(root, "calibration-monitor.json"), "--out", calibrationOut, "--json"}); err != nil {
		t.Fatalf("calibration monitor failed: %v", err)
	}
	var calibration feedback.CalibrationMonitorReport
	readMainTestJSON(t, filepath.Join(calibrationOut, "calibration-monitor.json"), &calibration)
	if calibration.Summary.Alerts != 1 {
		t.Fatalf("unexpected calibration monitor: %#v", calibration)
	}

	writeMainTestFile(t, root, "retention-lifecycle.json", `{
  "version": "patchline.feedback-retention-lifecycle/v1",
  "claim": "Operational feedback follows deterministic lifecycle policy: raw local evidence expires, mid-age evidence anonymizes, and aggregate evidence remains only within its retention window.",
  "as_of_date": "2026-06-02",
  "policies": [{"class":"live-feedback","raw_retention_days":30,"anonymize_after_days":14,"aggregate_retention_days":365}],
  "artifacts": [
    {"artifact_id":"raw-old","class":"live-feedback","created_at":"2026-04-01","contains_raw_evidence":true,"contains_local_examples":true,"observed_action":"delete"},
    {"artifact_id":"raw-mid","class":"live-feedback","created_at":"2026-05-10","contains_raw_evidence":true,"contains_local_examples":true,"observed_action":"anonymize"},
    {"artifact_id":"agg-new","class":"live-feedback","created_at":"2026-05-15","aggregated":true,"anonymized":true,"observed_action":"retain_aggregate"}
  ]
}`)
	retentionOut := filepath.Join(t.TempDir(), "retention")
	if err := run([]string{"feedback", "retention-lifecycle", "--spec", filepath.Join(root, "retention-lifecycle.json"), "--out", retentionOut, "--json"}); err != nil {
		t.Fatalf("retention lifecycle failed: %v", err)
	}
	var retention feedback.RetentionLifecycleReport
	readMainTestJSON(t, filepath.Join(retentionOut, "retention-lifecycle.json"), &retention)
	if !retention.OK || retention.Summary.DeleteRequired != 1 || retention.Summary.AnonymizeRequired != 1 {
		t.Fatalf("unexpected retention lifecycle: %#v", retention)
	}

	writeMainTestFile(t, root, "trust-regression.json", `{
  "version": "patchline.human-trust-regression/v1",
  "claim": "Human-trust regression checks learned-component updates for faithful explanations, citations, uncertainty disclosure, over-reliance, and review burden before accepting a new release.",
  "baseline": {"release":"v1.0.0","explanation_faithfulness_bp":9000,"evidence_citation_coverage_bp":8800,"uncertainty_disclosure_bp":8500,"overreliance_rate_bp":1200,"mean_review_burden_minutes":9},
  "current": {"release":"v1.1.0","explanation_faithfulness_bp":8950,"evidence_citation_coverage_bp":8800,"uncertainty_disclosure_bp":8600,"overreliance_rate_bp":1150,"mean_review_burden_minutes":9},
  "tolerances": {"max_faithfulness_drop_bp":100,"max_citation_coverage_drop_bp":100,"max_uncertainty_drop_bp":100,"max_overreliance_increase_bp":50,"max_burden_increase_minutes":1}
}`)
	trustOut := filepath.Join(t.TempDir(), "trust")
	if err := run([]string{"feedback", "trust-regression", "--spec", filepath.Join(root, "trust-regression.json"), "--out", trustOut, "--json"}); err != nil {
		t.Fatalf("trust regression failed: %v", err)
	}
	var trust feedback.TrustRegressionReport
	readMainTestJSON(t, filepath.Join(trustOut, "trust-regression.json"), &trust)
	if !trust.OK || trust.Summary.FailedChecks != 0 {
		t.Fatalf("unexpected trust regression: %#v", trust)
	}

	writeMainTestFile(t, root, "methodology.json", `{
  "version": "patchline.live-learning-methodology/v1",
  "claim": "The public methodology report demonstrates that live learning improves recall while avoiding increased reviewer over-reliance by linking every reported result to gate-backed evidence.",
  "experiments": [
    {"name":"shadow-detectors","population":"public-slices","baseline_recall_bp":7100,"live_learning_recall_bp":7900,"baseline_overreliance_bp":1300,"live_learning_overreliance_bp":1200,"baseline_burden_minutes":12,"live_learning_burden_minutes":11,"evidence":["safe-online-evaluation-gate","live-calibration-monitor-gate"]}
  ],
  "gate_evidence": [
    {"gate":"safe-online-evaluation-gate","report_hash":"hash-online","reproduction_command":"make safe-online-evaluation-gate"},
    {"gate":"live-calibration-monitor-gate","report_hash":"hash-calibration","reproduction_command":"make live-calibration-monitor-gate"}
  ]
}`)
	methodologyOut := filepath.Join(t.TempDir(), "methodology")
	if err := run([]string{"feedback", "methodology-report", "--spec", filepath.Join(root, "methodology.json"), "--out", methodologyOut, "--json"}); err != nil {
		t.Fatalf("methodology report failed: %v", err)
	}
	var methodology feedback.MethodologyReport
	readMainTestJSON(t, filepath.Join(methodologyOut, "live-learning-methodology.json"), &methodology)
	if !methodology.OK || methodology.Summary.RecallImproved != 1 || methodology.Summary.OverrelianceNotIncreased != 1 {
		t.Fatalf("unexpected methodology report: %#v", methodology)
	}
}

func TestSecurityReviewBlocksProtectedSurfaceWithoutRequiredGates(t *testing.T) {
	report := buildSecurityReviewReport(
		[]string{"internal/evidence/adapter.go", "internal/project/propose.go", "internal/archive/archive.go", "internal/dbdryrun/dryrun.go"},
		[]string{"threat-model-gate"},
	)
	if report.Summary.Success || report.Summary.ProtectedSurfaces != 4 || report.Summary.BlockedSurfaces != 4 || report.Summary.MissingGates == 0 {
		t.Fatalf("expected blocked protected surfaces, got %#v", report.Summary)
	}
	surfaces := map[string]securityReviewSurface{}
	for _, surface := range report.Surfaces {
		surfaces[surface.Name] = surface
	}
	for _, name := range []string{"adapters", "archive-handlers", "execution-features", "generators"} {
		if surfaces[name].Status != "blocked" {
			t.Fatalf("expected %s blocked, got %#v", name, surfaces[name])
		}
	}
	if !containsString(surfaces["archive-handlers"].MissingGates, "archive-security-gate") {
		t.Fatalf("expected archive gate requirement, got %#v", surfaces["archive-handlers"])
	}
	if !containsString(surfaces["generators"].MissingGates, "generated-code-quarantine-gate") {
		t.Fatalf("expected generated quarantine requirement, got %#v", surfaces["generators"])
	}
}

func TestSecurityReviewPassesWithSurfaceGatesAndWritesReport(t *testing.T) {
	passed := []string{
		"archive-security-gate",
		"db-dry-run-gate",
		"generated-code-quarantine-gate",
		"offline-validation-gate",
		"prompt-context-gate",
		"redaction-stability-gate",
		"secret-scan-gate",
		"threat-model-gate",
	}
	report := buildSecurityReviewReport(
		[]string{"internal/evidence/adapter.go", "internal/project/project.go", "internal/project/propose.go", "internal/project/compare.go", "internal/dbdryrun/dryrun.go"},
		passed,
	)
	if !report.Summary.Success || report.Summary.ProtectedSurfaces != 4 || report.Summary.MissingGates != 0 || report.Hash == "" {
		t.Fatalf("expected passing security review, got %#v", report)
	}
	if !strings.Contains(report.Markdown, "Patchline security review") || !strings.Contains(report.Markdown, "archive-security-gate") {
		t.Fatalf("expected review markdown with gates, got %s", report.Markdown)
	}
	out := filepath.Join(t.TempDir(), "security")
	if err := run([]string{"security", "review", "--changed-files", "internal/project/propose.go,internal/archive/archive.go", "--passed-gates", strings.Join(passed, ","), "--out", out, "--json"}); err != nil {
		t.Fatalf("security review command failed: %v", err)
	}
	var loaded securityReviewReport
	readMainTestJSON(t, filepath.Join(out, "security-review.json"), &loaded)
	if !loaded.Summary.Success || loaded.Summary.ProtectedSurfaces != 2 {
		t.Fatalf("expected written passing report, got %#v", loaded.Summary)
	}
	if stat, err := os.Stat(filepath.Join(out, "security-review.md")); err != nil || stat.Size() == 0 {
		t.Fatalf("expected security review markdown, stat=%#v err=%v", stat, err)
	}
}

func TestRepoCaseStudiesGenerateNarrativesFromAnalyses(t *testing.T) {
	rootA := t.TempDir()
	writeMainTestFile(t, rootA, "db/migrate/001_update.sql", "UPDATE accounts SET status = 'active';\n")
	outA := filepath.Join(t.TempDir(), "analysis-a")
	if err := run([]string{"repo", "analyze", rootA, "--stages", "inventory,baseline,propose,compare", "--proposal-kind", "all", "--budget", "files=2,lines=60,tokens=4000,changes=1", "--no-llm", "--out", outA, "--json"}); err != nil {
		t.Fatalf("first analysis failed: %v", err)
	}
	rootB := t.TempDir()
	writeMainTestFile(t, rootB, "db/migrate/001_delete.sql", "DELETE FROM account_events;\n")
	outB := filepath.Join(t.TempDir(), "analysis-b")
	if err := run([]string{"repo", "analyze", rootB, "--stages", "inventory,baseline,propose,compare", "--proposal-kind", "all", "--budget", "files=2,lines=60,tokens=4000,changes=1", "--no-llm", "--out", outB, "--json"}); err != nil {
		t.Fatalf("second analysis failed: %v", err)
	}
	caseOut := filepath.Join(t.TempDir(), "cases")
	if err := run([]string{"repo", "case-studies", "--analyses", outA + "," + outB, "--out", caseOut, "--json"}); err != nil {
		t.Fatalf("case studies failed: %v", err)
	}
	var report repoCaseStudiesReport
	readMainTestJSON(t, filepath.Join(caseOut, "case-studies.json"), &report)
	if report.Version != "patchline.repo-case-studies/v1" || report.Summary.Cases != 2 || len(report.Cases) != 2 || report.Hash == "" {
		t.Fatalf("unexpected case-study report: %#v", report)
	}
	for _, study := range report.Cases {
		if study.Problem == "" || len(study.Evidence) == 0 || study.GeneratedIntervention == "" || study.DeterministicOutcome == "" || study.MaintainerAction == "" {
			t.Fatalf("case missing required narrative fields: %#v", study)
		}
		if study.GeneratedFiles == 0 || !strings.Contains(study.GeneratedIntervention, "untrusted generated") {
			t.Fatalf("expected generated intervention summary, got %#v", study)
		}
	}
	markdown, err := os.ReadFile(filepath.Join(caseOut, "case-studies.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(markdown), "generated public-repo case studies") || !strings.Contains(string(markdown), "maintainer action") {
		t.Fatalf("expected narrative markdown, got:\n%s", string(markdown))
	}
}

func TestRepoTaxonomyClassifiesFailureModesFromAnalyses(t *testing.T) {
	rootA := t.TempDir()
	writeMainTestFile(t, rootA, "db/migrate/001_update.sql", "UPDATE accounts SET status = 'active';\n")
	writeMainTestFile(t, rootA, "db/migrate/002_add_index.sql", "CREATE INDEX idx_accounts_status ON accounts(status);\n")
	outA := filepath.Join(t.TempDir(), "analysis-a")
	if err := run([]string{"repo", "analyze", rootA, "--stages", "inventory,baseline,propose,compare", "--proposal-kind", "all", "--budget", "files=3,lines=80,tokens=4000,changes=2", "--no-llm", "--out", outA, "--json"}); err != nil {
		t.Fatalf("first analysis failed: %v", err)
	}
	rootB := t.TempDir()
	writeMainTestFile(t, rootB, "db/migrate/001_delete.sql", "DELETE FROM account_events;\n")
	writeMainTestFile(t, rootB, "scripts/repair_backfill.sql", "UPDATE account_events SET repaired = true;\n")
	outB := filepath.Join(t.TempDir(), "analysis-b")
	if err := run([]string{"repo", "analyze", rootB, "--stages", "inventory,baseline,propose,compare", "--proposal-kind", "all", "--budget", "files=3,lines=80,tokens=4000,changes=2", "--no-llm", "--out", outB, "--json"}); err != nil {
		t.Fatalf("second analysis failed: %v", err)
	}
	out := filepath.Join(t.TempDir(), "taxonomy")
	if err := run([]string{"repo", "taxonomy", "--analyses", outA + "," + outB, "--out", out, "--json"}); err != nil {
		t.Fatalf("taxonomy failed: %v", err)
	}
	var report repoTaxonomyReport
	readMainTestJSON(t, filepath.Join(out, "failure-taxonomy.json"), &report)
	if report.Version != "patchline.repo-failure-taxonomy/v1" || report.Summary.Analyses != 2 || report.Summary.FailureModes < 3 || report.Summary.Occurrences < 3 || report.Hash == "" {
		t.Fatalf("unexpected taxonomy report: %#v", report)
	}
	seen := map[string]bool{}
	for _, mode := range report.Modes {
		seen[mode.ID] = true
		if mode.Definition == "" || mode.RepairRisk == "" || mode.MaintainerDecision == "" || mode.Occurrences == 0 || len(mode.Examples) == 0 {
			t.Fatalf("mode missing taxonomy fields: %#v", mode)
		}
	}
	for _, id := range []string{"broad-or-destructive-mutation", "missing-transaction-boundary", "non-idempotent-or-unknown-repair"} {
		if !seen[id] {
			t.Fatalf("expected failure mode %s in %#v", id, report.Modes)
		}
	}
	markdown, err := os.ReadFile(filepath.Join(out, "failure-taxonomy.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(markdown), "public-corpus failure-mode taxonomy") || !strings.Contains(string(markdown), "maintainer decision") {
		t.Fatalf("expected taxonomy markdown, got:\n%s", string(markdown))
	}
}

func TestRepoQualitativeNotesWriteCodingNotesFromAnalyses(t *testing.T) {
	rootA := t.TempDir()
	writeMainTestFile(t, rootA, "db/migrate/001_update.sql", "UPDATE accounts SET status = 'active';\n")
	writeMainTestFile(t, rootA, "db/migrate/002_delete.sql", "DELETE FROM account_events;\n")
	outA := filepath.Join(t.TempDir(), "analysis-a")
	if err := run([]string{"repo", "analyze", rootA, "--stages", "inventory,baseline,propose,compare", "--proposal-kind", "all", "--budget", "files=3,lines=80,tokens=4000,changes=2", "--no-llm", "--out", outA, "--json"}); err != nil {
		t.Fatalf("first analysis failed: %v", err)
	}
	rootB := t.TempDir()
	writeMainTestFile(t, rootB, "db/migrate/001_alter.sql", "ALTER TABLE accounts ADD COLUMN repaired_at timestamp;\n")
	writeMainTestFile(t, rootB, "scripts/repair_backfill.sql", "UPDATE accounts SET repaired_at = now();\n")
	outB := filepath.Join(t.TempDir(), "analysis-b")
	if err := run([]string{"repo", "analyze", rootB, "--stages", "inventory,baseline,propose,compare", "--proposal-kind", "all", "--budget", "files=3,lines=80,tokens=4000,changes=2", "--no-llm", "--out", outB, "--json"}); err != nil {
		t.Fatalf("second analysis failed: %v", err)
	}
	out := filepath.Join(t.TempDir(), "notes")
	if err := run([]string{"repo", "qualitative-notes", "--analyses", outA + "," + outB, "--out", out, "--json"}); err != nil {
		t.Fatalf("qualitative notes failed: %v", err)
	}
	var report repoQualitativeNotesReport
	readMainTestJSON(t, filepath.Join(out, "qualitative-notes.json"), &report)
	if report.Version != "patchline.repo-qualitative-notes/v1" || report.Summary.Analyses != 2 || report.Summary.Notes < 6 || report.Hash == "" {
		t.Fatalf("unexpected qualitative notes report: %#v", report)
	}
	for _, label := range []string{"false_positive_candidate", "false_negative_candidate", "proof_hole", "maintainer_decision"} {
		if report.Summary.ByLabel[label] == 0 {
			t.Fatalf("expected label %s in %#v", label, report.Summary.ByLabel)
		}
	}
	for _, note := range report.Notes {
		if note.ID == "" || note.Label == "" || note.Status == "" || note.Observation == "" || note.CoderInstruction == "" || note.MaintainerQuestion == "" || note.RecommendedDecision == "" || len(note.Evidence) == 0 {
			t.Fatalf("note missing coding fields: %#v", note)
		}
	}
	markdown, err := os.ReadFile(filepath.Join(out, "qualitative-notes.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(markdown), "qualitative coding notes") || !strings.Contains(string(markdown), "false_positive_candidate") || !strings.Contains(string(markdown), "maintainer question") {
		t.Fatalf("expected qualitative markdown, got:\n%s", string(markdown))
	}
}

func TestRepoCrossFileExamplesShowBaselinesMissRepairClues(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "db/migrate/001_update_accounts.sql", "UPDATE accounts SET status = 'active';\n")
	writeMainTestFile(t, root, "scripts/repair_accounts.sql", "UPDATE accounts SET status = 'active' WHERE status IS NULL;\n")
	outAnalysis := filepath.Join(t.TempDir(), "analysis")
	if err := run([]string{"repo", "analyze", root, "--stages", "inventory,baseline", "--budget", "files=5,lines=80,tokens=4000,changes=2", "--no-llm", "--out", outAnalysis, "--json"}); err != nil {
		t.Fatalf("analysis failed: %v", err)
	}
	out := filepath.Join(t.TempDir(), "cross-file")
	if err := run([]string{"repo", "cross-file-examples", "--analyses", outAnalysis, "--out", out, "--json"}); err != nil {
		t.Fatalf("cross-file examples failed: %v", err)
	}
	var report repoCrossFileExamplesReport
	readMainTestJSON(t, filepath.Join(out, "cross-file-examples.json"), &report)
	if report.Version != "patchline.repo-cross-file-examples/v1" || report.Summary.Examples == 0 || report.Summary.RepairClues == 0 || report.Summary.GrepOnlyMisses == 0 || report.Summary.SQLOnlyMisses == 0 || report.Hash == "" {
		t.Fatalf("unexpected cross-file examples report: %#v", report)
	}
	foundRepair := false
	for _, example := range report.Examples {
		if example.ClueKind == "repair" {
			foundRepair = true
		}
		if example.PatchlineClue == "" || example.GrepOnlyResult == "" || example.SQLOnlyResult == "" || example.WhyGrepOnlyMissed == "" || example.WhySQLOnlyMissed == "" || example.MaintainerAction == "" || len(example.CluePaths) == 0 || len(example.Evidence) == 0 {
			t.Fatalf("example missing side-by-side fields: %#v", example)
		}
	}
	if !foundRepair {
		t.Fatalf("expected at least one repair clue: %#v", report.Examples)
	}
	markdown, err := os.ReadFile(filepath.Join(out, "cross-file-examples.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(markdown), "cross-file repair clue examples") || !strings.Contains(string(markdown), "grep-only") || !strings.Contains(string(markdown), "SQL-only") {
		t.Fatalf("expected side-by-side markdown, got:\n%s", string(markdown))
	}
}

func TestRepoRejectedGeneratedExamplesExplainPlausibleRejectedCode(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "db/migrate/001_update_accounts.sql", "UPDATE accounts SET status = 'active';\n")
	outAnalysis := filepath.Join(t.TempDir(), "analysis")
	llmCommand := `printf "%s\n" "-- Plausible generated repair for reviewer" "UPDATE accounts SET status = 'active';"`
	if err := run([]string{"repo", "analyze", root, "--stages", "inventory,baseline,propose,compare", "--proposal-kind", "guards", "--budget", "files=3,lines=80,tokens=4000,changes=2", "--llm-command", llmCommand, "--out", outAnalysis, "--json"}); err != nil {
		t.Fatalf("analysis failed: %v", err)
	}
	out := filepath.Join(t.TempDir(), "rejected")
	if err := run([]string{"repo", "rejected-generated", "--analyses", outAnalysis, "--out", out, "--json"}); err != nil {
		t.Fatalf("rejected generated examples failed: %v", err)
	}
	var report repoRejectedGeneratedReport
	readMainTestJSON(t, filepath.Join(out, "rejected-generated.json"), &report)
	if report.Version != "patchline.repo-rejected-generated/v1" || report.Summary.Examples == 0 || report.Summary.RejectedInterventions == 0 || report.Summary.HighRiskGeneratedSQL == 0 || report.Hash == "" {
		t.Fatalf("unexpected rejected-generated report: %#v", report)
	}
	for _, example := range report.Examples {
		if example.LooksUsefulBecause == "" || example.NormalDiffAppearance == "" || example.DeterministicRejection == "" || example.RejectedStatus != "rejected-by-deterministic-checks" || example.MaintainerAction == "" || len(example.ContentExcerpt) == 0 {
			t.Fatalf("example missing rejection explanation fields: %#v", example)
		}
	}
	markdown, err := os.ReadFile(filepath.Join(out, "rejected-generated.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(markdown), "rejected generated-code examples") || !strings.Contains(string(markdown), "looks useful because") || !strings.Contains(string(markdown), "deterministic rejection") {
		t.Fatalf("expected rejected-generated markdown, got:\n%s", string(markdown))
	}
}

func TestRepoReviewabilityExamplesDoNotClaimFullRepair(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "db/migrate/001_update_accounts.sql", "UPDATE accounts SET status = 'active';\n")
	outAnalysis := filepath.Join(t.TempDir(), "analysis")
	if err := run([]string{"repo", "analyze", root, "--stages", "inventory,baseline,propose,compare", "--proposal-kind", "all", "--budget", "files=10,lines=100,tokens=8000,changes=2", "--no-llm", "--out", outAnalysis, "--json"}); err != nil {
		t.Fatalf("analysis failed: %v", err)
	}
	out := filepath.Join(t.TempDir(), "reviewability")
	if err := run([]string{"repo", "reviewability-examples", "--analyses", outAnalysis, "--out", out, "--json"}); err != nil {
		t.Fatalf("reviewability examples failed: %v", err)
	}
	var report repoReviewabilityExamplesReport
	readMainTestJSON(t, filepath.Join(out, "reviewability-examples.json"), &report)
	if report.Version != "patchline.repo-reviewability-examples/v1" || report.Summary.Examples == 0 || report.Summary.TestExamples == 0 || report.Summary.GuardExamples == 0 || report.Summary.NoFullRepairClaims != report.Summary.Examples || report.Hash == "" {
		t.Fatalf("unexpected reviewability report: %#v", report)
	}
	for _, example := range report.Examples {
		if example.ReviewabilityGain == "" || !strings.Contains(example.NonRepairClaim, "does not claim to repair") || example.DeterministicOutcome == "" || example.MaintainerAction == "" || len(example.ContentExcerpt) == 0 || len(example.ProofHoles) == 0 {
			t.Fatalf("example missing reviewability/non-repair fields: %#v", example)
		}
	}
	markdown, err := os.ReadFile(filepath.Join(out, "reviewability-examples.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(markdown), "generated reviewability examples") || !strings.Contains(string(markdown), "non-repair claim") || !strings.Contains(string(markdown), "proof holes preserved") {
		t.Fatalf("expected reviewability markdown, got:\n%s", string(markdown))
	}
}

func TestRepoLimitationsLedgerDistinguishesLimitationCategories(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "db/migrate/001_update_accounts.sql", "UPDATE accounts SET status = 'active';\n")
	writeMainTestFile(t, root, "scripts/repair_accounts.sql", "UPDATE accounts SET status = 'active' WHERE status IS NULL;\n")
	outAnalysis := filepath.Join(t.TempDir(), "analysis")
	if err := run([]string{"repo", "analyze", root, "--stages", "inventory,baseline,propose,compare", "--proposal-kind", "all", "--budget", "files=6,lines=100,tokens=8000,changes=2", "--no-llm", "--out", outAnalysis, "--json"}); err != nil {
		t.Fatalf("analysis failed: %v", err)
	}
	out := filepath.Join(t.TempDir(), "limitations")
	if err := run([]string{"repo", "limitations-ledger", "--analyses", outAnalysis, "--out", out, "--json"}); err != nil {
		t.Fatalf("limitations ledger failed: %v", err)
	}
	var report repoLimitationsLedgerReport
	readMainTestJSON(t, filepath.Join(out, "limitations-ledger.json"), &report)
	if report.Version != "patchline.repo-limitations-ledger/v1" || report.Summary.Analyses != 1 || report.Summary.Limitations < 4 || report.Hash == "" {
		t.Fatalf("unexpected limitations ledger: %#v", report)
	}
	for _, category := range []string{"unsupported_ecosystem", "uncertain_causality", "missing_runtime_evidence", "intentionally_conservative_check"} {
		if report.Summary.ByCategory[category] == 0 {
			t.Fatalf("expected limitation category %s in %#v", category, report.Summary.ByCategory)
		}
	}
	for _, limitation := range report.Limitations {
		if limitation.ID == "" || limitation.Observation == "" || limitation.WhyItMatters == "" || limitation.NotAClaim == "" || len(limitation.Evidence) == 0 || len(limitation.NextEvidence) == 0 || len(limitation.AffectedArtifacts) == 0 {
			t.Fatalf("limitation missing required review fields: %#v", limitation)
		}
	}
	markdown, err := os.ReadFile(filepath.Join(out, "limitations-ledger.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(markdown), "limitations ledger") || !strings.Contains(string(markdown), "not a claim") || !strings.Contains(string(markdown), "missing_runtime_evidence") {
		t.Fatalf("expected limitations markdown, got:\n%s", string(markdown))
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

func TestBundleRedactorRemovesCanaryValues(t *testing.T) {
	redactor := newBundleRedactor()
	data := []byte(`{"path":"db/migrate/PATCHLINE_LEAK_CANARY_ALPHA.sql","message":"contact patchline_canary@example.invalid and quote 'PATCHLINE_LEAK_CANARY_BETA'"}` + "\n")
	redacted, err := redactor.redactJSONBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	text := string(redacted)
	for _, canary := range []string{"PATCHLINE_LEAK_CANARY_ALPHA", "PATCHLINE_LEAK_CANARY_BETA", "patchline_canary@example.invalid"} {
		if strings.Contains(text, canary) {
			t.Fatalf("redacted JSON leaked %s:\n%s", canary, text)
		}
	}
	if !strings.Contains(text, "[redacted:") {
		t.Fatalf("expected redaction tokens in:\n%s", text)
	}
}

func TestBundleRedactorStableAcrossInstancesAndFormats(t *testing.T) {
	jsonData := []byte(`{"path":"db/migrate/PATCHLINE_STABILITY_SECRET_VALUE.sql","email":"patchline_stability@example.invalid","message":"quote 'PATCHLINE_STABILITY_LITERAL' and PATCHLINE_STABILITY_SECRET_VALUE"}` + "\n")
	sarifData := []byte(`{"version":"2.1.0","runs":[{"results":[{"message":{"text":"PATCHLINE_STABILITY_SECRET_VALUE"},"locations":[{"physicalLocation":{"artifactLocation":{"uri":"db/migrate/PATCHLINE_STABILITY_SECRET_VALUE.sql"}}}]}]}]}` + "\n")
	jsonlData := []byte(`{"message":"patchline_stability@example.invalid PATCHLINE_STABILITY_SECRET_VALUE"}` + "\n")
	textData := []byte("prompt mentions patchline_stability@example.invalid and PATCHLINE_STABILITY_SECRET_VALUE\n")
	cases := []struct {
		name string
		path string
		data []byte
	}{
		{name: "json", path: "proposal.json", data: jsonData},
		{name: "sarif", path: "summary.sarif", data: sarifData},
		{name: "jsonl", path: "events.jsonl", data: jsonlData},
		{name: "text", path: "prompt.txt", data: textData},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			first, err := newBundleRedactor().redactFileBytes(tc.path, tc.data)
			if err != nil {
				t.Fatal(err)
			}
			second, err := newBundleRedactor().redactFileBytes(tc.path, tc.data)
			if err != nil {
				t.Fatal(err)
			}
			if string(first) != string(second) {
				t.Fatalf("redaction should be stable across redactor instances\nfirst=%s\nsecond=%s", first, second)
			}
			for _, canary := range []string{"PATCHLINE_STABILITY_SECRET_VALUE", "PATCHLINE_STABILITY_LITERAL", "patchline_stability@example.invalid"} {
				if strings.Contains(string(first), canary) {
					t.Fatalf("redacted %s leaked %s:\n%s", tc.name, canary, first)
				}
			}
			if !strings.Contains(string(first), "[redacted:") {
				t.Fatalf("expected stable redaction token in %s:\n%s", tc.name, first)
			}
		})
	}
}

func TestRepoAnalyzeRedactionStableAcrossResume(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "db/migrate/PATCHLINE_STABILITY_SECRET_VALUE.sql", strings.Join([]string{
		"-- contact patchline_stability@example.invalid",
		"UPDATE PATCHLINE_STABILITY_CUSTOMERS",
		"SET api_token = 'PATCHLINE_STABILITY_SECRET_VALUE'",
		"WHERE id = 42;",
	}, "\n"))
	out := filepath.Join(t.TempDir(), "analysis")
	args := []string{"repo", "analyze", root, "--stages", "inventory,baseline,propose,compare", "--proposal-kind", "all", "--budget", "files=4,lines=80,tokens=12000,changes=1", "--no-llm", "--redact", "--ci", "--out", out, "--json"}
	if err := run(args); err != nil {
		t.Fatalf("repo analyze redaction failed: %v", err)
	}
	surfaces := []string{
		"analysis-bundle/summary.sarif",
		"analysis-bundle/compare.json",
		"redacted-artifacts/prompts-and-generated/proposal/prompt.txt",
		"redacted-artifacts/reports/compare/compare.json",
		"redacted-artifacts/bundles/analysis-bundle/summary.sarif",
	}
	before := map[string]string{}
	for _, rel := range surfaces {
		data, err := os.ReadFile(filepath.Join(out, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		text := string(data)
		for _, canary := range []string{"PATCHLINE_STABILITY_SECRET_VALUE", "PATCHLINE_STABILITY_CUSTOMERS", "patchline_stability@example.invalid"} {
			if strings.Contains(text, canary) {
				t.Fatalf("%s leaked canary %s:\n%s", rel, canary, text)
			}
		}
		if !strings.Contains(text, "[redacted:") {
			t.Fatalf("expected redaction tokens in %s:\n%s", rel, text)
		}
		before[rel] = text
	}
	resumeArgs := append([]string(nil), args...)
	resumeArgs = append(resumeArgs, "--resume")
	if err := run(resumeArgs); err != nil {
		t.Fatalf("repo analyze redaction resume failed: %v", err)
	}
	for _, rel := range surfaces {
		data, err := os.ReadFile(filepath.Join(out, rel))
		if err != nil {
			t.Fatalf("read resumed %s: %v", rel, err)
		}
		if before[rel] != string(data) {
			t.Fatalf("%s changed after resume\nbefore=%s\nafter=%s", rel, before[rel], data)
		}
	}
}

func TestSupplyChainProvenanceCommandCoversRequiredArtifactKinds(t *testing.T) {
	root := t.TempDir()
	binary := filepath.Join(root, "bin", "patchline")
	release := filepath.Join(root, "release", "patchline.tar.gz")
	experimentDir := filepath.Join(root, "experiment")
	corpus := filepath.Join(root, "corpus", "lobsters.tar.gz")
	writeMainTestFile(t, root, "bin/patchline", "binary bytes\n")
	writeMainTestFile(t, root, "release/patchline.tar.gz", "archive bytes\n")
	writeMainTestFile(t, root, "experiment/proposal/proposal.json", `{"version":"patchline.proposal/v1"}`+"\n")
	writeMainTestFile(t, root, "experiment/analysis-bundle/summary.sarif", `{"version":"2.1.0"}`+"\n")
	writeMainTestFile(t, root, "corpus/lobsters.tar.gz", "public corpus bytes\n")
	out := filepath.Join(root, "provenance.json")
	if err := run([]string{
		"supply-chain", "provenance",
		"--subject", "patchline-test",
		"--source", "repo=lobsters/lobsters@3b80b47aa5aaba37ec44413e7d1dc96fcf1585b6",
		"--command", "go build -o bin/patchline ./cmd/patchline",
		"--artifact", "binary=" + binary,
		"--artifact", "release_archive=" + release,
		"--artifact", "generated_experiment_artifact=" + experimentDir,
		"--artifact", "public_corpus_download=" + corpus,
		"--out", out,
		"--json",
	}); err != nil {
		t.Fatalf("supply-chain provenance failed: %v", err)
	}
	var report supplyChainProvenanceReport
	readMainTestJSON(t, out, &report)
	if report.Version != "patchline.supply-chain-provenance/v1" || report.Subject != "patchline-test" || !report.Verification.Complete {
		t.Fatalf("unexpected provenance report: %#v", report)
	}
	if report.Summary.Binaries != 1 || report.Summary.ReleaseArchives != 1 || report.Summary.ExperimentArtifacts != 1 || report.Summary.PublicCorpusDownloads != 1 || report.Summary.Directories != 1 {
		t.Fatalf("unexpected provenance summary: %#v", report.Summary)
	}
	if len(report.Artifacts) != 4 || report.ReportHash == "" {
		t.Fatalf("expected four hashed artifacts and report hash: %#v", report)
	}
	for _, artifact := range report.Artifacts {
		if !strings.HasPrefix(artifact.SHA256, "sha256:") || artifact.Bytes <= 0 || artifact.Files <= 0 {
			t.Fatalf("artifact missing digest metadata: %#v", artifact)
		}
	}
}

func TestReleaseChecksumsSignsSortedArtifacts(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "dist/patchline-darwin-arm64.tar.gz", "darwin archive\n")
	writeMainTestFile(t, root, "dist/patchline-linux-amd64.tar.gz", "linux archive\n")
	seed, err := attest.GenerateSeed()
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(root, "release")
	if err := run([]string{
		"release", "checksums",
		"--subject", "patchline-test-release",
		"--seed-hex", attest.SeedHex(seed),
		"--artifact", filepath.Join(root, "dist/patchline-linux-amd64.tar.gz"),
		"--artifact", filepath.Join(root, "dist/patchline-darwin-arm64.tar.gz"),
		"--out", out,
		"--json",
	}); err != nil {
		t.Fatalf("release checksums failed: %v", err)
	}
	var report releaseChecksumReport
	readMainTestJSON(t, filepath.Join(out, "release-checksums.json"), &report)
	if report.Version != "patchline.release-checksums/v1" || !report.SignatureVerified || len(report.Artifacts) != 2 || report.ReportHash == "" {
		t.Fatalf("unexpected release checksum report: %#v", report)
	}
	if !strings.Contains(report.ReproducibleBuild.Command, "-trimpath") || !strings.Contains(report.ReproducibleBuild.Ldflags, "-buildid=") {
		t.Fatalf("expected reproducible build instructions in report: %#v", report.ReproducibleBuild)
	}
	checksums, err := os.ReadFile(filepath.Join(out, "checksums.sha256"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(checksums)), "\n")
	if len(lines) != 2 || !strings.Contains(lines[0], "darwin-arm64") || !strings.Contains(lines[1], "linux-amd64") {
		t.Fatalf("expected sorted checksum lines, got %q", string(checksums))
	}
	if err := run([]string{"verify-artifact", filepath.Join(out, "checksums.attestation.json"), "--artifact", filepath.Join(out, "checksums.sha256"), "--json"}); err != nil {
		t.Fatalf("release checksum attestation did not verify: %v", err)
	}
}

func TestContributorCheckPlanWritesExpectedSteps(t *testing.T) {
	out := filepath.Join(t.TempDir(), "contributor")
	if err := run([]string{"contributor", "check", "--root", ".", "--out", out, "--packages", "./cmd/patchline", "--gates", "gate,impact-gate", "--plan-only", "--json"}); err != nil {
		t.Fatalf("contributor check plan failed: %v", err)
	}
	var report contributorCheckReport
	readMainTestJSON(t, filepath.Join(out, "contributor-check.json"), &report)
	if report.Version != "patchline.contributor-check/v1" || report.Mode != "plan" || report.Summary.Planned != 7 || report.Summary.FastGates != 2 || !report.Summary.Success {
		t.Fatalf("unexpected contributor plan: %#v", report)
	}
	for _, id := range []string{"roadmap-ignore", "forbidden-doc-refs", "gofmt", "diff-check", "focused-go-tests", "fast-gate-gate", "fast-gate-impact-gate"} {
		if !contributorTestHasStep(report, id) {
			t.Fatalf("expected step %s in %#v", id, report.Steps)
		}
	}
}

func TestContributorForbiddenRefScannerIgnoresPrivateRoadmapOnly(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "100_STEPS.md", "100_STEPS is intentionally private here\n")
	writeMainTestFile(t, root, "docs/ok.md", "ordinary docs\n")
	matches, err := scanContributorForbiddenRefs(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("expected no matches, got %#v", matches)
	}
	writeMainTestFile(t, root, "docs/bad.md", "do not mention 100_STEPS in tracked docs\n")
	matches, err = scanContributorForbiddenRefs(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 || !strings.Contains(matches[0], "docs/bad.md") {
		t.Fatalf("unexpected matches: %#v", matches)
	}
}

func TestContributorCheckReportsFailures(t *testing.T) {
	root := initMainTestGitRepo(t)
	writeMainTestFile(t, root, ".gitignore", "100_STEPS.md\n")
	writeMainTestFile(t, root, "go.mod", "module example.test/contributor\n\ngo 1.22\n")
	writeMainTestFile(t, root, "bad.go", "package main\n\nfunc  main(){}\n")
	writeMainTestFile(t, root, "100_STEPS.md", "ignored roadmap\n")
	runMainTestGit(t, root, "add", ".gitignore", "go.mod", "bad.go")
	runMainTestGit(t, root, "commit", "-m", "fixture")
	out := filepath.Join(t.TempDir(), "contributor")
	err := run([]string{"contributor", "check", "--root", root, "--out", out, "--packages", ".", "--gates", "", "--json"})
	if err == nil {
		t.Fatal("expected contributor check to fail on gofmt")
	}
	var report contributorCheckReport
	readMainTestJSON(t, filepath.Join(out, "contributor-check.json"), &report)
	if report.Summary.Failed == 0 || report.Summary.Success {
		t.Fatalf("expected failed contributor report: %#v", report.Summary)
	}
	if !contributorTestStepFailed(report, "gofmt") {
		t.Fatalf("expected gofmt failure in %#v", report.Steps)
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

func contributorTestHasStep(report contributorCheckReport, id string) bool {
	for _, step := range report.Steps {
		if step.ID == id {
			return true
		}
	}
	return false
}

func contributorTestStepFailed(report contributorCheckReport, id string) bool {
	for _, step := range report.Steps {
		if step.ID == id && step.Status == "failed" {
			return true
		}
	}
	return false
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

func TestRepoPlaybookCommandWritesRemediationArtifacts(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, ".github/CODEOWNERS", "/db/migrate/ @org/db-team\n")
	writeMainTestFile(t, root, "db/migrate/001_accounts.sql", "UPDATE accounts SET status = 'disabled';\n")
	inv, err := project.InventoryPath(project.InventoryOptions{Path: root})
	if err != nil {
		t.Fatal(err)
	}
	intakeReport, err := intake.Run(context.Background(), intake.Options{Path: root})
	if err != nil {
		t.Fatal(err)
	}
	baseline := project.Baseline(inv, inv.Facts, intakeReport)
	baselineDir := filepath.Join(t.TempDir(), "baseline")
	if err := project.WriteBaseline(baselineDir, baseline); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "playbook")
	if err := run([]string{"repo", "playbook", "--baseline", baselineDir, "--out", out, "--json"}); err != nil {
		t.Fatalf("repo playbook failed: %v", err)
	}
	var report project.RemediationPlaybookReport
	readMainTestJSON(t, filepath.Join(out, "playbook.json"), &report)
	if report.Version != project.RemediationPlaybookVersion || report.Summary.Playbooks == 0 || report.Summary.RollbackPoints == 0 || report.Summary.CodeownersHandoffs == 0 {
		t.Fatalf("unexpected playbook report: %#v", report)
	}
	md, err := os.ReadFile(filepath.Join(out, "playbook.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(md), "Remediation playbooks") || !strings.Contains(string(md), "@org/db-team") {
		t.Fatalf("unexpected playbook markdown: %s", string(md))
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

func TestRepoClaimsEvidenceMapsPaperClaimsToArtifacts(t *testing.T) {
	root := t.TempDir()
	left := filepath.Join(root, "left")
	right := filepath.Join(root, "right")
	writeClaimsEvidenceAnalysisForTest(t, left, "example/rails", "Ruby", "aaaabbbbccccddddeeeeffff0000111122223333")
	writeClaimsEvidenceAnalysisForTest(t, right, "example/go", "Go", "bbbbccccddddeeeeffff00001111222233334444")

	report, err := buildRepoClaimsEvidenceReport([]string{left, right})
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.PublicRepos != 2 || report.Summary.Claims != 6 || report.Summary.AbstractClaims != 2 || report.Summary.IntroductionClaims != 2 || report.Summary.EvaluationClaims != 2 {
		t.Fatalf("unexpected claims summary: %#v", report.Summary)
	}
	if report.Summary.SupportedClaims == 0 || report.Summary.ClaimsWithLimitations != report.Summary.Claims || report.Hash == "" {
		t.Fatalf("expected supported limited claims with hash: %#v", report.Summary)
	}
	for _, claim := range report.Claims {
		if claim.Status == "unsupported" || len(claim.Evidence) == 0 || len(claim.Artifacts) == 0 || len(claim.Limitations) == 0 || len(claim.MissingEvidence) == 0 || claim.PaperWording == "" || claim.ReviewerCheck == "" {
			t.Fatalf("claim is not evidence-backed and qualified: %#v", claim)
		}
		if claim.Section != "abstract" && claim.Section != "introduction" && claim.Section != "evaluation" {
			t.Fatalf("unexpected section: %#v", claim)
		}
	}
	if !strings.Contains(report.Markdown, "claims-to-evidence map") || !strings.Contains(report.Markdown, "abstract") || !strings.Contains(report.Markdown, "evaluation") {
		t.Fatalf("expected rendered paper claim map, got: %s", report.Markdown)
	}
}

func TestRepoFiguresWritesPaperFigureSVGs(t *testing.T) {
	root := t.TempDir()
	left := filepath.Join(root, "left")
	right := filepath.Join(root, "right")
	out := filepath.Join(root, "figures")
	writeClaimsEvidenceAnalysisForTest(t, left, "example/rails", "Ruby", "aaaabbbbccccddddeeeeffff0000111122223333")
	writeClaimsEvidenceAnalysisForTest(t, right, "example/go", "Go", "bbbbccccddddeeeeffff00001111222233334444")

	report, err := buildRepoFiguresReport([]string{left, right}, out)
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.Figures != 5 || report.Summary.SVGs != 5 || report.Summary.DataFiles != 5 || report.Summary.PublicRepos != 2 || report.Hash == "" {
		t.Fatalf("unexpected figure summary: %#v", report.Summary)
	}
	if report.Summary.RepairAnalysisLoop != 1 || report.Summary.Architecture != 1 || report.Summary.CorpusComposition != 1 || report.Summary.Ablations != 1 || report.Summary.InterventionOutcomes != 1 {
		t.Fatalf("missing required figure kinds: %#v", report.Summary)
	}
	if err := writeRepoFiguresReport(out, report); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"repair-analysis-loop.svg", "architecture.svg", "corpus-composition.svg", "ablations.svg", "intervention-outcomes.svg", "figures.json", "figures.md"} {
		data, err := os.ReadFile(filepath.Join(out, name))
		if err != nil {
			t.Fatalf("expected %s: %v", name, err)
		}
		if strings.HasSuffix(name, ".svg") && (!strings.Contains(string(data), "<svg") || !strings.Contains(string(data), "</svg>")) {
			t.Fatalf("expected svg content in %s: %s", name, data)
		}
	}
	if !strings.Contains(report.Markdown, "paper figures") || !strings.Contains(report.Markdown, "Before/after intervention outcomes") {
		t.Fatalf("expected figure markdown, got: %s", report.Markdown)
	}
}

func writeClaimsEvidenceAnalysisForTest(t *testing.T, root, repo, language, ref string) {
	t.Helper()
	writeMainTestFile(t, root, "analyze.json", `{
  "version": "patchline.repo-analyze/v1",
  "input": "`+repo+`",
  "subpath": "db/migrate",
  "source": {"input":"`+repo+`","resolved_commit":"`+ref+`"}
}
`)
	writeMainTestFile(t, root, "inventory/inventory.json", `{
  "version": "patchline.project-inventory/v1",
  "root": "`+filepath.ToSlash(root)+`",
  "files_scanned": 2,
  "languages": [{"name":"`+language+`","count":2}],
  "frameworks": [{"kind":"framework","path":"Gemfile","summary":"rails"}],
  "migration_roots": [{"kind":"migration-root","path":"db/migrate","summary":"migrations"}],
  "native_commands": [{"name":"test","command":"make test"}],
  "test_commands": [{"name":"test","command":"make test"}]
}
`)
	writeMainTestFile(t, root, "inventory/facts.jsonl", `{"id":"fact:test","kind":"sql","path":"db/migrate/001.sql"}`+"\n")
	writeMainTestFile(t, root, "baseline/baseline.json", `{
  "version": "patchline.baseline/v1",
  "summary": {
    "ranked_risks": 2,
    "evidence_links": 3,
    "provenance_slices": 2,
    "policy_checks": 1,
    "repair_proof_summaries": 1,
    "abstract_proof_holes": 1,
    "repair_proof_open": 1,
    "identifier_only_links": 1
  },
  "risks": [{"id":"risk:test","stable_id":"stable-risk:test","path":"db/migrate/001.sql","kind":"high-risk-sql","severity":"high","score":90}],
  "evidence_links": [{"risk_id":"risk:test","fact_id":"fact:test","path":"db/migrate/001.sql","confidence":"identifier-shared"}],
  "provenance_slices": [{"risk_id":"risk:test","summary":"linked repair clue","confidence":"identifier-shared"}],
  "policy_checks": [{"id":"policy:test","risk_id":"risk:test","status":"warn","rule":"scope"}],
  "repair_proof_summaries": [{"id":"proof:test","risk_id":"risk:test","status":"open","proof_holes":["no runtime witness"]}],
  "hash": "baseline"
}
`)
	writeMainTestFile(t, root, "proposal/proposal.json", `{
  "version": "patchline.proposal/v1",
  "generated_files": [
    {"path":"patchline-proposals/tests/risk_test.md","kind":"tests","content_hash":"sha256:test","risk_ids":["risk:test"]},
    {"path":"patchline-proposals/guards/risk_guard.sql","kind":"guards","content_hash":"sha256:guard","risk_ids":["risk:test"]}
  ],
  "hash": "proposal"
}
`)
	writeMainTestFile(t, root, "proposal/patchline-proposals/tests/risk_test.md", "assert scoped update row counts before repair\n")
	writeMainTestFile(t, root, "proposal/patchline-proposals/guards/risk_guard.sql", "select count(*) from accounts where needs_repair = true;\n")
	writeMainTestFile(t, root, "compare/compare.json", `{
  "version": "patchline.repo-compare/v1",
  "summary": {"generated_files": 2, "patchline_checks_passed": 2, "native_checks_skipped": 1},
  "intervention_loop": {"status":"accepted-for-review", "required_next_actions":["run native tests"]},
  "review_badge": {"status":"needs-runtime-evidence", "proof_holes":["no native run"], "reasons":["runtime evidence missing"]},
  "generated_checks": [{"path":"patchline-proposals/tests/risk_test.md","status":"pass"}, {"path":"patchline-proposals/guards/risk_guard.sql","status":"pass"}],
  "hash": "compare"
}
`)
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

func mainTestRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		next := filepath.Dir(dir)
		if next == dir {
			t.Fatal("could not find repo root")
		}
		dir = next
	}
}

func recomputeMainTestCanonicalHash(s string) string {
	lines := strings.Split(strings.TrimSuffix(s, "\n"), "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "canonical-sha256: ") {
			canonicalText := strings.Join(lines[:i], "\n") + "\n"
			sum := sha256.Sum256([]byte(canonicalText))
			lines[i] = "canonical-sha256: " + hex.EncodeToString(sum[:])
			return strings.Join(lines, "\n") + "\n"
		}
	}
	return s
}

func mustReadMainTestFile(t *testing.T, path string) string {
	t.Helper()
	bytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(bytes)
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

func writeMainTestJSONFile(t *testing.T, path string, value any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
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

func mainTestEvidenceMarketplaceRegistry(t *testing.T, root string) evidencemarketplace.Registry {
	t.Helper()
	writeMainTestFile(t, root, "artifacts/redacted-hazard.json", `{
  "version": "patchline.redacted-hazard-example/v1",
  "finding": "redacted stable-risk for unsafe migration guard",
  "hazard_class": "redacted-submitter-label",
  "evidence": [{"path": "db/migrate/20260101010101_backfill_accounts.rb", "snippet": "<model>.find_each { |row| row.update!(<redacted_column>: <redacted_value>) }"}]
}
`)
	writeMainTestFile(t, root, "artifacts/cert-witness.json", `{
  "version": "patchline.redacted-certificate-witness/v1",
  "checks": ["redaction-reviewed", "license-cleared", "artifact-hashes-verified", "reproducible-without-private-data"]
}
`)
	example := evidencemarketplace.Example{
		ID:           "redacted-unsafe-migration-guard",
		Title:        "Redacted unsafe migration guard",
		Organization: "Example Data Platform",
		Ecosystem:    "postgres",
		HazardClass:  "unsafe-schema-change-without-guard",
		Source: evidencemarketplace.Source{
			Host:    "github",
			Repo:    "public/example-data-platform",
			Ref:     "refs/heads/main",
			Commit:  "fedcba9876543210fedcba9876543210fedcba98",
			Subpath: "migrations",
		},
		LicenseSPDX: "CC0-1.0",
		Consent:     "Example Data Platform approved publication of this redacted certificate-backed hazard example under the declared public license.",
		Redaction: evidencemarketplace.Redaction{
			Reviewed:      true,
			RawDataShared: false,
			Method:        "all project, table, owner, and literal names replaced by stable placeholders",
			Fields:        []string{"project names", "table names", "literal values"},
			Reviewer:      "artifact-review",
		},
		Artifacts: []evidencemarketplace.Artifact{
			{Path: "artifacts/redacted-hazard.json", Role: "redacted-hazard-example", SHA256: mainTestFileHash(t, filepath.Join(root, "artifacts/redacted-hazard.json")), Redacted: true},
			{Path: "artifacts/cert-witness.json", Role: "certificate-witness", SHA256: mainTestFileHash(t, filepath.Join(root, "artifacts/cert-witness.json")), Redacted: true},
		},
		Certificate: evidencemarketplace.Certificate{
			ID:       "cert-redacted-unsafe-migration-guard",
			Issuer:   "patchline-test-issuer",
			IssuedAt: "2026-06-02T21:22:11Z",
			Obligations: []string{
				"redaction-reviewed",
				"license-cleared",
				"artifact-hashes-verified",
				"reproducible-without-private-data",
			},
		},
		Reproduction: []string{
			"go run ./cmd/patchline evidence-marketplace publish --registry examples/evidence-marketplace/registry.json --out results/generated/evidence-marketplace --json",
			"jq -e '.summary.published >= 1' results/generated/evidence-marketplace/marketplace.json",
		},
		Limitations: []string{"The example proves publication mechanics and certificate obligations without exposing raw project data."},
	}
	example.Certificate.SubjectHash = evidencemarketplace.ExpectedSubjectHash(example)
	return evidencemarketplace.Registry{
		Version: evidencemarketplace.RegistryVersion,
		Claim:   "The public evidence marketplace admits only redacted, license-cleared, certificate-backed examples with reproducible commands and verified artifact hashes.",
		Marketplace: evidencemarketplace.Metadata{
			Name:       "Patchline public evidence marketplace",
			Maintainer: "Patchline maintainers",
			PolicyURL:  "docs/evidence-marketplace.md",
		},
		Examples: []evidencemarketplace.Example{example},
	}
}

func mainTestChallengeRegistry(t *testing.T, root string) evidencemarketplace.Registry {
	t.Helper()
	registry := mainTestEvidenceMarketplaceRegistry(t, root)
	writeMainTestFile(t, root, "artifacts/adversarial.sql", "UPDATE accounts SET status = 'active';\n")
	example := registry.Examples[0]
	example.ID = "redacted-cli-adversarial-broad-update"
	example.Title = "Redacted CLI adversarial broad update"
	example.Artifacts = append(example.Artifacts, evidencemarketplace.Artifact{
		Path:     "artifacts/adversarial.sql",
		Role:     "adversarial-migration",
		SHA256:   mainTestFileHash(t, filepath.Join(root, "artifacts/adversarial.sql")),
		Redacted: true,
	})
	example.Certificate.ID = "cert-redacted-cli-adversarial-broad-update"
	example.Certificate.Obligations = append(example.Certificate.Obligations, "responsible-disclosure-cleared")
	example.Reproduction = []string{
		"go run ./cmd/patchline evidence-marketplace challenge --registry examples/evidence-marketplace/challenge-registry.json --out results/generated/adversarial-challenge --json",
		"jq -e '.summary.scoreboard_entries >= 1' results/generated/adversarial-challenge/challenge.json",
	}
	example.GateReputation = &evidencemarketplace.GateReputationInput{
		ReproducibleRuns: 8,
		FirstVerifiedAt:  "2025-01-01T00:00:00Z",
		LastVerifiedAt:   "2025-07-01T00:00:00Z",
		IndependentConfirmations: []string{
			"Independent Artifact Review Lab",
			"Migration Safety Working Group",
		},
	}
	example.Challenge = &evidencemarketplace.ChallengeSubmission{
		TrackID:                  "patchline-adversarial-migrations-2026",
		AdversarialGoal:          "Preserve a high-risk broad data rewrite in a tiny public-safe migration proof.",
		AttackSurface:            "SQL migration analyzer and public benchmark importer",
		ExpectedDetectorBehavior: "flag-high-risk-migration",
		MigrationArtifact:        "artifacts/adversarial.sql",
		MaxPublicProofLines:      4,
		NoveltyStatement:         "The proof isolates the update-without-row-key hazard into one redacted migration statement.",
		Disclosure: evidencemarketplace.ChallengeDisclosureStatus{
			Status:               "public-safe",
			PublicReleaseAllowed: true,
			CoordinatedWith:      "Patchline Public Challenge Maintainers",
			ReportedAt:           "2026-06-02T21:00:00Z",
			FullExploitHash:      "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		},
	}
	example.Certificate.SubjectHash = evidencemarketplace.ExpectedSubjectHash(example)
	registry.ChallengeTrack = &evidencemarketplace.ChallengeTrack{
		ID:       "patchline-adversarial-migrations-2026",
		Name:     "Patchline public adversarial migration challenge",
		RulesURL: "docs/evidence-marketplace.md#adversarial-migration-challenge",
		ResponsibleDisclosure: evidencemarketplace.ResponsibleDisclosurePolicy{
			Contact:                 "security@patchline.example",
			PolicyURL:               "docs/evidence-marketplace.md#responsible-disclosure-rules",
			EmbargoDays:             90,
			PublicSafeArtifactsOnly: true,
		},
		Scoring: evidencemarketplace.ChallengeScoringPolicy{
			MinScoreboardScore: 75,
			Weights: evidencemarketplace.ChallengeScoreWeights{
				AnalyzerSignal:        40,
				Reproducibility:       20,
				Minimization:          15,
				Novelty:               15,
				ResponsibleDisclosure: 10,
			},
		},
	}
	registry.Examples = []evidencemarketplace.Example{example}
	return registry
}

func mainTestGovernanceRegistry(t *testing.T, root string) evidencemarketplace.Registry {
	t.Helper()
	registry := mainTestEvidenceMarketplaceRegistry(t, root)
	registry.Claim = "The CLI governance fixture publishes three redacted, certificate-backed examples so board decisions can accept, deprecate, and quarantine shared evidence with archive preservation."
	registry.Examples[0].ID = "redacted-cli-governance-accepted"
	registry.Examples[0].Title = "Redacted CLI governance accepted evidence"
	registry.Examples[0].Certificate.ID = "cert-redacted-cli-governance-accepted"
	registry.Examples[0].Reproduction = []string{
		"go run ./cmd/patchline evidence-marketplace govern --spec examples/evidence-marketplace/governance-board.json --out results/generated/evidence-governance-board --json",
		"jq -e '.ok == true' results/generated/evidence-governance-board/governance-board.json",
	}
	registry.Examples[0].Certificate.SubjectHash = evidencemarketplace.ExpectedSubjectHash(registry.Examples[0])
	deprecated := mainTestGovernanceExampleVariant(t, root, registry.Examples[0], "redacted-cli-governance-deprecated", "django", "constraint-tightening-before-complete-backfill", "artifacts/deprecated-hazard.json", "artifacts/deprecated-certificate.json")
	quarantined := mainTestGovernanceExampleVariant(t, root, registry.Examples[0], "redacted-cli-governance-quarantined", "sqlalchemy", "replication-lag-during-online-backfill", "artifacts/quarantined-hazard.json", "artifacts/quarantined-certificate.json")
	registry.Examples = []evidencemarketplace.Example{registry.Examples[0], deprecated, quarantined}
	return registry
}

func mainTestGovernanceExampleVariant(t *testing.T, root string, base evidencemarketplace.Example, id, ecosystem, hazardClass, hazardPath, certificatePath string) evidencemarketplace.Example {
	t.Helper()
	writeMainTestFile(t, root, hazardPath, fmt.Sprintf(`{
  "version": "patchline.redacted-hazard-example/v1",
  "finding": "redacted CLI governance fixture for %s",
  "hazard_class": %q,
  "evidence": [{"path": "db/migrate/<redacted>_%s.sql", "snippet": "UPDATE <table> SET <column> = <redacted> WHERE <guard> IS NULL"}]
}
`, id, hazardClass, id))
	writeMainTestFile(t, root, certificatePath, `{
  "version": "patchline.redacted-certificate-witness/v1",
  "checks": ["redaction-reviewed", "license-cleared", "artifact-hashes-verified", "reproducible-without-private-data"]
}
`)
	example := base
	example.ID = id
	example.Title = "Redacted CLI governance evidence " + id
	example.Ecosystem = ecosystem
	example.HazardClass = hazardClass
	example.Source.Repo = "public/" + ecosystem + "-governance-cli"
	example.Artifacts = []evidencemarketplace.Artifact{
		{Path: hazardPath, Role: "redacted-hazard-example", SHA256: mainTestFileHash(t, filepath.Join(root, filepath.FromSlash(hazardPath))), Redacted: true},
		{Path: certificatePath, Role: "certificate-witness", SHA256: mainTestFileHash(t, filepath.Join(root, filepath.FromSlash(certificatePath))), Redacted: true},
	}
	example.Certificate.ID = "cert-" + id
	example.Certificate.SubjectHash = evidencemarketplace.ExpectedSubjectHash(example)
	return example
}

func mainTestBoardReviewSpec(registry evidencemarketplace.Registry) evidencemarketplace.BoardReviewSpec {
	return evidencemarketplace.BoardReviewSpec{
		Version:      evidencemarketplace.BoardReviewSpecVersion,
		Claim:        "The Patchline CLI governance board fixture accepts, deprecates, and quarantines shared evidence only after quorum, independent approvals, conflict checks, hash binding, and archive-preserving tombstones are verified.",
		RegistryPath: "governance-registry.json",
		Board: evidencemarketplace.BoardPolicy{
			ID:                      "patchline-cli-shared-evidence-board",
			Name:                    "Patchline CLI shared evidence governance board",
			CharterURL:              "docs/evidence-governance-board.md",
			ConflictPolicy:          "Approvers affiliated with the submitting organization abstain and cannot count toward independent approval quorum.",
			Quorum:                  3,
			MinIndependentApprovers: 2,
		},
		Decisions: []evidencemarketplace.BoardDecisionInput{
			mainTestBoardDecision(registry.Examples[0], "accept"),
			mainTestBoardDecision(registry.Examples[1], "deprecate"),
			mainTestBoardDecision(registry.Examples[2], "quarantine"),
		},
	}
}

func mainTestBoardDecision(example evidencemarketplace.Example, status string) evidencemarketplace.BoardDecisionInput {
	decision := evidencemarketplace.BoardDecisionInput{
		EvidenceID:             example.ID,
		RequestedStatus:        status,
		Rationale:              "The board verified the redacted artifact hashes, certificate subject hash, and reviewer votes before recording this shared-evidence lifecycle decision.",
		EvidenceHash:           evidencemarketplace.EvidenceHash(example),
		CertificateSubjectHash: evidencemarketplace.ExpectedSubjectHash(example),
		Reviewers: []evidencemarketplace.BoardReviewer{
			{Name: "Database Reliability Guild", Role: "dba-reviewer", Affiliation: "Database Reliability Guild", Vote: "approve"},
			{Name: "Independent Artifact Review Lab", Role: "artifact-reviewer", Affiliation: "Independent Artifact Review Lab", Vote: "approve"},
			{Name: "Patchline Maintainer Chair", Role: "chair", Affiliation: "Patchline Maintainers", Vote: "abstain"},
		},
	}
	switch status {
	case "deprecate":
		decision.Deprecation = &evidencemarketplace.DeprecationPlan{
			EffectiveDate:         "2026-07-01",
			ReplacementEvidenceID: "redacted-cli-governance-accepted",
			ContinuingValidity:    "The deprecated evidence remains useful for historical prevalence but should no longer be selected for active release claims.",
		}
	case "quarantine":
		decision.Quarantine = &evidencemarketplace.QuarantinePlan{
			Trigger:                     "independent reproducibility challenge",
			Reason:                      "Active release is paused while maintainers investigate a disputed source-host provenance cue, but checksum-preserving archive evidence remains auditable.",
			RevocationOrSupersessionURL: "docs/evidence-governance-board.md#quarantine",
			PreserveTombstone:           true,
		}
	}
	return decision
}

func mainTestAppealWorkflowSpec(t *testing.T, registry evidencemarketplace.Registry, root string) evidencemarketplace.AppealWorkflowSpec {
	t.Helper()
	preservation := mainTestAppealPreservation(t, registry, root)
	return evidencemarketplace.AppealWorkflowSpec{
		Version:            evidencemarketplace.AppealWorkflowSpecVersion,
		Claim:              "The Patchline CLI appeal workflow fixture proves disputed findings preserve archive evidence, bind to governance-board decisions, collect independent reviewer rationales, and publish resolution audit trails.",
		RegistryPath:       "governance-registry.json",
		BoardDecisionsPath: "governance-board.json",
		Board: evidencemarketplace.BoardPolicy{
			ID:                      "patchline-cli-evidence-appeal-board",
			Name:                    "Patchline CLI evidence appeal board",
			CharterURL:              "docs/evidence-appeal-workflow.md",
			ConflictPolicy:          "Appeal reviewers must be independent of the evidence submitter and original board approvers.",
			Quorum:                  3,
			MinIndependentApprovers: 2,
		},
		Appeals: []evidencemarketplace.AppealInput{
			mainTestAppealInput(registry.Examples[0], preservation[registry.Examples[0].ID], "cli-appeal-accepted", "false-positive", "overturned", "upheld"),
			mainTestAppealInput(registry.Examples[1], preservation[registry.Examples[1].ID], "cli-appeal-deprecated", "severity", "modified", "modified"),
			mainTestAppealInput(registry.Examples[2], preservation[registry.Examples[2].ID], "cli-appeal-quarantined", "evidence-integrity", "overturned", "overturned"),
		},
	}
}

func mainTestAppealPreservation(t *testing.T, registry evidencemarketplace.Registry, root string) map[string][]evidencemarketplace.BoardArchivePreservation {
	t.Helper()
	base, err := evidencemarketplace.PublishRegistry(registry, root)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][]evidencemarketplace.BoardArchivePreservation{}
	for _, entry := range base.ArchiveMirror.Entries {
		out[entry.ExampleID] = append(out[entry.ExampleID], evidencemarketplace.BoardArchivePreservation{
			ExampleID:                       entry.ExampleID,
			ArtifactPath:                    entry.ArtifactPath,
			MirrorPath:                      entry.MirrorPath,
			Checksum:                        entry.Checksum,
			WithdrawalID:                    entry.Withdrawal.WithdrawalID,
			TombstoneRequired:               entry.Withdrawal.TombstoneRequired,
			PreserveChecksumAfterWithdrawal: entry.Withdrawal.PreserveChecksumAfterWithdrawal,
			ReviewRequired:                  entry.Withdrawal.ReviewRequired,
			ReplacementAllowed:              entry.Withdrawal.ReplacementAllowed,
		})
	}
	return out
}

func mainTestAppealInput(example evidencemarketplace.Example, preserved []evidencemarketplace.BoardArchivePreservation, appealID, disputeType, requested, resolved string) evidencemarketplace.AppealInput {
	evidenceRef := preserved[0].ArtifactPath
	checksumRef := preserved[0].Checksum
	return evidencemarketplace.AppealInput{
		AppealID:               appealID,
		EvidenceID:             example.ID,
		DisputedFinding:        "The CLI appeal fixture disputes the published migration-safety finding and requires a second review of the archived evidence.",
		DisputeType:            disputeType,
		SubmittedBy:            "CLI Adopter Reliability Team",
		SubmittedAt:            "2026-06-03T12:00:00Z",
		Rationale:              "The appeal rationale explains why the original finding should be rechecked against preserved evidence without changing the underlying archive artifact.",
		RequestedResolution:    requested,
		EvidenceHash:           evidencemarketplace.EvidenceHash(example),
		CertificateSubjectHash: evidencemarketplace.ExpectedSubjectHash(example),
		PreservedArtifacts:     preserved,
		ReviewerRationales: []evidencemarketplace.AppealReviewerRationale{
			{
				Reviewer:           evidencemarketplace.BoardReviewer{Name: "CLI Appeal Ombuds", Role: "appeal-chair", Affiliation: "CLI Appeal Ombuds Office", Vote: "approve"},
				Rationale:          "The preserved artifact path and checksum are sufficient for an independent appeal judgment.",
				EvidenceReferences: []string{evidenceRef, checksumRef},
			},
			{
				Reviewer:           evidencemarketplace.BoardReviewer{Name: "External CLI Migration Clinic", Role: "migration-reviewer", Affiliation: "External CLI Migration Clinic", Vote: "approve"},
				Rationale:          "The redacted evidence preserves the disputed migration shape and supports a reproducible review.",
				EvidenceReferences: []string{evidenceRef},
			},
			{
				Reviewer:           evidencemarketplace.BoardReviewer{Name: "CLI Appeal Clerk", Role: "appeal-clerk", Affiliation: "Patchline Maintainers", Vote: "abstain"},
				Rationale:          "The clerk records completeness while abstaining from the independent technical judgment.",
				EvidenceReferences: []string{checksumRef},
			},
		},
		Resolution: evidencemarketplace.AppealResolution{
			Status:     resolved,
			Rationale:  mainTestAppealResolutionRationale(resolved),
			ResolvedAt: "2026-06-04T15:30:00Z",
			Resolver:   "Patchline CLI Appeal Panel",
			FollowUpActions: []string{
				"Publish the appeal workflow report with the governance-board output.",
				"Retain reviewer rationales and preserved checksums in the audit trail.",
			},
		},
	}
}

func mainTestAppealResolutionRationale(status string) string {
	switch status {
	case "upheld":
		return "The appeal is processed but the original governance finding remains unchanged after independent review of preserved evidence."
	case "modified":
		return "The appeal narrows the finding language while preserving the original evidence and reviewer rationale trail."
	case "overturned":
		return "The appeal overturns the disputed interpretation and records the preserved evidence needed for follow-up audit."
	default:
		return "The appeal records a final resolution with preserved evidence and independent reviewer rationale."
	}
}

func mainTestFileHash(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func mainTestHasIncidentDrillCounterexample(report incidentdrill.Report, kind string) bool {
	for _, counterexample := range report.Counterexamples {
		if counterexample.Kind == kind {
			return true
		}
	}
	return false
}

func mainTestHasMatchingIncidentRegressionGate(report incidentdrill.Report) bool {
	for _, remediation := range report.Drill.Remediations {
		if remediation.Kind == "regression_gate" && remediation.HashMatches {
			return true
		}
	}
	return false
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
