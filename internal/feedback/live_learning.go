package feedback

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/thehalleyyoung/patchline/internal/canonical"
)

const (
	OnlineEvaluationSpecVersion      = "patchline.safe-online-evaluation/v1"
	OnlineEvaluationReportVersion    = "patchline.safe-online-evaluation-report/v1"
	ActiveLearningSpecVersion        = "patchline.adopter-active-learning/v1"
	ActiveLearningReportVersion      = "patchline.adopter-active-learning-report/v1"
	PolicyFreezeSpecVersion          = "patchline.policy-freeze/v1"
	PolicyFreezeReportVersion        = "patchline.policy-freeze-report/v1"
	CalibrationMonitorSpecVersion    = "patchline.live-calibration-monitor/v1"
	CalibrationMonitorReportVersion  = "patchline.live-calibration-monitor-report/v1"
	RetentionLifecycleSpecVersion    = "patchline.feedback-retention-lifecycle/v1"
	RetentionLifecycleReportVersion  = "patchline.feedback-retention-lifecycle-report/v1"
	TrustRegressionSpecVersion       = "patchline.human-trust-regression/v1"
	TrustRegressionReportVersion     = "patchline.human-trust-regression-report/v1"
	MethodologySpecVersion           = "patchline.live-learning-methodology/v1"
	MethodologyReportVersion         = "patchline.live-learning-methodology-report/v1"
	DetectorDeprecationSpecVersion   = "patchline.detector-deprecation/v1"
	DetectorDeprecationReportVersion = "patchline.detector-deprecation-report/v1"
	basisPointsScale                 = 10000
	dateLayout                       = "2006-01-02"
)

type LiveLearningPrivacy struct {
	SourceFree             bool     `json:"source_free"`
	RawEvidenceFree        bool     `json:"raw_values_free"`
	IdentifierFree         bool     `json:"identifier_free"`
	SaltEmitted            bool     `json:"salt_emitted"`
	IndividualExamplesFree bool     `json:"individual_examples_free"`
	LocalOnlyArtifacts     []string `json:"local_only_artifacts,omitempty"`
	SuppressedFields       []string `json:"suppressed_fields,omitempty"`
}

type OnlineEvaluationSpec struct {
	Version            string                    `json:"version"`
	Claim              string                    `json:"claim"`
	CandidateDetectors []OnlineDetectorCandidate `json:"candidate_detectors"`
}

type OnlineDetectorCandidate struct {
	Detector                string `json:"detector"`
	Release                 string `json:"release"`
	MinEvidence             int    `json:"min_evidence"`
	MinPrecisionBP          int    `json:"min_precision_bp"`
	MinRecallBP             int    `json:"min_recall_bp"`
	MaxAverageBurdenMinutes int    `json:"max_average_burden_minutes"`
	RequiresHumanGate       bool   `json:"requires_human_gate"`
}

type OnlineEvaluationReport struct {
	Version               string                  `json:"version"`
	OK                    bool                    `json:"ok"`
	FeedbackHash          string                  `json:"feedback_hash"`
	SpecHash              string                  `json:"spec_hash"`
	Hash                  string                  `json:"hash"`
	EvidenceBasis         string                  `json:"evidence_basis"`
	PolicyMutationAllowed bool                    `json:"policy_mutation_allowed"`
	Privacy               LiveLearningPrivacy     `json:"privacy"`
	Summary               OnlineEvaluationSummary `json:"summary"`
	Detectors             []OnlineDetectorReport  `json:"detectors"`
	Warnings              []string                `json:"warnings,omitempty"`
}

type OnlineEvaluationSummary struct {
	DetectorsEvaluated       int `json:"detectors_evaluated"`
	PromotionCandidates      int `json:"promotion_candidates"`
	ShadowOnly               int `json:"shadow_only"`
	GatesPassed              int `json:"gates_passed"`
	GatesFailed              int `json:"gates_failed"`
	PublishedFeedbackRecords int `json:"published_feedback_records"`
}

type OnlineDetectorReport struct {
	Detector string                `json:"detector"`
	Release  string                `json:"release"`
	Status   string                `json:"status"`
	Metrics  OnlineDetectorMetrics `json:"metrics"`
	Gates    []OnlineGateResult    `json:"gates"`
}

type OnlineDetectorMetrics struct {
	PublishedCount       int `json:"published_count"`
	Confirmed            int `json:"confirmed"`
	FalsePositive        int `json:"false_positive"`
	Missed               int `json:"missed"`
	Uncertain            int `json:"uncertain"`
	TotalBurdenMinutes   int `json:"total_burden_minutes"`
	AverageBurdenMinutes int `json:"average_burden_minutes"`
	PrecisionBP          int `json:"precision_bp"`
	RecallBP             int `json:"recall_bp"`
}

type OnlineGateResult struct {
	Name          string `json:"name"`
	Passed        bool   `json:"passed"`
	Observed      int    `json:"observed"`
	Required      int    `json:"required"`
	Comparator    string `json:"comparator"`
	FailureReason string `json:"failure_reason,omitempty"`
}

type ActiveLearningSpec struct {
	Version          string               `json:"version"`
	Claim            string               `json:"claim"`
	MinUncertaintyBP int                  `json:"min_uncertainty_bp"`
	MinInfoGainBP    int                  `json:"min_information_gain_bp"`
	MaxQueueSize     int                  `json:"max_queue_size"`
	Cases            []ActiveLearningCase `json:"cases"`
}

type ActiveLearningCase struct {
	LocalCaseID               string `json:"local_case_id"`
	Detector                  string `json:"detector"`
	Release                   string `json:"release"`
	ConfidenceBP              int    `json:"confidence_bp"`
	UncertaintyBP             int    `json:"uncertainty_bp"`
	ExpectedInformationGainBP int    `json:"expected_information_gain_bp"`
	EstimatedBurdenMinutes    int    `json:"estimated_burden_minutes"`
	AlreadyLabeled            bool   `json:"already_labeled"`
}

type ActiveLearningReport struct {
	Version    string                   `json:"version"`
	OK         bool                     `json:"ok"`
	Shareable  bool                     `json:"shareable"`
	SpecHash   string                   `json:"spec_hash"`
	Hash       string                   `json:"hash"`
	Privacy    LiveLearningPrivacy      `json:"privacy"`
	Summary    ActiveLearningSummary    `json:"summary"`
	LocalQueue ActiveLearningLocalQueue `json:"local_queue"`
	Aggregate  ActiveLearningAggregate  `json:"shareable_aggregate"`
}

type ActiveLearningSummary struct {
	InputCases          int `json:"input_cases"`
	EligibleCases       int `json:"eligible_cases"`
	QueuedCases         int `json:"queued_cases"`
	AlreadyLabeledCases int `json:"already_labeled_cases"`
	BelowThresholdCases int `json:"below_threshold_cases"`
	MaxQueueSize        int `json:"max_queue_size"`
}

type ActiveLearningLocalQueue struct {
	Shareable bool                      `json:"shareable"`
	Items     []ActiveLearningQueueItem `json:"items"`
}

type ActiveLearningQueueItem struct {
	Rank                      int    `json:"rank"`
	LocalCaseID               string `json:"local_case_id"`
	Detector                  string `json:"detector"`
	Release                   string `json:"release"`
	ConfidenceBP              int    `json:"confidence_bp"`
	UncertaintyBP             int    `json:"uncertainty_bp"`
	ExpectedInformationGainBP int    `json:"expected_information_gain_bp"`
	EstimatedBurdenMinutes    int    `json:"estimated_burden_minutes"`
}

type ActiveLearningAggregate struct {
	Version                       string                            `json:"version"`
	Shareable                     bool                              `json:"shareable"`
	SourceFree                    bool                              `json:"source_free"`
	QueuedCases                   int                               `json:"queued_cases"`
	MeanUncertaintyBP             int                               `json:"mean_uncertainty_bp"`
	MeanExpectedInformationGainBP int                               `json:"mean_expected_information_gain_bp"`
	MeanBurdenMinutes             int                               `json:"mean_burden_minutes"`
	ByDetector                    []ActiveLearningDetectorAggregate `json:"by_detector"`
	Hash                          string                            `json:"hash"`
}

type ActiveLearningDetectorAggregate struct {
	Detector                      string `json:"detector"`
	Release                       string `json:"release"`
	QueuedCases                   int    `json:"queued_cases"`
	MeanUncertaintyBP             int    `json:"mean_uncertainty_bp"`
	MeanExpectedInformationGainBP int    `json:"mean_expected_information_gain_bp"`
}

type PolicyFreezeSpec struct {
	Version         string                     `json:"version"`
	Claim           string                     `json:"claim"`
	AsOfDate        string                     `json:"as_of_date"`
	AuditedReleases []AuditedDetectorRelease   `json:"audited_releases"`
	Organizations   []PolicyFreezeOrganization `json:"organizations"`
}

type AuditedDetectorRelease struct {
	Release               string `json:"release"`
	AuditHash             string `json:"audit_hash"`
	ApprovedForHighStakes bool   `json:"approved_for_high_stakes"`
}

type PolicyFreezeOrganization struct {
	Organization    string `json:"organization"`
	HighStakes      bool   `json:"high_stakes"`
	IncidentActive  bool   `json:"incident_active"`
	IncidentID      string `json:"incident_id,omitempty"`
	CurrentRelease  string `json:"current_release"`
	ProposedRelease string `json:"proposed_release"`
}

type PolicyFreezeReport struct {
	Version   string                 `json:"version"`
	OK        bool                   `json:"ok"`
	SpecHash  string                 `json:"spec_hash"`
	Hash      string                 `json:"hash"`
	AsOfDate  string                 `json:"as_of_date"`
	Privacy   LiveLearningPrivacy    `json:"privacy"`
	Summary   PolicyFreezeSummary    `json:"summary"`
	Decisions []PolicyFreezeDecision `json:"decisions"`
}

