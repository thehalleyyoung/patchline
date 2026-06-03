package education

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

	"github.com/thehalleyyoung/patchline/internal/canonical"
)

const LongitudinalStudySpecVersion = "patchline.longitudinal-education-study/v1"
const LongitudinalStudyReportVersion = "patchline.longitudinal-education-study-report/v1"

type LongitudinalStudySpec struct {
	Version      string                    `json:"version"`
	Name         string                    `json:"name"`
	Claim        string                    `json:"claim,omitempty"`
	Criteria     LongitudinalCriteria      `json:"criteria"`
	Protocol     LongitudinalProtocol      `json:"protocol"`
	Hazards      []LongitudinalHazard      `json:"hazards"`
	Cohorts      []LongitudinalCohort      `json:"cohorts"`
	Observations []LongitudinalObservation `json:"observations"`
}

type LongitudinalCriteria struct {
	MinCohorts                         int     `json:"min_cohorts"`
	MinRealHazards                     int     `json:"min_real_hazards"`
	MinHeldOutHazards                  int     `json:"min_held_out_hazards"`
	MinFollowupMonths                  int     `json:"min_followup_months"`
	MinObservationsPerCohortTimepoint  int     `json:"min_observations_per_cohort_timepoint"`
	MinRetentionLiftPoints             float64 `json:"min_retention_lift_points"`
	RequireControlCohort               bool    `json:"require_control_cohort"`
	RequireTrainedCohort               bool    `json:"require_trained_cohort"`
	RequireBlindReview                 bool    `json:"require_blind_review"`
	RequireGateBackedHazards           bool    `json:"require_gate_backed_hazards"`
	RequireReproducibleCommands        bool    `json:"require_reproducible_commands"`
	RequireEvidenceCitations           bool    `json:"require_evidence_citations"`
	RequireGateCommandUseForDetections bool    `json:"require_gate_command_use_for_detections"`
	RequireBaseline                    bool    `json:"require_baseline"`
}

type LongitudinalProtocol struct {
	RandomizationUnit string   `json:"randomization_unit"`
	OutcomeDefinition string   `json:"outcome_definition"`
	BlindReview       bool     `json:"blind_review"`
	FollowupMonths    []int    `json:"followup_months"`
	TrainingArtifacts []string `json:"training_artifacts"`
}

type LongitudinalHazard struct {
	ID                string   `json:"id"`
	Title             string   `json:"title"`
	Repo              string   `json:"repo"`
	HazardClass       string   `json:"hazard_class"`
	RealHazard        bool     `json:"real_hazard"`
	HeldOut           bool     `json:"held_out"`
	Gate              string   `json:"gate"`
	ReproduceCommands []string `json:"reproduce_commands"`
	ExpectedDecision  string   `json:"expected_decision"`
	EvidencePaths     []string `json:"evidence_paths"`
}

type LongitudinalCohort struct {
	ID           string   `json:"id"`
	Kind         string   `json:"kind"`
	Description  string   `json:"description"`
	Participants []string `json:"participants"`
}

type LongitudinalObservation struct {
	CohortID          string   `json:"cohort_id"`
	ReviewerID        string   `json:"reviewer_id"`
	TimepointMonth    int      `json:"timepoint_month"`
	HazardID          string   `json:"hazard_id"`
	Detected          bool     `json:"detected"`
	Decision          string   `json:"decision"`
	EvidenceCitations []string `json:"evidence_citations"`
	Commands          []string `json:"commands"`
}

type LongitudinalStudyReport struct {
	Version          string                       `json:"version"`
	Name             string                       `json:"name"`
	OK               bool                         `json:"ok"`
	Criteria         LongitudinalCriteria         `json:"criteria"`
	Protocol         LongitudinalProtocol         `json:"protocol"`
	Summary          LongitudinalSummary          `json:"summary"`
	TrainingEvidence []ArtifactEvidence           `json:"training_evidence,omitempty"`
	Hazards          []LongitudinalHazardReport   `json:"hazards"`
	Cohorts          []LongitudinalCohortReport   `json:"cohorts"`
	Counterexamples  []LongitudinalCounterexample `json:"counterexamples,omitempty"`
	Hash             string                       `json:"hash"`
}

type LongitudinalSummary struct {
	Cohorts                      int     `json:"cohorts"`
	TrainedCohorts               int     `json:"trained_cohorts"`
	ControlCohorts               int     `json:"control_cohorts"`
	Participants                 int     `json:"participants"`
	RealHazards                  int     `json:"real_hazards"`
	HeldOutHazards               int     `json:"held_out_hazards"`
	GateBackedHazards            int     `json:"gate_backed_hazards"`
	TrainingArtifacts            int     `json:"training_artifacts"`
	EvidenceArtifacts            int     `json:"evidence_artifacts"`
	Observations                 int     `json:"observations"`
	BaselineMonth                int     `json:"baseline_month"`
	DelayedFollowupMonth         int     `json:"delayed_followup_month"`
	TrainedFollowupDetectionRate float64 `json:"trained_followup_detection_rate"`
	ControlFollowupDetectionRate float64 `json:"control_followup_detection_rate"`
	RetentionLiftPoints          float64 `json:"retention_lift_points"`
	Counterexamples              int     `json:"counterexamples"`
}

