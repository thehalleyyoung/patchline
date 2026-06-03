package evidencemarketplace

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/thehalleyyoung/patchline/internal/canonical"
)

const (
	RegistryVersion = "patchline.evidence-marketplace/v1"
	ReportVersion   = "patchline.evidence-marketplace-report/v1"
	MirrorVersion   = "patchline.evidence-marketplace-mirror/v1"
)

var requiredObligations = []string{
	"artifact-hashes-verified",
	"license-cleared",
	"redaction-reviewed",
	"reproducible-without-private-data",
}

var acceptedLicenses = map[string]bool{
	"Apache-2.0":   true,
	"BSD-2-Clause": true,
	"BSD-3-Clause": true,
	"CC-BY-4.0":    true,
	"CC0-1.0":      true,
	"MIT":          true,
}

var highSignalPrivateMarkers = []string{
	"-----BEGIN ",
	"authorization:",
	"aws_secret_access_key",
	"password=",
	"private_key",
	"secret=",
	"source_code",
	"token=",
}

type Registry struct {
	Version        string          `json:"version"`
	Claim          string          `json:"claim"`
	Marketplace    Metadata        `json:"marketplace"`
	ChallengeTrack *ChallengeTrack `json:"challenge_track,omitempty"`
	Examples       []Example       `json:"examples"`
}

type Metadata struct {
	Name       string `json:"name"`
	Maintainer string `json:"maintainer"`
	PolicyURL  string `json:"policy_url"`
}

type Example struct {
	ID             string               `json:"id"`
	Title          string               `json:"title"`
	Organization   string               `json:"organization"`
	Ecosystem      string               `json:"ecosystem"`
	HazardClass    string               `json:"hazard_class"`
	Source         Source               `json:"source"`
	LicenseSPDX    string               `json:"license_spdx"`
	Consent        string               `json:"consent"`
	Redaction      Redaction            `json:"redaction"`
	Artifacts      []Artifact           `json:"artifacts"`
	Certificate    Certificate          `json:"certificate"`
	Reproduction   []string             `json:"reproduction"`
	GateReputation *GateReputationInput `json:"gate_reputation,omitempty"`
	Challenge      *ChallengeSubmission `json:"challenge_submission,omitempty"`
	Limitations    []string             `json:"limitations,omitempty"`
}

type ChallengeTrack struct {
	ID                    string                      `json:"id"`
	Name                  string                      `json:"name"`
	RulesURL              string                      `json:"rules_url"`
	ResponsibleDisclosure ResponsibleDisclosurePolicy `json:"responsible_disclosure"`
	Scoring               ChallengeScoringPolicy      `json:"scoring"`
}

type ResponsibleDisclosurePolicy struct {
	Contact                 string `json:"contact"`
	PolicyURL               string `json:"policy_url"`
	EmbargoDays             int    `json:"embargo_days"`
	PublicSafeArtifactsOnly bool   `json:"public_safe_artifacts_only"`
}

type ChallengeScoringPolicy struct {
	MinScoreboardScore int                   `json:"min_scoreboard_score"`
	Weights            ChallengeScoreWeights `json:"weights"`
}

type ChallengeScoreWeights struct {
	AnalyzerSignal        int `json:"analyzer_signal"`
	Reproducibility       int `json:"reproducibility"`
	Minimization          int `json:"minimization"`
	Novelty               int `json:"novelty"`
	ResponsibleDisclosure int `json:"responsible_disclosure"`
}

type ChallengeSubmission struct {
	TrackID                  string                    `json:"track_id"`
	AdversarialGoal          string                    `json:"adversarial_goal"`
	AttackSurface            string                    `json:"attack_surface"`
	ExpectedDetectorBehavior string                    `json:"expected_detector_behavior"`
	MigrationArtifact        string                    `json:"migration_artifact"`
	MaxPublicProofLines      int                       `json:"max_public_proof_lines"`
	NoveltyStatement         string                    `json:"novelty_statement"`
	Disclosure               ChallengeDisclosureStatus `json:"disclosure"`
}

type ChallengeDisclosureStatus struct {
	Status               string `json:"status"`
	PublicReleaseAllowed bool   `json:"public_release_allowed"`
	CoordinatedWith      string `json:"coordinated_with"`
	ReportedAt           string `json:"reported_at"`
	EmbargoEndsAt        string `json:"embargo_ends_at,omitempty"`
	FullExploitHash      string `json:"full_exploit_hash,omitempty"`
}

type Source struct {
	Host    string `json:"host"`
	Repo    string `json:"repo"`
	Ref     string `json:"ref"`
	Commit  string `json:"commit"`
	Subpath string `json:"subpath"`
	URL     string `json:"url,omitempty"`
}

type Redaction struct {
	Reviewed      bool     `json:"redaction_reviewed"`
	RawDataShared bool     `json:"raw_data_shared"`
	Method        string   `json:"method"`
	Fields        []string `json:"fields"`
	Reviewer      string   `json:"reviewer"`
}

type Artifact struct {
	Path     string `json:"path"`
	Role     string `json:"role"`
	SHA256   string `json:"sha256"`
	Redacted bool   `json:"redacted"`
}

type Certificate struct {
	ID          string   `json:"id"`
	Issuer      string   `json:"issuer"`
	IssuedAt    string   `json:"issued_at"`
	SubjectHash string   `json:"subject_hash"`
	Obligations []string `json:"obligations"`
}

type GateReputationInput struct {
	ReproducibleRuns         int      `json:"reproducible_runs"`
	FirstVerifiedAt          string   `json:"first_verified_at"`
	LastVerifiedAt           string   `json:"last_verified_at"`
	IndependentConfirmations []string `json:"independent_confirmations"`
}

type GateReputationReport struct {
	Submitted                bool     `json:"submitted"`
	ReproducibleRuns         int      `json:"reproducible_runs"`
	FirstVerifiedAt          string   `json:"first_verified_at,omitempty"`
	LastVerifiedAt           string   `json:"last_verified_at,omitempty"`
	VerifiedDays             int      `json:"verified_days"`
	IndependentConfirmations []string `json:"independent_confirmations"`
	ReproducibilityPoints    int      `json:"reproducibility_points"`
	LongevityPoints          int      `json:"longevity_points"`
	ConfirmationPoints       int      `json:"confirmation_points"`
	Score                    int      `json:"score"`
	Tier                     string   `json:"tier"`
}

type ReleaseAdmissionReport struct {
	LicenseSPDX              string `json:"license_spdx"`
	LicenseAccepted          bool   `json:"license_accepted"`
	ConsentPresent           bool   `json:"consent_present"`
	ConsentNamesSubmitter    bool   `json:"consent_names_submitter"`
	ConsentGrantsPublication bool   `json:"consent_grants_publication"`
	ConsentNamesLicense      bool   `json:"consent_names_license"`
	PublicReleaseEligible    bool   `json:"public_release_eligible"`
}

type DuplicateAnalysisReport struct {
	ExactFingerprint         string `json:"exact_fingerprint"`
	NearFingerprint          string `json:"near_fingerprint"`
	ExactGroupID             string `json:"exact_group_id,omitempty"`
	ExactGroupSize           int    `json:"exact_group_size"`
	NearGroupID              string `json:"near_group_id,omitempty"`
	NearGroupSize            int    `json:"near_group_size"`
	PrevalenceGroupID        string `json:"prevalence_group_id"`
	PrevalenceGroupKind      string `json:"prevalence_group_kind"`
	PrevalenceRepresentative bool   `json:"prevalence_representative"`
	PrevalenceWeight         int    `json:"prevalence_weight"`
	DuplicateOf              string `json:"duplicate_of,omitempty"`
}

type DuplicateGroup struct {
	ID                        string   `json:"id"`
	Kind                      string   `json:"kind"`
	Fingerprint               string   `json:"fingerprint"`
	RepresentativeID          string   `json:"representative_id"`
	ExampleIDs                []string `json:"example_ids"`
	Count                     int      `json:"count"`
	DistinctExactFingerprints int      `json:"distinct_exact_fingerprints"`
}

