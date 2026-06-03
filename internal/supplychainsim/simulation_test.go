package supplychainsim

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildReportVerifiesCompromiseSimulations(t *testing.T) {
	root, spec := testSimulationSpec(t)
	report, err := BuildReport(spec, root)
	if err != nil {
		t.Fatalf("BuildReport failed: %v", err)
	}
	if !report.OK {
		t.Fatalf("expected compromise simulations to pass with rejected/quarantined attacks, got %#v", report.Counterexamples)
	}
	if report.Summary.DependencyPoisoning != 1 || report.Summary.MaliciousArchives != 1 || report.Summary.ForgedReleaseMetadata != 1 {
		t.Fatalf("unexpected attack-kind summary: %#v", report.Summary)
	}
	if report.Summary.AttackSignals < 15 || report.Summary.DependencySignals == 0 || report.Summary.ArchiveSignals == 0 || report.Summary.ReleaseMetadataSignals == 0 {
		t.Fatalf("expected concrete signals across all attack families, got %#v", report.Summary)
	}
	if report.Summary.DetectedAttacks != 3 || report.Summary.RejectedAttacks != 3 || report.Summary.QuarantinedAttacks != 3 {
		t.Fatalf("expected all attacks to be detected/rejected/quarantined, got %#v", report.Summary)
	}
	repeat, err := BuildReport(spec, root)
	if err != nil {
		t.Fatalf("repeat BuildReport failed: %v", err)
	}
	if report.Hash == "" || report.Hash != repeat.Hash {
		t.Fatalf("expected deterministic report hash, got %q then %q", report.Hash, repeat.Hash)
	}
	markdown := RenderMarkdown(report)
	for _, phrase := range []string{"Supply-chain compromise simulations", "Dependency poisoning", "Forged release metadata"} {
		if !strings.Contains(markdown, phrase) {
			t.Fatalf("markdown missing %q:\n%s", phrase, markdown)
		}
	}
}

func TestBuildReportRejectsUnmitigatedCompromises(t *testing.T) {
	root, spec := testSimulationSpec(t)
	for i := range spec.Simulations {
		spec.Simulations[i].Detected = false
		spec.Simulations[i].Rejected = false
		spec.Simulations[i].Quarantined = false
	}
	spec.Simulations[0].Dependency.SignaturePath = ""
	spec.Simulations[1].Archive.SignaturePath = ""
	spec.Simulations[2].ReleaseMetadata.SignaturePath = ""
	spec.Simulations[2].ReleaseMetadata.CertificateLogPath = ""
	report, err := BuildReport(spec, root)
	if err != nil {
		t.Fatalf("BuildReport failed: %v", err)
	}
	if report.OK {
		t.Fatalf("expected unmitigated compromise simulations to fail: %#v", report)
	}
	for _, kind := range []string{
		"dependency_source_mismatch_not_detected",
		"dependency_hash_mismatch_not_rejected",
		"dependency_lockfile_hash_mismatch_not_quarantined",
		"missing_dependency_signature",
		"unallowlisted_transitive_dependency_not_rejected",
		"archive_entry_path_escape_not_detected",
		"archive_symlink_escape_not_rejected",
		"archive_unexpected_executable_payload_not_quarantined",
		"missing_archive_signature",
		"release_metadata_digest_mismatch_not_detected",
		"release_manifest_hash_mismatch_not_rejected",
		"missing_release_signature",
		"missing_release_certificate_log",
		"release_ref_mismatch_not_quarantined",
	} {
		if !hasCounterexample(report, kind) {
			t.Fatalf("expected %s counterexample, got %#v", kind, report.Counterexamples)
		}
	}
}

func TestReadSpecRejectsUnknownFields(t *testing.T) {
	_, err := ReadSpec(strings.NewReader(`{"version":"patchline.supply-chain-compromise-simulation/v1","name":"x","criteria":{},"simulations":[],"extra":true}`))
	if err == nil {
		t.Fatal("expected unknown field error")
	}
}

