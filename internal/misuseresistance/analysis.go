package misuseresistance

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

const SpecVersion = "patchline.misuse-resistance/v1"
const ReportVersion = "patchline.misuse-resistance-report/v1"

var requiredSurfaces = []string{"adoption_metrics", "certificates", "scoreboards"}

type Spec struct {
	Version   string     `json:"version"`
	Name      string     `json:"name"`
	AsOfDate  string     `json:"as_of_date"`
	Criteria  Criteria   `json:"criteria"`
	Scenarios []Scenario `json:"scenarios"`
}

type Criteria struct {
	RequiredSurfaces           []string `json:"required_surfaces"`
	MinIndependentReviewers    int      `json:"min_independent_reviewers"`
	MinControlsPerScenario     int      `json:"min_controls_per_scenario"`
	MinControlTypesPerScenario int      `json:"min_control_types_per_scenario"`
	MaxRiskScore               float64  `json:"max_risk_score"`
	ReviewCadenceDays          int      `json:"review_cadence_days"`
	RequireEvidencePaths       bool     `json:"require_evidence_paths"`
	RequireSimulation          bool     `json:"require_simulation"`
	RequirePublicFailureMode   bool     `json:"require_public_failure_mode"`
	RequireControlOwner        bool     `json:"require_control_owner"`
	RequirePassedSimulation    bool     `json:"require_passed_simulation"`
}

type Scenario struct {
	ScenarioID        string       `json:"scenario_id"`
	Surface           string       `json:"surface"`
	Adversary         string       `json:"adversary"`
	AttackGoal        string       `json:"attack_goal"`
	AttackVectors     []string     `json:"attack_vectors"`
	TargetAsset       string       `json:"target_asset"`
	PublicFailureMode string       `json:"public_failure_mode,omitempty"`
	RiskScore         float64      `json:"risk_score"`
	LastReviewed      string       `json:"last_reviewed"`
	ReviewerRoles     []string     `json:"reviewer_roles"`
	Controls          []Control    `json:"controls"`
	Simulations       []Simulation `json:"simulations,omitempty"`
}

type Control struct {
	ControlID     string   `json:"control_id"`
	Type          string   `json:"type"`
	Description   string   `json:"description"`
	Owner         string   `json:"owner,omitempty"`
	EvidencePaths []string `json:"evidence_paths,omitempty"`
}

type Simulation struct {
	SimulationID     string `json:"simulation_id"`
	AttemptedVector  string `json:"attempted_vector"`
	ExpectedOutcome  string `json:"expected_outcome"`
	ObservedOutcome  string `json:"observed_outcome"`
	Passed           bool   `json:"passed"`
	ReproductionPath string `json:"reproduction_path,omitempty"`
}

type Report struct {
	Version         string           `json:"version"`
	Name            string           `json:"name"`
	AsOfDate        string           `json:"as_of_date"`
	OK              bool             `json:"ok"`
	Criteria        Criteria         `json:"criteria"`
	Summary         Summary          `json:"summary"`
	Surfaces        []SurfaceReport  `json:"surfaces"`
	Counterexamples []Counterexample `json:"counterexamples,omitempty"`
	Hash            string           `json:"hash"`
}

type Summary struct {
	Surfaces                   int     `json:"surfaces"`
	Scenarios                  int     `json:"scenarios"`
	Controls                   int     `json:"controls"`
	EvidenceFiles              int     `json:"evidence_files"`
	Simulations                int     `json:"simulations"`
	PassedSimulations          int     `json:"passed_simulations"`
	FailedSimulations          int     `json:"failed_simulations"`
	HighRiskScenarios          int     `json:"high_risk_scenarios"`
	StaleReviews               int     `json:"stale_reviews"`
	MissingEvidenceScenarios   int     `json:"missing_evidence_scenarios"`
	MinIndependentReviewers    int     `json:"min_independent_reviewers"`
	MinControlsPerScenario     int     `json:"min_controls_per_scenario"`
	MinControlTypesPerScenario int     `json:"min_control_types_per_scenario"`
	MaxRiskScore               float64 `json:"max_risk_score"`
	Counterexamples            int     `json:"counterexamples"`
}