type Report struct {
	Version            string             `json:"version"`
	OK                 bool               `json:"ok"`
	RegistryHash       string             `json:"registry_hash"`
	Hash               string             `json:"hash"`
	Summary            Summary            `json:"summary"`
	Marketplace        Metadata           `json:"marketplace"`
	Examples           []PublishedExample `json:"examples"`
	Rejected           []RejectedExample  `json:"rejected,omitempty"`
	DuplicateGroups    []DuplicateGroup   `json:"duplicate_groups,omitempty"`
	ArchiveMirror      ArchiveMirror      `json:"archive_mirror"`
	ByHazard           []Count            `json:"by_hazard"`
	ByHazardPrevalence []Count            `json:"by_hazard_prevalence"`
	ByEcosystem        []Count            `json:"by_ecosystem"`
	ByLicense          []Count            `json:"by_license"`
	ByReputationTier   []Count            `json:"by_reputation_tier"`
	Markdown           string             `json:"markdown,omitempty"`
	registryRoot       string
}

type Summary struct {
	Submitted                 int   `json:"submitted"`
	Published                 int   `json:"published"`
	Rejected                  int   `json:"rejected"`
	PrevalenceExamples        int   `json:"prevalence_examples"`
	DuplicateInflation        int   `json:"duplicate_inflation"`
	ExactDuplicateGroups      int   `json:"exact_duplicate_groups"`
	NearDuplicateGroups       int   `json:"near_duplicate_groups"`
	CertificateBacked         int   `json:"certificate_backed"`
	RedactionReviewed         int   `json:"redaction_reviewed"`
	ClearLicensed             int   `json:"clear_licensed"`
	PublicReleaseEligible     int   `json:"public_release_eligible"`
	ArtifactsVerified         int   `json:"artifacts_verified"`
	ReproductionCommandCount  int   `json:"reproduction_command_count"`
	GateReputationSubmitted   int   `json:"gate_reputation_submitted"`
	GateReputationReviewable  int   `json:"gate_reputation_reviewable"`
	GateReputationEstablished int   `json:"gate_reputation_established"`
	MirroredArtifacts         int   `json:"mirrored_artifacts"`
	MirrorBytes               int64 `json:"mirror_bytes"`
}

type PublishedExample struct {
	ID                     string                  `json:"id"`
	Title                  string                  `json:"title"`
	Organization           string                  `json:"organization"`
	Ecosystem              string                  `json:"ecosystem"`
	HazardClass            string                  `json:"hazard_class"`
	Source                 Source                  `json:"source"`
	LicenseSPDX            string                  `json:"license_spdx"`
	ReleaseAdmission       ReleaseAdmissionReport  `json:"release_admission"`
	CertificateID          string                  `json:"certificate_id"`
	CertificateIssuer      string                  `json:"certificate_issuer"`
	CertificateSubjectHash string                  `json:"certificate_subject_hash"`
	EvidenceHash           string                  `json:"evidence_hash"`
	Artifacts              []ArtifactSummary       `json:"artifacts"`
	Reproduction           []string                `json:"reproduction"`
	GateReputation         GateReputationReport    `json:"gate_reputation"`
	DuplicateAnalysis      DuplicateAnalysisReport `json:"duplicate_analysis"`
	Challenge              *ChallengeSubmission    `json:"challenge_submission,omitempty"`
	Limitations            []string                `json:"limitations,omitempty"`
}

type ArtifactSummary struct {
	Path     string `json:"path"`
	Role     string `json:"role"`
	SHA256   string `json:"sha256"`
	Redacted bool   `json:"redacted"`
	Bytes    int64  `json:"bytes"`
}

type RejectedExample struct {
	ID      string   `json:"id"`
	Reasons []string `json:"reasons"`
}

type Count struct {
	Key   string `json:"key"`
	Count int    `json:"count"`
}

type ArchiveMirror struct {
	Version      string               `json:"version"`
	RegistryHash string               `json:"registry_hash"`
	Marketplace  Metadata             `json:"marketplace"`
	Summary      ArchiveMirrorSummary `json:"summary"`
	Entries      []ArchiveMirrorEntry `json:"entries"`
	ByLicense    []Count              `json:"by_license"`
	Hash         string               `json:"hash"`
}

type ArchiveMirrorSummary struct {
	Examples    int   `json:"examples"`
	Artifacts   int   `json:"artifacts"`
	UniqueFiles int   `json:"unique_files"`
	Bytes       int64 `json:"bytes"`
	UniqueBytes int64 `json:"unique_bytes"`
	Active      int   `json:"active"`
	Withdrawn   int   `json:"withdrawn"`
}

type ArchiveMirrorEntry struct {
	ExampleID              string             `json:"example_id"`
	ArtifactPath           string             `json:"artifact_path"`
	ArtifactRole           string             `json:"artifact_role"`
	MirrorPath             string             `json:"mirror_path"`
	Checksum               string             `json:"checksum"`
	Bytes                  int64              `json:"bytes"`
	LicenseSPDX            string             `json:"license_spdx"`
	Redacted               bool               `json:"redacted"`
	Source                 Source             `json:"source"`
	CertificateSubjectHash string             `json:"certificate_subject_hash"`
	EvidenceHash           string             `json:"evidence_hash"`
	Withdrawal             WithdrawalMetadata `json:"withdrawal"`
}

type WithdrawalMetadata struct {
	Status                          string `json:"status"`
	Requested                       bool   `json:"requested"`
	WithdrawalID                    string `json:"withdrawal_id"`
	PolicyURL                       string `json:"policy_url"`
	Contact                         string `json:"contact"`
	ReviewRequired                  bool   `json:"review_required"`
	TombstoneRequired               bool   `json:"tombstone_required"`
	PreserveChecksumAfterWithdrawal bool   `json:"preserve_checksum_after_withdrawal"`
	ReplacementAllowed              bool   `json:"replacement_allowed"`
}

type certificateSubject struct {
	Version      string               `json:"version"`
	ExampleID    string               `json:"example_id"`
	Source       Source               `json:"source"`
	Ecosystem    string               `json:"ecosystem"`
	HazardClass  string               `json:"hazard_class"`
	LicenseSPDX  string               `json:"license_spdx"`
	Consent      string               `json:"consent"`
	Redaction    Redaction            `json:"redaction"`
	Artifacts    []Artifact           `json:"artifacts"`
	Obligations  []string             `json:"obligations"`
	Reproduction []string             `json:"reproduction"`
	Challenge    *ChallengeSubmission `json:"challenge_submission,omitempty"`
}

type evidenceSubject struct {
	Version      string               `json:"version"`
	ExampleID    string               `json:"example_id"`
	Source       Source               `json:"source"`
	Ecosystem    string               `json:"ecosystem"`
	HazardClass  string               `json:"hazard_class"`
	LicenseSPDX  string               `json:"license_spdx"`
	Redaction    Redaction            `json:"redaction"`
	Artifacts    []Artifact           `json:"artifacts"`
	Reproduction []string             `json:"reproduction"`
	Challenge    *ChallengeSubmission `json:"challenge_submission,omitempty"`
}

type exactDuplicateSubject struct {
	Version     string                   `json:"version"`
	Kind        string                   `json:"kind"`
	Source      Source                   `json:"source"`
	Ecosystem   string                   `json:"ecosystem"`
	HazardClass string                   `json:"hazard_class"`
	Artifacts   []exactDuplicateArtifact `json:"artifacts"`
}

type exactDuplicateArtifact struct {
	Role   string `json:"role"`
	SHA256 string `json:"sha256"`
}

type nearDuplicateSubject struct {
	Version     string              `json:"version"`
	Kind        string              `json:"kind"`
	Source      nearDuplicateSource `json:"source"`
	Ecosystem   string              `json:"ecosystem"`
	HazardClass string              `json:"hazard_class"`
	Cues        []string            `json:"cues"`
}

type nearDuplicateSource struct {
	Host    string `json:"host"`
	Repo    string `json:"repo"`
	Subpath string `json:"subpath"`
}

