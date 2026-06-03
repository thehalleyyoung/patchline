package hardwaresigning

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/thehalleyyoung/patchline/internal/canonical"
)

const SpecVersion = "patchline.hardware-signing/v1"
const ReportVersion = "patchline.hardware-signing-report/v1"

type Spec struct {
	Version           string            `json:"version"`
	Name              string            `json:"name"`
	Claim             string            `json:"claim,omitempty"`
	Criteria          Criteria          `json:"criteria"`
	SigningIdentities []SigningIdentity `json:"signing_identities"`
	SignedArtifacts   []SignedArtifact  `json:"signed_artifacts"`
	Drills            []Drill           `json:"drills"`
	EvidencePaths     []string          `json:"evidence_paths,omitempty"`
}

type Criteria struct {
	RequiredArtifactKinds    []string `json:"required_artifact_kinds"`
	MinSigningIdentities     int      `json:"min_signing_identities"`
	MinArtifactsPerKind      int      `json:"min_artifacts_per_kind"`
	RequireHardwareBacking   bool     `json:"require_hardware_backing"`
	RequireAttestation       bool     `json:"require_attestation"`
	RequireThresholdApproval bool     `json:"require_threshold_approval"`
	RequireKeyRotationDrill  bool     `json:"require_key_rotation_drill"`
	RequireRecoveryDrill     bool     `json:"require_recovery_drill"`
	RequireRevocationDrill   bool     `json:"require_revocation_drill"`
	RequireOfflineRoot       bool     `json:"require_offline_root"`
	RequireEvidenceHashes    bool     `json:"require_evidence_hashes"`
}

type SigningIdentity struct {
	ID                 string   `json:"id"`
	Role               string   `json:"role"`
	HardwareType       string   `json:"hardware_type"`
	Slot               string   `json:"slot"`
	OfflineRoot        bool     `json:"offline_root"`
	AttestationPath    string   `json:"attestation_path"`
	PublicKeyPath      string   `json:"public_key_path"`
	RecoverySharePaths []string `json:"recovery_share_paths,omitempty"`
	EvidencePaths      []string `json:"evidence_paths,omitempty"`
}

type SignedArtifact struct {
	ID                 string   `json:"id"`
	Kind               string   `json:"kind"`
	Path               string   `json:"path"`
	SHA256             string   `json:"sha256"`
	SignaturePath      string   `json:"signature_path"`
	SignerIDs          []string `json:"signer_ids"`
	Threshold          int      `json:"threshold"`
	CertificateLogPath string   `json:"certificate_log_path"`
	GateReportPath     string   `json:"gate_report_path"`
	EvidencePaths      []string `json:"evidence_paths,omitempty"`
}

type Drill struct {
	ID            string   `json:"id"`
	Kind          string   `json:"kind"`
	ArtifactID    string   `json:"artifact_id"`
	OldSignerID   string   `json:"old_signer_id,omitempty"`
	NewSignerID   string   `json:"new_signer_id,omitempty"`
	Steps         []string `json:"steps"`
	StartedAt     string   `json:"started_at"`
	CompletedAt   string   `json:"completed_at"`
	EvidencePaths []string `json:"evidence_paths,omitempty"`
	ResultPaths   []string `json:"result_paths,omitempty"`
}

type Report struct {
	Version           string             `json:"version"`
	Name              string             `json:"name"`
	OK                bool               `json:"ok"`
	Criteria          Criteria           `json:"criteria"`
	Summary           Summary            `json:"summary"`
	Evidence          []ArtifactEvidence `json:"evidence,omitempty"`
	SigningIdentities []IdentityReport   `json:"signing_identities"`
	SignedArtifacts   []ArtifactReport   `json:"signed_artifacts"`
	Drills            []DrillReport      `json:"drills"`
	Counterexamples   []Counterexample   `json:"counterexamples,omitempty"`
	Hash              string             `json:"hash"`
}

type Summary struct {
	SigningIdentities          int `json:"signing_identities"`
	HardwareBackedIdentities   int `json:"hardware_backed_identities"`
	OfflineRoots               int `json:"offline_roots"`
	RecoveryShares             int `json:"recovery_shares"`
	SignedArtifacts            int `json:"signed_artifacts"`
	RequiredArtifactKinds      int `json:"required_artifact_kinds"`
	RequiredArtifactKindsMet   int `json:"required_artifact_kinds_met"`
	ThresholdApprovedArtifacts int `json:"threshold_approved_artifacts"`
	Signatures                 int `json:"signatures"`
	Attestations               int `json:"attestations"`
	CertificateLogs            int `json:"certificate_logs"`
	GateReports                int `json:"gate_reports"`
	Drills                     int `json:"drills"`
	KeyRotationDrills          int `json:"key_rotation_drills"`
	RecoveryDrills             int `json:"recovery_drills"`
	RevocationDrills           int `json:"revocation_drills"`
	EvidenceArtifacts          int `json:"evidence_artifacts"`
	Counterexamples            int `json:"counterexamples"`
}

