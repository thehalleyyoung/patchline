package certlang

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeCanonicalizesEquivalentWitnessOrder(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "specs/certificate-interchange/v1/vectors/valid/patchline-proof-frame.plci")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	shuffled := strings.Replace(
		string(data),
		"obligation: obl.frame kind=frame status=checked evidence=ev.proof formula=\"frame conditions are emitted with bounded table and column effects\"\nobligation: obl.external kind=evidence status=assumed evidence=ev.proof formula=\"database replay discharges external row existence obligations\"",
		"obligation: obl.external kind=evidence status=assumed evidence=ev.proof formula=\"database replay discharges external row existence obligations\"\nobligation: obl.frame kind=frame status=checked evidence=ev.proof formula=\"frame conditions are emitted with bounded table and column effects\"",
		1,
	)
	shuffled = recomputeCanonicalHash(shuffled)
	left, err := Normalize(data, Options{Root: root, VerifyFiles: true})
	if err != nil {
		t.Fatal(err)
	}
	right, err := Normalize([]byte(shuffled), Options{Root: root, VerifyFiles: true})
	if err != nil {
		t.Fatal(err)
	}
	if left.NormalizedCanonicalSHA256 != right.NormalizedCanonicalSHA256 {
		t.Fatalf("expected equivalent witnesses to normalize to same hash: %s != %s", left.NormalizedCanonicalSHA256, right.NormalizedCanonicalSHA256)
	}
	if !left.Changed && !right.Changed {
		t.Fatal("expected at least one equivalent ordering to report changed normalized bytes")
	}
}

func TestNormalizePreservesBackslashFormula(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "specs/certificate-interchange/v1/vectors/valid/patchline-cli-safe.plci")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	mutated := strings.Replace(
		string(data),
		`formula="command dispatch preserves explicit proof and certificate entry points"`,
		`formula="command dispatch preserves path\to proof witnesses"`,
		1,
	)
	mutated = recomputeCanonicalHash(mutated)
	first, err := Normalize([]byte(mutated), Options{Root: root, VerifyFiles: true})
	if err != nil {
		t.Fatal(err)
	}
	second, err := Normalize(first.Normalized, Options{Root: root, VerifyFiles: true})
	if err != nil {
		t.Fatal(err)
	}
	if first.NormalizedCanonicalSHA256 != second.NormalizedCanonicalSHA256 {
		t.Fatalf("expected normalization to be idempotent: %s != %s", first.NormalizedCanonicalSHA256, second.NormalizedCanonicalSHA256)
	}
	if !strings.Contains(string(first.Normalized), `formula="command dispatch preserves path\to proof witnesses"`) {
		t.Fatalf("normalized certificate escaped backslash unexpectedly:\n%s", string(first.Normalized))
	}
}