type PolicyFreezeSummary struct {
	OrganizationsEvaluated int `json:"organizations_evaluated"`
	PinnedOrganizations    int `json:"pinned_organizations"`
	AllowedUpdates         int `json:"allowed_updates"`
	BlockedMissingAudit    int `json:"blocked_missing_audit"`
}

type PolicyFreezeDecision struct {
	Organization        string `json:"organization"`
	HighStakes          bool   `json:"high_stakes"`
	IncidentActive      bool   `json:"incident_active"`
	PinnedRelease       string `json:"pinned_release"`
	ProposedRelease     string `json:"proposed_release"`
	PolicyChangeAllowed bool   `json:"policy_change_allowed"`
	Reason              string `json:"reason"`
}

type CalibrationMonitorSpec struct {
	Version                  string   `json:"version"`
	Claim                    string   `json:"claim"`
	PreRegisteredToleranceBP int      `json:"pre_registered_tolerance_bp"`
	MinEvidence              int      `json:"min_evidence"`
	AlertChannels            []string `json:"alert_channels"`
}

type CalibrationMonitorReport struct {
	Version       string                    `json:"version"`
	OK            bool                      `json:"ok"`
	FeedbackHash  string                    `json:"feedback_hash"`
	SpecHash      string                    `json:"spec_hash"`
	Hash          string                    `json:"hash"`
	EvidenceBasis string                    `json:"evidence_basis"`
	Privacy       LiveLearningPrivacy       `json:"privacy"`
	Summary       CalibrationMonitorSummary `json:"summary"`
	Deciles       []CalibrationDecile       `json:"deciles"`
	Alerts        []CalibrationAlert        `json:"alerts"`
}

type CalibrationMonitorSummary struct {
	DecilesEvaluated int `json:"deciles_evaluated"`
	Alerts           int `json:"alerts"`
	MinEvidence      int `json:"min_evidence"`
	ToleranceBP      int `json:"tolerance_bp"`
}

type CalibrationDecile struct {
	Detector                string `json:"detector"`
	Release                 string `json:"release"`
	ConfidenceDecile        string `json:"confidence_decile"`
	PublishedCount          int    `json:"published_count"`
	ExpectedConfirmedRateBP int    `json:"expected_confirmed_rate_bp"`
	ObservedConfirmedRateBP int    `json:"observed_confirmed_rate_bp"`
	DriftBP                 int    `json:"drift_bp"`
}

type CalibrationAlert struct {
	Detector         string `json:"detector"`
	Release          string `json:"release"`
	ConfidenceDecile string `json:"confidence_decile"`
	DriftBP          int    `json:"drift_bp"`
	ToleranceBP      int    `json:"tolerance_bp"`
	Severity         string `json:"severity"`
}

type RetentionLifecycleSpec struct {
	Version   string              `json:"version"`
	Claim     string              `json:"claim"`
	AsOfDate  string              `json:"as_of_date"`
	Policies  []RetentionPolicy   `json:"policies"`
	Artifacts []RetentionArtifact `json:"artifacts"`
}

type RetentionPolicy struct {
	Class                  string `json:"class"`
	RawRetentionDays       int    `json:"raw_retention_days"`
	AnonymizeAfterDays     int    `json:"anonymize_after_days"`
	AggregateRetentionDays int    `json:"aggregate_retention_days"`
}

type RetentionArtifact struct {
	ArtifactID            string `json:"artifact_id"`
	Class                 string `json:"class"`
	CreatedAt             string `json:"created_at"`
	ContainsRawEvidence   bool   `json:"contains_raw_evidence"`
	ContainsLocalExamples bool   `json:"contains_local_examples"`
	Aggregated            bool   `json:"aggregated"`
	Anonymized            bool   `json:"anonymized"`
	ObservedAction        string `json:"observed_action"`
}

type RetentionLifecycleReport struct {
	Version   string                    `json:"version"`
	OK        bool                      `json:"ok"`
	SpecHash  string                    `json:"spec_hash"`
	Hash      string                    `json:"hash"`
	AsOfDate  string                    `json:"as_of_date"`
	Privacy   LiveLearningPrivacy       `json:"privacy"`
	Summary   RetentionLifecycleSummary `json:"summary"`
	Artifacts []RetentionDecision       `json:"artifacts"`
}

type RetentionLifecycleSummary struct {
	ArtifactsEvaluated int `json:"artifacts_evaluated"`
	CompliantArtifacts int `json:"compliant_artifacts"`
	Violations         int `json:"violations"`
	DeleteRequired     int `json:"delete_required"`
	AnonymizeRequired  int `json:"anonymize_required"`
	AggregateRetained  int `json:"aggregate_retained"`
}

type RetentionDecision struct {
	ArtifactID     string `json:"artifact_id"`
	Class          string `json:"class"`
	AgeDays        int    `json:"age_days"`
	ExpectedAction string `json:"expected_action"`
	ObservedAction string `json:"observed_action"`
	Compliant      bool   `json:"compliant"`
	Reason         string `json:"reason"`
}

type TrustRegressionSpec struct {
	Version    string                   `json:"version"`
	Claim      string                   `json:"claim"`
	Baseline   TrustMetrics             `json:"baseline"`
	Current    TrustMetrics             `json:"current"`
	Tolerances TrustRegressionTolerance `json:"tolerances"`
}

type TrustMetrics struct {
	Release                    string `json:"release"`
	ExplanationFaithfulnessBP  int    `json:"explanation_faithfulness_bp"`
	EvidenceCitationCoverageBP int    `json:"evidence_citation_coverage_bp"`
	UncertaintyDisclosureBP    int    `json:"uncertainty_disclosure_bp"`
	OverrelianceRateBP         int    `json:"overreliance_rate_bp"`
	MeanReviewBurdenMinutes    int    `json:"mean_review_burden_minutes"`
}

type TrustRegressionTolerance struct {
	MaxFaithfulnessDropBP     int `json:"max_faithfulness_drop_bp"`
	MaxCitationCoverageDropBP int `json:"max_citation_coverage_drop_bp"`
	MaxUncertaintyDropBP      int `json:"max_uncertainty_drop_bp"`
	MaxOverrelianceIncreaseBP int `json:"max_overreliance_increase_bp"`
	MaxBurdenIncreaseMinutes  int `json:"max_burden_increase_minutes"`
}

type TrustRegressionReport struct {
	Version  string                 `json:"version"`
	OK       bool                   `json:"ok"`
	SpecHash string                 `json:"spec_hash"`
	Hash     string                 `json:"hash"`
	Privacy  LiveLearningPrivacy    `json:"privacy"`
	Summary  TrustRegressionSummary `json:"summary"`
	Checks   []TrustRegressionCheck `json:"checks"`
}

type TrustRegressionSummary struct {
	Checks       int `json:"checks"`
	PassedChecks int `json:"passed_checks"`
	FailedChecks int `json:"failed_checks"`
}

type TrustRegressionCheck struct {
	Name       string `json:"name"`
	Passed     bool   `json:"passed"`
	Delta      int    `json:"delta"`
	Tolerance  int    `json:"tolerance"`
	Comparator string `json:"comparator"`
}

type MethodologySpec struct {
	Version      string                    `json:"version"`
	Claim        string                    `json:"claim"`
	Experiments  []MethodologyExperiment   `json:"experiments"`
	GateEvidence []MethodologyGateEvidence `json:"gate_evidence"`
}

type MethodologyExperiment struct {
	Name                       string   `json:"name"`
	Population                 string   `json:"population"`
	BaselineRecallBP           int      `json:"baseline_recall_bp"`
	LiveLearningRecallBP       int      `json:"live_learning_recall_bp"`
	BaselineOverrelianceBP     int      `json:"baseline_overreliance_bp"`
	LiveLearningOverrelianceBP int      `json:"live_learning_overreliance_bp"`
	BaselineBurdenMinutes      int      `json:"baseline_burden_minutes"`
	LiveLearningBurdenMinutes  int      `json:"live_learning_burden_minutes"`
	Evidence                   []string `json:"evidence"`
}

type MethodologyGateEvidence struct {
	Gate                string `json:"gate"`
	ReportHash          string `json:"report_hash"`
	ReproductionCommand string `json:"reproduction_command"`
}

type MethodologyReport struct {
	Version      string                    `json:"version"`
	OK           bool                      `json:"ok"`
	SpecHash     string                    `json:"spec_hash"`
	Hash         string                    `json:"hash"`
	Privacy      LiveLearningPrivacy       `json:"privacy"`
	Summary      MethodologySummary        `json:"summary"`
	Experiments  []MethodologyResult       `json:"experiments"`
	GateEvidence []MethodologyGateEvidence `json:"gate_evidence"`
}

type MethodologySummary struct {
	Experiments              int `json:"experiments"`
	RecallImproved           int `json:"recall_improved"`
	OverrelianceNotIncreased int `json:"overreliance_not_increased"`
	BurdenNotIncreased       int `json:"burden_not_increased"`
	LinkedGateEvidence       int `json:"linked_gate_evidence"`
}

type MethodologyResult struct {
	Name                     string   `json:"name"`
	Population               string   `json:"population"`
	RecallDeltaBP            int      `json:"recall_delta_bp"`
	OverrelianceDeltaBP      int      `json:"overreliance_delta_bp"`
	BurdenDeltaMinutes       int      `json:"burden_delta_minutes"`
	RecallImproved           bool     `json:"recall_improved"`
	OverrelianceNotIncreased bool     `json:"overreliance_not_increased"`
	BurdenNotIncreased       bool     `json:"burden_not_increased"`
	Evidence                 []string `json:"evidence"`
}

type DetectorDeprecationSpec struct {
	Version                 string                      `json:"version"`
	Claim                   string                      `json:"claim"`
	AsOfDate                string                      `json:"as_of_date"`
	MinEvidence             int                         `json:"min_evidence"`
	MinPrecisionBP          int                         `json:"min_precision_bp"`
	MaxAverageBurdenMinutes int                         `json:"max_average_burden_minutes"`
	MinNoticeDays           int                         `json:"min_notice_days"`
	MinReviewerRoles        int                         `json:"min_reviewer_roles"`
	MinAppealWindowDays     int                         `json:"min_appeal_window_days"`
	RequiredPublicChannels  []string                    `json:"required_public_channels"`
	Detectors               []DetectorDeprecationTarget `json:"detectors"`
}

