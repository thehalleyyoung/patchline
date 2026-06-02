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
