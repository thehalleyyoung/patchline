package certificationrenewal

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
	"time"
	"unicode"

	"github.com/thehalleyyoung/patchline/internal/canonical"
	"github.com/thehalleyyoung/patchline/internal/dbsemantics"
)

const SpecVersion = "patchline.certification-renewal/v1"
const ReportVersion = "patchline.certification-renewal-report/v1"

type Spec struct {
	Version         string                  `json:"version"`
	Name            string                  `json:"name"`
	Claim           string                  `json:"claim,omitempty"`
	AsOf            string                  `json:"as_of"`
	Criteria        Criteria                `json:"criteria"`
	EngineSemantics []EngineSemanticsUpdate `json:"engine_semantics"`
	HazardClasses   []HazardClassUpdate     `json:"hazard_classes"`
	Credentials     []Credential            `json:"credentials"`
	Attempts        []RenewalAttempt        `json:"attempts"`
}

type Criteria struct {
	MinEngineSemanticsUpdates int     `json:"min_engine_semantics_updates"`
	MinNewHazardClasses       int     `json:"min_new_hazard_classes"`
	PassingScorePercent       float64 `json:"passing_score_pct"`
	RequireEvidenceHashes     bool    `json:"require_evidence_hashes"`
	RequireReproducibleGates  bool    `json:"require_reproducible_gates"`
}

type EngineSemanticsUpdate struct {
	ID             string             `json:"id"`
	Engine         dbsemantics.Engine `json:"engine"`
	EngineVersion  string             `json:"engine_version"`
	EffectiveDate  string             `json:"effective_date"`
	Source         string             `json:"source"`
	Summary        string             `json:"summary"`
	RequiredTopics []string           `json:"required_topics"`
	EvidencePaths  []string           `json:"evidence_paths"`
}

type HazardClassUpdate struct {
	ID             string   `json:"id"`
	HazardClass    string   `json:"hazard_class"`
	DiscoveredAt   string   `json:"discovered_at"`
	Severity       string   `json:"severity"`
	Source         string   `json:"source"`
	Summary        string   `json:"summary"`
	RequiredTopics []string `json:"required_topics"`
	EvidencePaths  []string `json:"evidence_paths"`
}

type Credential struct {
	PractitionerID string `json:"practitioner_id"`
	CredentialID   string `json:"credential_id"`
	Status         string `json:"status"`
	IssuedAt       string `json:"issued_at"`
	ExpiresAt      string `json:"expires_at"`
	Track          string `json:"track"`
}

type RenewalAttempt struct {
	PractitionerID          string   `json:"practitioner_id"`
	CredentialID            string   `json:"credential_id"`
	SubmittedAt             string   `json:"submitted_at"`
	ScorePercent            float64  `json:"score_pct"`
	Gate                    string   `json:"gate"`
	Commands                []string `json:"commands"`
	CoveredEngineSemantics  []string `json:"covered_engine_semantics"`
	CoveredHazardClasses    []string `json:"covered_hazard_classes"`
	CoveredTopics           []string `json:"covered_topics"`
	EvidencePaths           []string `json:"evidence_paths"`
	ReviewerEvidenceHash    string   `json:"reviewer_evidence_hash"`
	ReviewerAttestationPath string   `json:"reviewer_attestation_path"`
}

type Report struct {
	Version         string                    `json:"version"`
	Name            string                    `json:"name"`
	AsOf            string                    `json:"as_of"`
	OK              bool                      `json:"ok"`
	Criteria        Criteria                  `json:"criteria"`
	Summary         Summary                   `json:"summary"`
	EngineSemantics []EngineSemanticsReport   `json:"engine_semantics"`
	HazardClasses   []HazardClassReport       `json:"hazard_classes"`
	Credentials     []CredentialRenewalReport `json:"credentials"`
	Attempts        []RenewalAttemptReport    `json:"attempts"`
	Counterexamples []Counterexample          `json:"counterexamples,omitempty"`
	Hash            string                    `json:"hash"`
}

type Summary struct {
	EngineSemanticsUpdates int     `json:"engine_semantics_updates"`
	NewHazardClasses       int     `json:"new_hazard_classes"`
	ActiveCredentials      int     `json:"active_credentials"`
	RenewedCredentials     int     `json:"renewed_credentials"`
	Attempts               int     `json:"attempts"`
	RequiredTopics         int     `json:"required_topics"`
	AverageScorePercent    float64 `json:"average_score_pct"`
	Counterexamples        int     `json:"counterexamples"`
}

type EngineSemanticsReport struct {
	ID             string             `json:"id"`
	Engine         dbsemantics.Engine `json:"engine"`
	EngineVersion  string             `json:"engine_version"`
	EffectiveDate  string             `json:"effective_date"`
	Source         string             `json:"source"`
	RequiredTopics []string           `json:"required_topics"`
	Evidence       []ArtifactEvidence `json:"evidence"`
}

type HazardClassReport struct {
	ID             string             `json:"id"`
	HazardClass    string             `json:"hazard_class"`
	DiscoveredAt   string             `json:"discovered_at"`
	Severity       string             `json:"severity"`
	Source         string             `json:"source"`
	RequiredTopics []string           `json:"required_topics"`
	Evidence       []ArtifactEvidence `json:"evidence"`
}