type DetectorDeprecationTarget struct {
	Detector            string   `json:"detector"`
	Release             string   `json:"release"`
	Owner               string   `json:"owner"`
	PublicNoticeID      string   `json:"public_notice_id"`
	NoticeOpenedAt      string   `json:"notice_opened_at"`
	PublicChannels      []string `json:"public_channels"`
	ReviewerRoles       []string `json:"reviewer_roles"`
	ReplacementDetector string   `json:"replacement_detector"`
	MigrationGuide      string   `json:"migration_guide"`
	AppealWindowDays    int      `json:"appeal_window_days"`
}

type DetectorDeprecationReport struct {
	Version       string                        `json:"version"`
	OK            bool                          `json:"ok"`
	FeedbackHash  string                        `json:"feedback_hash"`
	SpecHash      string                        `json:"spec_hash"`
	Hash          string                        `json:"hash"`
	AsOfDate      string                        `json:"as_of_date"`
	EvidenceBasis string                        `json:"evidence_basis"`
	Privacy       LiveLearningPrivacy           `json:"privacy"`
	Summary       DetectorDeprecationSummary    `json:"summary"`
	Detectors     []DetectorDeprecationDecision `json:"detectors"`
}

type DetectorDeprecationSummary struct {
	DetectorsEvaluated            int `json:"detectors_evaluated"`
	Retained                      int `json:"retained"`
	MonitoredInsufficientEvidence int `json:"monitored_insufficient_evidence"`
	ThresholdFailures             int `json:"threshold_failures"`
	DeprecationNotices            int `json:"deprecation_notices"`
	ReadyToDeprecate              int `json:"ready_to_deprecate"`
	ProcessViolations             int `json:"process_violations"`
}

type DetectorDeprecationDecision struct {
	Detector              string                `json:"detector"`
	Release               string                `json:"release"`
	Owner                 string                `json:"owner,omitempty"`
	Status                string                `json:"status"`
	PublicNoticeID        string                `json:"public_notice_id,omitempty"`
	NoticeAgeDays         int                   `json:"notice_age_days,omitempty"`
	ReplacementDetector   string                `json:"replacement_detector,omitempty"`
	MigrationGuide        string                `json:"migration_guide,omitempty"`
	Metrics               OnlineDetectorMetrics `json:"metrics"`
	ThresholdGates        []OnlineGateResult    `json:"threshold_gates"`
	TransparencyGates     []OnlineGateResult    `json:"transparency_gates,omitempty"`
	ProcessFailures       []string              `json:"process_failures,omitempty"`
	MissingPublicChannels []string              `json:"missing_public_channels,omitempty"`
}

func ReadOnlineEvaluationSpec(reader io.Reader) (OnlineEvaluationSpec, error) {
	var spec OnlineEvaluationSpec
	if err := readLiveLearningSpec(reader, &spec); err != nil {
		return OnlineEvaluationSpec{}, err
	}
	if spec.Version != OnlineEvaluationSpecVersion {
		return OnlineEvaluationSpec{}, fmt.Errorf("online evaluation spec version must be %s", OnlineEvaluationSpecVersion)
	}
	if strings.TrimSpace(spec.Claim) == "" {
		return OnlineEvaluationSpec{}, errors.New("online evaluation spec claim is required")
	}
	if len(spec.CandidateDetectors) == 0 {
		return OnlineEvaluationSpec{}, errors.New("online evaluation spec must include candidate detectors")
	}
	seen := map[string]struct{}{}
	for _, candidate := range spec.CandidateDetectors {
		key := candidate.Detector + "|" + candidate.Release
		if _, ok := seen[key]; ok {
			return OnlineEvaluationSpec{}, fmt.Errorf("duplicate online evaluation candidate %s", key)
		}
		seen[key] = struct{}{}
		if err := validateDetectorRelease(candidate.Detector, candidate.Release); err != nil {
			return OnlineEvaluationSpec{}, err
		}
		if candidate.MinEvidence < MinKFloor {
			return OnlineEvaluationSpec{}, fmt.Errorf("candidate %s must require at least %d examples", candidate.Detector, MinKFloor)
		}
		if err := validateBasisPoints("min_precision_bp", candidate.MinPrecisionBP); err != nil {
			return OnlineEvaluationSpec{}, err
		}
		if err := validateBasisPoints("min_recall_bp", candidate.MinRecallBP); err != nil {
			return OnlineEvaluationSpec{}, err
		}
		if candidate.MaxAverageBurdenMinutes <= 0 {
			return OnlineEvaluationSpec{}, fmt.Errorf("candidate %s must set positive max_average_burden_minutes", candidate.Detector)
		}
	}
	return spec, nil
}

func ComputeOnlineEvaluation(report Report, spec OnlineEvaluationSpec) (OnlineEvaluationReport, error) {
	if report.Version != ReportVersion {
		return OnlineEvaluationReport{}, fmt.Errorf("live feedback report version must be %s", ReportVersion)
	}
	if err := ValidateOnlineEvaluationSpec(spec); err != nil {
		return OnlineEvaluationReport{}, err
	}
	out := OnlineEvaluationReport{
		Version:               OnlineEvaluationReportVersion,
		OK:                    true,
		FeedbackHash:          canonical.Hash(report),
		SpecHash:              canonical.Hash(spec),
		EvidenceBasis:         "published_k_anonymous_groups_only",
		PolicyMutationAllowed: false,
		Privacy:               defaultShareablePrivacy(),
	}
	candidates := append([]OnlineDetectorCandidate(nil), spec.CandidateDetectors...)
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Detector != candidates[j].Detector {
			return candidates[i].Detector < candidates[j].Detector
		}
		return candidates[i].Release < candidates[j].Release
	})
	for _, candidate := range candidates {
		detectorReport := evaluateOnlineCandidate(report, candidate)
		out.Detectors = append(out.Detectors, detectorReport)
		out.Summary.DetectorsEvaluated++
		out.Summary.PublishedFeedbackRecords += detectorReport.Metrics.PublishedCount
		if detectorReport.Status == "candidate_ready_for_gated_review" {
			out.Summary.PromotionCandidates++
		} else {
			out.Summary.ShadowOnly++
		}
		for _, gate := range detectorReport.Gates {
			if gate.Passed {
				out.Summary.GatesPassed++
			} else {
				out.Summary.GatesFailed++
			}
		}
	}
	out.Hash = hashLiveLearning(out)
	return out, nil
}

func ValidateOnlineEvaluationSpec(spec OnlineEvaluationSpec) error {
	if spec.Version != OnlineEvaluationSpecVersion {
		return fmt.Errorf("online evaluation spec version must be %s", OnlineEvaluationSpecVersion)
	}
	if len(spec.CandidateDetectors) == 0 {
		return errors.New("online evaluation spec must include candidate detectors")
	}
	for _, candidate := range spec.CandidateDetectors {
		if err := validateDetectorRelease(candidate.Detector, candidate.Release); err != nil {
			return err
		}
		if candidate.MinEvidence < MinKFloor {
			return fmt.Errorf("candidate %s must require at least %d examples", candidate.Detector, MinKFloor)
		}
		if err := validateBasisPoints("min_precision_bp", candidate.MinPrecisionBP); err != nil {
			return err
		}
		if err := validateBasisPoints("min_recall_bp", candidate.MinRecallBP); err != nil {
			return err
		}
		if candidate.MaxAverageBurdenMinutes <= 0 {
			return fmt.Errorf("candidate %s must set positive max_average_burden_minutes", candidate.Detector)
		}
	}
	return nil
}

func ReadActiveLearningSpec(reader io.Reader) (ActiveLearningSpec, error) {
	var spec ActiveLearningSpec
	if err := readLiveLearningSpec(reader, &spec); err != nil {
		return ActiveLearningSpec{}, err
	}
	if spec.Version != ActiveLearningSpecVersion {
		return ActiveLearningSpec{}, fmt.Errorf("active learning spec version must be %s", ActiveLearningSpecVersion)
	}
	if strings.TrimSpace(spec.Claim) == "" {
		return ActiveLearningSpec{}, errors.New("active learning spec claim is required")
	}
	if spec.MaxQueueSize <= 0 {
		return ActiveLearningSpec{}, errors.New("active learning max_queue_size must be positive")
	}
	if err := validateBasisPoints("min_uncertainty_bp", spec.MinUncertaintyBP); err != nil {
		return ActiveLearningSpec{}, err
	}
	if err := validateBasisPoints("min_information_gain_bp", spec.MinInfoGainBP); err != nil {
		return ActiveLearningSpec{}, err
	}
	seen := map[string]struct{}{}
	for _, item := range spec.Cases {
		if item.LocalCaseID == "" || !safeIdentifier(item.LocalCaseID) || sourceLikeValue(item.LocalCaseID) {
			return ActiveLearningSpec{}, errors.New("active learning local_case_id must be an opaque source-free identifier")
		}
		if _, ok := seen[item.LocalCaseID]; ok {
			return ActiveLearningSpec{}, fmt.Errorf("duplicate active learning case %s", item.LocalCaseID)
		}
		seen[item.LocalCaseID] = struct{}{}
		if err := validateDetectorRelease(item.Detector, item.Release); err != nil {
			return ActiveLearningSpec{}, err
		}
		for name, value := range map[string]int{
			"confidence_bp":                item.ConfidenceBP,
			"uncertainty_bp":               item.UncertaintyBP,
			"expected_information_gain_bp": item.ExpectedInformationGainBP,
		} {
			if err := validateBasisPoints(name, value); err != nil {
				return ActiveLearningSpec{}, err
			}
		}
		if item.EstimatedBurdenMinutes < 0 || item.EstimatedBurdenMinutes > 480 {
			return ActiveLearningSpec{}, errors.New("active learning estimated burden must be between 0 and 480 minutes")
		}
	}
	return spec, nil
}

