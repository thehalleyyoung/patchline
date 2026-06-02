package certplugfest

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thehalleyyoung/patchline/internal/attest"
	"github.com/thehalleyyoung/patchline/internal/canonical"
	"github.com/thehalleyyoung/patchline/internal/certconformance"
	"github.com/thehalleyyoung/patchline/internal/certdiff"
	"github.com/thehalleyyoung/patchline/internal/certlang"
	"github.com/thehalleyyoung/patchline/internal/certrevocation"
)

func TestValidateOfflinePlugfestSubmission(t *testing.T) {
	root := repoRoot(t)
	submission := validSubmission(t, root)
	report, err := Validate(submission, root)
	if err != nil {
		t.Fatal(err)
	}
	if !report.AllOK || !report.ConformanceVerified || !report.NormalizationVerified || !report.DiffVerified || !report.RevocationVerified || !report.LogsVerified {
		t.Fatalf("expected fully verified plugfest report: %#v", report)
	}
}

func TestValidateRejectsNetworkCommandLog(t *testing.T) {
	root := repoRoot(t)
	submission := validSubmission(t, root)
	submission.Logs[0].Command = "curl https://example.invalid/corpus.json"
	report, err := Validate(submission, root)
	if err == nil {
		t.Fatal("expected network command to fail offline plugfest validation")
	}
	if report.LogsVerified {
		t.Fatalf("expected logs to be rejected: %#v", report)
	}
}

func TestValidateRejectsIncorrectDiffSummary(t *testing.T) {
	root := repoRoot(t)
	submission := validSubmission(t, root)
	submission.DiffSummary = map[string]int{"unchanged": 99}
	report, err := Validate(submission, root)
	if err == nil {
		t.Fatal("expected incorrect diff summary to fail")
	}
	if report.DiffVerified {
		t.Fatalf("expected diff to be rejected: %#v", report)
	}
}

func validSubmission(t *testing.T, root string) Submission {
	t.Helper()
	corpusPath := filepath.Join(root, "specs/certificate-conformance/v1/corpus.json")
	corpusHash, err := fileSHA256(corpusPath)
	if err != nil {
		t.Fatal(err)
	}
	conformance, err := certconformance.Verify(corpusPath, root)
	if err != nil {
		t.Fatal(err)
	}
	normalizePath := filepath.Join(root, "specs/certificate-interchange/v1/vectors/valid/patchline-proof-frame.plci")
	normalizeData, err := os.ReadFile(normalizePath)
	if err != nil {
		t.Fatal(err)
	}
	normalized, err := certlang.Normalize(normalizeData, certlang.Options{Root: root, VerifyFiles: true})
	if err != nil {
		t.Fatal(err)
	}
	weakened := strings.Replace(string(normalizeData), "obl.frame kind=frame status=checked", "obl.frame kind=frame status=assumed", 1)
	weakened = recomputeCanonicalHash(weakened)
	weakenedPath := filepath.Join(t.TempDir(), "weakened.plci")
	if err := os.WriteFile(weakenedPath, []byte(weakened), 0o644); err != nil {
		t.Fatal(err)
	}
	diff, err := certdiff.CompareBytes(normalizeData, []byte(weakened), certlang.Options{Root: root, VerifyFiles: true})
	if err != nil {
		t.Fatal(err)
	}
	seed, err := attest.SeedFromHex(strings.Repeat("04", 32))
	if err != nil {
		t.Fatal(err)
	}
	safePath := filepath.Join(root, "specs/certificate-interchange/v1/vectors/valid/patchline-cli-safe.plci")
	safeData, err := os.ReadFile(safePath)
	if err != nil {
		t.Fatal(err)
	}
	known, err := certrevocation.KnownFromBytes(safeData, certlang.Options{Root: root, VerifyFiles: true})
	if err != nil {
		t.Fatal(err)
	}
	record, err := certrevocation.Create("revoke", safeData, nil, "plugfest stale safe certificate proof", "2026-06-02T00:03:00Z", certlang.Options{Root: root, VerifyFiles: true}, seed)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := certrevocation.AppendRecord(nil, record)
	if err != nil {
		t.Fatal(err)
	}
	empty := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	return Submission{
		Version:                 SubmissionVersion,
		ToolName:                "external-fixture-checker",
		ToolVersion:             "1.0.0",
		Offline:                 true,
		CorpusPath:              corpusPath,
		CorpusSHA256:            corpusHash,
		ConformanceReportHash:   canonical.Hash(conformance),
		NormalizeCertificate:    normalizePath,
		NormalizedCanonicalHash: normalized.NormalizedCanonicalSHA256,
		DiffOldCertificate:      normalizePath,
		DiffNewCertificate:      weakenedPath,
		DiffSummary:             diff.Summary,
		KnownCertificates:       []certrevocation.KnownCertificate{known},
		RevocationLedger:        entries,
		RevocationRecords:       []certrevocation.Record{record},
		Logs: []Log{{
			Command:      "patchline cert plugfest --submission submission.json --json",
			ExitCode:     0,
			StdoutSHA256: empty,
			StderrSHA256: empty,
		}},
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

func recomputeCanonicalHash(s string) string {
	lines := strings.Split(strings.TrimSuffix(s, "\n"), "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "canonical-sha256: ") {
			canonicalText := strings.Join(lines[:i], "\n") + "\n"
			lines[i] = "canonical-sha256: " + sha256Hex([]byte(canonicalText))
			return strings.Join(lines, "\n") + "\n"
		}
	}
	return s
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