type CredentialRenewalReport struct {
	PractitionerID      string   `json:"practitioner_id"`
	CredentialID        string   `json:"credential_id"`
	Status              string   `json:"status"`
	Track               string   `json:"track"`
	RequiresRenewalFrom string   `json:"requires_renewal_from"`
	BestAttemptDate     string   `json:"best_attempt_date,omitempty"`
	BestAttemptScore    float64  `json:"best_attempt_score_pct,omitempty"`
	Renewed             bool     `json:"renewed"`
	MissingTopics       []string `json:"missing_topics,omitempty"`
	MissingEngineIDs    []string `json:"missing_engine_semantics,omitempty"`
	MissingHazardIDs    []string `json:"missing_hazard_classes,omitempty"`
}

type RenewalAttemptReport struct {
	PractitionerID              string             `json:"practitioner_id"`
	CredentialID                string             `json:"credential_id"`
	SubmittedAt                 string             `json:"submitted_at"`
	ScorePercent                float64            `json:"score_pct"`
	Gate                        string             `json:"gate"`
	GateBacked                  bool               `json:"gate_backed"`
	ReproducibleGateCommandOK   bool               `json:"reproducible_gate_command_ok"`
	CoversAllEngineSemantics    bool               `json:"covers_all_engine_semantics"`
	CoversAllHazardClasses      bool               `json:"covers_all_hazard_classes"`
	CoversAllRequiredTopics     bool               `json:"covers_all_required_topics"`
	SubmittedAfterLatestUpdate  bool               `json:"submitted_after_latest_update"`
	ReviewerEvidenceHashPresent bool               `json:"reviewer_evidence_hash_present"`
	ReviewerEvidenceHashMatches bool               `json:"reviewer_evidence_hash_matches"`
	Evidence                    []ArtifactEvidence `json:"evidence"`
	MissingEngineIDs            []string           `json:"missing_engine_semantics,omitempty"`
	MissingHazardIDs            []string           `json:"missing_hazard_classes,omitempty"`
	MissingTopics               []string           `json:"missing_topics,omitempty"`
	RequiredGateCommand         string             `json:"required_gate_command"`
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
		return Spec{}, fmt.Errorf("certification renewal spec version must be %s", SpecVersion)
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
	asOf, _ := parseDate(spec.AsOf)
	latestUpdate := latestUpdateDate(spec)
	requiredEngineIDs := sortedEngineIDs(spec.EngineSemantics)
	requiredHazardIDs := sortedHazardIDs(spec.HazardClasses)
	requiredTopics := requiredTopicSet(spec)

	report := Report{
		Version:  ReportVersion,
		Name:     spec.Name,
		AsOf:     spec.AsOf,
		OK:       true,
		Criteria: spec.Criteria,
	}

	for _, update := range sortedEngineSemantics(spec.EngineSemantics) {
		evidence, err := collectArtifacts(rootAbs, update.EvidencePaths)
		if err != nil {
			return Report{}, err
		}
		report.EngineSemantics = append(report.EngineSemantics, EngineSemanticsReport{
			ID:             update.ID,
			Engine:         update.Engine,
			EngineVersion:  update.EngineVersion,
			EffectiveDate:  update.EffectiveDate,
			Source:         update.Source,
			RequiredTopics: sortedStrings(update.RequiredTopics),
			Evidence:       evidence,
		})
	}
	for _, update := range sortedHazards(spec.HazardClasses) {
		evidence, err := collectArtifacts(rootAbs, update.EvidencePaths)
		if err != nil {
			return Report{}, err
		}
		report.HazardClasses = append(report.HazardClasses, HazardClassReport{
			ID:             update.ID,
			HazardClass:    update.HazardClass,
			DiscoveredAt:   update.DiscoveredAt,
			Severity:       update.Severity,
			Source:         update.Source,
			RequiredTopics: sortedStrings(update.RequiredTopics),
			Evidence:       evidence,
		})
	}

	attemptReportsByCredential := map[string][]RenewalAttemptReport{}
	for _, attempt := range sortedAttempts(spec.Attempts) {
		ar, err := buildAttemptReport(rootAbs, attempt, latestUpdate, requiredEngineIDs, requiredHazardIDs, requiredTopics)
		if err != nil {
			return Report{}, err
		}
		report.Attempts = append(report.Attempts, ar)
		key := credentialKey(attempt.PractitionerID, attempt.CredentialID)
		attemptReportsByCredential[key] = append(attemptReportsByCredential[key], ar)
		report.Counterexamples = append(report.Counterexamples, attemptCounterexamples(ar, spec.Criteria)...)
	}

	for _, credential := range sortedCredentials(spec.Credentials) {
		cr := CredentialRenewalReport{
			PractitionerID:      credential.PractitionerID,
			CredentialID:        credential.CredentialID,
			Status:              credential.Status,
			Track:               credential.Track,
			RequiresRenewalFrom: formatDate(latestUpdate),
			MissingEngineIDs:    append([]string{}, requiredEngineIDs...),
			MissingHazardIDs:    append([]string{}, requiredHazardIDs...),
			MissingTopics:       sortedSetKeys(requiredTopics),
		}
		if credential.Status == "active" {
			report.Summary.ActiveCredentials++
			for _, attempt := range attemptReportsByCredential[credentialKey(credential.PractitionerID, credential.CredentialID)] {
				if !attemptQualifies(attempt, spec.Criteria) {
					continue
				}
				if cr.BestAttemptDate == "" || attempt.SubmittedAt > cr.BestAttemptDate || (attempt.SubmittedAt == cr.BestAttemptDate && attempt.ScorePercent > cr.BestAttemptScore) {
					cr.BestAttemptDate = attempt.SubmittedAt
					cr.BestAttemptScore = attempt.ScorePercent
					cr.Renewed = true
					cr.MissingEngineIDs = nil
					cr.MissingHazardIDs = nil
					cr.MissingTopics = nil
				}
			}
			if !cr.Renewed {
				report.Counterexamples = append(report.Counterexamples, Counterexample{
					ID:      "credential." + stableID(credential.PractitionerID, credential.CredentialID) + ".renewal",
					Kind:    "credential_not_renewed",
					Subject: credential.CredentialID,
					Message: "active credential lacks a passing renewal attempt covering every new engine semantic, hazard class, and required topic after the latest update",
					Witness: append(append([]string{}, cr.MissingEngineIDs...), append(cr.MissingHazardIDs, cr.MissingTopics...)...),
				})
			} else {
				report.Summary.RenewedCredentials++
			}
		}
		report.Credentials = append(report.Credentials, cr)
	}

	if len(report.EngineSemantics) < spec.Criteria.MinEngineSemanticsUpdates {
		report.Counterexamples = append(report.Counterexamples, Counterexample{
			ID:      "criteria.min_engine_semantics_updates",
			Kind:    "insufficient_engine_semantics_updates",
			Message: fmt.Sprintf("engine semantics updates %d below required %d", len(report.EngineSemantics), spec.Criteria.MinEngineSemanticsUpdates),
		})
	}
	if len(report.HazardClasses) < spec.Criteria.MinNewHazardClasses {
		report.Counterexamples = append(report.Counterexamples, Counterexample{
			ID:      "criteria.min_new_hazard_classes",
			Kind:    "insufficient_new_hazard_classes",
			Message: fmt.Sprintf("new hazard classes %d below required %d", len(report.HazardClasses), spec.Criteria.MinNewHazardClasses),
		})
	}
	for _, update := range spec.EngineSemantics {
		date, _ := parseDate(update.EffectiveDate)
		if date.After(asOf) {
			report.Counterexamples = append(report.Counterexamples, Counterexample{
				ID:      "engine." + stableID(update.ID) + ".future",
				Kind:    "future_engine_semantics",
				Subject: update.ID,
				Message: "engine semantics update is dated after the report as_of date",
				Witness: []string{update.EffectiveDate, spec.AsOf},
			})
		}
	}
	for _, update := range spec.HazardClasses {
		date, _ := parseDate(update.DiscoveredAt)
		if date.After(asOf) {
			report.Counterexamples = append(report.Counterexamples, Counterexample{
				ID:      "hazard." + stableID(update.ID) + ".future",
				Kind:    "future_hazard_class",
				Subject: update.ID,
				Message: "hazard class update is dated after the report as_of date",
				Witness: []string{update.DiscoveredAt, spec.AsOf},
			})
		}
	}

	sortCounterexamples(report.Counterexamples)
	report.Summary.EngineSemanticsUpdates = len(report.EngineSemantics)
	report.Summary.NewHazardClasses = len(report.HazardClasses)
	report.Summary.Attempts = len(report.Attempts)
	report.Summary.RequiredTopics = len(requiredTopics)
	report.Summary.AverageScorePercent = averageScore(report.Attempts)
	report.Summary.Counterexamples = len(report.Counterexamples)
	report.OK = len(report.Counterexamples) == 0
	report.Hash = reportHash(report)
	return report, nil
}

