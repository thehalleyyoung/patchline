package reviewerfairness

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

const SpecVersion = "patchline.reviewer-fairness-audit/v1"
const ReportVersion = "patchline.reviewer-fairness-audit-report/v1"

type Spec struct {
	Version      string        `json:"version"`
	Name         string        `json:"name"`
	Criteria     Criteria      `json:"criteria"`
	Observations []Observation `json:"observations"`
}

type Criteria struct {
	MinTeams                int     `json:"min_teams"`
	MinEcosystems           int     `json:"min_ecosystems"`
	MinReviewsPerGroup      int     `json:"min_reviews_per_group"`
	MaxBurdenRatio          float64 `json:"max_burden_ratio"`
	MaxFalsePositiveRateGap float64 `json:"max_false_positive_rate_gap"`
	MaxEscalationRateGap    float64 `json:"max_escalation_rate_gap"`
}

type Observation struct {
	ReviewID         string   `json:"review_id"`
	ReviewerID       string   `json:"reviewer_id"`
	Team             string   `json:"team"`
	Ecosystem        string   `json:"ecosystem"`
	ReviewMinutes    float64  `json:"review_minutes"`
	FindingsReported int      `json:"findings_reported"`
	FalsePositives   int      `json:"false_positives"`
	Escalated        bool     `json:"escalated"`
	EvidencePaths    []string `json:"evidence_paths"`
}

type Report struct {
	Version         string           `json:"version"`
	Name            string           `json:"name"`
	OK              bool             `json:"ok"`
	Criteria        Criteria         `json:"criteria"`
	Summary         Summary          `json:"summary"`
	Teams           []GroupReport    `json:"teams"`
	Ecosystems      []GroupReport    `json:"ecosystems"`
	Counterexamples []Counterexample `json:"counterexamples,omitempty"`
	Hash            string           `json:"hash"`
}

type Summary struct {
	Reviews                       int     `json:"reviews"`
	Teams                         int     `json:"teams"`
	Ecosystems                    int     `json:"ecosystems"`
	ReviewMinutes                 float64 `json:"review_minutes"`
	FindingsReported              int     `json:"findings_reported"`
	FalsePositives                int     `json:"false_positives"`
	FalsePositiveRate             float64 `json:"false_positive_rate"`
	Escalations                   int     `json:"escalations"`
	EscalationRate                float64 `json:"escalation_rate"`
	TeamBurdenRatio               float64 `json:"team_burden_ratio"`
	EcosystemBurdenRatio          float64 `json:"ecosystem_burden_ratio"`
	TeamFalsePositiveRateGap      float64 `json:"team_false_positive_rate_gap"`
	EcosystemFalsePositiveRateGap float64 `json:"ecosystem_false_positive_rate_gap"`
	TeamEscalationRateGap         float64 `json:"team_escalation_rate_gap"`
	EcosystemEscalationRateGap    float64 `json:"ecosystem_escalation_rate_gap"`
	Counterexamples               int     `json:"counterexamples"`
}

