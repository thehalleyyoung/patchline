package hardwaresigning

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildReportVerifiesHardwareBackedSigning(t *testing.T) {
	root, spec := testHardwareSigningSpec(t)
	report, err := BuildReport(spec, root)
	if err != nil {
		t.Fatalf("BuildReport failed: %v", err)
	}
	if !report.OK {
		t.Fatalf("expected clean hardware-signing report, got counterexamples %#v", report.Counterexamples)
	}
	if report.Summary.SigningIdentities != 3 || report.Summary.HardwareBackedIdentities != 3 || report.Summary.OfflineRoots != 1 {
		t.Fatalf("unexpected signing identity summary: %#v", report.Summary)
	}
	if report.Summary.SignedArtifacts != 3 || report.Summary.RequiredArtifactKindsMet != 3 || report.Summary.ThresholdApprovedArtifacts != 3 {
		t.Fatalf("expected release/gate/certificate threshold artifacts, got %#v", report.Summary)
	}
	if report.Summary.KeyRotationDrills != 1 || report.Summary.RecoveryDrills != 1 || report.Summary.RevocationDrills != 1 {
		t.Fatalf("expected all signing drills, got %#v", report.Summary)
	}
	if report.Hash == "" {
		t.Fatal("expected deterministic report hash")
	}
	repeat, err := BuildReport(spec, root)
	if err != nil {
		t.Fatalf("repeat BuildReport failed: %v", err)
	}
	if repeat.Hash != report.Hash {
		t.Fatalf("expected deterministic hash, got %s then %s", report.Hash, repeat.Hash)
	}
	markdown := RenderMarkdown(report)
	if !strings.Contains(markdown, "Hardware-backed signing") || !strings.Contains(markdown, "Signed artifacts") {
		t.Fatalf("expected useful markdown, got:\n%s", markdown)
	}
}

func TestBuildReportRejectsSigningRegressions(t *testing.T) {
	root, spec := testHardwareSigningSpec(t)
	spec.SigningIdentities = spec.SigningIdentities[:2]
	spec.SigningIdentities[0].HardwareType = "software-file"
	spec.SigningIdentities[0].OfflineRoot = false
	spec.SigningIdentities[0].AttestationPath = "missing/release-root.attestation.json"
	spec.SigningIdentities[0].RecoverySharePaths = []string{"missing/release-root.share"}
	spec.SignedArtifacts = spec.SignedArtifacts[:2]
	spec.SignedArtifacts[0].SHA256 = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
	spec.SignedArtifacts[0].SignaturePath = "missing/release.sig"
	spec.SignedArtifacts[0].SignerIDs = []string{"release-root", "missing-signer"}
	spec.SignedArtifacts[0].Threshold = 3
	spec.SignedArtifacts[0].CertificateLogPath = "missing/release-log.jsonl"
	spec.SignedArtifacts[0].GateReportPath = "missing/release-gate.json"
	spec.Drills = spec.Drills[:1]
	spec.Drills[0].EvidencePaths = []string{"missing/key-rotation.md"}
	spec.Drills[0].ResultPaths = []string{"missing/key-rotation-result.json"}

	report, err := BuildReport(spec, root)
	if err != nil {
		t.Fatalf("BuildReport failed: %v", err)
	}
	if report.OK {
		t.Fatalf("expected deficient hardware signing to fail: %#v", report)
	}
	for _, kind := range []string{
		"insufficient_signing_identities",
		"missing_offline_root",
		"signer_not_hardware_backed",
		"missing_attestation",
		"missing_recovery_share",
		"missing_artifact_kind",
		"artifact_hash_mismatch",
		"missing_signature",
		"unknown_signer",
		"threshold_not_met",
		"missing_certificate_log",
		"missing_gate_report",
		"drill_missing_evidence",
		"drill_missing_result",
		"missing_drill_kind",
	} {
		if !hasCounterexample(report, kind) {
			t.Fatalf("expected %s counterexample, got %#v", kind, report.Counterexamples)
		}
	}
}

func TestReadSpecRejectsUnknownFields(t *testing.T) {
	_, err := ReadSpec(strings.NewReader(`{"version":"patchline.hardware-signing/v1","name":"x","criteria":{"required_artifact_kinds":["release"],"min_signing_identities":1,"min_artifacts_per_kind":1},"signing_identities":[],"signed_artifacts":[],"drills":[],"extra":true}`))
	if err == nil {
		t.Fatal("expected unknown field error")
	}
}

