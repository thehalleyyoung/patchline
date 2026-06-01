package artifact

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGeneratePaperTables(t *testing.T) {
	root := paperTablesTestRoot(t, true)
	report, err := GeneratePaperTables(root)
	if err != nil {
		t.Fatalf("GeneratePaperTables: %v", err)
	}
	if report.Version != PaperTablesVersion {
		t.Fatalf("version = %q, want %q", report.Version, PaperTablesVersion)
	}
	if len(report.Tables) != 5 {
		t.Fatalf("tables = %d, want 5", len(report.Tables))
	}
	if report.Hash == "" {
		t.Fatal("hash is empty")
	}
	if report.SourceHashes["public-archive"] == "" {
		t.Fatal("public archive source hash is missing")
	}
	if report.SourceHashes["public-archive-actual"] == "" || report.SourceHashes["public-archive-expected"] == "" {
		t.Fatal("public archive actual/expected source hashes are missing")
	}
	ids := map[string]bool{}
	for _, table := range report.Tables {
		ids[table.ID] = true
		if len(table.Columns) == 0 {
			t.Fatalf("%s has no columns", table.ID)
		}
		if len(table.Rows) == 0 {
			t.Fatalf("%s has no rows", table.ID)
		}
		for _, row := range table.Rows {
			if len(row) != len(table.Columns) {
				t.Fatalf("%s row has %d cells, want %d: %#v", table.ID, len(row), len(table.Columns), row)
			}
		}
	}
	for _, id := range []string{"table-1-corpus", "table-2-detection-actionability", "table-3-ablation", "table-4-historical-counterfactuals", "table-5-scale"} {
		if !ids[id] {
			t.Fatalf("missing table %s", id)
		}
	}
	if !strings.Contains(report.Markdown, "public-postmortem-derived") {
		t.Fatal("markdown does not state the public-postmortem-derived boundary")
	}
	if !strings.Contains(report.Markdown, "cannot_prove") {
		t.Fatal("markdown does not expose cannot_prove outcomes")
	}
}

func TestGeneratePaperTablesRequiresGeneratedBenchmarkReports(t *testing.T) {
	root := paperTablesTestRoot(t, false)
	report, err := RunBenchmarkManifest(filepath.Join("..", "..", "benchmarks", "manifests", "smoke.json"))
	if err != nil {
		t.Fatal(err)
	}
	report.DatasetID = "drifted-smoke"
	report.Hash = benchmarkRunHash(report)
	writeStudyJSON(t, filepath.Join(root, "benchmarks", "expected"), "smoke-report.json", report)

	_, err = GeneratePaperTables(root)
	if err == nil || !strings.Contains(err.Error(), "smoke-report.json") || !strings.Contains(err.Error(), "does not match expected") {
		t.Fatalf("expected generated/expected benchmark mismatch, got %v", err)
	}
}

func paperTablesTestRoot(t *testing.T, useCommittedExpected bool) string {
	t.Helper()
	root := t.TempDir()
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(repoRoot, "examples"), filepath.Join(root, "examples")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(repoRoot, "demos"), filepath.Join(root, "demos")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "benchmarks"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(repoRoot, "benchmarks", "ground_truth"), filepath.Join(root, "benchmarks", "ground_truth")); err != nil {
		t.Fatal(err)
	}
	if useCommittedExpected {
		if err := os.Symlink(filepath.Join(repoRoot, "benchmarks", "expected"), filepath.Join(root, "benchmarks", "expected")); err != nil {
			t.Fatal(err)
		}
	}
	writePaperTablesBenchmarkReports(t, root, !useCommittedExpected)
	return root
}

func writePaperTablesBenchmarkReports(t *testing.T, root string, writeExpected bool) {
	t.Helper()
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(cwd); err != nil {
			t.Fatal(err)
		}
	}()
	reports := map[string]string{
		"smoke":                "smoke-report.json",
		"negative":             "negative-report.json",
		"repair_cases":         "repair-cases-report.json",
		"semantic_regressions": "semantic-regressions-report.json",
		"public_migrations":    "public-migrations-report.json",
		"public_incidents":     "public-incidents-report.json",
		"public_repairs":       "public-repairs-report.json",
		"public_archive":       "public-archive-report.json",
	}
	for manifest, reportName := range reports {
		report, err := RunBenchmarkManifest(filepath.Join("benchmarks", "manifests", manifest+".json"))
		if err != nil {
			t.Fatal(err)
		}
		writeStudyJSON(t, filepath.Join(root, "results", "generated", "artifact-benchmark"), reportName, report)
		if writeExpected {
			writeStudyJSON(t, filepath.Join(root, "benchmarks", "expected"), reportName, report)
		}
	}
}
