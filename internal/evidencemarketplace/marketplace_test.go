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
	if report.Summary.PrevalenceExamples != 1 || report.Summary.DuplicateInflation != 0 {
		t.Fatalf("single unique example should count once for prevalence: %#v", report.Summary)
	}
	if len(report.ByHazardPrevalence) != 1 || report.ByHazardPrevalence[0].Count != 1 {
		t.Fatalf("unexpected prevalence hazard counts: %#v", report.ByHazardPrevalence)
	}
	if !report.Examples[0].DuplicateAnalysis.PrevalenceRepresentative || report.Examples[0].DuplicateAnalysis.PrevalenceWeight != 1 {
		t.Fatalf("unique example should be a prevalence representative: %#v", report.Examples[0].DuplicateAnalysis)
	}
	if report.Summary.PublicReleaseEligible != 1 || !report.Examples[0].ReleaseAdmission.PublicReleaseEligible {
		t.Fatalf("expected automated release admission to pass: %#v", report.Examples[0].ReleaseAdmission)
	}
	if report.Examples[0].GateReputation.Submitted || report.Examples[0].GateReputation.Score != 0 || report.Examples[0].GateReputation.Tier != "emerging" {
		t.Fatalf("omitted gate reputation should publish as zero-score emerging metadata: %#v", report.Examples[0].GateReputation)
	}
	assertStableReportHash(t, registry, root, report.Hash)
}

func TestPublishRegistryCollapsesExactAndNearDuplicatesForPrevalence(t *testing.T) {
	registry, root := validRegistry(t)

	exact := registry.Examples[0]
	exact.Artifacts = append([]Artifact(nil), exact.Artifacts...)
	exact.ID = "redacted-backfill-guard-resubmission"
	exact.Title = "Resubmitted redacted broad backfill missing a guard"
	exact.Certificate.ID = "cert-redacted-backfill-guard-resubmission"
	exact.Certificate.SubjectHash = ExpectedSubjectHash(exact)

	writeFile(t, root, "artifacts/hazard-near.json", `{
  "version": "patchline.redacted-hazard-example/v1",
  "finding": "second submitter described the same redacted backfill risk in different words",
  "repo": "public/example",
  "evidence": [{
    "path": "db/migrate/20260101010101_backfill_accounts.sql",
    "line": 99,
    "snippet": "UPDATE <table> SET <column> = <redacted> WHERE <guard> IS NULL"
  }],
  "review_note": "same public evidence cue, reformatted by a second submitter"
}
`)
	near := registry.Examples[0]
	near.Artifacts = append([]Artifact(nil), near.Artifacts...)
	near.ID = "redacted-backfill-guard-near-duplicate"
	near.Title = "Near-duplicate redacted broad backfill missing a guard"
	near.Artifacts[0].Path = "artifacts/hazard-near.json"
	near.Artifacts[0].SHA256 = fileHash(t, filepath.Join(root, "artifacts/hazard-near.json"))
	near.Certificate.ID = "cert-redacted-backfill-guard-near-duplicate"
	near.Certificate.SubjectHash = ExpectedSubjectHash(near)

	registry.Examples = []Example{registry.Examples[0], exact, near}
	report, err := PublishRegistry(registry, root)
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || report.Summary.Published != 3 || report.Summary.Rejected != 0 {
		t.Fatalf("unexpected duplicate publication report: %#v", report)
	}
	if report.Summary.PrevalenceExamples != 1 || report.Summary.DuplicateInflation != 2 {
		t.Fatalf("duplicates should collapse to one prevalence example: %#v", report.Summary)
	}
	if report.Summary.ExactDuplicateGroups != 1 || report.Summary.NearDuplicateGroups != 1 {
		t.Fatalf("expected one exact group and one near group: %#v", report.Summary)
	}
	if len(report.ByHazard) != 1 || report.ByHazard[0].Count != 3 {
		t.Fatalf("raw hazard count should preserve all submissions: %#v", report.ByHazard)
	}
	if len(report.ByHazardPrevalence) != 1 || report.ByHazardPrevalence[0].Count != 1 {
		t.Fatalf("prevalence hazard count should collapse duplicates: %#v", report.ByHazardPrevalence)
	}
	if len(report.DuplicateGroups) != 2 {
		t.Fatalf("expected exact and near duplicate groups, got %#v", report.DuplicateGroups)
	}

	representatives := 0
	weights := 0
	nearGroupIDs := map[string]bool{}
	for _, example := range report.Examples {
		analysis := example.DuplicateAnalysis
		if analysis.ExactFingerprint == "" || analysis.NearFingerprint == "" {
			t.Fatalf("missing duplicate fingerprints for %s: %#v", example.ID, analysis)
		}
		if analysis.NearGroupSize != 3 || analysis.PrevalenceGroupKind != "near" || analysis.PrevalenceGroupID == "" {
			t.Fatalf("expected all examples in a near prevalence group: %s %#v", example.ID, analysis)
		}
		nearGroupIDs[analysis.NearGroupID] = true
		if analysis.PrevalenceRepresentative {
			representatives++
		} else if analysis.DuplicateOf == "" || analysis.PrevalenceWeight != 0 {
			t.Fatalf("non-representative duplicate should point at representative with zero weight: %#v", analysis)
		}
		weights += analysis.PrevalenceWeight
	}
	if representatives != 1 || weights != 1 || len(nearGroupIDs) != 1 {
		t.Fatalf("unexpected representative/weight assignment: representatives=%d weights=%d groups=%v", representatives, weights, nearGroupIDs)
	}
	assertStableReportHash(t, registry, root, report.Hash)
}

