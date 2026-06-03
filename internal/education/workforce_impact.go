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

const WorkforceImpactSpecVersion = "patchline.workforce-impact-study/v1"
const WorkforceImpactReportVersion = "patchline.workforce-impact-study-report/v1"

type WorkforceImpactSpec struct {
	Version      string                  `json:"version"`
	Name         string                  `json:"name"`
	Claim        string                  `json:"claim,omitempty"`
	Criteria     WorkforceImpactCriteria `json:"criteria"`
	Protocol     WorkforceImpactProtocol `json:"protocol"`
	Automations  []WorkforceAutomation   `json:"automations"`
	Cohorts      []WorkforceCohort       `json:"cohorts"`
	Observations []WorkforceObservation  `json:"observations"`
}

type WorkforceImpactCriteria struct {
	MinCohorts                          int     `json:"min_cohorts"`
	MinAutomationReferences             int     `json:"min_automation_references"`
	MinObservationsPerCohortPeriod      int     `json:"min_observations_per_cohort_period"`
	MinOwnershipDiffInDiffPoints        float64 `json:"min_ownership_diff_in_diff_points"`
	MinEscalationDiffInDiffPoints       float64 `json:"min_escalation_diff_in_diff_points"`
	MinLearningDiffInDiffPoints         float64 `json:"min_learning_diff_in_diff_points"`
	MinHeldOutDetectionDiffInDiffPoints float64 `json:"min_held_out_detection_diff_in_diff_points"`
	MaxControlOwnershipShiftPoints      float64 `json:"max_control_ownership_shift_points"`
	MaxControlEscalationReductionPoints float64 `json:"max_control_escalation_reduction_points"`
	MaxControlLearningLiftPoints        float64 `json:"max_control_learning_lift_points"`
	MaxDefectRateIncreasePoints         float64 `json:"max_defect_rate_increase_points"`
	MaxAttritionRate                    float64 `json:"max_attrition_rate"`
	RequireControlCohort                bool    `json:"require_control_cohort"`
	RequireTreatedCohort                bool    `json:"require_treated_cohort"`
	RequireBeforeAfterPeriods           bool    `json:"require_before_after_periods"`
	RequireEvidenceCitations            bool    `json:"require_evidence_citations"`
	RequireGateCommandUse               bool    `json:"require_gate_command_use"`
	RequirePrivacyPreservingIDs         bool    `json:"require_privacy_preserving_ids"`
	RequireAutomationGateBacked         bool    `json:"require_automation_gate_backed"`
	RequireHeldOutDetectionLift         bool    `json:"require_held_out_detection_lift"`
	RequireQualityGuard                 bool    `json:"require_quality_guard"`
}

type WorkforceImpactProtocol struct {
	InterventionName  string `json:"intervention_name"`
	BeforePeriod      string `json:"before_period"`
	AfterPeriod       string `json:"after_period"`
	AssignmentUnit    string `json:"assignment_unit"`
	OwnershipOutcome  string `json:"ownership_outcome"`
	EscalationOutcome string `json:"escalation_outcome"`
	LearningOutcome   string `json:"learning_outcome"`
	QualityOutcome    string `json:"quality_outcome"`
}

type WorkforceAutomation struct {
	ID            string   `json:"id"`
	Gate          string   `json:"gate"`
	Description   string   `json:"description"`
	Commands      []string `json:"commands"`
	EvidencePaths []string `json:"evidence_paths"`
}

type WorkforceCohort struct {
	ID           string   `json:"id"`
	Kind         string   `json:"kind"`
	Description  string   `json:"description"`
	Participants []string `json:"participants"`
}

type WorkforceObservation struct {
	ReviewID                string   `json:"review_id"`
	CohortID                string   `json:"cohort_id"`
	ParticipantID           string   `json:"participant_id"`
	Period                  string   `json:"period"`
	Team                    string   `json:"team"`
	Ecosystem               string   `json:"ecosystem"`
	OwnedByPrimaryTeam      bool     `json:"owned_by_primary_team"`
	Escalations             int      `json:"escalations"`
	DownstreamMisses        int      `json:"downstream_misses"`
	LearningAssessmentScore float64  `json:"learning_assessment_score"`
	HeldOutDetections       int      `json:"held_out_detections"`
	HeldOutOpportunities    int      `json:"held_out_opportunities"`
	AutomationRefs          []string `json:"automation_refs"`
	Commands                []string `json:"commands"`
	EvidencePaths           []string `json:"evidence_paths"`
}

type WorkforceImpactReport struct {
	Version         string                       `json:"version"`
	Name            string                       `json:"name"`
	OK              bool                         `json:"ok"`
	Criteria        WorkforceImpactCriteria      `json:"criteria"`
	Protocol        WorkforceImpactProtocol      `json:"protocol"`
	Summary         WorkforceImpactSummary       `json:"summary"`
	Automations     []WorkforceAutomationReport  `json:"automations"`
	Cohorts         []WorkforceCohortReport      `json:"cohorts"`
	Observations    []WorkforceObservationReport `json:"observations"`
	Counterexamples []WorkforceImpactCountercase `json:"counterexamples,omitempty"`
	Hash            string                       `json:"hash"`
}

