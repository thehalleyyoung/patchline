package certlang

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLegacyCertificatesMigrateAndRemainCheckable(t *testing.T) {
	root := repoRoot(t)
	report, err := CheckMigrationDirectory(filepath.Join(root, "specs/certificate-interchange/v0"), Options{
		Root:        root,
		VerifyFiles: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !report.AllOK {
		t.Fatalf("expected legacy migration vectors to pass: %#v", report)
	}
	if report.TotalLegacyValid != 4 {
		t.Fatalf("expected four legacy valid vectors, got %d", report.TotalLegacyValid)
	}
	if report.TotalLegacyInvalid < 2 {
		t.Fatalf("expected at least two legacy invalid vectors, got %d", report.TotalLegacyInvalid)
	}
	for _, verdict := range []string{"safe", "guarded", "blocked", "unsupported"} {
		if report.Verdicts[verdict] == 0 {
			t.Fatalf("legacy migration did not cover verdict %q: %#v", verdict, report.Verdicts)
		}
	}
	for _, vector := range report.Vectors {
		if !vector.OK {
			t.Fatalf("migration vector failed: %#v", vector)
		}
	}
}

func TestLegacyMigrationPreservesVerdictAndEvidence(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "specs/certificate-interchange/v0/vectors/legacy-valid/legacy-proof-guarded.plci")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	result, err := MigrateToCurrent(data, Options{Root: root, VerifyFiles: true})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Fatal("expected legacy certificate to migrate")
	}
	if result.SourceVersion != LegacyVersion0 || result.TargetVersion != Version {
		t.Fatalf("unexpected migration versions: %#v", result)
	}
	if result.Verdict != "guarded" || result.RiskClass != "medium" || result.RiskBPS != 180 {
		t.Fatalf("unexpected migrated verdict/risk: %#v", result)
	}
	if result.SourceSHA256 == result.TargetSHA256 {
		t.Fatal("expected migrated certificate bytes to differ from legacy bytes")
	}
	if result.Certificate == nil {
		t.Fatal("expected parsed migrated certificate")
	}
	if got := len(result.Certificate.Obligations); got != 2 {
		t.Fatalf("expected both legacy obligations to be preserved, got %d", got)
	}
	if got := len(result.Certificate.Evidence); got != 1 {
		t.Fatalf("expected legacy evidence to be preserved, got %d", got)
	}
	if _, err := Parse(result.Migrated, Options{Root: root, VerifyFiles: true}); err != nil {
		t.Fatalf("migrated PLCI/1 certificate is not checkable: %v", err)
	}
}

func TestCurrentCertificateMigrationIsNoop(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "specs/certificate-interchange/v1/vectors/valid/patchline-cli-safe.plci")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	result, err := MigrateToCurrent(data, Options{Root: root, VerifyFiles: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Changed {
		t.Fatal("current PLCI/1 certificate should not be rewritten")
	}
	if result.SourceSHA256 != result.TargetSHA256 {
		t.Fatalf("current certificate hash changed: %#v", result)
	}
	if result.SourceCanonicalSHA256 != result.TargetCanonicalSHA256 {
		t.Fatalf("current canonical hash changed: %#v", result)
	}
}