type duplicateHazardArtifact struct {
	Summary     string                    `json:"summary"`
	Finding     string                    `json:"finding"`
	HazardClass string                    `json:"hazard_class"`
	Evidence    []duplicateHazardEvidence `json:"evidence"`
}

type duplicateHazardEvidence struct {
	Path    string `json:"path"`
	Snippet string `json:"snippet"`
}

var (
	anglePlaceholderRE = regexp.MustCompile(`<[^>]+>`)
	decimalRE          = regexp.MustCompile(`[0-9]+`)
	nonTokenRE         = regexp.MustCompile(`[^a-z0-9_]+`)
)

func ReadRegistry(reader io.Reader) (Registry, error) {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var registry Registry
	if err := decoder.Decode(&registry); err != nil {
		return Registry{}, err
	}
	return registry, nil
}

func ReadRegistryFile(path string) (Registry, error) {
	file, err := os.Open(path)
	if err != nil {
		return Registry{}, err
	}
	defer file.Close()
	return ReadRegistry(file)
}

func PublishRegistryFile(path string) (Report, error) {
	registry, err := ReadRegistryFile(path)
	if err != nil {
		return Report{}, err
	}
	return PublishRegistry(registry, filepath.Dir(path))
}

func PublishRegistry(registry Registry, root string) (Report, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return Report{}, err
	}
	report := Report{
		Version:      ReportVersion,
		RegistryHash: "sha256:" + canonical.Hash(registry),
		Marketplace:  registry.Marketplace,
		registryRoot: rootAbs,
		Summary: Summary{
			Submitted: len(registry.Examples),
		},
	}
	if registry.Version != RegistryVersion {
		report.Rejected = append(report.Rejected, RejectedExample{ID: "registry", Reasons: []string{fmt.Sprintf("unsupported registry version %q", registry.Version)}})
	}
	if strings.TrimSpace(registry.Claim) == "" {
		report.Rejected = append(report.Rejected, RejectedExample{ID: "registry", Reasons: []string{"claim is required"}})
	}
	seen := map[string]bool{}
	for _, example := range registry.Examples {
		published, reasons := validateExample(example, rootAbs, seen)
		if len(reasons) > 0 {
			report.Rejected = append(report.Rejected, RejectedExample{ID: stableRejectedID(example.ID), Reasons: reasons})
			continue
		}
		report.Examples = append(report.Examples, published)
		report.Summary.Published++
		report.Summary.CertificateBacked++
		report.Summary.RedactionReviewed++
		report.Summary.ClearLicensed++
		if published.ReleaseAdmission.PublicReleaseEligible {
			report.Summary.PublicReleaseEligible++
		}
		report.Summary.ArtifactsVerified += len(published.Artifacts)
		report.Summary.ReproductionCommandCount += len(published.Reproduction)
		if published.GateReputation.Submitted {
			report.Summary.GateReputationSubmitted++
		}
		if published.GateReputation.Score >= 50 {
			report.Summary.GateReputationReviewable++
		}
		if published.GateReputation.Tier == "established" {
			report.Summary.GateReputationEstablished++
		}
	}
	sort.Slice(report.Examples, func(i, j int) bool {
		return report.Examples[i].ID < report.Examples[j].ID
	})
	applyDuplicateAnalysis(&report)
	report.Summary.Rejected = len(report.Rejected)
	report.ByHazard = counts(report.Examples, func(example PublishedExample) string { return example.HazardClass })
	report.ByHazardPrevalence = counts(prevalenceExamples(report.Examples), func(example PublishedExample) string { return example.HazardClass })
	report.ByEcosystem = counts(report.Examples, func(example PublishedExample) string { return example.Ecosystem })
	report.ByLicense = counts(report.Examples, func(example PublishedExample) string { return example.LicenseSPDX })
	report.ByReputationTier = counts(report.Examples, func(example PublishedExample) string { return example.GateReputation.Tier })
	report.ArchiveMirror = buildArchiveMirror(report.Marketplace, report.RegistryHash, report.Examples)
	report.Summary.MirroredArtifacts = report.ArchiveMirror.Summary.Artifacts
	report.Summary.MirrorBytes = report.ArchiveMirror.Summary.Bytes
	report.OK = report.Summary.Published > 0 && report.Summary.Rejected == 0
	report.Hash = reportHash(report)
	report.Markdown = RenderMarkdown(report)
	return report, nil
}

func WriteReport(outDir string, report Report) error {
	outAbs, err := filepath.Abs(outDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(outAbs, 0o755); err != nil {
		return err
	}
	if err := writeArchiveMirror(outAbs, report.registryRoot, report.ArchiveMirror); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(outAbs, "marketplace.json"), report); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outAbs, "marketplace.md"), []byte(report.Markdown), 0o644); err != nil {
		return err
	}
	html, err := RenderHTML(report)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outAbs, "index.html"), []byte(html), 0o644)
}

func ExpectedSubjectHash(example Example) string {
	return "sha256:" + canonical.Hash(certificateSubjectFor(example))
}

func EvidenceHash(example Example) string {
	return "sha256:" + canonical.Hash(evidenceSubject{
		Version:      RegistryVersion,
		ExampleID:    strings.TrimSpace(example.ID),
		Source:       normalizeSource(example.Source),
		Ecosystem:    strings.TrimSpace(example.Ecosystem),
		HazardClass:  strings.TrimSpace(example.HazardClass),
		LicenseSPDX:  strings.TrimSpace(example.LicenseSPDX),
		Redaction:    normalizeRedaction(example.Redaction),
		Artifacts:    normalizeArtifacts(example.Artifacts),
		Reproduction: normalizeStringList(example.Reproduction, false),
		Challenge:    normalizeChallengeSubmission(example.Challenge),
	})
}

func RequiredObligations() []string {
	return append([]string(nil), requiredObligations...)
}

func validateExample(example Example, root string, seen map[string]bool) (PublishedExample, []string) {
	var reasons []string
	id := strings.TrimSpace(example.ID)
	if id == "" {
		reasons = append(reasons, "id is required")
	} else if seen[id] {
		reasons = append(reasons, "duplicate id")
	} else {
		seen[id] = true
	}
	for field, value := range map[string]string{
		"title":        example.Title,
		"organization": example.Organization,
		"ecosystem":    example.Ecosystem,
		"hazard_class": example.HazardClass,
	} {
		if strings.TrimSpace(value) == "" {
			reasons = append(reasons, field+" is required")
		}
	}
	reasons = append(reasons, validateSource(example.Source)...)
	releaseAdmission, releaseReasons := evaluateReleaseAdmission(example)
	reasons = append(reasons, releaseReasons...)
	if !example.Redaction.Reviewed {
		reasons = append(reasons, "redaction.redaction_reviewed must be true")
	}
	if example.Redaction.RawDataShared {
		reasons = append(reasons, "redaction.raw_data_shared must be false")
	}
	if strings.TrimSpace(example.Redaction.Method) == "" {
		reasons = append(reasons, "redaction.method is required")
	}
	if len(normalizeStringList(example.Redaction.Fields, true)) == 0 {
		reasons = append(reasons, "redaction.fields must list redacted fields")
	}
	if len(example.Artifacts) == 0 {
		reasons = append(reasons, "at least one redacted artifact is required")
	}
	artifactSummaries, artifactReasons := verifyArtifacts(root, example.Artifacts)
	reasons = append(reasons, artifactReasons...)
	reasons = append(reasons, validateCertificate(example)...)
	reasons = append(reasons, validateReproduction(example.Reproduction)...)
	gateReputation, reputationReasons := evaluateGateReputation(example)
	reasons = append(reasons, reputationReasons...)
	duplicateAnalysis, duplicateReasons := duplicateAnalysisForExample(example, root, artifactSummaries)
	reasons = append(reasons, duplicateReasons...)
	reasons = append(reasons, scanPublicStrings("metadata", metadataStrings(example))...)
	if len(reasons) > 0 {
		sort.Strings(reasons)
		return PublishedExample{}, reasons
	}
	return PublishedExample{
		ID:                     id,
		Title:                  strings.TrimSpace(example.Title),
		Organization:           strings.TrimSpace(example.Organization),
		Ecosystem:              strings.TrimSpace(example.Ecosystem),
		HazardClass:            strings.TrimSpace(example.HazardClass),
		Source:                 normalizeSource(example.Source),
		LicenseSPDX:            strings.TrimSpace(example.LicenseSPDX),
		ReleaseAdmission:       releaseAdmission,
		CertificateID:          strings.TrimSpace(example.Certificate.ID),
		CertificateIssuer:      strings.TrimSpace(example.Certificate.Issuer),
		CertificateSubjectHash: ExpectedSubjectHash(example),
		EvidenceHash:           EvidenceHash(example),
		Artifacts:              artifactSummaries,
		Reproduction:           normalizeStringList(example.Reproduction, false),
		GateReputation:         gateReputation,
		DuplicateAnalysis:      duplicateAnalysis,
		Challenge:              normalizeChallengeSubmission(example.Challenge),
		Limitations:            normalizeStringList(example.Limitations, false),
	}, nil
}