func TestWriteArtifactsIsDeterministic(t *testing.T) {
	root, spec := testSimulationSpec(t)
	report, err := BuildReport(spec, root)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "supply-chain-sim")
	if err := WriteArtifacts(out, report); err != nil {
		t.Fatal(err)
	}
	var reread Report
	file, err := os.Open(filepath.Join(out, "supply-chain-sim.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := json.NewDecoder(file).Decode(&reread); err != nil {
		t.Fatal(err)
	}
	if reread.Hash != report.Hash {
		t.Fatalf("report hash changed after write/read: got %s want %s", reread.Hash, report.Hash)
	}
	if stat, err := os.Stat(filepath.Join(out, "supply-chain-sim.md")); err != nil || stat.Size() == 0 {
		t.Fatalf("expected markdown artifact, stat=%#v err=%v", stat, err)
	}
}

func testSimulationSpec(t *testing.T) (string, Spec) {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"evidence/simulation.md":         "all supply-chain compromise simulations are rejected and quarantined\n",
		"evidence/dependency.md":         "dependency source mismatch and lockfile drift detection evidence\n",
		"evidence/dependency-allow.md":   "allowed dependency signer and transitive allowlist evidence\n",
		"evidence/archive.md":            "archive traversal and symlink escape quarantine evidence\n",
		"evidence/release.md":            "release metadata digest and ref mismatch rejection evidence\n",
		"dependency/go.mod":              "module example.com/service\nrequire github.com/patchline/core v1.0.0\n",
		"dependency/go.sum":              "github.com/patchline/core v1.0.0 h1:trusted\n",
		"dependency/core-1.0.0.tgz":      "poisoned dependency payload\n",
		"dependency/core-1.0.0.sig":      "signature by mallory\n",
		"archives/toolkit.tar":           "archive with unsafe entries represented in manifest\n",
		"archives/toolkit.tar.sig":       "signature by mallory\n",
		"release/patchline-1.0.0.tar.gz": "forged release archive bytes\n",
		"release/metadata.json":          `{"version":"1.0.1","ref":"refs/tags/v1.0.1","sha256":"sha256:1111111111111111111111111111111111111111111111111111111111111111"}` + "\n",
		"release/metadata.sig":           "metadata signature by mallory\n",
		"release/certificate-log.jsonl":  `{"release":"patchline-1.0.0","signer":"release-root"}` + "\n",
	}
	for path, contents := range files {
		writeTestFile(t, root, path, contents)
	}
	badHash := "sha256:0000000000000000000000000000000000000000000000000000000000000000"
	return root, Spec{
		Version: SpecVersion,
		Name:    "unit test supply-chain compromise simulations",
		Claim:   "Patchline proves dependency poisoning, malicious archive, and forged release metadata simulations are detected, rejected, quarantined, and backed by evidence hashes.",
		Criteria: Criteria{
			RequiredAttackKinds:              []string{"dependency_poisoning", "malicious_archive", "forged_release_metadata"},
			MinSimulationsPerKind:            1,
			RequireDetection:                 true,
			RequireRejection:                 true,
			RequireQuarantine:                true,
			RequireEvidenceHashes:            true,
			RequireDependencyLockIntegrity:   true,
			RequireArchiveEntrySafety:        true,
			RequireReleaseMetadataIntegrity:  true,
			RequireSignatureOrCertificateLog: true,
		},
		Simulations: []Simulation{
			{
				ID:              "dependency-typosquat",
				Kind:            "dependency_poisoning",
				Target:          "github.com/patchline/core",
				AttackGoal:      "replace a pinned module with a typosquat registry source and unallowlisted transitive payload",
				ExpectedOutcome: "rejected",
				Detected:        true,
				Rejected:        true,
				Quarantined:     true,
				Dependency: &DependencySimulation{
					PackageName:            "github.com/patch1ine/core",
					ExpectedPackageName:    "github.com/patchline/core",
					Version:                "v1.0.0",
					Source:                 "https://registry.example.invalid/patch1ine/core",
					ExpectedSource:         "https://proxy.golang.org/github.com/patchline/core",
					ManifestPath:           "dependency/go.mod",
					LockfilePath:           "dependency/go.sum",
					ExpectedLockfileSHA256: badHash,
					PackagePath:            "dependency/core-1.0.0.tgz",
					ExpectedPackageSHA256:  badHash,
					SignaturePath:          "dependency/core-1.0.0.sig",
					SignerID:               "mallory",
					AllowedSigners:         []string{"release-root"},
					Transitive:             true,
					TransitiveAllowlisted:  false,
					AllowlistEvidencePaths: []string{"evidence/dependency-allow.md"},
				},
				EvidencePaths: []string{"evidence/dependency.md"},
			},
			{
				ID:              "archive-traversal",
				Kind:            "malicious_archive",
				Target:          "patchline-toolkit.tar",
				AttackGoal:      "smuggle traversal, symlink escape, and executable payload entries into a release archive",
				ExpectedOutcome: "rejected",
				Detected:        true,
				Rejected:        true,
				Quarantined:     true,
				Archive: &ArchiveSimulation{
					ArchivePath:           "archives/toolkit.tar",
					ExpectedArchiveSHA256: badHash,
					SignaturePath:         "archives/toolkit.tar.sig",
					SignerID:              "mallory",
					AllowedSigners:        []string{"release-root"},
					QuarantinePath:        "quarantine/archives/toolkit.tar",
					Entries: []ArchiveEntry{
						{Path: "../scripts/postinstall.sh", Kind: "file", SHA256: hashLiteral("payload-a"), ExpectedSHA256: hashLiteral("payload-b"), Executable: true},
						{Path: `..\scripts\postinstall.ps1`, Kind: "file"},
						{Path: "safe/link", Kind: "symlink", LinkTarget: "../../.ssh/config"},
						{Path: "safe/windows-link", Kind: "symlink", LinkTarget: `C:\Users\runner\.ssh\config`},
					},
				},
				EvidencePaths: []string{"evidence/archive.md"},
			},
			{
				ID:              "release-metadata-forgery",
				Kind:            "forged_release_metadata",
				Target:          "patchline release metadata",
				AttackGoal:      "publish a stale tag and forged digest under an unapproved signer",
				ExpectedOutcome: "rejected",
				Detected:        true,
				Rejected:        true,
				Quarantined:     true,
				ReleaseMetadata: &ReleaseMetadataSimulation{
					ReleaseID:              "patchline-1.0.0",
					Version:                "1.0.1",
					ExpectedVersion:        "1.0.0",
					Ref:                    "refs/tags/v1.0.1",
					ExpectedRef:            "refs/tags/v1.0.0",
					Commit:                 "deadbeef",
					ExpectedCommit:         "0123456789abcdef0123456789abcdef01234567",
					ArtifactPath:           "release/patchline-1.0.0.tar.gz",
					ExpectedArtifactSHA256: badHash,
					MetadataArtifactSHA256: hashLiteral("forged-metadata"),
					ManifestPath:           "release/metadata.json",
					ExpectedManifestSHA256: badHash,
					SignaturePath:          "release/metadata.sig",
					SignerID:               "mallory",
					AllowedSigners:         []string{"release-root"},
					CertificateLogPath:     "release/certificate-log.jsonl",
				},
				EvidencePaths: []string{"evidence/release.md"},
			},
		},
		EvidencePaths: []string{"evidence/simulation.md"},
	}
}

func writeTestFile(t *testing.T, root, rel, contents string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func hashLiteral(value string) string {
	sum := sha256.Sum256([]byte(value))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func hasCounterexample(report Report, kind string) bool {
	for _, counterexample := range report.Counterexamples {
		if counterexample.Kind == kind {
			return true
		}
	}
	return false
}
