package practitionercertification

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

const SpecVersion = "patchline.practitioner-certification/v1"
const ReportVersion = "patchline.practitioner-certification-report/v1"

type Spec struct {
	Version   string     `json:"version"`
	Name      string     `json:"name"`
	Claim     string     `json:"claim,omitempty"`
	Criteria  Criteria   `json:"criteria"`
	Scenarios []Scenario `json:"scenarios"`
	Attempts  []Attempt  `json:"attempts"`
}

type Criteria struct {
	MinScenarios                int     `json:"min_scenarios"`
	MinTotalPoints              int     `json:"min_total_points"`
	PassingScorePercent         float64 `json:"passing_score_pct"`
	MinGateBackedScenarios      int     `json:"min_gate_backed_scenarios"`
	RequireReproducibleCommands bool    `json:"require_reproducible_commands"`
}

type Scenario struct {
	ID                string       `json:"id"`
	Title             string       `json:"title"`
	Role              string       `json:"role"`
	Repo              string       `json:"repo"`
	HazardClass       string       `json:"hazard_class"`
	Prompt            string       `json:"prompt"`
	EvidencePaths     []string     `json:"evidence_paths"`
	Gate              string       `json:"gate"`
	ReproduceCommands []string     `json:"reproduce_commands"`
	ExpectedDecision  string       `json:"expected_decision"`
	Rubric            []RubricItem `json:"rubric"`
}

type RubricItem struct {
	ID               string   `json:"id"`
	Description      string   `json:"description"`
	Points           int      `json:"points"`
	RequiredConcepts []string `json:"required_concepts"`
}

type Attempt struct {
	CandidateID string   `json:"candidate_id"`
	ScenarioID  string   `json:"scenario_id"`
	Decision    string   `json:"decision"`
	Concepts    []string `json:"concepts"`
	Commands    []string `json:"commands"`
}

type Report struct {
	Version         string            `json:"version"`
	Name            string            `json:"name"`
	OK              bool              `json:"ok"`
	Criteria        Criteria          `json:"criteria"`
	Summary         Summary           `json:"summary"`
	Scenarios       []ScenarioReport  `json:"scenarios"`
	Attempts        []AttemptReport   `json:"attempts"`
	Candidates      []CandidateReport `json:"candidates"`
	Counterexamples []Counterexample  `json:"counterexamples,omitempty"`
	Hash            string            `json:"hash"`
}

type Summary struct {
	Scenarios           int     `json:"scenarios"`
	GateBackedScenarios int     `json:"gate_backed_scenarios"`
	Attempts            int     `json:"attempts"`
	Candidates          int     `json:"candidates"`
	TotalPossiblePoints int     `json:"total_possible_points"`
	AverageScorePercent float64 `json:"average_score_pct"`
	PassedCandidates    int     `json:"passed_candidates"`
	Counterexamples     int     `json:"counterexamples"`
}

type ScenarioReport struct {
	ID                string             `json:"id"`
	Title             string             `json:"title"`
	Role              string             `json:"role"`
	Repo              string             `json:"repo"`
	HazardClass       string             `json:"hazard_class"`
	Gate              string             `json:"gate"`
	GateBacked        bool               `json:"gate_backed"`
	ReproduceCommands []string           `json:"reproduce_commands"`
	RubricPoints      int                `json:"rubric_points"`
	Attempts          int                `json:"attempts"`
	Evidence          []ArtifactEvidence `json:"evidence"`
}

type ArtifactEvidence struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

type AttemptReport struct {
	CandidateID           string   `json:"candidate_id"`
	ScenarioID            string   `json:"scenario_id"`
	Decision              string   `json:"decision"`
	ExpectedDecision      string   `json:"expected_decision"`
	DecisionCorrect       bool     `json:"decision_correct"`
	Points                int      `json:"points"`
	PossiblePoints        int      `json:"possible_points"`
	ScorePercent          float64  `json:"score_pct"`
	MatchedRubric         []string `json:"matched_rubric"`
	MissingRubric         []string `json:"missing_rubric,omitempty"`
	ReproducibleCommandOK bool     `json:"reproducible_command_ok"`
	RequiredGateCommand   string   `json:"required_gate_command"`
}