type LongitudinalHazardReport struct {
	ID                    string             `json:"id"`
	Title                 string             `json:"title"`
	Repo                  string             `json:"repo"`
	HazardClass           string             `json:"hazard_class"`
	RealHazard            bool               `json:"real_hazard"`
	HeldOut               bool               `json:"held_out"`
	Gate                  string             `json:"gate"`
	GateBacked            bool               `json:"gate_backed"`
	ReproducibleCommandOK bool               `json:"reproducible_command_ok"`
	ExpectedDecision      string             `json:"expected_decision"`
	Evidence              []ArtifactEvidence `json:"evidence"`
}

type LongitudinalCohortReport struct {
	ID                    string                  `json:"id"`
	Kind                  string                  `json:"kind"`
	Participants          int                     `json:"participants"`
	BaselineObservations  int                     `json:"baseline_observations"`
	BaselineDetections    int                     `json:"baseline_detections"`
	BaselineDetectionRate float64                 `json:"baseline_detection_rate"`
	FollowupObservations  int                     `json:"followup_observations"`
	FollowupDetections    int                     `json:"followup_detections"`
	FollowupDetectionRate float64                 `json:"followup_detection_rate"`
	Timepoints            []LongitudinalRatePoint `json:"timepoints"`
}

type LongitudinalRatePoint struct {
	Month         int     `json:"month"`
	Observations  int     `json:"observations"`
	Detections    int     `json:"detections"`
	DetectionRate float64 `json:"detection_rate"`
}

type LongitudinalCounterexample struct {
	ID      string   `json:"id"`
	Kind    string   `json:"kind"`
	Subject string   `json:"subject,omitempty"`
	Message string   `json:"message"`
	Witness []string `json:"witness,omitempty"`
}

func ReadLongitudinalStudySpec(reader io.Reader) (LongitudinalStudySpec, error) {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var spec LongitudinalStudySpec
	if err := decoder.Decode(&spec); err != nil {
		return LongitudinalStudySpec{}, err
	}
	if spec.Version != LongitudinalStudySpecVersion {
		return LongitudinalStudySpec{}, fmt.Errorf("longitudinal education study spec version must be %s", LongitudinalStudySpecVersion)
	}
	return spec, nil
}