func TestWriteArtifactsIsDeterministic(t *testing.T) {
	root, spec := testHardwareSigningSpec(t)
	report, err := BuildReport(spec, root)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "hardware-signing")
	if err := WriteArtifacts(out, report); err != nil {
		t.Fatal(err)
	}
	var reread Report
	file, err := os.Open(filepath.Join(out, "hardware-signing.json"))
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
	if stat, err := os.Stat(filepath.Join(out, "hardware-signing.md")); err != nil || stat.Size() == 0 {
		t.Fatalf("expected markdown artifact, stat=%#v err=%v", stat, err)
	}
}

func testHardwareSigningSpec(t *testing.T) (string, Spec) {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"evidence/signing-policy.md":           "Hardware-backed release, gate, and certificate signing policy.\n",
		"evidence/release-window.md":           "Release signing ceremony approved by two maintainers.\n",
		"evidence/gate-window.md":              "Gate report signing ceremony approved by reproducibility owner.\n",
		"evidence/certificate-window.md":       "Certificate ledger signing ceremony approved by standards owner.\n",
		"artifacts/patchline-1.0.0.tar.gz":     "patchline release archive bytes\n",
		"artifacts/release-gate.json":          `{"gate":"release-smoke","ok":true}` + "\n",
		"artifacts/certificate-ledger.plci":    "plci certificate ledger entry\n",
		"signatures/release.sig":               "hardware signature for release archive\n",
		"signatures/gate.sig":                  "hardware signature for gate report\n",
		"signatures/certificate.sig":           "hardware signature for certificate ledger\n",
		"keys/release-root.pub":                "release root public key\n",
		"keys/gate-signer.pub":                 "gate signer public key\n",
		"keys/certificate-signer.pub":          "certificate signer public key\n",
		"attestations/release-root.json":       `{"device":"yubikey","slot":"9c"}` + "\n",
		"attestations/gate-signer.json":        `{"device":"hsm","slot":"gate"}` + "\n",
		"attestations/certificate-signer.json": `{"device":"kms-hsm","slot":"cert"}` + "\n",
		"recovery/release-root-a.share":        "release root recovery share a\n",
		"recovery/gate-signer-a.share":         "gate signer recovery share a\n",
		"recovery/certificate-signer-a.share":  "certificate signer recovery share a\n",
		"logs/release-log.jsonl":               `{"artifact":"patchline-release","signers":["release-root","gate-signer"]}` + "\n",
		"logs/gate-log.jsonl":                  `{"artifact":"release-gate","signers":["gate-signer","release-root"]}` + "\n",
		"logs/certificate-log.jsonl":           `{"artifact":"certificate-ledger","signers":["certificate-signer","release-root"]}` + "\n",
		"gate-reports/release-gate.json":       `{"gate":"release-checksum-gate","ok":true}` + "\n",
		"gate-reports/gate-gate.json":          `{"gate":"resilient-analysis-gate","ok":true}` + "\n",
		"gate-reports/certificate-gate.json":   `{"gate":"certificate-plugfest-gate","ok":true}` + "\n",
		"drills/key-rotation.md":               "Rotated release-root to gate-signer without invalidating old release evidence.\n",
		"drills/key-rotation-result.json":      `{"kind":"key-rotation","ok":true}` + "\n",
		"drills/recovery.md":                   "Recovered release signing quorum from sealed recovery share.\n",
		"drills/recovery-result.json":          `{"kind":"recovery","ok":true}` + "\n",
		"drills/revocation.md":                 "Revoked old gate signer and verified ledger rejection.\n",
		"drills/revocation-result.json":        `{"kind":"revocation","ok":true}` + "\n",
	}
	for path, contents := range files {
		writeSigningTestFile(t, root, path, contents)
	}
	return root, Spec{
		Version: SpecVersion,
		Name:    "test hardware-backed signing",
		Claim:   "Patchline verifies release, gate, and certificate artifacts signed by attested hardware identities with threshold approval, certificate logs, gate reports, recovery shares, and replayed key-rotation, recovery, and revocation drills.",
		Criteria: Criteria{
			RequiredArtifactKinds:    []string{"release", "gate", "certificate"},
			MinSigningIdentities:     3,
			MinArtifactsPerKind:      1,
			RequireHardwareBacking:   true,
			RequireAttestation:       true,
			RequireThresholdApproval: true,
			RequireKeyRotationDrill:  true,
			RequireRecoveryDrill:     true,
			RequireRevocationDrill:   true,
			RequireOfflineRoot:       true,
			RequireEvidenceHashes:    true,
		},
		SigningIdentities: []SigningIdentity{
			signingIdentity("release-root", "offline-root", "yubikey", "9c", true, "attestations/release-root.json", "keys/release-root.pub", []string{"recovery/release-root-a.share"}, "evidence/release-window.md"),
			signingIdentity("gate-signer", "gate-signer", "hsm", "gate-slot", false, "attestations/gate-signer.json", "keys/gate-signer.pub", []string{"recovery/gate-signer-a.share"}, "evidence/gate-window.md"),
			signingIdentity("certificate-signer", "certificate-signer", "kms-hsm", "cert-slot", false, "attestations/certificate-signer.json", "keys/certificate-signer.pub", []string{"recovery/certificate-signer-a.share"}, "evidence/certificate-window.md"),
		},
		SignedArtifacts: []SignedArtifact{
			signedArtifact(t, root, "patchline-release", "release", "artifacts/patchline-1.0.0.tar.gz", "signatures/release.sig", []string{"release-root", "gate-signer"}, 2, "logs/release-log.jsonl", "gate-reports/release-gate.json", "evidence/release-window.md"),
			signedArtifact(t, root, "release-gate", "gate", "artifacts/release-gate.json", "signatures/gate.sig", []string{"gate-signer", "release-root"}, 2, "logs/gate-log.jsonl", "gate-reports/gate-gate.json", "evidence/gate-window.md"),
			signedArtifact(t, root, "certificate-ledger", "certificate", "artifacts/certificate-ledger.plci", "signatures/certificate.sig", []string{"certificate-signer", "release-root"}, 2, "logs/certificate-log.jsonl", "gate-reports/certificate-gate.json", "evidence/certificate-window.md"),
		},
		Drills: []Drill{
			signingDrill("rotate-release-root", "key_rotation", "patchline-release", "release-root", "gate-signer", "drills/key-rotation.md", "drills/key-rotation-result.json"),
			signingDrill("recover-release-root", "recovery", "patchline-release", "release-root", "release-root", "drills/recovery.md", "drills/recovery-result.json"),
			signingDrill("revoke-gate-signer", "revocation", "release-gate", "gate-signer", "certificate-signer", "drills/revocation.md", "drills/revocation-result.json"),
		},
		EvidencePaths: []string{"evidence/signing-policy.md"},
	}
}