func evaluateReleaseAdmission(example Example) (ReleaseAdmissionReport, []string) {
	license := strings.TrimSpace(example.LicenseSPDX)
	consent := strings.TrimSpace(example.Consent)
	report := ReleaseAdmissionReport{
		LicenseSPDX:              license,
		LicenseAccepted:          acceptedLicenses[license],
		ConsentPresent:           len(consent) >= 40,
		ConsentNamesSubmitter:    containsFold(consent, strings.TrimSpace(example.Organization)),
		ConsentGrantsPublication: containsFold(consent, "publish") || containsFold(consent, "publication"),
		ConsentNamesLicense:      containsFold(consent, license) || containsFold(consent, "declared public license") || containsFold(consent, "public license"),
	}
	report.PublicReleaseEligible = report.LicenseAccepted &&
		report.ConsentPresent &&
		report.ConsentNamesSubmitter &&
		report.ConsentGrantsPublication &&
		report.ConsentNamesLicense

	var reasons []string
	if !report.LicenseAccepted {
		reasons = append(reasons, "license_spdx must be a clear accepted public license")
	}
	if !report.ConsentPresent {
		reasons = append(reasons, "consent must describe publication permission")
	}
	if !report.ConsentNamesSubmitter {
		reasons = append(reasons, "consent must name the submitting organization")
	}
	if !report.ConsentGrantsPublication {
		reasons = append(reasons, "consent must explicitly grant publication")
	}
	if !report.ConsentNamesLicense {
		reasons = append(reasons, "consent must reference the declared public license")
	}
	return report, reasons
}

func containsFold(value, needle string) bool {
	value = strings.TrimSpace(value)
	needle = strings.TrimSpace(needle)
	if value == "" || needle == "" {
		return false
	}
	return strings.Contains(strings.ToLower(value), strings.ToLower(needle))
}

func validateSource(source Source) []string {
	var reasons []string
	for field, value := range map[string]string{
		"source.host":   source.Host,
		"source.repo":   source.Repo,
		"source.ref":    source.Ref,
		"source.commit": source.Commit,
	} {
		if strings.TrimSpace(value) == "" {
			reasons = append(reasons, field+" is required")
		}
	}
	if commit := strings.TrimSpace(source.Commit); len(commit) < 12 {
		reasons = append(reasons, "source.commit must be a pinned commit-like identifier")
	}
	return reasons
}

func verifyArtifacts(root string, artifacts []Artifact) ([]ArtifactSummary, []string) {
	var summaries []ArtifactSummary
	var reasons []string
	seenPaths := map[string]bool{}
	for _, artifact := range normalizeArtifacts(artifacts) {
		if artifact.Path == "" {
			reasons = append(reasons, "artifact path is required")
			continue
		}
		if seenPaths[artifact.Path] {
			reasons = append(reasons, "duplicate artifact path "+artifact.Path)
			continue
		}
		seenPaths[artifact.Path] = true
		if artifact.Role == "" {
			reasons = append(reasons, "artifact "+artifact.Path+" role is required")
		}
		if !artifact.Redacted {
			reasons = append(reasons, "artifact "+artifact.Path+" must be marked redacted")
		}
		path, err := resolveArtifact(root, artifact.Path)
		if err != nil {
			reasons = append(reasons, "artifact "+artifact.Path+": "+err.Error())
			continue
		}
		actual, bytes, err := sha256File(path)
		if err != nil {
			reasons = append(reasons, "artifact "+artifact.Path+": "+err.Error())
			continue
		}
		expected := strings.TrimSpace(artifact.SHA256)
		if expected != "sha256:"+actual {
			reasons = append(reasons, "artifact "+artifact.Path+" sha256 mismatch")
		}
		content, err := os.ReadFile(path)
		if err != nil {
			reasons = append(reasons, "artifact "+artifact.Path+": "+err.Error())
			continue
		}
		reasons = append(reasons, scanPublicStrings("artifact "+artifact.Path, []string{string(content)})...)
		summaries = append(summaries, ArtifactSummary{
			Path:     artifact.Path,
			Role:     artifact.Role,
			SHA256:   "sha256:" + actual,
			Redacted: artifact.Redacted,
			Bytes:    bytes,
		})
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].Path < summaries[j].Path
	})
	return summaries, reasons
}

func validateCertificate(example Example) []string {
	var reasons []string
	if strings.TrimSpace(example.Certificate.ID) == "" {
		reasons = append(reasons, "certificate.id is required")
	}
	if strings.TrimSpace(example.Certificate.Issuer) == "" {
		reasons = append(reasons, "certificate.issuer is required")
	}
	if strings.TrimSpace(example.Certificate.IssuedAt) == "" {
		reasons = append(reasons, "certificate.issued_at is required")
	}
	expectedHash := ExpectedSubjectHash(example)
	if strings.TrimSpace(example.Certificate.SubjectHash) != expectedHash {
		reasons = append(reasons, "certificate.subject_hash mismatch; expected "+expectedHash)
	}
	obligations := stringSet(example.Certificate.Obligations)
	for _, required := range requiredObligations {
		if !obligations[required] {
			reasons = append(reasons, "certificate missing obligation "+required)
		}
	}
	return reasons
}

func validateReproduction(commands []string) []string {
	var reasons []string
	if len(commands) == 0 {
		return []string{"at least one reproduction command is required"}
	}
	for i, command := range commands {
		command = strings.TrimSpace(command)
		if command == "" {
			reasons = append(reasons, fmt.Sprintf("reproduction command %d is empty", i+1))
			continue
		}
		if strings.Contains(command, "\n") || strings.Contains(command, "\r") {
			reasons = append(reasons, fmt.Sprintf("reproduction command %d must be single-line", i+1))
		}
		reasons = append(reasons, scanPublicStrings(fmt.Sprintf("reproduction command %d", i+1), []string{command})...)
	}
	return reasons
}

func evaluateGateReputation(example Example) (GateReputationReport, []string) {
	report := GateReputationReport{Tier: "emerging"}
	if example.GateReputation == nil {
		return report, nil
	}
	report.Submitted = true
	reputation := *example.GateReputation
	var reasons []string
	if reputation.ReproducibleRuns < 0 {
		reasons = append(reasons, "gate_reputation.reproducible_runs must be non-negative")
	}
	report.ReproducibleRuns = reputation.ReproducibleRuns

	first, firstReasons := parseGateReputationTime("gate_reputation.first_verified_at", reputation.FirstVerifiedAt)
	last, lastReasons := parseGateReputationTime("gate_reputation.last_verified_at", reputation.LastVerifiedAt)
	reasons = append(reasons, firstReasons...)
	reasons = append(reasons, lastReasons...)
	if len(firstReasons) == 0 {
		report.FirstVerifiedAt = first.Format(time.RFC3339)
	}
	if len(lastReasons) == 0 {
		report.LastVerifiedAt = last.Format(time.RFC3339)
	}
	if len(firstReasons) == 0 && len(lastReasons) == 0 {
		if last.Before(first) {
			reasons = append(reasons, "gate_reputation.last_verified_at must not be before first_verified_at")
		} else {
			report.VerifiedDays = int(last.Sub(first).Hours() / 24)
		}
	}

	confirmations, confirmationReasons := normalizeIndependentConfirmations(reputation.IndependentConfirmations, example.Organization)
	reasons = append(reasons, confirmationReasons...)
	report.IndependentConfirmations = confirmations
	if len(reasons) > 0 {
		return report, reasons
	}

	report.ReproducibilityPoints = minInt(40, reputation.ReproducibleRuns*4)
	report.LongevityPoints = minInt(30, (report.VerifiedDays/30)*5)
	report.ConfirmationPoints = minInt(30, len(confirmations)*10)
	report.Score = report.ReproducibilityPoints + report.LongevityPoints + report.ConfirmationPoints
	report.Tier = gateReputationTier(report.Score)
	return report, nil
}