type IdentityReport struct {
	ID             string             `json:"id"`
	Role           string             `json:"role"`
	HardwareType   string             `json:"hardware_type"`
	Slot           string             `json:"slot"`
	HardwareBacked bool               `json:"hardware_backed"`
	OfflineRoot    bool               `json:"offline_root"`
	Attestation    ArtifactEvidence   `json:"attestation,omitempty"`
	PublicKey      ArtifactEvidence   `json:"public_key,omitempty"`
	RecoveryShares []ArtifactEvidence `json:"recovery_shares,omitempty"`
	Evidence       []ArtifactEvidence `json:"evidence,omitempty"`
}

type ArtifactReport struct {
	ID             string             `json:"id"`
	Kind           string             `json:"kind"`
	Artifact       ArtifactEvidence   `json:"artifact"`
	ExpectedSHA256 string             `json:"expected_sha256"`
	Pinned         bool               `json:"pinned"`
	Signature      ArtifactEvidence   `json:"signature,omitempty"`
	Signers        []string           `json:"signers"`
	Threshold      int                `json:"threshold"`
	ThresholdMet   bool               `json:"threshold_met"`
	CertificateLog ArtifactEvidence   `json:"certificate_log,omitempty"`
	GateReport     ArtifactEvidence   `json:"gate_report,omitempty"`
	Evidence       []ArtifactEvidence `json:"evidence,omitempty"`
}

type DrillReport struct {
	ID          string             `json:"id"`
	Kind        string             `json:"kind"`
	ArtifactID  string             `json:"artifact_id"`
	OldSignerID string             `json:"old_signer_id,omitempty"`
	NewSignerID string             `json:"new_signer_id,omitempty"`
	Steps       int                `json:"steps"`
	StartedAt   string             `json:"started_at"`
	CompletedAt string             `json:"completed_at"`
	Complete    bool               `json:"complete"`
	Evidence    []ArtifactEvidence `json:"evidence,omitempty"`
	Results     []ArtifactEvidence `json:"results,omitempty"`
}

type ArtifactEvidence struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

type Counterexample struct {
	ID      string   `json:"id"`
	Kind    string   `json:"kind"`
	Subject string   `json:"subject,omitempty"`
	Message string   `json:"message"`
	Witness []string `json:"witness,omitempty"`
}

func ReadSpec(reader io.Reader) (Spec, error) {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var spec Spec
	if err := decoder.Decode(&spec); err != nil {
		return Spec{}, err
	}
	if spec.Version != SpecVersion {
		return Spec{}, fmt.Errorf("hardware signing spec version must be %s", SpecVersion)
	}
	return spec, nil
}

