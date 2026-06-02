package certdiff

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thehalleyyoung/patchline/internal/certlang"
)

func TestCompareClassifiesConfidenceWeakening(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "specs/certificate-interchange/v1/vectors/valid/patchline-proof-frame.plci")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	weakened := strings.Replace(string(data), "obl.frame kind=frame status=checked", "obl.frame kind=frame status=assumed", 1)
	weakened = recomputeCanonicalHash(weakened)
	report, err := CompareBytes(data, []byte(weakened), certlang.Options{Root: root, VerifyFiles: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary["weakened"] != 1 || report.Summary["unchanged"] != 1 {
		t.Fatalf("expected one weakened and one unchanged obligation, got %#v", report.Summary)
	}
	change := changeByID(report, "obl.frame")
	if change.Change != "weakened" || !strings.Contains(change.Reason, "checked to assumed") {
		t.Fatalf("unexpected frame change: %#v", change)
	}
}

func TestCompareTreatsRefutedAsCounterexampleState(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "specs/certificate-conformance/v1/cases/blocked-grammar-drift/positive.plci")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	repaired := strings.Replace(string(data), "verdict: blocked", "verdict: safe", 1)
	repaired = strings.Replace(repaired, "risk-bps: 9200", "risk-bps: 40", 1)
	repaired = strings.Replace(repaired, "status=refuted", "status=checked", 1)
	repaired = recomputeCanonicalHash(repaired)
	report, err := CompareBytes(data, []byte(repaired), certlang.Options{Root: root, VerifyFiles: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary["repaired"] != 1 {
		t.Fatalf("expected repaired refuted-state transition, got %#v", report.Summary)
	}
	change := changeByID(report, "obl.grammar")
	if change.Change != "repaired" {
		t.Fatalf("refuted transition should not be collapsed into strengthened/weakened: %#v", change)
	}
}

func TestCompareMigratesLegacyCertificateBeforeDiffing(t *testing.T) {
	root := repoRoot(t)
	legacyPath := filepath.Join(root, "specs/certificate-interchange/v0/vectors/legacy-valid/legacy-cli-safe.plci")
	currentPath := filepath.Join(root, "specs/certificate-interchange/v1/vectors/valid/patchline-cli-safe.plci")
	legacy, err := os.ReadFile(legacyPath)
	if err != nil {
		t.Fatal(err)
	}
	current, err := os.ReadFile(currentPath)
	if err != nil {
		t.Fatal(err)
	}
	report, err := CompareBytes(legacy, current, certlang.Options{Root: root, VerifyFiles: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.OldVersion != certlang.LegacyVersion0 || report.NewVersion != certlang.Version {
		t.Fatalf("expected legacy-to-current versions, got old=%s new=%s", report.OldVersion, report.NewVersion)
	}
	if report.Summary["unchanged"] == 0 {
		t.Fatalf("expected migrated legacy obligation to compare against current witness: %#v", report.Summary)
	}
}

func changeByID(report Report, id string) ObligationChange {
	for _, change := range report.Obligations {
		if change.ID == id {
			return change
		}
	}
	return ObligationChange{}
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
			canonical := strings.Join(lines[:i], "\n") + "\n"
			lines[i] = "canonical-sha256: " + certlangHash([]byte(canonical))
			return strings.Join(lines, "\n") + "\n"
		}
	}
	return s
}

func certlangHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