func parseGateReputationTime(field, value string) (time.Time, []string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, []string{field + " is required"}
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, []string{field + " must be RFC3339"}
	}
	return parsed, nil
}

func normalizeIndependentConfirmations(values []string, organization string) ([]string, []string) {
	var reasons []string
	organizationKey := reputationIdentityKey(organization)
	seen := map[string]string{}
	var out []string
	for i, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" {
			reasons = append(reasons, fmt.Sprintf("gate_reputation.independent_confirmations[%d] is empty", i))
			continue
		}
		key := reputationIdentityKey(value)
		if key == organizationKey && organizationKey != "" {
			reasons = append(reasons, "gate_reputation.independent_confirmations must not include the submitting organization")
			continue
		}
		if prior, ok := seen[key]; ok {
			reasons = append(reasons, "duplicate gate_reputation.independent_confirmations entry "+prior)
			continue
		}
		seen[key] = value
		out = append(out, value)
	}
	if len(out) == 0 {
		reasons = append(reasons, "gate_reputation.independent_confirmations must include at least one independent confirmation")
	}
	sort.Strings(out)
	return out, reasons
}

func reputationIdentityKey(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
}

func gateReputationTier(score int) string {
	switch {
	case score >= 75:
		return "established"
	case score >= 50:
		return "reviewable"
	default:
		return "emerging"
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func scanPublicStrings(scope string, values []string) []string {
	var reasons []string
	for _, value := range values {
		lower := strings.ToLower(value)
		for _, marker := range highSignalPrivateMarkers {
			if strings.Contains(lower, strings.ToLower(marker)) {
				reasons = append(reasons, scope+" contains private marker "+marker)
			}
		}
	}
	return reasons
}

func metadataStrings(example Example) []string {
	values := []string{
		example.ID,
		example.Title,
		example.Organization,
		example.Ecosystem,
		example.HazardClass,
		example.Source.Host,
		example.Source.Repo,
		example.Source.Ref,
		example.Source.Commit,
		example.Source.Subpath,
		example.Source.URL,
		example.LicenseSPDX,
		example.Consent,
		example.Redaction.Method,
		example.Redaction.Reviewer,
		example.Certificate.ID,
		example.Certificate.Issuer,
		example.Certificate.IssuedAt,
	}
	values = append(values, example.Redaction.Fields...)
	values = append(values, example.Certificate.Obligations...)
	values = append(values, example.Reproduction...)
	if example.GateReputation != nil {
		values = append(values,
			example.GateReputation.FirstVerifiedAt,
			example.GateReputation.LastVerifiedAt,
		)
		values = append(values, example.GateReputation.IndependentConfirmations...)
	}
	if example.Challenge != nil {
		values = append(values, challengeStrings(*example.Challenge)...)
	}
	values = append(values, example.Limitations...)
	return values
}

func certificateSubjectFor(example Example) certificateSubject {
	return certificateSubject{
		Version:      RegistryVersion,
		ExampleID:    strings.TrimSpace(example.ID),
		Source:       normalizeSource(example.Source),
		Ecosystem:    strings.TrimSpace(example.Ecosystem),
		HazardClass:  strings.TrimSpace(example.HazardClass),
		LicenseSPDX:  strings.TrimSpace(example.LicenseSPDX),
		Consent:      strings.TrimSpace(example.Consent),
		Redaction:    normalizeRedaction(example.Redaction),
		Artifacts:    normalizeArtifacts(example.Artifacts),
		Obligations:  normalizeStringList(example.Certificate.Obligations, true),
		Reproduction: normalizeStringList(example.Reproduction, false),
		Challenge:    normalizeChallengeSubmission(example.Challenge),
	}
}

func normalizeChallengeSubmission(challenge *ChallengeSubmission) *ChallengeSubmission {
	if challenge == nil {
		return nil
	}
	normalized := &ChallengeSubmission{
		TrackID:                  strings.TrimSpace(challenge.TrackID),
		AdversarialGoal:          strings.TrimSpace(challenge.AdversarialGoal),
		AttackSurface:            strings.TrimSpace(challenge.AttackSurface),
		ExpectedDetectorBehavior: strings.TrimSpace(challenge.ExpectedDetectorBehavior),
		MigrationArtifact:        filepath.ToSlash(strings.TrimSpace(challenge.MigrationArtifact)),
		MaxPublicProofLines:      challenge.MaxPublicProofLines,
		NoveltyStatement:         strings.TrimSpace(challenge.NoveltyStatement),
		Disclosure: ChallengeDisclosureStatus{
			Status:               strings.TrimSpace(challenge.Disclosure.Status),
			PublicReleaseAllowed: challenge.Disclosure.PublicReleaseAllowed,
			CoordinatedWith:      strings.TrimSpace(challenge.Disclosure.CoordinatedWith),
			ReportedAt:           strings.TrimSpace(challenge.Disclosure.ReportedAt),
			EmbargoEndsAt:        strings.TrimSpace(challenge.Disclosure.EmbargoEndsAt),
			FullExploitHash:      strings.TrimSpace(challenge.Disclosure.FullExploitHash),
		},
	}
	return normalized
}

func challengeStrings(challenge ChallengeSubmission) []string {
	return []string{
		challenge.TrackID,
		challenge.AdversarialGoal,
		challenge.AttackSurface,
		challenge.ExpectedDetectorBehavior,
		challenge.MigrationArtifact,
		challenge.NoveltyStatement,
		challenge.Disclosure.Status,
		challenge.Disclosure.CoordinatedWith,
		challenge.Disclosure.ReportedAt,
		challenge.Disclosure.EmbargoEndsAt,
		challenge.Disclosure.FullExploitHash,
	}
}

func normalizeSource(source Source) Source {
	return Source{
		Host:    strings.TrimSpace(source.Host),
		Repo:    strings.TrimSpace(source.Repo),
		Ref:     strings.TrimSpace(source.Ref),
		Commit:  strings.TrimSpace(source.Commit),
		Subpath: strings.TrimSpace(source.Subpath),
		URL:     strings.TrimSpace(source.URL),
	}
}

func normalizeRedaction(redaction Redaction) Redaction {
	return Redaction{
		Reviewed:      redaction.Reviewed,
		RawDataShared: redaction.RawDataShared,
		Method:        strings.TrimSpace(redaction.Method),
		Fields:        normalizeStringList(redaction.Fields, true),
		Reviewer:      strings.TrimSpace(redaction.Reviewer),
	}
}

func normalizeArtifacts(artifacts []Artifact) []Artifact {
	out := make([]Artifact, 0, len(artifacts))
	for _, artifact := range artifacts {
		out = append(out, Artifact{
			Path:     filepath.ToSlash(strings.TrimSpace(artifact.Path)),
			Role:     strings.TrimSpace(artifact.Role),
			SHA256:   strings.TrimSpace(artifact.SHA256),
			Redacted: artifact.Redacted,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Role < out[j].Role
	})
	return out
}

func normalizeStringList(values []string, sortValues bool) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	if sortValues {
		sort.Strings(out)
	}
	return out
}

func stringSet(values []string) map[string]bool {
	out := map[string]bool{}
	for _, value := range normalizeStringList(values, true) {
		out[value] = true
	}
	return out
}

func resolveArtifact(root, rel string) (string, error) {
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("absolute paths are not allowed")
	}
	clean := filepath.Clean(filepath.FromSlash(rel))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes registry root")
	}
	joined := filepath.Join(root, clean)
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	realPath, err := filepath.EvalSymlinks(joined)
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(realRoot, realPath)
	if err != nil {
		return "", err
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) || filepath.IsAbs(relative) {
		return "", fmt.Errorf("path escapes registry root")
	}
	return realPath, nil
}

