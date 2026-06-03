package ethicsreview

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/thehalleyyoung/patchline/internal/canonical"
)

const SpecVersion = "patchline.ethics-review-template/v1"
const ReportVersion = "patchline.ethics-review-template-report/v1"

var requiredReviewAreas = []string{"new_data_source", "live_feedback_loop", "adopter_outcome_study"}

type Spec struct {
	Version  string   `json:"version"`
	Name     string   `json:"name"`
	AsOfDate string   `json:"as_of_date"`
	Criteria Criteria `json:"criteria"`
	Entries  []Entry  `json:"entries"`
}

type Criteria struct {
	RequiredReviewAreas                     []string `json:"required_review_areas"`
	MinIndependentReviewers                 int      `json:"min_independent_reviewers"`
	MaxRiskScore                            float64  `json:"max_risk_score"`
	ReviewCadenceDays                       int      `json:"review_cadence_days"`
	RequireConsentBasis                     bool     `json:"require_consent_basis"`
	RequirePrivacyReview                    bool     `json:"require_privacy_review"`
	RequireRetentionPolicy                  bool     `json:"require_retention_policy"`
	RequireMinimization                     bool     `json:"require_minimization"`
	RequireWithdrawalPath                   bool     `json:"require_withdrawal_path"`
	RequireSecurityOwner                    bool     `json:"require_security_owner"`
	RequireEvidencePaths                    bool     `json:"require_evidence_paths"`
	RequireHumanOversightForFeedback        bool     `json:"require_human_oversight_for_feedback"`
	RequirePreregistrationForOutcomeStudies bool     `json:"require_preregistration_for_outcome_studies"`
	MinMitigationsPerHighRiskEntry          int      `json:"min_mitigations_per_high_risk_entry"`
}

type Entry struct {
	ReviewID        string   `json:"review_id"`
	Area            string   `json:"area"`
	Title           string   `json:"title"`
	Owner           string   `json:"owner"`
	DataSources     []string `json:"data_sources"`
	Purpose         string   `json:"purpose"`
	RiskScore       float64  `json:"risk_score"`
	LastReviewed    string   `json:"last_reviewed"`
	ReviewerRoles   []string `json:"reviewer_roles"`
	ConsentBasis    string   `json:"consent_basis,omitempty"`
	PrivacyReview   string   `json:"privacy_review,omitempty"`
	RetentionPolicy string   `json:"retention_policy,omitempty"`
	Minimization    string   `json:"minimization,omitempty"`
	WithdrawalPath  string   `json:"withdrawal_path,omitempty"`
	SecurityOwner   string   `json:"security_owner,omitempty"`
	HumanOversight  string   `json:"human_oversight,omitempty"`
	Preregistration string   `json:"preregistration,omitempty"`
	Mitigations     []string `json:"mitigations,omitempty"`
	EvidencePaths   []string `json:"evidence_paths,omitempty"`
}

type Report struct {
	Version         string           `json:"version"`
	Name            string           `json:"name"`
	AsOfDate        string           `json:"as_of_date"`
	OK              bool             `json:"ok"`
	Criteria        Criteria         `json:"criteria"`
	Summary         Summary          `json:"summary"`
	Areas           []AreaReport     `json:"areas"`
	Counterexamples []Counterexample `json:"counterexamples,omitempty"`
	Hash            string           `json:"hash"`
}

type Summary struct {
	Areas                   int     `json:"areas"`
	Entries                 int     `json:"entries"`
	EvidenceFiles           int     `json:"evidence_files"`
	HighRiskEntries         int     `json:"high_risk_entries"`
	StaleReviews            int     `json:"stale_reviews"`
	MissingRequiredFields   int     `json:"missing_required_fields"`
	MissingEvidenceEntries  int     `json:"missing_evidence_entries"`
	MinIndependentReviewers int     `json:"min_independent_reviewers"`
	MaxRiskScore            float64 `json:"max_risk_score"`
	Counterexamples         int     `json:"counterexamples"`
}