type SurfaceReport struct {
	Surface      string             `json:"surface"`
	Scenarios    int                `json:"scenarios"`
	Controls     int                `json:"controls"`
	ControlTypes int                `json:"control_types"`
	Evidence     []ArtifactEvidence `json:"evidence"`
	Items        []ScenarioReport   `json:"items"`
}

type ScenarioReport struct {
	ScenarioID           string             `json:"scenario_id"`
	Surface              string             `json:"surface"`
	Adversary            string             `json:"adversary"`
	AttackGoal           string             `json:"attack_goal"`
	AttackVectors        []string           `json:"attack_vectors"`
	TargetAsset          string             `json:"target_asset"`
	PublicFailureMode    string             `json:"public_failure_mode,omitempty"`
	RiskScore            float64            `json:"risk_score"`
	LastReviewed         string             `json:"last_reviewed"`
	ReviewAgeDays        int                `json:"review_age_days"`
	ReviewerRoles        []string           `json:"reviewer_roles"`
	IndependentReviewers int                `json:"independent_reviewers"`
	ControlTypes         int                `json:"control_types"`
	HighRisk             bool               `json:"high_risk"`
	StaleReview          bool               `json:"stale_review"`
	MissingEvidence      bool               `json:"missing_evidence"`
	Controls             []ControlReport    `json:"controls"`
	Simulations          []SimulationReport `json:"simulations,omitempty"`
	Evidence             []ArtifactEvidence `json:"evidence"`
}

type ControlReport struct {
	ControlID   string             `json:"control_id"`
	Type        string             `json:"type"`
	Description string             `json:"description"`
	Owner       string             `json:"owner,omitempty"`
	Evidence    []ArtifactEvidence `json:"evidence"`
}