func WriteArtifacts(outDir string, report Report) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	file, err := os.Create(filepath.Join(outDir, "certification-renewal.json"))
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
	return os.WriteFile(filepath.Join(outDir, "certification-renewal.md"), []byte(RenderMarkdown(report)), 0o644)
}

func RenderMarkdown(report Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Certification renewal\n\n")
	fmt.Fprintf(&b, "Patchline renews practitioner credentials only after new database-engine semantics and newly discovered hazard classes are covered by gate-backed evidence.\n\n")
	fmt.Fprintf(&b, "## Summary\n\n")
	fmt.Fprintf(&b, "| Metric | Value |\n| --- | ---: |\n")
	fmt.Fprintf(&b, "| OK | `%t` |\n", report.OK)
	fmt.Fprintf(&b, "| As of | `%s` |\n", report.AsOf)
	fmt.Fprintf(&b, "| Engine semantics updates | %d |\n", report.Summary.EngineSemanticsUpdates)
	fmt.Fprintf(&b, "| New hazard classes | %d |\n", report.Summary.NewHazardClasses)
	fmt.Fprintf(&b, "| Required topics | %d |\n", report.Summary.RequiredTopics)
	fmt.Fprintf(&b, "| Active credentials | %d |\n", report.Summary.ActiveCredentials)
	fmt.Fprintf(&b, "| Renewed credentials | %d |\n", report.Summary.RenewedCredentials)
	fmt.Fprintf(&b, "| Attempts | %d |\n", report.Summary.Attempts)
	fmt.Fprintf(&b, "| Average score | %.2f%% |\n", report.Summary.AverageScorePercent)
	fmt.Fprintf(&b, "| Counterexamples | %d |\n\n", report.Summary.Counterexamples)

	fmt.Fprintf(&b, "## Database-engine semantics tracked\n\n")
	fmt.Fprintf(&b, "| Update | Engine | Version | Effective | Topics | Evidence hashes |\n| --- | --- | --- | --- | --- | --- |\n")
	for _, update := range report.EngineSemantics {
		fmt.Fprintf(&b, "| `%s` | `%s` | `%s` | `%s` | %s | %s |\n",
			escapeTable(update.ID),
			escapeTable(string(update.Engine)),
			escapeTable(update.EngineVersion),
			escapeTable(update.EffectiveDate),
			escapeTable(strings.Join(update.RequiredTopics, ", ")),
			escapeTable(evidenceSummary(update.Evidence)),
		)
	}

	fmt.Fprintf(&b, "\n## Hazard classes tracked\n\n")
	fmt.Fprintf(&b, "| Update | Hazard | Severity | Discovered | Topics | Evidence hashes |\n| --- | --- | --- | --- | --- | --- |\n")
	for _, update := range report.HazardClasses {
		fmt.Fprintf(&b, "| `%s` | `%s` | `%s` | `%s` | %s | %s |\n",
			escapeTable(update.ID),
			escapeTable(update.HazardClass),
			escapeTable(update.Severity),
			escapeTable(update.DiscoveredAt),
			escapeTable(strings.Join(update.RequiredTopics, ", ")),
			escapeTable(evidenceSummary(update.Evidence)),
		)
	}

	fmt.Fprintf(&b, "\n## Credential renewal status\n\n")
	fmt.Fprintf(&b, "| Practitioner | Credential | Status | Renewed | Best attempt | Score |\n| --- | --- | --- | ---: | --- | ---: |\n")
	for _, credential := range report.Credentials {
		attemptDate := firstNonEmpty(credential.BestAttemptDate, "-")
		fmt.Fprintf(&b, "| `%s` | `%s` | `%s` | `%t` | `%s` | %.2f%% |\n",
			escapeTable(credential.PractitionerID),
			escapeTable(credential.CredentialID),
			escapeTable(credential.Status),
			credential.Renewed,
			escapeTable(attemptDate),
			credential.BestAttemptScore,
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

func validateSpec(spec Spec) error {
	if strings.TrimSpace(spec.Name) == "" {
		return fmt.Errorf("certification renewal name is required")
	}
	if _, err := parseDate(spec.AsOf); err != nil {
		return fmt.Errorf("as_of: %w", err)
	}
	if spec.Criteria.MinEngineSemanticsUpdates <= 0 {
		return fmt.Errorf("criteria.min_engine_semantics_updates must be positive")
	}
	if spec.Criteria.MinNewHazardClasses <= 0 {
		return fmt.Errorf("criteria.min_new_hazard_classes must be positive")
	}
	if spec.Criteria.PassingScorePercent <= 0 || spec.Criteria.PassingScorePercent > 100 {
		return fmt.Errorf("criteria.passing_score_pct must be between 0 and 100")
	}
	if len(spec.EngineSemantics) == 0 {
		return fmt.Errorf("engine_semantics are required")
	}
	if len(spec.HazardClasses) == 0 {
		return fmt.Errorf("hazard_classes are required")
	}
	if len(spec.Credentials) == 0 {
		return fmt.Errorf("credentials are required")
	}
	if len(spec.Attempts) == 0 {
		return fmt.Errorf("attempts are required")
	}
	engineIDs := map[string]struct{}{}
	for _, update := range spec.EngineSemantics {
		if strings.TrimSpace(update.ID) == "" {
			return fmt.Errorf("engine semantics id is required")
		}
		if _, exists := engineIDs[update.ID]; exists {
			return fmt.Errorf("duplicate engine semantics id %q", update.ID)
		}
		engineIDs[update.ID] = struct{}{}
		if !supportedEngine(update.Engine) {
			return fmt.Errorf("engine semantics %q uses unsupported engine %q", update.ID, update.Engine)
		}
		if strings.TrimSpace(update.EngineVersion) == "" || strings.TrimSpace(update.Source) == "" || strings.TrimSpace(update.Summary) == "" {
			return fmt.Errorf("engine semantics %q requires engine_version, source, and summary", update.ID)
		}
		if _, err := parseDate(update.EffectiveDate); err != nil {
			return fmt.Errorf("engine semantics %q effective_date: %w", update.ID, err)
		}
		if len(update.RequiredTopics) == 0 {
			return fmt.Errorf("engine semantics %q requires topics", update.ID)
		}
		if len(update.EvidencePaths) == 0 {
			return fmt.Errorf("engine semantics %q requires evidence paths", update.ID)
		}
		for _, path := range update.EvidencePaths {
			if err := validateRelativePath(path); err != nil {
				return fmt.Errorf("engine semantics %q evidence path: %w", update.ID, err)
			}
		}
	}
	hazardIDs := map[string]struct{}{}
	for _, update := range spec.HazardClasses {
		if strings.TrimSpace(update.ID) == "" {
			return fmt.Errorf("hazard class id is required")
		}
		if _, exists := hazardIDs[update.ID]; exists {
			return fmt.Errorf("duplicate hazard class id %q", update.ID)
		}
		hazardIDs[update.ID] = struct{}{}
		if strings.TrimSpace(update.HazardClass) == "" || strings.TrimSpace(update.Severity) == "" || strings.TrimSpace(update.Source) == "" || strings.TrimSpace(update.Summary) == "" {
			return fmt.Errorf("hazard class %q requires hazard_class, severity, source, and summary", update.ID)
		}
		if _, err := parseDate(update.DiscoveredAt); err != nil {
			return fmt.Errorf("hazard class %q discovered_at: %w", update.ID, err)
		}
		if len(update.RequiredTopics) == 0 {
			return fmt.Errorf("hazard class %q requires topics", update.ID)
		}
		if len(update.EvidencePaths) == 0 {
			return fmt.Errorf("hazard class %q requires evidence paths", update.ID)
		}
		for _, path := range update.EvidencePaths {
			if err := validateRelativePath(path); err != nil {
				return fmt.Errorf("hazard class %q evidence path: %w", update.ID, err)
			}
		}
	}
	credentials := map[string]struct{}{}
	for _, credential := range spec.Credentials {
		if strings.TrimSpace(credential.PractitionerID) == "" || strings.TrimSpace(credential.CredentialID) == "" {
			return fmt.Errorf("credential requires practitioner_id and credential_id")
		}
		if credential.Status != "active" && credential.Status != "expired" && credential.Status != "revoked" {
			return fmt.Errorf("credential %q status must be active, expired, or revoked", credential.CredentialID)
		}
		if strings.TrimSpace(credential.Track) == "" {
			return fmt.Errorf("credential %q requires track", credential.CredentialID)
		}
		if _, err := parseDate(credential.IssuedAt); err != nil {
			return fmt.Errorf("credential %q issued_at: %w", credential.CredentialID, err)
		}
		if _, err := parseDate(credential.ExpiresAt); err != nil {
			return fmt.Errorf("credential %q expires_at: %w", credential.CredentialID, err)
		}
		key := credentialKey(credential.PractitionerID, credential.CredentialID)
		if _, exists := credentials[key]; exists {
			return fmt.Errorf("duplicate credential %q for practitioner %q", credential.CredentialID, credential.PractitionerID)
		}
		credentials[key] = struct{}{}
	}
	seenAttempts := map[string]struct{}{}
	for _, attempt := range spec.Attempts {
		if strings.TrimSpace(attempt.PractitionerID) == "" || strings.TrimSpace(attempt.CredentialID) == "" {
			return fmt.Errorf("attempt requires practitioner_id and credential_id")
		}
		if _, exists := credentials[credentialKey(attempt.PractitionerID, attempt.CredentialID)]; !exists {
			return fmt.Errorf("attempt references unknown credential %q for practitioner %q", attempt.CredentialID, attempt.PractitionerID)
		}
		if _, err := parseDate(attempt.SubmittedAt); err != nil {
			return fmt.Errorf("attempt %s/%s submitted_at: %w", attempt.PractitionerID, attempt.CredentialID, err)
		}
		if attempt.ScorePercent < 0 || attempt.ScorePercent > 100 {
			return fmt.Errorf("attempt %s/%s score_pct must be between 0 and 100", attempt.PractitionerID, attempt.CredentialID)
		}
		if strings.TrimSpace(attempt.Gate) == "" {
			return fmt.Errorf("attempt %s/%s requires gate", attempt.PractitionerID, attempt.CredentialID)
		}
		if len(attempt.EvidencePaths) == 0 {
			return fmt.Errorf("attempt %s/%s requires evidence paths", attempt.PractitionerID, attempt.CredentialID)
		}
		for _, path := range attempt.EvidencePaths {
			if err := validateRelativePath(path); err != nil {
				return fmt.Errorf("attempt %s/%s evidence path: %w", attempt.PractitionerID, attempt.CredentialID, err)
			}
		}
		for _, id := range attempt.CoveredEngineSemantics {
			if _, exists := engineIDs[id]; !exists {
				return fmt.Errorf("attempt %s/%s references unknown engine semantics %q", attempt.PractitionerID, attempt.CredentialID, id)
			}
		}
		for _, id := range attempt.CoveredHazardClasses {
			if _, exists := hazardIDs[id]; !exists {
				return fmt.Errorf("attempt %s/%s references unknown hazard class %q", attempt.PractitionerID, attempt.CredentialID, id)
			}
		}
		key := credentialKey(attempt.PractitionerID, attempt.CredentialID) + "\x00" + attempt.SubmittedAt
		if _, exists := seenAttempts[key]; exists {
			return fmt.Errorf("duplicate renewal attempt for %s/%s on %s", attempt.PractitionerID, attempt.CredentialID, attempt.SubmittedAt)
		}
		seenAttempts[key] = struct{}{}
	}
	return nil
}

func buildAttemptReport(root string, attempt RenewalAttempt, latestUpdate time.Time, requiredEngineIDs, requiredHazardIDs []string, requiredTopics map[string]struct{}) (RenewalAttemptReport, error) {
	evidence, err := collectArtifacts(root, append(attempt.EvidencePaths, nonEmpty(attempt.ReviewerAttestationPath)...))
	if err != nil {
		return RenewalAttemptReport{}, err
	}
	submittedAt, _ := parseDate(attempt.SubmittedAt)
	missingEngine := missingStrings(requiredEngineIDs, attempt.CoveredEngineSemantics)
	missingHazard := missingStrings(requiredHazardIDs, attempt.CoveredHazardClasses)
	missingTopics := missingSet(requiredTopics, attempt.CoveredTopics)
	report := RenewalAttemptReport{
		PractitionerID:              attempt.PractitionerID,
		CredentialID:                attempt.CredentialID,
		SubmittedAt:                 attempt.SubmittedAt,
		ScorePercent:                round2(attempt.ScorePercent),
		Gate:                        attempt.Gate,
		GateBacked:                  gateExists(root, attempt.Gate),
		ReproducibleGateCommandOK:   containsCommand(attempt.Commands, requiredGateCommand(attempt.Gate)),
		CoversAllEngineSemantics:    len(missingEngine) == 0,
		CoversAllHazardClasses:      len(missingHazard) == 0,
		CoversAllRequiredTopics:     len(missingTopics) == 0,
		SubmittedAfterLatestUpdate:  !submittedAt.Before(latestUpdate),
		ReviewerEvidenceHashPresent: len(strings.TrimSpace(attempt.ReviewerEvidenceHash)) == 64,
		ReviewerEvidenceHashMatches: matchesEvidenceHash(evidence, attempt.ReviewerEvidenceHash),
		Evidence:                    evidence,
		MissingEngineIDs:            missingEngine,
		MissingHazardIDs:            missingHazard,
		MissingTopics:               missingTopics,
		RequiredGateCommand:         requiredGateCommand(attempt.Gate),
	}
	return report, nil
}

func attemptCounterexamples(attempt RenewalAttemptReport, criteria Criteria) []Counterexample {
	var counterexamples []Counterexample
	subject := attempt.PractitionerID + "/" + attempt.CredentialID
	if attempt.ScorePercent < criteria.PassingScorePercent {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "attempt." + stableID(subject, attempt.SubmittedAt) + ".score",
			Kind:    "renewal_score_below_threshold",
			Subject: subject,
			Message: fmt.Sprintf("renewal score %.2f%% below required %.2f%%", attempt.ScorePercent, criteria.PassingScorePercent),
			Witness: []string{fmt.Sprintf("%.2f", attempt.ScorePercent)},
		})
	}
	if !attempt.SubmittedAfterLatestUpdate {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "attempt." + stableID(subject, attempt.SubmittedAt) + ".stale",
			Kind:    "stale_renewal_attempt",
			Subject: subject,
			Message: "renewal attempt predates the latest declared engine semantic or hazard-class update",
			Witness: []string{attempt.SubmittedAt},
		})
	}
	if criteria.RequireReproducibleGates && !attempt.GateBacked {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "attempt." + stableID(subject, attempt.SubmittedAt) + ".gate",
			Kind:    "renewal_gate_unbacked",
			Subject: subject,
			Message: "renewal attempt gate is not present as a Make target backed by a script",
			Witness: []string{attempt.Gate},
		})
	}
	if criteria.RequireReproducibleGates && !attempt.ReproducibleGateCommandOK {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "attempt." + stableID(subject, attempt.SubmittedAt) + ".command",
			Kind:    "missing_reproducible_renewal_command",
			Subject: subject,
			Message: "renewal attempt does not list the gate command required to reproduce the renewal evidence",
			Witness: []string{attempt.RequiredGateCommand},
		})
	}
	if criteria.RequireEvidenceHashes && !attempt.ReviewerEvidenceHashPresent {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "attempt." + stableID(subject, attempt.SubmittedAt) + ".reviewer_hash",
			Kind:    "missing_reviewer_evidence_hash",
			Subject: subject,
			Message: "renewal attempt lacks a 64-character reviewer evidence hash",
		})
	} else if criteria.RequireEvidenceHashes && !attempt.ReviewerEvidenceHashMatches {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "attempt." + stableID(subject, attempt.SubmittedAt) + ".reviewer_hash_match",
			Kind:    "reviewer_evidence_hash_mismatch",
			Subject: subject,
			Message: "renewal attempt reviewer hash does not match any hashed renewal evidence artifact",
		})
	}
	if len(attempt.MissingEngineIDs) > 0 {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "attempt." + stableID(subject, attempt.SubmittedAt) + ".engine_semantics",
			Kind:    "missing_engine_semantics_coverage",
			Subject: subject,
			Message: "renewal attempt does not cover every declared database-engine semantics update",
			Witness: attempt.MissingEngineIDs,
		})
	}
	if len(attempt.MissingHazardIDs) > 0 {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "attempt." + stableID(subject, attempt.SubmittedAt) + ".hazard_classes",
			Kind:    "missing_hazard_class_coverage",
			Subject: subject,
			Message: "renewal attempt does not cover every newly discovered hazard class",
			Witness: attempt.MissingHazardIDs,
		})
	}
	if len(attempt.MissingTopics) > 0 {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "attempt." + stableID(subject, attempt.SubmittedAt) + ".topics",
			Kind:    "missing_renewal_topics",
			Subject: subject,
			Message: "renewal attempt does not cover every topic required by the semantics and hazard updates",
			Witness: attempt.MissingTopics,
		})
	}
	return counterexamples
}

