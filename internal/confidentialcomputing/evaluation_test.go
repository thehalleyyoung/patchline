package confidentialcomputing

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildReportVerifiesConfidentialComputingEvaluation(t *testing.T) {
	root, spec := testConfidentialComputingSpec(t)
	report, err := BuildReport(spec, root)
	if err != nil {
		t.Fatalf("BuildReport failed: %v", err)
	}
	if !report.OK {
		t.Fatalf("expected confidential-computing report to pass, got %#v", report.Counterexamples)
	}
	if report.Summary.Enclaves != 2 || report.Summary.AttestedEnclaves != 2 || report.Summary.RequiredTEEKindsMet != 2 {
		t.Fatalf("unexpected enclave summary: %#v", report.Summary)
	}
	if report.Summary.KeyReleasePolicies != 2 || report.Summary.Workloads != 2 {
		t.Fatalf("unexpected policy/workload summary: %#v", report.Summary)
	}
	if report.Summary.EncryptedInputs != 2 || report.Summary.PrivateOutputs != 2 || report.Summary.ReplayProofs != 2 {
		t.Fatalf("expected encrypted inputs, private outputs, and replay evidence, got %#v", report.Summary)
	}
	repeat, err := BuildReport(spec, root)
	if err != nil {
		t.Fatalf("repeat BuildReport failed: %v", err)
	}
	if report.Hash == "" || report.Hash != repeat.Hash {
		t.Fatalf("expected deterministic report hash, got %q then %q", report.Hash, repeat.Hash)
	}
	markdown := RenderMarkdown(report)
	for _, phrase := range []string{"Confidential-computing evaluation", "Attested enclaves", "Workloads"} {
		if !strings.Contains(markdown, phrase) {
			t.Fatalf("markdown missing %q:\n%s", phrase, markdown)
		}
	}
}

func TestBuildReportRejectsConfidentialComputingRegressions(t *testing.T) {
	root, spec := testConfidentialComputingSpec(t)
	spec.Enclaves[0].Measurement = hashLiteral("wrong sgx measurement")
	spec.Enclaves[1].TEEKind = "tdx"
	spec.Enclaves[1].AttestationQuotePath = "missing/sev-quote.json"
	spec.Enclaves[1].VerifierReportPath = "missing/sev-verifier.json"
	spec.KeyReleasePolicies[0].PlaintextExportAllowed = true
	spec.KeyReleasePolicies[0].RequiresFreshNonce = false
	spec.KeyReleasePolicies[0].RequiresReviewerQuorum = false
	spec.KeyReleasePolicies[1].AllowedMeasurements = nil
	spec.Workloads[0].EncryptedInputPaths = []string{"missing/private-corpus.age"}
	spec.Workloads[0].OutputManifestSHA256 = hashLiteral("forged output manifest")
	spec.Workloads[0].NetworkEgressAllowed = true
	spec.Workloads[0].OutputsRedacted = false
	spec.Workloads[0].AggregateOnly = false
	spec.Workloads[1].KeyPolicyID = "sgx-policy"
	spec.Workloads[1].PrivateOutputPaths = nil
	spec.Workloads[1].ReplayEvidencePaths = nil

	report, err := BuildReport(spec, root)
	if err != nil {
		t.Fatalf("BuildReport failed: %v", err)
	}
	if report.OK {
		t.Fatalf("expected deficient confidential-computing spec to fail: %#v", report)
	}
	for _, kind := range []string{
		"missing_required_tee_kind",
		"missing_attestation_quote",
		"missing_verifier_report",
		"attestation_measurement_mismatch",
		"measurement_not_allowlisted",
		"missing_measurement_allowlist",
		"plaintext_export_allowed",
		"missing_fresh_nonce",
		"missing_reviewer_quorum",
		"missing_encrypted_input",
		"missing_encrypted_inputs",
		"output_manifest_hash_mismatch",
		"network_egress_allowed",
		"public_output_not_redacted",
		"public_output_not_aggregate",
		"workload_policy_enclave_mismatch",
		"workload_measurement_not_allowed",
		"missing_private_output",
		"missing_replay_evidence",
	} {
		if !hasCounterexample(report, kind) {
			t.Fatalf("expected %s counterexample, got %#v", kind, report.Counterexamples)
		}
	}
}

func TestReadSpecRejectsUnknownFields(t *testing.T) {
	_, err := ReadSpec(strings.NewReader(`{"version":"patchline.confidential-computing/v1","name":"x","criteria":{},"enclaves":[],"key_release_policies":[],"workloads":[],"extra":true}`))
	if err == nil {
		t.Fatal("expected unknown field error")
	}
}

