package supplychainsim

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	pathpkg "path"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/thehalleyyoung/patchline/internal/canonical"
)

const SpecVersion = "patchline.supply-chain-compromise-simulation/v1"
const ReportVersion = "patchline.supply-chain-compromise-simulation-report/v1"

type Spec struct {
	Version       string       `json:"version"`
	Name          string       `json:"name"`
	Claim         string       `json:"claim,omitempty"`
	Criteria      Criteria     `json:"criteria"`
	Simulations   []Simulation `json:"simulations"`
	EvidencePaths []string     `json:"evidence_paths,omitempty"`
}

type Criteria struct {
	RequiredAttackKinds              []string `json:"required_attack_kinds"`
	MinSimulationsPerKind            int      `json:"min_simulations_per_kind"`
	RequireDetection                 bool     `json:"require_detection"`
	RequireRejection                 bool     `json:"require_rejection"`
	RequireQuarantine                bool     `json:"require_quarantine"`
	RequireEvidenceHashes            bool     `json:"require_evidence_hashes"`
	RequireDependencyLockIntegrity   bool     `json:"require_dependency_lock_integrity"`
	RequireArchiveEntrySafety        bool     `json:"require_archive_entry_safety"`
	RequireReleaseMetadataIntegrity  bool     `json:"require_release_metadata_integrity"`
	RequireSignatureOrCertificateLog bool     `json:"require_signature_or_certificate_log"`
}

type Simulation struct {
	ID              string                     `json:"id"`
	Kind            string                     `json:"kind"`
	Target          string                     `json:"target"`
	AttackGoal      string                     `json:"attack_goal"`
	ExpectedOutcome string                     `json:"expected_outcome"`
	Detected        bool                       `json:"detected"`
	Rejected        bool                       `json:"rejected"`
	Quarantined     bool                       `json:"quarantined"`
	Dependency      *DependencySimulation      `json:"dependency,omitempty"`
	Archive         *ArchiveSimulation         `json:"archive,omitempty"`
	ReleaseMetadata *ReleaseMetadataSimulation `json:"release_metadata,omitempty"`
	EvidencePaths   []string                   `json:"evidence_paths,omitempty"`
}

type DependencySimulation struct {
	PackageName            string   `json:"package_name"`
	ExpectedPackageName    string   `json:"expected_package_name"`
	Version                string   `json:"version"`
	Source                 string   `json:"source"`
	ExpectedSource         string   `json:"expected_source"`
	ManifestPath           string   `json:"manifest_path"`
	LockfilePath           string   `json:"lockfile_path"`
	ExpectedLockfileSHA256 string   `json:"expected_lockfile_sha256"`
	PackagePath            string   `json:"package_path"`
	ExpectedPackageSHA256  string   `json:"expected_package_sha256"`
	SignaturePath          string   `json:"signature_path"`
	SignerID               string   `json:"signer_id"`
	AllowedSigners         []string `json:"allowed_signers"`
	Transitive             bool     `json:"transitive"`
	TransitiveAllowlisted  bool     `json:"transitive_allowlisted"`
	AllowlistEvidencePaths []string `json:"allowlist_evidence_paths,omitempty"`
}

type ArchiveSimulation struct {
	ArchivePath           string         `json:"archive_path"`
	ExpectedArchiveSHA256 string         `json:"expected_archive_sha256"`
	SignaturePath         string         `json:"signature_path"`
	SignerID              string         `json:"signer_id"`
	AllowedSigners        []string       `json:"allowed_signers"`
	QuarantinePath        string         `json:"quarantine_path,omitempty"`
	Entries               []ArchiveEntry `json:"entries"`
}

type ArchiveEntry struct {
	Path              string `json:"path"`
	Kind              string `json:"kind"`
	LinkTarget        string `json:"link_target,omitempty"`
	SHA256            string `json:"sha256,omitempty"`
	ExpectedSHA256    string `json:"expected_sha256,omitempty"`
	Executable        bool   `json:"executable,omitempty"`
	ExecutableAllowed bool   `json:"executable_allowed,omitempty"`
}

type ReleaseMetadataSimulation struct {
	ReleaseID              string   `json:"release_id"`
	Version                string   `json:"version"`
	ExpectedVersion        string   `json:"expected_version"`
	Ref                    string   `json:"ref"`
	ExpectedRef            string   `json:"expected_ref"`
	Commit                 string   `json:"commit"`
	ExpectedCommit         string   `json:"expected_commit"`
	ArtifactPath           string   `json:"artifact_path"`
	ExpectedArtifactSHA256 string   `json:"expected_artifact_sha256"`
	MetadataArtifactSHA256 string   `json:"metadata_artifact_sha256"`
	ManifestPath           string   `json:"manifest_path"`
	ExpectedManifestSHA256 string   `json:"expected_manifest_sha256"`
	SignaturePath          string   `json:"signature_path"`
	SignerID               string   `json:"signer_id"`
	AllowedSigners         []string `json:"allowed_signers"`
	CertificateLogPath     string   `json:"certificate_log_path"`
}

type Report struct {
	Version         string             `json:"version"`
	Name            string             `json:"name"`
	OK              bool               `json:"ok"`
	Criteria        Criteria           `json:"criteria"`
	Summary         Summary            `json:"summary"`
	Evidence        []ArtifactEvidence `json:"evidence,omitempty"`
	Simulations     []SimulationReport `json:"simulations"`
	Counterexamples []Counterexample   `json:"counterexamples,omitempty"`
	Hash            string             `json:"hash"`
}