func ComputeActiveLearningQueue(spec ActiveLearningSpec) (ActiveLearningReport, error) {
	if spec.Version != ActiveLearningSpecVersion {
		return ActiveLearningReport{}, fmt.Errorf("active learning spec version must be %s", ActiveLearningSpecVersion)
	}
	out := ActiveLearningReport{
		Version:   ActiveLearningReportVersion,
		OK:        true,
		Shareable: false,
		SpecHash:  canonical.Hash(spec),
		Privacy: LiveLearningPrivacy{
			SourceFree:             true,
			RawEvidenceFree:        true,
			IdentifierFree:         false,
			SaltEmitted:            false,
			IndividualExamplesFree: false,
			LocalOnlyArtifacts:     []string{"active-learning-local-queue.json"},
			SuppressedFields:       []string{"source code", "diffs", "file paths", "finding IDs", "evidence hashes", "adopter IDs", "salts"},
		},
		LocalQueue: ActiveLearningLocalQueue{Shareable: false},
	}
	out.Summary.InputCases = len(spec.Cases)
	var eligible []ActiveLearningCase
	for _, item := range spec.Cases {
		if item.AlreadyLabeled {
			out.Summary.AlreadyLabeledCases++
			continue
		}
		if item.UncertaintyBP < spec.MinUncertaintyBP || item.ExpectedInformationGainBP < spec.MinInfoGainBP {
			out.Summary.BelowThresholdCases++
			continue
		}
		eligible = append(eligible, item)
	}
	sort.Slice(eligible, func(i, j int) bool {
		if eligible[i].UncertaintyBP != eligible[j].UncertaintyBP {
			return eligible[i].UncertaintyBP > eligible[j].UncertaintyBP
		}
		if eligible[i].ExpectedInformationGainBP != eligible[j].ExpectedInformationGainBP {
			return eligible[i].ExpectedInformationGainBP > eligible[j].ExpectedInformationGainBP
		}
		if eligible[i].EstimatedBurdenMinutes != eligible[j].EstimatedBurdenMinutes {
			return eligible[i].EstimatedBurdenMinutes < eligible[j].EstimatedBurdenMinutes
		}
		return eligible[i].LocalCaseID < eligible[j].LocalCaseID
	})
	out.Summary.EligibleCases = len(eligible)
	out.Summary.MaxQueueSize = spec.MaxQueueSize
	if len(eligible) > spec.MaxQueueSize {
		eligible = eligible[:spec.MaxQueueSize]
	}
	for index, item := range eligible {
		out.LocalQueue.Items = append(out.LocalQueue.Items, ActiveLearningQueueItem{
			Rank:                      index + 1,
			LocalCaseID:               item.LocalCaseID,
			Detector:                  item.Detector,
			Release:                   item.Release,
			ConfidenceBP:              item.ConfidenceBP,
			UncertaintyBP:             item.UncertaintyBP,
			ExpectedInformationGainBP: item.ExpectedInformationGainBP,
			EstimatedBurdenMinutes:    item.EstimatedBurdenMinutes,
		})
	}
	out.Summary.QueuedCases = len(out.LocalQueue.Items)
	out.Aggregate = buildActiveLearningAggregate(out.LocalQueue.Items)
	out.Hash = hashLiveLearning(out)
	return out, nil
}

func ReadPolicyFreezeSpec(reader io.Reader) (PolicyFreezeSpec, error) {
	var spec PolicyFreezeSpec
	if err := readLiveLearningSpec(reader, &spec); err != nil {
		return PolicyFreezeSpec{}, err
	}
	if spec.Version != PolicyFreezeSpecVersion {
		return PolicyFreezeSpec{}, fmt.Errorf("policy freeze spec version must be %s", PolicyFreezeSpecVersion)
	}
	if _, err := parseSpecDate(spec.AsOfDate); err != nil {
		return PolicyFreezeSpec{}, err
	}
	if len(spec.AuditedReleases) == 0 || len(spec.Organizations) == 0 {
		return PolicyFreezeSpec{}, errors.New("policy freeze spec requires audited releases and organizations")
	}
	for _, release := range spec.AuditedReleases {
		if release.Release == "" || !safeIdentifier(release.Release) || sourceLikeValue(release.Release) {
			return PolicyFreezeSpec{}, errors.New("audited release must be a source-free identifier")
		}
		if release.AuditHash == "" || !safeIdentifier(release.AuditHash) || sourceLikeValue(release.AuditHash) {
			return PolicyFreezeSpec{}, errors.New("audit hash must be a source-free identifier")
		}
	}
	for _, org := range spec.Organizations {
		if org.Organization == "" || !safeIdentifier(org.Organization) || sourceLikeValue(org.Organization) {
			return PolicyFreezeSpec{}, errors.New("organization must be a source-free identifier")
		}
		if org.IncidentID != "" && (!safeIdentifier(org.IncidentID) || sourceLikeValue(org.IncidentID)) {
			return PolicyFreezeSpec{}, errors.New("incident_id must be a source-free identifier")
		}
		if org.HighStakes && org.IncidentActive && org.IncidentID == "" {
			return PolicyFreezeSpec{}, errors.New("active high-stakes incident requires incident_id")
		}
		if org.CurrentRelease == "" || org.ProposedRelease == "" {
			return PolicyFreezeSpec{}, errors.New("policy freeze organization requires current and proposed releases")
		}
		if org.CurrentRelease != "" && (!safeIdentifier(org.CurrentRelease) || sourceLikeValue(org.CurrentRelease)) {
			return PolicyFreezeSpec{}, errors.New("current release must be a source-free identifier")
		}
		if org.ProposedRelease != "" && (!safeIdentifier(org.ProposedRelease) || sourceLikeValue(org.ProposedRelease)) {
			return PolicyFreezeSpec{}, errors.New("proposed release must be a source-free identifier")
		}
	}
	return spec, nil
}

func ComputePolicyFreeze(spec PolicyFreezeSpec) (PolicyFreezeReport, error) {
	if spec.Version != PolicyFreezeSpecVersion {
		return PolicyFreezeReport{}, fmt.Errorf("policy freeze spec version must be %s", PolicyFreezeSpecVersion)
	}
	if _, err := parseSpecDate(spec.AsOfDate); err != nil {
		return PolicyFreezeReport{}, err
	}
	audited := map[string]AuditedDetectorRelease{}
	for _, release := range spec.AuditedReleases {
		audited[release.Release] = release
	}
	report := PolicyFreezeReport{
		Version:  PolicyFreezeReportVersion,
		OK:       true,
		SpecHash: canonical.Hash(spec),
		AsOfDate: spec.AsOfDate,
		Privacy:  defaultShareablePrivacy(),
	}
	orgs := append([]PolicyFreezeOrganization(nil), spec.Organizations...)
	sort.Slice(orgs, func(i, j int) bool { return orgs[i].Organization < orgs[j].Organization })
	for _, org := range orgs {
		decision := PolicyFreezeDecision{
			Organization:    org.Organization,
			HighStakes:      org.HighStakes,
			IncidentActive:  org.IncidentActive,
			ProposedRelease: org.ProposedRelease,
		}
		currentAudit, currentAudited := audited[org.CurrentRelease]
		proposedAudit, proposedAudited := audited[org.ProposedRelease]
		switch {
		case org.HighStakes && org.IncidentActive:
			decision.PinnedRelease = org.CurrentRelease
			decision.PolicyChangeAllowed = false
			if !currentAudited || !currentAudit.ApprovedForHighStakes {
				decision.Reason = "active_high_stakes_incident_missing_audited_current_release"
				report.Summary.BlockedMissingAudit++
				report.OK = false
			} else {
				decision.Reason = "active_high_stakes_incident_pins_audited_release"
				report.Summary.PinnedOrganizations++
			}
		case !proposedAudited:
			decision.PinnedRelease = org.CurrentRelease
			decision.PolicyChangeAllowed = false
			decision.Reason = "proposed_release_missing_audit"
			report.Summary.BlockedMissingAudit++
			report.OK = false
		case org.HighStakes && !proposedAudit.ApprovedForHighStakes:
			decision.PinnedRelease = org.CurrentRelease
			decision.PolicyChangeAllowed = false
			decision.Reason = "proposed_release_not_approved_for_high_stakes"
			report.Summary.BlockedMissingAudit++
			report.OK = false
		default:
			decision.PinnedRelease = org.ProposedRelease
			decision.PolicyChangeAllowed = true
			decision.Reason = "proposed_release_audited_and_no_active_freeze"
			report.Summary.AllowedUpdates++
		}
		report.Decisions = append(report.Decisions, decision)
		report.Summary.OrganizationsEvaluated++
	}
	report.Hash = hashLiveLearning(report)
	return report, nil
}

func ReadCalibrationMonitorSpec(reader io.Reader) (CalibrationMonitorSpec, error) {
	var spec CalibrationMonitorSpec
	if err := readLiveLearningSpec(reader, &spec); err != nil {
		return CalibrationMonitorSpec{}, err
	}
	if spec.Version != CalibrationMonitorSpecVersion {
		return CalibrationMonitorSpec{}, fmt.Errorf("calibration monitor spec version must be %s", CalibrationMonitorSpecVersion)
	}
	if err := validateBasisPoints("pre_registered_tolerance_bp", spec.PreRegisteredToleranceBP); err != nil {
		return CalibrationMonitorSpec{}, err
	}
	if spec.PreRegisteredToleranceBP == 0 {
		return CalibrationMonitorSpec{}, errors.New("pre_registered_tolerance_bp must be positive")
	}
	if spec.MinEvidence < MinKFloor {
		return CalibrationMonitorSpec{}, fmt.Errorf("calibration monitor min_evidence must be at least %d", MinKFloor)
	}
	for _, channel := range spec.AlertChannels {
		if channel == "" || !safeIdentifier(channel) || sourceLikeValue(channel) {
			return CalibrationMonitorSpec{}, errors.New("alert channel must be a source-free identifier")
		}
	}
	return spec, nil
}