type SimulationReport struct {
	SimulationID     string `json:"simulation_id"`
	AttemptedVector  string `json:"attempted_vector"`
	ExpectedOutcome  string `json:"expected_outcome"`
	ObservedOutcome  string `json:"observed_outcome"`
	Passed           bool   `json:"passed"`
	ReproductionPath string `json:"reproduction_path,omitempty"`
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

type surfaceAccumulator struct {
	surface      string
	items        []ScenarioReport
	evidence     map[string]ArtifactEvidence
	controlTypes map[string]bool
}

func ReadSpec(reader io.Reader) (Spec, error) {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var spec Spec
	if err := decoder.Decode(&spec); err != nil {
		return Spec{}, err
	}
	if spec.Version != SpecVersion {
		return Spec{}, fmt.Errorf("misuse-resistance spec version must be %s", SpecVersion)
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
		Name:     strings.TrimSpace(spec.Name),
		AsOfDate: spec.AsOfDate,
		OK:       true,
		Criteria: spec.Criteria,
	}
	accs := map[string]*surfaceAccumulator{}
	evidenceSeen := map[string]ArtifactEvidence{}
	var counterexamples []Counterexample

	for _, scenario := range sortedScenarios(spec.Scenarios) {
		scenario.Surface = normalizeSurface(scenario.Surface)
		scenarioReport, scenarioCounterexamples := evaluateScenario(spec.Criteria, scenario, rootAbs, asOf)
		counterexamples = append(counterexamples, scenarioCounterexamples...)
		acc := accs[scenario.Surface]
		if acc == nil {
			acc = &surfaceAccumulator{surface: scenario.Surface, evidence: map[string]ArtifactEvidence{}, controlTypes: map[string]bool{}}
			accs[scenario.Surface] = acc
		}
		addScenario(acc, scenarioReport)
		for _, evidence := range scenarioReport.Evidence {
			evidenceSeen[evidence.Path] = evidence
		}
		report.Summary.Scenarios++
	}

	report.Surfaces = finalizeSurfaces(accs)
	report.Summary = summarize(report.Summary, report.Surfaces, evidenceSeen)
	counterexamples = append(counterexamples, criteriaCounterexamples(spec.Criteria, report.Surfaces)...)
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
	jsonFile, err := os.Create(filepath.Join(outDir, "misuse-resistance.json"))
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
	return os.WriteFile(filepath.Join(outDir, "misuse-resistance.md"), []byte(RenderMarkdown(report)), 0o644)
}

func RenderMarkdown(report Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Misuse-resistance analysis\n\n")
	fmt.Fprintf(&b, "Patchline treats certificates, scoreboards, and adoption metrics as adversarial surfaces: a claim is not trusted unless the repository records the attack it resists, the independent reviewers who assessed it, the controls with hashed evidence, and a reproduction of the attempted abuse.\n\n")
	fmt.Fprintf(&b, "## Summary\n\n")
	fmt.Fprintf(&b, "| Metric | Value |\n| --- | ---: |\n")
	fmt.Fprintf(&b, "| OK | `%t` |\n", report.OK)
	fmt.Fprintf(&b, "| As of | `%s` |\n", report.AsOfDate)
	fmt.Fprintf(&b, "| Surfaces | %d |\n", report.Summary.Surfaces)
	fmt.Fprintf(&b, "| Scenarios | %d |\n", report.Summary.Scenarios)
	fmt.Fprintf(&b, "| Controls | %d |\n", report.Summary.Controls)
	fmt.Fprintf(&b, "| Evidence files | %d |\n", report.Summary.EvidenceFiles)
	fmt.Fprintf(&b, "| Simulations | %d |\n", report.Summary.Simulations)
	fmt.Fprintf(&b, "| Failed simulations | %d |\n", report.Summary.FailedSimulations)
	fmt.Fprintf(&b, "| High-risk scenarios | %d |\n", report.Summary.HighRiskScenarios)
	fmt.Fprintf(&b, "| Stale reviews | %d |\n", report.Summary.StaleReviews)
	fmt.Fprintf(&b, "| Counterexamples | %d |\n\n", report.Summary.Counterexamples)
	fmt.Fprintf(&b, "Policy: required surfaces `%s`, at least `%d` independent reviewer roles, `%d` controls, `%d` control types, risk score at most `%.2f`, and review cadence `%d` days.\n\n",
		strings.Join(report.Criteria.RequiredSurfaces, ", "),
		report.Criteria.MinIndependentReviewers,
		report.Criteria.MinControlsPerScenario,
		report.Criteria.MinControlTypesPerScenario,
		report.Criteria.MaxRiskScore,
		report.Criteria.ReviewCadenceDays,
	)

	fmt.Fprintf(&b, "## Adversarial surfaces\n\n")
	fmt.Fprintf(&b, "| Surface | Scenarios | Controls | Control types | Evidence |\n| --- | ---: | ---: | ---: | ---: |\n")
	for _, surface := range report.Surfaces {
		fmt.Fprintf(&b, "| `%s` | %d | %d | %d | %d |\n", surface.Surface, surface.Scenarios, surface.Controls, surface.ControlTypes, len(surface.Evidence))
	}

	fmt.Fprintf(&b, "\n## Scenarios\n\n")
	fmt.Fprintf(&b, "| Surface | Scenario | Adversary | Risk | Reviewers | Controls | Control types | Simulations | Evidence |\n")
	fmt.Fprintf(&b, "| --- | --- | --- | ---: | ---: | ---: | ---: | ---: | ---: |\n")
	for _, surface := range report.Surfaces {
		for _, scenario := range surface.Items {
			fmt.Fprintf(&b, "| `%s` | `%s` | `%s` | %.4f | %d | %d | %d | %d | %d |\n",
				scenario.Surface,
				escapePipes(scenario.ScenarioID),
				escapePipes(scenario.Adversary),
				scenario.RiskScore,
				scenario.IndependentReviewers,
				len(scenario.Controls),
				scenario.ControlTypes,
				len(scenario.Simulations),
				len(scenario.Evidence),
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

func evaluateScenario(criteria Criteria, scenario Scenario, rootAbs string, asOf time.Time) (ScenarioReport, []Counterexample) {
	lastReviewed := mustParseTime(scenario.LastReviewed)
	ageDays := int(math.Floor(asOf.Sub(lastReviewed).Hours() / 24))
	report := ScenarioReport{
		ScenarioID:           strings.TrimSpace(scenario.ScenarioID),
		Surface:              normalizeSurface(scenario.Surface),
		Adversary:            strings.TrimSpace(scenario.Adversary),
		AttackGoal:           strings.TrimSpace(scenario.AttackGoal),
		AttackVectors:        sortedNonEmpty(scenario.AttackVectors),
		TargetAsset:          strings.TrimSpace(scenario.TargetAsset),
		PublicFailureMode:    strings.TrimSpace(scenario.PublicFailureMode),
		RiskScore:            round4(scenario.RiskScore),
		LastReviewed:         scenario.LastReviewed,
		ReviewAgeDays:        ageDays,
		ReviewerRoles:        sortedNonEmpty(scenario.ReviewerRoles),
		IndependentReviewers: len(distinctNormalized(scenario.ReviewerRoles)),
	}
	var counterexamples []Counterexample
	if lastReviewed.After(asOf) {
		counterexamples = append(counterexamples, Counterexample{
			ID:      fmt.Sprintf("review-in-future-%s", safeID(scenario.ScenarioID)),
			Kind:    "review_in_future",
			Subject: scenario.ScenarioID,
			Message: "last_reviewed must not be after the misuse-resistance as_of_date",
			Witness: []string{scenario.LastReviewed, asOf.Format(time.RFC3339)},
		})
	}
	if criteria.ReviewCadenceDays > 0 && ageDays > criteria.ReviewCadenceDays {
		report.StaleReview = true
		counterexamples = append(counterexamples, Counterexample{
			ID:      fmt.Sprintf("stale-review-%s", safeID(scenario.ScenarioID)),
			Kind:    "stale_review",
			Subject: scenario.ScenarioID,
			Message: fmt.Sprintf("misuse-resistance review age %d days exceeds cadence %d days", ageDays, criteria.ReviewCadenceDays),
			Witness: []string{scenario.LastReviewed, asOf.Format(time.RFC3339)},
		})
	}
	if report.RiskScore > criteria.MaxRiskScore {
		report.HighRisk = true
		counterexamples = append(counterexamples, Counterexample{
			ID:      fmt.Sprintf("risk-score-exceeded-%s", safeID(scenario.ScenarioID)),
			Kind:    "risk_score_exceeded",
			Subject: scenario.ScenarioID,
			Message: fmt.Sprintf("risk score %.4f exceeds allowed maximum %.4f", report.RiskScore, criteria.MaxRiskScore),
			Witness: []string{fmt.Sprintf("%.4f", report.RiskScore), fmt.Sprintf("%.4f", criteria.MaxRiskScore)},
		})
	}
	if report.IndependentReviewers < criteria.MinIndependentReviewers {
		counterexamples = append(counterexamples, Counterexample{
			ID:      fmt.Sprintf("insufficient-independent-reviewers-%s", safeID(scenario.ScenarioID)),
			Kind:    "insufficient_independent_reviewers",
			Subject: scenario.ScenarioID,
			Message: fmt.Sprintf("scenario has %d independent reviewer roles, below required %d", report.IndependentReviewers, criteria.MinIndependentReviewers),
		})
	}
	if criteria.RequirePublicFailureMode && report.PublicFailureMode == "" {
		counterexamples = append(counterexamples, Counterexample{
			ID:      fmt.Sprintf("missing-public-failure-mode-%s", safeID(scenario.ScenarioID)),
			Kind:    "missing_public_failure_mode",
			Subject: scenario.ScenarioID,
			Message: "misuse scenario must state the public-safe failure mode it prevents",
		})
	}

	controlTypes := map[string]bool{}
	evidenceByPath := map[string]ArtifactEvidence{}
	for _, control := range sortedControls(scenario.Controls) {
		controlReport, controlCounterexamples := evaluateControl(criteria, scenario.ScenarioID, control, rootAbs)
		counterexamples = append(counterexamples, controlCounterexamples...)
		report.Controls = append(report.Controls, controlReport)
		if controlReport.Type != "" {
			controlTypes[normalizeIdentity(controlReport.Type)] = true
		}
		for _, evidence := range controlReport.Evidence {
			evidenceByPath[evidence.Path] = evidence
		}
	}
	report.ControlTypes = len(controlTypes)
	report.Evidence = sortedEvidence(evidenceByPath)
	if len(report.Controls) < criteria.MinControlsPerScenario {
		counterexamples = append(counterexamples, Counterexample{
			ID:      fmt.Sprintf("insufficient-controls-%s", safeID(scenario.ScenarioID)),
			Kind:    "insufficient_controls",
			Subject: scenario.ScenarioID,
			Message: fmt.Sprintf("scenario has %d controls, below required %d", len(report.Controls), criteria.MinControlsPerScenario),
		})
	}
	if report.ControlTypes < criteria.MinControlTypesPerScenario {
		counterexamples = append(counterexamples, Counterexample{
			ID:      fmt.Sprintf("insufficient-control-types-%s", safeID(scenario.ScenarioID)),
			Kind:    "insufficient_control_types",
			Subject: scenario.ScenarioID,
			Message: fmt.Sprintf("scenario has %d distinct control types, below required %d", report.ControlTypes, criteria.MinControlTypesPerScenario),
		})
	}
	if criteria.RequireEvidencePaths && len(report.Evidence) == 0 {
		report.MissingEvidence = true
		counterexamples = append(counterexamples, Counterexample{
			ID:      fmt.Sprintf("missing-scenario-evidence-%s", safeID(scenario.ScenarioID)),
			Kind:    "missing_scenario_evidence",
			Subject: scenario.ScenarioID,
			Message: "misuse scenario must preserve at least one readable evidence file",
		})
	}

	for _, simulation := range sortedSimulations(scenario.Simulations) {
		simulationReport := SimulationReport{
			SimulationID:     strings.TrimSpace(simulation.SimulationID),
			AttemptedVector:  strings.TrimSpace(simulation.AttemptedVector),
			ExpectedOutcome:  strings.TrimSpace(simulation.ExpectedOutcome),
			ObservedOutcome:  strings.TrimSpace(simulation.ObservedOutcome),
			Passed:           simulation.Passed,
			ReproductionPath: strings.TrimSpace(simulation.ReproductionPath),
		}
		report.Simulations = append(report.Simulations, simulationReport)
		if criteria.RequirePassedSimulation && !simulationReport.Passed {
			counterexamples = append(counterexamples, Counterexample{
				ID:      fmt.Sprintf("failed-simulation-%s-%s", safeID(scenario.ScenarioID), safeID(simulation.SimulationID)),
				Kind:    "failed_simulation",
				Subject: scenario.ScenarioID,
				Message: fmt.Sprintf("simulation %q did not demonstrate the expected fail-closed outcome", simulation.SimulationID),
				Witness: []string{simulation.AttemptedVector, simulation.ObservedOutcome},
			})
		}
	}
	if criteria.RequireSimulation && len(report.Simulations) == 0 {
		counterexamples = append(counterexamples, Counterexample{
			ID:      fmt.Sprintf("missing-simulation-%s", safeID(scenario.ScenarioID)),
			Kind:    "missing_simulation",
			Subject: scenario.ScenarioID,
			Message: "misuse scenario must include at least one reproduced adversarial simulation",
		})
	}

	return report, counterexamples
}

func evaluateControl(criteria Criteria, scenarioID string, control Control, rootAbs string) (ControlReport, []Counterexample) {
	report := ControlReport{
		ControlID:   strings.TrimSpace(control.ControlID),
		Type:        normalizeIdentity(control.Type),
		Description: strings.TrimSpace(control.Description),
		Owner:       strings.TrimSpace(control.Owner),
	}
	var counterexamples []Counterexample
	if criteria.RequireControlOwner && report.Owner == "" {
		counterexamples = append(counterexamples, Counterexample{
			ID:      fmt.Sprintf("missing-control-owner-%s-%s", safeID(scenarioID), safeID(control.ControlID)),
			Kind:    "missing_control_owner",
			Subject: scenarioID,
			Message: fmt.Sprintf("control %q must name an accountable owner", control.ControlID),
		})
	}
	seen := map[string]bool{}
	for _, relPath := range sortedStrings(control.EvidencePaths) {
		clean := filepath.Clean(strings.TrimSpace(relPath))
		if seen[clean] {
			continue
		}
		seen[clean] = true
		evidence, fileCounterexamples := resolveFileUnderRoot(rootAbs, relPath, scenarioID, "misuse_evidence")
		counterexamples = append(counterexamples, fileCounterexamples...)
		if evidence != nil {
			report.Evidence = append(report.Evidence, *evidence)
		}
	}
	if criteria.RequireEvidencePaths && len(report.Evidence) == 0 {
		counterexamples = append(counterexamples, Counterexample{
			ID:      fmt.Sprintf("missing-control-evidence-%s-%s", safeID(scenarioID), safeID(control.ControlID)),
			Kind:    "missing_control_evidence",
			Subject: scenarioID,
			Message: fmt.Sprintf("control %q must preserve at least one readable evidence file", control.ControlID),
		})
	}
	return report, counterexamples
}

func addScenario(acc *surfaceAccumulator, scenario ScenarioReport) {
	acc.items = append(acc.items, scenario)
	for _, evidence := range scenario.Evidence {
		acc.evidence[evidence.Path] = evidence
	}
	for _, control := range scenario.Controls {
		acc.controlTypes[normalizeIdentity(control.Type)] = true
	}
}

func finalizeSurfaces(accs map[string]*surfaceAccumulator) []SurfaceReport {
	var surfaces []SurfaceReport
	for _, acc := range accs {
		surface := SurfaceReport{
			Surface:      acc.surface,
			Scenarios:    len(acc.items),
			ControlTypes: len(acc.controlTypes),
			Evidence:     sortedEvidence(acc.evidence),
			Items:        sortedScenarioReports(acc.items),
		}
		for _, scenario := range surface.Items {
			surface.Controls += len(scenario.Controls)
		}
		surfaces = append(surfaces, surface)
	}
	sort.Slice(surfaces, func(i, j int) bool {
		return surfaces[i].Surface < surfaces[j].Surface
	})
	return surfaces
}

func summarize(summary Summary, surfaces []SurfaceReport, evidenceSeen map[string]ArtifactEvidence) Summary {
	summary.Surfaces = len(surfaces)
	summary.EvidenceFiles = len(evidenceSeen)
	maxInt := int(^uint(0) >> 1)
	summary.MinIndependentReviewers = maxInt
	summary.MinControlsPerScenario = maxInt
	summary.MinControlTypesPerScenario = maxInt
	for _, surface := range surfaces {
		for _, scenario := range surface.Items {
			summary.Controls += len(scenario.Controls)
			summary.Simulations += len(scenario.Simulations)
			for _, simulation := range scenario.Simulations {
				if simulation.Passed {
					summary.PassedSimulations++
				} else {
					summary.FailedSimulations++
				}
			}
			if scenario.HighRisk {
				summary.HighRiskScenarios++
			}
			if scenario.StaleReview {
				summary.StaleReviews++
			}
			if scenario.MissingEvidence {
				summary.MissingEvidenceScenarios++
			}
			if scenario.IndependentReviewers < summary.MinIndependentReviewers {
				summary.MinIndependentReviewers = scenario.IndependentReviewers
			}
			if len(scenario.Controls) < summary.MinControlsPerScenario {
				summary.MinControlsPerScenario = len(scenario.Controls)
			}
			if scenario.ControlTypes < summary.MinControlTypesPerScenario {
				summary.MinControlTypesPerScenario = scenario.ControlTypes
			}
			if scenario.RiskScore > summary.MaxRiskScore {
				summary.MaxRiskScore = scenario.RiskScore
			}
		}
	}
	if summary.Scenarios == 0 {
		summary.MinIndependentReviewers = 0
		summary.MinControlsPerScenario = 0
		summary.MinControlTypesPerScenario = 0
	}
	summary.MaxRiskScore = round4(summary.MaxRiskScore)
	return summary
}

func criteriaCounterexamples(criteria Criteria, surfaces []SurfaceReport) []Counterexample {
	var counterexamples []Counterexample
	surfaceByName := map[string]SurfaceReport{}
	for _, surface := range surfaces {
		surfaceByName[surface.Surface] = surface
	}
	for _, required := range criteria.RequiredSurfaces {
		required = normalizeSurface(required)
		if _, ok := surfaceByName[required]; !ok {
			counterexamples = append(counterexamples, Counterexample{
				ID:      fmt.Sprintf("missing-required-surface-%s", safeID(required)),
				Kind:    "missing_required_surface",
				Subject: required,
				Message: "misuse-resistance analysis must cover every required adversarial surface",
			})
		}
	}
	return counterexamples
}

func validateSpec(spec Spec) error {
	if spec.Version != SpecVersion {
		return fmt.Errorf("misuse-resistance spec version must be %s", SpecVersion)
	}
	if strings.TrimSpace(spec.Name) == "" {
		return fmt.Errorf("misuse-resistance analysis name is required")
	}
	if _, err := time.Parse(time.RFC3339, spec.AsOfDate); err != nil {
		return fmt.Errorf("as_of_date must be RFC3339: %v", err)
	}
	if err := validateCriteria(spec.Criteria); err != nil {
		return err
	}
	if len(spec.Scenarios) == 0 {
		return fmt.Errorf("at least one misuse-resistance scenario is required")
	}
	seenScenarios := map[string]bool{}
	for _, scenario := range spec.Scenarios {
		if strings.TrimSpace(scenario.ScenarioID) == "" {
			return fmt.Errorf("scenario_id is required")
		}
		scenarioKey := normalizeIdentity(scenario.ScenarioID)
		if seenScenarios[scenarioKey] {
			return fmt.Errorf("duplicate scenario_id %q", scenario.ScenarioID)
		}
		seenScenarios[scenarioKey] = true
		surface := normalizeSurface(scenario.Surface)
		if !allowedSurface(surface) {
			return fmt.Errorf("scenario %q surface must be one of %s", scenario.ScenarioID, strings.Join(requiredSurfaces, ", "))
		}
		if strings.TrimSpace(scenario.Adversary) == "" || strings.TrimSpace(scenario.AttackGoal) == "" || strings.TrimSpace(scenario.TargetAsset) == "" {
			return fmt.Errorf("scenario %q must include adversary, attack_goal, and target_asset", scenario.ScenarioID)
		}
		if len(sortedNonEmpty(scenario.AttackVectors)) == 0 {
			return fmt.Errorf("scenario %q must include at least one attack vector", scenario.ScenarioID)
		}
		if scenario.RiskScore < 0 || scenario.RiskScore > 1 || !isFinite(scenario.RiskScore) {
			return fmt.Errorf("scenario %q risk_score must be finite and between 0 and 1", scenario.ScenarioID)
		}
		if _, err := time.Parse(time.RFC3339, scenario.LastReviewed); err != nil {
			return fmt.Errorf("scenario %q last_reviewed must be RFC3339: %v", scenario.ScenarioID, err)
		}
		if len(scenario.Controls) == 0 {
			return fmt.Errorf("scenario %q must include at least one control", scenario.ScenarioID)
		}
		seenControls := map[string]bool{}
		for _, control := range scenario.Controls {
			if strings.TrimSpace(control.ControlID) == "" || strings.TrimSpace(control.Type) == "" || strings.TrimSpace(control.Description) == "" {
				return fmt.Errorf("scenario %q controls must include control_id, type, and description", scenario.ScenarioID)
			}
			controlKey := normalizeIdentity(control.ControlID)
			if seenControls[controlKey] {
				return fmt.Errorf("scenario %q has duplicate control_id %q", scenario.ScenarioID, control.ControlID)
			}
			seenControls[controlKey] = true
		}
		seenSimulations := map[string]bool{}
		for _, simulation := range scenario.Simulations {
			if strings.TrimSpace(simulation.SimulationID) == "" || strings.TrimSpace(simulation.AttemptedVector) == "" || strings.TrimSpace(simulation.ExpectedOutcome) == "" || strings.TrimSpace(simulation.ObservedOutcome) == "" {
				return fmt.Errorf("scenario %q simulations must include simulation_id, attempted_vector, expected_outcome, and observed_outcome", scenario.ScenarioID)
			}
			simulationKey := normalizeIdentity(simulation.SimulationID)
			if seenSimulations[simulationKey] {
				return fmt.Errorf("scenario %q has duplicate simulation_id %q", scenario.ScenarioID, simulation.SimulationID)
			}
			seenSimulations[simulationKey] = true
		}
	}
	return nil
}

func validateCriteria(criteria Criteria) error {
	required := map[string]bool{}
	for _, surface := range criteria.RequiredSurfaces {
		normalized := normalizeSurface(surface)
		if !allowedSurface(normalized) {
			return fmt.Errorf("criteria.required_surfaces contains unsupported surface %q", surface)
		}
		required[normalized] = true
	}
	for _, surface := range requiredSurfaces {
		if !required[surface] {
			return fmt.Errorf("criteria.required_surfaces must include %q", surface)
		}
	}
	if criteria.MinIndependentReviewers < 2 {
		return fmt.Errorf("criteria.min_independent_reviewers must be at least 2")
	}
	if criteria.MinControlsPerScenario < 1 {
		return fmt.Errorf("criteria.min_controls_per_scenario must be at least 1")
	}
	if criteria.MinControlTypesPerScenario < 1 {
		return fmt.Errorf("criteria.min_control_types_per_scenario must be at least 1")
	}
	if criteria.MinControlTypesPerScenario > criteria.MinControlsPerScenario {
		return fmt.Errorf("criteria.min_control_types_per_scenario must not exceed min_controls_per_scenario")
	}
	if criteria.MaxRiskScore <= 0 || criteria.MaxRiskScore > 1 || !isFinite(criteria.MaxRiskScore) {
		return fmt.Errorf("criteria.max_risk_score must be finite and in (0,1]")
	}
	if criteria.ReviewCadenceDays < 1 {
		return fmt.Errorf("criteria.review_cadence_days must be at least 1")
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
			Message: fmt.Sprintf("%s path %q must be a relative file below the analysis root", strings.ReplaceAll(kind, "_", " "), relPath),
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
			Message: fmt.Sprintf("%s file %q must be a regular file under the analysis root", strings.ReplaceAll(kind, "_", " "), clean),
			Witness: []string{clean},
		}}
	}
	rootReal, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		return nil, []Counterexample{{
			ID:      fmt.Sprintf("invalid-%s-root-%s", strings.ReplaceAll(kind, "_", "-"), safeID(subject)),
			Kind:    "invalid_evidence_root",
			Subject: subject,
			Message: fmt.Sprintf("analysis root %q could not be resolved without symlinks: %v", rootAbs, err),
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
			Message: fmt.Sprintf("%s file %q resolves outside the analysis root", strings.ReplaceAll(kind, "_", " "), clean),
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

func sortedScenarios(scenarios []Scenario) []Scenario {
	sorted := append([]Scenario(nil), scenarios...)
	sort.Slice(sorted, func(i, j int) bool {
		left := sorted[i]
		right := sorted[j]
		if normalizeSurface(left.Surface) != normalizeSurface(right.Surface) {
			return normalizeSurface(left.Surface) < normalizeSurface(right.Surface)
		}
		return left.ScenarioID < right.ScenarioID
	})
	return sorted
}

func sortedControls(controls []Control) []Control {
	sorted := append([]Control(nil), controls...)
	sort.Slice(sorted, func(i, j int) bool {
		if normalizeIdentity(sorted[i].Type) != normalizeIdentity(sorted[j].Type) {
			return normalizeIdentity(sorted[i].Type) < normalizeIdentity(sorted[j].Type)
		}
		return sorted[i].ControlID < sorted[j].ControlID
	})
	return sorted
}

func sortedSimulations(simulations []Simulation) []Simulation {
	sorted := append([]Simulation(nil), simulations...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].SimulationID < sorted[j].SimulationID
	})
	return sorted
}

func sortedScenarioReports(scenarios []ScenarioReport) []ScenarioReport {
	sorted := append([]ScenarioReport(nil), scenarios...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Surface != sorted[j].Surface {
			return sorted[i].Surface < sorted[j].Surface
		}
		return sorted[i].ScenarioID < sorted[j].ScenarioID
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

func sortedNonEmpty(values []string) []string {
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
	sort.Strings(out)
	return out
}

func distinctNormalized(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		key := normalizeIdentity(value)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func normalizeSurface(value string) string {
	return normalizeIdentity(value)
}

func normalizeIdentity(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "-", "_")
	value = strings.ReplaceAll(value, " ", "_")
	for strings.Contains(value, "__") {
		value = strings.ReplaceAll(value, "__", "_")
	}
	return strings.Trim(value, "_")
}

func allowedSurface(surface string) bool {
	for _, allowed := range requiredSurfaces {
		if surface == allowed {
			return true
		}
	}
	return false
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
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "item"
	}
	return out
}

func escapePipes(value string) string {
	return strings.ReplaceAll(value, "|", "\\|")
}