type GroupReport struct {
	Kind              string             `json:"kind"`
	Name              string             `json:"name"`
	Reviews           int                `json:"reviews"`
	ReviewMinutes     float64            `json:"review_minutes"`
	AverageMinutes    float64            `json:"average_minutes"`
	FindingsReported  int                `json:"findings_reported"`
	FalsePositives    int                `json:"false_positives"`
	FalsePositiveRate float64            `json:"false_positive_rate"`
	Escalations       int                `json:"escalations"`
	EscalationRate    float64            `json:"escalation_rate"`
	Evidence          []ArtifactEvidence `json:"evidence"`
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

type groupAccumulator struct {
	kind      string
	name      string
	reviews   int
	minutes   float64
	findings  int
	falsePos  int
	escalated int
	evidence  map[string]ArtifactEvidence
}

func ReadSpec(reader io.Reader) (Spec, error) {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var spec Spec
	if err := decoder.Decode(&spec); err != nil {
		return Spec{}, err
	}
	if spec.Version != SpecVersion {
		return Spec{}, fmt.Errorf("reviewer fairness audit spec version must be %s", SpecVersion)
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
		Name:     spec.Name,
		OK:       true,
		Criteria: spec.Criteria,
	}
	teamAcc := map[string]*groupAccumulator{}
	ecosystemAcc := map[string]*groupAccumulator{}
	for _, observation := range sortedObservations(spec.Observations) {
		evidence, counterexamples := collectEvidence(rootAbs, observation)
		report.Counterexamples = append(report.Counterexamples, counterexamples...)
		addObservation(teamAcc, "team", observation.Team, observation, evidence)
		addObservation(ecosystemAcc, "ecosystem", observation.Ecosystem, observation, evidence)
		report.Summary.Reviews++
		report.Summary.ReviewMinutes += observation.ReviewMinutes
		report.Summary.FindingsReported += observation.FindingsReported
		report.Summary.FalsePositives += observation.FalsePositives
		if observation.Escalated {
			report.Summary.Escalations++
		}
	}
	report.Teams = finalizeGroups(teamAcc)
	report.Ecosystems = finalizeGroups(ecosystemAcc)
	report.Summary = summarize(report.Summary, report.Teams, report.Ecosystems)
	report.Counterexamples = append(report.Counterexamples, criteriaCounterexamples(report)...)
	sortCounterexamples(report.Counterexamples)
	report.Summary.Counterexamples = len(report.Counterexamples)
	report.OK = len(report.Counterexamples) == 0
	report.Hash = reportHash(report)
	return report, nil
}

func WriteArtifacts(outDir string, report Report) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	jsonFile, err := os.Create(filepath.Join(outDir, "reviewer-fairness-audit.json"))
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
	return os.WriteFile(filepath.Join(outDir, "reviewer-fairness-audit.md"), []byte(RenderMarkdown(report)), 0o644)
}

func RenderMarkdown(report Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Reviewer fairness audit\n\n")
	fmt.Fprintf(&b, "Patchline audits reviewer burden, false-positive burden, and escalation rates across teams and ecosystems before treating socio-technical outcomes as evidence of safety.\n\n")
	fmt.Fprintf(&b, "## Summary\n\n")
	fmt.Fprintf(&b, "| Metric | Value |\n| --- | ---: |\n")
	fmt.Fprintf(&b, "| OK | `%t` |\n", report.OK)
	fmt.Fprintf(&b, "| Reviews | %d |\n", report.Summary.Reviews)
	fmt.Fprintf(&b, "| Teams | %d |\n", report.Summary.Teams)
	fmt.Fprintf(&b, "| Ecosystems | %d |\n", report.Summary.Ecosystems)
	fmt.Fprintf(&b, "| Review minutes | %.2f |\n", report.Summary.ReviewMinutes)
	fmt.Fprintf(&b, "| False-positive rate | %.4f |\n", report.Summary.FalsePositiveRate)
	fmt.Fprintf(&b, "| Escalation rate | %.4f |\n", report.Summary.EscalationRate)
	fmt.Fprintf(&b, "| Team burden ratio | %.4f |\n", report.Summary.TeamBurdenRatio)
	fmt.Fprintf(&b, "| Ecosystem burden ratio | %.4f |\n", report.Summary.EcosystemBurdenRatio)
	fmt.Fprintf(&b, "| Team false-positive gap | %.4f |\n", report.Summary.TeamFalsePositiveRateGap)
	fmt.Fprintf(&b, "| Ecosystem false-positive gap | %.4f |\n", report.Summary.EcosystemFalsePositiveRateGap)
	fmt.Fprintf(&b, "| Team escalation gap | %.4f |\n", report.Summary.TeamEscalationRateGap)
	fmt.Fprintf(&b, "| Ecosystem escalation gap | %.4f |\n", report.Summary.EcosystemEscalationRateGap)
	fmt.Fprintf(&b, "| Counterexamples | %d |\n\n", report.Summary.Counterexamples)
	fmt.Fprintf(&b, "Policy: at least `%d` teams, `%d` ecosystems, `%d` reviews per group, burden ratio at most `%.2f`, false-positive rate gap at most `%.2f`, and escalation-rate gap at most `%.2f`.\n\n", report.Criteria.MinTeams, report.Criteria.MinEcosystems, report.Criteria.MinReviewsPerGroup, report.Criteria.MaxBurdenRatio, report.Criteria.MaxFalsePositiveRateGap, report.Criteria.MaxEscalationRateGap)
	renderGroups(&b, "Team audit", report.Teams)
	renderGroups(&b, "Ecosystem audit", report.Ecosystems)
	fmt.Fprintf(&b, "\n## Real-code evidence\n\n")
	fmt.Fprintf(&b, "| Scope | Evidence files |\n| --- | ---: |\n")
	for _, group := range append(append([]GroupReport{}, report.Teams...), report.Ecosystems...) {
		fmt.Fprintf(&b, "| `%s:%s` | %d |\n", group.Kind, group.Name, len(group.Evidence))
	}
	if len(report.Counterexamples) > 0 {
		fmt.Fprintf(&b, "\n## Counterexamples\n\n")
		fmt.Fprintf(&b, "| ID | Kind | Subject | Message |\n| --- | --- | --- | --- |\n")
		for _, counterexample := range report.Counterexamples {
			fmt.Fprintf(&b, "| `%s` | `%s` | `%s` | %s |\n", counterexample.ID, counterexample.Kind, firstNonEmpty(counterexample.Subject, "-"), counterexample.Message)
		}
	}
	return b.String()
}