func TestWriteArtifactsIsDeterministic(t *testing.T) {
	root, spec := testConfidentialComputingSpec(t)
	report, err := BuildReport(spec, root)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "confidential-computing")
	if err := WriteArtifacts(out, report); err != nil {
		t.Fatal(err)
	}
	var reread Report
	file, err := os.Open(filepath.Join(out, "confidential-computing.json"))
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
	if stat, err := os.Stat(filepath.Join(out, "confidential-computing.md")); err != nil || stat.Size() == 0 {
		t.Fatalf("expected markdown artifact, stat=%#v err=%v", stat, err)
	}
}

func testConfidentialComputingSpec(t *testing.T) (string, Spec) {
	t.Helper()
	root := t.TempDir()
	sgxMeasurement := hashLiteral("sgx measurement")
	sevMeasurement := hashLiteral("sev measurement")
	files := map[string]string{
		"evidence/run.md":                       "Confidential-computing evaluation for private corpus analysis binds attestation, key release, encrypted inputs, and replay.\n",
		"evidence/sgx.md":                       "SGX verifier accepted measurement and image digest for private Rails corpus analysis.\n",
		"evidence/sev.md":                       "SEV-SNP verifier accepted measurement and image digest for private Django incident analysis.\n",
		"evidence/sgx-policy.md":                "SGX key release policy approved by privacy and reproducibility reviewers.\n",
		"evidence/sev-policy.md":                "SEV key release policy approved by privacy and reproducibility reviewers.\n",
		"evidence/rails-workload.md":            "Rails private migration corpus workload replayed deterministically inside SGX.\n",
		"evidence/django-workload.md":           "Django private incident corpus workload replayed deterministically inside SEV-SNP.\n",
		"attestations/sgx-quote.json":           `{"tee":"sgx","measurement":"` + sgxMeasurement + `","nonce":"fresh-rails"}` + "\n",
		"attestations/sev-quote.json":           `{"tee":"sev-snp","measurement":"` + sevMeasurement + `","nonce":"fresh-django"}` + "\n",
		"verifier-reports/sgx.json":             `{"ok":true,"tee":"sgx","measurement":"` + sgxMeasurement + `"}` + "\n",
		"verifier-reports/sev.json":             `{"ok":true,"tee":"sev-snp","measurement":"` + sevMeasurement + `"}` + "\n",
		"policies/sgx-policy.json":              `{"policy":"sgx-policy","fresh_nonce":true,"reviewer_quorum":2}` + "\n",
		"policies/sev-policy.json":              `{"policy":"sev-policy","fresh_nonce":true,"reviewer_quorum":2}` + "\n",
		"manifests/rails-input.json":            `{"corpus":"private-rails-migrations","encrypted_inputs":["inputs/rails.age"]}` + "\n",
		"manifests/rails-output.json":           `{"public_outputs":["outputs/public/rails-aggregate.json"],"private_outputs":["outputs/private/rails-findings.jsonl"],"redacted":true,"aggregate_only":true}` + "\n",
		"manifests/django-input.json":           `{"corpus":"private-django-incidents","encrypted_inputs":["inputs/django.age"]}` + "\n",
		"manifests/django-output.json":          `{"public_outputs":["outputs/public/django-aggregate.json"],"private_outputs":["outputs/private/django-findings.jsonl"],"redacted":true,"aggregate_only":true}` + "\n",
		"inputs/rails.age":                      "age-encrypted private rails corpus bytes\n",
		"inputs/django.age":                     "age-encrypted private django corpus bytes\n",
		"outputs/public/rails-aggregate.json":   `{"hazard_classes":3,"examples":"redacted"}` + "\n",
		"outputs/public/django-aggregate.json":  `{"hazard_classes":2,"examples":"redacted"}` + "\n",
		"outputs/private/rails-findings.jsonl":  `{"finding":"private-rails-row","redacted_publicly":true}` + "\n",
		"outputs/private/django-findings.jsonl": `{"finding":"private-django-row","redacted_publicly":true}` + "\n",
		"replay/rails.json":                     `{"deterministic":true,"hash":"rails-replay"}` + "\n",
		"replay/django.json":                    `{"deterministic":true,"hash":"django-replay"}` + "\n",
	}
	for path, contents := range files {
		writeConfidentialTestFile(t, root, path, contents)
	}
	return root, Spec{
		Version: SpecVersion,
		Name:    "unit test confidential-computing evaluation",
		Claim:   "Patchline evaluates private corpus analysis inside verifiably attested confidential-computing enclaves with key-release policy, encrypted inputs, redacted aggregate outputs, private retained outputs, no network egress, and deterministic replay evidence.",
		Criteria: Criteria{
			RequiredTEEKinds:            []string{"sgx", "sev-snp"},
			MinEnclaves:                 2,
			MinKeyReleasePolicies:       2,
			MinWorkloads:                2,
			RequireAttestation:          true,
			RequireMeasurementAllowlist: true,
			RequireKeyReleasePolicy:     true,
			RequireEncryptedInputs:      true,
			RequirePrivateOutputs:       true,
			RequireNoPlaintextExport:    true,
			RequireNoNetworkEgress:      true,
			RequireVerifierEvidence:     true,
			RequireReplayEvidence:       true,
			RequireEvidenceHashes:       true,
		},
		Enclaves: []Enclave{
			{
				ID:                   "sgx-private-runner",
				TEEKind:              "sgx",
				Runtime:              "gramine",
				ImageDigest:          hashLiteral("sgx image"),
				Measurement:          sgxMeasurement,
				SignerID:             "confidential-release-root",
				AttestationQuotePath: "attestations/sgx-quote.json",
				VerifierReportPath:   "verifier-reports/sgx.json",
				EvidencePaths:        []string{"evidence/sgx.md"},
			},
			{
				ID:                   "sev-private-runner",
				TEEKind:              "sev-snp",
				Runtime:              "kata-confidential-containers",
				ImageDigest:          hashLiteral("sev image"),
				Measurement:          sevMeasurement,
				SignerID:             "confidential-release-root",
				AttestationQuotePath: "attestations/sev-quote.json",
				VerifierReportPath:   "verifier-reports/sev.json",
				EvidencePaths:        []string{"evidence/sev.md"},
			},
		},
		KeyReleasePolicies: []KeyReleasePolicy{
			{
				ID:                     "sgx-policy",
				EnclaveIDs:             []string{"sgx-private-runner"},
				AllowedMeasurements:    []string{sgxMeasurement},
				MaxAgeHours:            2,
				RequiresFreshNonce:     true,
				RequiresReviewerQuorum: true,
				PlaintextExportAllowed: false,
				PolicyPath:             "policies/sgx-policy.json",
				EvidencePaths:          []string{"evidence/sgx-policy.md"},
			},
			{
				ID:                     "sev-policy",
				EnclaveIDs:             []string{"sev-private-runner"},
				AllowedMeasurements:    []string{sevMeasurement},
				MaxAgeHours:            2,
				RequiresFreshNonce:     true,
				RequiresReviewerQuorum: true,
				PlaintextExportAllowed: false,
				PolicyPath:             "policies/sev-policy.json",
				EvidencePaths:          []string{"evidence/sev-policy.md"},
			},
		},
		Workloads: []Workload{
			confidentialWorkload(t, root, "rails-private-migrations", "migration-risk-analysis", "private-rails-migrations", "sgx-private-runner", "sgx-policy", "manifests/rails-input.json", "inputs/rails.age", "manifests/rails-output.json", "outputs/public/rails-aggregate.json", "outputs/private/rails-findings.jsonl", "replay/rails.json", "evidence/rails-workload.md"),
			confidentialWorkload(t, root, "django-private-incidents", "incident-link-analysis", "private-django-incidents", "sev-private-runner", "sev-policy", "manifests/django-input.json", "inputs/django.age", "manifests/django-output.json", "outputs/public/django-aggregate.json", "outputs/private/django-findings.jsonl", "replay/django.json", "evidence/django-workload.md"),
		},
		EvidencePaths: []string{"evidence/run.md"},
	}
}

func confidentialWorkload(t *testing.T, root, id, kind, corpus, enclave, policy, inputManifest, encryptedInput, outputManifest, publicOutput, privateOutput, replay, evidence string) Workload {
	t.Helper()
	return Workload{
		ID:                   id,
		Kind:                 kind,
		CorpusID:             corpus,
		EnclaveID:            enclave,
		KeyPolicyID:          policy,
		InputManifestPath:    inputManifest,
		InputManifestSHA256:  confidentialTestHash(t, root, inputManifest),
		EncryptedInputPaths:  []string{encryptedInput},
		OutputManifestPath:   outputManifest,
		OutputManifestSHA256: confidentialTestHash(t, root, outputManifest),
		PublicOutputPaths:    []string{publicOutput},
		PrivateOutputPaths:   []string{privateOutput},
		OutputsRedacted:      true,
		AggregateOnly:        true,
		ReplayEvidencePaths:  []string{replay},
		NetworkEgressAllowed: false,
		EvidencePaths:        []string{evidence},
	}
}

func writeConfidentialTestFile(t *testing.T, root, rel, contents string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func confidentialTestHash(t *testing.T, root, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
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