func ComputeCalibrationMonitor(report Report, spec CalibrationMonitorSpec) (CalibrationMonitorReport, error) {
	if report.Version != ReportVersion {
		return CalibrationMonitorReport{}, fmt.Errorf("live feedback report version must be %s", ReportVersion)
	}
	if spec.Version != CalibrationMonitorSpecVersion {
		return CalibrationMonitorReport{}, fmt.Errorf("calibration monitor spec version must be %s", CalibrationMonitorSpecVersion)
	}
	out := CalibrationMonitorReport{
		Version:       CalibrationMonitorReportVersion,
		OK:            true,
		FeedbackHash:  canonical.Hash(report),
		SpecHash:      canonical.Hash(spec),
		EvidenceBasis: "published_k_anonymous_groups_only",
		Privacy:       defaultShareablePrivacy(),
		Summary: CalibrationMonitorSummary{
			MinEvidence: spec.MinEvidence,
			ToleranceBP: spec.PreRegisteredToleranceBP,
		},
	}
	aggregates := map[string]calibrationAccumulator{}
	for _, group := range report.Groups {
		if err := validateDetectorRelease(group.Detector, group.Release); err != nil {
			return CalibrationMonitorReport{}, err
		}
		expected, err := decileMidpointBP(group.ConfidenceDecile)
		if err != nil {
			return CalibrationMonitorReport{}, err
		}
		key := group.Detector + "\x00" + group.Release + "\x00" + group.ConfidenceDecile
		acc := aggregates[key]
		acc.detector = group.Detector
		acc.release = group.Release
		acc.decile = group.ConfidenceDecile
		acc.expectedBP = expected
		acc.count += group.Count
		if group.Verdict == "confirmed" {
			acc.confirmed += group.Count
		}
		aggregates[key] = acc
	}
	keys := make([]string, 0, len(aggregates))
	for key := range aggregates {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		acc := aggregates[key]
		if acc.count < spec.MinEvidence {
			continue
		}
		observed := ratioBP(acc.confirmed, acc.count)
		drift := absInt(observed - acc.expectedBP)
		decile := CalibrationDecile{
			Detector:                acc.detector,
			Release:                 acc.release,
			ConfidenceDecile:        acc.decile,
			PublishedCount:          acc.count,
			ExpectedConfirmedRateBP: acc.expectedBP,
			ObservedConfirmedRateBP: observed,
			DriftBP:                 drift,
		}
		out.Deciles = append(out.Deciles, decile)
		out.Summary.DecilesEvaluated++
		if drift > spec.PreRegisteredToleranceBP {
			out.Alerts = append(out.Alerts, CalibrationAlert{
				Detector:         acc.detector,
				Release:          acc.release,
				ConfidenceDecile: acc.decile,
				DriftBP:          drift,
				ToleranceBP:      spec.PreRegisteredToleranceBP,
				Severity:         calibrationSeverity(drift),
			})
		}
	}
	out.Summary.Alerts = len(out.Alerts)
	out.Hash = hashLiveLearning(out)
	return out, nil
}

func ReadRetentionLifecycleSpec(reader io.Reader) (RetentionLifecycleSpec, error) {
	var spec RetentionLifecycleSpec
	if err := readLiveLearningSpec(reader, &spec); err != nil {
		return RetentionLifecycleSpec{}, err
	}
	if spec.Version != RetentionLifecycleSpecVersion {
		return RetentionLifecycleSpec{}, fmt.Errorf("retention lifecycle spec version must be %s", RetentionLifecycleSpecVersion)
	}
	if _, err := parseSpecDate(spec.AsOfDate); err != nil {
		return RetentionLifecycleSpec{}, err
	}
	if len(spec.Policies) == 0 || len(spec.Artifacts) == 0 {
		return RetentionLifecycleSpec{}, errors.New("retention lifecycle spec requires policies and artifacts")
	}
	seenPolicy := map[string]struct{}{}
	for _, policy := range spec.Policies {
		if policy.Class == "" || !safeIdentifier(policy.Class) || sourceLikeValue(policy.Class) {
			return RetentionLifecycleSpec{}, errors.New("retention policy class must be source-free")
		}
		if policy.RawRetentionDays < 0 || policy.AnonymizeAfterDays < 0 || policy.AggregateRetentionDays < 0 {
			return RetentionLifecycleSpec{}, errors.New("retention policy days must be non-negative")
		}
		if policy.AnonymizeAfterDays > policy.RawRetentionDays {
			return RetentionLifecycleSpec{}, errors.New("anonymize_after_days cannot exceed raw_retention_days")
		}
		seenPolicy[policy.Class] = struct{}{}
	}
	for _, artifact := range spec.Artifacts {
		if artifact.ArtifactID == "" || !safeIdentifier(artifact.ArtifactID) || sourceLikeValue(artifact.ArtifactID) {
			return RetentionLifecycleSpec{}, errors.New("artifact_id must be source-free")
		}
		if _, ok := seenPolicy[artifact.Class]; !ok {
			return RetentionLifecycleSpec{}, fmt.Errorf("artifact %s references unknown retention class %s", artifact.ArtifactID, artifact.Class)
		}
		if _, err := parseSpecDate(artifact.CreatedAt); err != nil {
			return RetentionLifecycleSpec{}, err
		}
		if !validRetentionAction(artifact.ObservedAction) {
			return RetentionLifecycleSpec{}, fmt.Errorf("invalid observed retention action %s", artifact.ObservedAction)
		}
	}
	return spec, nil
}

func ComputeRetentionLifecycle(spec RetentionLifecycleSpec) (RetentionLifecycleReport, error) {
	if spec.Version != RetentionLifecycleSpecVersion {
		return RetentionLifecycleReport{}, fmt.Errorf("retention lifecycle spec version must be %s", RetentionLifecycleSpecVersion)
	}
	asOf, err := parseSpecDate(spec.AsOfDate)
	if err != nil {
		return RetentionLifecycleReport{}, err
	}
	policies := map[string]RetentionPolicy{}
	for _, policy := range spec.Policies {
		policies[policy.Class] = policy
	}
	report := RetentionLifecycleReport{
		Version:  RetentionLifecycleReportVersion,
		OK:       true,
		SpecHash: canonical.Hash(spec),
		AsOfDate: spec.AsOfDate,
		Privacy:  defaultShareablePrivacy(),
	}
	artifacts := append([]RetentionArtifact(nil), spec.Artifacts...)
	sort.Slice(artifacts, func(i, j int) bool { return artifacts[i].ArtifactID < artifacts[j].ArtifactID })
	for _, artifact := range artifacts {
		created, err := parseSpecDate(artifact.CreatedAt)
		if err != nil {
			return RetentionLifecycleReport{}, err
		}
		policy := policies[artifact.Class]
		age := int(asOf.Sub(created).Hours() / 24)
		expected, reason := expectedRetentionAction(artifact, policy, age)
		decision := RetentionDecision{
			ArtifactID:     artifact.ArtifactID,
			Class:          artifact.Class,
			AgeDays:        age,
			ExpectedAction: expected,
			ObservedAction: artifact.ObservedAction,
			Compliant:      expected == artifact.ObservedAction,
			Reason:         reason,
		}
		if decision.Compliant {
			report.Summary.CompliantArtifacts++
		} else {
			report.Summary.Violations++
			report.OK = false
		}
		switch expected {
		case "delete":
			report.Summary.DeleteRequired++
		case "anonymize":
			report.Summary.AnonymizeRequired++
		case "retain_aggregate":
			report.Summary.AggregateRetained++
		}
		report.Artifacts = append(report.Artifacts, decision)
		report.Summary.ArtifactsEvaluated++
	}
	report.Hash = hashLiveLearning(report)
	return report, nil
}

func ReadTrustRegressionSpec(reader io.Reader) (TrustRegressionSpec, error) {
	var spec TrustRegressionSpec
	if err := readLiveLearningSpec(reader, &spec); err != nil {
		return TrustRegressionSpec{}, err
	}
	if spec.Version != TrustRegressionSpecVersion {
		return TrustRegressionSpec{}, fmt.Errorf("trust regression spec version must be %s", TrustRegressionSpecVersion)
	}
	if err := validateTrustMetrics(spec.Baseline); err != nil {
		return TrustRegressionSpec{}, err
	}
	if err := validateTrustMetrics(spec.Current); err != nil {
		return TrustRegressionSpec{}, err
	}
	return spec, nil
}

func ComputeTrustRegression(spec TrustRegressionSpec) (TrustRegressionReport, error) {
	if spec.Version != TrustRegressionSpecVersion {
		return TrustRegressionReport{}, fmt.Errorf("trust regression spec version must be %s", TrustRegressionSpecVersion)
	}
	checks := []TrustRegressionCheck{
		buildTrustCheck("faithfulness", spec.Current.ExplanationFaithfulnessBP-spec.Baseline.ExplanationFaithfulnessBP, -spec.Tolerances.MaxFaithfulnessDropBP, ">="),
		buildTrustCheck("citation_coverage", spec.Current.EvidenceCitationCoverageBP-spec.Baseline.EvidenceCitationCoverageBP, -spec.Tolerances.MaxCitationCoverageDropBP, ">="),
		buildTrustCheck("uncertainty_disclosure", spec.Current.UncertaintyDisclosureBP-spec.Baseline.UncertaintyDisclosureBP, -spec.Tolerances.MaxUncertaintyDropBP, ">="),
		buildTrustCheck("overreliance", spec.Current.OverrelianceRateBP-spec.Baseline.OverrelianceRateBP, spec.Tolerances.MaxOverrelianceIncreaseBP, "<="),
		buildTrustCheck("review_burden", spec.Current.MeanReviewBurdenMinutes-spec.Baseline.MeanReviewBurdenMinutes, spec.Tolerances.MaxBurdenIncreaseMinutes, "<="),
	}
	report := TrustRegressionReport{
		Version:  TrustRegressionReportVersion,
		OK:       true,
		SpecHash: canonical.Hash(spec),
		Privacy:  defaultShareablePrivacy(),
		Checks:   checks,
	}
	for _, check := range checks {
		report.Summary.Checks++
		if check.Passed {
			report.Summary.PassedChecks++
		} else {
			report.Summary.FailedChecks++
			report.OK = false
		}
	}
	report.Hash = hashLiveLearning(report)
	return report, nil
}

