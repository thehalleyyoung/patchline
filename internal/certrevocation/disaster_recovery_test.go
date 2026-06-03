package certrevocation

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thehalleyyoung/patchline/internal/attest"
	"github.com/thehalleyyoung/patchline/internal/certlang"
)

func TestReplayDynamicDisasterRecoveryCertificateLog(t *testing.T) {
	root := repoRoot(t)
	certData := dynamicRecoveryCertificate(t, root, "docs/disaster-recovery-exercise.md")
	opts := certlang.Options{Root: root, VerifyFiles: true}
	known, err := KnownFromBytes(certData, opts)
	if err != nil {
		t.Fatal(err)
	}
	seed, err := attest.SeedFromHex(strings.Repeat("06", 32))
	if err != nil {
		t.Fatal(err)
	}
	record, err := Create("revoke", certData, nil, "disaster recovery mirror replay", "2026-06-03T12:34:00Z", opts, seed)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := AppendRecord(nil, record)
	if err != nil {
		t.Fatal(err)
	}
	report, err := Replay(entries, []Record{record}, []KnownCertificate{known})
	if err != nil {
		t.Fatal(err)
	}
	if !report.AllOK || report.Records != 1 || report.Revoked != 1 || report.Checkpoint.TipHash == "" {
		t.Fatalf("unexpected dynamic recovery replay report: %#v", report)
	}
	record.CertificateSHA256 = strings.Repeat("0", 64)
	if _, err := Replay(entries, []Record{record}, []KnownCertificate{known}); err == nil {
		t.Fatal("expected tampered disaster recovery record to fail replay")
	}
}

func dynamicRecoveryCertificate(t *testing.T, root string, rel string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	sha := hex.EncodeToString(sum[:])
	lines := []string{
		"PLCI/1",
		"certificate-id: patchline.disaster.recovery.test.v1",
		"subject-repo: thehalleyoung/patchline",
		"subject-ref: disaster-recovery-test",
		"subject-path: " + rel,
		"subject-sha256: " + sha,
		"issued-at: 2026-06-03T12:30:00Z",
		"producer: patchline-disaster-recovery/1",
		"verdict: safe",
		"risk-bps: 30",
		"evidence: ev.dr type=source uri=file:" + rel + " sha256=" + sha,
		"obligation: obl.restore kind=evidence status=checked evidence=ev.dr formula=\"certificate log replay survives mirror disaster recovery\"",
	}
	canonicalText := strings.Join(lines, "\n") + "\n"
	canonicalSum := sha256.Sum256([]byte(canonicalText))
	lines = append(lines, "canonical-sha256: "+hex.EncodeToString(canonicalSum[:]), "END")
	return []byte(strings.Join(lines, "\n") + "\n")
}