func BuildReport(spec Spec, root string) (Report, error) {
	if err := validateSpec(spec); err != nil {
		return Report{}, err
	}
	if root == "" {
		root = "."
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return Report{}, err
	}
	criteria := normalizeCriteria(spec.Criteria)
	report := Report{
		Version:  ReportVersion,
		Name:     spec.Name,
		OK:       true,
		Criteria: criteria,
		Summary: Summary{
			RequiredArtifactKinds: len(criteria.RequiredArtifactKinds),
		},
	}

	var counterexamples []Counterexample
	report.Evidence, counterexamples = collectArtifacts(rootAbs, spec.EvidencePaths, spec.Name, "run_evidence", "missing_evidence", "empty_evidence", "invalid_evidence_path", "hardware-signing evidence could not be read")
	report.Summary.EvidenceArtifacts += len(report.Evidence)
	if criteria.RequireEvidenceHashes && len(report.Evidence) == 0 {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "run." + stableID(spec.Name, "evidence") + ".missing",
			Kind:    "missing_evidence",
			Subject: spec.Name,
			Message: "hardware-signing spec does not cite readable run-level evidence",
		})
	}

	identityReports, identityCounterexamples, identities := buildIdentityReports(rootAbs, spec.SigningIdentities, criteria)
	report.SigningIdentities = identityReports
	counterexamples = append(counterexamples, identityCounterexamples...)
	report.Summary.SigningIdentities = len(identityReports)
	for _, identity := range identityReports {
		if identity.HardwareBacked {
			report.Summary.HardwareBackedIdentities++
		}
		if identity.OfflineRoot {
			report.Summary.OfflineRoots++
		}
		if identity.Attestation.Path != "" {
			report.Summary.Attestations++
			report.Summary.EvidenceArtifacts++
		}
		if identity.PublicKey.Path != "" {
			report.Summary.EvidenceArtifacts++
		}
		report.Summary.RecoveryShares += len(identity.RecoveryShares)
		report.Summary.EvidenceArtifacts += len(identity.RecoveryShares) + len(identity.Evidence)
	}

	artifactReports, artifactCounterexamples, artifacts := buildArtifactReports(rootAbs, spec.SignedArtifacts, identities, criteria)
	report.SignedArtifacts = artifactReports
	counterexamples = append(counterexamples, artifactCounterexamples...)
	report.Summary.SignedArtifacts = len(artifactReports)
	kinds := map[string]int{}
	for _, artifact := range artifactReports {
		kinds[artifact.Kind]++
		if artifact.ThresholdMet {
			report.Summary.ThresholdApprovedArtifacts++
		}
		if artifact.Signature.Path != "" {
			report.Summary.Signatures++
			report.Summary.EvidenceArtifacts++
		}
		if artifact.CertificateLog.Path != "" {
			report.Summary.CertificateLogs++
			report.Summary.EvidenceArtifacts++
		}
		if artifact.GateReport.Path != "" {
			report.Summary.GateReports++
			report.Summary.EvidenceArtifacts++
		}
		if artifact.Artifact.Path != "" {
			report.Summary.EvidenceArtifacts++
		}
		report.Summary.EvidenceArtifacts += len(artifact.Evidence)
	}
	for _, requiredKind := range criteria.RequiredArtifactKinds {
		if kinds[requiredKind] >= criteria.MinArtifactsPerKind {
			report.Summary.RequiredArtifactKindsMet++
			continue
		}
		counterexamples = append(counterexamples, Counterexample{
			ID:      "artifact-kind." + stableID(requiredKind, "coverage") + ".missing",
			Kind:    "missing_artifact_kind",
			Subject: requiredKind,
			Message: fmt.Sprintf("required artifact kind has %d artifacts below required %d", kinds[requiredKind], criteria.MinArtifactsPerKind),
			Witness: []string{requiredKind},
		})
		if kinds[requiredKind] > 0 {
			counterexamples = append(counterexamples, Counterexample{
				ID:      "artifact-kind." + stableID(requiredKind, "count") + ".insufficient",
				Kind:    "insufficient_artifacts_for_kind",
				Subject: requiredKind,
				Message: fmt.Sprintf("artifact kind %q does not meet the per-kind minimum", requiredKind),
				Witness: []string{fmt.Sprint(kinds[requiredKind]), fmt.Sprint(criteria.MinArtifactsPerKind)},
			})
		}
	}

	drillReports, drillCounterexamples := buildDrillReports(rootAbs, spec.Drills, identities, artifacts, criteria)
	report.Drills = drillReports
	counterexamples = append(counterexamples, drillCounterexamples...)
	report.Summary.Drills = len(drillReports)
	drillKinds := map[string]int{}
	for _, drill := range drillReports {
		drillKinds[drill.Kind]++
		switch drill.Kind {
		case "key-rotation":
			report.Summary.KeyRotationDrills++
		case "recovery":
			report.Summary.RecoveryDrills++
		case "revocation":
			report.Summary.RevocationDrills++
		}
		report.Summary.EvidenceArtifacts += len(drill.Evidence) + len(drill.Results)
	}
	if criteria.RequireKeyRotationDrill && drillKinds["key-rotation"] == 0 {
		counterexamples = append(counterexamples, missingDrillKindCounterexample("key-rotation"))
	}
	if criteria.RequireRecoveryDrill && drillKinds["recovery"] == 0 {
		counterexamples = append(counterexamples, missingDrillKindCounterexample("recovery"))
	}
	if criteria.RequireRevocationDrill && drillKinds["revocation"] == 0 {
		counterexamples = append(counterexamples, missingDrillKindCounterexample("revocation"))
	}

	if len(spec.SigningIdentities) < criteria.MinSigningIdentities {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "criteria.signing-identities.insufficient",
			Kind:    "insufficient_signing_identities",
			Message: fmt.Sprintf("signing identities %d below required %d", len(spec.SigningIdentities), criteria.MinSigningIdentities),
		})
	}
	if criteria.RequireOfflineRoot && report.Summary.OfflineRoots == 0 {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "criteria.offline-root.missing",
			Kind:    "missing_offline_root",
			Message: "hardware signing requires at least one identity marked as an offline root",
		})
	}

	sortCounterexamples(counterexamples)
	report.Counterexamples = counterexamples
	report.Summary.Counterexamples = len(counterexamples)
	report.OK = len(counterexamples) == 0
	report.Hash = reportHash(report)
	return report, nil
}

func WriteArtifacts(outDir string, report Report) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	file, err := os.Create(filepath.Join(outDir, "hardware-signing.json"))
	if err != nil {
		return err
	}
	if err := canonical.WriteJSON(file, report); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "hardware-signing.md"), []byte(RenderMarkdown(report)), 0o644)
}

