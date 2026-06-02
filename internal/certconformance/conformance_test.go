package certconformance

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStandardsBodyCorpus(t *testing.T) {
	root := repoRoot(t)
	report, err := Verify(filepath.Join(root, "specs/certificate-conformance/v1/corpus.json"), root)
	if err != nil {
		t.Fatal(err)
	}
	if !report.AllOK {
		t.Fatalf("expected conformance corpus to pass: %#v", report)
	}
	if report.TotalCases != 4 {
		t.Fatalf("expected four standards-body cases, got %d", report.TotalCases)
	}
	if report.PositivesAccepted != report.TotalCases || report.NegativesRejected != report.TotalCases || report.ReferencesVerified != report.TotalCases {
		t.Fatalf("expected every case to have positive, negative, and signed reference proof: %#v", report)
	}
	seenVerdicts := map[string]bool{}
	for _, tc := range report.Cases {
		if !tc.OK {
			t.Fatalf("case %s did not pass: %#v", tc.ID, tc)
		}
		seenVerdicts[tc.Verdict] = true
	}
	for _, verdict := range []string{"safe", "guarded", "blocked", "unsupported"} {
		if !seenVerdicts[verdict] {
			t.Fatalf("missing verdict class %s in conformance corpus", verdict)
		}
	}
}

func TestRejectsTamperedReferenceSignature(t *testing.T) {
	root := repoRoot(t)
	tmp := t.TempDir()
	src := filepath.Join(root, "specs/certificate-conformance/v1")
	dst := filepath.Join(tmp, "v1")
	copyDir(t, src, dst)
	referencePath := filepath.Join(dst, "cases/safe-cli-dispatch/reference-output.json")
	data, err := os.ReadFile(referencePath)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.Replace(string(data), `"value": "7413`, `"value": "0413`, 1))
	if err := os.WriteFile(referencePath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = Verify(filepath.Join(dst, "corpus.json"), root)
	if err == nil {
		t.Fatal("expected tampered reference signature to fail")
	}
	if !strings.Contains(err.Error(), "signature") {
		t.Fatalf("expected signature failure, got %v", err)
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

func copyDir(t *testing.T, src string, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}