func attemptQualifies(attempt RenewalAttemptReport, criteria Criteria) bool {
	return attempt.ScorePercent >= criteria.PassingScorePercent &&
		attempt.SubmittedAfterLatestUpdate &&
		attempt.CoversAllEngineSemantics &&
		attempt.CoversAllHazardClasses &&
		attempt.CoversAllRequiredTopics &&
		(!criteria.RequireReproducibleGates || (attempt.GateBacked && attempt.ReproducibleGateCommandOK)) &&
		(!criteria.RequireEvidenceHashes || (attempt.ReviewerEvidenceHashPresent && attempt.ReviewerEvidenceHashMatches))
}

func collectArtifacts(root string, paths []string) ([]ArtifactEvidence, error) {
	var artifacts []ArtifactEvidence
	for _, relPath := range sortedStrings(paths) {
		fullPath, err := safeJoin(root, relPath)
		if err != nil {
			return nil, err
		}
		data, err := os.ReadFile(fullPath)
		if err != nil {
			return nil, fmt.Errorf("read certification renewal evidence %s: %w", relPath, err)
		}
		if len(data) == 0 {
			return nil, fmt.Errorf("certification renewal evidence %s is empty", relPath)
		}
		sum := sha256.Sum256(data)
		artifacts = append(artifacts, ArtifactEvidence{
			Path:   filepath.ToSlash(filepath.Clean(relPath)),
			SHA256: hex.EncodeToString(sum[:]),
			Bytes:  int64(len(data)),
		})
	}
	return artifacts, nil
}