func RenderMarkdown(report Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Hardware-backed signing\n\n")
	fmt.Fprintf(&b, "Patchline verifies release, gate, and certificate artifacts as hardware-backed, threshold-signed records with attested signing identities, certificate logs, gate reports, recovery shares, and replayed key-rotation, recovery, and revocation drills.\n\n")
	fmt.Fprintf(&b, "## Summary\n\n")
	fmt.Fprintf(&b, "| Metric | Value |\n| --- | ---: |\n")
	fmt.Fprintf(&b, "| OK | `%t` |\n", report.OK)
	fmt.Fprintf(&b, "| Signing identities | %d |\n", report.Summary.SigningIdentities)
	fmt.Fprintf(&b, "| Hardware-backed identities | %d |\n", report.Summary.HardwareBackedIdentities)
	fmt.Fprintf(&b, "| Offline roots | %d |\n", report.Summary.OfflineRoots)
	fmt.Fprintf(&b, "| Recovery shares | %d |\n", report.Summary.RecoveryShares)
	fmt.Fprintf(&b, "| Signed artifacts | %d |\n", report.Summary.SignedArtifacts)
	fmt.Fprintf(&b, "| Required artifact kinds met | %d / %d |\n", report.Summary.RequiredArtifactKindsMet, report.Summary.RequiredArtifactKinds)
	fmt.Fprintf(&b, "| Threshold-approved artifacts | %d |\n", report.Summary.ThresholdApprovedArtifacts)
	fmt.Fprintf(&b, "| Signatures | %d |\n", report.Summary.Signatures)
	fmt.Fprintf(&b, "| Attestations | %d |\n", report.Summary.Attestations)
	fmt.Fprintf(&b, "| Certificate logs | %d |\n", report.Summary.CertificateLogs)
	fmt.Fprintf(&b, "| Gate reports | %d |\n", report.Summary.GateReports)
	fmt.Fprintf(&b, "| Key-rotation drills | %d |\n", report.Summary.KeyRotationDrills)
	fmt.Fprintf(&b, "| Recovery drills | %d |\n", report.Summary.RecoveryDrills)
	fmt.Fprintf(&b, "| Revocation drills | %d |\n", report.Summary.RevocationDrills)
	fmt.Fprintf(&b, "| Evidence artifacts | %d |\n", report.Summary.EvidenceArtifacts)
	fmt.Fprintf(&b, "| Counterexamples | %d |\n\n", report.Summary.Counterexamples)

	fmt.Fprintf(&b, "## Signed artifacts\n\n")
	fmt.Fprintf(&b, "| Artifact | Kind | Pinned | Threshold | Signers | Certificate log | Gate report |\n| --- | --- | ---: | ---: | ---: | ---: | ---: |\n")
	for _, artifact := range report.SignedArtifacts {
		fmt.Fprintf(&b, "| `%s` | `%s` | `%t` | `%t` | %d | `%t` | `%t` |\n",
			escapeTable(artifact.ID),
			escapeTable(artifact.Kind),
			artifact.Pinned,
			artifact.ThresholdMet,
			len(artifact.Signers),
			artifact.CertificateLog.Path != "",
			artifact.GateReport.Path != "",
		)
	}

	fmt.Fprintf(&b, "\n## Signing identities\n\n")
	fmt.Fprintf(&b, "| Identity | Role | Hardware | Offline root | Attested | Recovery shares |\n| --- | --- | --- | ---: | ---: | ---: |\n")
	for _, identity := range report.SigningIdentities {
		fmt.Fprintf(&b, "| `%s` | `%s` | `%s` | `%t` | `%t` | %d |\n",
			escapeTable(identity.ID),
			escapeTable(identity.Role),
			escapeTable(identity.HardwareType),
			identity.OfflineRoot,
			identity.Attestation.Path != "",
			len(identity.RecoveryShares),
		)
	}

	if len(report.Counterexamples) > 0 {
		fmt.Fprintf(&b, "\n## Counterexamples\n\n")
		fmt.Fprintf(&b, "| ID | Kind | Subject | Message |\n| --- | --- | --- | --- |\n")
		for _, counterexample := range report.Counterexamples {
			fmt.Fprintf(&b, "| `%s` | `%s` | `%s` | %s |\n",
				escapeTable(counterexample.ID),
				escapeTable(counterexample.Kind),
				escapeTable(firstNonEmpty(counterexample.Subject, "-")),
				escapeTable(counterexample.Message),
			)
		}
	}
	return b.String()
}

type identityWithReport struct {
	spec   SigningIdentity
	report IdentityReport
}

