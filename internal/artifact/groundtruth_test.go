package artifact

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateGroundTruth(t *testing.T) {
	report, err := ValidateGroundTruth("../../benchmarks")
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK {
		t.Fatalf("expected valid benchmark ground truth, got errors: %#v", report.Errors)
	}
	if report.GroundTruthFiles < 9 {
		t.Fatalf("expected committed ground-truth files, got %d", report.GroundTruthFiles)
	}
	if report.Manifests < 3 {
		t.Fatalf("expected committed manifests, got %d", report.Manifests)
	}
	if report.PhaseCounts["pre_deploy"] == 0 {
		t.Fatalf("expected pre_deploy cases in report: %#v", report.PhaseCounts)
	}
	if report.ResultCounts["cannot_prove"] == 0 {
		t.Fatalf("expected negative cannot_prove cases in report: %#v", report.ResultCounts)
	}
}

func TestValidateGroundTruthAllowsFetchRequiredMissingFixtures(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "ground_truth", "migrations"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "manifests"), 0o755); err != nil {
		t.Fatal(err)
	}
	groundTruth := `{
		"case_id": "public-case",
		"case_type": "migration",
		"phase": "pre_deploy",
		"labels": {"expected_result": "flag", "risk": "high"},
		"evidence": [{"kind": "public_source", "locator": "https://example.invalid/migration.sql", "rationale": "pinned public source"}],
		"allowed_inputs": ["migration_text"],
		"excluded_inputs": ["postmortem_text"]
	}`
	if err := os.WriteFile(filepath.Join(root, "ground_truth", "migrations", "public-case.json"), []byte(groundTruth), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := `{
		"version": "patchline.artifact-benchmark/v1",
		"dataset_id": "public-fetch-required",
		"description": "missing fixture is fetched by explicit public target",
		"requires_fetch": true,
		"cases": [{
			"case_id": "public-case",
			"case_type": "migration",
			"available_at": "pre_deploy",
			"fixture": "../cache/missing.sql",
			"ground_truth": "../ground_truth/migrations/public-case.json"
		}]
	}`
	if err := os.WriteFile(filepath.Join(root, "manifests", "public.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := ValidateGroundTruth(root)
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK {
		t.Fatalf("expected requires_fetch manifest to allow missing fixture, got errors: %#v", report.Errors)
	}
}

func TestValidateGroundTruthRejectsMissingFixturesByDefault(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "ground_truth", "migrations"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "manifests"), 0o755); err != nil {
		t.Fatal(err)
	}
	groundTruth := `{
		"case_id": "local-case",
		"case_type": "migration",
		"phase": "pre_deploy",
		"labels": {"expected_result": "flag", "risk": "high"},
		"evidence": [{"kind": "file", "locator": "missing.sql", "rationale": "local fixture should exist"}],
		"allowed_inputs": ["migration_text"],
		"excluded_inputs": ["postmortem_text"]
	}`
	if err := os.WriteFile(filepath.Join(root, "ground_truth", "migrations", "local-case.json"), []byte(groundTruth), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := `{
		"version": "patchline.artifact-benchmark/v1",
		"dataset_id": "local-fixture-required",
		"description": "missing fixture should fail",
		"cases": [{
			"case_id": "local-case",
			"case_type": "migration",
			"available_at": "pre_deploy",
			"fixture": "missing.sql",
			"ground_truth": "../ground_truth/migrations/local-case.json"
		}]
	}`
	if err := os.WriteFile(filepath.Join(root, "manifests", "local.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := ValidateGroundTruth(root)
	if err != nil {
		t.Fatal(err)
	}
	if report.OK {
		t.Fatalf("expected missing local fixture to fail validation")
	}
}
