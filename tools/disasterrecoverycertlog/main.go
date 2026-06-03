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
	"github.com/thehalleyyoung/patchline/internal/certlang"
	"github.com/thehalleyyoung/patchline/internal/certrevocation"
)

func main() {
	root := flag.String("root", ".", "repository root")
	out := flag.String("out", "results/generated/disaster-recovery-certlog", "output directory")
	subject := flag.String("subject", "docs/disaster-recovery-exercise.md", "repository-relative evidence file")
	flag.Parse()
	if err := run(*root, *out, *subject); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(root string, out string, subjectRel string) error {
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	certData, err := certificateForFile(root, subjectRel)
	if err != nil {
		return err
	}
	certPath := filepath.Join(out, "disaster-recovery-certlog.plci")
	if err := os.WriteFile(certPath, certData, 0o644); err != nil {
		return err
	}
	opts := certlang.Options{Root: root, VerifyFiles: true}
	known, err := certrevocation.KnownFromBytes(certData, opts)
	if err != nil {
		return err
	}
	seed, err := attest.SeedFromHex(strings.Repeat("06", 32))
	if err != nil {
		return err
	}
	record, err := certrevocation.Create("revoke", certData, nil, "disaster recovery drill replays restored certificate log", "2026-06-03T12:34:00Z", opts, seed)
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
	report, err := certrevocation.ReplayBundle(bundle)
	if err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(out, "replay.json"), report); err != nil {
		return err
	}
	return writeJSON(filepath.Join(out, "generation.json"), map[string]any{
		"version":          "patchline.disaster-recovery-certificate-log-fixture/v1",
		"subject":          filepath.ToSlash(subjectRel),
		"certificate_id":   known.CertificateID,
		"canonical_sha256": known.CanonicalSHA256,
		"records":          report.Records,
		"tip_hash":         report.Checkpoint.TipHash,
	})
}

func certificateForFile(root string, rel string) ([]byte, error) {
	if rel == "" || strings.HasPrefix(rel, "/") || strings.Contains(rel, `\`) || strings.HasPrefix(filepath.Clean(rel), "..") {
		return nil, fmt.Errorf("invalid subject path %q", rel)
	}
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(data)
	sha := hex.EncodeToString(sum[:])
	lines := []string{
		"PLCI/1",
		"certificate-id: patchline.disaster.recovery.certlog.v1",
		"subject-repo: thehalleyyoung/patchline",
		"subject-ref: disaster-recovery-exercise",
		"subject-path: " + filepath.ToSlash(rel),
		"subject-sha256: " + sha,
		"issued-at: 2026-06-03T12:30:00Z",
		"producer: patchline-disaster-recovery/1",
		"verdict: safe",
		"risk-bps: 30",
		"evidence: ev.dr type=source uri=file:" + filepath.ToSlash(rel) + " sha256=" + sha,
		"obligation: obl.restore kind=evidence status=checked evidence=ev.dr formula=\"certificate log replay survives mirror disaster recovery\"",
	}
	canonicalText := strings.Join(lines, "\n") + "\n"
	canonicalSum := sha256.Sum256([]byte(canonicalText))
	lines = append(lines, "canonical-sha256: "+hex.EncodeToString(canonicalSum[:]), "END")
	return []byte(strings.Join(lines, "\n") + "\n"), nil
}

func writeJSON(path string, value any) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return canonical.WriteJSON(file, value)
}
