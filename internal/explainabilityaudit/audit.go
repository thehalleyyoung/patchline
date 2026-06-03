package explainabilityaudit

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

	"github.com/thehalleyyoung/patchline/internal/canonical"
)

const SpecVersion = "patchline.explainability-audit/v1"
const ReportVersion = "patchline.explainability-audit-report/v1"

var validJudgments = map[string]bool{
	"supported":   true,
	"partial":     true,
	"unsupported": true,
}

type Spec struct {
	Version  string   `json:"version"`
	Name     string   `json:"name"`
	Criteria Criteria `json:"criteria"`
	Reviews  []Review `json:"reviews"`
}

type Criteria struct {
	MinReviewers                int     `json:"min_reviewers"`
	MinVerdicts                 int     `json:"min_verdicts"`
	MinReviewsPerVerdict        int     `json:"min_reviews_per_verdict"`
	MinAgreementRate            float64 `json:"min_agreement_rate"`
	MinSupportedRate            float64 `json:"min_supported_rate"`
	MaxUnsupportedRate          float64 `json:"max_unsupported_rate"`
	RequireIndependentReviewers bool    `json:"require_independent_reviewers"`
}

type Review struct {
	ReviewID               string            `json:"review_id"`
	VerdictID              string            `json:"verdict_id"`
	ReviewerID             string            `json:"reviewer_id"`
	ReviewerRole           string            `json:"reviewer_role"`
	Independent            bool              `json:"independent"`
	StatedVerdict          string            `json:"stated_verdict"`
	Judgment               string            `json:"judgment"`
	EvidencePaths          []string          `json:"evidence_paths"`
	ExpectedEvidenceHashes map[string]string `json:"expected_evidence_hashes,omitempty"`
	MissingEvidenceNotes   string            `json:"missing_evidence_notes,omitempty"`
	Rationale              string            `json:"rationale"`
}

type Report struct {
	Version         string           `json:"version"`
	Name            string           `json:"name"`
	OK              bool             `json:"ok"`
	Criteria        Criteria         `json:"criteria"`
	Summary         Summary          `json:"summary"`
	Verdicts        []VerdictReport  `json:"verdicts"`
	Reviewers       []ReviewerReport `json:"reviewers"`
	Counterexamples []Counterexample `json:"counterexamples,omitempty"`
	Hash            string           `json:"hash"`
}

type Summary struct {
	Reviews              int     `json:"reviews"`
	Reviewers            int     `json:"reviewers"`
	IndependentReviewers int     `json:"independent_reviewers"`
	Verdicts             int     `json:"verdicts"`
	EvidenceFiles        int     `json:"evidence_files"`
	SupportedReviews     int     `json:"supported_reviews"`
	PartialReviews       int     `json:"partial_reviews"`
	UnsupportedReviews   int     `json:"unsupported_reviews"`
	SupportedRate        float64 `json:"supported_rate"`
	UnsupportedRate      float64 `json:"unsupported_rate"`
	AverageAgreementRate float64 `json:"average_agreement_rate"`
	MinAgreementRate     float64 `json:"min_agreement_rate"`
	Counterexamples      int     `json:"counterexamples"`
}

type VerdictReport struct {
	VerdictID       string             `json:"verdict_id"`
	StatedVerdicts  []string           `json:"stated_verdicts"`
	Reviews         int                `json:"reviews"`
	Reviewers       int                `json:"reviewers"`
	Supported       int                `json:"supported"`
	Partial         int                `json:"partial"`
	Unsupported     int                `json:"unsupported"`
	SupportedRate   float64            `json:"supported_rate"`
	UnsupportedRate float64            `json:"unsupported_rate"`
	AgreementRate   float64            `json:"agreement_rate"`
	ModalJudgment   string             `json:"modal_judgment"`
	Evidence        []ArtifactEvidence `json:"evidence"`
	ReviewReports   []ReviewReport     `json:"reviews_detail"`
}

type ReviewerReport struct {
	ReviewerID         string             `json:"reviewer_id"`
	ReviewerRoles      []string           `json:"reviewer_roles"`
	Reviews            int                `json:"reviews"`
	IndependentReviews int                `json:"independent_reviews"`
	AllIndependent     bool               `json:"all_independent"`
	Supported          int                `json:"supported"`
	Partial            int                `json:"partial"`
	Unsupported        int                `json:"unsupported"`
	Evidence           []ArtifactEvidence `json:"evidence"`
}