func TestPublishRegistryComputesGateReputationOnlyFromAllowedSignals(t *testing.T) {
	registry, root := validRegistry(t)
	registry.Examples[0].GateReputation = &GateReputationInput{
		ReproducibleRuns: 12,
		FirstVerifiedAt:  "2025-01-01T00:00:00Z",
		LastVerifiedAt:   "2025-07-01T00:00:00Z",
		IndependentConfirmations: []string{
			"Migration Safety Working Group",
			"Independent Artifact Review Lab",
			"Database Reliability Guild",
		},
	}

	report, err := PublishRegistry(registry, root)
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || report.Summary.GateReputationSubmitted != 1 || report.Summary.GateReputationEstablished != 1 {
		t.Fatalf("unexpected reputation summary: %#v", report.Summary)
	}
	reputation := report.Examples[0].GateReputation
	if !reputation.Submitted || reputation.Score != 100 || reputation.Tier != "established" {
		t.Fatalf("unexpected reputation score: %#v", reputation)
	}
	if reputation.ReproducibilityPoints != 40 || reputation.LongevityPoints != 30 || reputation.ConfirmationPoints != 30 {
		t.Fatalf("unexpected reputation dimensions: %#v", reputation)
	}
	if reputation.VerifiedDays != 181 {
		t.Fatalf("longevity should be derived from supplied timestamps, got %d days", reputation.VerifiedDays)
	}
	if got := strings.Join(reputation.IndependentConfirmations, ","); got != "Database Reliability Guild,Independent Artifact Review Lab,Migration Safety Working Group" {
		t.Fatalf("confirmations should be normalized and sorted, got %q", got)
	}
	if report.Examples[0].CertificateSubjectHash != registry.Examples[0].Certificate.SubjectHash {
		t.Fatalf("mutable reputation must not change certificate subject hash: %#v", report.Examples[0])
	}
	assertStableReportHash(t, registry, root, report.Hash)

	changed := registry
	changed.Examples[0].Title = "Same gate reputation with a different title"
	changed.Examples[0].Organization = "Different Submitter"
	changed.Examples[0].Consent = "Different Submitter approved publication of this redacted hazard example under the declared public license."
	changed.Examples[0].Certificate.SubjectHash = ExpectedSubjectHash(changed.Examples[0])
	changedReport, err := PublishRegistry(changed, root)
	if err != nil {
		t.Fatal(err)
	}
	if changedReport.Examples[0].GateReputation.Score != reputation.Score {
		t.Fatalf("non-reputation metadata changed reputation score: got %d want %d", changedReport.Examples[0].GateReputation.Score, reputation.Score)
	}
}

func TestPublishChallengeTrackScoresAnalyzerBackedAdversarialMigrations(t *testing.T) {
	registry, root := validChallengeRegistry(t)
	report, err := PublishChallengeTrack(registry, root)
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || report.Summary.Accepted != 1 || report.Summary.ScoreboardEntries != 1 || report.Summary.Rejected != 0 {
		t.Fatalf("unexpected challenge report: %#v", report)
	}
	entry := report.Scoreboard[0]
	if entry.Score != 100 || entry.Tier != "gold" || !entry.ScoreboardEligible {
		t.Fatalf("expected full deterministic score, got %#v", entry)
	}
	if entry.MigrationAnalysis.HighRisk != 1 || !entry.MigrationAnalysis.AnalyzerMatched || entry.MigrationAnalysis.ActualBehavior != "flag-high-risk-migration" {
		t.Fatalf("challenge did not run the migration analyzer against the proof artifact: %#v", entry.MigrationAnalysis)
	}
	if entry.Breakdown.AnalyzerSignal != 40 || entry.Breakdown.ResponsibleDisclosure != 10 {
		t.Fatalf("unexpected score breakdown: %#v", entry.Breakdown)
	}
	if report.Hash == "" || report.Markdown == "" {
		t.Fatalf("expected tamper-evident rendered report: %#v", report)
	}
}