func sha256File(path string) (string, int64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", 0, err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), int64(len(data)), nil
}

func duplicateAnalysisForExample(example Example, root string, artifacts []ArtifactSummary) (DuplicateAnalysisReport, []string) {
	exactArtifacts, cues, reasons := duplicateArtifactMaterial(root, artifacts)
	if len(reasons) > 0 {
		sort.Strings(reasons)
		return DuplicateAnalysisReport{}, reasons
	}
	if len(exactArtifacts) == 0 || len(cues) == 0 {
		return DuplicateAnalysisReport{}, []string{"duplicate detection requires at least one non-certificate redacted artifact"}
	}
	source := normalizeSource(example.Source)
	analysis := DuplicateAnalysisReport{
		ExactFingerprint: "sha256:" + canonical.Hash(exactDuplicateSubject{
			Version:     RegistryVersion,
			Kind:        "exact",
			Source:      source,
			Ecosystem:   strings.TrimSpace(example.Ecosystem),
			HazardClass: strings.TrimSpace(example.HazardClass),
			Artifacts:   exactArtifacts,
		}),
		NearFingerprint: "sha256:" + canonical.Hash(nearDuplicateSubject{
			Version: RegistryVersion,
			Kind:    "near",
			Source: nearDuplicateSource{
				Host:    source.Host,
				Repo:    source.Repo,
				Subpath: source.Subpath,
			},
			Ecosystem:   strings.TrimSpace(example.Ecosystem),
			HazardClass: strings.TrimSpace(example.HazardClass),
			Cues:        cues,
		}),
	}
	return analysis, nil
}

func duplicateArtifactMaterial(root string, artifacts []ArtifactSummary) ([]exactDuplicateArtifact, []string, []string) {
	selected := duplicateRelevantArtifacts(artifacts)
	var exact []exactDuplicateArtifact
	var cues []string
	var reasons []string
	for _, artifact := range selected {
		path, err := resolveArtifact(root, artifact.Path)
		if err != nil {
			reasons = append(reasons, "duplicate detection artifact "+artifact.Path+": "+err.Error())
			continue
		}
		content, err := os.ReadFile(path)
		if err != nil {
			reasons = append(reasons, "duplicate detection artifact "+artifact.Path+": "+err.Error())
			continue
		}
		exact = append(exact, exactDuplicateArtifact{Role: artifact.Role, SHA256: artifact.SHA256})
		cues = append(cues, nearDuplicateCue(content))
	}
	sort.Slice(exact, func(i, j int) bool {
		if exact[i].Role != exact[j].Role {
			return exact[i].Role < exact[j].Role
		}
		return exact[i].SHA256 < exact[j].SHA256
	})
	sort.Strings(cues)
	return exact, cues, reasons
}

func duplicateRelevantArtifacts(artifacts []ArtifactSummary) []ArtifactSummary {
	var hazard []ArtifactSummary
	for _, artifact := range artifacts {
		if artifact.Role == "redacted-hazard-example" {
			hazard = append(hazard, artifact)
		}
	}
	if len(hazard) > 0 {
		return hazard
	}
	var selected []ArtifactSummary
	for _, artifact := range artifacts {
		if artifact.Role != "certificate-witness" {
			selected = append(selected, artifact)
		}
	}
	return selected
}

func nearDuplicateCue(content []byte) string {
	var hazard duplicateHazardArtifact
	if err := json.Unmarshal(content, &hazard); err == nil {
		parts := []string{hazard.HazardClass}
		for _, evidence := range hazard.Evidence {
			parts = append(parts, evidence.Path, evidence.Snippet)
		}
		if len(hazard.Evidence) == 0 {
			parts = append(parts, hazard.Summary, hazard.Finding)
		}
		if normalized := normalizeNearDuplicateText(strings.Join(parts, "\n")); normalized != "" {
			return normalized
		}
	}
	return normalizeNearDuplicateText(string(content))
}

func normalizeNearDuplicateText(value string) string {
	value = strings.ToLower(value)
	value = anglePlaceholderRE.ReplaceAllString(value, " redacted ")
	value = decimalRE.ReplaceAllString(value, " 0 ")
	value = nonTokenRE.ReplaceAllString(value, " ")
	return strings.Join(strings.Fields(value), " ")
}

func applyDuplicateAnalysis(report *Report) {
	exactGroups := map[string][]int{}
	nearGroups := map[string][]int{}
	for i := range report.Examples {
		analysis := &report.Examples[i].DuplicateAnalysis
		analysis.ExactGroupSize = 1
		analysis.NearGroupSize = 1
		analysis.PrevalenceGroupKind = "unique"
		analysis.PrevalenceGroupID = duplicateGroupID("unique", analysis.NearFingerprint)
		analysis.PrevalenceRepresentative = true
		analysis.PrevalenceWeight = 1
		exactGroups[analysis.ExactFingerprint] = append(exactGroups[analysis.ExactFingerprint], i)
		nearGroups[analysis.NearFingerprint] = append(nearGroups[analysis.NearFingerprint], i)
	}

	exactGroupIDs := map[string]string{}
	exactGroupSizes := map[string]int{}
	for _, fingerprint := range sortedGroupKeys(exactGroups) {
		indices := exactGroups[fingerprint]
		if len(indices) <= 1 {
			continue
		}
		groupID := duplicateGroupID("exact", fingerprint)
		exactGroupIDs[fingerprint] = groupID
		exactGroupSizes[fingerprint] = len(indices)
		ids := exampleIDsFor(report.Examples, indices)
		report.DuplicateGroups = append(report.DuplicateGroups, DuplicateGroup{
			ID:                        groupID,
			Kind:                      "exact",
			Fingerprint:               fingerprint,
			RepresentativeID:          ids[0],
			ExampleIDs:                ids,
			Count:                     len(ids),
			DistinctExactFingerprints: 1,
		})
		report.Summary.ExactDuplicateGroups++
		for _, index := range indices {
			report.Examples[index].DuplicateAnalysis.ExactGroupID = groupID
			report.Examples[index].DuplicateAnalysis.ExactGroupSize = len(indices)
		}
	}

	nearGroupIDs := map[string]string{}
	nearGroupSizes := map[string]int{}
	nearGroupDistinctExact := map[string]int{}
	for _, fingerprint := range sortedGroupKeys(nearGroups) {
		indices := nearGroups[fingerprint]
		if len(indices) <= 1 {
			continue
		}
		distinctExact := distinctExactFingerprints(report.Examples, indices)
		if distinctExact <= 1 {
			continue
		}
		groupID := duplicateGroupID("near", fingerprint)
		nearGroupIDs[fingerprint] = groupID
		nearGroupSizes[fingerprint] = len(indices)
		nearGroupDistinctExact[fingerprint] = distinctExact
		ids := exampleIDsFor(report.Examples, indices)
		report.DuplicateGroups = append(report.DuplicateGroups, DuplicateGroup{
			ID:                        groupID,
			Kind:                      "near",
			Fingerprint:               fingerprint,
			RepresentativeID:          ids[0],
			ExampleIDs:                ids,
			Count:                     len(ids),
			DistinctExactFingerprints: distinctExact,
		})
		report.Summary.NearDuplicateGroups++
		for _, index := range indices {
			report.Examples[index].DuplicateAnalysis.NearGroupID = groupID
			report.Examples[index].DuplicateAnalysis.NearGroupSize = len(indices)
		}
	}

	for i := range report.Examples {
		example := &report.Examples[i]
		analysis := &example.DuplicateAnalysis
		groupKind := "unique"
		groupID := duplicateGroupID("unique", analysis.NearFingerprint)
		var indices []int
		if nearGroupID := nearGroupIDs[analysis.NearFingerprint]; nearGroupID != "" {
			groupKind = "near"
			groupID = nearGroupID
			indices = nearGroups[analysis.NearFingerprint]
		} else if exactGroupID := exactGroupIDs[analysis.ExactFingerprint]; exactGroupID != "" {
			groupKind = "exact"
			groupID = exactGroupID
			indices = exactGroups[analysis.ExactFingerprint]
		} else {
			indices = []int{i}
		}
		ids := exampleIDsFor(report.Examples, indices)
		representative := ids[0]
		analysis.PrevalenceGroupKind = groupKind
		analysis.PrevalenceGroupID = groupID
		analysis.PrevalenceRepresentative = example.ID == representative
		if analysis.PrevalenceRepresentative {
			analysis.PrevalenceWeight = 1
			analysis.DuplicateOf = ""
			report.Summary.PrevalenceExamples++
		} else {
			analysis.PrevalenceWeight = 0
			analysis.DuplicateOf = representative
		}
		if groupKind == "exact" {
			analysis.ExactGroupSize = exactGroupSizes[analysis.ExactFingerprint]
		}
		if groupKind == "near" {
			analysis.NearGroupSize = nearGroupSizes[analysis.NearFingerprint]
			_ = nearGroupDistinctExact[analysis.NearFingerprint]
		}
	}
	report.Summary.DuplicateInflation = report.Summary.Published - report.Summary.PrevalenceExamples
	sort.Slice(report.DuplicateGroups, func(i, j int) bool {
		return report.DuplicateGroups[i].ID < report.DuplicateGroups[j].ID
	})
}