func gateExists(root, gate string) bool {
	if strings.TrimSpace(gate) == "" {
		return false
	}
	makefile, err := os.ReadFile(filepath.Join(root, "Makefile"))
	if err != nil {
		return false
	}
	target := gate + ":"
	foundTarget := false
	for _, line := range strings.Split(string(makefile), "\n") {
		if strings.TrimSpace(line) == target {
			foundTarget = true
			break
		}
	}
	if !foundTarget {
		return false
	}
	info, err := os.Stat(filepath.Join(root, "scripts", gate+".sh"))
	return err == nil && !info.IsDir() && info.Size() > 0
}

func latestUpdateDate(spec Spec) time.Time {
	var latest time.Time
	for _, update := range spec.EngineSemantics {
		date, _ := parseDate(update.EffectiveDate)
		if date.After(latest) {
			latest = date
		}
	}
	for _, update := range spec.HazardClasses {
		date, _ := parseDate(update.DiscoveredAt)
		if date.After(latest) {
			latest = date
		}
	}
	return latest
}

func requiredTopicSet(spec Spec) map[string]struct{} {
	topics := map[string]struct{}{}
	for _, update := range spec.EngineSemantics {
		for _, topic := range update.RequiredTopics {
			if norm := normalizeToken(topic); norm != "" {
				topics[norm] = struct{}{}
			}
		}
	}
	for _, update := range spec.HazardClasses {
		for _, topic := range update.RequiredTopics {
			if norm := normalizeToken(topic); norm != "" {
				topics[norm] = struct{}{}
			}
		}
	}
	return topics
}

