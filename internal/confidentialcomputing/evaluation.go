package confidentialcomputing

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

const SpecVersion = "patchline.confidential-computing/v1"
const ReportVersion = "patchline.confidential-computing-report/v1"

type Spec struct {
	Version            string             `json:"version"`
	Name               string             `json:"name"`
	Claim              string             `json:"claim,omitempty"`
	Criteria           Criteria           `json:"criteria"`
	Enclaves           []Enclave          `json:"enclaves"`
	KeyReleasePolicies []KeyReleasePolicy `json:"key_release_policies"`
	Workloads          []Workload         `json:"workloads"`
	EvidencePaths      []string           `json:"evidence_paths,omitempty"`
}

type Criteria struct {
	RequiredTEEKinds            []string `json:"required_tee_kinds"`
	MinEnclaves                 int      `json:"min_enclaves"`
	MinKeyReleasePolicies       int      `json:"min_key_release_policies"`
	MinWorkloads                int      `json:"min_workloads"`
	RequireAttestation          bool     `json:"require_attestation"`
	RequireMeasurementAllowlist bool     `json:"require_measurement_allowlist"`
	RequireKeyReleasePolicy     bool     `json:"require_key_release_policy"`
	RequireEncryptedInputs      bool     `json:"require_encrypted_inputs"`
	RequirePrivateOutputs       bool     `json:"require_private_outputs"`
	RequireNoPlaintextExport    bool     `json:"require_no_plaintext_export"`
	RequireNoNetworkEgress      bool     `json:"require_no_network_egress"`
	RequireVerifierEvidence     bool     `json:"require_verifier_evidence"`
	RequireReplayEvidence       bool     `json:"require_replay_evidence"`
	RequireEvidenceHashes       bool     `json:"require_evidence_hashes"`
}

type Enclave struct {
	ID                   string   `json:"id"`
	TEEKind              string   `json:"tee_kind"`
	Runtime              string   `json:"runtime"`
	ImageDigest          string   `json:"image_digest"`
	Measurement          string   `json:"measurement"`
	SignerID             string   `json:"signer_id"`
	AttestationQuotePath string   `json:"attestation_quote_path"`
	VerifierReportPath   string   `json:"verifier_report_path"`
	EvidencePaths        []string `json:"evidence_paths,omitempty"`
}

type KeyReleasePolicy struct {
	ID                     string   `json:"id"`
	EnclaveIDs             []string `json:"enclave_ids"`
	AllowedMeasurements    []string `json:"allowed_measurements"`
	MaxAgeHours            int      `json:"max_age_hours"`
	RequiresFreshNonce     bool     `json:"requires_fresh_nonce"`
	RequiresReviewerQuorum bool     `json:"requires_reviewer_quorum"`
	PlaintextExportAllowed bool     `json:"plaintext_export_allowed"`
	PolicyPath             string   `json:"policy_path"`
	EvidencePaths          []string `json:"evidence_paths,omitempty"`
}

type Workload struct {
	ID                   string   `json:"id"`
	Kind                 string   `json:"kind"`
	CorpusID             string   `json:"corpus_id"`
	EnclaveID            string   `json:"enclave_id"`
	KeyPolicyID          string   `json:"key_policy_id"`
	InputManifestPath    string   `json:"input_manifest_path"`
	InputManifestSHA256  string   `json:"input_manifest_sha256"`
	EncryptedInputPaths  []string `json:"encrypted_input_paths"`
	OutputManifestPath   string   `json:"output_manifest_path"`
	OutputManifestSHA256 string   `json:"output_manifest_sha256"`
	PublicOutputPaths    []string `json:"public_output_paths,omitempty"`
	PrivateOutputPaths   []string `json:"private_output_paths,omitempty"`
	OutputsRedacted      bool     `json:"outputs_redacted"`
	AggregateOnly        bool     `json:"aggregate_only"`
	ReplayEvidencePaths  []string `json:"replay_evidence_paths,omitempty"`
	NetworkEgressAllowed bool     `json:"network_egress_allowed"`
	EvidencePaths        []string `json:"evidence_paths,omitempty"`
}

type Report struct {
	Version            string             `json:"version"`
	Name               string             `json:"name"`
	OK                 bool               `json:"ok"`
	Criteria           Criteria           `json:"criteria"`
	Summary            Summary            `json:"summary"`
	Evidence           []ArtifactEvidence `json:"evidence,omitempty"`
	Enclaves           []EnclaveReport    `json:"enclaves"`
	KeyReleasePolicies []PolicyReport     `json:"key_release_policies"`
	Workloads          []WorkloadReport   `json:"workloads"`
	Counterexamples    []Counterexample   `json:"counterexamples,omitempty"`
	Hash               string             `json:"hash"`
}

type Summary struct {
	Enclaves               int `json:"enclaves"`
	AttestedEnclaves       int `json:"attested_enclaves"`
	RequiredTEEKinds       int `json:"required_tee_kinds"`
	RequiredTEEKindsMet    int `json:"required_tee_kinds_met"`
	KeyReleasePolicies     int `json:"key_release_policies"`
	Workloads              int `json:"workloads"`
	NoNetworkWorkloads     int `json:"no_network_workloads"`
	EncryptedInputs        int `json:"encrypted_inputs"`
	PrivateOutputs         int `json:"private_outputs"`
	PublicOutputs          int `json:"public_outputs"`
	RedactedPublicOutputs  int `json:"redacted_public_outputs"`
	AggregatePublicOutputs int `json:"aggregate_public_outputs"`
	ReplayProofs           int `json:"replay_proofs"`
	VerifierReports        int `json:"verifier_reports"`
	EvidenceArtifacts      int `json:"evidence_artifacts"`
	Counterexamples        int `json:"counterexamples"`
}