type AreaReport struct {
	Area            string             `json:"area"`
	Entries         int                `json:"entries"`
	HighRiskEntries int                `json:"high_risk_entries"`
	StaleReviews    int                `json:"stale_reviews"`
	Evidence        []ArtifactEvidence `json:"evidence"`
	Reviews         []EntryReport      `json:"reviews"`
}

type EntryReport struct {
	ReviewID              string             `json:"review_id"`
	Area                  string             `json:"area"`
	Title                 string             `json:"title"`
	Owner                 string             `json:"owner"`
	DataSources           []string           `json:"data_sources"`
	Purpose               string             `json:"purpose"`
	RiskScore             float64            `json:"risk_score"`
	LastReviewed          string             `json:"last_reviewed"`
	ReviewAgeDays         int                `json:"review_age_days"`
	ReviewerRoles         []string           `json:"reviewer_roles"`
	IndependentReviewers  int                `json:"independent_reviewers"`
	ConsentBasis          string             `json:"consent_basis,omitempty"`
	PrivacyReview         string             `json:"privacy_review,omitempty"`
	RetentionPolicy       string             `json:"retention_policy,omitempty"`
	Minimization          string             `json:"minimization,omitempty"`
	WithdrawalPath        string             `json:"withdrawal_path,omitempty"`
	SecurityOwner         string             `json:"security_owner,omitempty"`
	HumanOversight        string             `json:"human_oversight,omitempty"`
	Preregistration       string             `json:"preregistration,omitempty"`
	Mitigations           []string           `json:"mitigations,omitempty"`
	Evidence              []ArtifactEvidence `json:"evidence"`
	HighRisk              bool               `json:"high_risk"`
	StaleReview           bool               `json:"stale_review"`
	MissingRequiredFields []string           `json:"missing_required_fields,omitempty"`
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

type areaAccumulator struct {
	area     string
	reviews  []EntryReport
	evidence map[string]ArtifactEvidence
}

func ReadSpec(reader io.Reader) (Spec, error) {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var spec Spec
	if err := decoder.Decode(&spec); err != nil {
		return Spec{}, err
	}
	if spec.Version != SpecVersion {
		return Spec{}, fmt.Errorf("ethics review template spec version must be %s", SpecVersion)
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
	asOf := mustParseTime(spec.AsOfDate)
	report := Report{
		Version:  ReportVersion,
		Name:     spec.Name,
		AsOfDate: spec.AsOfDate,
		OK:       true,
		Criteria: spec.Criteria,
	}
	accs := map[string]*areaAccumulator{}
	evidenceSeen := map[string]ArtifactEvidence{}
	var counterexamples []Counterexample

	for _, entry := range sortedEntries(spec.Entries) {
		entry.Area = normalizeArea(entry.Area)
		entryReport, entryCounterexamples := evaluateEntry(spec.Criteria, entry, rootAbs, asOf)
		counterexamples = append(counterexamples, entryCounterexamples...)
		acc := accs[entry.Area]
		if acc == nil {
			acc = &areaAccumulator{area: entry.Area, evidence: map[string]ArtifactEvidence{}}
			accs[entry.Area] = acc
		}
		addEntry(acc, entryReport)
		for _, evidence := range entryReport.Evidence {
			evidenceSeen[evidence.Path] = evidence
		}
		report.Summary.Entries++
	}

	report.Areas = finalizeAreas(accs)
	report.Summary = summarize(report.Summary, report.Areas, evidenceSeen)
	counterexamples = append(counterexamples, criteriaCounterexamples(spec.Criteria, report.Areas)...)
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
	jsonFile, err := os.Create(filepath.Join(outDir, "ethics-review-template.json"))
	if err != nil {
		return err
	}
	if err := canonical.WriteJSON(jsonFile, report); err != nil {
		_ = jsonFile.Close()
		return err
	}
	if err := jsonFile.Close(); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "ethics-review-template.md"), []byte(RenderMarkdown(report)), 0o644)
}

