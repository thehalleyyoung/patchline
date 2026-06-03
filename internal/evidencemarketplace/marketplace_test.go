package evidencemarketplace

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPublishRegistryPublishesCertificateBackedExamples(t *testing.T) {
	registry, root := validRegistry(t)
	report, err := PublishRegistry(registry, root)
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || report.Summary.Published != 1 || report.Summary.Rejected != 0 {
		t.Fatalf("unexpected report: %#v", report)
	}
	if report.Examples[0].CertificateSubjectHash != registry.Examples[0].Certificate.SubjectHash {
		t.Fatalf("certificate hash mismatch: %#v", report.Examples[0])
	}
	if report.Examples[0].EvidenceHash == "" || report.Hash == "" || report.RegistryHash == "" {
		t.Fatalf("expected stable hashes in report: %#v", report)
	}
	assertStableReportHash(t, registry, root, report.Hash)
}

func TestCertificateHashNormalizesSetLikeFields(t *testing.T) {
	registry, _ := validRegistry(t)
	example := registry.Examples[0]
	original := ExpectedSubjectHash(example)
	reordered := example
	reordered.Certificate.Obligations = []string{
		"reproducible-without-private-data",
		"license-cleared",
		"artifact-hashes-verified",
		"redaction-reviewed",
	}
	reordered.Artifacts = []Artifact{example.Artifacts[1], example.Artifacts[0]}
	if got := ExpectedSubjectHash(reordered); got != original {
		t.Fatalf("subject hash should ignore set-like ordering: got %s want %s", got, original)
	}
}

func TestRenderHTMLEscapesPublisherControlledStrings(t *testing.T) {
	registry, root := validRegistry(t)
	registry.Examples[0].Title = `<script>alert("x")</script>`
	registry.Examples[0].Certificate.SubjectHash = ExpectedSubjectHash(registry.Examples[0])
	report, err := PublishRegistry(registry, root)
	if err != nil {
		t.Fatal(err)
	}
	html, err := RenderHTML(report)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(html, "<script>alert") {
		t.Fatalf("html did not escape title:\n%s", html)
	}
	if !strings.Contains(html, "&lt;script&gt;") {
		t.Fatalf("expected escaped script marker in html:\n%s", html)
	}
}

func TestPublishRegistryRejectsInvalidExamples(t *testing.T) {
	tests := []struct {
		name string
		edit func(*Registry, string)
		want string
	}{
		{
			name: "duplicate id",
			edit: func(registry *Registry, root string) {
				registry.Examples = append(registry.Examples, registry.Examples[0])
			},
			want: "duplicate id",
		},
		{
			name: "bad license",
			edit: func(registry *Registry, root string) {
				registry.Examples[0].LicenseSPDX = "NOASSERTION"
				registry.Examples[0].Certificate.SubjectHash = ExpectedSubjectHash(registry.Examples[0])
			},
			want: "license_spdx",
		},
		{
			name: "empty consent",
			edit: func(registry *Registry, root string) {
				registry.Examples[0].Consent = ""
				registry.Examples[0].Certificate.SubjectHash = ExpectedSubjectHash(registry.Examples[0])
			},
			want: "consent",
		},
		{
			name: "unreviewed redaction",
			edit: func(registry *Registry, root string) {
				registry.Examples[0].Redaction.Reviewed = false
				registry.Examples[0].Certificate.SubjectHash = ExpectedSubjectHash(registry.Examples[0])
			},
			want: "redaction.redaction_reviewed",
		},
		{
			name: "raw data shared",
			edit: func(registry *Registry, root string) {
				registry.Examples[0].Redaction.RawDataShared = true
				registry.Examples[0].Certificate.SubjectHash = ExpectedSubjectHash(registry.Examples[0])
			},
			want: "redaction.raw_data_shared",
		},
		{
			name: "missing obligation",
			edit: func(registry *Registry, root string) {
				registry.Examples[0].Certificate.Obligations = []string{"redaction-reviewed", "license-cleared", "artifact-hashes-verified"}
				registry.Examples[0].Certificate.SubjectHash = ExpectedSubjectHash(registry.Examples[0])
			},
			want: "certificate missing obligation reproducible-without-private-data",
		},
		{
			name: "bad artifact hash",
			edit: func(registry *Registry, root string) {
				registry.Examples[0].Artifacts[0].SHA256 = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
				registry.Examples[0].Certificate.SubjectHash = ExpectedSubjectHash(registry.Examples[0])
			},
			want: "sha256 mismatch",
		},
		{
			name: "bad certificate hash",
			edit: func(registry *Registry, root string) {
				registry.Examples[0].Certificate.SubjectHash = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
			},
			want: "certificate.subject_hash mismatch",
		},
		{
			name: "private marker",
			edit: func(registry *Registry, root string) {
				writeFile(t, root, "artifacts/hazard.json", `{"note":"password=not-public"}`)
				registry.Examples[0].Artifacts[0].SHA256 = fileHash(t, filepath.Join(root, "artifacts/hazard.json"))
				registry.Examples[0].Certificate.SubjectHash = ExpectedSubjectHash(registry.Examples[0])
			},
			want: "private marker password=",
		},
		{
			name: "path escape",
			edit: func(registry *Registry, root string) {
				outside := filepath.Join(t.TempDir(), "outside.json")
				if err := os.WriteFile(outside, []byte(`{"redacted":true}`), 0o644); err != nil {
					t.Fatal(err)
				}
				link := filepath.Join(root, "artifacts", "outside.json")
				if err := os.Symlink(outside, link); err != nil {
					t.Skipf("symlink unavailable: %v", err)
				}
				registry.Examples[0].Artifacts[0].Path = "artifacts/outside.json"
				registry.Examples[0].Artifacts[0].SHA256 = fileHash(t, outside)
				registry.Examples[0].Certificate.SubjectHash = ExpectedSubjectHash(registry.Examples[0])
			},
			want: "escapes registry root",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry, root := validRegistry(t)
			tt.edit(&registry, root)
			report, err := PublishRegistry(registry, root)
			if err != nil {
				t.Fatal(err)
			}
			if report.OK {
				t.Fatalf("expected report to fail: %#v", report)
			}
			if !strings.Contains(strings.Join(rejectionReasons(report), "\n"), tt.want) {
				t.Fatalf("expected rejection containing %q, got %#v", tt.want, report.Rejected)
			}
		})
	}
}