func BuildLongitudinalStudyReport(spec LongitudinalStudySpec, root string) (LongitudinalStudyReport, error) {
	if err := validateLongitudinalStudySpec(spec); err != nil {
		return LongitudinalStudyReport{}, err
	}
	if root == "" {
		root = "."
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return LongitudinalStudyReport{}, err
	}
	hazardByID := map[string]LongitudinalHazard{}
	for _, hazard := range spec.Hazards {
		hazardByID[hazard.ID] = hazard
	}
	cohortByID := map[string]LongitudinalCohort{}
	for _, cohort := range spec.Cohorts {
		cohortByID[cohort.ID] = cohort
	}

	report := LongitudinalStudyReport{
		Version:  LongitudinalStudyReportVersion,
		Name:     spec.Name,
		OK:       true,
		Criteria: spec.Criteria,
		Protocol: normalizeLongitudinalProtocol(spec.Protocol),
	}
	report.Summary.BaselineMonth = 0
	report.Summary.DelayedFollowupMonth = -1

	if len(spec.Protocol.TrainingArtifacts) > 0 {
		trainingEvidence, counterexamples := collectLongitudinalEvidence(rootAbs, spec.Protocol.TrainingArtifacts, "training")
		report.TrainingEvidence = trainingEvidence
		report.Summary.TrainingArtifacts = len(trainingEvidence)
		report.Summary.EvidenceArtifacts += len(trainingEvidence)
		report.Counterexamples = append(report.Counterexamples, counterexamples...)
	}

	for _, hazard := range sortedLongitudinalHazards(spec.Hazards) {
		evidence, counterexamples := collectLongitudinalEvidence(rootAbs, hazard.EvidencePaths, hazard.ID)
		gateBacked := gateExists(rootAbs, hazard.Gate)
		reproducible := containsCommand(hazard.ReproduceCommands, requiredGateCommand(hazard.Gate))
		hr := LongitudinalHazardReport{
			ID:                    hazard.ID,
			Title:                 hazard.Title,
			Repo:                  hazard.Repo,
			HazardClass:           normalizeToken(hazard.HazardClass),
			RealHazard:            hazard.RealHazard,
			HeldOut:               hazard.HeldOut,
			Gate:                  hazard.Gate,
			GateBacked:            gateBacked,
			ReproducibleCommandOK: reproducible,
			ExpectedDecision:      strings.TrimSpace(hazard.ExpectedDecision),
			Evidence:              evidence,
		}
		report.Hazards = append(report.Hazards, hr)
		report.Counterexamples = append(report.Counterexamples, counterexamples...)
		if hazard.RealHazard {
			report.Summary.RealHazards++
		}
		if hazard.RealHazard && hazard.HeldOut {
			report.Summary.HeldOutHazards++
		}
		if gateBacked {
			report.Summary.GateBackedHazards++
		}
		report.Summary.EvidenceArtifacts += len(evidence)
		if spec.Criteria.RequireGateBackedHazards && !gateBacked {
			report.Counterexamples = append(report.Counterexamples, LongitudinalCounterexample{
				ID:      "hazard." + stableID(hazard.ID) + ".gate",
				Kind:    "hazard_unbacked",
				Subject: hazard.ID,
				Message: "longitudinal hazard is not backed by a Make target and script",
				Witness: []string{hazard.Gate},
			})
		}
		if spec.Criteria.RequireReproducibleCommands && !reproducible {
			report.Counterexamples = append(report.Counterexamples, LongitudinalCounterexample{
				ID:      "hazard." + stableID(hazard.ID) + ".reproduce_command",
				Kind:    "non_reproducible_hazard",
				Subject: hazard.ID,
				Message: "hazard does not list the gate command required to reproduce the evidence",
				Witness: []string{requiredGateCommand(hazard.Gate)},
			})
		}
	}

	timepoints := longitudinalTimepoints(spec)
	if spec.Criteria.RequireBaseline && !containsInt(timepoints, 0) {
		report.Counterexamples = append(report.Counterexamples, LongitudinalCounterexample{
			ID:      "protocol.baseline",
			Kind:    "missing_baseline",
			Message: "longitudinal study requires a baseline timepoint at month 0",
		})
	}
	for _, month := range timepoints {
		if month >= spec.Criteria.MinFollowupMonths && month > report.Summary.DelayedFollowupMonth {
			report.Summary.DelayedFollowupMonth = month
		}
	}
	if report.Summary.DelayedFollowupMonth < spec.Criteria.MinFollowupMonths {
		report.Counterexamples = append(report.Counterexamples, LongitudinalCounterexample{
			ID:      "protocol.delayed_followup",
			Kind:    "missing_delayed_followup",
			Message: fmt.Sprintf("no follow-up month is at least %d", spec.Criteria.MinFollowupMonths),
		})
	}

	realObservationsByCohortMonth := map[string]map[int][]LongitudinalObservation{}
	for _, observation := range sortedLongitudinalObservations(spec.Observations) {
		report.Summary.Observations++
		cohort, cohortOK := cohortByID[observation.CohortID]
		hazard, hazardOK := hazardByID[observation.HazardID]
		if !cohortOK {
			report.Counterexamples = append(report.Counterexamples, LongitudinalCounterexample{
				ID:      "observation." + stableID(observation.CohortID, observation.ReviewerID, observation.HazardID) + ".cohort",
				Kind:    "unknown_cohort",
				Subject: observation.CohortID,
				Message: "observation references a cohort not declared in the study",
			})
			continue
		}
		if !hazardOK {
			report.Counterexamples = append(report.Counterexamples, LongitudinalCounterexample{
				ID:      "observation." + stableID(observation.CohortID, observation.ReviewerID, observation.HazardID) + ".hazard",
				Kind:    "unknown_hazard",
				Subject: observation.HazardID,
				Message: "observation references a hazard not declared in the study",
			})
			continue
		}
		if hazard.RealHazard {
			if realObservationsByCohortMonth[cohort.ID] == nil {
				realObservationsByCohortMonth[cohort.ID] = map[int][]LongitudinalObservation{}
			}
			realObservationsByCohortMonth[cohort.ID][observation.TimepointMonth] = append(realObservationsByCohortMonth[cohort.ID][observation.TimepointMonth], observation)
		}
		report.Counterexamples = append(report.Counterexamples, longitudinalObservationCounterexamples(spec.Criteria, hazard, observation)...)
	}

	for _, cohort := range sortedLongitudinalCohorts(spec.Cohorts) {
		kind := normalizeToken(cohort.Kind)
		report.Summary.Participants += len(uniqueSorted(cohort.Participants))
		if kind == "trained" {
			report.Summary.TrainedCohorts++
		}
		if kind == "control" {
			report.Summary.ControlCohorts++
		}
		cr := LongitudinalCohortReport{
			ID:           cohort.ID,
			Kind:         kind,
			Participants: len(uniqueSorted(cohort.Participants)),
		}
		for _, month := range timepoints {
			observations := realObservationsByCohortMonth[cohort.ID][month]
			detections := countQualifiedDetections(spec.Criteria, hazardByID, observations)
			point := LongitudinalRatePoint{
				Month:         month,
				Observations:  len(observations),
				Detections:    detections,
				DetectionRate: ratePercent(detections, len(observations)),
			}
			cr.Timepoints = append(cr.Timepoints, point)
			if month == 0 {
				cr.BaselineObservations = point.Observations
				cr.BaselineDetections = point.Detections
				cr.BaselineDetectionRate = point.DetectionRate
			}
			if month == report.Summary.DelayedFollowupMonth {
				cr.FollowupObservations = point.Observations
				cr.FollowupDetections = point.Detections
				cr.FollowupDetectionRate = point.DetectionRate
			}
		}
		report.Cohorts = append(report.Cohorts, cr)
	}

	report.Summary.Cohorts = len(report.Cohorts)
	report.Counterexamples = append(report.Counterexamples, longitudinalCoverageCounterexamples(spec, report, realObservationsByCohortMonth)...)
	if report.Summary.DelayedFollowupMonth >= spec.Criteria.MinFollowupMonths {
		trainedDetections, trainedObservations := aggregateLongitudinalFollowup(spec.Criteria, hazardByID, cohortByID, realObservationsByCohortMonth, "trained", report.Summary.DelayedFollowupMonth)
		controlDetections, controlObservations := aggregateLongitudinalFollowup(spec.Criteria, hazardByID, cohortByID, realObservationsByCohortMonth, "control", report.Summary.DelayedFollowupMonth)
		report.Summary.TrainedFollowupDetectionRate = ratePercent(trainedDetections, trainedObservations)
		report.Summary.ControlFollowupDetectionRate = ratePercent(controlDetections, controlObservations)
		report.Summary.RetentionLiftPoints = round2Float(report.Summary.TrainedFollowupDetectionRate - report.Summary.ControlFollowupDetectionRate)
		if report.Summary.RetentionLiftPoints < spec.Criteria.MinRetentionLiftPoints {
			report.Counterexamples = append(report.Counterexamples, LongitudinalCounterexample{
				ID:      "summary.retention_lift",
				Kind:    "insufficient_retention_lift",
				Message: fmt.Sprintf("trained follow-up lift %.2f points is below required %.2f", report.Summary.RetentionLiftPoints, spec.Criteria.MinRetentionLiftPoints),
				Witness: []string{
					fmt.Sprintf("trained=%.2f", report.Summary.TrainedFollowupDetectionRate),
					fmt.Sprintf("control=%.2f", report.Summary.ControlFollowupDetectionRate),
				},
			})
		}
	}
	sortLongitudinalCounterexamples(report.Counterexamples)
	report.Summary.Counterexamples = len(report.Counterexamples)
	report.OK = len(report.Counterexamples) == 0
	report.Hash = longitudinalStudyReportHash(report)
	return report, nil
}