func buildIdentityReports(root string, identities []SigningIdentity, criteria Criteria) ([]IdentityReport, []Counterexample, map[string]identityWithReport) {
	var reports []IdentityReport
	var counterexamples []Counterexample
	byID := map[string]identityWithReport{}
	for _, identity := range sortedIdentities(identities) {
		key := normalizeToken(identity.ID)
		evidence, evidenceCounterexamples := collectArtifacts(root, identity.EvidencePaths, identity.ID, "identity_evidence", "missing_identity_evidence", "empty_identity_evidence", "invalid_identity_evidence_path", "signing identity evidence could not be read")
		counterexamples = append(counterexamples, evidenceCounterexamples...)
		attestation, attestationCounterexamples := collectOptionalArtifact(root, identity.AttestationPath, identity.ID, "attestation", criteria.RequireAttestation, "missing_attestation")
		counterexamples = append(counterexamples, attestationCounterexamples...)
		publicKey, publicKeyCounterexamples := collectOptionalArtifact(root, identity.PublicKeyPath, identity.ID, "public_key", true, "missing_public_key")
		counterexamples = append(counterexamples, publicKeyCounterexamples...)
		recoveryShares, recoveryCounterexamples := collectArtifacts(root, identity.RecoverySharePaths, identity.ID, "recovery_share", "missing_recovery_share", "empty_recovery_share", "invalid_recovery_share_path", "recovery share could not be read")
		counterexamples = append(counterexamples, recoveryCounterexamples...)
		if criteria.RequireRecoveryDrill && len(recoveryShares) == 0 {
			counterexamples = append(counterexamples, Counterexample{
				ID:      "identity." + stableID(identity.ID, "recovery-share") + ".missing",
				Kind:    "missing_recovery_share",
				Subject: identity.ID,
				Message: "signing identity does not cite a readable recovery share for recovery drills",
			})
		}
		hardwareBacked := isHardwareBacked(identity.HardwareType)
		if criteria.RequireHardwareBacking && !hardwareBacked {
			counterexamples = append(counterexamples, Counterexample{
				ID:      "identity." + stableID(identity.ID, "hardware") + ".software",
				Kind:    "signer_not_hardware_backed",
				Subject: identity.ID,
				Message: "signing identity is not backed by an accepted hardware key type",
				Witness: []string{identity.HardwareType},
			})
		}
		report := IdentityReport{
			ID:             identity.ID,
			Role:           normalizeToken(identity.Role),
			HardwareType:   normalizeToken(identity.HardwareType),
			Slot:           identity.Slot,
			HardwareBacked: hardwareBacked,
			OfflineRoot:    identity.OfflineRoot,
			Attestation:    attestation,
			PublicKey:      publicKey,
			RecoveryShares: recoveryShares,
			Evidence:       evidence,
		}
		reports = append(reports, report)
		byID[key] = identityWithReport{spec: identity, report: report}
	}
	sortCounterexamples(counterexamples)
	return reports, counterexamples, byID
}

func buildArtifactReports(root string, artifacts []SignedArtifact, identities map[string]identityWithReport, criteria Criteria) ([]ArtifactReport, []Counterexample, map[string]SignedArtifact) {
	var reports []ArtifactReport
	var counterexamples []Counterexample
	byID := map[string]SignedArtifact{}
	for _, artifact := range sortedArtifacts(artifacts) {
		subject := artifact.ID
		kind := normalizeToken(artifact.Kind)
		expected := normalizeHash(artifact.SHA256)
		artifactEvidence, artifactCounterexamples := collectArtifact(root, artifact.Path, subject, "signed_artifact", "missing_signed_artifact", "empty_signed_artifact", "invalid_signed_artifact_path", "signed artifact could not be read")
		counterexamples = append(counterexamples, artifactCounterexamples...)
		if expected == "" {
			counterexamples = append(counterexamples, Counterexample{
				ID:      "artifact." + stableID(subject, "sha256") + ".missing",
				Kind:    "unpinned_artifact",
				Subject: subject,
				Message: "signed artifact does not declare a sha256 pin",
				Witness: []string{artifact.Path},
			})
		}
		if expected != "" && artifactEvidence.Path != "" && artifactEvidence.SHA256 != expected {
			counterexamples = append(counterexamples, Counterexample{
				ID:      "artifact." + stableID(subject, "sha256") + ".mismatch",
				Kind:    "artifact_hash_mismatch",
				Subject: subject,
				Message: "signed artifact sha256 pin does not match artifact bytes",
				Witness: []string{artifact.Path, expected, artifactEvidence.SHA256},
			})
		}
		signature, signatureCounterexamples := collectOptionalArtifact(root, artifact.SignaturePath, subject, "signature", true, "missing_signature")
		counterexamples = append(counterexamples, signatureCounterexamples...)
		certificateLog, logCounterexamples := collectOptionalArtifact(root, artifact.CertificateLogPath, subject, "certificate_log", true, "missing_certificate_log")
		counterexamples = append(counterexamples, logCounterexamples...)
		gateReport, gateCounterexamples := collectOptionalArtifact(root, artifact.GateReportPath, subject, "gate_report", true, "missing_gate_report")
		counterexamples = append(counterexamples, gateCounterexamples...)
		evidence, evidenceCounterexamples := collectArtifacts(root, artifact.EvidencePaths, subject, "artifact_evidence", "missing_artifact_evidence", "empty_artifact_evidence", "invalid_artifact_evidence_path", "signed artifact evidence could not be read")
		counterexamples = append(counterexamples, evidenceCounterexamples...)
		signers := uniqueSortedTokens(artifact.SignerIDs)
		hardwareSigners := 0
		for _, signer := range signers {
			identity, ok := identities[normalizeToken(signer)]
			if !ok {
				counterexamples = append(counterexamples, Counterexample{
					ID:      "artifact." + stableID(subject, signer, "signer") + ".unknown",
					Kind:    "unknown_signer",
					Subject: subject,
					Message: "signed artifact references a signer that is not declared",
					Witness: []string{signer},
				})
				continue
			}
			if !identity.report.HardwareBacked {
				counterexamples = append(counterexamples, Counterexample{
					ID:      "artifact." + stableID(subject, signer, "hardware") + ".software",
					Kind:    "signer_not_hardware_backed",
					Subject: subject,
					Message: "artifact signer is declared but not hardware-backed",
					Witness: []string{signer, identity.spec.HardwareType},
				})
				continue
			}
			hardwareSigners++
		}
		thresholdMet := artifact.Threshold > 0 && hardwareSigners >= artifact.Threshold
		if criteria.RequireThresholdApproval && !thresholdMet {
			counterexamples = append(counterexamples, Counterexample{
				ID:      "artifact." + stableID(subject, "threshold") + ".unmet",
				Kind:    "threshold_not_met",
				Subject: subject,
				Message: "artifact does not meet its declared threshold with known hardware-backed signers",
				Witness: []string{fmt.Sprint(hardwareSigners), fmt.Sprint(artifact.Threshold)},
			})
		}
		reports = append(reports, ArtifactReport{
			ID:             artifact.ID,
			Kind:           kind,
			Artifact:       artifactEvidence,
			ExpectedSHA256: expected,
			Pinned:         expected != "" && artifactEvidence.Path != "" && artifactEvidence.SHA256 == expected,
			Signature:      signature,
			Signers:        signers,
			Threshold:      artifact.Threshold,
			ThresholdMet:   thresholdMet,
			CertificateLog: certificateLog,
			GateReport:     gateReport,
			Evidence:       evidence,
		})
		byID[normalizeToken(artifact.ID)] = artifact
	}
	sortCounterexamples(counterexamples)
	return reports, counterexamples, byID
}