type EnclaveReport struct {
	ID             string             `json:"id"`
	TEEKind        string             `json:"tee_kind"`
	Runtime        string             `json:"runtime"`
	ImageDigest    string             `json:"image_digest"`
	Measurement    string             `json:"measurement"`
	SignerID       string             `json:"signer_id"`
	Attested       bool               `json:"attested"`
	Quote          ArtifactEvidence   `json:"quote,omitempty"`
	VerifierReport ArtifactEvidence   `json:"verifier_report,omitempty"`
	Evidence       []ArtifactEvidence `json:"evidence,omitempty"`
}

type PolicyReport struct {
	ID                     string             `json:"id"`
	EnclaveIDs             []string           `json:"enclave_ids"`
	AllowedMeasurements    []string           `json:"allowed_measurements"`
	MaxAgeHours            int                `json:"max_age_hours"`
	RequiresFreshNonce     bool               `json:"requires_fresh_nonce"`
	RequiresReviewerQuorum bool               `json:"requires_reviewer_quorum"`
	PlaintextExportAllowed bool               `json:"plaintext_export_allowed"`
	BindsDeclaredEnclaves  bool               `json:"binds_declared_enclaves"`
	BindsMeasurements      bool               `json:"binds_measurements"`
	Policy                 ArtifactEvidence   `json:"policy,omitempty"`
	Evidence               []ArtifactEvidence `json:"evidence,omitempty"`
}

type WorkloadReport struct {
	ID                     string             `json:"id"`
	Kind                   string             `json:"kind"`
	CorpusID               string             `json:"corpus_id"`
	EnclaveID              string             `json:"enclave_id"`
	KeyPolicyID            string             `json:"key_policy_id"`
	EnclaveKnown           bool               `json:"enclave_known"`
	PolicyKnown            bool               `json:"policy_known"`
	InputManifest          ArtifactEvidence   `json:"input_manifest,omitempty"`
	InputManifestExpected  string             `json:"input_manifest_expected_sha256,omitempty"`
	InputManifestPinned    bool               `json:"input_manifest_pinned"`
	EncryptedInputs        []ArtifactEvidence `json:"encrypted_inputs,omitempty"`
	OutputManifest         ArtifactEvidence   `json:"output_manifest,omitempty"`
	OutputManifestExpected string             `json:"output_manifest_expected_sha256,omitempty"`
	OutputManifestPinned   bool               `json:"output_manifest_pinned"`
	PublicOutputs          []ArtifactEvidence `json:"public_outputs,omitempty"`
	PrivateOutputs         []ArtifactEvidence `json:"private_outputs,omitempty"`
	OutputsRedacted        bool               `json:"outputs_redacted"`
	AggregateOnly          bool               `json:"aggregate_only"`
	ReplayEvidence         []ArtifactEvidence `json:"replay_evidence,omitempty"`
	NetworkEgressAllowed   bool               `json:"network_egress_allowed"`
	Evidence               []ArtifactEvidence `json:"evidence,omitempty"`
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

type enclaveWithReport struct {
	spec   Enclave
	report EnclaveReport
}

type policyWithReport struct {
	spec   KeyReleasePolicy
	report PolicyReport
}

func ReadSpec(reader io.Reader) (Spec, error) {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var spec Spec
	if err := decoder.Decode(&spec); err != nil {
		return Spec{}, err
	}
	if spec.Version != SpecVersion {
		return Spec{}, fmt.Errorf("confidential computing spec version must be %s", SpecVersion)
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
			RequiredTEEKinds: len(criteria.RequiredTEEKinds),
		},
	}

	var counterexamples []Counterexample
	report.Evidence, counterexamples = collectArtifacts(rootAbs, spec.EvidencePaths, spec.Name, "run_evidence", "missing_evidence", "empty_evidence", "invalid_evidence_path", "confidential-computing evidence could not be read")
	report.Summary.EvidenceArtifacts += len(report.Evidence)
	if criteria.RequireEvidenceHashes && len(report.Evidence) == 0 {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "run." + stableID(spec.Name, "evidence") + ".missing",
			Kind:    "missing_evidence",
			Subject: spec.Name,
			Message: "confidential-computing evaluation does not cite readable run-level evidence",
		})
	}

	enclaveReports, enclaveCounterexamples, enclaves := buildEnclaveReports(rootAbs, spec.Enclaves, criteria)
	report.Enclaves = enclaveReports
	counterexamples = append(counterexamples, enclaveCounterexamples...)
	report.Summary.Enclaves = len(enclaveReports)
	teeKinds := map[string]int{}
	for _, enclave := range enclaveReports {
		teeKinds[enclave.TEEKind]++
		if enclave.Attested {
			report.Summary.AttestedEnclaves++
		}
		if enclave.VerifierReport.Path != "" {
			report.Summary.VerifierReports++
			report.Summary.EvidenceArtifacts++
		}
		if enclave.Quote.Path != "" {
			report.Summary.EvidenceArtifacts++
		}
		report.Summary.EvidenceArtifacts += len(enclave.Evidence)
	}
	for _, requiredKind := range criteria.RequiredTEEKinds {
		if teeKinds[requiredKind] > 0 {
			report.Summary.RequiredTEEKindsMet++
			continue
		}
		counterexamples = append(counterexamples, Counterexample{
			ID:      "criteria." + stableID(requiredKind, "tee-kind") + ".missing",
			Kind:    "missing_required_tee_kind",
			Subject: requiredKind,
			Message: "required confidential-computing TEE kind has no declared enclave",
		})
	}
	if len(enclaveReports) < criteria.MinEnclaves {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "criteria.enclaves.insufficient",
			Kind:    "insufficient_enclaves",
			Message: fmt.Sprintf("enclaves %d below required %d", len(enclaveReports), criteria.MinEnclaves),
		})
	}

	policyReports, policyCounterexamples, policies := buildPolicyReports(rootAbs, spec.KeyReleasePolicies, enclaves, criteria)
	report.KeyReleasePolicies = policyReports
	counterexamples = append(counterexamples, policyCounterexamples...)
	report.Summary.KeyReleasePolicies = len(policyReports)
	for _, policy := range policyReports {
		if policy.Policy.Path != "" {
			report.Summary.EvidenceArtifacts++
		}
		report.Summary.EvidenceArtifacts += len(policy.Evidence)
	}
	if len(policyReports) < criteria.MinKeyReleasePolicies {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "criteria.key-policies.insufficient",
			Kind:    "insufficient_key_release_policies",
			Message: fmt.Sprintf("key-release policies %d below required %d", len(policyReports), criteria.MinKeyReleasePolicies),
		})
	}

	workloadReports, workloadCounterexamples := buildWorkloadReports(rootAbs, spec.Workloads, enclaves, policies, criteria)
	report.Workloads = workloadReports
	counterexamples = append(counterexamples, workloadCounterexamples...)
	report.Summary.Workloads = len(workloadReports)
	for _, workload := range workloadReports {
		if !workload.NetworkEgressAllowed {
			report.Summary.NoNetworkWorkloads++
		}
		report.Summary.EncryptedInputs += len(workload.EncryptedInputs)
		report.Summary.PrivateOutputs += len(workload.PrivateOutputs)
		report.Summary.PublicOutputs += len(workload.PublicOutputs)
		if workload.OutputsRedacted {
			report.Summary.RedactedPublicOutputs++
		}
		if workload.AggregateOnly {
			report.Summary.AggregatePublicOutputs++
		}
		report.Summary.ReplayProofs += len(workload.ReplayEvidence)
		report.Summary.EvidenceArtifacts += len(workload.Evidence) + len(workload.EncryptedInputs) + len(workload.PublicOutputs) + len(workload.PrivateOutputs) + len(workload.ReplayEvidence)
		if workload.InputManifest.Path != "" {
			report.Summary.EvidenceArtifacts++
		}
		if workload.OutputManifest.Path != "" {
			report.Summary.EvidenceArtifacts++
		}
	}
	if len(workloadReports) < criteria.MinWorkloads {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "criteria.workloads.insufficient",
			Kind:    "insufficient_workloads",
			Message: fmt.Sprintf("workloads %d below required %d", len(workloadReports), criteria.MinWorkloads),
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
	file, err := os.Create(filepath.Join(outDir, "confidential-computing.json"))
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
	return os.WriteFile(filepath.Join(outDir, "confidential-computing.md"), []byte(RenderMarkdown(report)), 0o644)
}

