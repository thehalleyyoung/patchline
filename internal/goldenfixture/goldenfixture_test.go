package goldenfixture

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGenerateCreatesMinimalDeterministicGoTest(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "db/migrate/001_backfill.sql", "UPDATE accounts SET status = 'active';\nDELETE FROM account_events;\n")
	writeTestFile(t, root, "db/migrate/002_safe.sql", "CREATE TABLE accounts(id integer);\n")
	writeTestFile(t, root, "README.md", "not selected\n")
	out := filepath.Join(t.TempDir(), "golden")
	report, err := Generate(context.Background(), Options{Path: root, OutDir: out, MaxFiles: 2, MaxFileBytes: 4096, MaxTotalBytes: 8192})
	if err != nil {
		t.Fatal(err)
	}
	if report.Version != Version || report.Hash == "" {
		t.Fatalf("unexpected report metadata: %#v", report)
	}
	if report.Summary.SelectedFiles == 0 || report.Summary.SelectedFiles > 2 {
		t.Fatalf("unexpected selected file count: %#v", report.Summary)
	}
	if report.Expectations.RankedRisks == 0 || report.Expectations.FilesScanned != report.Summary.SelectedFiles {
		t.Fatalf("unexpected expectations: %#v summary=%#v", report.Expectations, report.Summary)
	}
	for _, rel := range []string{"generated_golden_test.go", "go.mod", "golden-fixture.json", "golden-fixture.md"} {
		if stat, err := os.Stat(filepath.Join(out, rel)); err != nil || stat.Size() == 0 {
			t.Fatalf("expected %s to be written, stat=%#v err=%v", rel, stat, err)
		}
	}
	cmd := exec.Command("go", "test", ".")
	cmd.Dir = out
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated test failed: %v\n%s", err, string(output))
	}
}

func writeTestFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
