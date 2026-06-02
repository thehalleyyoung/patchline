package certrevocation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thehalleyyoung/patchline/internal/attest"
	"github.com/thehalleyyoung/patchline/internal/certlang"
	"github.com/thehalleyyoung/patchline/internal/ledger"
)

func TestReplaySignedRevocationAndSupersession(t *testing.T) {
	root := repoRoot(t)
	seed, err := attest.SeedFromHex(strings.Repeat("01", 32))
	if err != nil {
		t.Fatal(err)
	}
	safe := readFixture(t, root, "specs/certificate-interchange/v1/vectors/valid/patchline-cli-safe.plci")
	guarded := readFixture(t, root, "specs/certificate-interchange/v1/vectors/valid/patchline-proof-frame.plci")
	knownSafe, err := KnownFromBytes(safe, certlang.Options{Root: root, VerifyFiles: true})
	if err != nil {
		t.Fatal(err)
	}
	knownGuarded, err := KnownFromBytes(guarded, certlang.Options{Root: root, VerifyFiles: true})
	if err != nil {
		t.Fatal(err)
	}
	supersede, err := Create("supersede", guarded, safe, "proof-frame certificate replaced by broader CLI dispatch proof", "2026-06-02T00:01:00Z", certlang.Options{Root: root, VerifyFiles: true}, seed)
	if err != nil {
		t.Fatal(err)
	}
	revoke, err := Create("revoke", safe, nil, "safe CLI certificate withdrawn after standards-body audit", "2026-06-02T00:02:00Z", certlang.Options{Root: root, VerifyFiles: true}, seed)
	if err != nil {
		t.Fatal(err)
	}
	var entries []ledgerEntry
	entries = appendRecord(t, entries, supersede)
	entries = appendRecord(t, entries, revoke)
	report, err := Replay(entries, []Record{supersede, revoke}, []KnownCertificate{knownSafe, knownGuarded})
	if err != nil {
		t.Fatal(err)
	}
	if !report.AllOK || report.Superseded != 1 || report.Revoked != 1 || report.Active != 0 {
		t.Fatalf("unexpected replay report: %#v", report)
	}
}

func TestReplayRejectsTamperedSignedPayload(t *testing.T) {
	root := repoRoot(t)
	seed, err := attest.SeedFromHex(strings.Repeat("02", 32))
	if err != nil {
		t.Fatal(err)
	}
	safe := readFixture(t, root, "specs/certificate-interchange/v1/vectors/valid/patchline-cli-safe.plci")
	knownSafe, err := KnownFromBytes(safe, certlang.Options{Root: root, VerifyFiles: true})
	if err != nil {
		t.Fatal(err)
	}
	record, err := Create("revoke", safe, nil, "original reason", "2026-06-02T00:01:00Z", certlang.Options{Root: root, VerifyFiles: true}, seed)
	if err != nil {
		t.Fatal(err)
	}
	entries := appendRecord(t, nil, record)
	record.Reason = "tampered reason"
	if _, err := Replay(entries, []Record{record}, []KnownCertificate{knownSafe}); err == nil {
		t.Fatal("expected tampered record to fail replay")
	}
}

func TestReplayRejectsTerminalStateChange(t *testing.T) {
	root := repoRoot(t)
	seed, err := attest.SeedFromHex(strings.Repeat("03", 32))
	if err != nil {
		t.Fatal(err)
	}
	safe := readFixture(t, root, "specs/certificate-interchange/v1/vectors/valid/patchline-cli-safe.plci")
	knownSafe, err := KnownFromBytes(safe, certlang.Options{Root: root, VerifyFiles: true})
	if err != nil {
		t.Fatal(err)
	}
	first, err := Create("revoke", safe, nil, "first revocation", "2026-06-02T00:01:00Z", certlang.Options{Root: root, VerifyFiles: true}, seed)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Create("revoke", safe, nil, "second revocation", "2026-06-02T00:02:00Z", certlang.Options{Root: root, VerifyFiles: true}, seed)
	if err != nil {
		t.Fatal(err)
	}
	var entries []ledgerEntry
	entries = appendRecord(t, entries, first)
	entries = appendRecord(t, entries, second)
	if _, err := Replay(entries, []Record{first, second}, []KnownCertificate{knownSafe}); err == nil {
		t.Fatal("expected second revocation to fail terminal-state replay")
	}
}

type ledgerEntry = ledger.Entry

func appendRecord(t *testing.T, entries []ledgerEntry, record Record) []ledgerEntry {
	t.Helper()
	next, err := AppendRecord(entries, record)
	if err != nil {
		t.Fatal(err)
	}
	return next
}

func readFixture(t *testing.T, root string, rel string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return data
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