func ReadMethodologySpec(reader io.Reader) (MethodologySpec, error) {
	var spec MethodologySpec
	if err := readLiveLearningSpec(reader, &spec); err != nil {
		return MethodologySpec{}, err
	}
	if spec.Version != MethodologySpecVersion {
		return MethodologySpec{}, fmt.Errorf("methodology spec version must be %s", MethodologySpecVersion)
	}
	if len(spec.Experiments) == 0 || len(spec.GateEvidence) == 0 {
		return MethodologySpec{}, errors.New("methodology spec requires experiments and gate evidence")
	}
	for _, experiment := range spec.Experiments {
		if experiment.Name == "" || !safeIdentifier(experiment.Name) || sourceLikeValue(experiment.Name) {
			return MethodologySpec{}, errors.New("methodology experiment name must be source-free")
		}
		if experiment.Population == "" || !safeIdentifier(experiment.Population) || sourceLikeValue(experiment.Population) {
			return MethodologySpec{}, errors.New("methodology population must be source-free")
		}
		for name, value := range map[string]int{
			"baseline_recall_bp":            experiment.BaselineRecallBP,
			"live_learning_recall_bp":       experiment.LiveLearningRecallBP,
			"baseline_overreliance_bp":      experiment.BaselineOverrelianceBP,
			"live_learning_overreliance_bp": experiment.LiveLearningOverrelianceBP,
		} {
			if err := validateBasisPoints(name, value); err != nil {
				return MethodologySpec{}, err
			}
		}
		if experiment.BaselineBurdenMinutes < 0 || experiment.LiveLearningBurdenMinutes < 0 {
			return MethodologySpec{}, errors.New("methodology burden minutes must be non-negative")
		}
		if len(experiment.Evidence) == 0 {
			return MethodologySpec{}, fmt.Errorf("methodology experiment %s requires evidence", experiment.Name)
		}
		for _, evidence := range experiment.Evidence {
			if evidence == "" || !safeIdentifier(evidence) || sourceLikeValue(evidence) {
				return MethodologySpec{}, errors.New("methodology evidence identifiers must be source-free")
			}
		}
	}
	for _, gate := range spec.GateEvidence {
		if gate.Gate == "" || !safeIdentifier(gate.Gate) || sourceLikeValue(gate.Gate) {
			return MethodologySpec{}, errors.New("methodology gate names must be source-free")
		}
		if gate.ReportHash == "" || !safeIdentifier(gate.ReportHash) || sourceLikeValue(gate.ReportHash) {
			return MethodologySpec{}, errors.New("methodology report hashes must be source-free")
		}
		if gate.ReproductionCommand == "" || !strings.HasPrefix(gate.ReproductionCommand, "make ") {
			return MethodologySpec{}, errors.New("methodology gate evidence must include make reproduction commands")
		}
	}
	return spec, nil
}

func ComputeMethodologyReport(spec MethodologySpec) (MethodologyReport, error) {
	if spec.Version != MethodologySpecVersion {
		return MethodologyReport{}, fmt.Errorf("methodology spec version must be %s", MethodologySpecVersion)
	}
	report := MethodologyReport{
		Version:      MethodologyReportVersion,
		OK:           true,
		SpecHash:     canonical.Hash(spec),
		Privacy:      defaultShareablePrivacy(),
		GateEvidence: append([]MethodologyGateEvidence(nil), spec.GateEvidence...),
	}
	for _, experiment := range spec.Experiments {
		result := MethodologyResult{
			Name:                experiment.Name,
			Population:          experiment.Population,
			RecallDeltaBP:       experiment.LiveLearningRecallBP - experiment.BaselineRecallBP,
			OverrelianceDeltaBP: experiment.LiveLearningOverrelianceBP - experiment.BaselineOverrelianceBP,
			BurdenDeltaMinutes:  experiment.LiveLearningBurdenMinutes - experiment.BaselineBurdenMinutes,
			Evidence:            append([]string(nil), experiment.Evidence...),
		}
		result.RecallImproved = result.RecallDeltaBP > 0
		result.OverrelianceNotIncreased = result.OverrelianceDeltaBP <= 0
		result.BurdenNotIncreased = result.BurdenDeltaMinutes <= 0
		if result.RecallImproved {
			report.Summary.RecallImproved++
		} else {
			report.OK = false
		}
		if result.OverrelianceNotIncreased {
			report.Summary.OverrelianceNotIncreased++
		} else {
			report.OK = false
		}
		if result.BurdenNotIncreased {
			report.Summary.BurdenNotIncreased++
		}
		report.Experiments = append(report.Experiments, result)
		report.Summary.Experiments++
	}
	report.Summary.LinkedGateEvidence = len(report.GateEvidence)
	sort.Slice(report.GateEvidence, func(i, j int) bool { return report.GateEvidence[i].Gate < report.GateEvidence[j].Gate })
	report.Hash = hashLiveLearning(report)
	return report, nil
}

func ReadDetectorDeprecationSpec(reader io.Reader) (DetectorDeprecationSpec, error) {
	var spec DetectorDeprecationSpec
	if err := readLiveLearningSpec(reader, &spec); err != nil {
		return DetectorDeprecationSpec{}, err
	}
	if err := validateDetectorDeprecationSpec(spec); err != nil {
		return DetectorDeprecationSpec{}, err
	}
	return spec, nil
}

func ComputeDetectorDeprecation(report Report, spec DetectorDeprecationSpec) (DetectorDeprecationReport, error) {
	if report.Version != ReportVersion {
		return DetectorDeprecationReport{}, fmt.Errorf("live feedback report version must be %s", ReportVersion)
	}
	if err := validateDetectorDeprecationSpec(spec); err != nil {
		return DetectorDeprecationReport{}, err
	}
	asOf, err := parseSpecDate(spec.AsOfDate)
	if err != nil {
		return DetectorDeprecationReport{}, err
	}
	out := DetectorDeprecationReport{
		Version:       DetectorDeprecationReportVersion,
		OK:            true,
		FeedbackHash:  canonical.Hash(report),
		SpecHash:      canonical.Hash(spec),
		AsOfDate:      spec.AsOfDate,
		EvidenceBasis: "published_k_anonymous_groups_only",
		Privacy:       defaultShareablePrivacy(),
	}
	targets := append([]DetectorDeprecationTarget(nil), spec.Detectors...)
	sort.Slice(targets, func(i, j int) bool {
		if targets[i].Detector != targets[j].Detector {
			return targets[i].Detector < targets[j].Detector
		}
		return targets[i].Release < targets[j].Release
	})
	for _, target := range targets {
		decision, err := evaluateDetectorDeprecation(report, spec, target, asOf)
		if err != nil {
			return DetectorDeprecationReport{}, err
		}
		out.Detectors = append(out.Detectors, decision)
		out.Summary.DetectorsEvaluated++
		switch decision.Status {
		case "retained_thresholds_met":
			out.Summary.Retained++
		case "monitor_insufficient_evidence":
			out.Summary.MonitoredInsufficientEvidence++
		case "notice_open_in_review":
			out.Summary.DeprecationNotices++
		case "ready_to_deprecate":
			out.Summary.DeprecationNotices++
			out.Summary.ReadyToDeprecate++
		case "blocked_process_violation":
			out.Summary.ProcessViolations++
			out.OK = false
		}
		for _, gate := range decision.ThresholdGates {
			if gate.Name == "precision" || gate.Name == "review_burden" {
				if !gate.Passed && decision.Metrics.PublishedCount >= spec.MinEvidence {
					out.Summary.ThresholdFailures++
					break
				}
			}
		}
	}
	out.Hash = hashLiveLearning(out)
	return out, nil
}