type WorkforceImpactSummary struct {
	Cohorts                                 int     `json:"cohorts"`
	TreatedCohorts                          int     `json:"treated_cohorts"`
	ControlCohorts                          int     `json:"control_cohorts"`
	Participants                            int     `json:"participants"`
	Observations                            int     `json:"observations"`
	AutomationReferences                    int     `json:"automation_references"`
	GateBackedAutomations                   int     `json:"gate_backed_automations"`
	EvidenceArtifacts                       int     `json:"evidence_artifacts"`
	TreatedOwnershipBefore                  float64 `json:"treated_ownership_before"`
	TreatedOwnershipAfter                   float64 `json:"treated_ownership_after"`
	ControlOwnershipBefore                  float64 `json:"control_ownership_before"`
	ControlOwnershipAfter                   float64 `json:"control_ownership_after"`
	TreatedOwnershipDeltaPoints             float64 `json:"treated_ownership_delta_points"`
	ControlOwnershipDeltaPoints             float64 `json:"control_ownership_delta_points"`
	OwnershipDiffInDiffPoints               float64 `json:"ownership_diff_in_diff_points"`
	TreatedEscalationRateBefore             float64 `json:"treated_escalation_rate_before"`
	TreatedEscalationRateAfter              float64 `json:"treated_escalation_rate_after"`
	ControlEscalationRateBefore             float64 `json:"control_escalation_rate_before"`
	ControlEscalationRateAfter              float64 `json:"control_escalation_rate_after"`
	TreatedEscalationReductionPoints        float64 `json:"treated_escalation_reduction_points"`
	ControlEscalationReductionPoints        float64 `json:"control_escalation_reduction_points"`
	EscalationDiffInDiffPoints              float64 `json:"escalation_diff_in_diff_points"`
	TreatedDownstreamMissRateBefore         float64 `json:"treated_downstream_miss_rate_before"`
	TreatedDownstreamMissRateAfter          float64 `json:"treated_downstream_miss_rate_after"`
	TreatedDownstreamMissRateIncreasePoints float64 `json:"treated_downstream_miss_rate_increase_points"`
	TreatedLearningScoreBefore              float64 `json:"treated_learning_score_before"`
	TreatedLearningScoreAfter               float64 `json:"treated_learning_score_after"`
	ControlLearningScoreBefore              float64 `json:"control_learning_score_before"`
	ControlLearningScoreAfter               float64 `json:"control_learning_score_after"`
	TreatedLearningDeltaPoints              float64 `json:"treated_learning_delta_points"`
	ControlLearningDeltaPoints              float64 `json:"control_learning_delta_points"`
	LearningDiffInDiffPoints                float64 `json:"learning_diff_in_diff_points"`
	TreatedHeldOutDetectionBefore           float64 `json:"treated_held_out_detection_before"`
	TreatedHeldOutDetectionAfter            float64 `json:"treated_held_out_detection_after"`
	ControlHeldOutDetectionBefore           float64 `json:"control_held_out_detection_before"`
	ControlHeldOutDetectionAfter            float64 `json:"control_held_out_detection_after"`
	TreatedHeldOutDetectionDeltaPoints      float64 `json:"treated_held_out_detection_delta_points"`
	ControlHeldOutDetectionDeltaPoints      float64 `json:"control_held_out_detection_delta_points"`
	HeldOutDetectionDiffInDiffPoints        float64 `json:"held_out_detection_diff_in_diff_points"`
	MaxAttritionRateObserved                float64 `json:"max_attrition_rate_observed"`
	Counterexamples                         int     `json:"counterexamples"`
}

type WorkforceAutomationReport struct {
	ID                    string             `json:"id"`
	Gate                  string             `json:"gate"`
	Description           string             `json:"description"`
	GateBacked            bool               `json:"gate_backed"`
	ReproducibleCommandOK bool               `json:"reproducible_command_ok"`
	Evidence              []ArtifactEvidence `json:"evidence"`
}

type WorkforceCohortReport struct {
	ID                       string  `json:"id"`
	Kind                     string  `json:"kind"`
	Participants             int     `json:"participants"`
	BeforeObservations       int     `json:"before_observations"`
	AfterObservations        int     `json:"after_observations"`
	OwnershipBefore          float64 `json:"ownership_before"`
	OwnershipAfter           float64 `json:"ownership_after"`
	EscalationRateBefore     float64 `json:"escalation_rate_before"`
	EscalationRateAfter      float64 `json:"escalation_rate_after"`
	DownstreamMissRateBefore float64 `json:"downstream_miss_rate_before"`
	DownstreamMissRateAfter  float64 `json:"downstream_miss_rate_after"`
	LearningScoreBefore      float64 `json:"learning_score_before"`
	LearningScoreAfter       float64 `json:"learning_score_after"`
	HeldOutDetectionBefore   float64 `json:"held_out_detection_before"`
	HeldOutDetectionAfter    float64 `json:"held_out_detection_after"`
	AttritionRate            float64 `json:"attrition_rate"`
}

type WorkforceObservationReport struct {
	ReviewID                string             `json:"review_id"`
	CohortID                string             `json:"cohort_id"`
	ParticipantID           string             `json:"participant_id"`
	Period                  string             `json:"period"`
	Team                    string             `json:"team"`
	Ecosystem               string             `json:"ecosystem"`
	OwnedByPrimaryTeam      bool               `json:"owned_by_primary_team"`
	Escalations             int                `json:"escalations"`
	DownstreamMisses        int                `json:"downstream_misses"`
	LearningAssessmentScore float64            `json:"learning_assessment_score"`
	HeldOutDetectionRate    float64            `json:"held_out_detection_rate"`
	AutomationRefs          []string           `json:"automation_refs"`
	AutomationCommandsOK    bool               `json:"automation_commands_ok"`
	Evidence                []ArtifactEvidence `json:"evidence"`
}

type WorkforceImpactCountercase struct {
	ID      string   `json:"id"`
	Kind    string   `json:"kind"`
	Subject string   `json:"subject,omitempty"`
	Message string   `json:"message"`
	Witness []string `json:"witness,omitempty"`
}

type workforceAggregate struct {
	observations      int
	owned             int
	escalated         int
	escalations       int
	downstreamMisses  int
	learningScore     float64
	heldOutDetections int
	heldOutTotal      int
	participants      map[string]struct{}
}

func ReadWorkforceImpactSpec(reader io.Reader) (WorkforceImpactSpec, error) {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var spec WorkforceImpactSpec
	if err := decoder.Decode(&spec); err != nil {
		return WorkforceImpactSpec{}, err
	}
	if spec.Version != WorkforceImpactSpecVersion {
		return WorkforceImpactSpec{}, fmt.Errorf("workforce impact study spec version must be %s", WorkforceImpactSpecVersion)
	}
	return spec, nil
}