type Summary struct {
	Simulations            int `json:"simulations"`
	AttackKinds            int `json:"attack_kinds"`
	DependencyPoisoning    int `json:"dependency_poisoning"`
	MaliciousArchives      int `json:"malicious_archives"`
	ForgedReleaseMetadata  int `json:"forged_release_metadata"`
	DetectedAttacks        int `json:"detected_attacks"`
	RejectedAttacks        int `json:"rejected_attacks"`
	QuarantinedAttacks     int `json:"quarantined_attacks"`
	AttackSignals          int `json:"attack_signals"`
	DependencySignals      int `json:"dependency_signals"`
	ArchiveSignals         int `json:"archive_signals"`
	ReleaseMetadataSignals int `json:"release_metadata_signals"`
	EvidenceArtifacts      int `json:"evidence_artifacts"`
	Counterexamples        int `json:"counterexamples"`
}

type SimulationReport struct {
	ID              string                      `json:"id"`
	Kind            string                      `json:"kind"`
	Target          string                      `json:"target"`
	AttackGoal      string                      `json:"attack_goal,omitempty"`
	ExpectedOutcome string                      `json:"expected_outcome"`
	Detected        bool                        `json:"detected"`
	Rejected        bool                        `json:"rejected"`
	Quarantined     bool                        `json:"quarantined"`
	Evidence        []ArtifactEvidence          `json:"evidence,omitempty"`
	Signals         []AttackSignal              `json:"signals"`
	Dependency      *DependencySimulationReport `json:"dependency,omitempty"`
	Archive         *ArchiveSimulationReport    `json:"archive,omitempty"`
	ReleaseMetadata *ReleaseSimulationReport    `json:"release_metadata,omitempty"`
}

type DependencySimulationReport struct {
	PackageName            string             `json:"package_name"`
	ExpectedPackageName    string             `json:"expected_package_name,omitempty"`
	Version                string             `json:"version"`
	Source                 string             `json:"source"`
	ExpectedSource         string             `json:"expected_source"`
	Manifest               ArtifactEvidence   `json:"manifest,omitempty"`
	Lockfile               ArtifactEvidence   `json:"lockfile,omitempty"`
	ExpectedLockfileSHA256 string             `json:"expected_lockfile_sha256,omitempty"`
	Package                ArtifactEvidence   `json:"package,omitempty"`
	ExpectedPackageSHA256  string             `json:"expected_package_sha256,omitempty"`
	Signature              ArtifactEvidence   `json:"signature,omitempty"`
	SignerID               string             `json:"signer_id,omitempty"`
	AllowedSigners         []string           `json:"allowed_signers,omitempty"`
	Transitive             bool               `json:"transitive"`
	TransitiveAllowlisted  bool               `json:"transitive_allowlisted"`
	AllowlistEvidence      []ArtifactEvidence `json:"allowlist_evidence,omitempty"`
}

type ArchiveSimulationReport struct {
	Archive               ArtifactEvidence `json:"archive,omitempty"`
	ExpectedArchiveSHA256 string           `json:"expected_archive_sha256,omitempty"`
	Signature             ArtifactEvidence `json:"signature,omitempty"`
	SignerID              string           `json:"signer_id,omitempty"`
	AllowedSigners        []string         `json:"allowed_signers,omitempty"`
	QuarantinePath        string           `json:"quarantine_path,omitempty"`
	Entries               []EntryReport    `json:"entries"`
}

type EntryReport struct {
	Path              string `json:"path"`
	Kind              string `json:"kind"`
	LinkTarget        string `json:"link_target,omitempty"`
	SHA256            string `json:"sha256,omitempty"`
	ExpectedSHA256    string `json:"expected_sha256,omitempty"`
	PathSafe          bool   `json:"path_safe"`
	LinkTargetSafe    bool   `json:"link_target_safe,omitempty"`
	Executable        bool   `json:"executable,omitempty"`
	ExecutableAllowed bool   `json:"executable_allowed,omitempty"`
}

type ReleaseSimulationReport struct {
	ReleaseID              string           `json:"release_id"`
	Version                string           `json:"version"`
	ExpectedVersion        string           `json:"expected_version,omitempty"`
	Ref                    string           `json:"ref"`
	ExpectedRef            string           `json:"expected_ref,omitempty"`
	Commit                 string           `json:"commit"`
	ExpectedCommit         string           `json:"expected_commit,omitempty"`
	Artifact               ArtifactEvidence `json:"artifact,omitempty"`
	ExpectedArtifactSHA256 string           `json:"expected_artifact_sha256,omitempty"`
	MetadataArtifactSHA256 string           `json:"metadata_artifact_sha256,omitempty"`
	Manifest               ArtifactEvidence `json:"manifest,omitempty"`
	ExpectedManifestSHA256 string           `json:"expected_manifest_sha256,omitempty"`
	Signature              ArtifactEvidence `json:"signature,omitempty"`
	SignerID               string           `json:"signer_id,omitempty"`
	AllowedSigners         []string         `json:"allowed_signers,omitempty"`
	CertificateLog         ArtifactEvidence `json:"certificate_log,omitempty"`
}

