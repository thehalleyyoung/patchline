package artifact

import "testing"

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
	if report.Manifests < 2 {
		t.Fatalf("expected committed manifests, got %d", report.Manifests)
	}
	if report.PhaseCounts["pre_deploy"] == 0 {
		t.Fatalf("expected pre_deploy cases in report: %#v", report.PhaseCounts)
	}
	if report.ResultCounts["cannot_prove"] == 0 {
		t.Fatalf("expected negative cannot_prove cases in report: %#v", report.ResultCounts)
	}
}