func buildDrillReports(root string, drills []Drill, identities map[string]identityWithReport, artifacts map[string]SignedArtifact, criteria Criteria) ([]DrillReport, []Counterexample) {
	var reports []DrillReport
	var counterexamples []Counterexample
	for _, drill := range sortedDrills(drills) {
		kind := normalizeDrillKind(drill.Kind)
		evidence, evidenceCounterexamples := collectArtifacts(root, drill.EvidencePaths, drill.ID, "drill_evidence", "drill_missing_evidence", "empty_drill_evidence", "invalid_drill_evidence_path", "drill evidence could not be read")
		counterexamples = append(counterexamples, evidenceCounterexamples...)
		results, resultCounterexamples := collectArtifacts(root, drill.ResultPaths, drill.ID, "drill_result", "drill_missing_result", "empty_drill_result", "invalid_drill_result_path", "drill result could not be read")
		counterexamples = append(counterexamples, resultCounterexamples...)
		if criteria.RequireEvidenceHashes && len(evidence) == 0 {
			counterexamples = append(counterexamples, Counterexample{
				ID:      "drill." + stableID(drill.ID, "evidence") + ".missing",
				Kind:    "drill_missing_evidence",
				Subject: drill.ID,
				Message: "drill does not cite readable evidence",
			})
		}
		if criteria.RequireEvidenceHashes && len(results) == 0 {
			counterexamples = append(counterexamples, Counterexample{
				ID:      "drill." + stableID(drill.ID, "result") + ".missing",
				Kind:    "drill_missing_result",
				Subject: drill.ID,
				Message: "drill does not cite a readable result artifact",
			})
		}
		if _, ok := artifacts[normalizeToken(drill.ArtifactID)]; !ok {
			counterexamples = append(counterexamples, Counterexample{
				ID:      "drill." + stableID(drill.ID, drill.ArtifactID, "artifact") + ".unknown",
				Kind:    "unknown_drill_artifact",
				Subject: drill.ID,
				Message: "drill references an artifact that is not declared",
				Witness: []string{drill.ArtifactID},
			})
		}
		for _, signer := range []string{drill.OldSignerID, drill.NewSignerID} {
			if strings.TrimSpace(signer) == "" {
				continue
			}
			if _, ok := identities[normalizeToken(signer)]; !ok {
				counterexamples = append(counterexamples, Counterexample{
					ID:      "drill." + stableID(drill.ID, signer, "signer") + ".unknown",
					Kind:    "unknown_drill_signer",
					Subject: drill.ID,
					Message: "drill references a signing identity that is not declared",
					Witness: []string{signer},
				})
			}
		}
		complete := len(drill.Steps) > 0 && strings.TrimSpace(drill.StartedAt) != "" && strings.TrimSpace(drill.CompletedAt) != "" && len(evidence) > 0 && len(results) > 0
		reports = append(reports, DrillReport{
			ID:          drill.ID,
			Kind:        kind,
			ArtifactID:  drill.ArtifactID,
			OldSignerID: drill.OldSignerID,
			NewSignerID: drill.NewSignerID,
			Steps:       len(trimmedStrings(drill.Steps)),
			StartedAt:   drill.StartedAt,
			CompletedAt: drill.CompletedAt,
			Complete:    complete,
			Evidence:    evidence,
			Results:     results,
		})
	}
	sortCounterexamples(counterexamples)
	return reports, counterexamples
}

func missingDrillKindCounterexample(kind string) Counterexample {
	return Counterexample{
		ID:      "drill-kind." + stableID(kind, "required") + ".missing",
		Kind:    "missing_drill_kind",
		Subject: kind,
		Message: "required hardware-signing drill kind is not covered",
		Witness: []string{kind},
	}
}