type AttackSignal struct {
	ID      string   `json:"id"`
	Kind    string   `json:"kind"`
	Subject string   `json:"subject"`
	Message string   `json:"message"`
	Witness []string `json:"witness,omitempty"`
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
		return Spec{}, fmt.Errorf("supply-chain compromise simulation spec version must be %s", SpecVersion)
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
	}

	var counterexamples []Counterexample
	report.Evidence, counterexamples = collectArtifacts(rootAbs, spec.EvidencePaths, spec.Name, "run_evidence", "missing_evidence", "empty_evidence", "invalid_evidence_path", "supply-chain simulation evidence could not be read")
	report.Summary.EvidenceArtifacts += len(report.Evidence)
	if criteria.RequireEvidenceHashes && len(report.Evidence) == 0 {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "run." + stableID(spec.Name, "evidence") + ".missing",
			Kind:    "missing_evidence",
			Subject: spec.Name,
			Message: "supply-chain compromise simulation does not cite readable run-level evidence",
		})
	}

	perKind := map[string]int{}
	for _, simulation := range sortedSimulations(spec.Simulations) {
		simReport, simCounterexamples := buildSimulationReport(rootAbs, simulation, criteria)
		report.Simulations = append(report.Simulations, simReport)
		counterexamples = append(counterexamples, simCounterexamples...)
		report.Summary.Simulations++
		report.Summary.AttackSignals += len(simReport.Signals)
		report.Summary.EvidenceArtifacts += len(simReport.Evidence)
		if len(simReport.Signals) > 0 && simReport.Detected {
			report.Summary.DetectedAttacks++
		}
		if len(simReport.Signals) > 0 && simReport.Rejected {
			report.Summary.RejectedAttacks++
		}
		if len(simReport.Signals) > 0 && simReport.Quarantined {
			report.Summary.QuarantinedAttacks++
		}
		perKind[normalizeKind(simulation.Kind)]++
		switch normalizeKind(simulation.Kind) {
		case "dependency_poisoning":
			report.Summary.DependencyPoisoning++
			report.Summary.DependencySignals += len(simReport.Signals)
			if simReport.Dependency != nil {
				report.Summary.EvidenceArtifacts += countDependencyEvidence(*simReport.Dependency)
			}
		case "malicious_archive":
			report.Summary.MaliciousArchives++
			report.Summary.ArchiveSignals += len(simReport.Signals)
			if simReport.Archive != nil {
				report.Summary.EvidenceArtifacts += countArchiveEvidence(*simReport.Archive)
			}
		case "forged_release_metadata":
			report.Summary.ForgedReleaseMetadata++
			report.Summary.ReleaseMetadataSignals += len(simReport.Signals)
			if simReport.ReleaseMetadata != nil {
				report.Summary.EvidenceArtifacts += countReleaseEvidence(*simReport.ReleaseMetadata)
			}
		}
	}
	report.Summary.AttackKinds = len(perKind)

	for _, kind := range criteria.RequiredAttackKinds {
		normalized := normalizeKind(kind)
		if perKind[normalized] < criteria.MinSimulationsPerKind {
			counterexamples = append(counterexamples, Counterexample{
				ID:      "criteria." + stableID(normalized, "coverage") + ".missing",
				Kind:    "missing_required_attack_kind",
				Subject: normalized,
				Message: fmt.Sprintf("attack kind has %d simulation(s), below required %d", perKind[normalized], criteria.MinSimulationsPerKind),
			})
		}
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
	file, err := os.Create(filepath.Join(outDir, "supply-chain-sim.json"))
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
	return os.WriteFile(filepath.Join(outDir, "supply-chain-sim.md"), []byte(RenderMarkdown(report)), 0o644)
}