type ReviewReport struct {
	ReviewID             string             `json:"review_id"`
	VerdictID            string             `json:"verdict_id"`
	ReviewerID           string             `json:"reviewer_id"`
	ReviewerRole         string             `json:"reviewer_role"`
	Independent          bool               `json:"independent"`
	StatedVerdict        string             `json:"stated_verdict"`
	Judgment             string             `json:"judgment"`
	MissingEvidenceNotes string             `json:"missing_evidence_notes,omitempty"`
	Rationale            string             `json:"rationale"`
	Evidence             []ArtifactEvidence `json:"evidence"`
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

type verdictAccumulator struct {
	id       string
	reviews  []ReviewReport
	evidence map[string]ArtifactEvidence
}

type reviewerAccumulator struct {
	id                 string
	roles              map[string]bool
	reviews            int
	independentReviews int
	supported          int
	partial            int
	unsupported        int
	evidence           map[string]ArtifactEvidence
}

func ReadSpec(reader io.Reader) (Spec, error) {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var spec Spec
	if err := decoder.Decode(&spec); err != nil {
		return Spec{}, err
	}
	if spec.Version != SpecVersion {
		return Spec{}, fmt.Errorf("explainability audit spec version must be %s", SpecVersion)
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
	report := Report{
		Version:  ReportVersion,
		Name:     strings.TrimSpace(spec.Name),
		OK:       true,
		Criteria: spec.Criteria,
	}
	verdicts := map[string]*verdictAccumulator{}
	reviewers := map[string]*reviewerAccumulator{}
	evidenceSeen := map[string]ArtifactEvidence{}
	var counterexamples []Counterexample

	for _, review := range sortedReviews(spec.Reviews) {
		reviewReport, reviewCounterexamples := evaluateReview(spec.Criteria, review, rootAbs)
		counterexamples = append(counterexamples, reviewCounterexamples...)
		addVerdictReview(verdicts, reviewReport)
		addReviewerReview(reviewers, reviewReport)
		for _, evidence := range reviewReport.Evidence {
			evidenceSeen[evidence.Path] = evidence
		}
		report.Summary.Reviews++
		switch reviewReport.Judgment {
		case "supported":
			report.Summary.SupportedReviews++
		case "partial":
			report.Summary.PartialReviews++
		case "unsupported":
			report.Summary.UnsupportedReviews++
		}
	}

	report.Verdicts = finalizeVerdicts(verdicts)
	report.Reviewers = finalizeReviewers(reviewers)
	report.Summary = summarize(report.Summary, report.Verdicts, report.Reviewers, evidenceSeen)
	counterexamples = append(counterexamples, criteriaCounterexamples(report)...)
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
	jsonFile, err := os.Create(filepath.Join(outDir, "explainability-audit.json"))
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
	return os.WriteFile(filepath.Join(outDir, "explainability-audit.md"), []byte(RenderMarkdown(report)), 0o644)
}

func RenderMarkdown(report Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Explainability audit\n\n")
	fmt.Fprintf(&b, "Independent reviewers judge whether Patchline evidence trails actually support the stated verdicts before those verdicts are treated as claims, certificates, or adoption evidence.\n\n")
	fmt.Fprintf(&b, "## Summary\n\n")
	fmt.Fprintf(&b, "| Metric | Value |\n| --- | ---: |\n")
	fmt.Fprintf(&b, "| OK | `%t` |\n", report.OK)
	fmt.Fprintf(&b, "| Reviews | %d |\n", report.Summary.Reviews)
	fmt.Fprintf(&b, "| Reviewers | %d |\n", report.Summary.Reviewers)
	fmt.Fprintf(&b, "| Independent reviewers | %d |\n", report.Summary.IndependentReviewers)
	fmt.Fprintf(&b, "| Verdicts | %d |\n", report.Summary.Verdicts)
	fmt.Fprintf(&b, "| Evidence files | %d |\n", report.Summary.EvidenceFiles)
	fmt.Fprintf(&b, "| Supported rate | %.4f |\n", report.Summary.SupportedRate)
	fmt.Fprintf(&b, "| Unsupported rate | %.4f |\n", report.Summary.UnsupportedRate)
	fmt.Fprintf(&b, "| Average agreement rate | %.4f |\n", report.Summary.AverageAgreementRate)
	fmt.Fprintf(&b, "| Minimum agreement rate | %.4f |\n", report.Summary.MinAgreementRate)
	fmt.Fprintf(&b, "| Counterexamples | %d |\n\n", report.Summary.Counterexamples)
	fmt.Fprintf(&b, "Policy: at least `%d` independent reviewers, `%d` verdicts, `%d` reviews per verdict, agreement at least `%.2f`, supported-review rate at least `%.2f`, and unsupported-review rate at most `%.2f`.\n\n",
		report.Criteria.MinReviewers,
		report.Criteria.MinVerdicts,
		report.Criteria.MinReviewsPerVerdict,
		report.Criteria.MinAgreementRate,
		report.Criteria.MinSupportedRate,
		report.Criteria.MaxUnsupportedRate,
	)

	fmt.Fprintf(&b, "## Verdict support\n\n")
	fmt.Fprintf(&b, "| Verdict | Reviews | Reviewers | Supported | Partial | Unsupported | Agreement | Evidence |\n| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |\n")
	for _, verdict := range report.Verdicts {
		fmt.Fprintf(&b, "| `%s` | %d | %d | %d | %d | %d | %.4f | %d |\n",
			escapePipes(verdict.VerdictID),
			verdict.Reviews,
			verdict.Reviewers,
			verdict.Supported,
			verdict.Partial,
			verdict.Unsupported,
			verdict.AgreementRate,
			len(verdict.Evidence),
		)
	}

	fmt.Fprintf(&b, "\n## Real-code evidence trails\n\n")
	fmt.Fprintf(&b, "| Verdict | Evidence paths |\n| --- | ---: |\n")
	for _, verdict := range report.Verdicts {
		fmt.Fprintf(&b, "| `%s` | %d |\n", escapePipes(verdict.VerdictID), len(verdict.Evidence))
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

func validateSpec(spec Spec) error {
	if strings.TrimSpace(spec.Name) == "" {
		return errorsf("explainability audit name is required")
	}
	if spec.Criteria.MinReviewers < 1 {
		return errorsf("criteria.min_reviewers must be at least 1")
	}
	if spec.Criteria.MinVerdicts < 1 {
		return errorsf("criteria.min_verdicts must be at least 1")
	}
	if spec.Criteria.MinReviewsPerVerdict < 1 {
		return errorsf("criteria.min_reviews_per_verdict must be at least 1")
	}
	if !validRate(spec.Criteria.MinAgreementRate) {
		return errorsf("criteria.min_agreement_rate must be between 0 and 1")
	}
	if !validRate(spec.Criteria.MinSupportedRate) {
		return errorsf("criteria.min_supported_rate must be between 0 and 1")
	}
	if !validRate(spec.Criteria.MaxUnsupportedRate) {
		return errorsf("criteria.max_unsupported_rate must be between 0 and 1")
	}
	if len(spec.Reviews) == 0 {
		return errorsf("at least one explainability review is required")
	}
	seen := map[string]bool{}
	for _, review := range spec.Reviews {
		if strings.TrimSpace(review.ReviewID) == "" {
			return errorsf("review_id is required")
		}
		if seen[review.ReviewID] {
			return errorsf("duplicate review_id %q", review.ReviewID)
		}
		seen[review.ReviewID] = true
		if strings.TrimSpace(review.VerdictID) == "" || strings.TrimSpace(review.ReviewerID) == "" || strings.TrimSpace(review.ReviewerRole) == "" {
			return errorsf("review %q must include verdict_id, reviewer_id, and reviewer_role", review.ReviewID)
		}
		if strings.TrimSpace(review.StatedVerdict) == "" {
			return errorsf("review %q must include stated_verdict", review.ReviewID)
		}
		if !validJudgments[normalizeJudgment(review.Judgment)] {
			return errorsf("review %q judgment must be supported, partial, or unsupported", review.ReviewID)
		}
		if len(review.EvidencePaths) == 0 {
			return errorsf("review %q must include evidence_paths", review.ReviewID)
		}
		if strings.TrimSpace(review.Rationale) == "" {
			return errorsf("review %q must include rationale", review.ReviewID)
		}
		for relPath, expected := range review.ExpectedEvidenceHashes {
			if strings.TrimSpace(relPath) == "" {
				return errorsf("review %q expected_evidence_hashes contains an empty path", review.ReviewID)
			}
			if !isSHA256(expected) {
				return errorsf("review %q expected hash for %q must be a 64-character sha256 hex digest", review.ReviewID, relPath)
			}
		}
	}
	return nil
}

func evaluateReview(criteria Criteria, review Review, rootAbs string) (ReviewReport, []Counterexample) {
	report := ReviewReport{
		ReviewID:             strings.TrimSpace(review.ReviewID),
		VerdictID:            strings.TrimSpace(review.VerdictID),
		ReviewerID:           strings.TrimSpace(review.ReviewerID),
		ReviewerRole:         strings.TrimSpace(review.ReviewerRole),
		Independent:          review.Independent,
		StatedVerdict:        strings.TrimSpace(review.StatedVerdict),
		Judgment:             normalizeJudgment(review.Judgment),
		MissingEvidenceNotes: strings.TrimSpace(review.MissingEvidenceNotes),
		Rationale:            strings.TrimSpace(review.Rationale),
	}
	var counterexamples []Counterexample
	if criteria.RequireIndependentReviewers && !report.Independent {
		counterexamples = append(counterexamples, Counterexample{
			ID:      fmt.Sprintf("non-independent-reviewer-%s", safeID(report.ReviewID)),
			Kind:    "non_independent_reviewer",
			Subject: report.ReviewID,
			Message: "explainability review must be marked independent before it can support the stated verdict",
			Witness: []string{report.ReviewerID},
		})
	}
	if report.Judgment != "supported" && report.MissingEvidenceNotes == "" {
		counterexamples = append(counterexamples, Counterexample{
			ID:      fmt.Sprintf("missing-limitation-notes-%s", safeID(report.ReviewID)),
			Kind:    "missing_limitation_notes",
			Subject: report.ReviewID,
			Message: "partial or unsupported judgments must state which evidence is missing or insufficient",
		})
	}
	expectedHashes := normalizedExpectedHashes(review.ExpectedEvidenceHashes)
	seenExpected := map[string]bool{}
	seenPaths := map[string]bool{}
	for _, relPath := range sortedStrings(review.EvidencePaths) {
		clean := filepath.Clean(strings.TrimSpace(relPath))
		if seenPaths[clean] {
			continue
		}
		seenPaths[clean] = true
		evidence, fileCounterexamples := resolveEvidence(rootAbs, relPath, report.ReviewID)
		counterexamples = append(counterexamples, fileCounterexamples...)
		if evidence == nil {
			continue
		}
		if expected, ok := expectedHashes[evidence.Path]; ok {
			seenExpected[evidence.Path] = true
			if !strings.EqualFold(expected, evidence.SHA256) {
				counterexamples = append(counterexamples, Counterexample{
					ID:      fmt.Sprintf("hash-mismatch-%s-%s", safeID(report.ReviewID), safeID(evidence.Path)),
					Kind:    "hash_mismatch",
					Subject: report.ReviewID,
					Message: fmt.Sprintf("evidence file %q hash does not match the expected digest", evidence.Path),
					Witness: []string{evidence.Path, strings.ToLower(expected), evidence.SHA256},
				})
			}
		}
		report.Evidence = append(report.Evidence, *evidence)
	}
	for expectedPath := range expectedHashes {
		if !seenExpected[expectedPath] {
			counterexamples = append(counterexamples, Counterexample{
				ID:      fmt.Sprintf("unused-expected-hash-%s-%s", safeID(report.ReviewID), safeID(expectedPath)),
				Kind:    "unused_expected_hash",
				Subject: report.ReviewID,
				Message: fmt.Sprintf("expected hash for %q is not tied to a readable evidence path", expectedPath),
				Witness: []string{expectedPath},
			})
		}
	}
	if len(report.Evidence) == 0 {
		counterexamples = append(counterexamples, Counterexample{
			ID:      fmt.Sprintf("missing-review-evidence-%s", safeID(report.ReviewID)),
			Kind:    "missing_review_evidence",
			Subject: report.ReviewID,
			Message: "explainability review must preserve at least one readable evidence file",
		})
	}
	return report, counterexamples
}

func addVerdictReview(accs map[string]*verdictAccumulator, review ReviewReport) {
	acc := accs[review.VerdictID]
	if acc == nil {
		acc = &verdictAccumulator{id: review.VerdictID, evidence: map[string]ArtifactEvidence{}}
		accs[review.VerdictID] = acc
	}
	acc.reviews = append(acc.reviews, review)
	for _, evidence := range review.Evidence {
		acc.evidence[evidence.Path] = evidence
	}
}

func addReviewerReview(accs map[string]*reviewerAccumulator, review ReviewReport) {
	acc := accs[review.ReviewerID]
	if acc == nil {
		acc = &reviewerAccumulator{id: review.ReviewerID, roles: map[string]bool{}, evidence: map[string]ArtifactEvidence{}}
		accs[review.ReviewerID] = acc
	}
	acc.roles[review.ReviewerRole] = true
	acc.reviews++
	if review.Independent {
		acc.independentReviews++
	}
	switch review.Judgment {
	case "supported":
		acc.supported++
	case "partial":
		acc.partial++
	case "unsupported":
		acc.unsupported++
	}
	for _, evidence := range review.Evidence {
		acc.evidence[evidence.Path] = evidence
	}
}

func finalizeVerdicts(accs map[string]*verdictAccumulator) []VerdictReport {
	var verdicts []VerdictReport
	for _, acc := range accs {
		report := VerdictReport{
			VerdictID:     acc.id,
			Evidence:      sortedEvidence(acc.evidence),
			ReviewReports: sortedReviewReports(acc.reviews),
		}
		reviewerIDs := map[string]bool{}
		stated := map[string]bool{}
		judgments := map[string]int{}
		for _, review := range report.ReviewReports {
			report.Reviews++
			reviewerIDs[review.ReviewerID] = true
			stated[review.StatedVerdict] = true
			judgments[review.Judgment]++
			switch review.Judgment {
			case "supported":
				report.Supported++
			case "partial":
				report.Partial++
			case "unsupported":
				report.Unsupported++
			}
		}
		report.Reviewers = len(reviewerIDs)
		report.StatedVerdicts = sortedMapKeys(stated)
		report.SupportedRate = rate(report.Supported, report.Reviews)
		report.UnsupportedRate = rate(report.Unsupported, report.Reviews)
		report.ModalJudgment, report.AgreementRate = modalJudgment(judgments, report.Reviews)
		verdicts = append(verdicts, report)
	}
	sort.Slice(verdicts, func(i, j int) bool {
		return verdicts[i].VerdictID < verdicts[j].VerdictID
	})
	return verdicts
}

func finalizeReviewers(accs map[string]*reviewerAccumulator) []ReviewerReport {
	var reviewers []ReviewerReport
	for _, acc := range accs {
		reviewers = append(reviewers, ReviewerReport{
			ReviewerID:         acc.id,
			ReviewerRoles:      sortedMapKeys(acc.roles),
			Reviews:            acc.reviews,
			IndependentReviews: acc.independentReviews,
			AllIndependent:     acc.reviews > 0 && acc.reviews == acc.independentReviews,
			Supported:          acc.supported,
			Partial:            acc.partial,
			Unsupported:        acc.unsupported,
			Evidence:           sortedEvidence(acc.evidence),
		})
	}
	sort.Slice(reviewers, func(i, j int) bool {
		return reviewers[i].ReviewerID < reviewers[j].ReviewerID
	})
	return reviewers
}

func summarize(summary Summary, verdicts []VerdictReport, reviewers []ReviewerReport, evidenceSeen map[string]ArtifactEvidence) Summary {
	summary.Verdicts = len(verdicts)
	summary.Reviewers = len(reviewers)
	summary.EvidenceFiles = len(evidenceSeen)
	summary.SupportedRate = rate(summary.SupportedReviews, summary.Reviews)
	summary.UnsupportedRate = rate(summary.UnsupportedReviews, summary.Reviews)
	minAgreement := math.MaxFloat64
	for _, verdict := range verdicts {
		summary.AverageAgreementRate += verdict.AgreementRate
		if verdict.AgreementRate < minAgreement {
			minAgreement = verdict.AgreementRate
		}
	}
	if len(verdicts) > 0 {
		summary.AverageAgreementRate = round4(summary.AverageAgreementRate / float64(len(verdicts)))
		summary.MinAgreementRate = round4(minAgreement)
	}
	for _, reviewer := range reviewers {
		if reviewer.AllIndependent {
			summary.IndependentReviewers++
		}
	}
	return summary
}

func criteriaCounterexamples(report Report) []Counterexample {
	var counterexamples []Counterexample
	if report.Summary.Verdicts < report.Criteria.MinVerdicts {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "insufficient-verdicts",
			Kind:    "insufficient_verdicts",
			Subject: "audit",
			Message: fmt.Sprintf("audit covers %d verdicts, below required %d", report.Summary.Verdicts, report.Criteria.MinVerdicts),
		})
	}
	reviewerCount := report.Summary.Reviewers
	if report.Criteria.RequireIndependentReviewers {
		reviewerCount = report.Summary.IndependentReviewers
	}
	if reviewerCount < report.Criteria.MinReviewers {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "insufficient-reviewers",
			Kind:    "insufficient_reviewers",
			Subject: "audit",
			Message: fmt.Sprintf("audit covers %d qualifying reviewers, below required %d", reviewerCount, report.Criteria.MinReviewers),
		})
	}
	if report.Summary.SupportedRate < report.Criteria.MinSupportedRate {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "low-supported-rate",
			Kind:    "low_supported_rate",
			Subject: "audit",
			Message: fmt.Sprintf("supported-review rate %.4f is below required %.4f", report.Summary.SupportedRate, report.Criteria.MinSupportedRate),
		})
	}
	if report.Summary.UnsupportedRate > report.Criteria.MaxUnsupportedRate {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "high-unsupported-rate",
			Kind:    "high_unsupported_rate",
			Subject: "audit",
			Message: fmt.Sprintf("unsupported-review rate %.4f exceeds allowed %.4f", report.Summary.UnsupportedRate, report.Criteria.MaxUnsupportedRate),
		})
	}
	for _, verdict := range report.Verdicts {
		if len(verdict.StatedVerdicts) > 1 {
			counterexamples = append(counterexamples, Counterexample{
				ID:      fmt.Sprintf("inconsistent-stated-verdict-%s", safeID(verdict.VerdictID)),
				Kind:    "inconsistent_stated_verdict",
				Subject: verdict.VerdictID,
				Message: "reviews for the same verdict_id must quote the same stated verdict",
				Witness: verdict.StatedVerdicts,
			})
		}
		if verdict.Reviews < report.Criteria.MinReviewsPerVerdict {
			counterexamples = append(counterexamples, Counterexample{
				ID:      fmt.Sprintf("insufficient-verdict-reviews-%s", safeID(verdict.VerdictID)),
				Kind:    "insufficient_verdict_reviews",
				Subject: verdict.VerdictID,
				Message: fmt.Sprintf("verdict has %d reviews, below required %d", verdict.Reviews, report.Criteria.MinReviewsPerVerdict),
			})
		}
		if verdict.AgreementRate < report.Criteria.MinAgreementRate {
			counterexamples = append(counterexamples, Counterexample{
				ID:      fmt.Sprintf("verdict-disagreement-%s", safeID(verdict.VerdictID)),
				Kind:    "verdict_disagreement",
				Subject: verdict.VerdictID,
				Message: fmt.Sprintf("verdict reviewer agreement %.4f is below required %.4f", verdict.AgreementRate, report.Criteria.MinAgreementRate),
				Witness: []string{verdict.ModalJudgment},
			})
		}
	}
	return counterexamples
}