func validateSpec(spec Spec) error {
	if strings.TrimSpace(spec.Name) == "" {
		return fmt.Errorf("hardware signing name is required")
	}
	criteria := spec.Criteria
	if len(criteria.RequiredArtifactKinds) == 0 {
		return fmt.Errorf("criteria.required_artifact_kinds is required")
	}
	if criteria.MinSigningIdentities <= 0 || criteria.MinArtifactsPerKind <= 0 {
		return fmt.Errorf("hardware signing minimum criteria must be positive")
	}
	identityIDs := map[string]struct{}{}
	for _, identity := range spec.SigningIdentities {
		if strings.TrimSpace(identity.ID) == "" {
			return fmt.Errorf("signing identity id is required")
		}
		key := normalizeToken(identity.ID)
		if _, ok := identityIDs[key]; ok {
			return fmt.Errorf("duplicate signing identity id %q", identity.ID)
		}
		identityIDs[key] = struct{}{}
		for _, path := range append([]string{identity.AttestationPath, identity.PublicKeyPath}, append(identity.RecoverySharePaths, identity.EvidencePaths...)...) {
			if strings.TrimSpace(path) == "" {
				continue
			}
			if err := validateRelativePath(path); err != nil {
				return fmt.Errorf("signing identity %q path: %w", identity.ID, err)
			}
		}
	}
	artifactIDs := map[string]struct{}{}
	for _, artifact := range spec.SignedArtifacts {
		if strings.TrimSpace(artifact.ID) == "" {
			return fmt.Errorf("signed artifact id is required")
		}
		if strings.TrimSpace(artifact.Kind) == "" || strings.TrimSpace(artifact.Path) == "" {
			return fmt.Errorf("signed artifact %q requires kind and path", artifact.ID)
		}
		key := normalizeToken(artifact.ID)
		if _, ok := artifactIDs[key]; ok {
			return fmt.Errorf("duplicate signed artifact id %q", artifact.ID)
		}
		artifactIDs[key] = struct{}{}
		for _, path := range append([]string{artifact.Path, artifact.SignaturePath, artifact.CertificateLogPath, artifact.GateReportPath}, artifact.EvidencePaths...) {
			if strings.TrimSpace(path) == "" {
				continue
			}
			if err := validateRelativePath(path); err != nil {
				return fmt.Errorf("signed artifact %q path: %w", artifact.ID, err)
			}
		}
		if artifact.SHA256 != "" && normalizeHash(artifact.SHA256) == "" {
			return fmt.Errorf("signed artifact %q sha256 must be sha256-prefixed or 64 hex characters", artifact.ID)
		}
	}
	drillIDs := map[string]struct{}{}
	for _, drill := range spec.Drills {
		if strings.TrimSpace(drill.ID) == "" {
			return fmt.Errorf("drill id is required")
		}
		key := normalizeToken(drill.ID)
		if _, ok := drillIDs[key]; ok {
			return fmt.Errorf("duplicate drill id %q", drill.ID)
		}
		drillIDs[key] = struct{}{}
		for _, path := range append(drill.EvidencePaths, drill.ResultPaths...) {
			if strings.TrimSpace(path) == "" {
				continue
			}
			if err := validateRelativePath(path); err != nil {
				return fmt.Errorf("drill %q path: %w", drill.ID, err)
			}
		}
	}
	for _, path := range spec.EvidencePaths {
		if err := validateRelativePath(path); err != nil {
			return fmt.Errorf("run evidence path: %w", err)
		}
	}
	return nil
}

func collectOptionalArtifact(root, relPath, subject, idPart string, required bool, missingKind string) (ArtifactEvidence, []Counterexample) {
	if strings.TrimSpace(relPath) == "" {
		if !required {
			return ArtifactEvidence{}, nil
		}
		return ArtifactEvidence{}, []Counterexample{{
			ID:      "artifact." + stableID(subject, idPart) + ".missing",
			Kind:    missingKind,
			Subject: subject,
			Message: idPart + " artifact path is required",
		}}
	}
	return collectArtifact(root, relPath, subject, idPart, missingKind, "empty_"+idPart, "invalid_"+idPart+"_path", idPart+" artifact could not be read")
}

func collectArtifacts(root string, paths []string, subject, idPart, missingKind, emptyKind, invalidKind, missingMessage string) ([]ArtifactEvidence, []Counterexample) {
	var artifacts []ArtifactEvidence
	var counterexamples []Counterexample
	for _, path := range uniqueSorted(paths) {
		artifact, artifactCounterexamples := collectArtifact(root, path, subject, idPart, missingKind, emptyKind, invalidKind, missingMessage)
		counterexamples = append(counterexamples, artifactCounterexamples...)
		if artifact.Path != "" {
			artifacts = append(artifacts, artifact)
		}
	}
	return artifacts, counterexamples
}