func TestPublishChallengeTrackRejectsEmbargoedPublicSubmissions(t *testing.T) {
	registry, root := validChallengeRegistry(t)
	registry.Examples[0].Challenge.Disclosure.Status = "embargoed"
	registry.Examples[0].Challenge.Disclosure.PublicReleaseAllowed = false
	registry.Examples[0].Certificate.SubjectHash = ExpectedSubjectHash(registry.Examples[0])
	report, err := PublishChallengeTrack(registry, root)
	if err != nil {
		t.Fatal(err)
	}
	if report.OK || report.Summary.Rejected != 1 || len(report.Scoreboard) != 0 {
		t.Fatalf("expected embargoed public challenge to be rejected: %#v", report)
	}
	var reasons []string
	for _, rejected := range report.Rejected {
		reasons = append(reasons, rejected.Reasons...)
	}
	joinedReasons := strings.Join(reasons, "\n")
	if !strings.Contains(joinedReasons, "disclosure.status must be public-safe") || !strings.Contains(joinedReasons, "public_release_allowed must be true") {
		t.Fatalf("expected responsible-disclosure rejection, got %#v", report.Rejected)
	}
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
			name: "consent missing submitter",
			edit: func(registry *Registry, root string) {
				registry.Examples[0].Consent = "A different group approved publication of this redacted hazard example under the declared public license."
				registry.Examples[0].Certificate.SubjectHash = ExpectedSubjectHash(registry.Examples[0])
			},
			want: "consent must name the submitting organization",
		},
		{
			name: "consent missing publication grant",
			edit: func(registry *Registry, root string) {
				registry.Examples[0].Consent = "Example Maintainers reviewed this redacted hazard example under the declared public license."
				registry.Examples[0].Certificate.SubjectHash = ExpectedSubjectHash(registry.Examples[0])
			},
			want: "consent must explicitly grant publication",
		},
		{
			name: "consent missing license",
			edit: func(registry *Registry, root string) {
				registry.Examples[0].Consent = "Example Maintainers approved publication of this redacted hazard example for public release."
				registry.Examples[0].Certificate.SubjectHash = ExpectedSubjectHash(registry.Examples[0])
			},
			want: "consent must reference the declared public license",
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
		{
			name: "bad reputation timestamp",
			edit: func(registry *Registry, root string) {
				registry.Examples[0].GateReputation = &GateReputationInput{
					ReproducibleRuns:         3,
					FirstVerifiedAt:          "not-a-time",
					LastVerifiedAt:           "2025-07-01T00:00:00Z",
					IndependentConfirmations: []string{"Independent Artifact Review Lab"},
				}
			},
			want: "gate_reputation.first_verified_at must be RFC3339",
		},
		{
			name: "reputation last before first",
			edit: func(registry *Registry, root string) {
				registry.Examples[0].GateReputation = &GateReputationInput{
					ReproducibleRuns:         3,
					FirstVerifiedAt:          "2025-07-01T00:00:00Z",
					LastVerifiedAt:           "2025-01-01T00:00:00Z",
					IndependentConfirmations: []string{"Independent Artifact Review Lab"},
				}
			},
			want: "gate_reputation.last_verified_at must not be before first_verified_at",
		},
		{
			name: "negative reputation runs",
			edit: func(registry *Registry, root string) {
				registry.Examples[0].GateReputation = &GateReputationInput{
					ReproducibleRuns:         -1,
					FirstVerifiedAt:          "2025-01-01T00:00:00Z",
					LastVerifiedAt:           "2025-07-01T00:00:00Z",
					IndependentConfirmations: []string{"Independent Artifact Review Lab"},
				}
			},
			want: "gate_reputation.reproducible_runs must be non-negative",
		},
		{
			name: "self confirmation",
			edit: func(registry *Registry, root string) {
				registry.Examples[0].GateReputation = &GateReputationInput{
					ReproducibleRuns:         3,
					FirstVerifiedAt:          "2025-01-01T00:00:00Z",
					LastVerifiedAt:           "2025-07-01T00:00:00Z",
					IndependentConfirmations: []string{" example maintainers "},
				}
			},
			want: "gate_reputation.independent_confirmations must not include the submitting organization",
		},
		{
			name: "duplicate confirmation",
			edit: func(registry *Registry, root string) {
				registry.Examples[0].GateReputation = &GateReputationInput{
					ReproducibleRuns: 3,
					FirstVerifiedAt:  "2025-01-01T00:00:00Z",
					LastVerifiedAt:   "2025-07-01T00:00:00Z",
					IndependentConfirmations: []string{
						"Independent Artifact Review Lab",
						" independent artifact review lab ",
					},
				}
			},
			want: "duplicate gate_reputation.independent_confirmations entry",
		},
		{
			name: "private marker in confirmation",
			edit: func(registry *Registry, root string) {
				registry.Examples[0].GateReputation = &GateReputationInput{
					ReproducibleRuns:         3,
					FirstVerifiedAt:          "2025-01-01T00:00:00Z",
					LastVerifiedAt:           "2025-07-01T00:00:00Z",
					IndependentConfirmations: []string{"token=not-public"},
				}
			},
			want: "metadata contains private marker token=",
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

func validChallengeRegistry(t *testing.T) (Registry, string) {
	t.Helper()
	registry, root := validRegistry(t)
	writeFile(t, root, "artifacts/adversarial.sql", "UPDATE accounts SET status = 'active';\n")
	example := registry.Examples[0]
	example.ID = "redacted-adversarial-broad-update"
	example.Title = "Redacted adversarial broad update"
	example.Artifacts = append(example.Artifacts, Artifact{
		Path:     "artifacts/adversarial.sql",
		Role:     "adversarial-migration",
		SHA256:   fileHash(t, filepath.Join(root, "artifacts/adversarial.sql")),
		Redacted: true,
	})
	example.Certificate.ID = "cert-redacted-adversarial-broad-update"
	example.Certificate.Obligations = append(example.Certificate.Obligations, "responsible-disclosure-cleared")
	example.Reproduction = []string{
		"go run ./cmd/patchline evidence-marketplace challenge --registry examples/evidence-marketplace/challenge-registry.json --out results/generated/adversarial-challenge --json",
		"jq -e '.summary.scoreboard_entries >= 1' results/generated/adversarial-challenge/challenge.json",
	}
	example.GateReputation = &GateReputationInput{
		ReproducibleRuns: 8,
		FirstVerifiedAt:  "2025-01-01T00:00:00Z",
		LastVerifiedAt:   "2025-07-01T00:00:00Z",
		IndependentConfirmations: []string{
			"Independent Artifact Review Lab",
			"Migration Safety Working Group",
		},
	}
	example.Challenge = &ChallengeSubmission{
		TrackID:                  "patchline-adversarial-migrations-2026",
		AdversarialGoal:          "Preserve a high-risk broad data rewrite in a tiny public-safe migration proof.",
		AttackSurface:            "SQL migration analyzer and public benchmark importer",
		ExpectedDetectorBehavior: "flag-high-risk-migration",
		MigrationArtifact:        "artifacts/adversarial.sql",
		MaxPublicProofLines:      4,
		NoveltyStatement:         "The proof isolates the update-without-row-key hazard into one redacted migration statement.",
		Disclosure: ChallengeDisclosureStatus{
			Status:               "public-safe",
			PublicReleaseAllowed: true,
			CoordinatedWith:      "Patchline Public Challenge Maintainers",
			ReportedAt:           "2026-06-02T21:00:00Z",
			FullExploitHash:      "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		},
	}
	example.Certificate.SubjectHash = ExpectedSubjectHash(example)
	registry.ChallengeTrack = &ChallengeTrack{
		ID:       "patchline-adversarial-migrations-2026",
		Name:     "Patchline public adversarial migration challenge",
		RulesURL: "docs/evidence-marketplace.md#adversarial-migration-challenge",
		ResponsibleDisclosure: ResponsibleDisclosurePolicy{
			Contact:                 "security@patchline.example",
			PolicyURL:               "docs/evidence-marketplace.md#responsible-disclosure-rules",
			EmbargoDays:             90,
			PublicSafeArtifactsOnly: true,
		},
		Scoring: ChallengeScoringPolicy{
			MinScoreboardScore: 75,
			Weights: ChallengeScoreWeights{
				AnalyzerSignal:        40,
				Reproducibility:       20,
				Minimization:          15,
				Novelty:               15,
				ResponsibleDisclosure: 10,
			},
		},
	}
	registry.Examples = []Example{example}
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