func RenderMarkdown(report Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Supply-chain compromise simulations\n\n")
	fmt.Fprintf(&b, "Patchline replays dependency poisoning, malicious archive, and forged release metadata simulations against local artifacts and requires each compromise signal to be detected, rejected, and quarantined with evidence hashes.\n\n")
	fmt.Fprintf(&b, "## Summary\n\n")
	fmt.Fprintf(&b, "| Metric | Value |\n| --- | ---: |\n")
	fmt.Fprintf(&b, "| OK | `%t` |\n", report.OK)
	fmt.Fprintf(&b, "| Simulations | %d |\n", report.Summary.Simulations)
	fmt.Fprintf(&b, "| Dependency poisoning | %d |\n", report.Summary.DependencyPoisoning)
	fmt.Fprintf(&b, "| Malicious archives | %d |\n", report.Summary.MaliciousArchives)
	fmt.Fprintf(&b, "| Forged release metadata | %d |\n", report.Summary.ForgedReleaseMetadata)
	fmt.Fprintf(&b, "| Attack signals | %d |\n", report.Summary.AttackSignals)
	fmt.Fprintf(&b, "| Detected attacks | %d |\n", report.Summary.DetectedAttacks)
	fmt.Fprintf(&b, "| Rejected attacks | %d |\n", report.Summary.RejectedAttacks)
	fmt.Fprintf(&b, "| Quarantined attacks | %d |\n", report.Summary.QuarantinedAttacks)
	fmt.Fprintf(&b, "| Evidence artifacts | %d |\n", report.Summary.EvidenceArtifacts)
	fmt.Fprintf(&b, "| Counterexamples | %d |\n\n", report.Summary.Counterexamples)
	fmt.Fprintf(&b, "## Simulations\n\n")
	fmt.Fprintf(&b, "| Simulation | Kind | Target | Signals | Detected | Rejected | Quarantined |\n| --- | --- | --- | ---: | ---: | ---: | ---: |\n")
	for _, simulation := range report.Simulations {
		fmt.Fprintf(&b, "| `%s` | `%s` | `%s` | %d | `%t` | `%t` | `%t` |\n",
			escapeTable(simulation.ID),
			escapeTable(simulation.Kind),
			escapeTable(simulation.Target),
			len(simulation.Signals),
			simulation.Detected,
			simulation.Rejected,
			simulation.Quarantined,
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

func buildSimulationReport(root string, simulation Simulation, criteria Criteria) (SimulationReport, []Counterexample) {
	var counterexamples []Counterexample
	evidence, evidenceCounterexamples := collectArtifacts(root, simulation.EvidencePaths, simulation.ID, "simulation_evidence", "missing_simulation_evidence", "empty_simulation_evidence", "invalid_simulation_evidence_path", "simulation evidence could not be read")
	counterexamples = append(counterexamples, evidenceCounterexamples...)
	if criteria.RequireEvidenceHashes && len(evidence) == 0 {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "simulation." + stableID(simulation.ID, "evidence") + ".missing",
			Kind:    "missing_simulation_evidence",
			Subject: simulation.ID,
			Message: "simulation does not cite readable evidence",
		})
	}

	report := SimulationReport{
		ID:              simulation.ID,
		Kind:            normalizeKind(simulation.Kind),
		Target:          simulation.Target,
		AttackGoal:      simulation.AttackGoal,
		ExpectedOutcome: strings.ToLower(strings.TrimSpace(simulation.ExpectedOutcome)),
		Detected:        simulation.Detected,
		Rejected:        simulation.Rejected,
		Quarantined:     simulation.Quarantined,
		Evidence:        evidence,
	}
	var signals []AttackSignal
	switch report.Kind {
	case "dependency_poisoning":
		if simulation.Dependency == nil {
			counterexamples = append(counterexamples, missingComponentCounterexample(simulation.ID, "dependency"))
			break
		}
		dependency, dependencySignals, dependencyCounterexamples := buildDependencyReport(root, *simulation.Dependency, criteria)
		report.Dependency = &dependency
		signals = append(signals, dependencySignals...)
		counterexamples = append(counterexamples, dependencyCounterexamples...)
	case "malicious_archive":
		if simulation.Archive == nil {
			counterexamples = append(counterexamples, missingComponentCounterexample(simulation.ID, "archive"))
			break
		}
		archive, archiveSignals, archiveCounterexamples := buildArchiveReport(root, *simulation.Archive, criteria)
		report.Archive = &archive
		signals = append(signals, archiveSignals...)
		counterexamples = append(counterexamples, archiveCounterexamples...)
	case "forged_release_metadata":
		if simulation.ReleaseMetadata == nil {
			counterexamples = append(counterexamples, missingComponentCounterexample(simulation.ID, "release_metadata"))
			break
		}
		release, releaseSignals, releaseCounterexamples := buildReleaseReport(root, *simulation.ReleaseMetadata, criteria)
		report.ReleaseMetadata = &release
		signals = append(signals, releaseSignals...)
		counterexamples = append(counterexamples, releaseCounterexamples...)
	default:
		counterexamples = append(counterexamples, Counterexample{
			ID:      "simulation." + stableID(simulation.ID, "kind") + ".unknown",
			Kind:    "unknown_attack_kind",
			Subject: simulation.ID,
			Message: "simulation kind must be dependency_poisoning, malicious_archive, or forged_release_metadata",
			Witness: []string{simulation.Kind},
		})
	}
	sortSignals(signals)
	report.Signals = signals
	if len(signals) == 0 {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "simulation." + stableID(simulation.ID, "signals") + ".missing",
			Kind:    "missing_simulated_attack_signal",
			Subject: simulation.ID,
			Message: "simulation did not contain a concrete compromise signal",
		})
	}
	if criteria.RequireRejection && report.ExpectedOutcome != "rejected" {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "simulation." + stableID(simulation.ID, "expected-outcome") + ".not-rejected",
			Kind:    "expected_outcome_not_rejected",
			Subject: simulation.ID,
			Message: "supply-chain compromise simulation must model a rejected attack outcome",
			Witness: []string{report.ExpectedOutcome},
		})
	}
	counterexamples = append(counterexamples, mitigationCounterexamples(report, criteria)...)
	sortCounterexamples(counterexamples)
	return report, counterexamples
}

