package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/thehalleyyoung/patchline/internal/attest"
	"github.com/thehalleyyoung/patchline/internal/canonical"
	"github.com/thehalleyyoung/patchline/internal/certconformance"
	"github.com/thehalleyyoung/patchline/internal/certdiff"
	"github.com/thehalleyyoung/patchline/internal/certlang"
	"github.com/thehalleyyoung/patchline/internal/certplugfest"
	"github.com/thehalleyyoung/patchline/internal/certrevocation"
)

func main() {
	root := flag.String("root", ".", "repository root")
	out := flag.String("out", "results/generated/certificate-lifecycle-fixture", "output directory")
	flag.Parse()
	if err := run(*root, *out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(root string, out string) error {
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	safeRel := "specs/certificate-interchange/v1/vectors/valid/patchline-cli-safe.plci"
	proofRel := "specs/certificate-interchange/v1/vectors/valid/patchline-proof-frame.plci"
	corpusRel := "specs/certificate-conformance/v1/corpus.json"
	safe, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(safeRel)))
	if err != nil {
		return err
	}
	proof, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(proofRel)))
	if err != nil {
		return err
	}
	weakened := strings.Replace(string(proof), "obl.frame kind=frame status=checked", "obl.frame kind=frame status=assumed", 1)
	weakened = recomputeCanonicalHash(weakened)
	weakenedPath := filepath.Join(out, "weakened-proof-frame.plci")
	if err := os.WriteFile(weakenedPath, []byte(weakened), 0o644); err != nil {
		return err
	}
	opts := certlang.Options{Root: root, VerifyFiles: true}
	known, err := certrevocation.KnownFromBytes(safe, opts)
	if err != nil {
		return err
	}
	seed, err := attest.SeedFromHex(strings.Repeat("05", 32))
	if err != nil {
		return err
	}
	record, err := certrevocation.Create("revoke", safe, nil, "lifecycle fixture revokes stale CLI certificate", "2026-06-02T00:04:00Z", opts, seed)
	if err != nil {
		return err
	}
	entries, err := certrevocation.AppendRecord(nil, record)
	if err != nil {
		return err
	}
	bundle := certrevocation.Bundle{
		Version:           certrevocation.BundleVersion,
		KnownCertificates: []certrevocation.KnownCertificate{known},
		Ledger:            entries,
		Records:           []certrevocation.Record{record},
	}
	if err := writeJSON(filepath.Join(out, "revocation-bundle.json"), bundle); err != nil {
		return err
	}
	conformance, err := certconformance.Verify(filepath.Join(root, filepath.FromSlash(corpusRel)), root)
	if err != nil {
		return err
	}
	corpusHash, err := fileSHA256(filepath.Join(root, filepath.FromSlash(corpusRel)))
	if err != nil {
		return err
	}
	normalized, err := certlang.Normalize(proof, opts)
	if err != nil {
		return err
	}
	diff, err := certdiff.CompareBytes(proof, []byte(weakened), opts)
	if err != nil {
		return err
	}
	empty := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	submission := certplugfest.Submission{
		Version:                 certplugfest.SubmissionVersion,
		ToolName:                "patchline-go-fixture",
		ToolVersion:             "1.0.0",
		Offline:                 true,
		CorpusPath:              corpusRel,
		CorpusSHA256:            corpusHash,
		ConformanceReportHash:   canonical.Hash(conformance),
		NormalizeCertificate:    proofRel,
		NormalizedCanonicalHash: normalized.NormalizedCanonicalSHA256,
		DiffOldCertificate:      proofRel,
		DiffNewCertificate:      weakenedPath,
		DiffSummary:             diff.Summary,
		KnownCertificates:       []certrevocation.KnownCertificate{known},
		RevocationLedger:        entries,
		RevocationRecords:       []certrevocation.Record{record},
		Logs: []certplugfest.Log{{
			Command:      "patchline cert plugfest --submission plugfest-submission.json --root . --json",
			ExitCode:     0,
			StdoutSHA256: empty,
			StderrSHA256: empty,
		}},
	}
	return writeJSON(filepath.Join(out, "plugfest-submission.json"), submission)
}

func writeJSON(path string, value any) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return canonical.WriteJSON(file, value)
}

func fileSHA256(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func recomputeCanonicalHash(s string) string {
	lines := strings.Split(strings.TrimSuffix(s, "\n"), "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "canonical-sha256: ") {
			canonicalText := strings.Join(lines[:i], "\n") + "\n"
			sum := sha256.Sum256([]byte(canonicalText))
			lines[i] = "canonical-sha256: " + hex.EncodeToString(sum[:])
			return strings.Join(lines, "\n") + "\n"
		}
	}
	return s
}