func validRegistry(t *testing.T) (Registry, string) {
	t.Helper()
	root := t.TempDir()
	writeFile(t, root, "artifacts/hazard.json", `{
  "version": "patchline.redacted-hazard-example/v1",
  "finding": "redacted stable-risk for broad backfill without guard",
  "repo": "public/example",
  "evidence": [{"path": "db/migrate/20260101010101_backfill_accounts.sql", "line": 12, "snippet": "UPDATE <table> SET <column> = <redacted> WHERE <guard> IS NULL"}]
}
`)
	writeFile(t, root, "artifacts/certificate.json", `{
  "version": "patchline.redacted-certificate-witness/v1",
  "obligations": ["redaction-reviewed", "license-cleared", "artifact-hashes-verified", "reproducible-without-private-data"]
}
`)
	example := Example{
		ID:           "redacted-backfill-guard",
		Title:        "Redacted broad backfill missing a guard",
		Organization: "Example Maintainers",
		Ecosystem:    "rails",
		HazardClass:  "broad-backfill-without-guard",
		Source: Source{
			Host:    "github",
			Repo:    "public/example",
			Ref:     "refs/heads/main",
			Commit:  "0123456789abcdef0123456789abcdef01234567",
			Subpath: "db/migrate",
			URL:     "https://github.com/public/example/tree/0123456789abcdef0123456789abcdef01234567/db/migrate",
		},
		LicenseSPDX: "CC-BY-4.0",
		Consent:     "Example Maintainers approved publication of this redacted hazard example and certificate under the declared public license.",
		Redaction: Redaction{
			Reviewed:      true,
			RawDataShared: false,
			Method:        "identifiers, literals, owners, and row values replaced with stable placeholders",
			Fields:        []string{"customer identifiers", "owner names", "literal values"},
			Reviewer:      "artifact-review",
		},
		Artifacts: []Artifact{
			{Path: "artifacts/hazard.json", Role: "redacted-hazard-example", SHA256: fileHash(t, filepath.Join(root, "artifacts/hazard.json")), Redacted: true},
			{Path: "artifacts/certificate.json", Role: "certificate-witness", SHA256: fileHash(t, filepath.Join(root, "artifacts/certificate.json")), Redacted: true},
		},
		Certificate: Certificate{
			ID:       "cert-redacted-backfill-guard",
			Issuer:   "patchline-public-evidence-fixture",
			IssuedAt: "2026-06-02T21:22:11Z",
			Obligations: []string{
				"redaction-reviewed",
				"license-cleared",
				"artifact-hashes-verified",
				"reproducible-without-private-data",
			},
		},
		Reproduction: []string{
			"go run ./cmd/patchline evidence-marketplace publish --registry examples/evidence-marketplace/registry.json --out results/generated/evidence-marketplace --json",
			"jq -e '.ok == true' results/generated/evidence-marketplace/marketplace.json",
		},
		Limitations: []string{"The public example keeps code shape and certificate obligations but intentionally omits private table and owner names."},
	}
	example.Certificate.SubjectHash = ExpectedSubjectHash(example)
	registry := Registry{
		Version: RegistryVersion,
		Claim:   "The marketplace admits only redacted, certificate-backed hazard examples whose artifacts hash to declared values, whose licenses are explicit, and whose reproduction path does not require private data.",
		Marketplace: Metadata{
			Name:       "Patchline public evidence marketplace",
			Maintainer: "Patchline maintainers",
			PolicyURL:  "docs/evidence-marketplace.md",
		},
		Examples: []Example{example},
	}
	return registry, root
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func fileHash(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func rejectionReasons(report Report) []string {
	var out []string
	for _, rejected := range report.Rejected {
		out = append(out, rejected.Reasons...)
	}
	return out
}

func assertStableReportHash(t *testing.T, registry Registry, root string, want string) {
	t.Helper()
	next, err := PublishRegistry(registry, root)
	if err != nil {
		t.Fatal(err)
	}
	if next.Hash != want {
		t.Fatalf("report hash changed: got %s want %s", next.Hash, want)
	}
}