func resolveEvidence(rootAbs, relPath, subject string) (*ArtifactEvidence, []Counterexample) {
	clean := filepath.Clean(strings.TrimSpace(relPath))
	if filepath.IsAbs(clean) || clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return nil, []Counterexample{{
			ID:      fmt.Sprintf("invalid-evidence-path-%s-%s", safeID(subject), safeID(relPath)),
			Kind:    "invalid_evidence_path",
			Subject: subject,
			Message: fmt.Sprintf("evidence path %q must be a relative file below the audit root", relPath),
			Witness: []string{relPath},
		}}
	}
	artifactPath := filepath.Join(rootAbs, clean)
	info, err := os.Lstat(artifactPath)
	if err != nil {
		return nil, []Counterexample{{
			ID:      fmt.Sprintf("missing-evidence-%s-%s", safeID(subject), safeID(clean)),
			Kind:    "missing_evidence",
			Subject: subject,
			Message: fmt.Sprintf("evidence file %q is missing", clean),
			Witness: []string{clean},
		}}
	}
	if !info.Mode().IsRegular() {
		return nil, []Counterexample{{
			ID:      fmt.Sprintf("invalid-evidence-file-%s-%s", safeID(subject), safeID(clean)),
			Kind:    "invalid_evidence_file",
			Subject: subject,
			Message: fmt.Sprintf("evidence file %q must be a regular file under the audit root", clean),
			Witness: []string{clean},
		}}
	}
	bytes, err := os.ReadFile(artifactPath)
	if err != nil {
		return nil, []Counterexample{{
			ID:      fmt.Sprintf("unreadable-evidence-%s-%s", safeID(subject), safeID(clean)),
			Kind:    "unreadable_evidence",
			Subject: subject,
			Message: fmt.Sprintf("evidence file %q could not be read: %v", clean, err),
			Witness: []string{clean},
		}}
	}
	sum := sha256.Sum256(bytes)
	return &ArtifactEvidence{Path: clean, SHA256: hex.EncodeToString(sum[:]), Bytes: info.Size()}, nil
}