type CandidateReport struct {
	CandidateID    string  `json:"candidate_id"`
	Attempts       int     `json:"attempts"`
	Points         int     `json:"points"`
	PossiblePoints int     `json:"possible_points"`
	ScorePercent   float64 `json:"score_pct"`
	Passed         bool    `json:"passed"`
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
		return Spec{}, fmt.Errorf("practitioner certification spec version must be %s", SpecVersion)
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
	scenarioByID := map[string]Scenario{}
	for _, scenario := range spec.Scenarios {
		scenarioByID[scenario.ID] = scenario
	}
	report := Report{
		Version:  ReportVersion,
		Name:     spec.Name,
		OK:       true,
		Criteria: spec.Criteria,
	}

	attemptsByScenario := map[string]int{}
	for _, attempt := range spec.Attempts {
		attemptsByScenario[attempt.ScenarioID]++
	}
	for _, scenario := range sortedScenarios(spec.Scenarios) {
		evidence, err := collectArtifacts(rootAbs, scenario.EvidencePaths)
		if err != nil {
			return Report{}, err
		}
		gateBacked := gateExists(rootAbs, scenario.Gate)
		sr := ScenarioReport{
			ID:                scenario.ID,
			Title:             scenario.Title,
			Role:              scenario.Role,
			Repo:              scenario.Repo,
			HazardClass:       scenario.HazardClass,
			Gate:              scenario.Gate,
			GateBacked:        gateBacked,
			ReproduceCommands: sortedStrings(scenario.ReproduceCommands),
			RubricPoints:      rubricPoints(scenario.Rubric),
			Attempts:          attemptsByScenario[scenario.ID],
			Evidence:          evidence,
		}
		report.Scenarios = append(report.Scenarios, sr)
		if !gateBacked {
			report.Counterexamples = append(report.Counterexamples, Counterexample{
				ID:      "scenario." + stableID(scenario.ID) + ".gate",
				Kind:    "scenario_unbacked",
				Subject: scenario.ID,
				Message: "scenario gate is not present as a Make target backed by a script",
				Witness: []string{scenario.Gate},
			})
		}
		if spec.Criteria.RequireReproducibleCommands && !containsCommand(scenario.ReproduceCommands, requiredGateCommand(scenario.Gate)) {
			report.Counterexamples = append(report.Counterexamples, Counterexample{
				ID:      "scenario." + stableID(scenario.ID) + ".reproduce_command",
				Kind:    "non_reproducible_scenario",
				Subject: scenario.ID,
				Message: "scenario does not list the gate command required to reproduce grading",
				Witness: []string{requiredGateCommand(scenario.Gate)},
			})
		}
	}

	candidateReports := map[string]*CandidateReport{}
	for _, attempt := range sortedAttempts(spec.Attempts) {
		scenario := scenarioByID[attempt.ScenarioID]
		ar := gradeAttempt(scenario, attempt, spec.Criteria)
		report.Attempts = append(report.Attempts, ar)
		cr := candidateReports[attempt.CandidateID]
		if cr == nil {
			cr = &CandidateReport{CandidateID: attempt.CandidateID}
			candidateReports[attempt.CandidateID] = cr
		}
		cr.Attempts++
		cr.Points += ar.Points
		cr.PossiblePoints += ar.PossiblePoints
		report.Counterexamples = append(report.Counterexamples, attemptCounterexamples(ar)...)
	}

	for _, candidateID := range sortedMapKeys(candidateReports) {
		cr := candidateReports[candidateID]
		cr.ScorePercent = percent(cr.Points, cr.PossiblePoints)
		cr.Passed = cr.ScorePercent >= spec.Criteria.PassingScorePercent
		report.Candidates = append(report.Candidates, *cr)
		if !cr.Passed {
			report.Counterexamples = append(report.Counterexamples, Counterexample{
				ID:      "candidate." + stableID(candidateID) + ".score",
				Kind:    "candidate_failed",
				Subject: candidateID,
				Message: fmt.Sprintf("candidate scored %.2f%% below required %.2f%%", cr.ScorePercent, spec.Criteria.PassingScorePercent),
				Witness: []string{fmt.Sprintf("%d/%d", cr.Points, cr.PossiblePoints)},
			})
		}
	}

	report.Counterexamples = append(report.Counterexamples, coverageCounterexamples(spec, report)...)
	sortCounterexamples(report.Counterexamples)
	report.Summary = summarize(report)
	report.Summary.Counterexamples = len(report.Counterexamples)
	report.OK = len(report.Counterexamples) == 0
	report.Hash = reportHash(report)
	return report, nil
}

