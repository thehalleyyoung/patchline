package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/thehalleyyoung/patchline/internal/certconformance"
	"github.com/thehalleyyoung/patchline/internal/certlang"
)

func TestDashboardPassesCleanCorpusCheckerReport(t *testing.T) {
	root := repoRoot(t)
	reportPath := writeGoCheckerReport(t, root)

	dashboard, err := buildDashboard(
		filepath.Join(root, "specs/certificate-conformance/v1/corpus.json"),
		root,
		[]checkerInput{{Name: "go", Path: reportPath}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !dashboard.AllOK {
		t.Fatalf("expected clean dashboard to pass: %#v", dashboard.DriftTotals)
	}
	if dashboard.DriftTotals.sum() != 0 {
		t.Fatalf("expected no drift, got %#v", dashboard.DriftTotals)
	}
	if dashboard.TotalCases != 4 || dashboard.SignedReferencesVerified != 4 {
		t.Fatalf("expected all signed corpus cases to be verified: %#v", dashboard)
	}
}

func TestDashboardFlagsCanonicalDrift(t *testing.T) {
	root := repoRoot(t)
	reportPath := writeGoCheckerReport(t, root)

	var report checkerReport
	if err := readJSON(reportPath, &report); err != nil {
		t.Fatal(err)
	}
	for i := range report.Vectors {
		if report.Vectors[i].Expected == "valid" && report.Vectors[i].Accepted {
			report.Vectors[i].CanonicalSHA256 = "0000000000000000000000000000000000000000000000000000000000000000"
			break
		}
	}
	tamperedPath := filepath.Join(filepath.Dir(reportPath), "go-tampered.json")
	writeJSON(t, tamperedPath, report)

	dashboard, err := buildDashboard(
		filepath.Join(root, "specs/certificate-conformance/v1/corpus.json"),
		root,
		[]checkerInput{{Name: "go", Path: tamperedPath}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if dashboard.AllOK {
		t.Fatal("expected tampered canonical hash to fail the dashboard")
	}
	if dashboard.DriftTotals.CanonicalSHA256 != 1 {
		t.Fatalf("expected one canonical drift, got %#v", dashboard.DriftTotals)
	}
}

func TestMinimizerWritesSinglePositiveCertificateWitness(t *testing.T) {
	root := repoRoot(t)
	reportPath := writeGoCheckerReport(t, root)

	var report checkerReport
	if err := readJSON(reportPath, &report); err != nil {
		t.Fatal(err)
	}
	for i := range report.Vectors {
		if report.Vectors[i].Path == "valid/safe-cli-dispatch.plci" {
			report.Vectors[i].CanonicalSHA256 = "0000000000000000000000000000000000000000000000000000000000000000"
			break
		}
	}
	tamperedPath := filepath.Join(filepath.Dir(reportPath), "go-canonical-drift.json")
	writeJSON(t, tamperedPath, report)

	corpus := filepath.Join(root, "specs/certificate-conformance/v1/corpus.json")
	dashboard, err := buildDashboard(corpus, root, []checkerInput{{Name: "go", Path: tamperedPath}})
	if err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(t.TempDir(), "witness")
	witness, err := minimizeFailure(corpus, dashboard, []checkerInput{{Name: "go", Path: tamperedPath}}, outDir)
	if err != nil {
		t.Fatal(err)
	}
	if witness.Status != "minimized" || witness.CaseID != "safe-cli-dispatch" || witness.DriftKind != "canonical_sha256" || witness.VectorKind != "positive" {
		t.Fatalf("unexpected witness: %#v", witness)
	}
	if witness.WitnessPath != "witness.plci" || witness.WitnessSHA256 == "" {
		t.Fatalf("expected a single copied certificate witness: %#v", witness)
	}
	got, err := os.ReadFile(filepath.Join(outDir, witness.WitnessPath))
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join(report.SpecDir, "vectors/valid/safe-cli-dispatch.plci"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("witness.plci does not equal the checker vector")
	}
	if witness.Reference.CanonicalSHA256 == witness.Observed.CanonicalSHA256 {
		t.Fatalf("expected witness to preserve canonical delta: %#v", witness)
	}
}

func TestMinimizerReportsCleanDashboardWithoutWitness(t *testing.T) {
	root := repoRoot(t)
	reportPath := writeGoCheckerReport(t, root)
	corpus := filepath.Join(root, "specs/certificate-conformance/v1/corpus.json")
	dashboard, err := buildDashboard(corpus, root, []checkerInput{{Name: "go", Path: reportPath}})
	if err != nil {
		t.Fatal(err)
	}
	witness, err := minimizeFailure(corpus, dashboard, []checkerInput{{Name: "go", Path: reportPath}}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if witness.Status != "no_failure" || !witness.AllOK || witness.WitnessPath != "" {
		t.Fatalf("expected clean no_failure witness, got %#v", witness)
	}
}

func TestMinimizerCanWitnessExtraVectorFromRawReport(t *testing.T) {
	root := repoRoot(t)
	reportPath := writeGoCheckerReport(t, root)

	var report checkerReport
	if err := readJSON(reportPath, &report); err != nil {
		t.Fatal(err)
	}
	extraPath := filepath.Join(report.SpecDir, "vectors/valid/zzz-extra.plci")
	copyFile(t, filepath.Join(report.SpecDir, "vectors/valid/safe-cli-dispatch.plci"), extraPath)
	report.Vectors = append(report.Vectors, checkerVector{
		Path:            "valid/zzz-extra.plci",
		Expected:        "valid",
		Accepted:        true,
		OK:              true,
		CertificateID:   "safe-cli-dispatch",
		Verdict:         "safe",
		RiskBPS:         intPtr(25),
		CanonicalSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	})
	extraReportPath := filepath.Join(filepath.Dir(reportPath), "go-extra.json")
	writeJSON(t, extraReportPath, report)

	corpus := filepath.Join(root, "specs/certificate-conformance/v1/corpus.json")
	dashboard, err := buildDashboard(corpus, root, []checkerInput{{Name: "go", Path: extraReportPath}})
	if err != nil {
		t.Fatal(err)
	}
	if dashboard.DriftTotals.ExtraVector != 1 {
		t.Fatalf("expected extra vector drift, got %#v", dashboard.DriftTotals)
	}
	outDir := filepath.Join(t.TempDir(), "witness")
	witness, err := minimizeFailure(corpus, dashboard, []checkerInput{{Name: "go", Path: extraReportPath}}, outDir)
	if err != nil {
		t.Fatal(err)
	}
	if witness.DriftKind != "extra_vector" || witness.VectorPath != "valid/zzz-extra.plci" || witness.WitnessPath != "witness.plci" {
		t.Fatalf("expected minimized extra-vector certificate witness, got %#v", witness)
	}
}

func writeGoCheckerReport(t *testing.T, root string) string {
	t.Helper()
	tmp := t.TempDir()
	specDir := filepath.Join(tmp, "corpus-vectors")
	writeCorpusVectors(t, root, specDir)
	report, err := certlang.CheckDirectory(specDir, certlang.Options{Root: root, VerifyFiles: true})
	if err != nil {
		t.Fatal(err)
	}
	reportPath := filepath.Join(tmp, "go.json")
	writeJSON(t, reportPath, report)
	return reportPath
}

func writeCorpusVectors(t *testing.T, root string, specDir string) {
	t.Helper()
	corpusPath := filepath.Join(root, "specs/certificate-conformance/v1/corpus.json")
	var corpus certconformance.Corpus
	if err := readJSON(corpusPath, &corpus); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{"valid", "invalid"} {
		if err := os.MkdirAll(filepath.Join(specDir, "vectors", dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range corpus.Cases {
		copyCorpusCert(t, filepath.Dir(corpusPath), tc.Positive, filepath.Join(specDir, "vectors", "valid", tc.ID+".plci"))
		copyCorpusCert(t, filepath.Dir(corpusPath), tc.NegativeControl, filepath.Join(specDir, "vectors", "invalid", tc.ID+".plci"))
	}
}

func copyCorpusCert(t *testing.T, corpusDir string, rel string, dst string) {
	t.Helper()
	src, err := resolveCorpusPath(corpusDir, rel)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func copyFile(t *testing.T, src string, dst string) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func intPtr(value int) *int {
	return &value
}

func writeJSON(t *testing.T, filePath string, value any) {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func repoRoot(t *testing.T) string {
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
