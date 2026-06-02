package verdictx

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCrossToolRoundTripEquivalence(t *testing.T) {
	root := repoRoot(t)
	report, err := RunSuite(filepath.Join(root, "specs/verdict-exchange/v1"), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Analyzers) < 3 {
		t.Fatalf("expected at least three analyzers, got %v", report.Analyzers)
	}
	if report.PositiveCases != 3 {
		t.Fatalf("expected three positive cases, got %d", report.PositiveCases)
	}
	if report.RoundTrips != report.PositiveCases {
		t.Fatalf("roundtrip mismatch: %#v", report.Cases)
	}
	if !report.Verified {
		t.Fatalf("expected verified report: %#v", report)
	}
	for _, row := range report.Cases {
		if !row.CertificateAccepted || !row.Equivalent {
			t.Fatalf("case did not round-trip: %#v", row)
		}
		if row.OriginalProjectionSHA256 != row.RoundTripProjectionSHA256 {
			t.Fatalf("projection hashes differ for %s: %#v", row.CaseID, row)
		}
	}
}

func TestNegativeControlsFailForNonEquivalentOrInvalidInputs(t *testing.T) {
	root := repoRoot(t)
	report, err := RunSuite(filepath.Join(root, "specs/verdict-exchange/v1"), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.NegativeControls) < 3 {
		t.Fatalf("expected at least three negative controls, got %d", len(report.NegativeControls))
	}
	for _, control := range report.NegativeControls {
		if !control.Passed {
			t.Fatalf("negative control did not pass: %#v", control)
		}
	}
}

func TestProjectionUsesPLCILegalAnalyzerIDs(t *testing.T) {
	projection, err := ParseNativeFile(filepath.Join(repoRoot(t), "specs/verdict-exchange/v1/analyzers/strong-migrations/drop-email-column.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got := projection.CertificateID(); got != "strong-migrations.drop-email-column.v1" {
		t.Fatalf("unexpected certificate id %q", got)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	for {
		if exists(filepath.Join(dir, "go.mod")) {
			return dir
		}
		next := filepath.Dir(dir)
		if next == dir {
			t.Fatal("could not find repo root")
		}
		dir = next
	}
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
