package certlang

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGrammarABNFCoversParserRules(t *testing.T) {
	root := repoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "specs/certificate-interchange/v1/grammar.abnf"))
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateGrammar(data); err != nil {
		t.Fatal(err)
	}
}

func TestVectorsAcrossGrammar(t *testing.T) {
	root := repoRoot(t)
	report, err := CheckDirectory(filepath.Join(root, "specs/certificate-interchange/v1"), Options{
		Root:        root,
		VerifyFiles: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.TotalValid < 2 {
		t.Fatalf("expected at least two valid vectors, got %d", report.TotalValid)
	}
	if report.TotalInvalid < 4 {
		t.Fatalf("expected at least four invalid vectors, got %d", report.TotalInvalid)
	}
	if !report.AllOK {
		t.Fatalf("expected all vectors to match expectations: %#v", report.Vectors)
	}
}

func TestRejectsSafeVerdictWithAssumption(t *testing.T) {
	root := repoRoot(t)
	valid := filepath.Join(root, "specs/certificate-interchange/v1/vectors/valid/patchline-cli-safe.plci")
	data, err := os.ReadFile(valid)
	if err != nil {
		t.Fatal(err)
	}
	mutated := []byte(recomputeCanonicalHash(replaceOnce(string(data), "status=checked", "status=assumed")))
	if _, err := Parse(mutated, Options{Root: root, VerifyFiles: true}); err == nil {
		t.Fatalf("expected safe certificate with assumed obligation to be rejected")
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

func replaceOnce(s, old, new string) string {
	for i := 0; i+len(old) <= len(s); i++ {
		if s[i:i+len(old)] == old {
			return s[:i] + new + s[i+len(old):]
		}
	}
	return s
}

func recomputeCanonicalHash(s string) string {
	lines := strings.Split(strings.TrimSuffix(s, "\n"), "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "canonical-sha256: ") {
			canonical := strings.Join(lines[:i], "\n") + "\n"
			lines[i] = "canonical-sha256: " + sha256Hex([]byte(canonical))
			return strings.Join(lines, "\n") + "\n"
		}
	}
	return s
}
