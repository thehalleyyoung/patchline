package artifact

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thehalleyyoung/patchline/internal/evidencemarketplace"
)

func TestImportMarketplaceBenchmarkDerivesLabelsWithoutTrustingSubmitterLabels(t *testing.T) {
	root := t.TempDir()
	registry := marketplaceImportTestRegistry(t, root, marketplaceImportTestRegistryOptions{
		ExampleID:           "community-says-benign",
		RegistryHazardClass: "benign-maintenance-note",
		ArtifactHazardClass: "safe-submitter-artifact-label",
		EvidencePath:        "db/migrate/20260101010101_backfill_accounts.rb",
		EvidenceSnippet:     "<model>.find_each { |row| row.update!(<redacted_column>: <redacted_value>) }",
	})
	registryPath := filepath.Join(root, "registry.json")
	if err := writeArtifactJSON(registryPath, registry); err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(t.TempDir(), "marketplace-import")
	report, err := ImportMarketplaceBenchmark(MarketplaceBenchmarkImportOptions{RegistryPath: registryPath, OutDir: out})
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || report.Summary.Imported != 1 || report.Summary.Rejected != 0 {
		t.Fatalf("unexpected import report: %#v", report)
	}
	if report.Summary.LabelDisagreements != 1 {
		t.Fatalf("expected label disagreement to be preserved, got %#v", report.Summary)
	}
	imported := report.Cases[0]
	if imported.ClaimedHazardClass == imported.DerivedHazardClass || imported.ArtifactHazardClass == imported.DerivedHazardClass {
		t.Fatalf("test fixture did not prove label independence: %#v", imported)
	}
	if imported.DerivedHazardClass != "broad-backfill-without-guard" || imported.CueID != "rails-find-each-update-backfill" {
		t.Fatalf("unexpected derived label: %#v", imported)
	}
	if imported.SubmitterLabelsTrusted {
		t.Fatalf("submitter labels must be recorded but not trusted: %#v", imported)
	}

	manifestPath := filepath.Join(out, filepath.FromSlash(report.Manifest))
	validation, err := ValidateBenchmarkManifest(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if !validation.OK {
		t.Fatalf("generated manifest did not validate: %#v", validation.Errors)
	}
	runReport, err := RunBenchmarkManifest(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if !runReport.OK || runReport.Metrics.Total != 1 || runReport.Cases[0].ActualResult != ResultFlag {
		t.Fatalf("generated benchmark did not execute as a passing real benchmark: %#v", runReport)
	}
	if !contains(runReport.Cases[0].Signals, "high-risk-statements=1") {
		t.Fatalf("expected generated SQL to exercise the migration analyzer, got %v", runReport.Cases[0].Signals)
	}

	var gt GroundTruthCase
	if err := readJSON(filepath.Join(out, filepath.FromSlash(imported.GroundTruth)), &gt); err != nil {
		t.Fatal(err)
	}
	if !groundTruthHasEvidence(gt, "certificate") || !groundTruthHasEvidence(gt, "independent_cue") {
		t.Fatalf("ground truth did not preserve certificate and independent cue provenance: %#v", gt.Evidence)
	}
}

func TestImportMarketplaceBenchmarkRejectsUnsupportedEvidenceCue(t *testing.T) {
	root := t.TempDir()
	registry := marketplaceImportTestRegistry(t, root, marketplaceImportTestRegistryOptions{
		ExampleID:           "unsupported-note",
		RegistryHazardClass: "broad-backfill-without-guard",
		ArtifactHazardClass: "broad-backfill-without-guard",
		EvidencePath:        "docs/note.md",
		EvidenceSnippet:     "This redacted example mentions a migration, but no importer cue matches it.",
	})
	registryPath := filepath.Join(root, "registry.json")
	if err := writeArtifactJSON(registryPath, registry); err != nil {
		t.Fatal(err)
	}

	report, err := ImportMarketplaceBenchmark(MarketplaceBenchmarkImportOptions{RegistryPath: registryPath, OutDir: filepath.Join(t.TempDir(), "unsupported")})
	if err != nil {
		t.Fatal(err)
	}
	if report.OK || report.Summary.Imported != 0 || len(report.Rejected) != 1 {
		t.Fatalf("expected fail-closed unsupported import, got %#v", report)
	}
	if !strings.Contains(strings.Join(report.Rejected[0].Reasons, "\n"), "unsupported redacted evidence cue") {
		t.Fatalf("expected unsupported cue rejection, got %#v", report.Rejected)
	}
}

func TestImportMarketplaceBenchmarkSkipsDuplicatePrevalenceExamples(t *testing.T) {
	root := t.TempDir()
	registry := marketplaceImportTestRegistry(t, root, marketplaceImportTestRegistryOptions{
		ExampleID:           "community-backfill",
		RegistryHazardClass: "broad-backfill-without-guard",
		ArtifactHazardClass: "broad-backfill-without-guard",
		EvidencePath:        "db/migrate/20260101010101_backfill_accounts.rb",
		EvidenceSnippet:     "<model>.find_each { |row| row.update!(<redacted_column>: <redacted_value>) }",
	})
	duplicate := registry.Examples[0]
	duplicate.ID = "community-backfill-resubmission"
	duplicate.Title = "Resubmitted marketplace import fixture"
	duplicate.Certificate.ID = "cert-community-backfill-resubmission"
	duplicate.Certificate.SubjectHash = evidencemarketplace.ExpectedSubjectHash(duplicate)
	registry.Examples = append(registry.Examples, duplicate)

	registryPath := filepath.Join(root, "registry.json")
	if err := writeArtifactJSON(registryPath, registry); err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(t.TempDir(), "marketplace-import-deduplicated")
	report, err := ImportMarketplaceBenchmark(MarketplaceBenchmarkImportOptions{RegistryPath: registryPath, OutDir: out})
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || report.Summary.Published != 2 || report.Summary.Imported != 1 || report.Summary.DuplicateImportsSkipped != 1 || report.Summary.Rejected != 0 {
		t.Fatalf("expected duplicate submission to be skipped without failing import: %#v", report)
	}
	if len(report.Deduplicated) != 1 || report.Deduplicated[0].DuplicateOf == "" || report.Deduplicated[0].PrevalenceGroupKind != "exact" {
		t.Fatalf("unexpected deduplicated import metadata: %#v", report.Deduplicated)
	}
	manifestPath := filepath.Join(out, filepath.FromSlash(report.Manifest))
	runReport, err := RunBenchmarkManifest(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if !runReport.OK || runReport.Metrics.Total != 1 {
		t.Fatalf("duplicate marketplace examples should produce one runnable case: %#v", runReport)
	}
}

type marketplaceImportTestRegistryOptions struct {
	ExampleID           string
	RegistryHazardClass string
	ArtifactHazardClass string
	EvidencePath        string
	EvidenceSnippet     string
}

func marketplaceImportTestRegistry(t *testing.T, root string, options marketplaceImportTestRegistryOptions) evidencemarketplace.Registry {
	t.Helper()
	writeBenchmarkTestFile(t, root, "artifacts/hazard.json", `{
  "version": "patchline.redacted-hazard-example/v1",
  "summary": "Redacted marketplace benchmark import fixture.",
  "hazard_class": "`+options.ArtifactHazardClass+`",
  "evidence": [{
    "path": "`+options.EvidencePath+`",
    "line_range": "10-18",
    "snippet": "`+options.EvidenceSnippet+`"
  }]
}
`)
	writeBenchmarkTestFile(t, root, "artifacts/certificate.json", `{
  "version": "patchline.redacted-certificate-witness/v1",
  "checks": ["redaction-reviewed", "license-cleared", "artifact-hashes-verified", "reproducible-without-private-data"]
}
`)
	example := evidencemarketplace.Example{
		ID:           options.ExampleID,
		Title:        "Redacted marketplace import fixture",
		Organization: "Patchline Import Tests",
		Ecosystem:    "rails",
		HazardClass:  options.RegistryHazardClass,
		Source: evidencemarketplace.Source{
			Host:    "github",
			Repo:    "public/example",
			Ref:     "refs/heads/main",
			Commit:  "0123456789abcdef0123456789abcdef01234567",
			Subpath: "db/migrate",
		},
		LicenseSPDX: "CC-BY-4.0",
		Consent:     "Patchline Import Tests approved publication of this redacted hazard example and certificate under the declared public license.",
		Redaction: evidencemarketplace.Redaction{
			Reviewed:      true,
			RawDataShared: false,
			Method:        "identifiers, literals, owners, and row values replaced with stable placeholders",
			Fields:        []string{"identifiers", "literals", "owners", "row values"},
			Reviewer:      "artifact-review",
		},
		Artifacts: []evidencemarketplace.Artifact{
			{Path: "artifacts/hazard.json", Role: "redacted-hazard-example", SHA256: marketplaceImportTestHash(t, filepath.Join(root, "artifacts", "hazard.json")), Redacted: true},
			{Path: "artifacts/certificate.json", Role: "certificate-witness", SHA256: marketplaceImportTestHash(t, filepath.Join(root, "artifacts", "certificate.json")), Redacted: true},
		},
		Certificate: evidencemarketplace.Certificate{
			ID:       "cert-" + options.ExampleID,
			Issuer:   "patchline-import-test",
			IssuedAt: "2026-06-02T21:22:11Z",
			Obligations: []string{
				"redaction-reviewed",
				"license-cleared",
				"artifact-hashes-verified",
				"reproducible-without-private-data",
			},
		},
		Reproduction: []string{
			"go run ./cmd/patchline artifact-benchmark import-marketplace --registry examples/evidence-marketplace/registry.json --out results/generated/marketplace-import --json",
		},
	}
	example.Certificate.SubjectHash = evidencemarketplace.ExpectedSubjectHash(example)
	return evidencemarketplace.Registry{
		Version: evidencemarketplace.RegistryVersion,
		Claim:   "The test marketplace admits only redacted, certificate-backed hazard examples and lets the benchmark importer derive labels from evidence snippets instead of trusting submitter labels.",
		Marketplace: evidencemarketplace.Metadata{
			Name:       "Patchline marketplace import tests",
			Maintainer: "Patchline tests",
			PolicyURL:  "docs/evidence-marketplace.md",
		},
		Examples: []evidencemarketplace.Example{example},
	}
}

func marketplaceImportTestHash(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func groundTruthHasEvidence(gt GroundTruthCase, kind string) bool {
	for _, evidence := range gt.Evidence {
		if evidence.Kind == kind {
			return true
		}
	}
	return false
}