func WriteLongitudinalStudyArtifacts(outDir string, report LongitudinalStudyReport) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	file, err := os.Create(filepath.Join(outDir, "longitudinal-education-study.json"))
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
	return os.WriteFile(filepath.Join(outDir, "longitudinal-education-study.md"), []byte(RenderLongitudinalStudyMarkdown(report)), 0o644)
}

func RenderLongitudinalStudyMarkdown(report LongitudinalStudyReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Longitudinal education study\n\n")
	fmt.Fprintf(&b, "Patchline measures whether trained reviewers still catch real, gate-backed hazards months later, using blinded follow-up observations, evidence citations, reproduced gate commands, and a control cohort.\n\n")
	fmt.Fprintf(&b, "## Summary\n\n")
	fmt.Fprintf(&b, "| Metric | Value |\n| --- | ---: |\n")
	fmt.Fprintf(&b, "| OK | `%t` |\n", report.OK)
	fmt.Fprintf(&b, "| Cohorts | %d |\n", report.Summary.Cohorts)
	fmt.Fprintf(&b, "| Participants | %d |\n", report.Summary.Participants)
	fmt.Fprintf(&b, "| Real hazards | %d |\n", report.Summary.RealHazards)
	fmt.Fprintf(&b, "| Held-out hazards | %d |\n", report.Summary.HeldOutHazards)
	fmt.Fprintf(&b, "| Gate-backed hazards | %d |\n", report.Summary.GateBackedHazards)
	fmt.Fprintf(&b, "| Training artifacts | %d |\n", report.Summary.TrainingArtifacts)
	fmt.Fprintf(&b, "| Evidence artifacts | %d |\n", report.Summary.EvidenceArtifacts)
	fmt.Fprintf(&b, "| Observations | %d |\n", report.Summary.Observations)
	fmt.Fprintf(&b, "| Delayed follow-up month | %d |\n", report.Summary.DelayedFollowupMonth)
	fmt.Fprintf(&b, "| Trained follow-up detection | %.2f%% |\n", report.Summary.TrainedFollowupDetectionRate)
	fmt.Fprintf(&b, "| Control follow-up detection | %.2f%% |\n", report.Summary.ControlFollowupDetectionRate)
	fmt.Fprintf(&b, "| Retention lift | %.2f points |\n", report.Summary.RetentionLiftPoints)
	fmt.Fprintf(&b, "| Counterexamples | %d |\n\n", report.Summary.Counterexamples)

	fmt.Fprintf(&b, "Policy: at least `%d` cohorts, `%d` real hazards, `%d` held-out hazards, month `%d` follow-up, `%d` real-hazard observations per cohort/timepoint, and `%.2f` percentage-point trained-over-control lift.\n\n",
		report.Criteria.MinCohorts,
		report.Criteria.MinRealHazards,
		report.Criteria.MinHeldOutHazards,
		report.Criteria.MinFollowupMonths,
		report.Criteria.MinObservationsPerCohortTimepoint,
		report.Criteria.MinRetentionLiftPoints,
	)

	fmt.Fprintf(&b, "## Gate-backed hazards\n\n")
	fmt.Fprintf(&b, "| Hazard | Class | Gate | Real | Held out | Evidence hashes |\n| --- | --- | --- | ---: | ---: | --- |\n")
	for _, hazard := range report.Hazards {
		hashes := make([]string, 0, len(hazard.Evidence))
		for _, evidence := range hazard.Evidence {
			hash := evidence.SHA256
			if len(hash) > 16 {
				hash = hash[:16]
			}
			hashes = append(hashes, evidence.Path+":"+hash)
		}
		fmt.Fprintf(&b, "| `%s` | `%s` | `%s` | `%t` | `%t` | %s |\n",
			escapeTable(hazard.ID),
			escapeTable(hazard.HazardClass),
			escapeTable(hazard.Gate),
			hazard.RealHazard,
			hazard.HeldOut,
			escapeTable(strings.Join(hashes, "; ")),
		)
	}

	fmt.Fprintf(&b, "\n## Cohort retention\n\n")
	fmt.Fprintf(&b, "| Cohort | Kind | Participants | Baseline | Follow-up | Rate change |\n| --- | --- | ---: | ---: | ---: | ---: |\n")
	for _, cohort := range report.Cohorts {
		change := round2Float(cohort.FollowupDetectionRate - cohort.BaselineDetectionRate)
		fmt.Fprintf(&b, "| `%s` | `%s` | %d | %.2f%% | %.2f%% | %.2f |\n",
			escapeTable(cohort.ID),
			escapeTable(cohort.Kind),
			cohort.Participants,
			cohort.BaselineDetectionRate,
			cohort.FollowupDetectionRate,
			change,
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

func validateLongitudinalStudySpec(spec LongitudinalStudySpec) error {
	if strings.TrimSpace(spec.Name) == "" {
		return fmt.Errorf("longitudinal education study name is required")
	}
	criteria := spec.Criteria
	if criteria.MinCohorts <= 0 {
		return fmt.Errorf("criteria.min_cohorts must be positive")
	}
	if criteria.MinRealHazards <= 0 {
		return fmt.Errorf("criteria.min_real_hazards must be positive")
	}
	if criteria.MinHeldOutHazards <= 0 {
		return fmt.Errorf("criteria.min_held_out_hazards must be positive")
	}
	if criteria.MinFollowupMonths <= 0 {
		return fmt.Errorf("criteria.min_followup_months must be positive")
	}
	if criteria.MinObservationsPerCohortTimepoint <= 0 {
		return fmt.Errorf("criteria.min_observations_per_cohort_timepoint must be positive")
	}
	if criteria.MinRetentionLiftPoints < 0 {
		return fmt.Errorf("criteria.min_retention_lift_points must be non-negative")
	}
	if strings.TrimSpace(spec.Protocol.RandomizationUnit) == "" || strings.TrimSpace(spec.Protocol.OutcomeDefinition) == "" {
		return fmt.Errorf("protocol requires randomization_unit and outcome_definition")
	}
	for _, path := range spec.Protocol.TrainingArtifacts {
		if err := validateRelativePath(path); err != nil {
			return fmt.Errorf("protocol training artifact: %w", err)
		}
	}
	cohortIDs := map[string]struct{}{}
	for _, cohort := range spec.Cohorts {
		if strings.TrimSpace(cohort.ID) == "" {
			return fmt.Errorf("cohort id is required")
		}
		if _, ok := cohortIDs[cohort.ID]; ok {
			return fmt.Errorf("duplicate cohort id %q", cohort.ID)
		}
		cohortIDs[cohort.ID] = struct{}{}
		kind := normalizeToken(cohort.Kind)
		if kind != "trained" && kind != "control" {
			return fmt.Errorf("cohort %q kind must be trained or control", cohort.ID)
		}
		if len(cohort.Participants) == 0 {
			return fmt.Errorf("cohort %q requires participants", cohort.ID)
		}
	}
	hazardIDs := map[string]struct{}{}
	for _, hazard := range spec.Hazards {
		if strings.TrimSpace(hazard.ID) == "" {
			return fmt.Errorf("hazard id is required")
		}
		if _, ok := hazardIDs[hazard.ID]; ok {
			return fmt.Errorf("duplicate hazard id %q", hazard.ID)
		}
		hazardIDs[hazard.ID] = struct{}{}
		if strings.TrimSpace(hazard.Title) == "" || strings.TrimSpace(hazard.Repo) == "" || strings.TrimSpace(hazard.HazardClass) == "" {
			return fmt.Errorf("hazard %q requires title, repo, and hazard_class", hazard.ID)
		}
		if strings.TrimSpace(hazard.Gate) == "" {
			return fmt.Errorf("hazard %q requires gate", hazard.ID)
		}
		if strings.TrimSpace(hazard.ExpectedDecision) == "" {
			return fmt.Errorf("hazard %q requires expected_decision", hazard.ID)
		}
		if len(hazard.EvidencePaths) == 0 {
			return fmt.Errorf("hazard %q requires evidence_paths", hazard.ID)
		}
		for _, path := range hazard.EvidencePaths {
			if err := validateRelativePath(path); err != nil {
				return fmt.Errorf("hazard %q evidence path: %w", hazard.ID, err)
			}
		}
	}
	if len(spec.Observations) == 0 {
		return fmt.Errorf("observations are required")
	}
	for _, observation := range spec.Observations {
		if strings.TrimSpace(observation.CohortID) == "" || strings.TrimSpace(observation.ReviewerID) == "" || strings.TrimSpace(observation.HazardID) == "" {
			return fmt.Errorf("observation requires cohort_id, reviewer_id, and hazard_id")
		}
		if observation.TimepointMonth < 0 {
			return fmt.Errorf("observation %s/%s has negative timepoint_month", observation.CohortID, observation.ReviewerID)
		}
	}
	return nil
}

func normalizeLongitudinalProtocol(protocol LongitudinalProtocol) LongitudinalProtocol {
	protocol.RandomizationUnit = normalizeCommand(protocol.RandomizationUnit)
	protocol.OutcomeDefinition = normalizeCommand(protocol.OutcomeDefinition)
	protocol.FollowupMonths = uniqueSortedInts(protocol.FollowupMonths)
	protocol.TrainingArtifacts = uniqueSorted(protocol.TrainingArtifacts)
	return protocol
}

func collectLongitudinalEvidence(root string, paths []string, subject string) ([]ArtifactEvidence, []LongitudinalCounterexample) {
	var artifacts []ArtifactEvidence
	var counterexamples []LongitudinalCounterexample
	for _, relPath := range uniqueSorted(paths) {
		fullPath, err := safeJoin(root, relPath)
		if err != nil {
			counterexamples = append(counterexamples, LongitudinalCounterexample{
				ID:      "hazard." + stableID(subject, relPath) + ".evidence_path",
				Kind:    "invalid_evidence_path",
				Subject: subject,
				Message: err.Error(),
				Witness: []string{relPath},
			})
			continue
		}
		data, err := os.ReadFile(fullPath)
		if err != nil {
			counterexamples = append(counterexamples, LongitudinalCounterexample{
				ID:      "hazard." + stableID(subject, relPath) + ".evidence_missing",
				Kind:    "missing_evidence",
				Subject: subject,
				Message: "hazard evidence could not be read",
				Witness: []string{relPath},
			})
			continue
		}
		if len(data) == 0 {
			counterexamples = append(counterexamples, LongitudinalCounterexample{
				ID:      "hazard." + stableID(subject, relPath) + ".evidence_empty",
				Kind:    "empty_evidence",
				Subject: subject,
				Message: "hazard evidence is empty",
				Witness: []string{relPath},
			})
			continue
		}
		sum := sha256.Sum256(data)
		artifacts = append(artifacts, ArtifactEvidence{
			Path:   filepath.ToSlash(filepath.Clean(relPath)),
			SHA256: hex.EncodeToString(sum[:]),
			Bytes:  int64(len(data)),
		})
	}
	return artifacts, counterexamples
}

func longitudinalObservationCounterexamples(criteria LongitudinalCriteria, hazard LongitudinalHazard, observation LongitudinalObservation) []LongitudinalCounterexample {
	if !observation.Detected {
		return nil
	}
	var counterexamples []LongitudinalCounterexample
	subject := observation.CohortID + "/" + observation.ReviewerID + "/" + observation.HazardID
	if criteria.RequireEvidenceCitations && !citesHazardEvidence(observation.EvidenceCitations, hazard.EvidencePaths) {
		counterexamples = append(counterexamples, LongitudinalCounterexample{
			ID:      "observation." + stableID(subject, "evidence") + ".citation",
			Kind:    "uncited_detection",
			Subject: subject,
			Message: "detected hazard does not cite one of the hazard's evidence paths",
			Witness: []string{observation.HazardID},
		})
	}
	if criteria.RequireGateCommandUseForDetections && !containsCommand(observation.Commands, requiredGateCommand(hazard.Gate)) {
		counterexamples = append(counterexamples, LongitudinalCounterexample{
			ID:      "observation." + stableID(subject, "command") + ".gate_command",
			Kind:    "missing_gate_command",
			Subject: subject,
			Message: "detected hazard does not include the reproducing gate command",
			Witness: []string{requiredGateCommand(hazard.Gate)},
		})
	}
	if strings.TrimSpace(observation.Decision) != strings.TrimSpace(hazard.ExpectedDecision) {
		counterexamples = append(counterexamples, LongitudinalCounterexample{
			ID:      "observation." + stableID(subject, "decision") + ".decision",
			Kind:    "decision_mismatch",
			Subject: subject,
			Message: "detected hazard uses a safety decision that does not match the expected decision",
			Witness: []string{observation.Decision, hazard.ExpectedDecision},
		})
	}
	return counterexamples
}

func longitudinalCoverageCounterexamples(spec LongitudinalStudySpec, report LongitudinalStudyReport, observations map[string]map[int][]LongitudinalObservation) []LongitudinalCounterexample {
	var counterexamples []LongitudinalCounterexample
	if report.Summary.Cohorts < spec.Criteria.MinCohorts {
		counterexamples = append(counterexamples, LongitudinalCounterexample{
			ID:      "criteria.min_cohorts",
			Kind:    "insufficient_cohorts",
			Message: fmt.Sprintf("cohorts %d below required %d", report.Summary.Cohorts, spec.Criteria.MinCohorts),
		})
	}
	if spec.Criteria.RequireControlCohort && report.Summary.ControlCohorts == 0 {
		counterexamples = append(counterexamples, LongitudinalCounterexample{
			ID:      "criteria.control_cohort",
			Kind:    "missing_control_cohort",
			Message: "study requires a control cohort for delayed follow-up comparison",
		})
	}
	if spec.Criteria.RequireTrainedCohort && report.Summary.TrainedCohorts == 0 {
		counterexamples = append(counterexamples, LongitudinalCounterexample{
			ID:      "criteria.trained_cohort",
			Kind:    "missing_trained_cohort",
			Message: "study requires a Patchline-trained cohort",
		})
	}
	if report.Summary.RealHazards < spec.Criteria.MinRealHazards {
		counterexamples = append(counterexamples, LongitudinalCounterexample{
			ID:      "criteria.real_hazards",
			Kind:    "insufficient_real_hazards",
			Message: fmt.Sprintf("real hazards %d below required %d", report.Summary.RealHazards, spec.Criteria.MinRealHazards),
		})
	}
	if report.Summary.HeldOutHazards < spec.Criteria.MinHeldOutHazards {
		counterexamples = append(counterexamples, LongitudinalCounterexample{
			ID:      "criteria.held_out_hazards",
			Kind:    "insufficient_held_out_hazards",
			Message: fmt.Sprintf("held-out hazards %d below required %d", report.Summary.HeldOutHazards, spec.Criteria.MinHeldOutHazards),
		})
	}
	if spec.Criteria.RequireBlindReview && !spec.Protocol.BlindReview {
		counterexamples = append(counterexamples, LongitudinalCounterexample{
			ID:      "protocol.blind_review",
			Kind:    "blind_protocol_missing",
			Message: "study requires blinded follow-up review",
		})
	}
	requiredMonths := []int{}
	if spec.Criteria.RequireBaseline {
		requiredMonths = append(requiredMonths, 0)
	}
	if report.Summary.DelayedFollowupMonth >= spec.Criteria.MinFollowupMonths {
		requiredMonths = append(requiredMonths, report.Summary.DelayedFollowupMonth)
	}
	for _, cohort := range sortedLongitudinalCohorts(spec.Cohorts) {
		for _, month := range uniqueSortedInts(requiredMonths) {
			count := len(observations[cohort.ID][month])
			if count < spec.Criteria.MinObservationsPerCohortTimepoint {
				counterexamples = append(counterexamples, LongitudinalCounterexample{
					ID:      "cohort." + stableID(cohort.ID, fmt.Sprintf("%d", month)) + ".observations",
					Kind:    "insufficient_timepoint_observations",
					Subject: cohort.ID,
					Message: fmt.Sprintf("cohort has %d real-hazard observations at month %d below required %d", count, month, spec.Criteria.MinObservationsPerCohortTimepoint),
				})
			}
		}
	}
	return counterexamples
}

func countQualifiedDetections(criteria LongitudinalCriteria, hazards map[string]LongitudinalHazard, observations []LongitudinalObservation) int {
	count := 0
	for _, observation := range observations {
		hazard, ok := hazards[observation.HazardID]
		if ok && qualifiedLongitudinalDetection(criteria, hazard, observation) {
			count++
		}
	}
	return count
}

func aggregateLongitudinalFollowup(criteria LongitudinalCriteria, hazards map[string]LongitudinalHazard, cohorts map[string]LongitudinalCohort, observations map[string]map[int][]LongitudinalObservation, kind string, month int) (int, int) {
	detections := 0
	total := 0
	for cohortID, cohort := range cohorts {
		if normalizeToken(cohort.Kind) != kind {
			continue
		}
		cohortObservations := observations[cohortID][month]
		total += len(cohortObservations)
		detections += countQualifiedDetections(criteria, hazards, cohortObservations)
	}
	return detections, total
}

func qualifiedLongitudinalDetection(criteria LongitudinalCriteria, hazard LongitudinalHazard, observation LongitudinalObservation) bool {
	if !hazard.RealHazard || !observation.Detected {
		return false
	}
	if criteria.RequireEvidenceCitations && !citesHazardEvidence(observation.EvidenceCitations, hazard.EvidencePaths) {
		return false
	}
	if criteria.RequireGateCommandUseForDetections && !containsCommand(observation.Commands, requiredGateCommand(hazard.Gate)) {
		return false
	}
	return strings.TrimSpace(observation.Decision) == strings.TrimSpace(hazard.ExpectedDecision)
}

func citesHazardEvidence(citations, evidencePaths []string) bool {
	allowed := map[string]struct{}{}
	for _, path := range evidencePaths {
		allowed[filepath.ToSlash(filepath.Clean(path))] = struct{}{}
	}
	for _, citation := range citations {
		if _, ok := allowed[filepath.ToSlash(filepath.Clean(citation))]; ok {
			return true
		}
	}
	return false
}

func longitudinalTimepoints(spec LongitudinalStudySpec) []int {
	months := append([]int(nil), spec.Protocol.FollowupMonths...)
	for _, observation := range spec.Observations {
		months = append(months, observation.TimepointMonth)
	}
	return uniqueSortedInts(months)
}

func sortedLongitudinalHazards(hazards []LongitudinalHazard) []LongitudinalHazard {
	out := append([]LongitudinalHazard(nil), hazards...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func sortedLongitudinalCohorts(cohorts []LongitudinalCohort) []LongitudinalCohort {
	out := append([]LongitudinalCohort(nil), cohorts...)
	sort.SliceStable(out, func(i, j int) bool {
		left := normalizeToken(out[i].Kind) + "\x00" + out[i].ID
		right := normalizeToken(out[j].Kind) + "\x00" + out[j].ID
		return left < right
	})
	return out
}

func sortedLongitudinalObservations(observations []LongitudinalObservation) []LongitudinalObservation {
	out := append([]LongitudinalObservation(nil), observations...)
	sort.SliceStable(out, func(i, j int) bool {
		left := fmt.Sprintf("%s\x00%03d\x00%s\x00%s", out[i].CohortID, out[i].TimepointMonth, out[i].ReviewerID, out[i].HazardID)
		right := fmt.Sprintf("%s\x00%03d\x00%s\x00%s", out[j].CohortID, out[j].TimepointMonth, out[j].ReviewerID, out[j].HazardID)
		return left < right
	})
	return out
}

func uniqueSortedInts(values []int) []int {
	seen := map[int]struct{}{}
	out := make([]int, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Ints(out)
	return out
}

func containsInt(values []int, want int) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func ratePercent(detections, observations int) float64 {
	if observations <= 0 {
		return 0
	}
	return round2Float(float64(detections) / float64(observations) * 100)
}

func round2Float(value float64) float64 {
	if value >= 0 {
		return float64(int(value*100+0.5)) / 100
	}
	return float64(int(value*100-0.5)) / 100
}

func sortLongitudinalCounterexamples(counterexamples []LongitudinalCounterexample) {
	sort.SliceStable(counterexamples, func(i, j int) bool { return counterexamples[i].ID < counterexamples[j].ID })
}

func longitudinalStudyReportHash(report LongitudinalStudyReport) string {
	report.Hash = ""
	return canonical.Hash(report)
}