func buildDependencyReport(root string, dependency DependencySimulation, criteria Criteria) (DependencySimulationReport, []AttackSignal, []Counterexample) {
	var signals []AttackSignal
	var counterexamples []Counterexample
	manifest, manifestCounterexamples := collectOptionalArtifact(root, dependency.ManifestPath, dependency.PackageName, "dependency_manifest", "missing_dependency_manifest")
	lockfile, lockfileCounterexamples := collectOptionalArtifact(root, dependency.LockfilePath, dependency.PackageName, "dependency_lockfile", "missing_dependency_lockfile")
	packageArtifact, packageCounterexamples := collectOptionalArtifact(root, dependency.PackagePath, dependency.PackageName, "dependency_package", "missing_dependency_package")
	signature, signatureCounterexamples := collectOptionalArtifact(root, dependency.SignaturePath, dependency.PackageName, "dependency_signature", "missing_dependency_signature")
	allowlistEvidence, allowlistCounterexamples := collectArtifacts(root, dependency.AllowlistEvidencePaths, dependency.PackageName, "dependency_allowlist_evidence", "missing_dependency_allowlist_evidence", "empty_dependency_allowlist_evidence", "invalid_dependency_allowlist_evidence_path", "dependency allowlist evidence could not be read")
	counterexamples = append(counterexamples, manifestCounterexamples...)
	counterexamples = append(counterexamples, lockfileCounterexamples...)
	counterexamples = append(counterexamples, packageCounterexamples...)
	counterexamples = append(counterexamples, signatureCounterexamples...)
	counterexamples = append(counterexamples, allowlistCounterexamples...)
	report := DependencySimulationReport{
		PackageName:            dependency.PackageName,
		ExpectedPackageName:    dependency.ExpectedPackageName,
		Version:                dependency.Version,
		Source:                 dependency.Source,
		ExpectedSource:         dependency.ExpectedSource,
		Manifest:               manifest,
		Lockfile:               lockfile,
		ExpectedLockfileSHA256: normalizeHash(dependency.ExpectedLockfileSHA256),
		Package:                packageArtifact,
		ExpectedPackageSHA256:  normalizeHash(dependency.ExpectedPackageSHA256),
		Signature:              signature,
		SignerID:               dependency.SignerID,
		AllowedSigners:         sortedStrings(normalizedStrings(dependency.AllowedSigners)),
		Transitive:             dependency.Transitive,
		TransitiveAllowlisted:  dependency.TransitiveAllowlisted,
		AllowlistEvidence:      allowlistEvidence,
	}
	subject := firstNonEmpty(dependency.PackageName, dependency.PackagePath, "dependency")
	if dependency.ExpectedPackageName != "" && dependency.PackageName != dependency.ExpectedPackageName {
		signals = append(signals, signal("dependency_name_mismatch", subject, "dependency name does not match the expected package identity", dependency.ExpectedPackageName, dependency.PackageName))
	}
	if dependency.ExpectedSource != "" && dependency.Source != dependency.ExpectedSource {
		signals = append(signals, signal("dependency_source_mismatch", subject, "dependency source does not match the pinned registry or module source", dependency.ExpectedSource, dependency.Source))
	}
	if report.ExpectedPackageSHA256 != "" && packageArtifact.Path != "" && packageArtifact.SHA256 != report.ExpectedPackageSHA256 {
		signals = append(signals, signal("dependency_hash_mismatch", subject, "dependency package hash does not match the pinned lock entry", report.ExpectedPackageSHA256, packageArtifact.SHA256))
	}
	if criteria.RequireDependencyLockIntegrity && report.ExpectedLockfileSHA256 != "" && lockfile.Path != "" && lockfile.SHA256 != report.ExpectedLockfileSHA256 {
		signals = append(signals, signal("dependency_lockfile_hash_mismatch", subject, "dependency lockfile hash drifted from the trusted release ledger", report.ExpectedLockfileSHA256, lockfile.SHA256))
	}
	if criteria.RequireSignatureOrCertificateLog && signature.Path == "" {
		signals = append(signals, signal("missing_dependency_signature", subject, "dependency package lacks a readable signature artifact", dependency.SignaturePath))
	}
	if dependency.SignerID != "" && !containsNormalized(dependency.AllowedSigners, dependency.SignerID) {
		signals = append(signals, signal("unapproved_dependency_signer", subject, "dependency package signer is not in the allowed signer set", dependency.SignerID))
	}
	if dependency.Transitive && !dependency.TransitiveAllowlisted {
		signals = append(signals, signal("unallowlisted_transitive_dependency", subject, "transitive dependency is not covered by the dependency allowlist", dependency.PackageName))
	}
	sortSignals(signals)
	return report, signals, counterexamples
}

func buildArchiveReport(root string, archive ArchiveSimulation, criteria Criteria) (ArchiveSimulationReport, []AttackSignal, []Counterexample) {
	var signals []AttackSignal
	var counterexamples []Counterexample
	archiveArtifact, archiveCounterexamples := collectOptionalArtifact(root, archive.ArchivePath, archive.ArchivePath, "archive", "missing_archive")
	signature, signatureCounterexamples := collectOptionalArtifact(root, archive.SignaturePath, archive.ArchivePath, "archive_signature", "missing_archive_signature")
	counterexamples = append(counterexamples, archiveCounterexamples...)
	counterexamples = append(counterexamples, signatureCounterexamples...)
	report := ArchiveSimulationReport{
		Archive:               archiveArtifact,
		ExpectedArchiveSHA256: normalizeHash(archive.ExpectedArchiveSHA256),
		Signature:             signature,
		SignerID:              archive.SignerID,
		AllowedSigners:        sortedStrings(normalizedStrings(archive.AllowedSigners)),
		QuarantinePath:        archive.QuarantinePath,
	}
	subject := firstNonEmpty(archive.ArchivePath, "archive")
	if report.ExpectedArchiveSHA256 != "" && archiveArtifact.Path != "" && archiveArtifact.SHA256 != report.ExpectedArchiveSHA256 {
		signals = append(signals, signal("archive_hash_mismatch", subject, "archive bytes do not match the pinned digest", report.ExpectedArchiveSHA256, archiveArtifact.SHA256))
	}
	if criteria.RequireSignatureOrCertificateLog && signature.Path == "" {
		signals = append(signals, signal("missing_archive_signature", subject, "archive lacks a readable signature artifact", archive.SignaturePath))
	}
	if archive.SignerID != "" && !containsNormalized(archive.AllowedSigners, archive.SignerID) {
		signals = append(signals, signal("unapproved_archive_signer", subject, "archive signer is not in the allowed signer set", archive.SignerID))
	}
	for _, entry := range sortedArchiveEntries(archive.Entries) {
		entryReport := EntryReport{
			Path:              entry.Path,
			Kind:              normalizeKind(entry.Kind),
			LinkTarget:        entry.LinkTarget,
			SHA256:            normalizeHash(entry.SHA256),
			ExpectedSHA256:    normalizeHash(entry.ExpectedSHA256),
			PathSafe:          safeArchivePath(entry.Path),
			Executable:        entry.Executable,
			ExecutableAllowed: entry.ExecutableAllowed,
		}
		if entryReport.Kind == "symlink" {
			entryReport.LinkTargetSafe = safeArchivePath(entry.LinkTarget)
		}
		if criteria.RequireArchiveEntrySafety && !entryReport.PathSafe {
			signals = append(signals, signal("archive_entry_path_escape", entry.Path, "archive entry path escapes the extraction root", entry.Path))
		}
		if criteria.RequireArchiveEntrySafety && entryReport.Kind == "symlink" && !entryReport.LinkTargetSafe {
			signals = append(signals, signal("archive_symlink_escape", entry.Path, "archive symlink target escapes the extraction root", entry.LinkTarget))
		}
		if entry.Executable && !entry.ExecutableAllowed {
			signals = append(signals, signal("archive_unexpected_executable_payload", entry.Path, "archive contains an executable payload not allowed by the manifest", entry.Path))
		}
		if entryReport.ExpectedSHA256 != "" && entryReport.SHA256 != "" && entryReport.SHA256 != entryReport.ExpectedSHA256 {
			signals = append(signals, signal("archive_entry_hash_mismatch", entry.Path, "archive entry hash does not match the manifest", entryReport.ExpectedSHA256, entryReport.SHA256))
		}
		report.Entries = append(report.Entries, entryReport)
	}
	sortSignals(signals)
	return report, signals, counterexamples
}