func renderGroups(b *strings.Builder, title string, groups []GroupReport) {
	fmt.Fprintf(b, "\n## %s\n\n", title)
	fmt.Fprintf(b, "| Group | Reviews | Avg minutes | Findings | False-positive rate | Escalation rate | Evidence |\n| --- | ---: | ---: | ---: | ---: | ---: | ---: |\n")
	for _, group := range groups {
		fmt.Fprintf(b, "| `%s` | %d | %.2f | %d | %.4f | %.4f | %d |\n", group.Name, group.Reviews, group.AverageMinutes, group.FindingsReported, group.FalsePositiveRate, group.EscalationRate, len(group.Evidence))
	}
}

func validateSpec(spec Spec) error {
	if strings.TrimSpace(spec.Name) == "" {
		return errorsf("reviewer fairness audit name is required")
	}
	if spec.Criteria.MinTeams < 2 {
		return errorsf("criteria.min_teams must be at least 2")
	}
	if spec.Criteria.MinEcosystems < 2 {
		return errorsf("criteria.min_ecosystems must be at least 2")
	}
	if spec.Criteria.MinReviewsPerGroup < 1 {
		return errorsf("criteria.min_reviews_per_group must be at least 1")
	}
	if spec.Criteria.MaxBurdenRatio < 1 || !isFinite(spec.Criteria.MaxBurdenRatio) {
		return errorsf("criteria.max_burden_ratio must be finite and at least 1")
	}
	if !validRateGap(spec.Criteria.MaxFalsePositiveRateGap) {
		return errorsf("criteria.max_false_positive_rate_gap must be between 0 and 1")
	}
	if !validRateGap(spec.Criteria.MaxEscalationRateGap) {
		return errorsf("criteria.max_escalation_rate_gap must be between 0 and 1")
	}
	if len(spec.Observations) == 0 {
		return errorsf("at least one reviewer observation is required")
	}
	seen := map[string]bool{}
	for _, observation := range spec.Observations {
		if strings.TrimSpace(observation.ReviewID) == "" {
			return errorsf("observation review_id is required")
		}
		if seen[observation.ReviewID] {
			return errorsf("duplicate review_id %q", observation.ReviewID)
		}
		seen[observation.ReviewID] = true
		if strings.TrimSpace(observation.ReviewerID) == "" || strings.TrimSpace(observation.Team) == "" || strings.TrimSpace(observation.Ecosystem) == "" {
			return errorsf("observation %q must include reviewer_id, team, and ecosystem", observation.ReviewID)
		}
		if observation.ReviewMinutes <= 0 || !isFinite(observation.ReviewMinutes) {
			return errorsf("observation %q review_minutes must be finite and positive", observation.ReviewID)
		}
		if observation.FindingsReported < 0 {
			return errorsf("observation %q findings_reported must be non-negative", observation.ReviewID)
		}
		if observation.FalsePositives < 0 || observation.FalsePositives > observation.FindingsReported {
			return errorsf("observation %q false_positives must be between 0 and findings_reported", observation.ReviewID)
		}
		if len(observation.EvidencePaths) == 0 {
			return errorsf("observation %q must include evidence_paths", observation.ReviewID)
		}
	}
	return nil
}