func RenderMarkdown(report Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Ethics review template\n\n")
	fmt.Fprintf(&b, "Patchline gate-checks new data sources, live feedback loops, and adopter outcome studies before they can become benchmark, calibration, or claims evidence.\n\n")
	fmt.Fprintf(&b, "## Summary\n\n")
	fmt.Fprintf(&b, "| Metric | Value |\n| --- | ---: |\n")
	fmt.Fprintf(&b, "| OK | `%t` |\n", report.OK)
	fmt.Fprintf(&b, "| As of | `%s` |\n", report.AsOfDate)
	fmt.Fprintf(&b, "| Areas | %d |\n", report.Summary.Areas)
	fmt.Fprintf(&b, "| Reviews | %d |\n", report.Summary.Entries)
	fmt.Fprintf(&b, "| Evidence files | %d |\n", report.Summary.EvidenceFiles)
	fmt.Fprintf(&b, "| High-risk reviews | %d |\n", report.Summary.HighRiskEntries)
	fmt.Fprintf(&b, "| Stale reviews | %d |\n", report.Summary.StaleReviews)
	fmt.Fprintf(&b, "| Missing required fields | %d |\n", report.Summary.MissingRequiredFields)
	fmt.Fprintf(&b, "| Missing evidence reviews | %d |\n", report.Summary.MissingEvidenceEntries)
	fmt.Fprintf(&b, "| Min independent reviewers | %d |\n", report.Summary.MinIndependentReviewers)
	fmt.Fprintf(&b, "| Max risk score | %.4f |\n", report.Summary.MaxRiskScore)
	fmt.Fprintf(&b, "| Counterexamples | %d |\n\n", report.Summary.Counterexamples)
	fmt.Fprintf(&b, "Policy: required areas `%s`, at least `%d` independent reviewer roles, risk score at most `%.2f`, review cadence `%d` days, and high-risk reviews require `%d` mitigations.\n\n",
		strings.Join(report.Criteria.RequiredReviewAreas, ", "),
		report.Criteria.MinIndependentReviewers,
		report.Criteria.MaxRiskScore,
		report.Criteria.ReviewCadenceDays,
		report.Criteria.MinMitigationsPerHighRiskEntry,
	)

	fmt.Fprintf(&b, "## Template obligations\n\n")
	fmt.Fprintf(&b, "| Area | Reviews | High risk | Stale | Evidence |\n| --- | ---: | ---: | ---: | ---: |\n")
	for _, area := range report.Areas {
		fmt.Fprintf(&b, "| `%s` | %d | %d | %d | %d |\n", area.Area, area.Entries, area.HighRiskEntries, area.StaleReviews, len(area.Evidence))
	}

	fmt.Fprintf(&b, "\n## Reviews\n\n")
	fmt.Fprintf(&b, "| Area | Review | Owner | Risk | Reviewers | Evidence | Missing fields |\n| --- | --- | --- | ---: | ---: | ---: | ---: |\n")
	for _, area := range report.Areas {
		for _, review := range area.Reviews {
			fmt.Fprintf(&b, "| `%s` | `%s` | `%s` | %.4f | %d | %d | %d |\n",
				review.Area,
				escapePipes(review.ReviewID),
				escapePipes(review.Owner),
				review.RiskScore,
				review.IndependentReviewers,
				len(review.Evidence),
				len(review.MissingRequiredFields),
			)
		}
	}

	if len(report.Counterexamples) > 0 {
		fmt.Fprintf(&b, "\n## Counterexamples\n\n")
		fmt.Fprintf(&b, "| ID | Kind | Subject | Message |\n| --- | --- | --- | --- |\n")
		for _, counterexample := range report.Counterexamples {
			fmt.Fprintf(&b, "| `%s` | `%s` | `%s` | %s |\n", counterexample.ID, counterexample.Kind, firstNonEmpty(counterexample.Subject, "-"), escapePipes(counterexample.Message))
		}
	}
	return b.String()
}