func sortedReviews(reviews []Review) []Review {
	out := append([]Review{}, reviews...)
	sort.Slice(out, func(i, j int) bool {
		return out[i].ReviewID < out[j].ReviewID
	})
	return out
}

func sortedReviewReports(reviews []ReviewReport) []ReviewReport {
	out := append([]ReviewReport{}, reviews...)
	sort.Slice(out, func(i, j int) bool {
		return out[i].ReviewID < out[j].ReviewID
	})
	return out
}

func sortedEvidence(evidence map[string]ArtifactEvidence) []ArtifactEvidence {
	var out []ArtifactEvidence
	for _, item := range evidence {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Path < out[j].Path
	})
	return out
}

func sortedMapKeys(values map[string]bool) []string {
	var keys []string
	for value := range values {
		keys = append(keys, value)
	}
	sort.Strings(keys)
	return keys
}

func sortedStrings(values []string) []string {
	out := append([]string{}, values...)
	sort.Strings(out)
	return out
}

func sortCounterexamples(counterexamples []Counterexample) {
	sort.Slice(counterexamples, func(i, j int) bool {
		if counterexamples[i].ID == counterexamples[j].ID {
			return counterexamples[i].Kind < counterexamples[j].Kind
		}
		return counterexamples[i].ID < counterexamples[j].ID
	})
}