func BuildWorkforceImpactReport(spec WorkforceImpactSpec, root string) (WorkforceImpactReport, error) {
	if err := validateWorkforceImpactSpec(spec); err != nil {
		return WorkforceImpactReport{}, err
	}
	if root == "" {
		root = "."
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return WorkforceImpactReport{}, err
	}

	report := WorkforceImpactReport{
		Version:  WorkforceImpactReportVersion,
		Name:     spec.Name,
		OK:       true,
		Criteria: spec.Criteria,
		Protocol: normalizeWorkforceProtocol(spec.Protocol),
	}
	automationByID := map[string]WorkforceAutomation{}
	for _, automation := range spec.Automations {
		automationByID[automation.ID] = automation
	}
	for _, automation := range sortedWorkforceAutomations(spec.Automations) {
		evidence, counterexamples := collectWorkforceEvidence(rootAbs, automation.EvidencePaths, automation.ID)
		report.Counterexamples = append(report.Counterexamples, counterexamples...)
		gateBacked := gateExists(rootAbs, automation.Gate)
		commandOK := containsCommand(automation.Commands, requiredGateCommand(automation.Gate))
		report.Automations = append(report.Automations, WorkforceAutomationReport{
			ID:                    automation.ID,
			Gate:                  automation.Gate,
			Description:           strings.TrimSpace(automation.Description),
			GateBacked:            gateBacked,
			ReproducibleCommandOK: commandOK,
			Evidence:              evidence,
		})
		report.Summary.EvidenceArtifacts += len(evidence)
		if gateBacked {
			report.Summary.GateBackedAutomations++
		}
		if spec.Criteria.RequireEvidenceCitations && len(evidence) == 0 {
			report.Counterexamples = append(report.Counterexamples, WorkforceImpactCountercase{
				ID:      "automation." + stableID(automation.ID, "evidence") + ".missing",
				Kind:    "missing_evidence",
				Subject: automation.ID,
				Message: "automation reference must cite at least one readable evidence artifact",
			})
		}
		if spec.Criteria.RequireAutomationGateBacked && !gateBacked {
			report.Counterexamples = append(report.Counterexamples, WorkforceImpactCountercase{
				ID:      "automation." + stableID(automation.ID, "gate") + ".missing",
				Kind:    "missing_gate_reference",
				Subject: automation.ID,
				Message: "automation reference is not backed by a Make target and script",
				Witness: []string{automation.Gate},
			})
		}
		if spec.Criteria.RequireGateCommandUse && !commandOK {
			report.Counterexamples = append(report.Counterexamples, WorkforceImpactCountercase{
				ID:      "automation." + stableID(automation.ID, "command") + ".missing",
				Kind:    "non_reproducible_automation",
				Subject: automation.ID,
				Message: "automation reference does not list its reproducing make command",
				Witness: []string{requiredGateCommand(automation.Gate)},
			})
		}
	}

	cohortByID := map[string]WorkforceCohort{}
	for _, cohort := range spec.Cohorts {
		cohortByID[cohort.ID] = cohort
	}
	observationsByCohortPeriod := map[string]map[string][]WorkforceObservation{}
	participants := map[string]struct{}{}
	for _, observation := range sortedWorkforceObservations(spec.Observations) {
		cohort, cohortOK := cohortByID[observation.CohortID]
		if !cohortOK {
			report.Counterexamples = append(report.Counterexamples, WorkforceImpactCountercase{
				ID:      "observation." + stableID(observation.ReviewID, observation.CohortID) + ".cohort",
				Kind:    "unknown_cohort",
				Subject: observation.CohortID,
				Message: "observation references a cohort that is not declared",
			})
			continue
		}
		if observationsByCohortPeriod[cohort.ID] == nil {
			observationsByCohortPeriod[cohort.ID] = map[string][]WorkforceObservation{}
		}
		period := normalizeWorkforcePeriod(observation.Period)
		observationsByCohortPeriod[cohort.ID][period] = append(observationsByCohortPeriod[cohort.ID][period], observation)
		participants[observation.ParticipantID] = struct{}{}
		report.Summary.Observations++

		evidence, counterexamples := collectWorkforceEvidence(rootAbs, observation.EvidencePaths, observation.ReviewID)
		report.Counterexamples = append(report.Counterexamples, counterexamples...)
		if spec.Criteria.RequireEvidenceCitations && len(evidence) == 0 {
			report.Counterexamples = append(report.Counterexamples, WorkforceImpactCountercase{
				ID:      "observation." + stableID(observation.ReviewID, "evidence") + ".missing",
				Kind:    "missing_evidence",
				Subject: observation.ReviewID,
				Message: "observation must cite at least one readable evidence artifact",
			})
		}
		report.Summary.EvidenceArtifacts += len(evidence)

		if spec.Criteria.RequirePrivacyPreservingIDs && !privacyPreservingID(observation.ParticipantID) {
			report.Counterexamples = append(report.Counterexamples, WorkforceImpactCountercase{
				ID:      "observation." + stableID(observation.ReviewID, observation.ParticipantID) + ".privacy",
				Kind:    "pii_like_identifier",
				Subject: observation.ReviewID,
				Message: "participant identifier is not privacy-preserving",
				Witness: []string{observation.ParticipantID},
			})
		}

		automationCommandsOK := true
		if period == report.Protocol.AfterPeriod && spec.Criteria.RequireAutomationGateBacked && len(observation.AutomationRefs) == 0 {
			automationCommandsOK = false
			report.Counterexamples = append(report.Counterexamples, WorkforceImpactCountercase{
				ID:      "observation." + stableID(observation.ReviewID, "automation") + ".missing",
				Kind:    "missing_automation_reference",
				Subject: observation.ReviewID,
				Message: "post-intervention observation must name the automation references it used",
			})
		}
		for _, ref := range uniqueSorted(observation.AutomationRefs) {
			automation, ok := automationByID[ref]
			if !ok {
				automationCommandsOK = false
				report.Counterexamples = append(report.Counterexamples, WorkforceImpactCountercase{
					ID:      "observation." + stableID(observation.ReviewID, ref) + ".unknown_automation",
					Kind:    "unknown_automation_reference",
					Subject: observation.ReviewID,
					Message: "observation references an automation not declared in the study",
					Witness: []string{ref},
				})
				continue
			}
			if spec.Criteria.RequireGateCommandUse && !containsCommand(observation.Commands, requiredGateCommand(automation.Gate)) {
				automationCommandsOK = false
				report.Counterexamples = append(report.Counterexamples, WorkforceImpactCountercase{
					ID:      "observation." + stableID(observation.ReviewID, ref, "command") + ".missing",
					Kind:    "missing_gate_command",
					Subject: observation.ReviewID,
					Message: "observation did not record the reproducing command for its automation reference",
					Witness: []string{requiredGateCommand(automation.Gate)},
				})
			}
		}

		report.Observations = append(report.Observations, WorkforceObservationReport{
			ReviewID:                observation.ReviewID,
			CohortID:                observation.CohortID,
			ParticipantID:           observation.ParticipantID,
			Period:                  period,
			Team:                    normalizeToken(observation.Team),
			Ecosystem:               normalizeToken(observation.Ecosystem),
			OwnedByPrimaryTeam:      observation.OwnedByPrimaryTeam,
			Escalations:             observation.Escalations,
			DownstreamMisses:        observation.DownstreamMisses,
			LearningAssessmentScore: round2Float(observation.LearningAssessmentScore),
			HeldOutDetectionRate:    ratePercent(observation.HeldOutDetections, observation.HeldOutOpportunities),
			AutomationRefs:          uniqueSorted(observation.AutomationRefs),
			AutomationCommandsOK:    automationCommandsOK,
			Evidence:                evidence,
		})
	}

	report.Summary.Participants = len(participants)
	for _, cohort := range sortedWorkforceCohorts(spec.Cohorts) {
		kind := normalizeToken(cohort.Kind)
		before := aggregateWorkforceObservations(observationsByCohortPeriod[cohort.ID][report.Protocol.BeforePeriod])
		after := aggregateWorkforceObservations(observationsByCohortPeriod[cohort.ID][report.Protocol.AfterPeriod])
		cohortReport := WorkforceCohortReport{
			ID:                       cohort.ID,
			Kind:                     kind,
			Participants:             len(uniqueSorted(cohort.Participants)),
			BeforeObservations:       before.observations,
			AfterObservations:        after.observations,
			OwnershipBefore:          before.ownershipRate(),
			OwnershipAfter:           after.ownershipRate(),
			EscalationRateBefore:     before.escalationRate(),
			EscalationRateAfter:      after.escalationRate(),
			DownstreamMissRateBefore: before.downstreamMissRate(),
			DownstreamMissRateAfter:  after.downstreamMissRate(),
			LearningScoreBefore:      before.learningMean(),
			LearningScoreAfter:       after.learningMean(),
			HeldOutDetectionBefore:   before.heldOutDetectionRate(),
			HeldOutDetectionAfter:    after.heldOutDetectionRate(),
			AttritionRate:            participantAttritionRate(before.participants, after.participants),
		}
		report.Cohorts = append(report.Cohorts, cohortReport)
		report.Summary.MaxAttritionRateObserved = maxFloat(report.Summary.MaxAttritionRateObserved, cohortReport.AttritionRate)
		if kind == "treated" {
			report.Summary.TreatedCohorts++
		}
		if kind == "control" {
			report.Summary.ControlCohorts++
		}
	}
	report.Summary.Cohorts = len(report.Cohorts)
	report.Summary.AutomationReferences = len(report.Automations)
	populateWorkforceSummaryMetrics(&report, observationsByCohortPeriod, cohortByID)
	report.Counterexamples = append(report.Counterexamples, workforceCriteriaCounterexamples(spec, report, observationsByCohortPeriod)...)
	sortWorkforceCounterexamples(report.Counterexamples)
	report.Summary.Counterexamples = len(report.Counterexamples)
	report.OK = len(report.Counterexamples) == 0
	report.Hash = workforceImpactReportHash(report)
	return report, nil
}