func sortedGroupKeys(groups map[string][]int) []string {
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func exampleIDsFor(examples []PublishedExample, indices []int) []string {
	ids := make([]string, 0, len(indices))
	for _, index := range indices {
		ids = append(ids, examples[index].ID)
	}
	sort.Strings(ids)
	return ids
}

func distinctExactFingerprints(examples []PublishedExample, indices []int) int {
	seen := map[string]bool{}
	for _, index := range indices {
		seen[examples[index].DuplicateAnalysis.ExactFingerprint] = true
	}
	return len(seen)
}

func duplicateGroupID(kind, fingerprint string) string {
	fingerprint = strings.TrimPrefix(fingerprint, "sha256:")
	if len(fingerprint) > 16 {
		fingerprint = fingerprint[:16]
	}
	if fingerprint == "" {
		fingerprint = "missing"
	}
	return kind + "-" + fingerprint
}

func prevalenceExamples(examples []PublishedExample) []PublishedExample {
	out := make([]PublishedExample, 0, len(examples))
	for _, example := range examples {
		if example.DuplicateAnalysis.PrevalenceRepresentative {
			out = append(out, example)
		}
	}
	return out
}

func counts(examples []PublishedExample, key func(PublishedExample) string) []Count {
	values := map[string]int{}
	for _, example := range examples {
		values[key(example)]++
	}
	out := make([]Count, 0, len(values))
	for key, count := range values {
		out = append(out, Count{Key: key, Count: count})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Key < out[j].Key
	})
	return out
}

func stableRejectedID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return "missing-id"
	}
	return id
}

func buildArchiveMirror(marketplace Metadata, registryHash string, examples []PublishedExample) ArchiveMirror {
	mirror := ArchiveMirror{
		Version:      MirrorVersion,
		RegistryHash: strings.TrimSpace(registryHash),
		Marketplace:  marketplace,
		Entries:      []ArchiveMirrorEntry{},
		ByLicense:    []Count{},
	}
	exampleIDs := map[string]bool{}
	uniqueFiles := map[string]int64{}
	for _, example := range examples {
		exampleIDs[example.ID] = true
		for _, artifact := range example.Artifacts {
			entry := ArchiveMirrorEntry{
				ExampleID:              example.ID,
				ArtifactPath:           artifact.Path,
				ArtifactRole:           artifact.Role,
				MirrorPath:             archiveMirrorPath(artifact.SHA256),
				Checksum:               artifact.SHA256,
				Bytes:                  artifact.Bytes,
				LicenseSPDX:            example.LicenseSPDX,
				Redacted:               artifact.Redacted,
				Source:                 example.Source,
				CertificateSubjectHash: example.CertificateSubjectHash,
				EvidenceHash:           example.EvidenceHash,
				Withdrawal:             withdrawalMetadataFor(marketplace, example, artifact),
			}
			mirror.Entries = append(mirror.Entries, entry)
			mirror.Summary.Artifacts++
			mirror.Summary.Bytes += artifact.Bytes
			mirror.Summary.Active++
			if _, ok := uniqueFiles[artifact.SHA256]; !ok {
				uniqueFiles[artifact.SHA256] = artifact.Bytes
				mirror.Summary.UniqueBytes += artifact.Bytes
			}
		}
	}
	mirror.Summary.Examples = len(exampleIDs)
	mirror.Summary.UniqueFiles = len(uniqueFiles)
	sort.Slice(mirror.Entries, func(i, j int) bool {
		if mirror.Entries[i].ExampleID != mirror.Entries[j].ExampleID {
			return mirror.Entries[i].ExampleID < mirror.Entries[j].ExampleID
		}
		if mirror.Entries[i].ArtifactPath != mirror.Entries[j].ArtifactPath {
			return mirror.Entries[i].ArtifactPath < mirror.Entries[j].ArtifactPath
		}
		return mirror.Entries[i].ArtifactRole < mirror.Entries[j].ArtifactRole
	})
	mirror.ByLicense = archiveMirrorCounts(mirror.Entries, func(entry ArchiveMirrorEntry) string { return entry.LicenseSPDX })
	mirror.Hash = archiveMirrorHash(mirror)
	return mirror
}

func withdrawalMetadataFor(marketplace Metadata, example PublishedExample, artifact ArtifactSummary) WithdrawalMetadata {
	material := strings.Join([]string{example.ID, artifact.Path, artifact.SHA256}, "\n")
	sum := sha256.Sum256([]byte(material))
	return WithdrawalMetadata{
		Status:                          "active",
		Requested:                       false,
		WithdrawalID:                    "sha256:" + hex.EncodeToString(sum[:]),
		PolicyURL:                       strings.TrimSpace(marketplace.PolicyURL),
		Contact:                         strings.TrimSpace(marketplace.Maintainer),
		ReviewRequired:                  true,
		TombstoneRequired:               true,
		PreserveChecksumAfterWithdrawal: true,
		ReplacementAllowed:              true,
	}
}

func archiveMirrorPath(checksum string) string {
	return "archive/sha256/" + strings.TrimPrefix(strings.TrimSpace(checksum), "sha256:")
}

func archiveMirrorCounts(entries []ArchiveMirrorEntry, key func(ArchiveMirrorEntry) string) []Count {
	values := map[string]int{}
	for _, entry := range entries {
		values[key(entry)]++
	}
	out := make([]Count, 0, len(values))
	for key, count := range values {
		out = append(out, Count{Key: key, Count: count})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Key < out[j].Key
	})
	return out
}

func archiveMirrorHash(mirror ArchiveMirror) string {
	copy := mirror
	copy.Hash = ""
	return "sha256:" + canonical.Hash(copy)
}

func writeArchiveMirror(outDir, root string, mirror ArchiveMirror) error {
	if mirror.Version == "" {
		mirror = ArchiveMirror{Version: MirrorVersion, Entries: []ArchiveMirrorEntry{}, ByLicense: []Count{}}
		mirror.Hash = archiveMirrorHash(mirror)
	}
	if err := os.MkdirAll(filepath.Join(outDir, "archive", "sha256"), 0o755); err != nil {
		return err
	}
	if len(mirror.Entries) > 0 && strings.TrimSpace(root) == "" {
		return fmt.Errorf("registry root unavailable for archive mirror")
	}
	for _, entry := range mirror.Entries {
		if err := materializeArchiveMirrorEntry(outDir, root, entry); err != nil {
			return err
		}
	}
	return writeJSON(filepath.Join(outDir, "archive-mirror.json"), mirror)
}