func WriteArtifacts(outDir string, report Report) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	file, err := os.Create(filepath.Join(outDir, "practitioner-certification.json"))
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
	return os.WriteFile(filepath.Join(outDir, "practitioner-certification.md"), []byte(RenderMarkdown(report)), 0o644)
}

func RenderMarkdown(report Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Practitioner certification exam\n\n")
	fmt.Fprintf(&b, "Patchline grades hands-on migration-safety scenarios against gate-backed evidence, reproducible commands, and an explicit rubric.\n\n")
	fmt.Fprintf(&b, "## Summary\n\n")
	fmt.Fprintf(&b, "| Metric | Value |\n| --- | ---: |\n")
	fmt.Fprintf(&b, "| OK | `%t` |\n", report.OK)
	fmt.Fprintf(&b, "| Scenarios | %d |\n", report.Summary.Scenarios)
	fmt.Fprintf(&b, "| Gate-backed scenarios | %d |\n", report.Summary.GateBackedScenarios)
	fmt.Fprintf(&b, "| Candidates | %d |\n", report.Summary.Candidates)
	fmt.Fprintf(&b, "| Attempts | %d |\n", report.Summary.Attempts)
	fmt.Fprintf(&b, "| Total possible points | %d |\n", report.Summary.TotalPossiblePoints)
	fmt.Fprintf(&b, "| Average score | %.2f%% |\n", report.Summary.AverageScorePercent)
	fmt.Fprintf(&b, "| Passed candidates | %d |\n", report.Summary.PassedCandidates)
	fmt.Fprintf(&b, "| Counterexamples | %d |\n\n", report.Summary.Counterexamples)

	fmt.Fprintf(&b, "Policy: at least `%d` scenarios, at least `%d` total points, at least `%d` gate-backed scenarios, and a candidate passing score of at least `%.2f%%`.\n\n",
		report.Criteria.MinScenarios,
		report.Criteria.MinTotalPoints,
		report.Criteria.MinGateBackedScenarios,
		report.Criteria.PassingScorePercent,
	)

	fmt.Fprintf(&b, "## Hands-on scenarios\n\n")
	fmt.Fprintf(&b, "| Scenario | Role | Hazard | Gate | Points | Evidence hashes |\n| --- | --- | --- | --- | ---: | --- |\n")
	for _, scenario := range report.Scenarios {
		hashes := make([]string, 0, len(scenario.Evidence))
		for _, evidence := range scenario.Evidence {
			hashes = append(hashes, fmt.Sprintf("%s:%s", evidence.Path, evidence.SHA256[:16]))
		}
		fmt.Fprintf(&b, "| `%s` | `%s` | `%s` | `%s` | %d | %s |\n",
			escapeTable(scenario.ID),
			escapeTable(scenario.Role),
			escapeTable(scenario.HazardClass),
			escapeTable(scenario.Gate),
			scenario.RubricPoints,
			escapeTable(strings.Join(hashes, "; ")),
		)
	}

	fmt.Fprintf(&b, "\n## Candidate scores\n\n")
	fmt.Fprintf(&b, "| Candidate | Attempts | Points | Score | Passed |\n| --- | ---: | ---: | ---: | ---: |\n")
	for _, candidate := range report.Candidates {
		fmt.Fprintf(&b, "| `%s` | %d | %d/%d | %.2f%% | `%t` |\n",
			escapeTable(candidate.CandidateID),
			candidate.Attempts,
			candidate.Points,
			candidate.PossiblePoints,
			candidate.ScorePercent,
			candidate.Passed,
		)
	}

	fmt.Fprintf(&b, "\n## Attempt grading\n\n")
	fmt.Fprintf(&b, "| Candidate | Scenario | Points | Decision | Gate command | Missing rubric |\n| --- | --- | ---: | ---: | ---: | --- |\n")
	for _, attempt := range report.Attempts {
		missing := "-"
		if len(attempt.MissingRubric) > 0 {
			missing = strings.Join(attempt.MissingRubric, ", ")
		}
		fmt.Fprintf(&b, "| `%s` | `%s` | %d/%d | `%t` | `%t` | %s |\n",
			escapeTable(attempt.CandidateID),
			escapeTable(attempt.ScenarioID),
			attempt.Points,
			attempt.PossiblePoints,
			attempt.DecisionCorrect,
			attempt.ReproducibleCommandOK,
			escapeTable(missing),
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
		return fmt.Errorf("practitioner certification name is required")
	}
	if spec.Criteria.MinScenarios <= 0 {
		return fmt.Errorf("criteria.min_scenarios must be positive")
	}
	if spec.Criteria.MinTotalPoints <= 0 {
		return fmt.Errorf("criteria.min_total_points must be positive")
	}
	if spec.Criteria.PassingScorePercent <= 0 || spec.Criteria.PassingScorePercent > 100 {
		return fmt.Errorf("criteria.passing_score_pct must be between 0 and 100")
	}
	if spec.Criteria.MinGateBackedScenarios <= 0 {
		return fmt.Errorf("criteria.min_gate_backed_scenarios must be positive")
	}
	scenarioIDs := map[string]struct{}{}
	for _, scenario := range spec.Scenarios {
		if strings.TrimSpace(scenario.ID) == "" {
			return fmt.Errorf("scenario id is required")
		}
		if _, exists := scenarioIDs[scenario.ID]; exists {
			return fmt.Errorf("duplicate scenario id %q", scenario.ID)
		}
		scenarioIDs[scenario.ID] = struct{}{}
		if strings.TrimSpace(scenario.Title) == "" || strings.TrimSpace(scenario.Role) == "" || strings.TrimSpace(scenario.Repo) == "" {
			return fmt.Errorf("scenario %q requires title, role, and repo", scenario.ID)
		}
		if strings.TrimSpace(scenario.HazardClass) == "" || strings.TrimSpace(scenario.Prompt) == "" {
			return fmt.Errorf("scenario %q requires hazard_class and prompt", scenario.ID)
		}
		if strings.TrimSpace(scenario.Gate) == "" {
			return fmt.Errorf("scenario %q requires gate", scenario.ID)
		}
		if len(scenario.EvidencePaths) == 0 {
			return fmt.Errorf("scenario %q requires evidence paths", scenario.ID)
		}
		for _, path := range scenario.EvidencePaths {
			if err := validateRelativePath(path); err != nil {
				return fmt.Errorf("scenario %q evidence path: %w", scenario.ID, err)
			}
		}
		if len(scenario.Rubric) == 0 {
			return fmt.Errorf("scenario %q requires rubric", scenario.ID)
		}
		rubricIDs := map[string]struct{}{}
		for _, item := range scenario.Rubric {
			if strings.TrimSpace(item.ID) == "" {
				return fmt.Errorf("scenario %q rubric item id is required", scenario.ID)
			}
			if _, exists := rubricIDs[item.ID]; exists {
				return fmt.Errorf("scenario %q contains duplicate rubric item %q", scenario.ID, item.ID)
			}
			rubricIDs[item.ID] = struct{}{}
			if item.Points <= 0 {
				return fmt.Errorf("scenario %q rubric item %q must have positive points", scenario.ID, item.ID)
			}
			if len(item.RequiredConcepts) == 0 {
				return fmt.Errorf("scenario %q rubric item %q requires concepts", scenario.ID, item.ID)
			}
		}
	}
	if len(spec.Attempts) == 0 {
		return fmt.Errorf("attempts are required")
	}
	seenAttempt := map[string]struct{}{}
	for _, attempt := range spec.Attempts {
		if strings.TrimSpace(attempt.CandidateID) == "" || strings.TrimSpace(attempt.ScenarioID) == "" {
			return fmt.Errorf("attempt requires candidate_id and scenario_id")
		}
		if _, exists := scenarioIDs[attempt.ScenarioID]; !exists {
			return fmt.Errorf("attempt references unknown scenario %q", attempt.ScenarioID)
		}
		key := attempt.CandidateID + "\x00" + attempt.ScenarioID
		if _, exists := seenAttempt[key]; exists {
			return fmt.Errorf("duplicate attempt for candidate %q scenario %q", attempt.CandidateID, attempt.ScenarioID)
		}
		seenAttempt[key] = struct{}{}
		if strings.TrimSpace(attempt.Decision) == "" {
			return fmt.Errorf("attempt %s/%s requires decision", attempt.CandidateID, attempt.ScenarioID)
		}
	}
	return nil
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
			return nil, fmt.Errorf("read exam evidence %s: %w", relPath, err)
		}
		if len(data) == 0 {
			return nil, fmt.Errorf("exam evidence %s is empty", relPath)
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

func gradeAttempt(scenario Scenario, attempt Attempt, criteria Criteria) AttemptReport {
	concepts := normalizedSet(attempt.Concepts)
	ar := AttemptReport{
		CandidateID:           attempt.CandidateID,
		ScenarioID:            attempt.ScenarioID,
		Decision:              attempt.Decision,
		ExpectedDecision:      scenario.ExpectedDecision,
		DecisionCorrect:       normalizeConcept(attempt.Decision) == normalizeConcept(scenario.ExpectedDecision),
		PossiblePoints:        rubricPoints(scenario.Rubric),
		RequiredGateCommand:   requiredGateCommand(scenario.Gate),
		ReproducibleCommandOK: !criteria.RequireReproducibleCommands || containsCommand(attempt.Commands, requiredGateCommand(scenario.Gate)),
	}
	for _, item := range sortedRubric(scenario.Rubric) {
		if conceptsContainAll(concepts, item.RequiredConcepts) {
			ar.Points += item.Points
			ar.MatchedRubric = append(ar.MatchedRubric, item.ID)
		} else {
			ar.MissingRubric = append(ar.MissingRubric, item.ID)
		}
	}
	ar.ScorePercent = percent(ar.Points, ar.PossiblePoints)
	return ar
}

func attemptCounterexamples(attempt AttemptReport) []Counterexample {
	var counterexamples []Counterexample
	if !attempt.DecisionCorrect {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "attempt." + stableID(attempt.CandidateID, attempt.ScenarioID) + ".decision",
			Kind:    "decision_mismatch",
			Subject: attempt.ScenarioID,
			Message: "candidate decision does not match the scenario's expected safety decision",
			Witness: []string{attempt.Decision, attempt.ExpectedDecision},
		})
	}
	if !attempt.ReproducibleCommandOK {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "attempt." + stableID(attempt.CandidateID, attempt.ScenarioID) + ".command",
			Kind:    "missing_reproducible_command",
			Subject: attempt.ScenarioID,
			Message: "candidate did not include the gate command required for reproducible grading",
			Witness: []string{attempt.RequiredGateCommand},
		})
	}
	if len(attempt.MissingRubric) > 0 {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "attempt." + stableID(attempt.CandidateID, attempt.ScenarioID) + ".rubric",
			Kind:    "rubric_miss",
			Subject: attempt.ScenarioID,
			Message: "candidate response missed one or more required rubric items",
			Witness: attempt.MissingRubric,
		})
	}
	return counterexamples
}