func evaluateEntry(criteria Criteria, entry Entry, rootAbs string, asOf time.Time) (EntryReport, []Counterexample) {
	lastReviewed := mustParseTime(entry.LastReviewed)
	ageDays := int(math.Floor(asOf.Sub(lastReviewed).Hours() / 24))
	report := EntryReport{
		ReviewID:             strings.TrimSpace(entry.ReviewID),
		Area:                 normalizeArea(entry.Area),
		Title:                strings.TrimSpace(entry.Title),
		Owner:                strings.TrimSpace(entry.Owner),
		DataSources:          sortedNonEmpty(entry.DataSources),
		Purpose:              strings.TrimSpace(entry.Purpose),
		RiskScore:            round4(entry.RiskScore),
		LastReviewed:         entry.LastReviewed,
		ReviewAgeDays:        ageDays,
		ReviewerRoles:        sortedNonEmpty(entry.ReviewerRoles),
		IndependentReviewers: len(distinctNormalized(entry.ReviewerRoles)),
		ConsentBasis:         strings.TrimSpace(entry.ConsentBasis),
		PrivacyReview:        strings.TrimSpace(entry.PrivacyReview),
		RetentionPolicy:      strings.TrimSpace(entry.RetentionPolicy),
		Minimization:         strings.TrimSpace(entry.Minimization),
		WithdrawalPath:       strings.TrimSpace(entry.WithdrawalPath),
		SecurityOwner:        strings.TrimSpace(entry.SecurityOwner),
		HumanOversight:       strings.TrimSpace(entry.HumanOversight),
		Preregistration:      strings.TrimSpace(entry.Preregistration),
		Mitigations:          sortedNonEmpty(entry.Mitigations),
	}
	var counterexamples []Counterexample
	if lastReviewed.After(asOf) {
		counterexamples = append(counterexamples, Counterexample{
			ID:      fmt.Sprintf("review-in-future-%s", safeID(entry.ReviewID)),
			Kind:    "review_in_future",
			Subject: entry.ReviewID,
			Message: "last_reviewed must not be after the ethics review as_of_date",
			Witness: []string{entry.LastReviewed, asOf.Format(time.RFC3339)},
		})
	}
	if criteria.ReviewCadenceDays > 0 && ageDays > criteria.ReviewCadenceDays {
		report.StaleReview = true
		counterexamples = append(counterexamples, Counterexample{
			ID:      fmt.Sprintf("stale-review-%s", safeID(entry.ReviewID)),
			Kind:    "stale_review",
			Subject: entry.ReviewID,
			Message: fmt.Sprintf("ethics review age %d days exceeds cadence %d days", ageDays, criteria.ReviewCadenceDays),
			Witness: []string{entry.LastReviewed, asOf.Format(time.RFC3339)},
		})
	}
	if report.RiskScore > criteria.MaxRiskScore {
		report.HighRisk = true
		counterexamples = append(counterexamples, Counterexample{
			ID:      fmt.Sprintf("risk-score-exceeded-%s", safeID(entry.ReviewID)),
			Kind:    "risk_score_exceeded",
			Subject: entry.ReviewID,
			Message: fmt.Sprintf("risk score %.4f exceeds allowed maximum %.4f", report.RiskScore, criteria.MaxRiskScore),
			Witness: []string{fmt.Sprintf("%.4f", report.RiskScore), fmt.Sprintf("%.4f", criteria.MaxRiskScore)},
		})
	}
	if report.HighRisk && len(report.Mitigations) < criteria.MinMitigationsPerHighRiskEntry {
		counterexamples = append(counterexamples, Counterexample{
			ID:      fmt.Sprintf("insufficient-high-risk-mitigations-%s", safeID(entry.ReviewID)),
			Kind:    "insufficient_high_risk_mitigations",
			Subject: entry.ReviewID,
			Message: fmt.Sprintf("high-risk ethics review has %d mitigations, below required %d", len(report.Mitigations), criteria.MinMitigationsPerHighRiskEntry),
		})
	}
	if report.IndependentReviewers < criteria.MinIndependentReviewers {
		counterexamples = append(counterexamples, Counterexample{
			ID:      fmt.Sprintf("insufficient-independent-reviewers-%s", safeID(entry.ReviewID)),
			Kind:    "insufficient_independent_reviewers",
			Subject: entry.ReviewID,
			Message: fmt.Sprintf("ethics review has %d independent reviewer roles, below required %d", report.IndependentReviewers, criteria.MinIndependentReviewers),
		})
	}

	for _, field := range missingRequiredFields(criteria, report) {
		report.MissingRequiredFields = append(report.MissingRequiredFields, field)
		counterexamples = append(counterexamples, Counterexample{
			ID:      fmt.Sprintf("missing-required-field-%s-%s", safeID(entry.ReviewID), safeID(field)),
			Kind:    "missing_required_field",
			Subject: entry.ReviewID,
			Message: fmt.Sprintf("ethics review is missing required field %s", field),
			Witness: []string{field},
		})
	}
	if criteria.RequireHumanOversightForFeedback && report.Area == "live_feedback_loop" && report.HumanOversight == "" {
		counterexamples = append(counterexamples, Counterexample{
			ID:      fmt.Sprintf("missing-human-oversight-%s", safeID(entry.ReviewID)),
			Kind:    "missing_human_oversight",
			Subject: entry.ReviewID,
			Message: "live feedback loops must name the human oversight path before learning or calibration results are trusted",
		})
	}
	if criteria.RequirePreregistrationForOutcomeStudies && report.Area == "adopter_outcome_study" && report.Preregistration == "" {
		counterexamples = append(counterexamples, Counterexample{
			ID:      fmt.Sprintf("missing-preregistration-%s", safeID(entry.ReviewID)),
			Kind:    "missing_preregistration",
			Subject: entry.ReviewID,
			Message: "adopter outcome studies must preserve a preregistration or study protocol before results are used as claims evidence",
		})
	}

	seen := map[string]bool{}
	for _, relPath := range sortedStrings(entry.EvidencePaths) {
		clean := filepath.Clean(strings.TrimSpace(relPath))
		if seen[clean] {
			continue
		}
		seen[clean] = true
		evidence, fileCounterexamples := resolveFileUnderRoot(rootAbs, relPath, entry.ReviewID, "ethics_evidence")
		counterexamples = append(counterexamples, fileCounterexamples...)
		if evidence != nil {
			report.Evidence = append(report.Evidence, *evidence)
		}
	}
	if criteria.RequireEvidencePaths && len(report.Evidence) == 0 {
		counterexamples = append(counterexamples, Counterexample{
			ID:      fmt.Sprintf("missing-entry-evidence-%s", safeID(entry.ReviewID)),
			Kind:    "missing_entry_evidence",
			Subject: entry.ReviewID,
			Message: "ethics review must preserve at least one readable evidence file",
		})
	}
	sort.Strings(report.MissingRequiredFields)
	return report, counterexamples
}