func buildReleaseReport(root string, release ReleaseMetadataSimulation, criteria Criteria) (ReleaseSimulationReport, []AttackSignal, []Counterexample) {
	var signals []AttackSignal
	var counterexamples []Counterexample
	artifact, artifactCounterexamples := collectOptionalArtifact(root, release.ArtifactPath, release.ReleaseID, "release_artifact", "missing_release_artifact")
	manifest, manifestCounterexamples := collectOptionalArtifact(root, release.ManifestPath, release.ReleaseID, "release_manifest", "missing_release_manifest")
	signature, signatureCounterexamples := collectOptionalArtifact(root, release.SignaturePath, release.ReleaseID, "release_signature", "missing_release_signature")
	certificateLog, certificateCounterexamples := collectOptionalArtifact(root, release.CertificateLogPath, release.ReleaseID, "release_certificate_log", "missing_release_certificate_log")
	counterexamples = append(counterexamples, artifactCounterexamples...)
	counterexamples = append(counterexamples, manifestCounterexamples...)
	counterexamples = append(counterexamples, signatureCounterexamples...)
	counterexamples = append(counterexamples, certificateCounterexamples...)
	report := ReleaseSimulationReport{
		ReleaseID:              release.ReleaseID,
		Version:                release.Version,
		ExpectedVersion:        release.ExpectedVersion,
		Ref:                    release.Ref,
		ExpectedRef:            release.ExpectedRef,
		Commit:                 release.Commit,
		ExpectedCommit:         release.ExpectedCommit,
		Artifact:               artifact,
		ExpectedArtifactSHA256: normalizeHash(release.ExpectedArtifactSHA256),
		MetadataArtifactSHA256: normalizeHash(release.MetadataArtifactSHA256),
		Manifest:               manifest,
		ExpectedManifestSHA256: normalizeHash(release.ExpectedManifestSHA256),
		Signature:              signature,
		SignerID:               release.SignerID,
		AllowedSigners:         sortedStrings(normalizedStrings(release.AllowedSigners)),
		CertificateLog:         certificateLog,
	}
	subject := firstNonEmpty(release.ReleaseID, release.ManifestPath, "release")
	if criteria.RequireReleaseMetadataIntegrity && report.ExpectedVersion != "" && report.Version != report.ExpectedVersion {
		signals = append(signals, signal("release_version_mismatch", subject, "release metadata version differs from the expected release", report.ExpectedVersion, report.Version))
	}
	if criteria.RequireReleaseMetadataIntegrity && report.ExpectedRef != "" && report.Ref != report.ExpectedRef {
		signals = append(signals, signal("release_ref_mismatch", subject, "release metadata ref differs from the expected ref", report.ExpectedRef, report.Ref))
	}
	if criteria.RequireReleaseMetadataIntegrity && report.ExpectedCommit != "" && report.Commit != report.ExpectedCommit {
		signals = append(signals, signal("release_commit_mismatch", subject, "release metadata commit differs from the expected commit", report.ExpectedCommit, report.Commit))
	}
	if report.ExpectedArtifactSHA256 != "" && artifact.Path != "" && artifact.SHA256 != report.ExpectedArtifactSHA256 {
		signals = append(signals, signal("release_artifact_hash_mismatch", subject, "release artifact bytes do not match the trusted digest", report.ExpectedArtifactSHA256, artifact.SHA256))
	}
	if criteria.RequireReleaseMetadataIntegrity && report.MetadataArtifactSHA256 != "" && artifact.Path != "" && report.MetadataArtifactSHA256 != artifact.SHA256 {
		signals = append(signals, signal("release_metadata_digest_mismatch", subject, "release metadata points at a digest different from the artifact bytes", artifact.SHA256, report.MetadataArtifactSHA256))
	}
	if criteria.RequireReleaseMetadataIntegrity && report.ExpectedManifestSHA256 != "" && manifest.Path != "" && manifest.SHA256 != report.ExpectedManifestSHA256 {
		signals = append(signals, signal("release_manifest_hash_mismatch", subject, "release metadata manifest hash differs from the trusted ledger", report.ExpectedManifestSHA256, manifest.SHA256))
	}
	if criteria.RequireSignatureOrCertificateLog && signature.Path == "" {
		signals = append(signals, signal("missing_release_signature", subject, "release metadata lacks a readable signature artifact", release.SignaturePath))
	}
	if release.SignerID != "" && !containsNormalized(release.AllowedSigners, release.SignerID) {
		signals = append(signals, signal("unapproved_release_signer", subject, "release metadata signer is not in the allowed signer set", release.SignerID))
	}
	if criteria.RequireSignatureOrCertificateLog && certificateLog.Path == "" {
		signals = append(signals, signal("missing_release_certificate_log", subject, "release metadata lacks a readable certificate log entry", release.CertificateLogPath))
	}
	sortSignals(signals)
	return report, signals, counterexamples
}