func collectEvidence(rootAbs string, observation Observation) ([]ArtifactEvidence, []Counterexample) {
	seen := map[string]bool{}
	var evidence []ArtifactEvidence
	var counterexamples []Counterexample
	for _, relPath := range sortedStrings(observation.EvidencePaths) {
		clean := filepath.Clean(relPath)
		if filepath.IsAbs(clean) || clean == "." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || clean == ".." {
			counterexamples = append(counterexamples, Counterexample{
				ID:      fmt.Sprintf("evidence-path-%s-%s", safeID(observation.ReviewID), safeID(relPath)),
				Kind:    "invalid_evidence_path",
				Subject: observation.ReviewID,
				Message: fmt.Sprintf("evidence path %q must be a relative file below the audit root", relPath),
				Witness: []string{relPath},
			})
			continue
		}
		if seen[clean] {
			continue
		}
		seen[clean] = true
		artifactPath := filepath.Join(rootAbs, clean)
		info, err := os.Lstat(artifactPath)
		if err != nil {
			counterexamples = append(counterexamples, Counterexample{
				ID:      fmt.Sprintf("missing-evidence-%s-%s", safeID(observation.ReviewID), safeID(clean)),
				Kind:    "missing_evidence",
				Subject: observation.ReviewID,
				Message: fmt.Sprintf("evidence file %q is missing", clean),
				Witness: []string{clean},
			})
			continue
		}
		if !info.Mode().IsRegular() {
			counterexamples = append(counterexamples, Counterexample{
				ID:      fmt.Sprintf("invalid-evidence-file-%s-%s", safeID(observation.ReviewID), safeID(clean)),
				Kind:    "invalid_evidence_file",
				Subject: observation.ReviewID,
				Message: fmt.Sprintf("evidence file %q must be a regular file under the audit root", clean),
				Witness: []string{clean},
			})
			continue
		}
		bytes, err := os.ReadFile(artifactPath)
		if err != nil {
			counterexamples = append(counterexamples, Counterexample{
				ID:      fmt.Sprintf("unreadable-evidence-%s-%s", safeID(observation.ReviewID), safeID(clean)),
				Kind:    "unreadable_evidence",
				Subject: observation.ReviewID,
				Message: fmt.Sprintf("evidence file %q could not be read: %v", clean, err),
				Witness: []string{clean},
			})
			continue
		}
		sum := sha256.Sum256(bytes)
		evidence = append(evidence, ArtifactEvidence{
			Path:   clean,
			SHA256: hex.EncodeToString(sum[:]),
			Bytes:  info.Size(),
		})
	}
	return evidence, counterexamples
}

func addObservation(groups map[string]*groupAccumulator, kind, name string, observation Observation, evidence []ArtifactEvidence) {
	acc := groups[name]
	if acc == nil {
		acc = &groupAccumulator{kind: kind, name: name, evidence: map[string]ArtifactEvidence{}}
		groups[name] = acc
	}
	acc.reviews++
	acc.minutes += observation.ReviewMinutes
	acc.findings += observation.FindingsReported
	acc.falsePos += observation.FalsePositives
	if observation.Escalated {
		acc.escalated++
	}
	for _, item := range evidence {
		acc.evidence[item.Path] = item
	}
}