func missingRequiredFields(criteria Criteria, report EntryReport) []string {
	required := map[string]string{
		"title":        report.Title,
		"owner":        report.Owner,
		"purpose":      report.Purpose,
		"data_sources": strings.Join(report.DataSources, ","),
	}
	if criteria.RequireConsentBasis {
		required["consent_basis"] = report.ConsentBasis
	}
	if criteria.RequirePrivacyReview {
		required["privacy_review"] = report.PrivacyReview
	}
	if criteria.RequireRetentionPolicy {
		required["retention_policy"] = report.RetentionPolicy
	}
	if criteria.RequireMinimization {
		required["minimization"] = report.Minimization
	}
	if criteria.RequireWithdrawalPath {
		required["withdrawal_path"] = report.WithdrawalPath
	}
	if criteria.RequireSecurityOwner {
		required["security_owner"] = report.SecurityOwner
	}
	var missing []string
	for field, value := range required {
		if strings.TrimSpace(value) == "" {
			missing = append(missing, field)
		}
	}
	sort.Strings(missing)
	return missing
}

func addEntry(acc *areaAccumulator, entry EntryReport) {
	acc.reviews = append(acc.reviews, entry)
	for _, evidence := range entry.Evidence {
		acc.evidence[evidence.Path] = evidence
	}
}