func normalizedExpectedHashes(values map[string]string) map[string]string {
	out := map[string]string{}
	for relPath, expected := range values {
		clean := filepath.Clean(strings.TrimSpace(relPath))
		out[clean] = strings.ToLower(strings.TrimSpace(expected))
	}
	return out
}

func modalJudgment(counts map[string]int, total int) (string, float64) {
	if total == 0 {
		return "", 0
	}
	order := []string{"supported", "partial", "unsupported"}
	modal := order[0]
	modalCount := -1
	for _, judgment := range order {
		if counts[judgment] > modalCount {
			modal = judgment
			modalCount = counts[judgment]
		}
	}
	return modal, round4(float64(modalCount) / float64(total))
}

func reportHash(report Report) string {
	report.Hash = ""
	return canonical.Hash(report)
}

func normalizeJudgment(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func rate(numerator, denominator int) float64 {
	if denominator == 0 {
		return 0
	}
	return round4(float64(numerator) / float64(denominator))
}

func round4(value float64) float64 {
	return math.Round(value*10000) / 10000
}

func validRate(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0 && value <= 1
}

func isSHA256(value string) bool {
	if len(strings.TrimSpace(value)) != 64 {
		return false
	}
	_, err := hex.DecodeString(strings.TrimSpace(value))
	return err == nil
}

func safeID(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if ok {
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func escapePipes(value string) string {
	return strings.ReplaceAll(value, "|", "\\|")
}

func errorsf(format string, args ...any) error {
	return fmt.Errorf(format, args...)
}