func materializeArchiveMirrorEntry(outDir, root string, entry ArchiveMirrorEntry) error {
	if !strings.HasPrefix(entry.MirrorPath, "archive/sha256/") {
		return fmt.Errorf("archive mirror entry %s has invalid mirror path %q", entry.ArtifactPath, entry.MirrorPath)
	}
	sourcePath, err := resolveArtifact(root, entry.ArtifactPath)
	if err != nil {
		return fmt.Errorf("archive mirror artifact %s: %w", entry.ArtifactPath, err)
	}
	content, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("archive mirror artifact %s: %w", entry.ArtifactPath, err)
	}
	sum := sha256.Sum256(content)
	actualChecksum := "sha256:" + hex.EncodeToString(sum[:])
	if actualChecksum != entry.Checksum {
		return fmt.Errorf("archive mirror artifact %s sha256 drift: expected %s got %s", entry.ArtifactPath, entry.Checksum, actualChecksum)
	}
	if int64(len(content)) != entry.Bytes {
		return fmt.Errorf("archive mirror artifact %s byte count drift: expected %d got %d", entry.ArtifactPath, entry.Bytes, len(content))
	}
	destPath := filepath.Join(outDir, filepath.FromSlash(entry.MirrorPath))
	relative, err := filepath.Rel(outDir, destPath)
	if err != nil {
		return err
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) || filepath.IsAbs(relative) {
		return fmt.Errorf("archive mirror path escapes output directory")
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}
	if existing, err := os.ReadFile(destPath); err == nil {
		existingSum := sha256.Sum256(existing)
		existingChecksum := "sha256:" + hex.EncodeToString(existingSum[:])
		if existingChecksum != entry.Checksum {
			return fmt.Errorf("archive mirror destination %s checksum mismatch: expected %s got %s", entry.MirrorPath, entry.Checksum, existingChecksum)
		}
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.WriteFile(destPath, content, 0o644)
}

func reportHash(report Report) string {
	copy := report
	copy.Hash = ""
	copy.Markdown = ""
	return "sha256:" + canonical.Hash(copy)
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func RenderMarkdown(report Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Public evidence marketplace\n\n")
	fmt.Fprintf(&b, "Patchline publishes only redacted, certificate-backed hazard examples with clear licenses, reproducible commands, and a content-addressed archive mirror.\n\n")
	fmt.Fprintf(&b, "| Metric | Count |\n| --- | ---: |\n")
	fmt.Fprintf(&b, "| Submitted examples | %d |\n", report.Summary.Submitted)
	fmt.Fprintf(&b, "| Published examples | %d |\n", report.Summary.Published)
	fmt.Fprintf(&b, "| Duplicate-collapsed prevalence examples | %d |\n", report.Summary.PrevalenceExamples)
	fmt.Fprintf(&b, "| Duplicate inflation prevented | %d |\n", report.Summary.DuplicateInflation)
	fmt.Fprintf(&b, "| Exact duplicate groups | %d |\n", report.Summary.ExactDuplicateGroups)
	fmt.Fprintf(&b, "| Near-duplicate groups | %d |\n", report.Summary.NearDuplicateGroups)
	fmt.Fprintf(&b, "| Rejected examples | %d |\n", report.Summary.Rejected)
	fmt.Fprintf(&b, "| Verified artifacts | %d |\n", report.Summary.ArtifactsVerified)
	fmt.Fprintf(&b, "| Mirrored archive artifacts | %d |\n", report.Summary.MirroredArtifacts)
	fmt.Fprintf(&b, "| Archive mirror bytes | %d |\n", report.Summary.MirrorBytes)
	fmt.Fprintf(&b, "| Public-release eligible | %d |\n", report.Summary.PublicReleaseEligible)
	fmt.Fprintf(&b, "| Gate reputations submitted | %d |\n", report.Summary.GateReputationSubmitted)
	fmt.Fprintf(&b, "| Reviewable gate reputations | %d |\n", report.Summary.GateReputationReviewable)
	fmt.Fprintf(&b, "| Established gate reputations | %d |\n", report.Summary.GateReputationEstablished)
	fmt.Fprintf(&b, "\n## Published examples\n\n")
	if len(report.Examples) == 0 {
		fmt.Fprintf(&b, "No examples cleared publication checks.\n\n")
	} else {
		fmt.Fprintf(&b, "| ID | Hazard | Ecosystem | Prevalence | License | Release | Gate reputation | Certificate | Evidence |\n")
		fmt.Fprintf(&b, "| --- | --- | --- | ---: | --- | --- | ---: | --- | --- |\n")
		for _, example := range report.Examples {
			fmt.Fprintf(&b, "| `%s` | %s | %s | %d | `%s` | %t | %s (%d) | `%s` | `%s` |\n",
				escapePipe(example.ID),
				escapePipe(example.HazardClass),
				escapePipe(example.Ecosystem),
				example.DuplicateAnalysis.PrevalenceWeight,
				escapePipe(example.LicenseSPDX),
				example.ReleaseAdmission.PublicReleaseEligible,
				escapePipe(example.GateReputation.Tier),
				example.GateReputation.Score,
				escapePipe(example.CertificateSubjectHash),
				escapePipe(example.EvidenceHash),
			)
		}
		fmt.Fprintf(&b, "\n")
	}
	if len(report.DuplicateGroups) > 0 {
		fmt.Fprintf(&b, "## Duplicate groups\n\n")
		fmt.Fprintf(&b, "| Group | Kind | Representative | Count | Examples |\n")
		fmt.Fprintf(&b, "| --- | --- | --- | ---: | --- |\n")
		for _, group := range report.DuplicateGroups {
			fmt.Fprintf(&b, "| `%s` | %s | `%s` | %d | %s |\n",
				escapePipe(group.ID),
				escapePipe(group.Kind),
				escapePipe(group.RepresentativeID),
				group.Count,
				escapePipe(strings.Join(group.ExampleIDs, ", ")),
			)
		}
		fmt.Fprintf(&b, "\n")
	}
	if len(report.Rejected) > 0 {
		fmt.Fprintf(&b, "## Rejected examples\n\n")
		fmt.Fprintf(&b, "| ID | Reasons |\n| --- | --- |\n")
		for _, rejected := range report.Rejected {
			fmt.Fprintf(&b, "| `%s` | %s |\n", escapePipe(rejected.ID), escapePipe(strings.Join(rejected.Reasons, "; ")))
		}
	}
	return b.String()
}

func RenderHTML(report Report) (string, error) {
	const page = `<!doctype html>
<meta charset="utf-8">
<title>Patchline public evidence marketplace</title>
<h1>Patchline public evidence marketplace</h1>
<p>Published {{.Summary.Published}} redacted, certificate-backed hazard examples; {{.Summary.PrevalenceExamples}} duplicate-collapsed prevalence examples; mirrored {{.Summary.MirroredArtifacts}} archive artifacts; rejected {{.Summary.Rejected}}.</p>
<table>
<thead><tr><th>ID</th><th>Title</th><th>Hazard</th><th>Ecosystem</th><th>Prevalence</th><th>License</th><th>Release</th><th>Gate reputation</th><th>Certificate</th></tr></thead>
<tbody>
{{range .Examples}}<tr><td><code>{{.ID}}</code></td><td>{{.Title}}</td><td>{{.HazardClass}}</td><td>{{.Ecosystem}}</td><td>{{.DuplicateAnalysis.PrevalenceWeight}}</td><td><code>{{.LicenseSPDX}}</code></td><td>{{.ReleaseAdmission.PublicReleaseEligible}}</td><td>{{.GateReputation.Tier}} ({{.GateReputation.Score}})</td><td><code>{{.CertificateSubjectHash}}</code></td></tr>
{{end}}</tbody>
</table>
`
	tmpl, err := template.New("marketplace").Parse(page)
	if err != nil {
		return "", err
	}
	var b bytes.Buffer
	if err := tmpl.Execute(&b, report); err != nil {
		return "", err
	}
	return b.String(), nil
}

func escapePipe(value string) string {
	return strings.ReplaceAll(value, "|", "\\|")
}