func missingSet(required map[string]struct{}, covered []string) []string {
	have := normalizedSet(covered)
	var missing []string
	for topic := range required {
		if _, ok := have[topic]; !ok {
			missing = append(missing, topic)
		}
	}
	sort.Strings(missing)
	return missing
}

func missingStrings(required, covered []string) []string {
	have := map[string]struct{}{}
	for _, value := range covered {
		have[value] = struct{}{}
	}
	var missing []string
	for _, value := range required {
		if _, ok := have[value]; !ok {
			missing = append(missing, value)
		}
	}
	sort.Strings(missing)
	return missing
}

func sortedEngineIDs(updates []EngineSemanticsUpdate) []string {
	ids := make([]string, 0, len(updates))
	for _, update := range updates {
		ids = append(ids, update.ID)
	}
	sort.Strings(ids)
	return ids
}

func sortedHazardIDs(updates []HazardClassUpdate) []string {
	ids := make([]string, 0, len(updates))
	for _, update := range updates {
		ids = append(ids, update.ID)
	}
	sort.Strings(ids)
	return ids
}

func parseDate(value string) (time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, fmt.Errorf("date is required")
	}
	date, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, fmt.Errorf("must be YYYY-MM-DD")
	}
	return date, nil
}

func formatDate(date time.Time) string {
	if date.IsZero() {
		return ""
	}
	return date.Format("2006-01-02")
}