func collectArtifact(root, relPath, subject, idPart, missingKind, emptyKind, invalidKind, missingMessage string) (ArtifactEvidence, []Counterexample) {
	fullPath, err := safeJoin(root, relPath)
	if err != nil {
		return ArtifactEvidence{}, []Counterexample{{
			ID:      "artifact." + stableID(subject, relPath, idPart, "path") + ".invalid",
			Kind:    invalidKind,
			Subject: subject,
			Message: err.Error(),
			Witness: []string{relPath},
		}}
	}
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return ArtifactEvidence{}, []Counterexample{{
			ID:      "artifact." + stableID(subject, relPath, idPart) + ".missing",
			Kind:    missingKind,
			Subject: subject,
			Message: missingMessage,
			Witness: []string{relPath},
		}}
	}
	if len(data) == 0 {
		return ArtifactEvidence{}, []Counterexample{{
			ID:      "artifact." + stableID(subject, relPath, idPart) + ".empty",
			Kind:    emptyKind,
			Subject: subject,
			Message: idPart + " artifact is empty",
			Witness: []string{relPath},
		}}
	}
	sum := sha256.Sum256(data)
	return ArtifactEvidence{
		Path:   filepath.ToSlash(filepath.Clean(relPath)),
		SHA256: "sha256:" + hex.EncodeToString(sum[:]),
		Bytes:  int64(len(data)),
	}, nil
}

func normalizeCriteria(criteria Criteria) Criteria {
	criteria.RequiredArtifactKinds = sortedStrings(normalizedStrings(criteria.RequiredArtifactKinds))
	return criteria
}

func sortedIdentities(identities []SigningIdentity) []SigningIdentity {
	out := append([]SigningIdentity(nil), identities...)
	sort.SliceStable(out, func(i, j int) bool { return normalizeToken(out[i].ID) < normalizeToken(out[j].ID) })
	return out
}

func sortedArtifacts(artifacts []SignedArtifact) []SignedArtifact {
	out := append([]SignedArtifact(nil), artifacts...)
	sort.SliceStable(out, func(i, j int) bool { return normalizeToken(out[i].ID) < normalizeToken(out[j].ID) })
	return out
}

func sortedDrills(drills []Drill) []Drill {
	out := append([]Drill(nil), drills...)
	sort.SliceStable(out, func(i, j int) bool { return normalizeToken(out[i].ID) < normalizeToken(out[j].ID) })
	return out
}

func sortCounterexamples(counterexamples []Counterexample) {
	sort.SliceStable(counterexamples, func(i, j int) bool { return counterexamples[i].ID < counterexamples[j].ID })
}

func sortedStrings(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
}

func normalizedStrings(values []string) []string {
	var out []string
	seen := map[string]struct{}{}
	for _, value := range values {
		normalized := normalizeToken(value)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	sort.Strings(out)
	return out
}

func trimmedStrings(values []string) []string {
	var out []string
	seen := map[string]struct{}{}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	sort.Strings(out)
	return out
}

func uniqueSorted(values []string) []string {
	var out []string
	seen := map[string]struct{}{}
	for _, value := range values {
		clean := filepath.ToSlash(filepath.Clean(strings.TrimSpace(value)))
		if clean == "." || clean == "" {
			continue
		}
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		out = append(out, clean)
	}
	sort.Strings(out)
	return out
}

func uniqueSortedTokens(values []string) []string {
	var out []string
	seen := map[string]struct{}{}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		key := normalizeToken(trimmed)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, trimmed)
	}
	sort.SliceStable(out, func(i, j int) bool { return normalizeToken(out[i]) < normalizeToken(out[j]) })
	return out
}

func normalizeToken(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func normalizeDrillKind(value string) string {
	value = normalizeToken(value)
	switch value {
	case "key-rotation", "rotation", "rotate-key", "rotate":
		return "key-rotation"
	case "recovery", "recover", "disaster-recovery":
		return "recovery"
	case "revocation", "revoke", "revocation-drill":
		return "revocation"
	default:
		return value
	}
}

func normalizeHash(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.TrimPrefix(value, "sha256:")
	if len(value) != 64 {
		return ""
	}
	for _, r := range value {
		if !((r >= 'a' && r <= 'f') || (r >= '0' && r <= '9')) {
			return ""
		}
	}
	return "sha256:" + value
}

func isHardwareBacked(value string) bool {
	return inSet(normalizeToken(value), "yubikey", "hsm", "kms-hsm", "cloud-hsm", "nitrokey", "piv-token", "smartcard", "smart-card", "fips-hsm")
}

func safeJoin(root, relPath string) (string, error) {
	if err := validateRelativePath(relPath); err != nil {
		return "", err
	}
	clean := filepath.Clean(filepath.FromSlash(relPath))
	fullPath := filepath.Join(root, clean)
	rel, err := filepath.Rel(root, fullPath)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes root", relPath)
	}
	return fullPath, nil
}

func validateRelativePath(path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("path is required")
	}
	if filepath.IsAbs(path) {
		return fmt.Errorf("path %q must be relative", path)
	}
	clean := filepath.Clean(filepath.FromSlash(path))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path %q escapes root", path)
	}
	return nil
}

func inSet(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func stableID(parts ...string) string {
	joined := strings.Join(parts, "\x00")
	sum := sha256.Sum256([]byte(joined))
	return hex.EncodeToString(sum[:])[:16]
}

func reportHash(report Report) string {
	report.Hash = ""
	return canonical.Hash(report)
}

func escapeTable(value string) string {
	return strings.ReplaceAll(value, "|", "\\|")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