func finalizeAreas(accs map[string]*areaAccumulator) []AreaReport {
	var areas []AreaReport
	for _, acc := range accs {
		area := AreaReport{
			Area:     acc.area,
			Entries:  len(acc.reviews),
			Evidence: sortedEvidence(acc.evidence),
			Reviews:  sortedEntryReports(acc.reviews),
		}
		for _, review := range area.Reviews {
			if review.HighRisk {
				area.HighRiskEntries++
			}
			if review.StaleReview {
				area.StaleReviews++
			}
		}
		areas = append(areas, area)
	}
	sort.Slice(areas, func(i, j int) bool {
		return areas[i].Area < areas[j].Area
	})
	return areas
}

func summarize(summary Summary, areas []AreaReport, evidenceSeen map[string]ArtifactEvidence) Summary {
	summary.Areas = len(areas)
	summary.EvidenceFiles = len(evidenceSeen)
	maxInt := int(^uint(0) >> 1)
	summary.MinIndependentReviewers = maxInt
	for _, area := range areas {
		summary.HighRiskEntries += area.HighRiskEntries
		summary.StaleReviews += area.StaleReviews
		for _, review := range area.Reviews {
			if review.IndependentReviewers < summary.MinIndependentReviewers {
				summary.MinIndependentReviewers = review.IndependentReviewers
			}
			if review.RiskScore > summary.MaxRiskScore {
				summary.MaxRiskScore = review.RiskScore
			}
			summary.MissingRequiredFields += len(review.MissingRequiredFields)
			if len(review.Evidence) == 0 {
				summary.MissingEvidenceEntries++
			}
		}
	}
	if summary.Entries == 0 {
		summary.MinIndependentReviewers = 0
	}
	summary.MaxRiskScore = round4(summary.MaxRiskScore)
	return summary
}

func criteriaCounterexamples(criteria Criteria, areas []AreaReport) []Counterexample {
	var counterexamples []Counterexample
	areaByName := map[string]AreaReport{}
	for _, area := range areas {
		areaByName[area.Area] = area
	}
	for _, required := range criteria.RequiredReviewAreas {
		required = normalizeArea(required)
		if _, ok := areaByName[required]; !ok {
			counterexamples = append(counterexamples, Counterexample{
				ID:      fmt.Sprintf("missing-required-area-%s", safeID(required)),
				Kind:    "missing_required_area",
				Subject: required,
				Message: "ethics review template must include every required review area",
			})
		}
	}
	return counterexamples
}

func validateSpec(spec Spec) error {
	if spec.Version != SpecVersion {
		return fmt.Errorf("ethics review template spec version must be %s", SpecVersion)
	}
	if strings.TrimSpace(spec.Name) == "" {
		return fmt.Errorf("ethics review template name is required")
	}
	if _, err := time.Parse(time.RFC3339, spec.AsOfDate); err != nil {
		return fmt.Errorf("as_of_date must be RFC3339: %v", err)
	}
	if err := validateCriteria(spec.Criteria); err != nil {
		return err
	}
	if len(spec.Entries) == 0 {
		return fmt.Errorf("at least one ethics review entry is required")
	}
	seenReviews := map[string]bool{}
	for _, entry := range spec.Entries {
		if strings.TrimSpace(entry.ReviewID) == "" {
			return fmt.Errorf("entry review_id is required")
		}
		reviewKey := normalizeIdentity(entry.ReviewID)
		if seenReviews[reviewKey] {
			return fmt.Errorf("duplicate review_id %q", entry.ReviewID)
		}
		seenReviews[reviewKey] = true
		area := normalizeArea(entry.Area)
		if !allowedArea(area) {
			return fmt.Errorf("entry %q area must be one of %s", entry.ReviewID, strings.Join(requiredReviewAreas, ", "))
		}
		if entry.RiskScore < 0 || entry.RiskScore > 1 || !isFinite(entry.RiskScore) {
			return fmt.Errorf("entry %q risk_score must be finite and between 0 and 1", entry.ReviewID)
		}
		if _, err := time.Parse(time.RFC3339, entry.LastReviewed); err != nil {
			return fmt.Errorf("entry %q last_reviewed must be RFC3339: %v", entry.ReviewID, err)
		}
	}
	return nil
}