func mitigationCounterexamples(report SimulationReport, criteria Criteria) []Counterexample {
	if len(report.Signals) == 0 {
		return nil
	}
	var counterexamples []Counterexample
	for _, sig := range report.Signals {
		if criteria.RequireDetection && !report.Detected {
			counterexamples = append(counterexamples, Counterexample{
				ID:      "simulation." + stableID(report.ID, sig.Kind, "detect") + ".missing",
				Kind:    sig.Kind + "_not_detected",
				Subject: report.ID,
				Message: "compromise signal was present but the simulation did not detect it",
				Witness: append([]string{sig.Subject}, sig.Witness...),
			})
		}
		if criteria.RequireRejection && !report.Rejected {
			counterexamples = append(counterexamples, Counterexample{
				ID:      "simulation." + stableID(report.ID, sig.Kind, "reject") + ".missing",
				Kind:    sig.Kind + "_not_rejected",
				Subject: report.ID,
				Message: "compromise signal was present but the simulation did not reject it",
				Witness: append([]string{sig.Subject}, sig.Witness...),
			})
		}
		if criteria.RequireQuarantine && !report.Quarantined {
			counterexamples = append(counterexamples, Counterexample{
				ID:      "simulation." + stableID(report.ID, sig.Kind, "quarantine") + ".missing",
				Kind:    sig.Kind + "_not_quarantined",
				Subject: report.ID,
				Message: "compromise signal was present but the simulation did not quarantine the artifact or metadata",
				Witness: append([]string{sig.Subject}, sig.Witness...),
			})
		}
	}
	return counterexamples
}

func validateSpec(spec Spec) error {
	if strings.TrimSpace(spec.Name) == "" {
		return errorsNew("supply-chain compromise simulation name is required")
	}
	if len(spec.Simulations) == 0 {
		return errorsNew("at least one supply-chain compromise simulation is required")
	}
	seen := map[string]bool{}
	for _, simulation := range spec.Simulations {
		if strings.TrimSpace(simulation.ID) == "" {
			return errorsNew("simulation id is required")
		}
		id := normalizeToken(simulation.ID)
		if seen[id] {
			return fmt.Errorf("duplicate simulation id %q", simulation.ID)
		}
		seen[id] = true
	}
	return nil
}

func normalizeCriteria(criteria Criteria) Criteria {
	if len(criteria.RequiredAttackKinds) == 0 {
		criteria.RequiredAttackKinds = []string{"dependency_poisoning", "malicious_archive", "forged_release_metadata"}
	}
	criteria.RequiredAttackKinds = sortedStrings(normalizedStrings(criteria.RequiredAttackKinds))
	if criteria.MinSimulationsPerKind <= 0 {
		criteria.MinSimulationsPerKind = 1
	}
	return criteria
}

func missingComponentCounterexample(id, component string) Counterexample {
	return Counterexample{
		ID:      "simulation." + stableID(id, component) + ".missing",
		Kind:    "missing_" + component + "_simulation",
		Subject: id,
		Message: "simulation kind does not include the required component details",
	}
}

func collectArtifacts(root string, paths []string, subject, idPrefix, missingKind, emptyKind, invalidKind, readMessage string) ([]ArtifactEvidence, []Counterexample) {
	var artifacts []ArtifactEvidence
	var counterexamples []Counterexample
	for _, rel := range sortedStrings(paths) {
		artifact, counterexample := readArtifact(root, rel, subject, idPrefix, missingKind, emptyKind, invalidKind, readMessage)
		if counterexample.Kind != "" {
			counterexamples = append(counterexamples, counterexample)
			continue
		}
		artifacts = append(artifacts, artifact)
	}
	return artifacts, counterexamples
}

func collectOptionalArtifact(root, rel, subject, idPrefix, missingKind string) (ArtifactEvidence, []Counterexample) {
	if strings.TrimSpace(rel) == "" {
		return ArtifactEvidence{}, []Counterexample{{
			ID:      idPrefix + "." + stableID(subject, "path") + ".missing",
			Kind:    missingKind,
			Subject: subject,
			Message: "artifact path is empty",
		}}
	}
	artifact, counterexample := readArtifact(root, rel, subject, idPrefix, missingKind, "empty_"+strings.TrimPrefix(missingKind, "missing_"), "invalid_"+strings.TrimPrefix(missingKind, "missing_")+"_path", "artifact could not be read")
	if counterexample.Kind != "" {
		return ArtifactEvidence{}, []Counterexample{counterexample}
	}
	return artifact, nil
}

func readArtifact(root, rel, subject, idPrefix, missingKind, emptyKind, invalidKind, readMessage string) (ArtifactEvidence, Counterexample) {
	path, err := safeJoin(root, rel)
	if err != nil {
		return ArtifactEvidence{}, Counterexample{
			ID:      idPrefix + "." + stableID(subject, rel) + ".invalid",
			Kind:    invalidKind,
			Subject: subject,
			Message: err.Error(),
			Witness: []string{rel},
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		return ArtifactEvidence{}, Counterexample{
			ID:      idPrefix + "." + stableID(subject, rel) + ".missing",
			Kind:    missingKind,
			Subject: subject,
			Message: readMessage,
			Witness: []string{rel},
		}
	}
	if info.IsDir() {
		return ArtifactEvidence{}, Counterexample{
			ID:      idPrefix + "." + stableID(subject, rel) + ".invalid",
			Kind:    invalidKind,
			Subject: subject,
			Message: "artifact path is a directory",
			Witness: []string{rel},
		}
	}
	if info.Size() == 0 {
		return ArtifactEvidence{}, Counterexample{
			ID:      idPrefix + "." + stableID(subject, rel) + ".empty",
			Kind:    emptyKind,
			Subject: subject,
			Message: "artifact is empty",
			Witness: []string{rel},
		}
	}
	hash, err := fileHash(path)
	if err != nil {
		return ArtifactEvidence{}, Counterexample{
			ID:      idPrefix + "." + stableID(subject, rel) + ".unreadable",
			Kind:    missingKind,
			Subject: subject,
			Message: err.Error(),
			Witness: []string{rel},
		}
	}
	return ArtifactEvidence{Path: filepath.ToSlash(rel), SHA256: hash, Bytes: info.Size()}, Counterexample{}
}

func safeJoin(root, rel string) (string, error) {
	if strings.TrimSpace(rel) == "" {
		return "", errorsNew("path is empty")
	}
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("path %q must be relative", rel)
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	clean := filepath.Clean(filepath.FromSlash(rel))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes root", rel)
	}
	joined := filepath.Join(rootAbs, clean)
	relative, err := filepath.Rel(rootAbs, joined)
	if err != nil {
		return "", err
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", fmt.Errorf("path %q escapes root", rel)
	}
	return joined, nil
}