func WriteWorkforceImpactArtifacts(outDir string, report WorkforceImpactReport) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	file, err := os.Create(filepath.Join(outDir, "workforce-impact-study.json"))
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
	return os.WriteFile(filepath.Join(outDir, "workforce-impact-study.md"), []byte(RenderWorkforceImpactMarkdown(report)), 0o644)
}

func RenderWorkforceImpactMarkdown(report WorkforceImpactReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Workforce-impact study\n\n")
	fmt.Fprintf(&b, "Patchline measures whether automation changes review ownership, escalation load, and learning outcomes using a difference-in-differences design, evidence-hashed observations, gate-backed automation references, and quality guards against suppressed escalation.\n\n")
	fmt.Fprintf(&b, "## Summary\n\n")
	fmt.Fprintf(&b, "| Metric | Value |\n| --- | ---: |\n")
	fmt.Fprintf(&b, "| OK | `%t` |\n", report.OK)
	fmt.Fprintf(&b, "| Cohorts | %d |\n", report.Summary.Cohorts)
	fmt.Fprintf(&b, "| Observations | %d |\n", report.Summary.Observations)
	fmt.Fprintf(&b, "| Automation references | %d |\n", report.Summary.AutomationReferences)
	fmt.Fprintf(&b, "| Gate-backed automations | %d |\n", report.Summary.GateBackedAutomations)
	fmt.Fprintf(&b, "| Evidence artifacts | %d |\n", report.Summary.EvidenceArtifacts)
	fmt.Fprintf(&b, "| Ownership diff-in-diff | %.2f points |\n", report.Summary.OwnershipDiffInDiffPoints)
	fmt.Fprintf(&b, "| Escalation diff-in-diff | %.2f points |\n", report.Summary.EscalationDiffInDiffPoints)
	fmt.Fprintf(&b, "| Learning diff-in-diff | %.2f points |\n", report.Summary.LearningDiffInDiffPoints)
	fmt.Fprintf(&b, "| Held-out detection diff-in-diff | %.2f points |\n", report.Summary.HeldOutDetectionDiffInDiffPoints)
	fmt.Fprintf(&b, "| Treated downstream-miss increase | %.2f points |\n", report.Summary.TreatedDownstreamMissRateIncreasePoints)
	fmt.Fprintf(&b, "| Max attrition rate | %.2f |\n", report.Summary.MaxAttritionRateObserved)
	fmt.Fprintf(&b, "| Counterexamples | %d |\n\n", report.Summary.Counterexamples)

	fmt.Fprintf(&b, "Policy: at least `%d` cohorts, `%d` automation references, `%d` observations per cohort/period, ownership diff-in-diff at least `%.2f`, escalation-load diff-in-diff at least `%.2f`, learning diff-in-diff at least `%.2f`, held-out detection diff-in-diff at least `%.2f`, and downstream miss-rate increase at most `%.2f` points.\n\n",
		report.Criteria.MinCohorts,
		report.Criteria.MinAutomationReferences,
		report.Criteria.MinObservationsPerCohortPeriod,
		report.Criteria.MinOwnershipDiffInDiffPoints,
		report.Criteria.MinEscalationDiffInDiffPoints,
		report.Criteria.MinLearningDiffInDiffPoints,
		report.Criteria.MinHeldOutDetectionDiffInDiffPoints,
		report.Criteria.MaxDefectRateIncreasePoints,
	)

	fmt.Fprintf(&b, "## Difference-in-differences\n\n")
	fmt.Fprintf(&b, "| Dimension | Treated before | Treated after | Control before | Control after | Diff-in-diff |\n| --- | ---: | ---: | ---: | ---: | ---: |\n")
	fmt.Fprintf(&b, "| Ownership by primary team | %.2f | %.2f | %.2f | %.2f | %.2f |\n", report.Summary.TreatedOwnershipBefore, report.Summary.TreatedOwnershipAfter, report.Summary.ControlOwnershipBefore, report.Summary.ControlOwnershipAfter, report.Summary.OwnershipDiffInDiffPoints)
	fmt.Fprintf(&b, "| Escalation rate | %.2f | %.2f | %.2f | %.2f | %.2f |\n", report.Summary.TreatedEscalationRateBefore, report.Summary.TreatedEscalationRateAfter, report.Summary.ControlEscalationRateBefore, report.Summary.ControlEscalationRateAfter, report.Summary.EscalationDiffInDiffPoints)
	fmt.Fprintf(&b, "| Learning score | %.2f | %.2f | %.2f | %.2f | %.2f |\n", report.Summary.TreatedLearningScoreBefore, report.Summary.TreatedLearningScoreAfter, report.Summary.ControlLearningScoreBefore, report.Summary.ControlLearningScoreAfter, report.Summary.LearningDiffInDiffPoints)
	fmt.Fprintf(&b, "| Held-out detection | %.2f | %.2f | %.2f | %.2f | %.2f |\n", report.Summary.TreatedHeldOutDetectionBefore, report.Summary.TreatedHeldOutDetectionAfter, report.Summary.ControlHeldOutDetectionBefore, report.Summary.ControlHeldOutDetectionAfter, report.Summary.HeldOutDetectionDiffInDiffPoints)

	fmt.Fprintf(&b, "\n## Cohorts\n\n")
	fmt.Fprintf(&b, "| Cohort | Kind | Participants | Before obs | After obs | Ownership before/after | Escalation before/after | Learning before/after | Attrition |\n| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |\n")
	for _, cohort := range report.Cohorts {
		fmt.Fprintf(&b, "| `%s` | `%s` | %d | %d | %d | %.2f / %.2f | %.2f / %.2f | %.2f / %.2f | %.2f |\n",
			escapeTable(cohort.ID),
			escapeTable(cohort.Kind),
			cohort.Participants,
			cohort.BeforeObservations,
			cohort.AfterObservations,
			cohort.OwnershipBefore,
			cohort.OwnershipAfter,
			cohort.EscalationRateBefore,
			cohort.EscalationRateAfter,
			cohort.LearningScoreBefore,
			cohort.LearningScoreAfter,
			cohort.AttritionRate,
		)
	}

	fmt.Fprintf(&b, "\n## Automation references\n\n")
	fmt.Fprintf(&b, "| Automation | Gate | Gate-backed | Command OK | Evidence hashes |\n| --- | --- | ---: | ---: | --- |\n")
	for _, automation := range report.Automations {
		hashes := make([]string, 0, len(automation.Evidence))
		for _, evidence := range automation.Evidence {
			hash := evidence.SHA256
			if len(hash) > 16 {
				hash = hash[:16]
			}
			hashes = append(hashes, evidence.Path+":"+hash)
		}
		fmt.Fprintf(&b, "| `%s` | `%s` | `%t` | `%t` | %s |\n",
			escapeTable(automation.ID),
			escapeTable(automation.Gate),
			automation.GateBacked,
			automation.ReproducibleCommandOK,
			escapeTable(strings.Join(hashes, "; ")),
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

func validateWorkforceImpactSpec(spec WorkforceImpactSpec) error {
	if strings.TrimSpace(spec.Name) == "" {
		return fmt.Errorf("workforce impact study name is required")
	}
	criteria := spec.Criteria
	if criteria.MinCohorts < 2 {
		return fmt.Errorf("criteria.min_cohorts must be at least 2")
	}
	if criteria.MinAutomationReferences < 1 {
		return fmt.Errorf("criteria.min_automation_references must be positive")
	}
	if criteria.MinObservationsPerCohortPeriod < 1 {
		return fmt.Errorf("criteria.min_observations_per_cohort_period must be positive")
	}
	if criteria.MinOwnershipDiffInDiffPoints < 0 || criteria.MinEscalationDiffInDiffPoints < 0 || criteria.MinLearningDiffInDiffPoints < 0 || criteria.MinHeldOutDetectionDiffInDiffPoints < 0 {
		return fmt.Errorf("criteria minimum diff-in-diff thresholds must be non-negative")
	}
	if criteria.MaxControlOwnershipShiftPoints < 0 || criteria.MaxControlEscalationReductionPoints < 0 || criteria.MaxControlLearningLiftPoints < 0 || criteria.MaxDefectRateIncreasePoints < 0 {
		return fmt.Errorf("criteria maximum control and defect thresholds must be non-negative")
	}
	if criteria.MaxAttritionRate < 0 || criteria.MaxAttritionRate > 1 {
		return fmt.Errorf("criteria.max_attrition_rate must be between 0 and 1")
	}
	if strings.TrimSpace(spec.Protocol.InterventionName) == "" || strings.TrimSpace(spec.Protocol.BeforePeriod) == "" || strings.TrimSpace(spec.Protocol.AfterPeriod) == "" {
		return fmt.Errorf("protocol requires intervention_name, before_period, and after_period")
	}
	if normalizeWorkforcePeriod(spec.Protocol.BeforePeriod) == normalizeWorkforcePeriod(spec.Protocol.AfterPeriod) {
		return fmt.Errorf("protocol before_period and after_period must differ")
	}
	if strings.TrimSpace(spec.Protocol.AssignmentUnit) == "" || strings.TrimSpace(spec.Protocol.OwnershipOutcome) == "" || strings.TrimSpace(spec.Protocol.EscalationOutcome) == "" || strings.TrimSpace(spec.Protocol.LearningOutcome) == "" || strings.TrimSpace(spec.Protocol.QualityOutcome) == "" {
		return fmt.Errorf("protocol requires assignment_unit, ownership_outcome, escalation_outcome, learning_outcome, and quality_outcome")
	}
	automationIDs := map[string]struct{}{}
	for _, automation := range spec.Automations {
		if strings.TrimSpace(automation.ID) == "" {
			return fmt.Errorf("automation id is required")
		}
		if _, ok := automationIDs[automation.ID]; ok {
			return fmt.Errorf("duplicate automation id %q", automation.ID)
		}
		automationIDs[automation.ID] = struct{}{}
		if strings.TrimSpace(automation.Gate) == "" || strings.TrimSpace(automation.Description) == "" {
			return fmt.Errorf("automation %q requires gate and description", automation.ID)
		}
		for _, path := range automation.EvidencePaths {
			if err := validateRelativePath(path); err != nil {
				return fmt.Errorf("automation %q evidence path: %w", automation.ID, err)
			}
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
		if kind != "treated" && kind != "control" {
			return fmt.Errorf("cohort %q kind must be treated or control", cohort.ID)
		}
		if len(cohort.Participants) == 0 {
			return fmt.Errorf("cohort %q requires participants", cohort.ID)
		}
	}
	if len(spec.Observations) == 0 {
		return fmt.Errorf("observations are required")
	}
	observationIDs := map[string]struct{}{}
	for _, observation := range spec.Observations {
		if strings.TrimSpace(observation.ReviewID) == "" || strings.TrimSpace(observation.CohortID) == "" || strings.TrimSpace(observation.ParticipantID) == "" || strings.TrimSpace(observation.Period) == "" {
			return fmt.Errorf("observation requires review_id, cohort_id, participant_id, and period")
		}
		if _, ok := observationIDs[observation.ReviewID]; ok {
			return fmt.Errorf("duplicate observation review_id %q", observation.ReviewID)
		}
		observationIDs[observation.ReviewID] = struct{}{}
		if strings.TrimSpace(observation.Team) == "" || strings.TrimSpace(observation.Ecosystem) == "" {
			return fmt.Errorf("observation %q requires team and ecosystem", observation.ReviewID)
		}
		if observation.Escalations < 0 || observation.DownstreamMisses < 0 {
			return fmt.Errorf("observation %q has negative escalation or downstream miss count", observation.ReviewID)
		}
		if observation.LearningAssessmentScore < 0 || observation.LearningAssessmentScore > 100 {
			return fmt.Errorf("observation %q learning_assessment_score must be between 0 and 100", observation.ReviewID)
		}
		if observation.HeldOutDetections < 0 || observation.HeldOutOpportunities < 0 || observation.HeldOutDetections > observation.HeldOutOpportunities {
			return fmt.Errorf("observation %q held-out detections must be between 0 and opportunities", observation.ReviewID)
		}
		for _, path := range observation.EvidencePaths {
			if err := validateRelativePath(path); err != nil {
				return fmt.Errorf("observation %q evidence path: %w", observation.ReviewID, err)
			}
		}
	}
	return nil
}

func normalizeWorkforceProtocol(protocol WorkforceImpactProtocol) WorkforceImpactProtocol {
	protocol.InterventionName = strings.Join(strings.Fields(strings.TrimSpace(protocol.InterventionName)), " ")
	protocol.BeforePeriod = normalizeWorkforcePeriod(protocol.BeforePeriod)
	protocol.AfterPeriod = normalizeWorkforcePeriod(protocol.AfterPeriod)
	protocol.AssignmentUnit = normalizeToken(protocol.AssignmentUnit)
	protocol.OwnershipOutcome = strings.Join(strings.Fields(strings.TrimSpace(protocol.OwnershipOutcome)), " ")
	protocol.EscalationOutcome = strings.Join(strings.Fields(strings.TrimSpace(protocol.EscalationOutcome)), " ")
	protocol.LearningOutcome = strings.Join(strings.Fields(strings.TrimSpace(protocol.LearningOutcome)), " ")
	protocol.QualityOutcome = strings.Join(strings.Fields(strings.TrimSpace(protocol.QualityOutcome)), " ")
	return protocol
}

func normalizeWorkforcePeriod(period string) string {
	return normalizeToken(period)
}

func collectWorkforceEvidence(root string, paths []string, subject string) ([]ArtifactEvidence, []WorkforceImpactCountercase) {
	var artifacts []ArtifactEvidence
	var counterexamples []WorkforceImpactCountercase
	for _, relPath := range uniqueSorted(paths) {
		fullPath, err := safeJoin(root, relPath)
		if err != nil {
			counterexamples = append(counterexamples, WorkforceImpactCountercase{
				ID:      "workforce." + stableID(subject, relPath, "evidence-path") + ".invalid",
				Kind:    "invalid_evidence_path",
				Subject: subject,
				Message: err.Error(),
				Witness: []string{relPath},
			})
			continue
		}
		data, err := os.ReadFile(fullPath)
		if err != nil {
			counterexamples = append(counterexamples, WorkforceImpactCountercase{
				ID:      "workforce." + stableID(subject, relPath, "evidence") + ".missing",
				Kind:    "missing_evidence",
				Subject: subject,
				Message: "workforce-impact evidence could not be read",
				Witness: []string{relPath},
			})
			continue
		}
		if len(data) == 0 {
			counterexamples = append(counterexamples, WorkforceImpactCountercase{
				ID:      "workforce." + stableID(subject, relPath, "evidence") + ".empty",
				Kind:    "empty_evidence",
				Subject: subject,
				Message: "workforce-impact evidence is empty",
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

func populateWorkforceSummaryMetrics(report *WorkforceImpactReport, observations map[string]map[string][]WorkforceObservation, cohorts map[string]WorkforceCohort) {
	before := report.Protocol.BeforePeriod
	after := report.Protocol.AfterPeriod
	treatedBefore := aggregateWorkforceKind(cohorts, observations, "treated", before)
	treatedAfter := aggregateWorkforceKind(cohorts, observations, "treated", after)
	controlBefore := aggregateWorkforceKind(cohorts, observations, "control", before)
	controlAfter := aggregateWorkforceKind(cohorts, observations, "control", after)

	report.Summary.TreatedOwnershipBefore = treatedBefore.ownershipRate()
	report.Summary.TreatedOwnershipAfter = treatedAfter.ownershipRate()
	report.Summary.ControlOwnershipBefore = controlBefore.ownershipRate()
	report.Summary.ControlOwnershipAfter = controlAfter.ownershipRate()
	report.Summary.TreatedOwnershipDeltaPoints = round2Float(report.Summary.TreatedOwnershipAfter - report.Summary.TreatedOwnershipBefore)
	report.Summary.ControlOwnershipDeltaPoints = round2Float(report.Summary.ControlOwnershipAfter - report.Summary.ControlOwnershipBefore)
	report.Summary.OwnershipDiffInDiffPoints = round2Float(report.Summary.TreatedOwnershipDeltaPoints - report.Summary.ControlOwnershipDeltaPoints)

	report.Summary.TreatedEscalationRateBefore = treatedBefore.escalationRate()
	report.Summary.TreatedEscalationRateAfter = treatedAfter.escalationRate()
	report.Summary.ControlEscalationRateBefore = controlBefore.escalationRate()
	report.Summary.ControlEscalationRateAfter = controlAfter.escalationRate()
	report.Summary.TreatedEscalationReductionPoints = round2Float(report.Summary.TreatedEscalationRateBefore - report.Summary.TreatedEscalationRateAfter)
	report.Summary.ControlEscalationReductionPoints = round2Float(report.Summary.ControlEscalationRateBefore - report.Summary.ControlEscalationRateAfter)
	report.Summary.EscalationDiffInDiffPoints = round2Float(report.Summary.TreatedEscalationReductionPoints - report.Summary.ControlEscalationReductionPoints)

	report.Summary.TreatedDownstreamMissRateBefore = treatedBefore.downstreamMissRate()
	report.Summary.TreatedDownstreamMissRateAfter = treatedAfter.downstreamMissRate()
	report.Summary.TreatedDownstreamMissRateIncreasePoints = round2Float(report.Summary.TreatedDownstreamMissRateAfter - report.Summary.TreatedDownstreamMissRateBefore)

	report.Summary.TreatedLearningScoreBefore = treatedBefore.learningMean()
	report.Summary.TreatedLearningScoreAfter = treatedAfter.learningMean()
	report.Summary.ControlLearningScoreBefore = controlBefore.learningMean()
	report.Summary.ControlLearningScoreAfter = controlAfter.learningMean()
	report.Summary.TreatedLearningDeltaPoints = round2Float(report.Summary.TreatedLearningScoreAfter - report.Summary.TreatedLearningScoreBefore)
	report.Summary.ControlLearningDeltaPoints = round2Float(report.Summary.ControlLearningScoreAfter - report.Summary.ControlLearningScoreBefore)
	report.Summary.LearningDiffInDiffPoints = round2Float(report.Summary.TreatedLearningDeltaPoints - report.Summary.ControlLearningDeltaPoints)

	report.Summary.TreatedHeldOutDetectionBefore = treatedBefore.heldOutDetectionRate()
	report.Summary.TreatedHeldOutDetectionAfter = treatedAfter.heldOutDetectionRate()
	report.Summary.ControlHeldOutDetectionBefore = controlBefore.heldOutDetectionRate()
	report.Summary.ControlHeldOutDetectionAfter = controlAfter.heldOutDetectionRate()
	report.Summary.TreatedHeldOutDetectionDeltaPoints = round2Float(report.Summary.TreatedHeldOutDetectionAfter - report.Summary.TreatedHeldOutDetectionBefore)
	report.Summary.ControlHeldOutDetectionDeltaPoints = round2Float(report.Summary.ControlHeldOutDetectionAfter - report.Summary.ControlHeldOutDetectionBefore)
	report.Summary.HeldOutDetectionDiffInDiffPoints = round2Float(report.Summary.TreatedHeldOutDetectionDeltaPoints - report.Summary.ControlHeldOutDetectionDeltaPoints)
}

func workforceCriteriaCounterexamples(spec WorkforceImpactSpec, report WorkforceImpactReport, observations map[string]map[string][]WorkforceObservation) []WorkforceImpactCountercase {
	var counterexamples []WorkforceImpactCountercase
	criteria := spec.Criteria
	if report.Summary.Cohorts < criteria.MinCohorts {
		counterexamples = append(counterexamples, WorkforceImpactCountercase{
			ID:      "criteria.min_cohorts",
			Kind:    "underpowered_study",
			Message: fmt.Sprintf("cohorts %d below required %d", report.Summary.Cohorts, criteria.MinCohorts),
		})
	}
	if report.Summary.AutomationReferences < criteria.MinAutomationReferences {
		counterexamples = append(counterexamples, WorkforceImpactCountercase{
			ID:      "criteria.min_automation_references",
			Kind:    "missing_automation_reference",
			Message: fmt.Sprintf("automation references %d below required %d", report.Summary.AutomationReferences, criteria.MinAutomationReferences),
		})
	}
	if criteria.RequireControlCohort && report.Summary.ControlCohorts == 0 {
		counterexamples = append(counterexamples, WorkforceImpactCountercase{
			ID:      "criteria.control_cohort",
			Kind:    "missing_control_cohort",
			Message: "study requires a control cohort for difference-in-differences",
		})
	}
	if criteria.RequireTreatedCohort && report.Summary.TreatedCohorts == 0 {
		counterexamples = append(counterexamples, WorkforceImpactCountercase{
			ID:      "criteria.treated_cohort",
			Kind:    "missing_treated_cohort",
			Message: "study requires a treated cohort exposed to automation",
		})
	}
	if criteria.RequireBeforeAfterPeriods {
		for _, cohort := range sortedWorkforceCohorts(spec.Cohorts) {
			for _, period := range []string{normalizeWorkforcePeriod(spec.Protocol.BeforePeriod), normalizeWorkforcePeriod(spec.Protocol.AfterPeriod)} {
				count := len(observations[cohort.ID][period])
				if count < criteria.MinObservationsPerCohortPeriod {
					counterexamples = append(counterexamples, WorkforceImpactCountercase{
						ID:      "cohort." + stableID(cohort.ID, period) + ".observations",
						Kind:    "missing_period_observations",
						Subject: cohort.ID,
						Message: fmt.Sprintf("cohort has %d observations for period %s below required %d", count, period, criteria.MinObservationsPerCohortPeriod),
					})
				}
			}
		}
	}
	if report.Summary.MaxAttritionRateObserved > criteria.MaxAttritionRate {
		counterexamples = append(counterexamples, WorkforceImpactCountercase{
			ID:      "criteria.attrition",
			Kind:    "differential_attrition",
			Message: fmt.Sprintf("max attrition %.2f exceeds allowed %.2f", report.Summary.MaxAttritionRateObserved, criteria.MaxAttritionRate),
		})
	}
	if report.Summary.OwnershipDiffInDiffPoints < criteria.MinOwnershipDiffInDiffPoints {
		counterexamples = append(counterexamples, WorkforceImpactCountercase{
			ID:      "criteria.ownership_diff_in_diff",
			Kind:    "insufficient_ownership_shift",
			Subject: "ownership",
			Message: fmt.Sprintf("ownership diff-in-diff %.2f below required %.2f", report.Summary.OwnershipDiffInDiffPoints, criteria.MinOwnershipDiffInDiffPoints),
		})
	}
	if absFloat(report.Summary.ControlOwnershipDeltaPoints) > criteria.MaxControlOwnershipShiftPoints {
		counterexamples = append(counterexamples, WorkforceImpactCountercase{
			ID:      "criteria.ownership_control_shift",
			Kind:    "confounded_by_secular_trend",
			Subject: "ownership",
			Message: fmt.Sprintf("control ownership changed %.2f points, above allowed %.2f", report.Summary.ControlOwnershipDeltaPoints, criteria.MaxControlOwnershipShiftPoints),
		})
	}
	if report.Summary.EscalationDiffInDiffPoints < criteria.MinEscalationDiffInDiffPoints {
		counterexamples = append(counterexamples, WorkforceImpactCountercase{
			ID:      "criteria.escalation_diff_in_diff",
			Kind:    "insufficient_escalation_reduction",
			Subject: "escalation",
			Message: fmt.Sprintf("escalation diff-in-diff %.2f below required %.2f", report.Summary.EscalationDiffInDiffPoints, criteria.MinEscalationDiffInDiffPoints),
		})
	}
	if report.Summary.ControlEscalationReductionPoints > criteria.MaxControlEscalationReductionPoints {
		counterexamples = append(counterexamples, WorkforceImpactCountercase{
			ID:      "criteria.escalation_control_shift",
			Kind:    "confounded_by_secular_trend",
			Subject: "escalation",
			Message: fmt.Sprintf("control escalation reduction %.2f points, above allowed %.2f", report.Summary.ControlEscalationReductionPoints, criteria.MaxControlEscalationReductionPoints),
		})
	}
	if criteria.RequireQualityGuard && report.Summary.TreatedEscalationReductionPoints > 0 && report.Summary.TreatedDownstreamMissRateIncreasePoints > criteria.MaxDefectRateIncreasePoints {
		counterexamples = append(counterexamples, WorkforceImpactCountercase{
			ID:      "criteria.suppressed_escalation",
			Kind:    "suppressed_escalation",
			Subject: "escalation",
			Message: fmt.Sprintf("escalation dropped while downstream miss rate rose %.2f points above allowed %.2f", report.Summary.TreatedDownstreamMissRateIncreasePoints, criteria.MaxDefectRateIncreasePoints),
		})
	}
	if report.Summary.LearningDiffInDiffPoints < criteria.MinLearningDiffInDiffPoints {
		counterexamples = append(counterexamples, WorkforceImpactCountercase{
			ID:      "criteria.learning_diff_in_diff",
			Kind:    "insufficient_learning_lift",
			Subject: "learning",
			Message: fmt.Sprintf("learning diff-in-diff %.2f below required %.2f", report.Summary.LearningDiffInDiffPoints, criteria.MinLearningDiffInDiffPoints),
		})
	}
	if report.Summary.ControlLearningDeltaPoints > criteria.MaxControlLearningLiftPoints {
		counterexamples = append(counterexamples, WorkforceImpactCountercase{
			ID:      "criteria.learning_control_shift",
			Kind:    "confounded_by_secular_trend",
			Subject: "learning",
			Message: fmt.Sprintf("control learning changed %.2f points, above allowed %.2f", report.Summary.ControlLearningDeltaPoints, criteria.MaxControlLearningLiftPoints),
		})
	}
	if criteria.RequireHeldOutDetectionLift && report.Summary.HeldOutDetectionDiffInDiffPoints < criteria.MinHeldOutDetectionDiffInDiffPoints {
		counterexamples = append(counterexamples, WorkforceImpactCountercase{
			ID:      "criteria.heldout_detection_diff_in_diff",
			Kind:    "insufficient_heldout_detection_lift",
			Subject: "learning",
			Message: fmt.Sprintf("held-out detection diff-in-diff %.2f below required %.2f", report.Summary.HeldOutDetectionDiffInDiffPoints, criteria.MinHeldOutDetectionDiffInDiffPoints),
		})
		if report.Summary.LearningDiffInDiffPoints >= criteria.MinLearningDiffInDiffPoints {
			counterexamples = append(counterexamples, WorkforceImpactCountercase{
				ID:      "criteria.learning_teaching_to_test",
				Kind:    "teaching_to_test",
				Subject: "learning",
				Message: "assessment-score lift is not corroborated by held-out detection lift",
			})
		}
	}
	if report.Summary.ControlHeldOutDetectionDeltaPoints > criteria.MaxControlLearningLiftPoints {
		counterexamples = append(counterexamples, WorkforceImpactCountercase{
			ID:      "criteria.heldout_control_shift",
			Kind:    "confounded_by_secular_trend",
			Subject: "learning",
			Message: fmt.Sprintf("control held-out detection changed %.2f points, above allowed %.2f", report.Summary.ControlHeldOutDetectionDeltaPoints, criteria.MaxControlLearningLiftPoints),
		})
	}
	return counterexamples
}

func aggregateWorkforceKind(cohorts map[string]WorkforceCohort, observations map[string]map[string][]WorkforceObservation, kind, period string) workforceAggregate {
	agg := workforceAggregate{participants: map[string]struct{}{}}
	for _, cohortID := range sortedWorkforceCohortIDs(cohorts) {
		cohort := cohorts[cohortID]
		if normalizeToken(cohort.Kind) != kind {
			continue
		}
		agg.add(observations[cohortID][period])
	}
	return agg
}

func aggregateWorkforceObservations(observations []WorkforceObservation) workforceAggregate {
	agg := workforceAggregate{participants: map[string]struct{}{}}
	agg.add(observations)
	return agg
}

func (agg *workforceAggregate) add(observations []WorkforceObservation) {
	if agg.participants == nil {
		agg.participants = map[string]struct{}{}
	}
	for _, observation := range observations {
		agg.observations++
		if observation.OwnedByPrimaryTeam {
			agg.owned++
		}
		if observation.Escalations > 0 {
			agg.escalated++
		}
		agg.escalations += observation.Escalations
		if observation.DownstreamMisses > 0 {
			agg.downstreamMisses++
		}
		agg.learningScore += observation.LearningAssessmentScore
		agg.heldOutDetections += observation.HeldOutDetections
		agg.heldOutTotal += observation.HeldOutOpportunities
		agg.participants[observation.ParticipantID] = struct{}{}
	}
}

func (agg workforceAggregate) ownershipRate() float64 {
	return ratePercent(agg.owned, agg.observations)
}

func (agg workforceAggregate) escalationRate() float64 {
	return ratePercent(agg.escalated, agg.observations)
}

func (agg workforceAggregate) downstreamMissRate() float64 {
	return ratePercent(agg.downstreamMisses, agg.observations)
}

func (agg workforceAggregate) learningMean() float64 {
	if agg.observations == 0 {
		return 0
	}
	return round2Float(agg.learningScore / float64(agg.observations))
}

func (agg workforceAggregate) heldOutDetectionRate() float64 {
	return ratePercent(agg.heldOutDetections, agg.heldOutTotal)
}

func participantAttritionRate(before, after map[string]struct{}) float64 {
	if len(before) == 0 {
		return 0
	}
	retained := 0
	for participant := range before {
		if _, ok := after[participant]; ok {
			retained++
		}
	}
	return round2Float(1 - float64(retained)/float64(len(before)))
}

func sortedWorkforceAutomations(automations []WorkforceAutomation) []WorkforceAutomation {
	out := append([]WorkforceAutomation(nil), automations...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func sortedWorkforceCohorts(cohorts []WorkforceCohort) []WorkforceCohort {
	out := append([]WorkforceCohort(nil), cohorts...)
	sort.SliceStable(out, func(i, j int) bool {
		left := normalizeToken(out[i].Kind) + "\x00" + out[i].ID
		right := normalizeToken(out[j].Kind) + "\x00" + out[j].ID
		return left < right
	})
	return out
}

func sortedWorkforceCohortIDs(cohorts map[string]WorkforceCohort) []string {
	keys := make([]string, 0, len(cohorts))
	for key := range cohorts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedWorkforceObservations(observations []WorkforceObservation) []WorkforceObservation {
	out := append([]WorkforceObservation(nil), observations...)
	sort.SliceStable(out, func(i, j int) bool {
		left := fmt.Sprintf("%s\x00%s\x00%s\x00%s", normalizeWorkforcePeriod(out[i].Period), out[i].CohortID, out[i].ParticipantID, out[i].ReviewID)
		right := fmt.Sprintf("%s\x00%s\x00%s\x00%s", normalizeWorkforcePeriod(out[j].Period), out[j].CohortID, out[j].ParticipantID, out[j].ReviewID)
		return left < right
	})
	return out
}

func privacyPreservingID(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || strings.ContainsAny(value, "@ .") {
		return false
	}
	if normalizeToken(value) != value {
		return false
	}
	blocked := map[string]struct{}{
		"alice": {}, "bob": {}, "bruno": {}, "casey": {}, "devon": {},
	}
	_, blockedName := blocked[value]
	return !blockedName
}

func sortWorkforceCounterexamples(counterexamples []WorkforceImpactCountercase) {
	sort.SliceStable(counterexamples, func(i, j int) bool { return counterexamples[i].ID < counterexamples[j].ID })
}

func workforceImpactReportHash(report WorkforceImpactReport) string {
	report.Hash = ""
	return canonical.Hash(report)
}

func absFloat(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}

func maxFloat(left, right float64) float64 {
	if right > left {
		return right
	}
	return left
}