func validateCriteria(criteria Criteria) error {
	required := map[string]bool{}
	for _, area := range criteria.RequiredReviewAreas {
		normalized := normalizeArea(area)
		if !allowedArea(normalized) {
			return fmt.Errorf("criteria.required_review_areas contains unsupported area %q", area)
		}
		required[normalized] = true
	}
	for _, area := range requiredReviewAreas {
		if !required[area] {
			return fmt.Errorf("criteria.required_review_areas must include %q", area)
		}
	}
	if criteria.MinIndependentReviewers < 2 {
		return fmt.Errorf("criteria.min_independent_reviewers must be at least 2")
	}
	if criteria.MaxRiskScore <= 0 || criteria.MaxRiskScore > 1 || !isFinite(criteria.MaxRiskScore) {
		return fmt.Errorf("criteria.max_risk_score must be finite and in (0,1]")
	}
	if criteria.ReviewCadenceDays < 1 {
		return fmt.Errorf("criteria.review_cadence_days must be at least 1")
	}
	if criteria.MinMitigationsPerHighRiskEntry < 0 {
		return fmt.Errorf("criteria.min_mitigations_per_high_risk_entry must be non-negative")
	}
	return nil
}

func resolveFileUnderRoot(rootAbs, relPath, subject, kind string) (*ArtifactEvidence, []Counterexample) {
	clean := filepath.Clean(strings.TrimSpace(relPath))
	if clean == "" || clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return nil, []Counterexample{{
			ID:      fmt.Sprintf("%s-path-%s-%s", strings.ReplaceAll(kind, "_", "-"), safeID(subject), safeID(relPath)),
			Kind:    "invalid_evidence_path",
			Subject: subject,
			Message: fmt.Sprintf("%s path %q must be a relative file below the review root", strings.ReplaceAll(kind, "_", " "), relPath),
			Witness: []string{relPath},
		}}
	}
	artifactPath := filepath.Join(rootAbs, clean)
	info, err := os.Lstat(artifactPath)
	if err != nil {
		return nil, []Counterexample{{
			ID:      fmt.Sprintf("missing-%s-%s-%s", strings.ReplaceAll(kind, "_", "-"), safeID(subject), safeID(clean)),
			Kind:    "missing_evidence",
			Subject: subject,
			Message: fmt.Sprintf("%s file %q is missing", strings.ReplaceAll(kind, "_", " "), clean),
			Witness: []string{clean},
		}}
	}
	if !info.Mode().IsRegular() {
		return nil, []Counterexample{{
			ID:      fmt.Sprintf("invalid-%s-file-%s-%s", strings.ReplaceAll(kind, "_", "-"), safeID(subject), safeID(clean)),
			Kind:    "invalid_evidence_file",
			Subject: subject,
			Message: fmt.Sprintf("%s file %q must be a regular file under the review root", strings.ReplaceAll(kind, "_", " "), clean),
			Witness: []string{clean},
		}}
	}
	rootReal, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		return nil, []Counterexample{{
			ID:      fmt.Sprintf("invalid-%s-root-%s", strings.ReplaceAll(kind, "_", "-"), safeID(subject)),
			Kind:    "invalid_evidence_root",
			Subject: subject,
			Message: fmt.Sprintf("review root %q could not be resolved without symlinks: %v", rootAbs, err),
			Witness: []string{rootAbs},
		}}
	}
	artifactReal, err := filepath.EvalSymlinks(artifactPath)
	if err != nil {
		return nil, []Counterexample{{
			ID:      fmt.Sprintf("unreadable-%s-%s-%s", strings.ReplaceAll(kind, "_", "-"), safeID(subject), safeID(clean)),
			Kind:    "unreadable_evidence",
			Subject: subject,
			Message: fmt.Sprintf("%s file %q could not be resolved without symlinks: %v", strings.ReplaceAll(kind, "_", " "), clean, err),
			Witness: []string{clean},
		}}
	}
	relToRoot, err := filepath.Rel(rootReal, artifactReal)
	if err != nil || relToRoot == ".." || strings.HasPrefix(relToRoot, ".."+string(filepath.Separator)) || filepath.IsAbs(relToRoot) {
		return nil, []Counterexample{{
			ID:      fmt.Sprintf("escaped-%s-file-%s-%s", strings.ReplaceAll(kind, "_", "-"), safeID(subject), safeID(clean)),
			Kind:    "invalid_evidence_file",
			Subject: subject,
			Message: fmt.Sprintf("%s file %q resolves outside the review root", strings.ReplaceAll(kind, "_", " "), clean),
			Witness: []string{clean, artifactReal, rootReal},
		}}
	}
	bytes, err := os.ReadFile(artifactPath)
	if err != nil {
		return nil, []Counterexample{{
			ID:      fmt.Sprintf("unreadable-%s-%s-%s", strings.ReplaceAll(kind, "_", "-"), safeID(subject), safeID(clean)),
			Kind:    "unreadable_evidence",
			Subject: subject,
			Message: fmt.Sprintf("%s file %q could not be read: %v", strings.ReplaceAll(kind, "_", " "), clean, err),
			Witness: []string{clean},
		}}
	}
	sum := sha256.Sum256(bytes)
	return &ArtifactEvidence{Path: filepath.ToSlash(clean), SHA256: hex.EncodeToString(sum[:]), Bytes: info.Size()}, nil
}