func finalizeGroups(groups map[string]*groupAccumulator) []GroupReport {
	var reports []GroupReport
	for _, acc := range groups {
		reports = append(reports, GroupReport{
			Kind:              acc.kind,
			Name:              acc.name,
			Reviews:           acc.reviews,
			ReviewMinutes:     round4(acc.minutes),
			AverageMinutes:    round4(acc.minutes / float64(acc.reviews)),
			FindingsReported:  acc.findings,
			FalsePositives:    acc.falsePos,
			FalsePositiveRate: rate(acc.falsePos, acc.findings),
			Escalations:       acc.escalated,
			EscalationRate:    rate(acc.escalated, acc.reviews),
			Evidence:          sortedEvidence(acc.evidence),
		})
	}
	sort.Slice(reports, func(i, j int) bool {
		if reports[i].Kind == reports[j].Kind {
			return reports[i].Name < reports[j].Name
		}
		return reports[i].Kind < reports[j].Kind
	})
	return reports
}

func summarize(summary Summary, teams, ecosystems []GroupReport) Summary {
	summary.ReviewMinutes = round4(summary.ReviewMinutes)
	summary.Teams = len(teams)
	summary.Ecosystems = len(ecosystems)
	summary.FalsePositiveRate = rate(summary.FalsePositives, summary.FindingsReported)
	summary.EscalationRate = rate(summary.Escalations, summary.Reviews)
	summary.TeamBurdenRatio = burdenRatio(teams)
	summary.EcosystemBurdenRatio = burdenRatio(ecosystems)
	summary.TeamFalsePositiveRateGap = rateGap(teams, func(group GroupReport) float64 { return group.FalsePositiveRate })
	summary.EcosystemFalsePositiveRateGap = rateGap(ecosystems, func(group GroupReport) float64 { return group.FalsePositiveRate })
	summary.TeamEscalationRateGap = rateGap(teams, func(group GroupReport) float64 { return group.EscalationRate })
	summary.EcosystemEscalationRateGap = rateGap(ecosystems, func(group GroupReport) float64 { return group.EscalationRate })
	return summary
}

func criteriaCounterexamples(report Report) []Counterexample {
	var counterexamples []Counterexample
	criteria := report.Criteria
	if report.Summary.Teams < criteria.MinTeams {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "insufficient-teams",
			Kind:    "insufficient_groups",
			Subject: "team",
			Message: fmt.Sprintf("audit covers %d teams, below required %d", report.Summary.Teams, criteria.MinTeams),
		})
	}
	if report.Summary.Ecosystems < criteria.MinEcosystems {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "insufficient-ecosystems",
			Kind:    "insufficient_groups",
			Subject: "ecosystem",
			Message: fmt.Sprintf("audit covers %d ecosystems, below required %d", report.Summary.Ecosystems, criteria.MinEcosystems),
		})
	}
	counterexamples = append(counterexamples, groupCounterexamples(report.Teams, criteria)...)
	counterexamples = append(counterexamples, groupCounterexamples(report.Ecosystems, criteria)...)
	counterexamples = append(counterexamples, gapCounterexample("team", "burden_ratio", report.Summary.TeamBurdenRatio, criteria.MaxBurdenRatio)...)
	counterexamples = append(counterexamples, gapCounterexample("ecosystem", "burden_ratio", report.Summary.EcosystemBurdenRatio, criteria.MaxBurdenRatio)...)
	counterexamples = append(counterexamples, gapCounterexample("team", "false_positive_gap", report.Summary.TeamFalsePositiveRateGap, criteria.MaxFalsePositiveRateGap)...)
	counterexamples = append(counterexamples, gapCounterexample("ecosystem", "false_positive_gap", report.Summary.EcosystemFalsePositiveRateGap, criteria.MaxFalsePositiveRateGap)...)
	counterexamples = append(counterexamples, gapCounterexample("team", "escalation_gap", report.Summary.TeamEscalationRateGap, criteria.MaxEscalationRateGap)...)
	counterexamples = append(counterexamples, gapCounterexample("ecosystem", "escalation_gap", report.Summary.EcosystemEscalationRateGap, criteria.MaxEscalationRateGap)...)
	return counterexamples
}

