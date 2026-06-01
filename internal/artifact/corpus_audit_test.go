package artifact

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateCorpusAuditRejectsMissingCandidateLedgerEntry(t *testing.T) {
	root := t.TempDir()
	writeMinimalCorpusAuditFixture(t, root)
	protocolPath := filepath.Join(root, "benchmarks", "corpus_protocol.json")
	var protocol CorpusProtocol
	if err := readJSONFile(protocolPath, &protocol); err != nil {
		t.Fatal(err)
	}
	protocol.CandidatePools[0].Candidates = nil
	writeStudyJSON(t, filepath.Join(root, "benchmarks"), "corpus_protocol.json", protocol)

	report, err := GenerateCorpusAudit(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if report.OK {
		t.Fatalf("expected missing candidate ledger entry to fail audit: %+v", report)
	}
}

func TestGenerateCorpusAuditRejectsMissingRequiredEvidence(t *testing.T) {
	root := t.TempDir()
	writeMinimalCorpusAuditFixture(t, root)
	gtPath := filepath.Join(root, "benchmarks", "ground_truth", "public", "case.json")
	var gt GroundTruthCase
	if err := readJSONFile(gtPath, &gt); err != nil {
		t.Fatal(err)
	}
	gt.Evidence = []Evidence{{Kind: "rule", Locator: "local", Rationale: "not public"}}
	writeStudyJSON(t, filepath.Dir(gtPath), "case.json", gt)

	report, err := GenerateCorpusAudit(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if report.OK {
		t.Fatalf("expected missing public evidence to fail audit: %+v", report)
	}
}

func TestGenerateCorpusAuditRejectsReviewerRefreshCommand(t *testing.T) {
	root := t.TempDir()
	writeMinimalCorpusAuditFixture(t, root)
	protocolPath := filepath.Join(root, "benchmarks", "corpus_protocol.json")
	var protocol CorpusProtocol
	if err := readJSONFile(protocolPath, &protocol); err != nil {
		t.Fatal(err)
	}
	protocol.ReviewerCommands = append(protocol.ReviewerCommands, CorpusReviewerCommand{ID: "bad-refresh", Command: "make artifact-benchmark-refresh", ExpectedExit: 0, Mode: "reviewer"})
	writeStudyJSON(t, filepath.Join(root, "benchmarks"), "corpus_protocol.json", protocol)

	report, err := GenerateCorpusAudit(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if report.OK {
		t.Fatalf("expected reviewer refresh command to fail audit: %+v", report)
	}
	var found bool
	for _, validationErr := range report.Errors {
		if strings.Contains(validationErr.Message, "refresh") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected refresh error, got %+v", report.Errors)
	}
}

func writeMinimalCorpusAuditFixture(t *testing.T, root string) {
	t.Helper()
	manifestDir := filepath.Join(root, "benchmarks", "manifests")
	gtDir := filepath.Join(root, "benchmarks", "ground_truth", "public")
	if err := os.MkdirAll(manifestDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(gtDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeStudyJSON(t, manifestDir, "public.json", Manifest{
		Version:   "patchline.artifact-benchmark/v1",
		DatasetID: "test-public",
		Cases: []ManifestCase{{
			CaseID:      "case",
			CaseType:    "migration",
			AvailableAt: "pre_deploy",
			Fixture:     "../../fixture.sql",
			GroundTruth: "../ground_truth/public/case.json",
		}},
	})
	writeStudyJSON(t, gtDir, "case.json", GroundTruthCase{
		CaseID:   "case",
		CaseType: "migration",
		Phase:    "pre_deploy",
		Labels:   GroundTruthLabel{ExpectedResult: "flag", Risk: "high"},
		Evidence: []Evidence{
			{Kind: "public_source", Locator: "https://example.test/case.sql", Rationale: "public source"},
			{Kind: "sha256", Locator: "fixture.sql", Rationale: "pinned hash"},
		},
		AllowedInputs:  []string{"migration_text"},
		ExcludedInputs: []string{"postmortem_text"},
	})
	protocol := CorpusProtocol{
		Version:         "patchline.corpus-protocol/v1",
		Description:     "test",
		InclusionPolicy: "include deterministic public cases",
		ExclusionPolicy: "exclude private cases",
		RequiredManifests: []CorpusManifestRequirement{{
			DatasetID:           "test-public",
			Manifest:            "benchmarks/manifests/public.json",
			MinCases:            1,
			MinResults:          map[string]int{"flag": 1},
			RequiredEvidenceAll: []string{"public_source", "sha256"},
		}},
		CandidatePools: []CorpusCandidatePool{{
			ID:          "test-pool",
			Description: "test pool",
			Source:      "fixture",
			Candidates: []CorpusCandidate{{
				CaseID:      "case",
				Disposition: "included",
				Manifest:    "benchmarks/manifests/public.json",
				Source:      "fixture.sql",
				Rationale:   "test case",
			}},
		}},
		ReviewerCommands: []CorpusReviewerCommand{{ID: "artifact-corpus-audit", Command: "make artifact-corpus-audit", ExpectedExit: 0, Mode: "reviewer"}},
	}
	writeStudyJSON(t, filepath.Join(root, "benchmarks"), "corpus_protocol.json", protocol)
	if err := os.WriteFile(filepath.Join(root, "Makefile"), []byte("artifact-corpus-audit:\n\ttrue\nartifact-benchmark-refresh:\n\ttrue\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}
