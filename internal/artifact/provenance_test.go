package artifact

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateDemoBundleManifestRejectsMissingListedFile(t *testing.T) {
	root := t.TempDir()
	demoDir := filepath.Join(root, "results", "generated", "artifact-demo")
	if err := os.MkdirAll(demoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(demoDir, "stage.json"), []byte(`{"ok":true}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sha, bytes, err := sha256File(filepath.Join(demoDir, "stage.json"))
	if err != nil {
		t.Fatal(err)
	}
	manifest := demoBundleManifest{
		FileCount: 2,
		Files: []demoBundleFile{
			{Path: "stage.json", SHA256: sha, Bytes: bytes},
			{Path: "missing.json", SHA256: sha, Bytes: bytes},
		},
	}
	writeStudyJSON(t, demoDir, "bundle-manifest.json", manifest)

	_, err = validateDemoBundleManifest(root)
	if err == nil || !strings.Contains(err.Error(), "file_count") {
		t.Fatalf("expected demo bundle manifest mismatch, got %v", err)
	}
}

func TestValidateDemoBundleManifestRejectsUnexpectedFile(t *testing.T) {
	root := t.TempDir()
	demoDir := filepath.Join(root, "results", "generated", "artifact-demo")
	if err := os.MkdirAll(demoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(demoDir, "stage.json"), []byte(`{"ok":true}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(demoDir, "extra.json"), []byte(`{"extra":true}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sha, bytes, err := sha256File(filepath.Join(demoDir, "stage.json"))
	if err != nil {
		t.Fatal(err)
	}
	manifest := demoBundleManifest{
		FileCount: 1,
		Files:     []demoBundleFile{{Path: "stage.json", SHA256: sha, Bytes: bytes}},
	}
	writeStudyJSON(t, demoDir, "bundle-manifest.json", manifest)

	_, err = validateDemoBundleManifest(root)
	if err == nil || !strings.Contains(err.Error(), "on-disk stage files") {
		t.Fatalf("expected unexpected demo file mismatch, got %v", err)
	}
}

func TestValidatePublicCorpusFetchRehashesCacheFiles(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "examples", "public-corpus")
	downloadDir := filepath.Join(sourceDir, "downloads")
	reportDir := filepath.Join(root, "results", "generated", "public-corpus")
	if err := os.MkdirAll(downloadDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(reportDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cachePath := filepath.Join(downloadDir, "case.sql")
	if err := os.WriteFile(cachePath, []byte("select 1;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cacheSHA, _, err := sha256File(cachePath)
	if err != nil {
		t.Fatal(err)
	}
	manifest := publicCorpusSourceManifest{
		Version: "test",
		Name:    "test-corpus",
		Sources: []publicCorpusSourceRecord{{ID: "case", URL: "https://example.test/case.sql", SHA256: cacheSHA, Out: "case.sql"}},
	}
	writeStudyJSON(t, sourceDir, "sources.json", manifest)
	manifestSHA, _, err := sha256File(filepath.Join(sourceDir, "sources.json"))
	if err != nil {
		t.Fatal(err)
	}
	report := publicCorpusFetchReport{
		Version:              "test",
		SourceManifest:       "examples/public-corpus/sources.json",
		SourceManifestSHA256: manifestSHA,
		OutputDir:            "examples/public-corpus/downloads",
		Offline:              true,
		OK:                   true,
		Files: []publicCorpusFetchReportEntry{{
			ID:             "case",
			URL:            "https://example.test/case.sql",
			Path:           "examples/public-corpus/downloads/case.sql",
			Status:         "cached",
			ExpectedSHA256: cacheSHA,
			ActualSHA256:   cacheSHA,
			OK:             true,
		}},
	}
	writeStudyJSON(t, reportDir, "fetch-report.json", report)
	if err := os.WriteFile(cachePath, []byte("select 2;\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err = validatePublicCorpusFetch(root)
	if err == nil || !strings.Contains(err.Error(), "on-disk hash") {
		t.Fatalf("expected on-disk cache hash mismatch, got %v", err)
	}
}

func TestValidatePublicCorpusFetchRejectsReportControlledPath(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "examples", "public-corpus")
	downloadDir := filepath.Join(sourceDir, "downloads")
	reportDir := filepath.Join(root, "results", "generated", "public-corpus")
	if err := os.MkdirAll(downloadDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(reportDir, 0o755); err != nil {
		t.Fatal(err)
	}
	expectedPath := filepath.Join(downloadDir, "case.sql")
	if err := os.WriteFile(expectedPath, []byte("corrupt cache\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	attackerPath := filepath.Join(downloadDir, "attacker.sql")
	if err := os.WriteFile(attackerPath, []byte("select 1;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	attackerSHA, _, err := sha256File(attackerPath)
	if err != nil {
		t.Fatal(err)
	}
	manifest := publicCorpusSourceManifest{
		Version: "test",
		Name:    "test-corpus",
		Sources: []publicCorpusSourceRecord{{ID: "case", URL: "https://example.test/case.sql", SHA256: attackerSHA, Out: "case.sql"}},
	}
	writeStudyJSON(t, sourceDir, "sources.json", manifest)
	manifestSHA, _, err := sha256File(filepath.Join(sourceDir, "sources.json"))
	if err != nil {
		t.Fatal(err)
	}
	report := publicCorpusFetchReport{
		Version:              "test",
		SourceManifest:       "examples/public-corpus/sources.json",
		SourceManifestSHA256: manifestSHA,
		OutputDir:            "examples/public-corpus/downloads",
		Offline:              true,
		OK:                   true,
		Files: []publicCorpusFetchReportEntry{{
			ID:             "case",
			URL:            "https://example.test/case.sql",
			Path:           "examples/public-corpus/downloads/attacker.sql",
			Status:         "cached",
			ExpectedSHA256: attackerSHA,
			ActualSHA256:   attackerSHA,
			OK:             true,
		}},
	}
	writeStudyJSON(t, reportDir, "fetch-report.json", report)

	_, err = validatePublicCorpusFetch(root)
	if err == nil || !strings.Contains(err.Error(), "manifest-derived cache path") {
		t.Fatalf("expected report-controlled path rejection, got %v", err)
	}
}

func TestValidatePublicCorpusFetchRejectsExtraReportEntry(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "examples", "public-corpus")
	downloadDir := filepath.Join(sourceDir, "downloads")
	reportDir := filepath.Join(root, "results", "generated", "public-corpus")
	if err := os.MkdirAll(downloadDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(reportDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cachePath := filepath.Join(downloadDir, "case.sql")
	if err := os.WriteFile(cachePath, []byte("select 1;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cacheSHA, _, err := sha256File(cachePath)
	if err != nil {
		t.Fatal(err)
	}
	manifest := publicCorpusSourceManifest{
		Version: "test",
		Name:    "test-corpus",
		Sources: []publicCorpusSourceRecord{{ID: "case", URL: "https://example.test/case.sql", SHA256: cacheSHA, Out: "case.sql"}},
	}
	writeStudyJSON(t, sourceDir, "sources.json", manifest)
	manifestSHA, _, err := sha256File(filepath.Join(sourceDir, "sources.json"))
	if err != nil {
		t.Fatal(err)
	}
	report := publicCorpusFetchReport{
		Version:              "test",
		SourceManifest:       "examples/public-corpus/sources.json",
		SourceManifestSHA256: manifestSHA,
		OutputDir:            "examples/public-corpus/downloads",
		Offline:              true,
		OK:                   true,
		Files: []publicCorpusFetchReportEntry{
			{ID: "case", URL: "https://example.test/case.sql", Path: "examples/public-corpus/downloads/case.sql", Status: "cached", ExpectedSHA256: cacheSHA, ActualSHA256: cacheSHA, OK: true},
			{ID: "extra", URL: "https://example.test/extra.sql", Path: "examples/public-corpus/downloads/case.sql", Status: "cached", ExpectedSHA256: cacheSHA, ActualSHA256: cacheSHA, OK: true},
		},
	}
	writeStudyJSON(t, reportDir, "fetch-report.json", report)

	_, err = validatePublicCorpusFetch(root)
	if err == nil || !strings.Contains(err.Error(), "unexpected source") {
		t.Fatalf("expected extra report entry mismatch, got %v", err)
	}
}