func groupCounterexamples(groups []GroupReport, criteria Criteria) []Counterexample {
	var counterexamples []Counterexample
	for _, group := range groups {
		subject := group.Kind + ":" + group.Name
		if group.Reviews < criteria.MinReviewsPerGroup {
			counterexamples = append(counterexamples, Counterexample{
				ID:      fmt.Sprintf("insufficient-reviews-%s-%s", group.Kind, safeID(group.Name)),
				Kind:    "insufficient_reviews",
				Subject: subject,
				Message: fmt.Sprintf("%s has %d reviews, below required %d", subject, group.Reviews, criteria.MinReviewsPerGroup),
			})
		}
		if group.FindingsReported == 0 {
			counterexamples = append(counterexamples, Counterexample{
				ID:      fmt.Sprintf("undefined-fp-rate-%s-%s", group.Kind, safeID(group.Name)),
				Kind:    "undefined_false_positive_rate",
				Subject: subject,
				Message: fmt.Sprintf("%s has zero reported findings, so false-positive burden cannot be audited", subject),
			})
		}
		if len(group.Evidence) == 0 {
			counterexamples = append(counterexamples, Counterexample{
				ID:      fmt.Sprintf("missing-group-evidence-%s-%s", group.Kind, safeID(group.Name)),
				Kind:    "missing_group_evidence",
				Subject: subject,
				Message: fmt.Sprintf("%s has no readable evidence files", subject),
			})
		}
	}
	return counterexamples
}

func gapCounterexample(subject, kind string, observed, limit float64) []Counterexample {
	if observed <= limit {
		return nil
	}
	return []Counterexample{{
		ID:      fmt.Sprintf("%s-%s", subject, strings.ReplaceAll(kind, "_", "-")),
		Kind:    kind,
		Subject: subject,
		Message: fmt.Sprintf("%s %.4f exceeds limit %.4f", strings.ReplaceAll(kind, "_", " "), observed, limit),
	}}
}

func burdenRatio(groups []GroupReport) float64 {
	if len(groups) == 0 {
		return 0
	}
	minValue := math.MaxFloat64
	maxValue := 0.0
	for _, group := range groups {
		if group.AverageMinutes < minValue {
			minValue = group.AverageMinutes
		}
		if group.AverageMinutes > maxValue {
			maxValue = group.AverageMinutes
		}
	}
	if minValue <= 0 || minValue == math.MaxFloat64 {
		return 0
	}
	return round4(maxValue / minValue)
}

func rateGap(groups []GroupReport, selector func(GroupReport) float64) float64 {
	if len(groups) == 0 {
		return 0
	}
	minValue := math.MaxFloat64
	maxValue := 0.0
	for _, group := range groups {
		value := selector(group)
		if value < minValue {
			minValue = value
		}
		if value > maxValue {
			maxValue = value
		}
	}
	if minValue == math.MaxFloat64 {
		return 0
	}
	return round4(maxValue - minValue)
}

func reportHash(report Report) string {
	copyReport := report
	copyReport.Hash = ""
	return canonical.Hash(copyReport)
}

func sortedObservations(observations []Observation) []Observation {
	sorted := append([]Observation(nil), observations...)
	sort.Slice(sorted, func(i, j int) bool {
		left := sorted[i]
		right := sorted[j]
		if left.ReviewID != right.ReviewID {
			return left.ReviewID < right.ReviewID
		}
		if left.Team != right.Team {
			return left.Team < right.Team
		}
		if left.Ecosystem != right.Ecosystem {
			return left.Ecosystem < right.Ecosystem
		}
		return left.ReviewerID < right.ReviewerID
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
	sorted := append([]string(nil), values...)
	sort.Strings(sorted)
	return sorted
}

func rate(numerator int, denominator int) float64 {
	if denominator == 0 {
		return 0
	}
	return round4(float64(numerator) / float64(denominator))
}

func round4(value float64) float64 {
	return math.Round(value*10000) / 10000
}

func isFinite(value float64) bool {
	return !math.IsInf(value, 0) && !math.IsNaN(value)
}

func validRateGap(value float64) bool {
	return isFinite(value) && value >= 0 && value <= 1
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
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

func errorsf(format string, args ...any) error {
	return fmt.Errorf(format, args...)
}