func coverageCounterexamples(spec Spec, report Report) []Counterexample {
	var counterexamples []Counterexample
	if len(report.Scenarios) < spec.Criteria.MinScenarios {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "criteria.min_scenarios",
			Kind:    "insufficient_scenarios",
			Message: fmt.Sprintf("scenarios %d below required %d", len(report.Scenarios), spec.Criteria.MinScenarios),
		})
	}
	totalPoints := 0
	gateBacked := 0
	for _, scenario := range report.Scenarios {
		totalPoints += scenario.RubricPoints
		if scenario.GateBacked {
			gateBacked++
		}
		if scenario.Attempts == 0 {
			counterexamples = append(counterexamples, Counterexample{
				ID:      "scenario." + stableID(scenario.ID) + ".missing_attempt",
				Kind:    "missing_attempt",
				Subject: scenario.ID,
				Message: "scenario has no candidate attempt",
			})
		}
	}
	if totalPoints < spec.Criteria.MinTotalPoints {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "criteria.min_total_points",
			Kind:    "insufficient_points",
			Message: fmt.Sprintf("total rubric points %d below required %d", totalPoints, spec.Criteria.MinTotalPoints),
		})
	}
	if gateBacked < spec.Criteria.MinGateBackedScenarios {
		counterexamples = append(counterexamples, Counterexample{
			ID:      "criteria.min_gate_backed_scenarios",
			Kind:    "insufficient_gate_backing",
			Message: fmt.Sprintf("gate-backed scenarios %d below required %d", gateBacked, spec.Criteria.MinGateBackedScenarios),
		})
	}
	return counterexamples
}