func safeArchivePath(path string) bool {
	if strings.TrimSpace(path) == "" || filepath.IsAbs(path) {
		return false
	}
	if strings.ContainsAny(path, "\\\x00") || hasWindowsVolume(path) {
		return false
	}
	clean := pathpkg.Clean(path)
	return clean != "." && clean != ".." && !strings.HasPrefix(clean, "../") && !pathpkg.IsAbs(clean)
}

func hasWindowsVolume(value string) bool {
	if len(value) < 2 || value[1] != ':' {
		return false
	}
	first := value[0]
	return (first >= 'a' && first <= 'z') || (first >= 'A' && first <= 'Z')
}

func fileHash(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(hasher.Sum(nil)), nil
}

func reportHash(report Report) string {
	copy := report
	copy.Hash = ""
	return canonical.Hash(copy)
}

func signal(kind, subject, message string, witness ...string) AttackSignal {
	return AttackSignal{
		ID:      "signal." + stableID(kind, subject, strings.Join(witness, "\x00")),
		Kind:    kind,
		Subject: subject,
		Message: message,
		Witness: sortedStrings(nonEmptyStrings(witness)),
	}
}

func countDependencyEvidence(report DependencySimulationReport) int {
	count := 0
	for _, artifact := range []ArtifactEvidence{report.Manifest, report.Lockfile, report.Package, report.Signature} {
		if artifact.Path != "" {
			count++
		}
	}
	return count + len(report.AllowlistEvidence)
}

func countArchiveEvidence(report ArchiveSimulationReport) int {
	count := 0
	for _, artifact := range []ArtifactEvidence{report.Archive, report.Signature} {
		if artifact.Path != "" {
			count++
		}
	}
	return count
}

func countReleaseEvidence(report ReleaseSimulationReport) int {
	count := 0
	for _, artifact := range []ArtifactEvidence{report.Artifact, report.Manifest, report.Signature, report.CertificateLog} {
		if artifact.Path != "" {
			count++
		}
	}
	return count
}

func sortedSimulations(values []Simulation) []Simulation {
	out := append([]Simulation(nil), values...)
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

func sortedArchiveEntries(values []ArchiveEntry) []ArchiveEntry {
	out := append([]ArchiveEntry(nil), values...)
	sort.Slice(out, func(i, j int) bool {
		return out[i].Path < out[j].Path
	})
	return out
}

func sortSignals(signals []AttackSignal) {
	sort.Slice(signals, func(i, j int) bool {
		if signals[i].Kind != signals[j].Kind {
			return signals[i].Kind < signals[j].Kind
		}
		if signals[i].Subject != signals[j].Subject {
			return signals[i].Subject < signals[j].Subject
		}
		return signals[i].ID < signals[j].ID
	})
}

func sortCounterexamples(counterexamples []Counterexample) {
	sort.Slice(counterexamples, func(i, j int) bool {
		if counterexamples[i].Kind != counterexamples[j].Kind {
			return counterexamples[i].Kind < counterexamples[j].Kind
		}
		if counterexamples[i].Subject != counterexamples[j].Subject {
			return counterexamples[i].Subject < counterexamples[j].Subject
		}
		return counterexamples[i].ID < counterexamples[j].ID
	})
}

func sortedStrings(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
}

func normalizedStrings(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		normalized := normalizeKind(value)
		if normalized == "" || seen[normalized] {
			continue
		}
		seen[normalized] = true
		out = append(out, normalized)
	}
	sort.Strings(out)
	return out
}

func nonEmptyStrings(values []string) []string {
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func containsNormalized(values []string, target string) bool {
	target = normalizeKind(target)
	for _, value := range values {
		if normalizeKind(value) == target {
			return true
		}
	}
	return false
}

func normalizeHash(hash string) string {
	hash = strings.TrimSpace(strings.ToLower(hash))
	if hash == "" {
		return ""
	}
	if strings.HasPrefix(hash, "sha256:") {
		return hash
	}
	return "sha256:" + hash
}

func normalizeKind(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	var b strings.Builder
	lastUnderscore := false
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastUnderscore = false
		default:
			if !lastUnderscore {
				b.WriteByte('_')
				lastUnderscore = true
			}
		}
	}
	return strings.Trim(b.String(), "_")
}

func normalizeToken(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastDash = false
		} else if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func stableID(parts ...string) string {
	normalized := normalizeToken(strings.Join(parts, "-"))
	if normalized == "" {
		normalized = "item"
	}
	hash := canonical.Hash(strings.Join(parts, "\x00"))
	if len(normalized) > 48 {
		normalized = normalized[:48]
	}
	return normalized + "-" + hash[:10]
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func escapeTable(value string) string {
	value = strings.ReplaceAll(value, "|", "\\|")
	value = strings.ReplaceAll(value, "\n", " ")
	return value
}

func errorsNew(message string) error {
	return fmt.Errorf("%s", message)
}