func reportHash(report Report) string {
	copy := report
	copy.Hash = ""
	return canonical.Hash(copy)
}

func averageScore(attempts []RenewalAttemptReport) float64 {
	if len(attempts) == 0 {
		return 0
	}
	var total float64
	for _, attempt := range attempts {
		total += attempt.ScorePercent
	}
	return round2(total / float64(len(attempts)))
}

func supportedEngine(engine dbsemantics.Engine) bool {
	for _, supported := range dbsemantics.SupportedEngines() {
		if engine == supported {
			return true
		}
	}
	return false
}

func safeJoin(root, relPath string) (string, error) {
	if err := validateRelativePath(relPath); err != nil {
		return "", err
	}
	fullPath := filepath.Join(root, filepath.Clean(relPath))
	rootWithSep := root
	if !strings.HasSuffix(rootWithSep, string(os.PathSeparator)) {
		rootWithSep += string(os.PathSeparator)
	}
	if fullPath != root && !strings.HasPrefix(fullPath, rootWithSep) {
		return "", fmt.Errorf("path escapes root: %s", relPath)
	}
	return fullPath, nil
}

func validateRelativePath(path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("empty path")
	}
	if filepath.IsAbs(path) {
		return fmt.Errorf("absolute paths are not allowed")
	}
	clean := filepath.Clean(path)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("path escapes root: %s", path)
	}
	return nil
}