func validateDetectorDeprecationSpec(spec DetectorDeprecationSpec) error {
	if spec.Version != DetectorDeprecationSpecVersion {
		return fmt.Errorf("detector deprecation spec version must be %s", DetectorDeprecationSpecVersion)
	}
	if strings.TrimSpace(spec.Claim) == "" {
		return errors.New("detector deprecation spec claim is required")
	}
	if _, err := parseSpecDate(spec.AsOfDate); err != nil {
		return err
	}
	if spec.MinEvidence < MinKFloor {
		return fmt.Errorf("detector deprecation min_evidence must be at least %d", MinKFloor)
	}
	if err := validateBasisPoints("min_precision_bp", spec.MinPrecisionBP); err != nil {
		return err
	}
	if spec.MaxAverageBurdenMinutes <= 0 {
		return errors.New("detector deprecation max_average_burden_minutes must be positive")
	}
	if spec.MinNoticeDays < 0 || spec.MinReviewerRoles <= 0 || spec.MinAppealWindowDays < 0 {
		return errors.New("detector deprecation transparency minimums must be non-negative and require at least one reviewer role")
	}
	if len(spec.RequiredPublicChannels) == 0 {
		return errors.New("detector deprecation requires at least one public channel")
	}
	seenChannels := map[string]struct{}{}
	for _, channel := range spec.RequiredPublicChannels {
		if !safeSourceFreeLabel(channel) {
			return errors.New("detector deprecation public channels must be source-free labels")
		}
		seenChannels[channel] = struct{}{}
	}
	if len(seenChannels) != len(spec.RequiredPublicChannels) {
		return errors.New("detector deprecation required public channels must be unique")
	}
	if len(spec.Detectors) == 0 {
		return errors.New("detector deprecation spec requires detectors")
	}
	seenTargets := map[string]struct{}{}
	for _, target := range spec.Detectors {
		if err := validateDetectorRelease(target.Detector, target.Release); err != nil {
			return err
		}
		key := target.Detector + "|" + target.Release
		if _, ok := seenTargets[key]; ok {
			return fmt.Errorf("duplicate detector deprecation target %s", key)
		}
		seenTargets[key] = struct{}{}
		if target.Owner != "" && !safeSourceFreeLabel(target.Owner) {
			return errors.New("detector deprecation owner must be source-free")
		}
		if target.PublicNoticeID != "" && (!safeIdentifier(target.PublicNoticeID) || sourceLikeValue(target.PublicNoticeID)) {
			return errors.New("detector deprecation public_notice_id must be source-free")
		}
		if target.NoticeOpenedAt != "" {
			if _, err := parseSpecDate(target.NoticeOpenedAt); err != nil {
				return err
			}
		}
		for _, channel := range target.PublicChannels {
			if !safeSourceFreeLabel(channel) {
				return errors.New("detector deprecation public channels must be source-free labels")
			}
		}
		for _, role := range target.ReviewerRoles {
			if !safeSourceFreeLabel(role) {
				return errors.New("detector deprecation reviewer roles must be source-free labels")
			}
		}
		if target.ReplacementDetector != "" && target.ReplacementDetector != "none" && (!safeIdentifier(target.ReplacementDetector) || sourceLikeValue(target.ReplacementDetector)) {
			return errors.New("detector deprecation replacement_detector must be source-free")
		}
		if target.MigrationGuide != "" && !safeSourceFreeLabel(target.MigrationGuide) {
			return errors.New("detector deprecation migration_guide must be source-free")
		}
		if target.AppealWindowDays < 0 {
			return errors.New("detector deprecation appeal_window_days must be non-negative")
		}
	}
	return nil
}

func evaluateDetectorDeprecation(report Report, spec DetectorDeprecationSpec, target DetectorDeprecationTarget, asOf time.Time) (DetectorDeprecationDecision, error) {
	metrics := onlineMetrics(report, target.Detector, target.Release)
	decision := DetectorDeprecationDecision{
		Detector:            target.Detector,
		Release:             target.Release,
		Owner:               target.Owner,
		PublicNoticeID:      target.PublicNoticeID,
		ReplacementDetector: target.ReplacementDetector,
		MigrationGuide:      target.MigrationGuide,
		Metrics:             metrics,
		ThresholdGates: []OnlineGateResult{
			{
				Name:       "minimum_evidence",
				Passed:     metrics.PublishedCount >= spec.MinEvidence,
				Observed:   metrics.PublishedCount,
				Required:   spec.MinEvidence,
				Comparator: ">=",
			},
			{
				Name:       "precision",
				Passed:     metrics.PrecisionBP >= spec.MinPrecisionBP,
				Observed:   metrics.PrecisionBP,
				Required:   spec.MinPrecisionBP,
				Comparator: ">=",
			},
			{
				Name:       "review_burden",
				Passed:     metrics.AverageBurdenMinutes <= spec.MaxAverageBurdenMinutes && metrics.PublishedCount > 0,
				Observed:   metrics.AverageBurdenMinutes,
				Required:   spec.MaxAverageBurdenMinutes,
				Comparator: "<=",
			},
		},
	}
	for i := range decision.ThresholdGates {
		if !decision.ThresholdGates[i].Passed {
			decision.ThresholdGates[i].FailureReason = "threshold_not_met"
		}
	}
	if !decision.ThresholdGates[0].Passed {
		decision.Status = "monitor_insufficient_evidence"
		return decision, nil
	}
	if decision.ThresholdGates[1].Passed && decision.ThresholdGates[2].Passed {
		decision.Status = "retained_thresholds_met"
		return decision, nil
	}
	transparency, failures, missingChannels, noticeAge, err := detectorDeprecationTransparency(spec, target, asOf)
	if err != nil {
		return DetectorDeprecationDecision{}, err
	}
	decision.TransparencyGates = transparency
	decision.ProcessFailures = failures
	decision.MissingPublicChannels = missingChannels
	decision.NoticeAgeDays = noticeAge
	if len(failures) > 0 {
		decision.Status = "blocked_process_violation"
		return decision, nil
	}
	if noticeAge < requiredDetectorDeprecationNoticeAge(spec, target) {
		decision.Status = "notice_open_in_review"
		return decision, nil
	}
	decision.Status = "ready_to_deprecate"
	return decision, nil
}

func detectorDeprecationTransparency(spec DetectorDeprecationSpec, target DetectorDeprecationTarget, asOf time.Time) ([]OnlineGateResult, []string, []string, int, error) {
	var gates []OnlineGateResult
	var failures []string
	addPresenceGate := func(name, value string) {
		passed := strings.TrimSpace(value) != ""
		gate := OnlineGateResult{Name: name, Passed: passed, Comparator: "present"}
		if passed {
			gate.Observed = 1
			gate.Required = 1
		} else {
			gate.FailureReason = "missing_transparency_field"
			failures = append(failures, name)
		}
		gates = append(gates, gate)
	}
	addPresenceGate("owner", target.Owner)
	addPresenceGate("public_notice", target.PublicNoticeID)
	addPresenceGate("replacement_detector", target.ReplacementDetector)
	addPresenceGate("migration_guide", target.MigrationGuide)

	missingChannels := missingRequiredChannels(spec.RequiredPublicChannels, target.PublicChannels)
	channelsGate := OnlineGateResult{
		Name:       "required_public_channels",
		Passed:     len(missingChannels) == 0,
		Observed:   len(spec.RequiredPublicChannels) - len(missingChannels),
		Required:   len(spec.RequiredPublicChannels),
		Comparator: ">=",
	}
	if !channelsGate.Passed {
		channelsGate.FailureReason = "missing_public_channel"
		failures = append(failures, "required_public_channels")
	}
	gates = append(gates, channelsGate)

	roles := uniqueSourceFreeLabels(target.ReviewerRoles)
	rolesGate := OnlineGateResult{
		Name:       "independent_reviewer_roles",
		Passed:     len(roles) >= spec.MinReviewerRoles,
		Observed:   len(roles),
		Required:   spec.MinReviewerRoles,
		Comparator: ">=",
	}
	if !rolesGate.Passed {
		rolesGate.FailureReason = "insufficient_reviewer_roles"
		failures = append(failures, "independent_reviewer_roles")
	}
	gates = append(gates, rolesGate)

	appealGate := OnlineGateResult{
		Name:       "appeal_window_days",
		Passed:     target.AppealWindowDays >= spec.MinAppealWindowDays,
		Observed:   target.AppealWindowDays,
		Required:   spec.MinAppealWindowDays,
		Comparator: ">=",
	}
	if !appealGate.Passed {
		appealGate.FailureReason = "appeal_window_too_short"
		failures = append(failures, "appeal_window_days")
	}
	gates = append(gates, appealGate)

	noticeAge := -1
	requiredNoticeAge := requiredDetectorDeprecationNoticeAge(spec, target)
	noticeGate := OnlineGateResult{Name: "notice_age_days", Passed: false, Observed: noticeAge, Required: requiredNoticeAge, Comparator: ">="}
	if target.NoticeOpenedAt == "" {
		noticeGate.FailureReason = "missing_transparency_field"
		failures = append(failures, "notice_opened_at")
		gates = append(gates, noticeGate)
		return gates, failures, missingChannels, noticeAge, nil
	}
	opened, err := parseSpecDate(target.NoticeOpenedAt)
	if err != nil {
		return nil, nil, nil, 0, err
	}
	noticeAge = int(asOf.Sub(opened).Hours() / 24)
	noticeGate.Observed = noticeAge
	noticeGate.Passed = noticeAge >= requiredNoticeAge
	if noticeAge < 0 {
		noticeGate.FailureReason = "notice_opened_in_future"
		failures = append(failures, "notice_opened_in_future")
	} else if !noticeGate.Passed {
		noticeGate.FailureReason = "notice_or_appeal_period_in_progress"
	}
	gates = append(gates, noticeGate)
	return gates, failures, missingChannels, noticeAge, nil
}

func requiredDetectorDeprecationNoticeAge(spec DetectorDeprecationSpec, target DetectorDeprecationTarget) int {
	if target.AppealWindowDays > spec.MinNoticeDays {
		return target.AppealWindowDays
	}
	return spec.MinNoticeDays
}

type calibrationAccumulator struct {
	detector   string
	release    string
	decile     string
	expectedBP int
	count      int
	confirmed  int
}

func readLiveLearningSpec(reader io.Reader, target any) error {
	content, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	trimmed := bytes.TrimSpace(content)
	if len(trimmed) == 0 {
		return errors.New("live-learning spec is empty")
	}
	if containsBlockedKey(json.RawMessage(trimmed)) {
		return errors.New("live-learning spec contains blocked raw-evidence fields")
	}
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	return nil
}

func defaultShareablePrivacy() LiveLearningPrivacy {
	return LiveLearningPrivacy{
		SourceFree:             true,
		RawEvidenceFree:        true,
		IdentifierFree:         true,
		SaltEmitted:            false,
		IndividualExamplesFree: true,
		SuppressedFields:       []string{"source code", "diffs", "file paths", "finding IDs", "evidence hashes", "adopter IDs", "salts"},
	}
}