func signingIdentity(id, role, hardwareType, slot string, offlineRoot bool, attestation, publicKey string, shares []string, evidence string) SigningIdentity {
	return SigningIdentity{
		ID:                 id,
		Role:               role,
		HardwareType:       hardwareType,
		Slot:               slot,
		OfflineRoot:        offlineRoot,
		AttestationPath:    attestation,
		PublicKeyPath:      publicKey,
		RecoverySharePaths: shares,
		EvidencePaths:      []string{evidence},
	}
}

func signedArtifact(t *testing.T, root, id, kind, path, signature string, signers []string, threshold int, logPath, gatePath, evidence string) SignedArtifact {
	t.Helper()
	return SignedArtifact{
		ID:                 id,
		Kind:               kind,
		Path:               path,
		SHA256:             signingTestHash(t, root, path),
		SignaturePath:      signature,
		SignerIDs:          signers,
		Threshold:          threshold,
		CertificateLogPath: logPath,
		GateReportPath:     gatePath,
		EvidencePaths:      []string{evidence},
	}
}

func signingDrill(id, kind, artifactID, oldSigner, newSigner, evidence, result string) Drill {
	return Drill{
		ID:            id,
		Kind:          kind,
		ArtifactID:    artifactID,
		OldSignerID:   oldSigner,
		NewSignerID:   newSigner,
		Steps:         []string{"prepare ceremony", "sign with hardware key", "verify certificate log"},
		StartedAt:     "2026-06-03T10:00:00Z",
		CompletedAt:   "2026-06-03T10:15:00Z",
		EvidencePaths: []string{evidence},
		ResultPaths:   []string{result},
	}
}

func writeSigningTestFile(t *testing.T, root, rel, contents string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func signingTestHash(t *testing.T, root, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
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