func requiredGateCommand(gate string) string {
	return "make " + strings.TrimSpace(gate)
}

func containsCommand(commands []string, want string) bool {
	want = strings.TrimSpace(want)
	for _, command := range commands {
		if strings.TrimSpace(command) == want {
			return true
		}
	}
	return false
}

func sortedEngineSemantics(updates []EngineSemanticsUpdate) []EngineSemanticsUpdate {
	out := append([]EngineSemanticsUpdate{}, updates...)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func sortedHazards(updates []HazardClassUpdate) []HazardClassUpdate {
	out := append([]HazardClassUpdate{}, updates...)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func sortedCredentials(credentials []Credential) []Credential {
	out := append([]Credential{}, credentials...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].PractitionerID != out[j].PractitionerID {
			return out[i].PractitionerID < out[j].PractitionerID
		}
		return out[i].CredentialID < out[j].CredentialID
	})
	return out
}

func sortedAttempts(attempts []RenewalAttempt) []RenewalAttempt {
	out := append([]RenewalAttempt{}, attempts...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].PractitionerID != out[j].PractitionerID {
			return out[i].PractitionerID < out[j].PractitionerID
		}
		if out[i].CredentialID != out[j].CredentialID {
			return out[i].CredentialID < out[j].CredentialID
		}
		return out[i].SubmittedAt < out[j].SubmittedAt
	})
	return out
}

func sortedStrings(values []string) []string {
	out := append([]string{}, values...)
	sort.Strings(out)
	return out
}

func sortedSetKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for value := range values {
		keys = append(keys, value)
	}
	sort.Strings(keys)
	return keys
}

func sortCounterexamples(counterexamples []Counterexample) {
	sort.Slice(counterexamples, func(i, j int) bool { return counterexamples[i].ID < counterexamples[j].ID })
}

func normalizedSet(values []string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, value := range values {
		if norm := normalizeToken(value); norm != "" {
			out[norm] = struct{}{}
		}
	}
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

func stableID(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])[:12]
}

func credentialKey(practitionerID, credentialID string) string {
	return practitionerID + "\x00" + credentialID
}

func round2(value float64) float64 {
	if value >= 0 {
		return float64(int(value*100+0.5)) / 100
	}
	return float64(int(value*100-0.5)) / 100
}

func nonEmpty(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return []string{value}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func evidenceSummary(evidence []ArtifactEvidence) string {
	parts := make([]string, 0, len(evidence))
	for _, item := range evidence {
		hash := item.SHA256
		if len(hash) > 16 {
			hash = hash[:16]
		}
		parts = append(parts, item.Path+":"+hash)
	}
	sort.Strings(parts)
	return strings.Join(parts, "; ")
}

func matchesEvidenceHash(evidence []ArtifactEvidence, hash string) bool {
	hash = strings.TrimSpace(hash)
	if len(hash) != 64 {
		return false
	}
	for _, item := range evidence {
		if item.SHA256 == hash {
			return true
		}
	}
	return false
}

func escapeTable(value string) string {
	return strings.ReplaceAll(value, "|", "\\|")
}