func summarize(report Report) Summary {
	summary := Summary{
		Scenarios:           len(report.Scenarios),
		Attempts:            len(report.Attempts),
		Candidates:          len(report.Candidates),
		TotalPossiblePoints: 0,
	}
	for _, scenario := range report.Scenarios {
		summary.TotalPossiblePoints += scenario.RubricPoints
		if scenario.GateBacked {
			summary.GateBackedScenarios++
		}
	}
	for _, candidate := range report.Candidates {
		summary.AverageScorePercent += candidate.ScorePercent
		if candidate.Passed {
			summary.PassedCandidates++
		}
	}
	if len(report.Candidates) > 0 {
		summary.AverageScorePercent = round2(summary.AverageScorePercent / float64(len(report.Candidates)))
	}
	return summary
}

func reportHash(report Report) string {
	copy := report
	copy.Hash = ""
	return canonical.Hash(copy)
}

func rubricPoints(rubric []RubricItem) int {
	total := 0
	for _, item := range rubric {
		total += item.Points
	}
	return total
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

func conceptsContainAll(have map[string]struct{}, want []string) bool {
	for _, concept := range want {
		if _, ok := have[normalizeConcept(concept)]; !ok {
			return false
		}
	}
	return true
}

func normalizedSet(values []string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, value := range values {
		norm := normalizeConcept(value)
		if norm != "" {
			out[norm] = struct{}{}
		}
	}
	return out
}

func normalizeConcept(value string) string {
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

func percent(points, possible int) float64 {
	if possible <= 0 {
		return 0
	}
	return round2(float64(points) / float64(possible) * 100)
}

func round2(value float64) float64 {
	if value >= 0 {
		return float64(int(value*100+0.5)) / 100
	}
	return float64(int(value*100-0.5)) / 100
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

func sortedScenarios(scenarios []Scenario) []Scenario {
	out := append([]Scenario{}, scenarios...)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func sortedRubric(rubric []RubricItem) []RubricItem {
	out := append([]RubricItem{}, rubric...)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func sortedAttempts(attempts []Attempt) []Attempt {
	out := append([]Attempt{}, attempts...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].CandidateID != out[j].CandidateID {
			return out[i].CandidateID < out[j].CandidateID
		}
		return out[i].ScenarioID < out[j].ScenarioID
	})
	return out
}

func sortedStrings(values []string) []string {
	out := append([]string{}, values...)
	sort.Strings(out)
	return out
}

func sortedMapKeys(values map[string]*CandidateReport) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortCounterexamples(counterexamples []Counterexample) {
	sort.Slice(counterexamples, func(i, j int) bool { return counterexamples[i].ID < counterexamples[j].ID })
}

func stableID(parts ...string) string {
	joined := strings.Join(parts, "\x00")
	sum := sha256.Sum256([]byte(joined))
	return hex.EncodeToString(sum[:])[:12]
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