func reportHash(report Report) string {
	copyReport := report
	copyReport.Hash = ""
	return canonical.Hash(copyReport)
}

func sortedEntries(entries []Entry) []Entry {
	sorted := append([]Entry(nil), entries...)
	sort.Slice(sorted, func(i, j int) bool {
		left := sorted[i]
		right := sorted[j]
		if normalizeArea(left.Area) != normalizeArea(right.Area) {
			return normalizeArea(left.Area) < normalizeArea(right.Area)
		}
		return left.ReviewID < right.ReviewID
	})
	return sorted
}

func sortedEntryReports(entries []EntryReport) []EntryReport {
	sorted := append([]EntryReport(nil), entries...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Area != sorted[j].Area {
			return sorted[i].Area < sorted[j].Area
		}
		return sorted[i].ReviewID < sorted[j].ReviewID
	})
	return sorted
}

func sortedEvidence(evidence map[string]ArtifactEvidence) []ArtifactEvidence {
	var paths []string
	for path := range evidence {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	var sorted []ArtifactEvidence
	for _, path := range paths {
		sorted = append(sorted, evidence[path])
	}
	return sorted
}

func sortedStrings(values []string) []string {
	sorted := append([]string(nil), values...)
	sort.Strings(sorted)
	return sorted
}

func sortedNonEmpty(values []string) []string {
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}

func distinctNormalized(values []string) map[string]bool {
	distinct := map[string]bool{}
	for _, value := range values {
		normalized := normalizeIdentity(value)
		if normalized != "" {
			distinct[normalized] = true
		}
	}
	return distinct
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

func allowedArea(area string) bool {
	for _, required := range requiredReviewAreas {
		if area == required {
			return true
		}
	}
	return false
}

func normalizeArea(area string) string {
	area = strings.ToLower(strings.TrimSpace(area))
	area = strings.ReplaceAll(area, "-", "_")
	area = strings.ReplaceAll(area, " ", "_")
	return area
}

func normalizeIdentity(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func mustParseTime(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		panic(err)
	}
	return parsed
}

func round4(value float64) float64 {
	return math.Round(value*10000) / 10000
}

func isFinite(value float64) bool {
	return !math.IsInf(value, 0) && !math.IsNaN(value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func escapePipes(value string) string {
	return strings.ReplaceAll(value, "|", "\\|")
}

func safeID(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
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