func evaluateOnlineCandidate(report Report, candidate OnlineDetectorCandidate) OnlineDetectorReport {
	metrics := onlineMetrics(report, candidate.Detector, candidate.Release)
	gates := []OnlineGateResult{
		{
			Name:       "minimum_evidence",
			Passed:     metrics.PublishedCount >= candidate.MinEvidence,
			Observed:   metrics.PublishedCount,
			Required:   candidate.MinEvidence,
			Comparator: ">=",
		},
		{
			Name:       "precision",
			Passed:     metrics.PrecisionBP >= candidate.MinPrecisionBP,
			Observed:   metrics.PrecisionBP,
			Required:   candidate.MinPrecisionBP,
			Comparator: ">=",
		},
		{
			Name:       "recall_proxy",
			Passed:     metrics.RecallBP >= candidate.MinRecallBP,
			Observed:   metrics.RecallBP,
			Required:   candidate.MinRecallBP,
			Comparator: ">=",
		},
		{
			Name:       "review_burden",
			Passed:     metrics.AverageBurdenMinutes <= candidate.MaxAverageBurdenMinutes && metrics.PublishedCount > 0,
			Observed:   metrics.AverageBurdenMinutes,
			Required:   candidate.MaxAverageBurdenMinutes,
			Comparator: "<=",
		},
	}
	passed := true
	for i := range gates {
		if !gates[i].Passed {
			gates[i].FailureReason = "candidate_remains_shadow_only"
			passed = false
		}
	}
	status := "shadow_only"
	if passed {
		status = "candidate_ready_for_gated_review"
	}
	return OnlineDetectorReport{
		Detector: candidate.Detector,
		Release:  candidate.Release,
		Status:   status,
		Metrics:  metrics,
		Gates:    gates,
	}
}

func onlineMetrics(report Report, detector, release string) OnlineDetectorMetrics {
	var metrics OnlineDetectorMetrics
	for _, group := range report.Groups {
		if group.Detector != detector || group.Release != release {
			continue
		}
		metrics.PublishedCount += group.Count
		metrics.TotalBurdenMinutes += group.TotalBurdenMinutes
		switch group.Verdict {
		case "confirmed":
			metrics.Confirmed += group.Count
		case "false_positive":
			metrics.FalsePositive += group.Count
		case "missed":
			metrics.Missed += group.Count
		case "uncertain":
			metrics.Uncertain += group.Count
		}
	}
	if metrics.PublishedCount > 0 {
		metrics.AverageBurdenMinutes = roundedDiv(metrics.TotalBurdenMinutes, metrics.PublishedCount)
	}
	metrics.PrecisionBP = ratioBP(metrics.Confirmed, metrics.Confirmed+metrics.FalsePositive+metrics.Uncertain)
	metrics.RecallBP = ratioBP(metrics.Confirmed, metrics.Confirmed+metrics.Missed)
	return metrics
}

func buildActiveLearningAggregate(items []ActiveLearningQueueItem) ActiveLearningAggregate {
	aggregate := ActiveLearningAggregate{
		Version:     "patchline.adopter-active-learning-aggregate/v1",
		Shareable:   true,
		SourceFree:  true,
		QueuedCases: len(items),
	}
	type acc struct {
		detector string
		release  string
		count    int
		uncert   int
		gain     int
	}
	byDetector := map[string]acc{}
	totalUncertainty := 0
	totalGain := 0
	totalBurden := 0
	for _, item := range items {
		totalUncertainty += item.UncertaintyBP
		totalGain += item.ExpectedInformationGainBP
		totalBurden += item.EstimatedBurdenMinutes
		key := item.Detector + "\x00" + item.Release
		current := byDetector[key]
		current.detector = item.Detector
		current.release = item.Release
		current.count++
		current.uncert += item.UncertaintyBP
		current.gain += item.ExpectedInformationGainBP
		byDetector[key] = current
	}
	if len(items) > 0 {
		aggregate.MeanUncertaintyBP = roundedDiv(totalUncertainty, len(items))
		aggregate.MeanExpectedInformationGainBP = roundedDiv(totalGain, len(items))
		aggregate.MeanBurdenMinutes = roundedDiv(totalBurden, len(items))
	}
	keys := make([]string, 0, len(byDetector))
	for key := range byDetector {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value := byDetector[key]
		aggregate.ByDetector = append(aggregate.ByDetector, ActiveLearningDetectorAggregate{
			Detector:                      value.detector,
			Release:                       value.release,
			QueuedCases:                   value.count,
			MeanUncertaintyBP:             roundedDiv(value.uncert, value.count),
			MeanExpectedInformationGainBP: roundedDiv(value.gain, value.count),
		})
	}
	hashable := aggregate
	hashable.Hash = ""
	aggregate.Hash = canonical.Hash(hashable)
	return aggregate
}

func expectedRetentionAction(artifact RetentionArtifact, policy RetentionPolicy, age int) (string, string) {
	if (artifact.ContainsRawEvidence || artifact.ContainsLocalExamples) && age > policy.RawRetentionDays {
		return "delete", "raw_or_local_feedback_exceeded_retention"
	}
	if (artifact.ContainsRawEvidence || artifact.ContainsLocalExamples) && age > policy.AnonymizeAfterDays {
		return "anonymize", "raw_or_local_feedback_exceeded_anonymization_window"
	}
	if artifact.Aggregated {
		if age > policy.AggregateRetentionDays {
			return "delete", "aggregate_exceeded_retention"
		}
		return "retain_aggregate", "aggregate_within_retention"
	}
	return "retain_local", "local_feedback_within_retention"
}

func buildTrustCheck(name string, delta, tolerance int, comparator string) TrustRegressionCheck {
	passed := false
	switch comparator {
	case ">=":
		passed = delta >= tolerance
	case "<=":
		passed = delta <= tolerance
	}
	return TrustRegressionCheck{
		Name:       name,
		Passed:     passed,
		Delta:      delta,
		Tolerance:  tolerance,
		Comparator: comparator,
	}
}

func validateTrustMetrics(metrics TrustMetrics) error {
	if metrics.Release == "" || !safeIdentifier(metrics.Release) || sourceLikeValue(metrics.Release) {
		return errors.New("trust metric release must be source-free")
	}
	for name, value := range map[string]int{
		"explanation_faithfulness_bp":   metrics.ExplanationFaithfulnessBP,
		"evidence_citation_coverage_bp": metrics.EvidenceCitationCoverageBP,
		"uncertainty_disclosure_bp":     metrics.UncertaintyDisclosureBP,
		"overreliance_rate_bp":          metrics.OverrelianceRateBP,
	} {
		if err := validateBasisPoints(name, value); err != nil {
			return err
		}
	}
	if metrics.MeanReviewBurdenMinutes < 0 {
		return errors.New("trust metric review burden must be non-negative")
	}
	return nil
}

func validateDetectorRelease(detector, release string) error {
	if detector == "" || !safeIdentifier(detector) || sourceLikeValue(detector) {
		return errors.New("detector must be a source-free identifier")
	}
	if release == "" || !safeIdentifier(release) || sourceLikeValue(release) {
		return errors.New("release must be a source-free identifier")
	}
	return nil
}

func safeSourceFreeLabel(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 180 || strings.ContainsAny(value, "\r\n\t") {
		return false
	}
	return !sourceLikeValue(value)
}

func missingRequiredChannels(required, observed []string) []string {
	seen := map[string]struct{}{}
	for _, channel := range observed {
		seen[channel] = struct{}{}
	}
	var missing []string
	for _, channel := range required {
		if _, ok := seen[channel]; !ok {
			missing = append(missing, channel)
		}
	}
	sort.Strings(missing)
	return missing
}

func uniqueSourceFreeLabels(values []string) []string {
	seen := map[string]struct{}{}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		seen[trimmed] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for value := range seen {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func validateBasisPoints(name string, value int) error {
	if value < 0 || value > basisPointsScale {
		return fmt.Errorf("%s must be between 0 and %d", name, basisPointsScale)
	}
	return nil
}

func ratioBP(numerator, denominator int) int {
	if denominator <= 0 {
		return 0
	}
	return roundedDiv(numerator*basisPointsScale, denominator)
}

func roundedDiv(numerator, denominator int) int {
	if denominator <= 0 {
		return 0
	}
	return (numerator + denominator/2) / denominator
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func decileMidpointBP(decile string) (int, error) {
	parts := strings.Split(decile, "-")
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid confidence decile %q", decile)
	}
	low, err := decimalToBP(parts[0])
	if err != nil {
		return 0, fmt.Errorf("invalid confidence decile %q", decile)
	}
	high, err := decimalToBP(parts[1])
	if err != nil {
		return 0, fmt.Errorf("invalid confidence decile %q", decile)
	}
	if low < 0 || high > basisPointsScale || low >= high {
		return 0, fmt.Errorf("invalid confidence decile %q", decile)
	}
	return roundedDiv(low+high, 2), nil
}

func decimalToBP(value string) (int, error) {
	parts := strings.Split(value, ".")
	if len(parts) > 2 {
		return 0, fmt.Errorf("invalid decimal %q", value)
	}
	whole, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, err
	}
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
	}
	if len(fraction) > 4 {
		fraction = fraction[:4]
	}
	for len(fraction) < 4 {
		fraction += "0"
	}
	frac, err := strconv.Atoi(fraction)
	if err != nil {
		return 0, err
	}
	return whole*basisPointsScale + frac, nil
}

func calibrationSeverity(drift int) string {
	switch {
	case drift >= 5000:
		return "critical"
	case drift >= 2500:
		return "high"
	default:
		return "medium"
	}
}

func parseSpecDate(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, errors.New("date value is required")
	}
	parsed, err := time.Parse(dateLayout, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("date %q must use %s", value, dateLayout)
	}
	return parsed, nil
}

func validRetentionAction(value string) bool {
	switch value {
	case "delete", "anonymize", "retain_aggregate", "retain_local":
		return true
	default:
		return false
	}
}

func hashLiveLearning[T any](value T) string {
	switch typed := any(&value).(type) {
	case *OnlineEvaluationReport:
		typed.Hash = ""
	case *ActiveLearningReport:
		typed.Hash = ""
	case *PolicyFreezeReport:
		typed.Hash = ""
	case *CalibrationMonitorReport:
		typed.Hash = ""
	case *RetentionLifecycleReport:
		typed.Hash = ""
	case *TrustRegressionReport:
		typed.Hash = ""
	case *MethodologyReport:
		typed.Hash = ""
	case *DetectorDeprecationReport:
		typed.Hash = ""
	}
	return canonical.Hash(value)
}