func RenderMarkdown(report Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Confidential-computing evaluation\n\n")
	fmt.Fprintf(&b, "Patchline evaluates private corpus analysis plans by binding each workload to attested enclave measurements, key-release policy, encrypted inputs, private outputs, redacted aggregate disclosures, and deterministic replay evidence.\n\n")
	fmt.Fprintf(&b, "## Summary\n\n")
	fmt.Fprintf(&b, "| Metric | Value |\n| --- | ---: |\n")
	fmt.Fprintf(&b, "| OK | `%t` |\n", report.OK)
	fmt.Fprintf(&b, "| Enclaves | %d |\n", report.Summary.Enclaves)
	fmt.Fprintf(&b, "| Attested enclaves | %d |\n", report.Summary.AttestedEnclaves)
	fmt.Fprintf(&b, "| Required TEE kinds met | %d / %d |\n", report.Summary.RequiredTEEKindsMet, report.Summary.RequiredTEEKinds)
	fmt.Fprintf(&b, "| Key-release policies | %d |\n", report.Summary.KeyReleasePolicies)
	fmt.Fprintf(&b, "| Workloads | %d |\n", report.Summary.Workloads)
	fmt.Fprintf(&b, "| No-network workloads | %d |\n", report.Summary.NoNetworkWorkloads)
	fmt.Fprintf(&b, "| Encrypted inputs | %d |\n", report.Summary.EncryptedInputs)
	fmt.Fprintf(&b, "| Private outputs | %d |\n", report.Summary.PrivateOutputs)
	fmt.Fprintf(&b, "| Public outputs | %d |\n", report.Summary.PublicOutputs)
	fmt.Fprintf(&b, "| Redacted public outputs | %d |\n", report.Summary.RedactedPublicOutputs)
	fmt.Fprintf(&b, "| Aggregate public outputs | %d |\n", report.Summary.AggregatePublicOutputs)
	fmt.Fprintf(&b, "| Replay proofs | %d |\n", report.Summary.ReplayProofs)
	fmt.Fprintf(&b, "| Verifier reports | %d |\n", report.Summary.VerifierReports)
	fmt.Fprintf(&b, "| Evidence artifacts | %d |\n", report.Summary.EvidenceArtifacts)
	fmt.Fprintf(&b, "| Counterexamples | %d |\n\n", report.Summary.Counterexamples)

	fmt.Fprintf(&b, "## Enclaves\n\n")
	fmt.Fprintf(&b, "| Enclave | TEE | Runtime | Attested | Measurement | Verifier report |\n| --- | --- | --- | ---: | --- | ---: |\n")
	for _, enclave := range report.Enclaves {
		fmt.Fprintf(&b, "| `%s` | `%s` | `%s` | `%t` | `%s` | `%t` |\n",
			escapeTable(enclave.ID),
			escapeTable(enclave.TEEKind),
			escapeTable(enclave.Runtime),
			enclave.Attested,
			escapeTable(enclave.Measurement),
			enclave.VerifierReport.Path != "",
		)
	}

	fmt.Fprintf(&b, "\n## Workloads\n\n")
	fmt.Fprintf(&b, "| Workload | Kind | Corpus | Enclave | Policy | Encrypted inputs | Private outputs | Redacted | Aggregate | Replay proofs |\n| --- | --- | --- | --- | --- | ---: | ---: | ---: | ---: | ---: |\n")
	for _, workload := range report.Workloads {
		fmt.Fprintf(&b, "| `%s` | `%s` | `%s` | `%s` | `%s` | %d | %d | `%t` | `%t` | %d |\n",
			escapeTable(workload.ID),
			escapeTable(workload.Kind),
			escapeTable(workload.CorpusID),
			escapeTable(workload.EnclaveID),
			escapeTable(workload.KeyPolicyID),
			len(workload.EncryptedInputs),
			len(workload.PrivateOutputs),
			workload.OutputsRedacted,
			workload.AggregateOnly,
			len(workload.ReplayEvidence),
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

func buildEnclaveReports(root string, enclaves []Enclave, criteria Criteria) ([]EnclaveReport, []Counterexample, map[string]enclaveWithReport) {
	var reports []EnclaveReport
	var counterexamples []Counterexample
	byID := map[string]enclaveWithReport{}
	for _, enclave := range sortedEnclaves(enclaves) {
		subject := enclave.ID
		evidence, evidenceCounterexamples := collectArtifacts(root, enclave.EvidencePaths, subject, "enclave_evidence", "missing_enclave_evidence", "empty_enclave_evidence", "invalid_enclave_evidence_path", "enclave evidence could not be read")
		counterexamples = append(counterexamples, evidenceCounterexamples...)
		quote, quoteCounterexamples := collectOptionalArtifact(root, enclave.AttestationQuotePath, subject, "attestation_quote", criteria.RequireAttestation, "missing_attestation_quote")
		counterexamples = append(counterexamples, quoteCounterexamples...)
		verifierReport, verifierCounterexamples := collectOptionalArtifact(root, enclave.VerifierReportPath, subject, "verifier_report", criteria.RequireVerifierEvidence, "missing_verifier_report")
		counterexamples = append(counterexamples, verifierCounterexamples...)
		measurement := normalizeMeasurement(enclave.Measurement)
		imageDigest := normalizeHash(enclave.ImageDigest)
		if criteria.RequireAttestation && measurement == "" {
			counterexamples = append(counterexamples, Counterexample{
				ID:      "enclave." + stableID(subject, "measurement") + ".missing",
				Kind:    "missing_enclave_measurement",
				Subject: subject,
				Message: "enclave attestation does not declare a measurement",
			})
		}
		if imageDigest == "" {
			counterexamples = append(counterexamples, Counterexample{
				ID:      "enclave." + stableID(subject, "image-digest") + ".missing",
				Kind:    "missing_enclave_image_digest",
				Subject: subject,
				Message: "enclave image digest is not pinned to sha256",
			})
		}
		quoteBindsMeasurement := true
		if criteria.RequireAttestation && quote.Path != "" && measurement != "" {
			contains, err := artifactContains(root, enclave.AttestationQuotePath, measurement)
			if err != nil {
				counterexamples = append(counterexamples, Counterexample{
					ID:      "enclave." + stableID(subject, "quote-read") + ".unreadable",
					Kind:    "unreadable_attestation_quote",
					Subject: subject,
					Message: err.Error(),
					Witness: []string{enclave.AttestationQuotePath},
				})
				quoteBindsMeasurement = false
			} else if !contains {
				counterexamples = append(counterexamples, Counterexample{
					ID:      "enclave." + stableID(subject, measurement, "quote") + ".mismatch",
					Kind:    "attestation_measurement_mismatch",
					Subject: subject,
					Message: "attestation quote does not contain the declared enclave measurement",
					Witness: []string{enclave.AttestationQuotePath, measurement},
				})
				quoteBindsMeasurement = false
			}
		}
		report := EnclaveReport{
			ID:             subject,
			TEEKind:        normalizeToken(enclave.TEEKind),
			Runtime:        strings.TrimSpace(enclave.Runtime),
			ImageDigest:    imageDigest,
			Measurement:    measurement,
			SignerID:       strings.TrimSpace(enclave.SignerID),
			Attested:       quote.Path != "" && measurement != "" && quoteBindsMeasurement && (!criteria.RequireVerifierEvidence || verifierReport.Path != ""),
			Quote:          quote,
			VerifierReport: verifierReport,
			Evidence:       evidence,
		}
		reports = append(reports, report)
		byID[normalizeToken(subject)] = enclaveWithReport{spec: enclave, report: report}
	}
	sortCounterexamples(counterexamples)
	return reports, counterexamples, byID
}

func buildPolicyReports(root string, policies []KeyReleasePolicy, enclaves map[string]enclaveWithReport, criteria Criteria) ([]PolicyReport, []Counterexample, map[string]policyWithReport) {
	var reports []PolicyReport
	var counterexamples []Counterexample
	byID := map[string]policyWithReport{}
	for _, policy := range sortedPolicies(policies) {
		subject := policy.ID
		evidence, evidenceCounterexamples := collectArtifacts(root, policy.EvidencePaths, subject, "policy_evidence", "missing_policy_evidence", "empty_policy_evidence", "invalid_policy_evidence_path", "key-release policy evidence could not be read")
		counterexamples = append(counterexamples, evidenceCounterexamples...)
		policyArtifact, policyCounterexamples := collectOptionalArtifact(root, policy.PolicyPath, subject, "key_release_policy", criteria.RequireKeyReleasePolicy, "missing_key_release_policy")
		counterexamples = append(counterexamples, policyCounterexamples...)
		enclaveIDs := normalizedTokens(policy.EnclaveIDs)
		allowedMeasurements := normalizedMeasurements(policy.AllowedMeasurements)
		bindsEnclaves := len(enclaveIDs) > 0
		bindsMeasurements := len(allowedMeasurements) > 0
		if criteria.RequireMeasurementAllowlist && len(allowedMeasurements) == 0 {
			counterexamples = append(counterexamples, Counterexample{
				ID:      "policy." + stableID(subject, "measurement-allowlist") + ".missing",
				Kind:    "missing_measurement_allowlist",
				Subject: subject,
				Message: "key-release policy does not declare any allowed enclave measurements",
			})
		}
		for _, enclaveID := range enclaveIDs {
			enclave, ok := enclaves[normalizeToken(enclaveID)]
			if !ok {
				bindsEnclaves = false
				counterexamples = append(counterexamples, Counterexample{
					ID:      "policy." + stableID(subject, enclaveID, "enclave") + ".unknown",
					Kind:    "unknown_policy_enclave",
					Subject: subject,
					Message: "key-release policy references an enclave that is not declared",
					Witness: []string{enclaveID},
				})
				continue
			}
			if criteria.RequireMeasurementAllowlist && !containsString(allowedMeasurements, enclave.report.Measurement) {
				bindsMeasurements = false
				counterexamples = append(counterexamples, Counterexample{
					ID:      "policy." + stableID(subject, enclaveID, enclave.report.Measurement, "measurement") + ".not-allowed",
					Kind:    "measurement_not_allowlisted",
					Subject: subject,
					Message: "key-release policy does not allow the declared enclave measurement",
					Witness: []string{enclaveID, enclave.report.Measurement},
				})
			}
		}
		if criteria.RequireNoPlaintextExport && policy.PlaintextExportAllowed {
			counterexamples = append(counterexamples, Counterexample{
				ID:      "policy." + stableID(subject, "plaintext-export") + ".allowed",
				Kind:    "plaintext_export_allowed",
				Subject: subject,
				Message: "key-release policy permits plaintext private-corpus export",
			})
		}
		if criteria.RequireKeyReleasePolicy && !policy.RequiresFreshNonce {
			counterexamples = append(counterexamples, Counterexample{
				ID:      "policy." + stableID(subject, "fresh-nonce") + ".missing",
				Kind:    "missing_fresh_nonce",
				Subject: subject,
				Message: "key-release policy does not require a fresh attestation nonce",
			})
		}
		if criteria.RequireKeyReleasePolicy && !policy.RequiresReviewerQuorum {
			counterexamples = append(counterexamples, Counterexample{
				ID:      "policy." + stableID(subject, "reviewer-quorum") + ".missing",
				Kind:    "missing_reviewer_quorum",
				Subject: subject,
				Message: "key-release policy does not require reviewer quorum before key release",
			})
		}
		if criteria.RequireKeyReleasePolicy && policy.MaxAgeHours <= 0 {
			counterexamples = append(counterexamples, Counterexample{
				ID:      "policy." + stableID(subject, "max-age") + ".invalid",
				Kind:    "invalid_key_policy_max_age",
				Subject: subject,
				Message: "key-release policy must declare a positive maximum attestation age",
			})
		}
		report := PolicyReport{
			ID:                     subject,
			EnclaveIDs:             enclaveIDs,
			AllowedMeasurements:    allowedMeasurements,
			MaxAgeHours:            policy.MaxAgeHours,
			RequiresFreshNonce:     policy.RequiresFreshNonce,
			RequiresReviewerQuorum: policy.RequiresReviewerQuorum,
			PlaintextExportAllowed: policy.PlaintextExportAllowed,
			BindsDeclaredEnclaves:  bindsEnclaves,
			BindsMeasurements:      bindsMeasurements,
			Policy:                 policyArtifact,
			Evidence:               evidence,
		}
		reports = append(reports, report)
		byID[normalizeToken(subject)] = policyWithReport{spec: policy, report: report}
	}
	sortCounterexamples(counterexamples)
	return reports, counterexamples, byID
}

func buildWorkloadReports(root string, workloads []Workload, enclaves map[string]enclaveWithReport, policies map[string]policyWithReport, criteria Criteria) ([]WorkloadReport, []Counterexample) {
	var reports []WorkloadReport
	var counterexamples []Counterexample
	for _, workload := range sortedWorkloads(workloads) {
		subject := workload.ID
		evidence, evidenceCounterexamples := collectArtifacts(root, workload.EvidencePaths, subject, "workload_evidence", "missing_workload_evidence", "empty_workload_evidence", "invalid_workload_evidence_path", "workload evidence could not be read")
		counterexamples = append(counterexamples, evidenceCounterexamples...)
		inputManifest, inputManifestCounterexamples := collectOptionalArtifact(root, workload.InputManifestPath, subject, "input_manifest", true, "missing_input_manifest")
		counterexamples = append(counterexamples, inputManifestCounterexamples...)
		outputManifest, outputManifestCounterexamples := collectOptionalArtifact(root, workload.OutputManifestPath, subject, "output_manifest", true, "missing_output_manifest")
		counterexamples = append(counterexamples, outputManifestCounterexamples...)
		encryptedInputs, inputCounterexamples := collectArtifacts(root, workload.EncryptedInputPaths, subject, "encrypted_input", "missing_encrypted_input", "empty_encrypted_input", "invalid_encrypted_input_path", "encrypted private-corpus input could not be read")
		counterexamples = append(counterexamples, inputCounterexamples...)
		publicOutputs, publicOutputCounterexamples := collectArtifacts(root, workload.PublicOutputPaths, subject, "public_output", "missing_public_output", "empty_public_output", "invalid_public_output_path", "public output could not be read")
		counterexamples = append(counterexamples, publicOutputCounterexamples...)
		privateOutputs, privateOutputCounterexamples := collectArtifacts(root, workload.PrivateOutputPaths, subject, "private_output", "missing_private_output", "empty_private_output", "invalid_private_output_path", "private output could not be read")
		counterexamples = append(counterexamples, privateOutputCounterexamples...)
		replayEvidence, replayCounterexamples := collectArtifacts(root, workload.ReplayEvidencePaths, subject, "replay_evidence", "missing_replay_evidence", "empty_replay_evidence", "invalid_replay_evidence_path", "replay evidence could not be read")
		counterexamples = append(counterexamples, replayCounterexamples...)

		inputExpected := normalizeHash(workload.InputManifestSHA256)
		if inputExpected == "" {
			counterexamples = append(counterexamples, Counterexample{
				ID:      "workload." + stableID(subject, "input-manifest-sha") + ".missing",
				Kind:    "unpinned_input_manifest",
				Subject: subject,
				Message: "input manifest is not pinned to sha256",
				Witness: []string{workload.InputManifestPath},
			})
		}
		inputPinned := inputExpected != "" && inputManifest.Path != "" && inputManifest.SHA256 == inputExpected
		if inputExpected != "" && inputManifest.Path != "" && inputManifest.SHA256 != inputExpected {
			counterexamples = append(counterexamples, Counterexample{
				ID:      "workload." + stableID(subject, "input-manifest-sha") + ".mismatch",
				Kind:    "input_manifest_hash_mismatch",
				Subject: subject,
				Message: "input manifest sha256 pin does not match artifact bytes",
				Witness: []string{workload.InputManifestPath, inputExpected, inputManifest.SHA256},
			})
		}
		outputExpected := normalizeHash(workload.OutputManifestSHA256)
		if outputExpected == "" {
			counterexamples = append(counterexamples, Counterexample{
				ID:      "workload." + stableID(subject, "output-manifest-sha") + ".missing",
				Kind:    "unpinned_output_manifest",
				Subject: subject,
				Message: "output manifest is not pinned to sha256",
				Witness: []string{workload.OutputManifestPath},
			})
		}
		outputPinned := outputExpected != "" && outputManifest.Path != "" && outputManifest.SHA256 == outputExpected
		if outputExpected != "" && outputManifest.Path != "" && outputManifest.SHA256 != outputExpected {
			counterexamples = append(counterexamples, Counterexample{
				ID:      "workload." + stableID(subject, "output-manifest-sha") + ".mismatch",
				Kind:    "output_manifest_hash_mismatch",
				Subject: subject,
				Message: "output manifest sha256 pin does not match artifact bytes",
				Witness: []string{workload.OutputManifestPath, outputExpected, outputManifest.SHA256},
			})
		}
		if criteria.RequireEncryptedInputs && len(encryptedInputs) == 0 {
			counterexamples = append(counterexamples, Counterexample{
				ID:      "workload." + stableID(subject, "encrypted-inputs") + ".missing",
				Kind:    "missing_encrypted_inputs",
				Subject: subject,
				Message: "workload does not cite any readable encrypted private-corpus inputs",
			})
		}
		if criteria.RequirePrivateOutputs && len(privateOutputs) == 0 {
			counterexamples = append(counterexamples, Counterexample{
				ID:      "workload." + stableID(subject, "private-outputs") + ".missing",
				Kind:    "missing_private_output",
				Subject: subject,
				Message: "workload does not cite any private output artifact retained inside the protected boundary",
			})
		}
		if criteria.RequireReplayEvidence && len(replayEvidence) == 0 {
			counterexamples = append(counterexamples, Counterexample{
				ID:      "workload." + stableID(subject, "replay") + ".missing",
				Kind:    "missing_replay_evidence",
				Subject: subject,
				Message: "workload does not cite deterministic replay evidence for the enclave run",
			})
		}
		if criteria.RequireNoNetworkEgress && workload.NetworkEgressAllowed {
			counterexamples = append(counterexamples, Counterexample{
				ID:      "workload." + stableID(subject, "network") + ".egress",
				Kind:    "network_egress_allowed",
				Subject: subject,
				Message: "private-corpus workload permits network egress",
			})
		}
		if !workload.OutputsRedacted {
			counterexamples = append(counterexamples, Counterexample{
				ID:      "workload." + stableID(subject, "redaction") + ".missing",
				Kind:    "public_output_not_redacted",
				Subject: subject,
				Message: "public workload outputs are not marked as redacted",
			})
		}
		if !workload.AggregateOnly {
			counterexamples = append(counterexamples, Counterexample{
				ID:      "workload." + stableID(subject, "aggregate") + ".missing",
				Kind:    "public_output_not_aggregate",
				Subject: subject,
				Message: "public workload outputs are not restricted to aggregate disclosures",
			})
		}

		enclave, enclaveKnown := enclaves[normalizeToken(workload.EnclaveID)]
		if !enclaveKnown {
			counterexamples = append(counterexamples, Counterexample{
				ID:      "workload." + stableID(subject, workload.EnclaveID, "enclave") + ".unknown",
				Kind:    "unknown_workload_enclave",
				Subject: subject,
				Message: "workload references an enclave that is not declared",
				Witness: []string{workload.EnclaveID},
			})
		}
		policy, policyKnown := policies[normalizeToken(workload.KeyPolicyID)]
		if criteria.RequireKeyReleasePolicy && !policyKnown {
			counterexamples = append(counterexamples, Counterexample{
				ID:      "workload." + stableID(subject, workload.KeyPolicyID, "policy") + ".unknown",
				Kind:    "unknown_workload_policy",
				Subject: subject,
				Message: "workload references a key-release policy that is not declared",
				Witness: []string{workload.KeyPolicyID},
			})
		}
		if enclaveKnown && policyKnown {
			if !containsToken(policy.report.EnclaveIDs, workload.EnclaveID) {
				counterexamples = append(counterexamples, Counterexample{
					ID:      "workload." + stableID(subject, workload.EnclaveID, workload.KeyPolicyID, "binding") + ".mismatch",
					Kind:    "workload_policy_enclave_mismatch",
					Subject: subject,
					Message: "workload policy does not bind the workload enclave",
					Witness: []string{workload.EnclaveID, workload.KeyPolicyID},
				})
			}
			if criteria.RequireMeasurementAllowlist && !containsString(policy.report.AllowedMeasurements, enclave.report.Measurement) {
				counterexamples = append(counterexamples, Counterexample{
					ID:      "workload." + stableID(subject, enclave.report.Measurement, workload.KeyPolicyID, "measurement") + ".not-allowed",
					Kind:    "workload_measurement_not_allowed",
					Subject: subject,
					Message: "workload policy does not allow the workload enclave measurement",
					Witness: []string{workload.EnclaveID, enclave.report.Measurement, workload.KeyPolicyID},
				})
			}
		}

		reports = append(reports, WorkloadReport{
			ID:                     subject,
			Kind:                   normalizeToken(workload.Kind),
			CorpusID:               strings.TrimSpace(workload.CorpusID),
			EnclaveID:              strings.TrimSpace(workload.EnclaveID),
			KeyPolicyID:            strings.TrimSpace(workload.KeyPolicyID),
			EnclaveKnown:           enclaveKnown,
			PolicyKnown:            policyKnown,
			InputManifest:          inputManifest,
			InputManifestExpected:  inputExpected,
			InputManifestPinned:    inputPinned,
			EncryptedInputs:        encryptedInputs,
			OutputManifest:         outputManifest,
			OutputManifestExpected: outputExpected,
			OutputManifestPinned:   outputPinned,
			PublicOutputs:          publicOutputs,
			PrivateOutputs:         privateOutputs,
			OutputsRedacted:        workload.OutputsRedacted,
			AggregateOnly:          workload.AggregateOnly,
			ReplayEvidence:         replayEvidence,
			NetworkEgressAllowed:   workload.NetworkEgressAllowed,
			Evidence:               evidence,
		})
	}
	sortCounterexamples(counterexamples)
	return reports, counterexamples
}

func validateSpec(spec Spec) error {
	if strings.TrimSpace(spec.Name) == "" {
		return fmt.Errorf("confidential-computing spec name is required")
	}
	if spec.Criteria.MinEnclaves < 0 || spec.Criteria.MinKeyReleasePolicies < 0 || spec.Criteria.MinWorkloads < 0 {
		return fmt.Errorf("minimum counts cannot be negative")
	}
	enclaveIDs := map[string]struct{}{}
	for _, enclave := range spec.Enclaves {
		if strings.TrimSpace(enclave.ID) == "" {
			return fmt.Errorf("enclave id is required")
		}
		key := normalizeToken(enclave.ID)
		if _, ok := enclaveIDs[key]; ok {
			return fmt.Errorf("duplicate enclave id %q", enclave.ID)
		}
		enclaveIDs[key] = struct{}{}
		if enclave.ImageDigest != "" && normalizeHash(enclave.ImageDigest) == "" {
			return fmt.Errorf("enclave %q image digest must be sha256-prefixed or 64 hex characters", enclave.ID)
		}
		paths := append([]string{enclave.AttestationQuotePath, enclave.VerifierReportPath}, enclave.EvidencePaths...)
		for _, path := range paths {
			if strings.TrimSpace(path) == "" {
				continue
			}
			if err := validateRelativePath(path); err != nil {
				return fmt.Errorf("enclave %q path: %w", enclave.ID, err)
			}
		}
	}
	policyIDs := map[string]struct{}{}
	for _, policy := range spec.KeyReleasePolicies {
		if strings.TrimSpace(policy.ID) == "" {
			return fmt.Errorf("key-release policy id is required")
		}
		key := normalizeToken(policy.ID)
		if _, ok := policyIDs[key]; ok {
			return fmt.Errorf("duplicate key-release policy id %q", policy.ID)
		}
		policyIDs[key] = struct{}{}
		for _, path := range append([]string{policy.PolicyPath}, policy.EvidencePaths...) {
			if strings.TrimSpace(path) == "" {
				continue
			}
			if err := validateRelativePath(path); err != nil {
				return fmt.Errorf("key-release policy %q path: %w", policy.ID, err)
			}
		}
	}
	workloadIDs := map[string]struct{}{}
	for _, workload := range spec.Workloads {
		if strings.TrimSpace(workload.ID) == "" {
			return fmt.Errorf("workload id is required")
		}
		key := normalizeToken(workload.ID)
		if _, ok := workloadIDs[key]; ok {
			return fmt.Errorf("duplicate workload id %q", workload.ID)
		}
		workloadIDs[key] = struct{}{}
		if workload.InputManifestSHA256 != "" && normalizeHash(workload.InputManifestSHA256) == "" {
			return fmt.Errorf("workload %q input manifest sha256 must be sha256-prefixed or 64 hex characters", workload.ID)
		}
		if workload.OutputManifestSHA256 != "" && normalizeHash(workload.OutputManifestSHA256) == "" {
			return fmt.Errorf("workload %q output manifest sha256 must be sha256-prefixed or 64 hex characters", workload.ID)
		}
		paths := append([]string{workload.InputManifestPath, workload.OutputManifestPath}, workload.EncryptedInputPaths...)
		paths = append(paths, workload.PublicOutputPaths...)
		paths = append(paths, workload.PrivateOutputPaths...)
		paths = append(paths, workload.ReplayEvidencePaths...)
		paths = append(paths, workload.EvidencePaths...)
		for _, path := range paths {
			if strings.TrimSpace(path) == "" {
				continue
			}
			if err := validateRelativePath(path); err != nil {
				return fmt.Errorf("workload %q path: %w", workload.ID, err)
			}
		}
	}
	for _, path := range spec.EvidencePaths {
		if strings.TrimSpace(path) == "" {
			continue
		}
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

func artifactContains(root, relPath, needle string) (bool, error) {
	fullPath, err := safeJoin(root, relPath)
	if err != nil {
		return false, err
	}
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return false, err
	}
	return strings.Contains(strings.ToLower(string(data)), strings.ToLower(needle)), nil
}

func normalizeCriteria(criteria Criteria) Criteria {
	criteria.RequiredTEEKinds = normalizedTokens(criteria.RequiredTEEKinds)
	return criteria
}

func sortedEnclaves(values []Enclave) []Enclave {
	out := append([]Enclave(nil), values...)
	sort.SliceStable(out, func(i, j int) bool { return normalizeToken(out[i].ID) < normalizeToken(out[j].ID) })
	return out
}

func sortedPolicies(values []KeyReleasePolicy) []KeyReleasePolicy {
	out := append([]KeyReleasePolicy(nil), values...)
	sort.SliceStable(out, func(i, j int) bool { return normalizeToken(out[i].ID) < normalizeToken(out[j].ID) })
	return out
}

func sortedWorkloads(values []Workload) []Workload {
	out := append([]Workload(nil), values...)
	sort.SliceStable(out, func(i, j int) bool { return normalizeToken(out[i].ID) < normalizeToken(out[j].ID) })
	return out
}

func sortCounterexamples(counterexamples []Counterexample) {
	sort.SliceStable(counterexamples, func(i, j int) bool { return counterexamples[i].ID < counterexamples[j].ID })
}

func normalizedTokens(values []string) []string {
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

func normalizedMeasurements(values []string) []string {
	var out []string
	seen := map[string]struct{}{}
	for _, value := range values {
		normalized := normalizeMeasurement(value)
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

func containsToken(values []string, target string) bool {
	return containsString(values, normalizeToken(target))
}

func containsString(values []string, target string) bool {
	target = strings.TrimSpace(strings.ToLower(target))
	for _, value := range values {
		if strings.TrimSpace(strings.ToLower(value)) == target {
			return true
		}
	}
	return false
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

func normalizeMeasurement(value string) string {
	return normalizeHash(value)
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
	value = strings.ReplaceAll(value, "|", "\\|")
	value = strings.ReplaceAll(value, "\n", " ")
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
